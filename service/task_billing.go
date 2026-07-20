package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// LogTaskConsumption 记录任务消费日志和统计信息（仅记录，不涉及实际扣费）。
// 实际扣费已由 BillingSession（PreConsumeBilling + SettleBilling）完成。
func LogTaskConsumption(c *gin.Context, info *relaycommon.RelayInfo) int {
	tokenName := c.GetString("token_name")
	logContent := fmt.Sprintf("操作 %s", info.Action)
	// 支持任务仅按次计费
	if common.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
		logContent = fmt.Sprintf("%s，按次计费", logContent)
	} else {
		if len(info.PriceData.OtherRatios) > 0 {
			var contents []string
			for key, ra := range info.PriceData.OtherRatios {
				if 1.0 != ra {
					contents = append(contents, fmt.Sprintf("%s: %.2f", key, ra))
				}
			}
			if len(contents) > 0 {
				logContent = fmt.Sprintf("%s, 计算参数：%s", logContent, strings.Join(contents, ", "))
			}
		}
	}
	other := make(map[string]interface{})
	other["is_task"] = true
	other["request_path"] = c.Request.URL.Path
	other["model_price"] = info.PriceData.ModelPrice
	if info.PriceData.ModelRatio > 0 {
		other["model_ratio"] = info.PriceData.ModelRatio
	}
	other["group_ratio"] = info.PriceData.GroupRatioInfo.GroupRatio
	if info.PriceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = info.PriceData.GroupRatioInfo.GroupSpecialRatio
	}
	if info.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = info.UpstreamModelName
	}
	// 记录非敏感的任务请求参数（mode/size/duration/prompt），便于日志详情查看与审计。
	if req, err := relaycommon.GetTaskRequest(c); err == nil {
		if req.Prompt != "" {
			other["prompt"] = req.Prompt
		}
		if req.Mode != "" {
			other["mode"] = req.Mode
		}
		if req.Size != "" {
			other["size"] = req.Size
		}
		if req.Duration > 0 {
			other["duration"] = req.Duration
		}
	}
	attachQuotaSaturation(c, info, other)
	submitLogID := model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
		ChannelId: info.ChannelId,
		ModelName: info.OriginModelName,
		TokenName: tokenName,
		Quota:     info.PriceData.Quota,
		Content:   logContent,
		TokenId:   info.TokenId,
		Group:     info.UsingGroup,
		Other:     other,
	})
	model.UpdateUserUsedQuotaAndRequestCount(info.UserId, info.PriceData.Quota)
	model.UpdateChannelUsedQuota(info.ChannelId, info.PriceData.Quota)
	return submitLogID
}

// ---------------------------------------------------------------------------
// 异步任务计费辅助函数
// ---------------------------------------------------------------------------

// taskUseTimeSeconds 计算任务实际耗时（秒），用于结算时回填消费日志的 use_time。
// 优先用 finish-submit；缺 finish_time 时退回到 finish-start，均不可用则返回 0。
func taskUseTimeSeconds(task *model.Task) int {
	if task.FinishTime <= 0 {
		return 0
	}
	base := task.SubmitTime
	if base <= 0 {
		base = task.StartTime
	}
	if base <= 0 || task.FinishTime < base {
		return 0
	}
	return int(task.FinishTime - base)
}

// resolveTokenKey 通过 TokenId 运行时获取令牌 Key（用于 Redis 缓存操作）。
// 如果令牌已被删除或查询失败，返回空字符串。
func resolveTokenKey(ctx context.Context, tokenId int, taskID string) string {
	token, err := model.GetTokenById(tokenId)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("获取令牌 key 失败 (tokenId=%d, task=%s): %s", tokenId, taskID, err.Error()))
		return ""
	}
	return token.Key
}

// taskIsSubscription 判断任务是否通过订阅计费。
func taskIsSubscription(task *model.Task) bool {
	return task.PrivateData.BillingSource == BillingSourceSubscription && task.PrivateData.SubscriptionId > 0
}

