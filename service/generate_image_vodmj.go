package service

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/tencentvod"
	"github.com/gin-gonic/gin"
)

// Tencent VOD Midjourney polling knobs. The upstream task takes 60-100 seconds
// in practice, so the first polls land while it is still running.
const (
	vodMJPollInterval = 8 * time.Second
	vodMJPollJitter   = 2 * time.Second
	vodMJPollTimeout  = 10 * time.Minute
	// vodMJFileInfoMaxBytes mirrors the documented Base64 input cap: all
	// reference files together must stay under 7 MB.
	vodMJFileInfoMaxBytes = 7 << 20
)

// vodMJClientFor builds a VOD client for a task's channel. The channel key
// holds "SecretId|SecretKey", matching the existing Tencent video channel.
func vodMJClientFor(task *model.Task) (*tencentvod.Client, int64, error) {
	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		return nil, 0, fmt.Errorf("获取渠道信息失败: %v", err)
	}
	cred, err := tencentvod.ParseCredential(channel.Key)
	if err != nil {
		return nil, 0, err
	}
	subAppID := vodMJSubAppID(channel)
	if subAppID <= 0 {
		return nil, 0, fmt.Errorf("渠道未配置 SubAppId，请在渠道设置中填写 VOD 应用 ID")
	}
	proxy := channel.GetSetting().Proxy
	httpClient, err := GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, 0, fmt.Errorf("创建 HTTP 客户端失败: %v", err)
	}
	return tencentvod.NewClient(cred, httpClient), subAppID, nil
}

// vodMJSubAppID reads the VOD application id from the channel's other
// settings. It is not a secret, so it lives beside the other channel settings
// rather than in the key field.
func vodMJSubAppID(channel *model.Channel) int64 {
	settings := channel.GetOtherSettings()
	if settings.TencentVodSubAppID == nil {
		return 0
	}
	return *settings.TencentVodSubAppID
}

// processGenerateImageVodMJ runs one Midjourney task: submit, poll until
// terminal, then store the images through the channel's output strategy and
// settle billing. Billing is per task, not per image: one task always returns
// four images for a single charge, and the tier (1k or 2k) is decided by the
// "--hd" prompt flag, which only the upstream result can confirm.
func processGenerateImageVodMJ(ctx context.Context, c *gin.Context, task *model.Task, asyncReq *dto.AsyncImageRequest) {
	if asyncReq == nil {
		if err := task.GetData(&asyncReq); err != nil {
			failGenerateImageTask(task, fmt.Sprintf("解析任务数据失败: %v", err))
			return
		}
	}

	client, subAppID, err := vodMJClientFor(task)
	if err != nil {
		failGenerateImageTask(task, err.Error())
		return
	}

	// A resumed task already holds an upstream id; re-submitting would bill a
	// second generation, so only submit when there is none.
	upstreamTaskID := task.PrivateData.UpstreamTaskID
	if upstreamTaskID == "" {
		upstreamTaskID, err = submitVodMJTask(ctx, client, subAppID, task, asyncReq)
		if err != nil {
			failGenerateImageTask(task, fmt.Sprintf("提交上游任务失败: %v", err))
			return
		}
		// Persist before polling: a crash between submit and poll must not lose
		// the id, or the task becomes unqueryable while still billed upstream.
		task.PrivateData.UpstreamTaskID = upstreamTaskID
		if err := task.Update(); err != nil {
			logger.LogError(ctx, fmt.Sprintf("generate_image(vodmj): 保存上游任务 ID 失败 task=%s: %v", task.TaskID, err))
		}
	} else {
		logger.LogInfo(ctx, fmt.Sprintf("generate_image(vodmj): 恢复轮询 task=%s upstream=%s", task.TaskID, upstreamTaskID))
	}

	detail, err := waitVodMJTask(ctx, client, subAppID, task, upstreamTaskID)
	if err != nil {
		failGenerateImageTask(task, err.Error())
		return
	}

	images := vodMJImages(detail)
	if len(images) == 0 {
		failGenerateImageTask(task, "上游任务完成但未返回图片")
		return
	}

	relayInfo, relayErr := buildGenerateImageRelayInfo(c, task, 0)
	if relayErr != nil {
		failGenerateImageTask(task, relayErr.Error())
		return
	}

	outputStrategy := generateImageOutputStrategy(task, relayInfo.ChannelOtherSettings.ImageOutputStrategy)
	stored, outputTiming, storeErr := prepareGenerateImageResultsWithStrategyTiming(ctx, images, task.Properties.RequestHost, outputStrategy)
	if storeErr != nil {
		failGenerateImageTask(task, fmt.Sprintf("上传图片到对象存储失败: %v", storeErr))
		return
	}
	if audit := task.PrivateData.GenerateImageTiming; audit != nil {
		outputTiming.applyToAudit(audit)
	}

	logger.LogInfo(ctx, fmt.Sprintf(
		"generate_image(vodmj): task=%s upstream=%s images=%d tier=%s strategy=%s spec=%s",
		task.TaskID, upstreamTaskID, len(stored), detail.AigcImageTask.Output.Usage.ResolutionTier(),
		outputStrategy, vodMJSpecification(detail),
	))

	// Token usage does not apply to a per-task MJ charge. The image count is
	// recorded as a fact so the log shows four images while the quota stays a
	// single charge.
	tokenDetails := map[string]interface{}{
		"generated_image_count": len(stored),
	}
	if spec := vodMJSpecification(detail); spec != "" {
		tokenDetails["upstream_specification"] = spec
	}
	if tier := detail.AigcImageTask.Output.Usage.ResolutionTier(); tier != "" {
		tokenDetails["upstream_resolution_tier"] = tier
	}

	finalizeGenerateImageTask(ctx, task, stored, 0, 0, tokenDetails,
		relayInfo.UpstreamModelName, relayInfo.IsModelMapped, tencentvod.ModelVersionV82)
}

