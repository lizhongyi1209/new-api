package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
	"golang.org/x/sync/errgroup"
)

// imageProvider 标识一个生图模型应走哪条处理路径。
type imageProvider string

const (
	imageProviderGeminiNative imageProvider = "gemini_native"
	imageProviderOpenAIImage  imageProvider = "openai_image"
)

var uploadGenerateImageBase64 = UploadBase64ImageToHostStorageCompressed

const imageRetryActionResubmit = "resubmit"

const (
	nanoBananaGenerateContentMaxBodyBytes = 20 * 1024 * 1024
	nanoBananaResizeMaxAttempts           = 8
)

// imageRoute 是一条分发规则：模型名匹配 match 时走对应 provider。
// provider 与 action 一起决定提交时如何构造任务、处理时走哪个分支。
// 扩展新 provider 只需在 imageRoutes 增加一条规则，并在 ProcessGenerateImageTask 增加一个 case。
type imageRoute struct {
	match    func(modelName string) bool
	provider imageProvider
	action   string
}

// imageRoutes 是统一生图端点的分发注册表，按顺序匹配，第一条命中即生效。
var imageRoutes = []imageRoute{
	{match: isGeminiImageModelName, provider: imageProviderGeminiNative, action: "generateContent"},
}

// isGeminiImageModelName 判定模型是否走 Gemini 原生生图路径。
// 规则与 controller/async_image.go 中既有的 isGeminiImageModel 保持一致。
//
// 客户端参数规则（nano-banana* / Gemini image preview 模型）：
//   - image_compression 不需要；
//   - response_modalities 可选，默认不传；
//   - media_resolution 可选，透传到 generationConfig.mediaResolution；
//   - 内嵌图片字节会与文本提示、系统指令一起计入 20MB 总请求大小；
//   - n 用不到，默认不传，传入视为废弃；
//   - google_search 可选，只有 true 时启用 Google Search grounding；
//   - 不需要图片预签名上传。
func isGeminiImageModelName(modelName string) bool {
	return isNanoBananaModelName(modelName) ||
		modelName == "gemini-3-pro-image" ||
		modelName == "gemini-3-pro-image-preview" ||
		modelName == "gemini-3.1-flash-image-preview"
}

func isNanoBananaModelName(modelName string) bool {
	return strings.HasPrefix(modelName, "nano-banana")
}

// resolveImageRoute 根据模型名 + 渠道类型选出处理路径。
// Gemini 原生路径要求渠道本身是 Gemini/Vertex；否则即使模型名匹配也回退到通用 OpenAI image 路径。
func resolveImageRoute(modelName string, channelType int) imageRoute {
	isGeminiChannel := channelType == constant.ChannelTypeGemini || channelType == constant.ChannelTypeVertexAi
	for _, r := range imageRoutes {
		if !r.match(modelName) {
			continue
		}
		if r.provider == imageProviderGeminiNative && !isGeminiChannel {
			continue
		}
		return r
	}
	// 兜底：通用 OpenAI image 适配器路径。
	return imageRoute{provider: imageProviderOpenAIImage, action: "generate"}
}

// ResolveImageRoute 是 resolveImageRoute 的导出包装，供控制器在提交阶段决定任务构造方式。
// 返回 (action, isGeminiNative)。
func ResolveImageRoute(modelName string, channelType int) (action string, isGeminiNative bool) {
	r := resolveImageRoute(modelName, channelType)
	return r.action, r.provider == imageProviderGeminiNative
}

const generateImageMaskImageURLMaxLength = 20971520

var generateImageQualityValues = map[string]struct{}{
	"low":      {},
	"medium":   {},
	"high":     {},
	"auto":     {},
	"standard": {},
	"hd":       {},
}

var generateImageOutputFormatValues = map[string]struct{}{
	"png":  {},
	"jpeg": {},
	"webp": {},
}

// ValidateGenerateImageRequest validates the unified /async/v1/generateImage request.
func ValidateGenerateImageRequest(req *dto.GenerateImageRequest) error {
	if strings.TrimSpace(req.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}

	req.Size = strings.TrimSpace(req.Size)
	if strings.EqualFold(req.Size, "auto") {
		req.Size = "auto"
	}
	if strings.Contains(req.Size, "×") {
		return fmt.Errorf("size an unexpected error occurred in the parameter, please use 'x' instead of the multiplication sign '×'")
	}

	req.Quality = strings.ToLower(strings.TrimSpace(req.Quality))
	if req.Quality != "" {
		if _, ok := generateImageQualityValues[req.Quality]; !ok {
			return fmt.Errorf("quality must be one of low, medium, high, auto")
		}
	}

	if req.OutputFormat != nil {
		outputFormat := strings.ToLower(strings.TrimSpace(*req.OutputFormat))
		if _, ok := generateImageOutputFormatValues[outputFormat]; !ok {
			return fmt.Errorf("output_format must be one of png, jpeg, webp")
		}
		*req.OutputFormat = outputFormat
	}

	req.MediaResolution = strings.TrimSpace(req.MediaResolution)

	if req.ThinkingLevel != nil {
		thinkingLevel := strings.TrimSpace(*req.ThinkingLevel)
		if thinkingLevel == "" {
			return fmt.Errorf("thinking_level must not be empty")
		}
		*req.ThinkingLevel = thinkingLevel
	}

	if err := validateGenerateImageMask(req.Mask); err != nil {
		return err
	}
	return nil
}