// taskAdjustFunding 调整任务的资金来源（钱包或订阅），delta > 0 表示扣费，delta < 0 表示退还。
func taskAdjustFunding(task *model.Task, delta int) error {
	if taskIsSubscription(task) {
		return model.PostConsumeUserSubscriptionDelta(task.PrivateData.SubscriptionId, int64(delta))
	}
	if delta > 0 {
		return model.DecreaseUserQuota(task.UserId, delta, false)
	}
	return model.IncreaseUserQuota(task.UserId, -delta, false)
}

// taskAdjustTokenQuota 调整任务的令牌额度，delta > 0 表示扣费，delta < 0 表示退还。
// 需要通过 resolveTokenKey 运行时获取 key（不从 PrivateData 中读取）。
func taskAdjustTokenQuota(ctx context.Context, task *model.Task, delta int) {
	if task.PrivateData.TokenId <= 0 || delta == 0 {
		return
	}
	tokenKey := resolveTokenKey(ctx, task.PrivateData.TokenId, task.TaskID)
	if tokenKey == "" {
		return
	}
	var err error
	if delta > 0 {
		err = model.DecreaseTokenQuota(task.PrivateData.TokenId, tokenKey, delta)
	} else {
		err = model.IncreaseTokenQuota(task.PrivateData.TokenId, tokenKey, -delta)
	}
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("调整令牌额度失败 (delta=%d, task=%s): %s", delta, task.TaskID, err.Error()))
	}
}

// taskBillingOther 从 task 的 BillingContext 构建日志 Other 字段。
func taskBillingOther(task *model.Task) map[string]interface{} {
	other := make(map[string]interface{})
	if len(task.PrivateData.UsedChannels) > 0 {
		other["admin_info"] = imageTaskAdminInfo(task)
	}
	if bc := task.PrivateData.BillingContext; bc != nil {
		other["model_price"] = bc.ModelPrice
		if bc.ModelRatio > 0 {
			other["model_ratio"] = bc.ModelRatio
		}
		other["group_ratio"] = bc.GroupRatio
		if len(bc.OtherRatios) > 0 {
			for k, v := range bc.OtherRatios {
				other[k] = v
			}
		}
	}
	props := task.Properties
	if props.UpstreamModelName != "" && props.UpstreamModelName != props.OriginModelName {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = props.UpstreamModelName
	}
	return other
}

// taskModelName 从 BillingContext 或 Properties 中获取模型名称。
func taskModelName(task *model.Task) string {
	if bc := task.PrivateData.BillingContext; bc != nil && bc.OriginModelName != "" {
		return bc.OriginModelName
	}
	return task.Properties.OriginModelName
}

// RefundTaskQuota 统一的任务失败退款逻辑。
// 当异步任务失败时，将预扣的 quota 退还给用户（支持钱包和订阅），并退还令牌额度。
// 返回资金来源是否已成功退还；失败时保留 quota 作为后续对账标记。
func RefundTaskQuota(ctx context.Context, task *model.Task, reason string) bool {
	quota := task.Quota
	if quota == 0 {
		return true
	}

	// 1. 退还资金来源（钱包或订阅）
	if err := taskAdjustFunding(task, -quota); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("退还资金来源失败 task %s: %s", task.TaskID, err.Error()))
		return false
	}

	// 2. 退还令牌额度
	taskAdjustTokenQuota(ctx, task, -quota)

	// 3. 记录日志
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["reason"] = reason
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		Content:   "",
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     quota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})

	// 4. 资金退款完成后再清除持久化标记；失败时保留非零 quota，
	// 由后续对账重试。回写失败必须显式告警，避免漏掉潜在的重复退款风险。
	task.Quota = 0
	if err := task.UpdateQuota(); err != nil {
		logger.LogError(ctx, fmt.Sprintf("退款成功但清除 task quota 失败 task %s: %s", task.TaskID, err.Error()))
	}
	return true
}

// 异步生图退款统一原则：只看上游是否报告了 token 消耗（即上游是否已对本站计费），
// 不看是否返回图片。上游有消耗 → 不退款；上游零消耗 → 全额退款。
// 以下两个函数分别是失败路径（用量存在 ErrorDetail）和成功路径（用量为入参）的出口。