// submitVodMJTask creates the upstream task and returns its id.
func submitVodMJTask(ctx context.Context, client *tencentvod.Client, subAppID int64, task *model.Task, asyncReq *dto.AsyncImageRequest) (string, error) {
	files, err := vodMJReferenceFiles(ctx, asyncReq)
	if err != nil {
		return "", err
	}

	// SessionId ties a gateway task to one upstream task. Reusing it for the
	// same task makes an accidental resubmission fail upstream instead of
	// billing twice, while a retry on another channel still gets its own id.
	resp, err := client.CreateImageTask(ctx, tencentvod.CreateImageTaskRequest{
		SubAppID:     subAppID,
		ModelName:    tencentvod.ModelNameMJ,
		ModelVersion: vodMJModelVersion(asyncReq.Model),
		FileInfos:    files,
		Prompt:       asyncReq.Prompt,
		OutputConfig: &tencentvod.ImageOutputConfig{
			// Temporary keeps the documented, verified storage mode. The
			// gateway archives results through its own output strategy.
			StorageMode: tencentvod.StorageModeTemporary,
		},
		SessionID: vodMJSessionID(task),
	})
	if err != nil {
		return "", err
	}
	return resp.TaskID, nil
}

// vodMJModelVersion maps the gateway model name to the VOD ModelVersion value.
// Only v8.2 is exposed today.
func vodMJModelVersion(modelName string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	if suffix, found := strings.CutPrefix(normalized, "mj-"); found {
		// "mj-v8.2" -> "v8.2"; any other suffix is passed through unchanged so
		// an operator can enable a future version from the channel alone.
		return suffix
	}
	return tencentvod.ModelVersionV82
}

// vodMJSessionID derives a stable, bounded deduplication key. VOD rejects a
// repeated SessionId within three days and caps the field at 50 characters.
func vodMJSessionID(task *model.Task) string {
	const prefix = "mj-"
	const limit = 50
	id := strings.TrimSpace(task.TaskID)
	if id == "" {
		id = model.GenerateTaskID()
	}
	max := limit - len(prefix)
	if len(id) > max {
		id = id[:max]
	}
	return prefix + id
}