func validateGenerateImageMask(mask *dto.ImageReference) error {
	if mask == nil {
		return nil
	}

	if (mask.FileID != nil) == (mask.ImageURL != nil) {
		return fmt.Errorf("mask must provide exactly one of file_id or image_url")
	}

	if mask.FileID != nil {
		fileID := strings.TrimSpace(*mask.FileID)
		if fileID == "" {
			return fmt.Errorf("mask.file_id is required")
		}
		*mask.FileID = fileID
		return nil
	}

	imageURL := strings.TrimSpace(*mask.ImageURL)
	if imageURL == "" {
		return fmt.Errorf("mask.image_url is required")
	}
	if len(imageURL) > generateImageMaskImageURLMaxLength {
		return fmt.Errorf("mask.image_url length must be <= %d", generateImageMaskImageURLMaxLength)
	}
	if !isGenerateImageURLReference(imageURL) {
		return fmt.Errorf("mask.image_url must be a fully qualified http(s) URL or base64 data URL")
	}
	*mask.ImageURL = imageURL
	return nil
}

func isGenerateImageURLReference(value string) bool {
	lowerValue := strings.ToLower(value)
	if strings.HasPrefix(lowerValue, "data:") {
		return strings.Contains(lowerValue, ";base64,")
	}
	parsedURL, err := url.Parse(value)
	if err != nil {
		return false
	}
	return (strings.EqualFold(parsedURL.Scheme, "http") || strings.EqualFold(parsedURL.Scheme, "https")) && parsedURL.Host != ""
}

// SetGenerateContentRequestOmittedData stores a small placeholder instead of
// persisting generateContent request bodies, which can contain multi-MB inline
// image base64 payloads.
func SetGenerateContentRequestOmittedData(task *model.Task, stage string) {
	if task == nil {
		return
	}
	task.SetData(map[string]interface{}{
		"object":  "generate_content_request",
		"omitted": true,
		"stage":   stage,
		"reason":  "request payload omitted to avoid storing inline image data",
	})
}

type geminiInlineImageRef struct {
	inline   map[string]interface{}
	dataKey  string
	mimeKey  string
	mimeType string
	data     string
}

// fitNanoBananaGenerateContentBody 按上游体积上限对内联图做缩放。
// 注意：自「性能优先，超限直接打回」策略上线后，生产路径不再调用此函数
// （见 ProcessGenerateImageTask），保留它仅供潜在的可配置回退与单测使用。
func fitNanoBananaGenerateContentBody(ctx context.Context, requestBody map[string]interface{}, maxBytes int) ([]byte, bool, error) {
	jsonData, err := common.Marshal(requestBody)
	if err != nil {
		return nil, false, fmt.Errorf("serialize request: %w", err)
	}
	if len(jsonData) <= maxBytes {
		return jsonData, false, nil
	}

	resized := false
	targetBytes := nanoBananaTargetBodyBytes(maxBytes)
	for attempt := 0; attempt < nanoBananaResizeMaxAttempts; attempt++ {
		refs := collectGeminiInlineImageRefs(requestBody)
		if len(refs) == 0 {
			return nil, resized, fmt.Errorf("request body %d bytes exceeds %d bytes and has no inline images to resize", len(jsonData), maxBytes)
		}

		scale := nanoBananaScaleForBodySize(len(jsonData), targetBytes)
		changed := false
		for _, ref := range refs {
			refChanged, err := resizeGeminiInlineImage(ref, scale)
			if err != nil {
				return nil, resized, err
			}
			changed = changed || refChanged
		}
		if !changed {
			return nil, resized, fmt.Errorf("request body %d bytes exceeds %d bytes and inline images cannot be resized further", len(jsonData), maxBytes)
		}
		resized = true

		jsonData, err = common.Marshal(requestBody)
		if err != nil {
			return nil, resized, fmt.Errorf("serialize resized request: %w", err)
		}
		logger.LogInfo(ctx, fmt.Sprintf("generate_image(gemini): nano-banana resize attempt=%d scale=%.4f bodyLen=%d max=%d", attempt+1, scale, len(jsonData), maxBytes))
		if len(jsonData) <= maxBytes {
			return jsonData, true, nil
		}
	}

	return nil, resized, fmt.Errorf("request body remains %d bytes after resizing, max is %d bytes", len(jsonData), maxBytes)
}

func nanoBananaTargetBodyBytes(maxBytes int) int {
	if maxBytes <= 0 {
		return 0
	}
	margin := maxBytes / 50 // 2% safety margin for re-encoding variance.
	if margin < 1024 {
		margin = 1024
	}
	if margin > 128*1024 {
		margin = 128 * 1024
	}
	target := maxBytes - margin
	if target <= 0 {
		return maxBytes
	}
	return target
}

func nanoBananaScaleForBodySize(currentBytes, targetBytes int) float64 {
	if currentBytes <= 0 || targetBytes <= 0 || currentBytes <= targetBytes {
		return 1
	}
	// Body size scales roughly with image pixel area, so sqrt(target/current)
	// estimates the linear scale needed to hit the target in a single pass.
	scale := math.Sqrt(float64(targetBytes) / float64(currentBytes))
	// Enforce a minimum effective step: even when the body only slightly
	// exceeds the limit, shrink by at least 5% per attempt so we converge in
	// 1-2 rounds instead of crawling at 2%/round (each round re-decodes and
	// re-encodes every inline image, which is the dominant CPU cost).
	if scale >= 0.95 {
		return 0.95
	}
	if scale < 0.01 {
		return 0.01
	}
	return scale
}