// RefundFailedTaskQuotaByUpstreamUsage 失败任务的统一退款出口（方案A）。
// 以"上游是否已计费"为准，而非是否返图或是否回显 token：
// 上游返回 200 但没给图（UpstreamBilled=true）视为上游已在其侧计费，不退款、扣住预扣值；
// 非 200（真实上游错误、网络失败等）则全额退款。
func RefundFailedTaskQuotaByUpstreamUsage(ctx context.Context, task *model.Task) {
	if detail := task.PrivateData.ErrorDetail; detail != nil && detail.UpstreamBilled {
		logger.LogWarn(ctx, fmt.Sprintf(
			"任务 %s 失败但上游返回200已计费(prompt=%d, completion=%d tokens)，不退款，用户已扣费 %s",
			task.TaskID, detail.UpstreamPromptTokens, detail.UpstreamCompletionTokens,
			logger.LogQuota(task.Quota)))
		return
	}
	RefundTaskQuota(ctx, task, task.FailReason)
}

// RefundZeroUsageTaskQuota 成功任务的零用量退款检查：上游既未报告任何 token 用量、
// 也未返回图片（视为未提供服务，常见于风控拦截）时全额退款。
// 只要返了图（部分上游正常返图但不回显 token 用量）或报告了任何用量，都视为服务已提供，正常扣费不退款。
// scene 仅用于日志标注调用来源。
func RefundZeroUsageTaskQuota(ctx context.Context, task *model.Task, promptTokens, completionTokens, imageCount int, scene string) {
	if task.Quota <= 0 || promptTokens > 0 || completionTokens > 0 {
		return
	}
	if imageCount > 0 {
		logger.LogInfo(ctx, fmt.Sprintf(
			"%s: 上游未回显token用量但已正常返图（%d 张），视为服务已提供，正常扣费，任务 %s",
			scene, imageCount, task.TaskID))
		return
	}
	logger.LogWarn(ctx, fmt.Sprintf(
		"%s: 上游未返回任何token用量（未计费，疑似风控），退还扣费，任务 %s，模型 %s，额度 %s",
		scene, task.TaskID, taskModelName(task), logger.LogQuota(task.Quota)))
	RefundTaskQuota(ctx, task, "上游未返回任何token用量（未计费，疑似风控），退还全部扣费")
}

