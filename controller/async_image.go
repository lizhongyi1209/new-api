package controller

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

func AsyncImageSubmit(c *gin.Context) {
	var req dto.AsyncImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("请求参数错误: %v", err),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	if err := service.ValidateAsyncImageReferences(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	userId := c.GetInt("id")
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	channelId := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	tokenId := c.GetInt("token_id")

	// Build relay info for billing
	relayInfo, priceData, billingErr := prepareAsyncBilling(c, userId, group, channelId, tokenId, req.Model)
	if billingErr != nil {
		c.JSON(billingErr.StatusCode, gin.H{
			"error": gin.H{
				"message": billingErr.Error(),
				"type":    "billing_error",
			},
		})
		return
	}

	task := &model.Task{
		TaskID:     model.GenerateTaskID(),
		UserId:     userId,
		Group:      group,
		ChannelId:  channelId,
		Platform:   constant.TaskPlatformUnifiedImage,
		Action:     "generate",
		Status:     model.TaskStatusSubmitted,
		Progress:   "0%",
		SubmitTime: time.Now().Unix(),
		Quota:      priceData.Quota,
		Properties: model.Properties{
			Input:           req.Prompt,
			OriginModelName: req.Model,
			TokenId:         tokenId,
			RequestHost:     c.Request.Host,
		},
	}

	// Store billing context for later refund/settlement
	// tiered_expr 模型即使预扣为 0（信任用户）也必须存 BillingContext，否则完成时无法结算
	if relayInfo != nil && (priceData.Quota > 0 || c.GetBool("tiered_billing_active")) {
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = tokenId
		bc := &model.TaskBillingContext{
			ModelPrice:      priceData.ModelPrice,
			GroupRatio:      priceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      priceData.ModelRatio,
			OtherRatios:     priceData.OtherRatios,
			OriginModelName: req.Model,
			PerCallBilling:  priceData.UsePrice,
		}
		if snapBytes, ok := c.Get("tiered_snapshot_bytes"); ok {
			if b, ok := snapBytes.([]byte); ok {
				bc.TieredSnapshot = b
			}
		}
		task.PrivateData.BillingContext = bc
	}

	// Check if this should use Gemini native processing
	// Models with nano-banana prefix or specific Gemini image models use native format
	channelType := relayInfo.ChannelMeta.ChannelType
	isGeminiChannel := channelType == constant.ChannelTypeGemini || channelType == constant.ChannelTypeVertexAi
	isGeminiImageModel := strings.HasPrefix(req.Model, "nano-banana") ||
		req.Model == "gemini-3-pro-image" ||
		req.Model == "gemini-3.1-flash-image-preview"
	useGeminiNative := isGeminiChannel && isGeminiImageModel
	var nativeReq map[string]interface{}

	if useGeminiNative {
		// Convert OpenAI-format request to Gemini native format
		var convertErr error
		nativeReq, convertErr = service.ConvertAsyncImageToGeminiNative(context.Background(), &req)
		if convertErr != nil {
			// Refund on conversion failure
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("转换请求格式失败: %v", convertErr),
					"type":    "invalid_request_error",
				},
			})
			return
		}
		task.Action = "generateContent"
		service.SetGenerateContentRequestOmittedData(task, "submitted")
	} else {
		task.SetData(req)
	}

	if err := task.Insert(); err != nil {
		// Refund pre-consumed quota on insert failure
		if relayInfo != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("创建任务失败: %v", err),
				"type":    "internal_error",
			},
		})
		return
	}

	if useGeminiNative {
		service.RecordAsyncImageSubmitLog(c, task, &req, relayInfo)
		_ = task.Update()

		ctx := context.WithValue(context.Background(), "gin_context", c)
		gopool.Go(func() {
			service.ProcessUnifiedImageTask(ctx, task, nativeReq)
		})
	} else {
		// Record usage log at submission time and persist the log ID for later update
		service.RecordAsyncImageSubmitLog(c, task, &req, relayInfo)
		_ = task.Update()

		ctx := context.WithValue(context.Background(), "gin_context", c)
		gopool.Go(func() {
			service.ProcessAsyncImageTask(ctx, task)
		})
	}

	c.JSON(http.StatusOK, dto.AsyncTaskResponse{
		TaskID: task.TaskID,
		Status: string(task.Status),
	})
}