// vodMJReferenceFiles converts the request's reference image into the Base64
// form the gateway endpoint accepts. MJ takes a single reference image.
func vodMJReferenceFiles(ctx context.Context, asyncReq *dto.AsyncImageRequest) ([]tencentvod.InputFileInfo, error) {
	if len(asyncReq.Images) > 1 {
		return nil, fmt.Errorf("MJ 模型首期只支持 1 张参考图，收到 %d 张", len(asyncReq.Images))
	}

	var reference string
	if len(asyncReq.Images) == 1 {
		reference = strings.TrimSpace(asyncReq.Images[0])
	} else if len(asyncReq.Image) > 0 {
		var value string
		if err := common.Unmarshal(asyncReq.Image, &value); err == nil {
			reference = strings.TrimSpace(value)
		}
	}
	if reference == "" {
		return nil, nil
	}

	// The API takes raw Base64 without a data URL prefix, so a data URI has to
	// be unwrapped before it is sent.
	var mimeType, encoded string
	if strings.HasPrefix(reference, "data:") {
		header, payload, found := strings.Cut(strings.TrimPrefix(reference, "data:"), ",")
		if !found {
			return nil, fmt.Errorf("参考图 data URI 格式无效")
		}
		mimeType = strings.TrimSuffix(header, ";base64")
		encoded = payload
	} else if strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://") {
		fetchedMime, fetched, err := GetImageFromUrlWithLimit(reference, AsyncImageMaxURLSizeMB)
		if err != nil {
			return nil, fmt.Errorf("下载参考图失败: %v", err)
		}
		mimeType, encoded = fetchedMime, fetched
	} else {
		return nil, fmt.Errorf("参考图必须是可以访问的 http(s) URL 或 Base64 数据")
	}

	decodedSize := len(encoded) / 4 * 3
	if decodedSize > vodMJFileInfoMaxBytes {
		return nil, fmt.Errorf("参考图超过 %d MiB 上限", vodMJFileInfoMaxBytes/(1<<20))
	}
	if mimeType != "" && !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return nil, fmt.Errorf("参考图类型 %s 不受支持", mimeType)
	}
	return []tencentvod.InputFileInfo{{Type: "Base64", Base64: encoded}}, nil
}

// waitVodMJTask polls until the task is terminal. A timeout returns an error
// rather than a failure result, because the upstream task may still succeed and
// the caller keeps the upstream id for a later resume.
func waitVodMJTask(ctx context.Context, client *tencentvod.Client, subAppID int64, task *model.Task, upstreamTaskID string) (tencentvod.TaskDetailResponse, error) {
	deadline := time.Now().Add(vodMJPollTimeout)
	var last tencentvod.TaskDetailResponse

	for {
		detail, err := client.DescribeTask(ctx, subAppID, upstreamTaskID)
		if err != nil {
			return last, fmt.Errorf("查询上游任务失败: %v", err)
		}
		last = detail
		inner := detail.AigcImageTask

		// Progress is coarse (0 until it jumps to 100), so a running task
		// stays at 50% until the provider reports a terminal state.
		if inner.Progress > 0 && inner.Progress < 100 {
			task.Progress = fmt.Sprintf("%d%%", inner.Progress)
		}

		if inner.Terminal() {
			if !inner.Succeeded() {
				return last, fmt.Errorf("上游生成失败: %s", inner.FailureReason())
			}
			return detail, nil
		}

		if time.Now().After(deadline) {
			return last, fmt.Errorf("上游任务超时未完成（%s），上游任务 ID %s 已保留", vodMJPollTimeout, upstreamTaskID)
		}

		select {
		case <-ctx.Done():
			return last, fmt.Errorf("任务被取消: %v", ctx.Err())
		case <-time.After(vodMJPollInterval + time.Duration(rand.Int63n(int64(vodMJPollJitter)))):
		}
	}
}

// vodMJImages converts the upstream file list into gateway image entries. The
// count is whatever the provider returned, never a hardcoded four.
func vodMJImages(detail tencentvod.TaskDetailResponse) []dto.GenerateImageData {
	files := detail.AigcImageTask.Output.FileInfos
	images := make([]dto.GenerateImageData, 0, len(files))
	for _, file := range files {
		url := strings.TrimSpace(file.FileURL)
		if url == "" {
			continue
		}
		image := dto.GenerateImageData{Url: url}
		if file.MetaData != nil && file.MetaData.Width > 0 && file.MetaData.Height > 0 {
			image.Size = fmt.Sprintf("%dx%d", file.MetaData.Width, file.MetaData.Height)
		}
		// FileType is empty for MJ, so the format comes from the URL.
		image.OutputFormat = vodMJOutputFormat(file)
		images = append(images, image)
	}
	return images
}

// vodMJOutputFormat resolves the result format. MJ leaves FileType empty, so
// the URL extension is the reliable source, with Container as a fallback.
func vodMJOutputFormat(file tencentvod.OutputFileInfo) string {
	if ext := strings.TrimSpace(file.FileType); ext != "" {
		return strings.ToLower(ext)
	}
	if path, _, found := strings.Cut(file.FileURL, "?"); found {
		file.FileURL = path
	}
	if idx := strings.LastIndex(file.FileURL, "."); idx >= 0 && idx < len(file.FileURL)-1 {
		return strings.ToLower(file.FileURL[idx+1:])
	}
	if file.MetaData != nil {
		if container := strings.TrimSpace(file.MetaData.Container); container != "" {
			return strings.TrimSuffix(strings.ToLower(container), "_pipe")
		}
	}
	return ""
}