// RecalculateTaskQuota 通用的异步差额结算，更新原有的提交日志。
// actualQuota 是任务完成后的实际应扣额度，与预扣额度 (task.Quota) 做差额结算。
// reason 用于日志记录（例如 "token重算" 或 "adaptor调整"）。
// clamps 可选：若计算 actualQuota 时发生额度饱和，将其记入日志 admin_info（仅管理员可见）。
func RecalculateTaskQuota(ctx context.Context, task *model.Task, actualQuota int, reason string, clamps ...*common.QuotaClamp) {
	if actualQuota <= 0 {
		return
	}
	preConsumedQuota := task.Quota
	quotaDelta := actualQuota - preConsumedQuota

	// 任务实际耗时（提交时日志的 use_time 为 0，完成结算时才知道）。
	useTimeSeconds := taskUseTimeSeconds(task)

	if quotaDelta == 0 {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 预扣费准确（%s，%s）",
			task.TaskID, logger.LogQuota(actualQuota), reason))
		// 即使没有差额，也要更新日志的 other 字段，添加 actual_quota 信息
		if task.PrivateData.SubmitLogID > 0 {
			otherUpdates := map[string]interface{}{
				"pre_consumed_quota": preConsumedQuota,
				"actual_quota":       actualQuota,
				"settlement_reason":  reason,
			}
			if useTimeSeconds > 0 {
				otherUpdates["use_time_seconds"] = useTimeSeconds
			}
			for _, clamp := range clamps {
				attachQuotaSaturationToOther(otherUpdates, clamp)
			}
			model.UpdateConsumeLogQuotaAndOther(task.PrivateData.SubmitLogID, actualQuota, otherUpdates)
		}
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("任务 %s 差额结算：delta=%s（实际：%s，预扣：%s，%s）",
		task.TaskID,
		logger.LogQuota(quotaDelta),
		logger.LogQuota(actualQuota),
		logger.LogQuota(preConsumedQuota),
		reason,
	))

	// 调整资金来源
	if err := taskAdjustFunding(task, quotaDelta); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算资金调整失败 task %s: %s", task.TaskID, err.Error()))
		return
	}

	// 调整令牌额度
	taskAdjustTokenQuota(ctx, task, quotaDelta)

	task.Quota = actualQuota
	_ = task.Update()

	// 更新用户和渠道统计（只更新差额部分；负差额即退款场景做相应扣减）
	model.UpdateUserUsedQuotaAndRequestCount(task.UserId, quotaDelta)
	model.UpdateChannelUsedQuota(task.ChannelId, quotaDelta)

	// 更新原有的提交日志，而不是创建新日志
	if task.PrivateData.SubmitLogID > 0 {
		otherUpdates := map[string]interface{}{
			"pre_consumed_quota": preConsumedQuota,
			"actual_quota":       actualQuota,
			"settlement_reason":  reason,
		}

		if useTimeSeconds > 0 {
			otherUpdates["use_time_seconds"] = useTimeSeconds
		}
		for _, clamp := range clamps {
			attachQuotaSaturationToOther(otherUpdates, clamp)
		}

		// 添加 tiered_expr 计费信息（如果提交时没有的话）
		if bc := task.PrivateData.BillingContext; bc != nil && len(bc.TieredSnapshot) > 0 {
			var snap struct {
				ExprString    string `json:"expr_string"`
				EstimatedTier string `json:"estimated_tier"`
			}
			if err := common.Unmarshal(bc.TieredSnapshot, &snap); err == nil {
				otherUpdates["billing_mode"] = "tiered_expr"
				otherUpdates["expr_b64"] = base64.StdEncoding.EncodeToString([]byte(snap.ExprString))
				otherUpdates["matched_tier"] = snap.EstimatedTier
			}
		}

		model.UpdateConsumeLogQuotaAndOther(task.PrivateData.SubmitLogID, actualQuota, otherUpdates)
	}
}

// RecalculateTaskQuotaByTokens 根据实际 token 消耗重新计费（异步差额结算）。
// 当任务成功且返回了 totalTokens 时，根据模型倍率和分组倍率重新计算实际扣费额度，
// 与预扣费的差额进行补扣或退还。支持钱包和订阅计费来源。
func RecalculateTaskQuotaByTokens(ctx context.Context, task *model.Task, totalTokens int) {
	if totalTokens <= 0 {
		return
	}

	modelName := taskModelName(task)

	// 获取模型价格和倍率
	modelRatio, hasRatioSetting, _ := ratio_setting.GetModelRatio(modelName)
	// 只有配置了倍率(非固定价格)时才按 token 重新计费
	if !hasRatioSetting || modelRatio <= 0 {
		return
	}

	// 获取用户和组的倍率信息
	group := task.Group
	if group == "" {
		user, err := model.GetUserById(task.UserId, false)
		if err == nil {
			group = user.Group
		}
	}
	if group == "" {
		return
	}

	groupRatio := ratio_setting.GetGroupRatio(group)
	userGroupRatio, hasUserGroupRatio := ratio_setting.GetGroupGroupRatio(group, group)

	var finalGroupRatio float64
	if hasUserGroupRatio {
		finalGroupRatio = userGroupRatio
	} else {
		finalGroupRatio = groupRatio
	}

	// 计算 OtherRatios 乘积（视频折扣、时长等）
	otherMultiplier := 1.0
	if bc := task.PrivateData.BillingContext; bc != nil {
		for _, r := range bc.OtherRatios {
			if r != 1.0 && r > 0 {
				otherMultiplier *= r
			}
		}
	}

	// 计算实际应扣费额度: totalTokens * modelRatio * groupRatio * otherMultiplier（饱和转换，防止溢出成负数）
	actualQuota, clamp := common.QuotaFromFloatChecked(float64(totalTokens) * modelRatio * finalGroupRatio * otherMultiplier)

	reason := fmt.Sprintf("token重算：tokens=%d, modelRatio=%.2f, groupRatio=%.2f, otherMultiplier=%.4f", totalTokens, modelRatio, finalGroupRatio, otherMultiplier)
	RecalculateTaskQuota(ctx, task, actualQuota, reason, clamp)
}