func collectGeminiInlineImageRefs(value interface{}) []geminiInlineImageRef {
	var refs []geminiInlineImageRef
	var walk func(interface{})
	walk = func(v interface{}) {
		switch typed := v.(type) {
		case map[string]interface{}:
			for _, key := range []string{"inlineData", "inline_data"} {
				inline, ok := typed[key].(map[string]interface{})
				if !ok {
					continue
				}
				if ref, ok := geminiInlineImageRefFromMap(inline); ok {
					refs = append(refs, ref)
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return refs
}

func geminiInlineImageRefFromMap(inline map[string]interface{}) (geminiInlineImageRef, bool) {
	dataKey, data, ok := stringValueForKeys(inline, "data")
	if !ok || strings.TrimSpace(data) == "" {
		return geminiInlineImageRef{}, false
	}
	mimeKey, mimeType, _ := stringValueForKeys(inline, "mimeType", "mime_type")
	if strings.TrimSpace(mimeType) == "" && strings.HasPrefix(strings.TrimSpace(data), "data:") {
		mimeType, _ = parseDataURI(strings.TrimSpace(data))
	}
	if strings.TrimSpace(mimeType) != "" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "image/") {
		return geminiInlineImageRef{}, false
	}
	if mimeKey == "" {
		mimeKey = "mimeType"
	}
	return geminiInlineImageRef{
		inline:   inline,
		dataKey:  dataKey,
		mimeKey:  mimeKey,
		mimeType: strings.TrimSpace(mimeType),
		data:     strings.TrimSpace(data),
	}, true
}

func stringValueForKeys(values map[string]interface{}, keys ...string) (string, string, bool) {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		str, ok := value.(string)
		if !ok {
			continue
		}
		return key, str, true
	}
	return "", "", false
}

func resizeGeminiInlineImage(ref geminiInlineImageRef, scale float64) (bool, error) {
	img, mimeType, err := decodeGeminiInlineImage(ref.mimeType, ref.data)
	if err != nil {
		return false, fmt.Errorf("decode inline image failed: %w", err)
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return false, fmt.Errorf("inline image has invalid dimensions %dx%d", width, height)
	}

	newWidth := int(math.Floor(float64(width) * scale))
	newHeight := int(math.Floor(float64(height) * scale))
	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}
	if newWidth >= width && width > 1 {
		newWidth = width - 1
	}
	if newHeight >= height && height > 1 {
		newHeight = height - 1
	}
	if newWidth == width && newHeight == height {
		return false, nil
	}

	outMimeType, b64Data, err := encodeScaledGeminiInlineImage(img, mimeType, newWidth, newHeight)
	if err != nil {
		return false, err
	}
	ref.inline[ref.dataKey] = b64Data
	ref.inline[ref.mimeKey] = outMimeType
	return true, nil
}

func decodeGeminiInlineImage(mimeType, data string) (image.Image, string, error) {
	if strings.HasPrefix(data, "data:") {
		parsedMimeType, parsedData := parseDataURI(data)
		if parsedMimeType == "" || parsedData == "" {
			return nil, "", fmt.Errorf("invalid data URI")
		}
		mimeType = parsedMimeType
		data = parsedData
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, "", fmt.Errorf("base64 decode failed: %w", err)
	}

	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		if webpImg, webpErr := webp.Decode(bytes.NewReader(raw)); webpErr == nil {
			if strings.TrimSpace(mimeType) == "" {
				mimeType = "image/webp"
			}
			return webpImg, mimeType, nil
		}
		return nil, "", err
	}
	if strings.TrimSpace(mimeType) == "" && format != "" {
		if format == "jpeg" {
			format = "jpg"
		}
		mimeType = "image/" + format
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/png"
	}
	return img, mimeType, nil
}

func encodeScaledGeminiInlineImage(src image.Image, mimeType string, width, height int) (string, string, error) {
	outMimeType := nanoBananaScaledImageMimeType(mimeType)
	dstRect := image.Rect(0, 0, width, height)
	var buf bytes.Buffer

	if outMimeType == "image/png" {
		dst := image.NewNRGBA(dstRect)
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&buf, dst); err != nil {
			return "", "", fmt.Errorf("png encode failed: %w", err)
		}
	} else {
		dst := image.NewRGBA(dstRect)
		xdraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, xdraw.Src)
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 90}); err != nil {
			return "", "", fmt.Errorf("jpeg encode failed: %w", err)
		}
	}

	return outMimeType, base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func nanoBananaScaledImageMimeType(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return "image/png"
	default:
		return "image/jpeg"
	}
}

// failGenerateImageTask 把任务标记为失败并落库。退款由 ProcessGenerateImageTask 的 defer 统一处理。
func failGenerateImageTask(task *model.Task, reason string) {
	task.Status = model.TaskStatusFailure
	task.FailReason = reason
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	if task.Action == "generateContent" {
		SetGenerateContentRequestOmittedData(task, "failure")
	}
	_ = task.Update()
}