func vodMJSpecification(detail tencentvod.TaskDetailResponse) string {
	usages := detail.AigcImageTask.Output.Usage.SpecificationUsages
	if len(usages) == 0 {
		return ""
	}
	return usages[0].Specification
}

// validateGenerateImageVodMJ applies the Midjourney contract. MJ has no count
// parameter and no size parameter: every task returns four images and the
// resolution tier is chosen by the prompt's "--hd" flag, so a client-supplied
// value would only be ignored upstream while still affecting billing.
func validateGenerateImageVodMJ(req *dto.GenerateImageRequest) error {
	if req.N != nil {
		return fmt.Errorf("MJ 模型不支持 n 参数：每次任务固定生成 4 张图，请不要传递 n")
	}
	if strings.TrimSpace(req.Size) != "" && !strings.EqualFold(strings.TrimSpace(req.Size), "auto") {
		return fmt.Errorf("MJ 模型不支持 size 参数：请在 prompt 中用 --ar 控制比例，2K 用 --hd 开启")
	}
	if req.Quality != "" {
		return fmt.Errorf("MJ 模型不支持 quality 参数：请在 prompt 中用 --q 控制画质")
	}
	if aspectRatio := strings.TrimSpace(req.AspectRatio); aspectRatio != "" {
		return fmt.Errorf("MJ 模型不支持 aspect_ratio 参数：请在 prompt 中用 --ar 控制比例")
	}
	if mediaResolution := strings.TrimSpace(req.MediaResolution); mediaResolution != "" {
		return fmt.Errorf("MJ 模型不支持 media_resolution 参数")
	}
	if len(req.ResponseModalities) > 0 {
		return fmt.Errorf("MJ 模型不支持 response_modalities 参数")
	}
	if req.LayerDecomposition != nil {
		return fmt.Errorf("MJ 模型不支持 layer_decomposition 参数")
	}
	if req.GoogleSearch != nil || req.IncludeThoughts != nil {
		return fmt.Errorf("MJ 模型不支持 google_search / include_thoughts 参数")
	}
	// Fixed order, so the reported parameter is deterministic when a caller
	// sends several unsupported fields at once.
	for _, field := range []struct {
		name string
		set  bool
	}{
		{name: "output_format", set: req.OutputFormat != nil},
		{name: "background", set: req.Background != nil},
		{name: "moderation", set: req.Moderation != nil},
		{name: "thinking_level", set: req.ThinkingLevel != nil},
		{name: "markup", set: req.Markup != nil},
		{name: "mask", set: req.Mask != nil},
		{name: "complexity", set: req.Complexity != nil},
		{name: "frame_count", set: req.FrameCount != nil},
		{name: "frame_duration_ms", set: req.FrameDurationMs != nil},
		{name: "partial_images", set: req.PartialImages != nil},
		{name: "output_compression", set: req.OutputCompression != nil},
		{name: "input_fidelity", set: req.InputFidelity != nil},
		{name: "watermark", set: req.Watermark != nil},
	} {
		if field.set {
			return fmt.Errorf("MJ 模型不支持 %s 参数，请通过 prompt 中的 MJ 参数控制", field.name)
		}
	}
	if len(req.Images) > 1 {
		return fmt.Errorf("MJ 模型首期只支持 1 张参考图，收到 %d 张", len(req.Images))
	}
	return nil
}


func resumeVodMJTasks(ctx context.Context, limit int) {
	tasks := model.GetUnfinishedTasksByAction("vodMJGenerate", limit)
	resumed := 0
	for _, task := range tasks {
		// A task without an upstream id never reached submit, so there is
		// nothing to resume; the timeout sweep owns it.
		if task.PrivateData.UpstreamTaskID == "" {
			continue
		}
		resumed++
		taskContext := context.WithValue(ctx, "gin_context", newAsyncGinContext(task))
		go func(t *model.Task) {
			ProcessGenerateImageTask(taskContext, t)
		}(task)
	}
	if resumed > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("generate_image(vodmj): 恢复 %d 个未完成任务", resumed))
	}
}