func AsyncGeminiSubmit(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("请求参数错误: %v", err),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	userId := c.GetInt("id")
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	channelId := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	tokenId := c.GetInt("token_id")

	path := c.Param("path")
	modelName := ""
	if len(path) > 1 {
		parts := strings.Split(path[1:], ":")
		if len(parts) > 0 {
			modelName = parts[0]
		}
	}
	if modelName == "" {
		modelName = fmt.Sprintf("%v", req["model"])
	}

	// Build relay info for billing
	relayInfo, priceData, billingErr := prepareAsyncBilling(c, userId, group, channelId, tokenId, modelName)
	if billingErr != nil {
		c.JSON(billingErr.StatusCode, gin.H{
			"error": gin.H{
				"message": billingErr.Error(),
				"type":    "billing_error",
			},
		})
		return
	}

	task := &model.Task{
		TaskID:     model.GenerateTaskID(),
		UserId:     userId,
		Group:      group,
		ChannelId:  channelId,
		Platform:   constant.TaskPlatformAsyncImage,
		Action:     "generateContent",
		Status:     model.TaskStatusSubmitted,
		Progress:   "0%",
		SubmitTime: time.Now().Unix(),
		Quota:      priceData.Quota,
		Properties: model.Properties{
			OriginModelName: modelName,
			TokenId:         tokenId,
			RequestHost:     c.Request.Host,
		},
	}
	service.SetGenerateContentRequestOmittedData(task, "submitted")

	// Store billing context for later refund/settlement
	// tiered_expr 模型即使预扣为 0（信任用户）也必须存 BillingContext，否则完成时无法结算
	if relayInfo != nil && (priceData.Quota > 0 || c.GetBool("tiered_billing_active")) {
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = tokenId
		bc := &model.TaskBillingContext{
			ModelPrice:      priceData.ModelPrice,
			GroupRatio:      priceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      priceData.ModelRatio,
			OtherRatios:     priceData.OtherRatios,
			OriginModelName: modelName,
			PerCallBilling:  priceData.UsePrice,
		}
		if snapBytes, ok := c.Get("tiered_snapshot_bytes"); ok {
			if b, ok := snapBytes.([]byte); ok {
				bc.TieredSnapshot = b
			}
		}
		task.PrivateData.BillingContext = bc
	}

	if err := task.Insert(); err != nil {
		// Refund pre-consumed quota on insert failure
		if relayInfo != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("创建任务失败: %v", err),
				"type":    "internal_error",
			},
		})
		return
	}

	// Record usage log at submission time and persist the log ID for later update
	service.RecordAsyncGeminiSubmitLog(c, task, modelName, relayInfo)
	_ = task.Update()

	ctx := context.WithValue(context.Background(), "gin_context", c)
	gopool.Go(func() {
		service.ProcessAsyncGeminiTask(ctx, task, req)
	})

	c.JSON(http.StatusOK, dto.AsyncTaskResponse{
		TaskID: task.TaskID,
		Status: string(task.Status),
	})
}