func failGenerateImageTaskWithDetail(task *model.Task, reason string, detail *model.TaskErrorDetail) {
	if detail != nil {
		task.PrivateData.ErrorDetail = detail
	}
	failGenerateImageTask(task, reason)
}

func failGenerateImageTaskWithRelayError(task *model.Task, relayErr *types.NewAPIError, headers http.Header) {
	reason := "上游返回错误"
	if relayErr != nil {
		reason = fmt.Sprintf("上游返回错误: %s", relayErr.MaskSensitiveErrorWithStatusCode())
	}
	failGenerateImageTaskWithDetail(task, reason, buildGenerateImageTaskErrorDetail(relayErr, headers))
}

func buildGenerateImageTaskErrorDetail(relayErr *types.NewAPIError, headers http.Header) *model.TaskErrorDetail {
	detail := &model.TaskErrorDetail{}
	if relayErr != nil {
		detail.UpstreamStatus = relayErr.StatusCode
		openAIError := relayErr.ToOpenAIError()
		detail.UpstreamCode = stringifyImageUpstreamCode(openAIError.Code)
		detail.UpstreamType = openAIError.Type
	}
	if headers != nil {
		detail.RetryAfterSeconds = parseRetryAfterSeconds(headers.Get("Retry-After"))
	}
	if isRetryableImageUpstreamStatus(detail.UpstreamStatus) {
		detail.RetryAction = imageRetryActionResubmit
		if detail.RetryAfterSeconds == 0 {
			detail.RetryAfterSeconds = defaultImageRetryAfterSeconds(detail.UpstreamStatus)
		}
	}
	if detail.UpstreamStatus == 0 && detail.UpstreamCode == "" && detail.UpstreamType == "" &&
		detail.RetryAfterSeconds == 0 && detail.RetryAction == "" {
		return nil
	}
	return detail
}

func stringifyImageUpstreamCode(code any) string {
	if code == nil {
		return ""
	}
	if value, ok := code.(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fmt.Sprintf("%v", code))
}

func parseRetryAfterSeconds(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds > 0 {
			return seconds
		}
		return 0
	}
	retryAt, err := time.Parse(http.TimeFormat, value)
	if err != nil {
		return 0
	}
	seconds := int(time.Until(retryAt).Seconds())
	if seconds > 0 {
		return seconds
	}
	return 0
}

func isRetryableImageUpstreamStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusRequestTimeout:
		return true
	default:
		return false
	}
}

func defaultImageRetryAfterSeconds(status int) int {
	switch status {
	case http.StatusTooManyRequests:
		return 30
	case http.StatusServiceUnavailable:
		return 60
	case http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusRequestTimeout:
		return 15
	default:
		return 0
	}
}

// newAsyncGinContext 构造一个用于异步执行的最小 gin.Context，并写入用户名供日志使用。
func newAsyncGinContext(userId int) *gin.Context {
	c := &gin.Context{
		Request: &http.Request{
			Method: "POST",
			Header: http.Header{"Content-Type": []string{"application/json"}},
			Body:   http.NoBody,
		},
	}
	if user, err := model.GetUserById(userId, false); err == nil {
		c.Set("username", user.Username)
	}
	return c
}

// ProcessGenerateImageTask 是统一生图端点的异步处理入口，按 task.Action 分发到具体 provider。
func ProcessGenerateImageTask(ctx context.Context, task *model.Task, requestData ...any) {
	defer func() {
		if r := recover(); r != nil {
			logger.LogError(ctx, fmt.Sprintf("generate_image: panic recovered: %v", r))
			failGenerateImageTask(task, fmt.Sprintf("内部错误 (panic): %v", r))
		}
		if task.Status == model.TaskStatusFailure {
			if task.Action == "generateContent" {
				SetGenerateContentRequestOmittedData(task, "failure")
				_ = task.Update()
			}
			RefundTaskQuota(ctx, task, task.FailReason)
		}
	}()

	c := newAsyncGinContext(task.UserId)

	task.Status = model.TaskStatusInProgress
	task.StartTime = time.Now().Unix()
	task.Progress = "50%"
	_ = task.Update()

	switch task.Action {
	case "generateContent":
		var nativeReq map[string]interface{}
		if len(requestData) > 0 {
			nativeReq, _ = requestData[0].(map[string]interface{})
		}
		processGenerateImageGemini(ctx, c, task, nativeReq)
	default:
		var asyncReq *dto.AsyncImageRequest
		if len(requestData) > 0 {
			asyncReq, _ = requestData[0].(*dto.AsyncImageRequest)
		}
		processGenerateImageOpenAI(ctx, c, task, asyncReq)
	}
}