// SettleAsyncImageTaskBilling 异步生图任务完成后的统一差额结算入口。
// tiered_expr 模型用冻结的 BillingSnapshot + 真实 token 重算；
// 其余按 token 计费的模型走倍率重算；按次计费（PerCallBilling）不重算。
// tokenDetails 中的 image_tokens / image_output_tokens 分别映射到表达式的 img / img_o 变量。
//
// 契约：本函数不为 img/img_o 做兜底，各异步路径的 usage 提取函数必须自行填好
// tokenDetails（Gemini 取 candidatesTokensDetails 的 IMAGE modality；OpenAI 图像
// 端点无输出明细时输出 token 全记为 image_output_tokens）。漏填不会报错，
// 图像输出会静默按低价的 c 计费。另 TieredSnapshot 为空时静默退回倍率/按次
// 逻辑——新增提交入口漏存快照即触发（见 prepareAsyncBilling 的契约说明）。
func SettleAsyncImageTaskBilling(ctx context.Context, task *model.Task, promptTokens, completionTokens int, tokenDetails map[string]interface{}) {
	bc := task.PrivateData.BillingContext
	if bc == nil {
		return
	}

	if len(bc.TieredSnapshot) == 0 {
		if !bc.PerCallBilling {
			RecalculateTaskQuotaByTokens(ctx, task, promptTokens+completionTokens)
		}
		return
	}

	var snap billingexpr.BillingSnapshot
	if err := common.Unmarshal(bc.TieredSnapshot, &snap); err != nil {
		logger.LogError(ctx, fmt.Sprintf("任务 %s tiered 结算失败：快照解析错误 %v", task.TaskID, err))
		return
	}

	params := billingexpr.TokenParams{
		P:   float64(promptTokens),
		C:   float64(completionTokens),
		Len: float64(promptTokens + completionTokens),
	}
	if v, ok := tokenDetails["image_tokens"].(int); ok && v > 0 {
		params.Img = float64(v)
		params.P -= params.Img
		if params.P < 0 {
			params.P = 0
		}
	}
	if v, ok := tokenDetails["image_output_tokens"].(int); ok && v > 0 {
		params.ImgO = float64(v)
		params.C -= params.ImgO
		if params.C < 0 {
			params.C = 0
		}
	}

	tr, err := billingexpr.ComputeTieredQuota(&snap, params)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("任务 %s tiered 结算失败：表达式计算错误 %v", task.TaskID, err))
		return
	}

	var parts []string
	if params.P > 0 {
		parts = append(parts, fmt.Sprintf("文本输入%.0f tokens", params.P))
	}
	if params.C > 0 {
		parts = append(parts, fmt.Sprintf("文本输出%.0f tokens", params.C))
	}
	if params.Img > 0 {
		parts = append(parts, fmt.Sprintf("图像输入%.0f tokens", params.Img))
	}
	if params.ImgO > 0 {
		parts = append(parts, fmt.Sprintf("图像输出%.0f tokens", params.ImgO))
	}
	breakdown := strings.Join(parts, " + ")
	finalCost := float64(tr.ActualQuotaAfterGroup) / snap.QuotaPerUnit
	RecalculateTaskQuota(ctx, task, tr.ActualQuotaAfterGroup,
		fmt.Sprintf("tiered_expr重算 [%s档]：%s → $%.3f (%d额度)",
			tr.MatchedTier, breakdown, finalCost, tr.ActualQuotaAfterGroup))
}