// prepareAsyncBilling builds a RelayInfo, calculates price, and pre-consumes quota.
// Returns the relayInfo (with billing session attached) and priceData.
// Returns nil relayInfo and a NewAPIError if billing fails.
//
// 契约：tiered_expr 模型的 BillingSnapshot 经 c.Set("tiered_snapshot_bytes") 传出，
// 每个调用本函数的异步提交入口在创建 Task 时都必须把它写入
// TaskBillingContext.TieredSnapshot（写法见 AsyncImageSubmit / AsyncGeminiSubmit /
// GenerateImageSubmit）。漏写不会报错：提交日志仍显示 billing_mode=tiered_expr
// （日志读的是本处快照），但完成侧结算读的是任务里的快照，读不到就静默跳过
// 表达式重算，扣费永远停在预扣值。
func prepareAsyncBilling(c *gin.Context, userId int, group string, channelId int, tokenId int, modelName string, requestInput ...billingexpr.RequestInput) (*relaycommon.RelayInfo, types.PriceData, *types.NewAPIError) {
	channel, err := model.CacheGetChannel(channelId)
	if err != nil {
		return nil, types.PriceData{}, types.NewError(
			fmt.Errorf("获取渠道信息失败: %v", err),
			types.ErrorCodeQueryDataError,
			types.ErrOptionWithSkipRetry(),
		)
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)

	upstreamModelName := service.ApplyModelMapping(modelName, channel.ModelMapping)
	isModelMapped := upstreamModelName != modelName

	relayInfo := &relaycommon.RelayInfo{
		UserId:          userId,
		UserGroup:       common.GetContextKeyString(c, constant.ContextKeyUserGroup),
		UsingGroup:      group,
		OriginModelName: modelName,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          channel.Type,
			ChannelId:            channel.Id,
			ApiType:              apiType,
			UpstreamModelName:    upstreamModelName,
			IsModelMapped:        isModelMapped,
			ChannelOtherSettings: channel.GetOtherSettings(),
		},
		TokenId:        tokenId,
		TokenKey:       common.GetContextKeyString(c, constant.ContextKeyTokenKey),
		TokenUnlimited: common.GetContextKeyBool(c, constant.ContextKeyTokenUnlimited),
	}

	// Populate user settings for billing preference
	if userSetting, ok := common.GetContextKeyType[dto.UserSetting](c, constant.ContextKeyUserSetting); ok {
		relayInfo.UserSetting = userSetting
	}

	if service.CalculatePriceFunc == nil {
		return nil, types.PriceData{}, types.NewError(
			fmt.Errorf("计费函数未初始化"),
			types.ErrorCodeInvalidApiType,
			types.ErrOptionWithSkipRetry(),
		)
	}

	// tiered_expr 模型走独立的分级表达式预扣费路径，不经过 CalculatePriceFunc（它不认 tiered）。
	if billing_setting.GetBillingMode(modelName) == billing_setting.BillingModeTieredExpr {
		groupRatioInfo := helper.HandleGroupRatio(c, relayInfo)
		exprStr, ok := billing_setting.GetBillingExpr(modelName)
		if !ok {
			return nil, types.PriceData{}, types.NewError(
				fmt.Errorf("模型 %s 配置为 tiered_expr 但缺少计费表达式", modelName),
				types.ErrorCodeModelPriceError,
				types.ErrOptionWithStatusCode(http.StatusBadRequest),
			)
		}
		// 预估 token（异步提交时无法精确知道，用保守估算）
		estimatedPrompt := 1000
		if common.PreConsumedQuota > 0 {
			estimatedPrompt = common.PreConsumedQuota
		}
		billingInput := billingexpr.RequestInput{}
		if len(requestInput) > 0 {
			billingInput = requestInput[0]
		}
		rawCost, trace, err := billingexpr.RunExprWithRequest(exprStr, billingexpr.TokenParams{
			P:   float64(estimatedPrompt),
			C:   0,
			Len: float64(estimatedPrompt),
		}, billingInput)
		if err != nil {
			return nil, types.PriceData{}, types.NewError(
				fmt.Errorf("tiered_expr 计算失败: %v", err),
				types.ErrorCodeModelPriceError,
				types.ErrOptionWithStatusCode(http.StatusBadRequest),
			)
		}
		quotaBeforeGroup := rawCost / 1_000_000 * common.QuotaPerUnit
		preConsumedQuota := billingexpr.QuotaRound(quotaBeforeGroup * groupRatioInfo.GroupRatio)

		snapshot := &billingexpr.BillingSnapshot{
			BillingMode:               billing_setting.BillingModeTieredExpr,
			ModelName:                 modelName,
			ExprString:                exprStr,
			ExprHash:                  billingexpr.ExprHashString(exprStr),
			GroupRatio:                groupRatioInfo.GroupRatio,
			EstimatedPromptTokens:     estimatedPrompt,
			EstimatedCompletionTokens: 0,
			EstimatedQuotaBeforeGroup: quotaBeforeGroup,
			EstimatedQuotaAfterGroup:  preConsumedQuota,
			EstimatedTier:             trace.MatchedTier,
			QuotaPerUnit:              common.QuotaPerUnit,
			ExprVersion:               billingexpr.ExprVersion(exprStr),
		}
		snapshotBytes, _ := common.Marshal(snapshot)

		priceData := types.PriceData{
			GroupRatioInfo: groupRatioInfo,
			Quota:          preConsumedQuota,
		}
		relayInfo.PriceData = priceData
		relayInfo.TieredBillingSnapshot = snapshot

		// 预扣费
		relayInfo.ForcePreConsume = true
		if preConsumedQuota > 0 {
			if apiErr := service.PreConsumeBilling(c, preConsumedQuota, relayInfo); apiErr != nil {
				return nil, priceData, apiErr
			}
		}

		// 把快照存入 relayInfo 的临时字段，供调用方写入 TaskBillingContext
		c.Set("tiered_snapshot_bytes", snapshotBytes)
		c.Set("tiered_billing_active", true)
		return relayInfo, priceData, nil
	}

	priceData, err := service.CalculatePriceFunc(c, relayInfo)
	if err != nil {
		return nil, types.PriceData{}, types.NewError(
			fmt.Errorf("计算价格失败: %v", err),
			types.ErrorCodeModelPriceError,
			types.ErrOptionWithStatusCode(http.StatusBadRequest),
		)
	}
	relayInfo.PriceData = priceData

	// Pre-consume billing
	// 异步任务必须强制预扣，禁用信任额度旁路，确保余额实际扣除
	relayInfo.ForcePreConsume = true
	if !priceData.FreeModel && priceData.Quota > 0 {
		if apiErr := service.PreConsumeBilling(c, priceData.Quota, relayInfo); apiErr != nil {
			return nil, priceData, apiErr
		}
	}

	return relayInfo, priceData, nil
}