// buildGenerateImageRelayInfo 构造处理阶段所需的 RelayInfo（含渠道密钥、模型映射、价格）。
func buildGenerateImageRelayInfo(c *gin.Context, task *model.Task, relayMode int) (*relaycommon.RelayInfo, error) {
	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		return nil, fmt.Errorf("获取渠道信息失败: %v", err)
	}
	apiType, _ := common.ChannelType2APIType(channel.Type)
	key, keyIndex, keyErr := channel.GetNextEnabledKey()
	if keyErr != nil {
		return nil, fmt.Errorf("获取渠道密钥失败: %v", keyErr.Error())
	}
	upstreamModelName := ApplyModelMapping(task.Properties.OriginModelName, channel.ModelMapping)
	relayInfo := &relaycommon.RelayInfo{
		UserId:          task.UserId,
		UserGroup:       common.GetContextKeyString(c, constant.ContextKeyUserGroup),
		UsingGroup:      task.Group,
		RelayMode:       relayMode,
		OriginModelName: task.Properties.OriginModelName,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: task.TaskID},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          channel.Type,
			ChannelId:            channel.Id,
			ChannelIsMultiKey:    channel.ChannelInfo.IsMultiKey,
			ChannelMultiKeyIndex: keyIndex,
			ChannelBaseUrl:       channel.GetBaseURL(),
			ApiType:              apiType,
			ApiVersion:           channel.Other,
			ApiKey:               key,
			UpstreamModelName:    upstreamModelName,
			IsModelMapped:        upstreamModelName != task.Properties.OriginModelName,
		},
	}
	// 异步 goroutine 没有真实 HTTP 请求,必须显式设置 RequestURLPath 供适配器拼 URL
	relayInfo.RequestURLPath = asyncImageRequestURLPath(relayMode, upstreamModelName)
	if CalculatePriceFunc != nil {
		if priceData, err := CalculatePriceFunc(c, relayInfo); err == nil {
			relayInfo.PriceData = priceData
		}
	}
	return relayInfo, nil
}

// processGenerateImageGemini 处理 Gemini 原生 generateContent 路径。
// 上游 inlineData(base64) 会上传到对象存储后返回 URL。
func processGenerateImageGemini(ctx context.Context, c *gin.Context, task *model.Task, requestBody map[string]interface{}) {
	if requestBody == nil {
		if err := task.GetData(&requestBody); err != nil {
			failGenerateImageTask(task, fmt.Sprintf("解析请求数据失败: %v", err))
			return
		}
	}
	if omitted, _ := requestBody["omitted"].(bool); omitted {
		failGenerateImageTask(task, "任务请求数据已省略，无法重新处理")
		return
	}
	imageCompression, _ := requestBody["image_compression"].(string)
	delete(requestBody, "image_compression") // 客户端参数，不透传上游

	// 规范化：首个 content 缺 role 时补 user
	if contents, ok := requestBody["contents"].([]interface{}); ok && len(contents) > 0 {
		if first, ok := contents[0].(map[string]interface{}); ok {
			if _, has := first["role"]; !has {
				first["role"] = "user"
			}
		}
	}

	relayInfo, err := buildGenerateImageRelayInfo(c, task, relayconstant.RelayModeGemini)
	if err != nil {
		failGenerateImageTask(task, err.Error())
		return
	}

	var jsonData []byte
	jsonData, err = common.Marshal(requestBody)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("序列化请求失败: %v", err))
		return
	}
	// nano-banana 的上游有 20MB 请求体硬上限。为保证网关 CPU 不被图片缩放
	// 拖垮，这里不再在服务端做 resize 救场，而是直接打回，让客户端自行压缩。
	if isNanoBananaModelName(task.Properties.OriginModelName) || isNanoBananaModelName(relayInfo.UpstreamModelName) {
		if len(jsonData) > nanoBananaGenerateContentMaxBodyBytes {
			maxMB := nanoBananaGenerateContentMaxBodyBytes / 1024 / 1024
			curMB := float64(len(jsonData)) / 1024 / 1024
			// 请求体含 base64 编码（约放大 1.37 倍），据此估算原图体积，便于客户端定位。
			rawMB := curMB / 1.37
			failGenerateImageTask(task, fmt.Sprintf(
				"请求体过大，请缩小图片体积。当前约 %.1f MB（含 base64 编码，原图约 %.1f MB），上限 %d MB。",
				curMB, rawMB, maxMB))
			return
		}
	}

	if GetGeminiAdaptorFunc == nil {
		failGenerateImageTask(task, "内部错误：Gemini 适配器未初始化")
		return
	}
	adaptor := GetGeminiAdaptorFunc(relayInfo.ApiType)
	if adaptor == nil {
		failGenerateImageTask(task, fmt.Sprintf("不支持的 API 类型: %d", relayInfo.ApiType))
		return
	}
	adaptor.Init(relayInfo)

	logger.LogInfo(ctx, fmt.Sprintf("generate_image(gemini): model=%s, baseUrl=%s, bodyLen=%d",
		relayInfo.UpstreamModelName, relayInfo.ChannelBaseUrl, len(jsonData)))

	resp, err := adaptor.DoRequest(c, relayInfo, bytes.NewReader(jsonData))
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("请求上游失败: %v", err))
		return
	}
	httpResp := resp.(*http.Response)
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		httpResp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		logger.LogError(ctx, fmt.Sprintf("generate_image(gemini): upstream error status=%d body=%s", httpResp.StatusCode, string(bodyBytes)))
		relayErr := RelayErrorHandler(ctx, httpResp, false)
		failGenerateImageTaskWithRelayError(task, relayErr, httpResp.Header)
		return
	}
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("读取响应失败: %v", err))
		return
	}

	var geminiResp map[string]interface{}
	if err := common.Unmarshal(bodyBytes, &geminiResp); err != nil {
		failGenerateImageTask(task, fmt.Sprintf("解析响应失败: %v", err))
		return
	}

	promptTokens, completionTokens, tokenDetails := extractGeminiUsage(geminiResp)
	images := extractGeminiImages(geminiResp)
	if len(images) == 0 {
		failGenerateImageTask(task, "上游未返回图片数据")
		return
	}
	images, err = prepareGenerateImageResults(images, imageCompression, task.Properties.RequestHost)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("上传图片到对象存储失败: %v", err))
		return
	}

	finalizeGenerateImageTask(ctx, task, images, promptTokens, completionTokens, tokenDetails,
		relayInfo.UpstreamModelName, relayInfo.IsModelMapped, asString(geminiResp["modelVersion"]))
}

// extractGeminiUsage 从 Gemini 响应的 usageMetadata 提取 token 用量。
func extractGeminiUsage(geminiResp map[string]interface{}) (promptTokens, completionTokens int, details map[string]interface{}) {
	details = map[string]interface{}{}
	usage, ok := geminiResp["usageMetadata"].(map[string]interface{})
	if !ok {
		return 0, 0, details
	}
	if pt, ok := usage["promptTokenCount"].(float64); ok {
		promptTokens = int(pt)
	}
	if ct, ok := usage["candidatesTokenCount"].(float64); ok {
		completionTokens = int(ct)
	}
	if tt, ok := usage["totalTokenCount"].(float64); ok {
		details["total_tokens"] = int(tt)
	}
	if th, ok := usage["thoughtsTokenCount"].(float64); ok {
		completionTokens += int(th)
		details["thought_tokens"] = int(th)
	}
	return promptTokens, completionTokens, details
}

// extractGeminiImages 从 Gemini 响应的 candidates.parts 提取图片（base64），跳过 thought 部分。
func extractGeminiImages(geminiResp map[string]interface{}) []dto.GenerateImageData {
	var images []dto.GenerateImageData
	candidates, ok := geminiResp["candidates"].([]interface{})
	if !ok {
		return images
	}
	for _, candidate := range candidates {
		candidateMap, ok := candidate.(map[string]interface{})
		if !ok {
			continue
		}
		content, ok := candidateMap["content"].(map[string]interface{})
		if !ok {
			continue
		}
		parts, ok := content["parts"].([]interface{})
		if !ok {
			continue
		}
		for _, part := range parts {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			if isThought, _ := partMap["thought"].(bool); isThought {
				continue
			}
			inlineData, ok := partMap["inlineData"].(map[string]interface{})
			if !ok {
				continue
			}
			b64, ok := inlineData["data"].(string)
			if !ok || b64 == "" {
				continue
			}
			mimeType := "image/png"
			if mt, ok := inlineData["mimeType"].(string); ok && mt != "" {
				mimeType = mt
			}
			images = append(images, dto.GenerateImageData{B64Json: b64, MimeType: mimeType})
		}
	}
	return images
}

// asString 安全地把 interface{} 转为 string。
func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

// processGenerateImageOpenAI 处理通用 OpenAI image 适配器路径（兜底 provider）。
// 任务数据存的是 dto.AsyncImageRequest。上游 url 原样保留，b64_json 上传到对象存储后返回 URL。
func processGenerateImageOpenAI(ctx context.Context, c *gin.Context, task *model.Task, asyncReqInput *dto.AsyncImageRequest) {
	var asyncReq dto.AsyncImageRequest
	if asyncReqInput != nil {
		asyncReq = *asyncReqInput
	} else if err := task.GetData(&asyncReq); err != nil {
		failGenerateImageTask(task, fmt.Sprintf("解析任务数据失败: %v", err))
		return
	}

	// 参考图：URL → base64 data-uri（供上游使用）
	resolvedImage, resolvedImages, err := resolveReferenceImagesForUpstream(asyncReq.Image, asyncReq.Images)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("下载参考图片失败: %v", err))
		return
	}

	imageReq := &dto.ImageRequest{
		Model:          asyncReq.Model,
		Prompt:         asyncReq.Prompt,
		N:              asyncReq.N,
		Size:           asyncReq.Size,
		AspectRatio:    asyncReq.AspectRatio,
		Quality:        asyncReq.Quality,
		ResponseFormat: asyncReq.ResponseFormat,
		OutputFormat:   stringPtrToRawMessage(asyncReq.OutputFormat),
		Style:          asyncReq.Style,
		User:           asyncReq.User,
		Image:          resolvedImage,
		Images:         resolvedImages,
		Mask:           imageReferenceToRawMessage(asyncReq.Mask),
	}

	relayMode := asyncOpenAIImageRelayMode(imageReq)
	relayInfo, err := buildGenerateImageRelayInfo(c, task, relayMode)
	if err != nil {
		failGenerateImageTask(task, err.Error())
		return
	}
	relayInfo.Request = imageReq

	if GetImageAdaptorFunc == nil {
		failGenerateImageTask(task, "内部错误：适配器未初始化")
		return
	}
	adaptor := GetImageAdaptorFunc(relayInfo.ApiType)
	if adaptor == nil {
		failGenerateImageTask(task, fmt.Sprintf("不支持的 API 类型: %d", relayInfo.ApiType))
		return
	}
	adaptor.Init(relayInfo)

	upstreamImageReq := prepareAsyncOpenAIImageRequest(imageReq, relayInfo)
	relayInfo.Request = upstreamImageReq

	requestBody, requestBodyLen, err := buildAsyncOpenAIImageRequestBody(c, adaptor, relayInfo, upstreamImageReq)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("转换请求失败: %v", err))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("generate_image(openai): model=%s, baseUrl=%s, bodyLen=%d",
		relayInfo.UpstreamModelName, relayInfo.ChannelBaseUrl, requestBodyLen))

	resp, err := adaptor.DoRequest(c, relayInfo, requestBody)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("请求上游失败: %v", err))
		return
	}
	httpResp := resp.(*http.Response)
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		httpResp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		logger.LogError(ctx, fmt.Sprintf("generate_image(openai): upstream error status=%d body=%s", httpResp.StatusCode, string(bodyBytes)))
		relayErr := RelayErrorHandler(ctx, httpResp, false)
		failGenerateImageTaskWithRelayError(task, relayErr, httpResp.Header)
		return
	}
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("读取响应失败: %v", err))
		return
	}

	var imageResp dto.ImageResponse
	if err := common.Unmarshal(bodyBytes, &imageResp); err != nil {
		failGenerateImageTask(task, fmt.Sprintf("解析响应失败: %v", err))
		return
	}
	if len(imageResp.Data) == 0 {
		failGenerateImageTask(task, "上游未返回图片数据")
		return
	}

	images := make([]dto.GenerateImageData, 0, len(imageResp.Data))
	for _, d := range imageResp.Data {
		if d.B64Json != "" {
			images = append(images, dto.GenerateImageData{B64Json: d.B64Json, MimeType: detectImageMimeType(d.B64Json)})
		} else if d.Url != "" {
			images = append(images, dto.GenerateImageData{Url: d.Url})
		}
	}
	if len(images) == 0 {
		failGenerateImageTask(task, "上游未返回图片数据")
		return
	}
	images, err = prepareGenerateImageResults(images, asyncReq.ImageCompression, task.Properties.RequestHost)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("上传图片到对象存储失败: %v", err))
		return
	}

	promptTokens, completionTokens, tokenDetails, modelVersion := extractOpenAIImageUsage(bodyBytes)
	finalizeGenerateImageTask(ctx, task, images, promptTokens, completionTokens, tokenDetails,
		relayInfo.UpstreamModelName, relayInfo.IsModelMapped, modelVersion)
}