func AsyncTaskFetch(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		taskID = c.Query("task_id")
	}

	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "缺少 task_id 参数",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	userId := c.GetInt("id")
	task, exists, err := model.GetByTaskId(userId, taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("查询任务失败: %v", err),
				"type":    "internal_error",
			},
		})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "任务不存在",
				"type":    "not_found_error",
			},
		})
		return
	}

	resp := dto.AsyncTaskFetchResponse{
		TaskID:   task.TaskID,
		Status:   string(task.Status),
		Progress: task.Progress,
	}

	if task.Status == model.TaskStatusSuccess {
		if task.Platform == constant.TaskPlatformGenerateImage {
			// 统一生图：完成时已存入 GenerateImageResult，原样返回
			resp.Data = task.Data
		} else if task.Platform == constant.TaskPlatformUnifiedImage {
			// Return standard OpenAI ImageResponse format
			imageResp := dto.ImageResponse{Created: task.FinishTime}
			var storedData map[string]interface{}
			if err := task.GetData(&storedData); err == nil {
				if urls, ok := storedData["urls"].([]interface{}); ok {
					for _, u := range urls {
						if urlStr, ok := u.(string); ok {
							imageResp.Data = append(imageResp.Data, dto.ImageData{Url: urlStr})
						}
					}
				} else if dataList, ok := storedData["data"].([]interface{}); ok {
					for _, d := range dataList {
						if b64Str, ok := d.(string); ok {
							imageResp.Data = append(imageResp.Data, dto.ImageData{B64Json: b64Str})
						}
					}
				}
			}
			// Fallback: single image via legacy ResultURL
			if len(imageResp.Data) == 0 && task.PrivateData.ResultURL != "" {
				imageResp.Data = append(imageResp.Data, dto.ImageData{Url: task.PrivateData.ResultURL})
			}
			if len(imageResp.Data) > 0 {
				dataBytes, _ := common.Marshal(imageResp)
				resp.Data = dataBytes
			} else {
				resp.Data = task.Data
			}
		} else if task.Platform == constant.TaskPlatformAsyncImage && task.PrivateData.ResultURL != "" {
			// Legacy format for old async_image tasks
			data := map[string]interface{}{
				"image_url": task.PrivateData.ResultURL,
			}
			dataBytes, _ := common.Marshal(data)
			resp.Data = dataBytes
		} else {
			resp.Data = task.Data
		}
	} else if task.Status == model.TaskStatusFailure {
		// 失败原因原样透传上游信息，不做友好化改写
		resp.Error = task.FailReason
	}

	responseBytes, err := common.Marshal(resp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("序列化任务响应失败: %v", err),
				"type":    "internal_error",
			},
		})
		return
	}
	c.Header("Content-Type", "application/json; charset=utf-8")
	service.IOCopyBytesGracefully(c, nil, responseBytes)
}

// Deprecated: use AsyncTaskFetch
func AsyncImageFetch(c *gin.Context) {
	AsyncTaskFetch(c)
}