func prepareGenerateImageResults(images []dto.GenerateImageData, compression, requestHost string) ([]dto.GenerateImageData, error) {
	out := make([]dto.GenerateImageData, 0, len(images))
	for _, image := range images {
		if image.Url != "" {
			out = append(out, dto.GenerateImageData{
				Url:      image.Url,
				MimeType: image.MimeType,
			})
			continue
		}
		if image.B64Json == "" {
			continue
		}
		mimeType := image.MimeType
		if mimeType == "" {
			mimeType = detectImageMimeType(image.B64Json)
		}
		url, err := uploadGenerateImageBase64(mimeType, image.B64Json, compression, requestHost)
		if err != nil {
			return nil, err
		}
		out = append(out, dto.GenerateImageData{
			Url:      url,
			MimeType: mimeType,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("上游未返回图片数据")
	}
	return out, nil
}

// extractOpenAIImageUsage 从 OpenAI-格式 image 响应原始体提取 token 用量与 modelVersion。
// gpt-image 系列上游返回 input_tokens/output_tokens（含 input_tokens_details.image_tokens），
// 而非 prompt_tokens/completion_tokens。这里复用 dto.SimpleResponse 同时兼容两套字段名，
// 归一逻辑与同步图像路径 OpenaiHandlerWithUsage 保持一致，避免按量计费取到 0 token。
func extractOpenAIImageUsage(bodyBytes []byte) (promptTokens, completionTokens int, details map[string]interface{}, modelVersion string) {
	details = map[string]interface{}{}

	var resp dto.SimpleResponse
	if err := common.Unmarshal(bodyBytes, &resp); err != nil {
		return 0, 0, details, ""
	}

	// 归一：把 input_tokens/output_tokens 累加进 prompt/completion（与同步路径一致）。
	if resp.InputTokens > 0 {
		resp.PromptTokens += resp.InputTokens
	}
	if resp.OutputTokens > 0 {
		resp.CompletionTokens += resp.OutputTokens
	}
	promptTokens = resp.PromptTokens
	completionTokens = resp.CompletionTokens

	if resp.TotalTokens > 0 {
		details["total_tokens"] = resp.TotalTokens
	}
	if resp.InputTokensDetails != nil && resp.InputTokensDetails.ImageTokens > 0 {
		details["image_tokens"] = resp.InputTokensDetails.ImageTokens
	}

	// modelVersion 在响应顶层，SimpleResponse 不含，单独提取。
	var raw map[string]interface{}
	if err := common.Unmarshal(bodyBytes, &raw); err == nil {
		modelVersion = asString(raw["modelVersion"])
	}

	return promptTokens, completionTokens, details, modelVersion
}

func stringPtrToRawMessage(value *string) json.RawMessage {
	if value == nil {
		return nil
	}
	raw, err := common.Marshal(*value)
	if err != nil {
		return nil
	}
	return raw
}

func imageReferenceToRawMessage(value *dto.ImageReference) json.RawMessage {
	if value == nil {
		return nil
	}
	raw, err := common.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}

// resolveReferenceImagesForUpstream 把参考图的 http(s) URL 下载并转成 base64 data-uri，
// 非 URL（已是 base64 / data-uri）原样保留。供 OpenAI image 适配器使用。
func resolveReferenceImagesForUpstream(image json.RawMessage, images []string) (json.RawMessage, json.RawMessage, error) {
	var resolvedImage json.RawMessage
	if len(image) > 0 {
		var imageStr string
		if err := common.Unmarshal(image, &imageStr); err == nil {
			if strings.HasPrefix(imageStr, "http://") || strings.HasPrefix(imageStr, "https://") {
				mimeType, b64, err := GetImageFromUrlWithLimit(imageStr, AsyncImageMaxURLSizeMB)
				if err != nil {
					return nil, nil, err
				}
				resolved := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
				resolvedImage, _ = common.Marshal(resolved)
			} else {
				resolvedImage = image
			}
		} else {
			resolvedImage = image
		}
	}

	var resolvedImages json.RawMessage
	if len(images) > 0 {
		out := make([]string, len(images))
		var g errgroup.Group
		for i, imgURL := range images {
			i, imgURL := i, imgURL // 捕获循环变量
			if strings.HasPrefix(imgURL, "http://") || strings.HasPrefix(imgURL, "https://") {
				g.Go(func() error {
					mimeType, b64, err := GetImageFromUrlWithLimit(imgURL, AsyncImageMaxURLSizeMB)
					if err != nil {
						return err
					}
					out[i] = fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
					return nil
				})
			} else {
				out[i] = imgURL
			}
		}
		if err := g.Wait(); err != nil {
			return nil, nil, err
		}
		resolvedImages, _ = common.Marshal(out)
	}
	return resolvedImage, resolvedImages, nil
}

// finalizeGenerateImageTask 把生成结果写入任务、结算计费、更新提交日志，是两条 provider 路径的统一收尾。
func finalizeGenerateImageTask(ctx context.Context, task *model.Task, images []dto.GenerateImageData,
	promptTokens, completionTokens int, tokenDetails map[string]interface{},
	upstreamModelName string, isModelMapped bool, upstreamModelVersion string) {

	result := dto.GenerateImageResult{
		Model:   task.Properties.OriginModelName,
		Created: time.Now().Unix(),
		Images:  images,
	}
	task.SetData(result)
	if len(images) > 0 && images[0].Url != "" {
		task.PrivateData.ResultURL = images[0].Url
	}
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	_ = task.Update()

	// 按 token 计费的模型：完成后用真实用量重新结算差额
	if bc := task.PrivateData.BillingContext; bc != nil && !bc.PerCallBilling {
		if len(bc.TieredSnapshot) > 0 {
			// tiered_expr 模型：用冻结的 BillingSnapshot + 真实 token 重算
			var snap billingexpr.BillingSnapshot
			if err := common.Unmarshal(bc.TieredSnapshot, &snap); err == nil {
				params := billingexpr.TokenParams{
					P:   float64(promptTokens),
					C:   float64(completionTokens),
					Len: float64(promptTokens + completionTokens),
				}
				if imgTokens, ok := tokenDetails["image_tokens"]; ok {
					if v, ok := imgTokens.(int); ok {
						params.Img = float64(v)
						params.P -= params.Img
						if params.P < 0 {
							params.P = 0
						}
					}
				}
				tr, err := billingexpr.ComputeTieredQuota(&snap, params)
				if err == nil {
					SettleTaskQuotaInSubmitLog(ctx, task, tr.ActualQuotaAfterGroup,
						fmt.Sprintf("tiered_expr重算：p=%d, c=%d, img=%.0f, tier=%s",
							promptTokens, completionTokens, params.Img, tr.MatchedTier),
						map[string]interface{}{
							"matched_tier": tr.MatchedTier,
						})
				} else {
					logger.LogError(ctx, fmt.Sprintf("generate_image: tiered settle failed: %v", err))
				}
			}
		} else {
			RecalculateTaskQuotaByTokens(ctx, task, promptTokens, completionTokens)
		}
	}

	// 0 输出 token（疑似风控）：全额退款
	if completionTokens == 0 && task.Quota > 0 {
		logger.LogWarn(ctx, fmt.Sprintf("generate_image: 上游返回0输出token（疑似风控），退还扣费，任务 %s，模型 %s",
			task.TaskID, task.Properties.OriginModelName))
		RefundTaskQuota(ctx, task, "上游返回0输出token（疑似风控），退还全部扣费")
	}

	// 更新提交时的消费日志为完成态
	useTime := int(task.FinishTime - task.StartTime)
	updateContent := fmt.Sprintf("统一生图，生成 %d 张图片，异步任务 %s（已完成）", len(images), task.TaskID)
	otherUpdates := map[string]interface{}{
		"task_status":           "SUCCESS",
		"generated_image_count": len(images),
	}
	for k, v := range tokenDetails {
		otherUpdates[k] = v
	}
	if isModelMapped {
		otherUpdates["is_model_mapped"] = true
		otherUpdates["upstream_model_name"] = upstreamModelName
	}
	if upstreamModelVersion != "" {
		otherUpdates["upstream_model_version"] = upstreamModelVersion
	}
	model.UpdateConsumeLogOnComplete(task.PrivateData.SubmitLogID, useTime, promptTokens, completionTokens, updateContent, otherUpdates)

	task.Properties.UpstreamModelName = upstreamModelName
	_ = task.Update()

	logger.LogInfo(ctx, fmt.Sprintf("generate_image: task %s 完成，生成 %d 张图片", task.TaskID, len(images)))
}
