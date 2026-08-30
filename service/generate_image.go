package service

import (
	"bytes"
	"context"
	"crypto/tls"
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
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gabriel-vasile/mimetype"
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
//   - response_modalities 可选，默认不传；
//   - media_resolution 可选，透传到 generationConfig.mediaResolution；
//   - 内嵌图片字节会与文本提示、系统指令一起计入 20MB 总请求大小；
//   - n 用不到，默认不传，传入视为废弃；
//   - google_search 可选，只有 true 时启用 Google Search grounding；
//   - 不需要图片预签名上传。
func isGeminiImageModelName(modelName string) bool {
	normalizedModelName := strings.ToLower(strings.TrimSpace(modelName))
	return isNanoBananaModelName(normalizedModelName) ||
		(strings.HasPrefix(normalizedModelName, "gemini-") && strings.Contains(normalizedModelName, "image"))
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

var generateImageBackgroundValues = map[string]struct{}{
	"auto":        {},
	"transparent": {},
}

var generateImageModerationValues = map[string]struct{}{
	"auto": {},
	"low":  {},
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

	if req.Background != nil {
		background := strings.ToLower(strings.TrimSpace(*req.Background))
		if _, ok := generateImageBackgroundValues[background]; !ok {
			return fmt.Errorf("background must be one of auto, transparent")
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(req.Model)), "gpt-image") {
			if isGeminiImageModelName(req.Model) {
				return fmt.Errorf("background is not supported by Banana/Gemini image models; it is only supported by gpt-image models")
			}
			return fmt.Errorf("background is only supported by models with the gpt-image prefix")
		}
		if background == "transparent" && req.OutputFormat != nil && *req.OutputFormat == "jpeg" {
			return fmt.Errorf("output_format must be png or webp when background is transparent")
		}
		*req.Background = background
	}

	if req.Moderation != nil {
		moderation := strings.ToLower(strings.TrimSpace(*req.Moderation))
		if _, ok := generateImageModerationValues[moderation]; !ok {
			return fmt.Errorf("moderation must be one of auto, low")
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(req.Model)), "gpt-image") {
			if isGeminiImageModelName(req.Model) {
				return fmt.Errorf("moderation is not supported by Banana/Gemini image models; it is only supported by gpt-image models")
			}
			return fmt.Errorf("moderation is only supported by models with the gpt-image prefix")
		}
		*req.Moderation = moderation
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
	for i := range req.Images {
		if err := validateGenerateImageInput(&req.Images[i]); err != nil {
			return fmt.Errorf("images[%d]: %w", i, err)
		}
	}
	return nil
}

func validateGenerateImageInput(input *dto.GenerateImageInput) error {
	if input == nil {
		return fmt.Errorf("image input is required")
	}
	if input.Value != nil {
		value := strings.TrimSpace(*input.Value)
		if value == "" {
			return fmt.Errorf("image string must not be empty")
		}
		if isHTTPImageURL(value) {
			if err := ValidateSSRFProtectedFetchURL(value); err != nil {
				return fmt.Errorf("image URL is not allowed: %w", err)
			}
		}
		*input.Value = value
		return nil
	}
	if input.InlineData != nil {
		mimeType := normalizeGenerateImageMIMEType(input.InlineData.MimeType)
		data := strings.TrimSpace(input.InlineData.Data)
		if !strings.HasPrefix(mimeType, "image/") {
			return fmt.Errorf("inlineData.mimeType must start with image/")
		}
		if data == "" {
			return fmt.Errorf("inlineData.data must not be empty")
		}
		if strings.HasPrefix(data, "data:") {
			return fmt.Errorf("inlineData.data must contain raw base64 without a data URL prefix")
		}
		raw, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return fmt.Errorf("inlineData.data is not valid base64: %w", err)
		}
		maxBytes := AsyncImageMaxBase64SizeMB * 1024 * 1024
		if len(raw) > maxBytes {
			return fmt.Errorf("inlineData image size %.2f MB exceeds limit %d MB", float64(len(raw))/1024/1024, AsyncImageMaxBase64SizeMB)
		}
		if !mimetype.Detect(raw).Is(mimeType) {
			return fmt.Errorf("inlineData content does not match declared MIME type %s", mimeType)
		}
		input.InlineData.MimeType = mimeType
		input.InlineData.Data = data
		return nil
	}
	if input.FileData != nil {
		mimeType := normalizeGenerateImageMIMEType(input.FileData.MimeType)
		fileURI := strings.TrimSpace(input.FileData.FileURI)
		if !strings.HasPrefix(mimeType, "image/") {
			return fmt.Errorf("fileData.mimeType must start with image/")
		}
		if fileURI == "" {
			return fmt.Errorf("fileData.fileUri must not be empty")
		}
		if len(fileURI) > generateImageMaskImageURLMaxLength {
			return fmt.Errorf("fileData.fileUri length must be <= %d", generateImageMaskImageURLMaxLength)
		}
		if !isHTTPImageURL(fileURI) {
			return fmt.Errorf("fileData.fileUri must be a fully qualified http(s) URL")
		}
		if err := ValidateSSRFProtectedFetchURL(fileURI); err != nil {
			return fmt.Errorf("fileData.fileUri is not allowed: %w", err)
		}
		input.FileData.MimeType = mimeType
		input.FileData.FileURI = fileURI
		return nil
	}
	return fmt.Errorf("image input must provide a string, inlineData, or fileData")
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

func isHTTPImageURL(value string) bool {
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

// imageUpstreamUsageDetail 记录"上游返回 200 却没给图"这一事实。调用点均在上游 200 校验之后，
// 此时上游已受理请求并在其侧计费（方案A：以 HTTP 200 为已计费锚点，不依赖是否回显 token），
// 因此始终标记 UpstreamBilled=true，使失败退款逻辑不再退还预扣。若响应仍带 token 用量则一并
// 记录，仅用于核对。不保留响应体本身（可能含大量 base64 图片数据）。
func imageUpstreamUsageDetail(promptTokens, completionTokens int) *model.TaskErrorDetail {
	return &model.TaskErrorDetail{
		UpstreamBilled:           true,
		UpstreamPromptTokens:     promptTokens,
		UpstreamCompletionTokens: completionTokens,
	}
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

type generateImageUpstreamTrace struct {
	mu sync.Mutex

	startedAt          time.Time
	dnsStartedAt       time.Time
	dnsDoneAt          time.Time
	connectStartedAt   time.Time
	connectDoneAt      time.Time
	tlsStartedAt       time.Time
	tlsDoneAt          time.Time
	gotConnAt          time.Time
	wroteRequestAt     time.Time
	firstResponseAt    time.Time
	connectionReused   bool
	connectionWasIdle  bool
	connectionIdleTime time.Duration
	wroteRequestError  bool
	requestWrites      int
}

func newGenerateImageUpstreamTrace(startedAt time.Time) (*generateImageUpstreamTrace, *httptrace.ClientTrace) {
	timing := &generateImageUpstreamTrace{startedAt: startedAt}
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			timing.mu.Lock()
			if timing.dnsStartedAt.IsZero() {
				timing.dnsStartedAt = time.Now()
			}
			timing.mu.Unlock()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			timing.mu.Lock()
			timing.dnsDoneAt = time.Now()
			timing.mu.Unlock()
		},
		ConnectStart: func(_, _ string) {
			timing.mu.Lock()
			if timing.connectStartedAt.IsZero() {
				timing.connectStartedAt = time.Now()
			}
			timing.mu.Unlock()
		},
		ConnectDone: func(_, _ string, err error) {
			if err != nil {
				return
			}
			timing.mu.Lock()
			timing.connectDoneAt = time.Now()
			timing.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			timing.mu.Lock()
			timing.tlsStartedAt = time.Now()
			timing.mu.Unlock()
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			timing.mu.Lock()
			timing.tlsDoneAt = time.Now()
			timing.mu.Unlock()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			timing.mu.Lock()
			timing.gotConnAt = time.Now()
			timing.connectionReused = info.Reused
			timing.connectionWasIdle = info.WasIdle
			timing.connectionIdleTime = info.IdleTime
			timing.mu.Unlock()
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			timing.mu.Lock()
			timing.wroteRequestAt = time.Now()
			timing.wroteRequestError = info.Err != nil
			timing.requestWrites++
			timing.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			timing.mu.Lock()
			timing.firstResponseAt = time.Now()
			timing.mu.Unlock()
		},
	}
	return timing, trace
}

func generateImageDurationMilliseconds(start, end time.Time) float64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return -1
	}
	return end.Sub(start).Seconds() * 1000
}

func traceGenerateImageUpstreamRequest(
	ctx context.Context,
	c *gin.Context,
	task *model.Task,
	relayInfo *relaycommon.RelayInfo,
	provider string,
	requestBytes int64,
	request func() (any, error),
) (any, error) {
	startedAt := time.Now()
	timing, trace := newGenerateImageUpstreamTrace(startedAt)
	previousTrace, hadPreviousTrace := c.Get(common.UpstreamHTTPTraceKey)
	c.Set(common.UpstreamHTTPTraceKey, trace)
	response, err := request()
	if hadPreviousTrace {
		c.Set(common.UpstreamHTTPTraceKey, previousTrace)
	} else {
		c.Set(common.UpstreamHTTPTraceKey, nil)
	}
	finishedAt := time.Now()

	timing.mu.Lock()
	dnsMilliseconds := generateImageDurationMilliseconds(timing.dnsStartedAt, timing.dnsDoneAt)
	connectMilliseconds := generateImageDurationMilliseconds(timing.connectStartedAt, timing.connectDoneAt)
	tlsMilliseconds := generateImageDurationMilliseconds(timing.tlsStartedAt, timing.tlsDoneAt)
	connectionWaitMilliseconds := generateImageDurationMilliseconds(timing.startedAt, timing.gotConnAt)
	requestWriteMilliseconds := generateImageDurationMilliseconds(timing.gotConnAt, timing.wroteRequestAt)
	upstreamWaitMilliseconds := generateImageDurationMilliseconds(timing.wroteRequestAt, timing.firstResponseAt)
	responseHeaderMilliseconds := generateImageDurationMilliseconds(timing.firstResponseAt, finishedAt)
	connectionReused := timing.connectionReused
	connectionWasIdle := timing.connectionWasIdle
	connectionIdleMilliseconds := timing.connectionIdleTime.Seconds() * 1000
	wroteRequestError := timing.wroteRequestError
	transportAttempts := timing.requestWrites
	timing.mu.Unlock()
	requestMegabitsPerSecond := -1.0
	if requestWriteMilliseconds > 0 && requestBytes >= 0 {
		requestMegabitsPerSecond = float64(requestBytes) * 8 / 1000 / requestWriteMilliseconds
	}

	status := 0
	protocol := "unknown"
	if httpResponse, ok := response.(*http.Response); ok && httpResponse != nil {
		status = httpResponse.StatusCode
		protocol = httpResponse.Proto
	}
	logger.LogInfo(ctx, fmt.Sprintf(
		"generate_image_timing: phase=upstream_headers task=%s request_id=%s provider=%s model=%s channel=%d attempt=%d transport_attempts=%d status=%d protocol=%s request_bytes=%d total_ms=%.3f dns_ms=%.3f connect_ms=%.3f tls_ms=%.3f connection_wait_ms=%.3f request_write_ms=%.3f request_mbps=%.3f upstream_wait_ms=%.3f response_header_ms=%.3f conn_reused=%t conn_was_idle=%t conn_idle_ms=%.3f write_error=%t request_error=%t",
		task.TaskID,
		task.PrivateData.RequestID,
		provider,
		relayInfo.UpstreamModelName,
		task.ChannelId,
		len(task.PrivateData.UsedChannels),
		transportAttempts,
		status,
		protocol,
		requestBytes,
		finishedAt.Sub(startedAt).Seconds()*1000,
		dnsMilliseconds,
		connectMilliseconds,
		tlsMilliseconds,
		connectionWaitMilliseconds,
		requestWriteMilliseconds,
		requestMegabitsPerSecond,
		upstreamWaitMilliseconds,
		responseHeaderMilliseconds,
		connectionReused,
		connectionWasIdle,
		connectionIdleMilliseconds,
		wroteRequestError,
		err != nil,
	))
	return response, err
}

// doGenerateImageRequestWithStatusRetry applies the global relay retry policy to
// unified asynchronous image requests. Retries stay on the channel selected when
// the task was created so task attribution and billing remain consistent.
func doGenerateImageRequestWithStatusRetry(ctx context.Context, task *model.Task, request func() (any, error)) (*http.Response, *types.NewAPIError, error) {
	for retry := 0; ; retry++ {
		task.PrivateData.UsedChannels = append(task.PrivateData.UsedChannels, strconv.Itoa(task.ChannelId))
		resp, err := request()
		if err != nil {
			return nil, nil, err
		}
		httpResp, ok := resp.(*http.Response)
		if !ok || httpResp == nil {
			return nil, nil, fmt.Errorf("上游返回了无效的 HTTP 响应")
		}
		if httpResp.StatusCode == http.StatusOK {
			return httpResp, nil, nil
		}

		relayErr := RelayErrorHandler(ctx, httpResp, false)
		if retry >= common.RetryTimes || !operation_setting.ShouldRetryByStatusCode(httpResp.StatusCode) {
			return httpResp, relayErr, nil
		}

		_ = httpResp.Body.Close()
		logger.LogInfo(ctx, fmt.Sprintf(
			"generate_image: task=%s channel=%d retrying upstream after status=%d, retry=%d/%d",
			task.TaskID, task.ChannelId, httpResp.StatusCode, retry+1, common.RetryTimes,
		))
	}
}

func imageTaskAdminInfo(task *model.Task) map[string]interface{} {
	return map[string]interface{}{"use_channel": task.PrivateData.UsedChannels}
}

// newAsyncGinContext 构造一个用于异步执行的最小 gin.Context，并写入用户名供日志使用。
func newAsyncGinContext(task *model.Task) *gin.Context {
	requestContext := context.Background()
	if task.PrivateData.RequestID != "" {
		requestContext = context.WithValue(requestContext, common.RequestIdKey, task.PrivateData.RequestID)
	}
	c := &gin.Context{
		Request: &http.Request{
			Method: "POST",
			Header: http.Header{"Content-Type": []string{"application/json"}},
			Body:   http.NoBody,
		},
	}
	c.Request = c.Request.WithContext(requestContext)
	c.Set(common.RequestIdKey, task.PrivateData.RequestID)
	if user, err := model.GetUserById(task.UserId, false); err == nil {
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
			RefundFailedTaskQuotaByUpstreamUsage(ctx, task)
		}
	}()

	c := newAsyncGinContext(task)

	taskStartUpdateStartedAt := time.Now()
	task.Status = model.TaskStatusInProgress
	task.StartTime = time.Now().Unix()
	task.Progress = "50%"
	_ = task.Update()
	logger.LogInfo(ctx, fmt.Sprintf(
		"generate_image_timing: phase=task_start task=%s request_id=%s model=%s channel=%d task_start_update_ms=%.3f coarse_queue_ms=%d",
		task.TaskID, task.PrivateData.RequestID, task.Properties.OriginModelName, task.ChannelId, time.Since(taskStartUpdateStartedAt).Seconds()*1000, (task.StartTime-task.SubmitTime)*1000,
	))

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
			ChannelOtherSettings: channel.GetOtherSettings(),
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
// 上游图片可由 inlineData(base64) 或 fileData(URL) 返回，再按渠道图片输出策略处理。
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
	delete(requestBody, "image_compression") // 丢弃已废弃的客户端参数，避免透传上游

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

	httpResp, relayErr, err := doGenerateImageRequestWithStatusRetry(ctx, task, func() (any, error) {
		return traceGenerateImageUpstreamRequest(ctx, c, task, relayInfo, "gemini", int64(len(jsonData)), func() (any, error) {
			return adaptor.DoRequest(c, relayInfo, bytes.NewReader(jsonData))
		})
	})
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("请求上游失败: %v", err))
		return
	}
	defer httpResp.Body.Close()
	if relayErr != nil {
		logger.LogError(ctx, fmt.Sprintf("generate_image(gemini): upstream error status=%d: %s", httpResp.StatusCode, relayErr.Error()))
		failGenerateImageTaskWithRelayError(task, relayErr, httpResp.Header)
		return
	}
	bodyReadStartedAt := time.Now()
	bodyBytes, err := io.ReadAll(httpResp.Body)
	logger.LogInfo(ctx, fmt.Sprintf(
		"generate_image_timing: phase=upstream_body task=%s request_id=%s provider=gemini channel=%d body_bytes=%d body_read_ms=%.3f read_error=%t",
		task.TaskID, task.PrivateData.RequestID, task.ChannelId, len(bodyBytes), time.Since(bodyReadStartedAt).Seconds()*1000, err != nil,
	))
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("读取响应失败: %v", err))
		return
	}

	parseStartedAt := time.Now()
	var geminiResp map[string]interface{}
	if err := common.Unmarshal(bodyBytes, &geminiResp); err != nil {
		failGenerateImageTask(task, fmt.Sprintf("解析响应失败: %v", err))
		return
	}

	promptTokens, completionTokens, tokenDetails := extractGeminiUsage(geminiResp)
	images := extractGeminiImages(geminiResp)
	if len(images) == 0 {
		failGenerateImageTaskWithDetail(task, "上游未返回图片数据", imageUpstreamUsageDetail(promptTokens, completionTokens))
		return
	}
	logger.LogInfo(ctx, fmt.Sprintf(
		"generate_image_timing: phase=response_parse task=%s request_id=%s provider=gemini channel=%d images=%d parse_ms=%.3f",
		task.TaskID, task.PrivateData.RequestID, task.ChannelId, len(images), time.Since(parseStartedAt).Seconds()*1000,
	))
	images, outputTiming, err := prepareGenerateImageResultsWithStrategyTiming(ctx, images, task.Properties.RequestHost, relayInfo.ChannelOtherSettings.ImageOutputStrategy)
	logger.LogInfo(ctx, fmt.Sprintf(
		"generate_image_timing: phase=output_storage task=%s request_id=%s provider=gemini channel=%d strategy=%s images=%d source_base64=%d source_url=%d download_ms=%.3f upload_ms=%.3f upload_mbps=%.3f upload_transport_attempts=%d upload_connection_wait_ms=%.3f upload_request_write_ms=%.3f upload_server_wait_ms=%.3f upload_conn_reused=%t total_ms=%.3f output_bytes=%d storage_error=%t",
		task.TaskID, task.PrivateData.RequestID, task.ChannelId, relayInfo.ChannelOtherSettings.ImageOutputStrategy, len(images), outputTiming.SourceBase64, outputTiming.SourceURL, outputTiming.Download.Seconds()*1000, outputTiming.Upload.Seconds()*1000, outputTiming.UploadMegabitsPerSecond(), outputTiming.UploadTransportAttempts, outputTiming.UploadConnectionWaitMilliseconds, outputTiming.UploadRequestWriteMilliseconds, outputTiming.UploadServerWaitMilliseconds, outputTiming.UploadConnectionReused, outputTiming.Total.Seconds()*1000, outputTiming.OutputBytes, err != nil,
	))
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

	// 提取输出图像 token（用于 tiered_expr 表达式的 img_o 变量）
	if candidatesTokensDetails, ok := usage["candidatesTokensDetails"].([]interface{}); ok {
		for _, detail := range candidatesTokensDetails {
			if detailMap, ok := detail.(map[string]interface{}); ok {
				if modality, _ := detailMap["modality"].(string); modality == "IMAGE" {
					if tokenCount, ok := detailMap["tokenCount"].(float64); ok {
						details["image_output_tokens"] = int(tokenCount)
						break
					}
				}
			}
		}
	}

	return promptTokens, completionTokens, details
}

// extractGeminiImages 从 Gemini 响应的 candidates.parts 提取图片，跳过 thought 部分。
// 图片既可能以内联 base64（inlineData）返回，也可能以文件 URL（fileData）返回。
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
			if inlineData, ok := partMap["inlineData"].(map[string]interface{}); ok {
				b64, _ := inlineData["data"].(string)
				if b64 != "" {
					mimeType := "image/png"
					if mt, ok := inlineData["mimeType"].(string); ok && mt != "" {
						mimeType = mt
					}
					images = append(images, dto.GenerateImageData{B64Json: b64, MimeType: mimeType})
					continue
				}
			}

			fileData, ok := partMap["fileData"].(map[string]interface{})
			if !ok {
				continue
			}
			fileURI, _ := fileData["fileUri"].(string)
			fileURI = strings.TrimSpace(fileURI)
			if fileURI == "" {
				continue
			}
			mimeType := "image/png"
			if mt, ok := fileData["mimeType"].(string); ok && strings.TrimSpace(mt) != "" {
				mimeType = strings.TrimSpace(mt)
			}
			images = append(images, dto.GenerateImageData{Url: fileURI, MimeType: mimeType})
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

	imageReq := newAsyncOpenAIImageRequest(&asyncReq, resolvedImage, resolvedImages)

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

	firstRequestBody := requestBody
	httpResp, relayErr, err := doGenerateImageRequestWithStatusRetry(ctx, task, func() (any, error) {
		if firstRequestBody != nil {
			body := firstRequestBody
			firstRequestBody = nil
			return traceGenerateImageUpstreamRequest(ctx, c, task, relayInfo, "openai", int64(requestBodyLen), func() (any, error) {
				return adaptor.DoRequest(c, relayInfo, body)
			})
		}
		requestBody, rebuiltRequestBodyLen, buildErr := buildAsyncOpenAIImageRequestBody(c, adaptor, relayInfo, upstreamImageReq)
		if buildErr != nil {
			return nil, buildErr
		}
		return traceGenerateImageUpstreamRequest(ctx, c, task, relayInfo, "openai", int64(rebuiltRequestBodyLen), func() (any, error) {
			return adaptor.DoRequest(c, relayInfo, requestBody)
		})
	})
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("请求上游失败: %v", err))
		return
	}
	defer httpResp.Body.Close()
	if relayErr != nil {
		logger.LogError(ctx, fmt.Sprintf("generate_image(openai): upstream error status=%d: %s", httpResp.StatusCode, relayErr.Error()))
		failGenerateImageTaskWithRelayError(task, relayErr, httpResp.Header)
		return
	}
	bodyReadStartedAt := time.Now()
	bodyBytes, err := io.ReadAll(httpResp.Body)
	logger.LogInfo(ctx, fmt.Sprintf(
		"generate_image_timing: phase=upstream_body task=%s request_id=%s provider=openai channel=%d body_bytes=%d body_read_ms=%.3f read_error=%t",
		task.TaskID, task.PrivateData.RequestID, task.ChannelId, len(bodyBytes), time.Since(bodyReadStartedAt).Seconds()*1000, err != nil,
	))
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("读取响应失败: %v", err))
		return
	}

	parseStartedAt := time.Now()
	var imageResp dto.ImageResponse
	if err := common.Unmarshal(bodyBytes, &imageResp); err != nil {
		failGenerateImageTask(task, fmt.Sprintf("解析响应失败: %v", err))
		return
	}

	promptTokens, completionTokens, tokenDetails, modelVersion := extractOpenAIImageUsage(bodyBytes)
	if len(imageResp.Data) == 0 {
		failGenerateImageTaskWithDetail(task, "上游未返回图片数据", imageUpstreamUsageDetail(promptTokens, completionTokens))
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
		failGenerateImageTaskWithDetail(task, "上游未返回图片数据", imageUpstreamUsageDetail(promptTokens, completionTokens))
		return
	}
	logger.LogInfo(ctx, fmt.Sprintf(
		"generate_image_timing: phase=response_parse task=%s request_id=%s provider=openai channel=%d images=%d parse_ms=%.3f",
		task.TaskID, task.PrivateData.RequestID, task.ChannelId, len(images), time.Since(parseStartedAt).Seconds()*1000,
	))
	images, outputTiming, err := prepareGenerateImageResultsWithStrategyTiming(ctx, images, task.Properties.RequestHost, relayInfo.ChannelOtherSettings.ImageOutputStrategy)
	logger.LogInfo(ctx, fmt.Sprintf(
		"generate_image_timing: phase=output_storage task=%s request_id=%s provider=openai channel=%d strategy=%s images=%d source_base64=%d source_url=%d download_ms=%.3f upload_ms=%.3f upload_mbps=%.3f upload_transport_attempts=%d upload_connection_wait_ms=%.3f upload_request_write_ms=%.3f upload_server_wait_ms=%.3f upload_conn_reused=%t total_ms=%.3f output_bytes=%d storage_error=%t",
		task.TaskID, task.PrivateData.RequestID, task.ChannelId, relayInfo.ChannelOtherSettings.ImageOutputStrategy, len(images), outputTiming.SourceBase64, outputTiming.SourceURL, outputTiming.Download.Seconds()*1000, outputTiming.Upload.Seconds()*1000, outputTiming.UploadMegabitsPerSecond(), outputTiming.UploadTransportAttempts, outputTiming.UploadConnectionWaitMilliseconds, outputTiming.UploadRequestWriteMilliseconds, outputTiming.UploadServerWaitMilliseconds, outputTiming.UploadConnectionReused, outputTiming.Total.Seconds()*1000, outputTiming.OutputBytes, err != nil,
	))
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("上传图片到对象存储失败: %v", err))
		return
	}

	finalizeGenerateImageTask(ctx, task, images, promptTokens, completionTokens, tokenDetails,
		relayInfo.UpstreamModelName, relayInfo.IsModelMapped, modelVersion)
}

type generateImageOutputTiming struct {
	SourceBase64                     int
	SourceURL                        int
	OutputBytes                      int64
	Download                         time.Duration
	Upload                           time.Duration
	Total                            time.Duration
	UploadTransportAttempts          int
	UploadConnectionWaitMilliseconds float64
	UploadRequestWriteMilliseconds   float64
	UploadServerWaitMilliseconds     float64
	UploadConnectionReused           bool
}

func (timing generateImageOutputTiming) UploadMegabitsPerSecond() float64 {
	if timing.Upload <= 0 {
		return -1
	}
	return float64(timing.OutputBytes) * 8 / 1_000_000 / timing.Upload.Seconds()
}

func generateImageBase64DecodedSize(data string) int64 {
	if comma := strings.IndexByte(data, ','); comma >= 0 {
		data = data[comma+1:]
	}
	data = strings.TrimSpace(data)
	if data == "" {
		return 0
	}
	decodedSize := int64(base64.StdEncoding.DecodedLen(len(data)))
	if strings.HasSuffix(data, "==") {
		return decodedSize - 2
	}
	if strings.HasSuffix(data, "=") {
		return decodedSize - 1
	}
	return decodedSize
}

func (timing *generateImageOutputTiming) upload(ctx context.Context, mimeType, base64Data, strategy, requestHost string) (string, error) {
	startedAt := time.Now()
	if strategy != dto.ImageOutputStrategyOSS {
		url, err := UploadBase64ImageWithOutputStrategy(mimeType, base64Data, strategy, requestHost)
		timing.Upload += time.Since(startedAt)
		return url, err
	}

	httpTiming, trace := newGenerateImageUpstreamTrace(startedAt)
	uploadContext := httptrace.WithClientTrace(ctx, trace)
	url, err := UploadBase64ImageToOSSContext(uploadContext, mimeType, base64Data)
	finishedAt := time.Now()
	timing.Upload += finishedAt.Sub(startedAt)

	httpTiming.mu.Lock()
	timing.UploadTransportAttempts += httpTiming.requestWrites
	timing.UploadConnectionWaitMilliseconds += generateImageDurationMilliseconds(httpTiming.startedAt, httpTiming.gotConnAt)
	timing.UploadRequestWriteMilliseconds += generateImageDurationMilliseconds(httpTiming.gotConnAt, httpTiming.wroteRequestAt)
	timing.UploadServerWaitMilliseconds += generateImageDurationMilliseconds(httpTiming.wroteRequestAt, httpTiming.firstResponseAt)
	timing.UploadConnectionReused = httpTiming.connectionReused
	httpTiming.mu.Unlock()
	return url, err
}

func prepareGenerateImageResultsWithStrategy(images []dto.GenerateImageData, requestHost, strategy string) ([]dto.GenerateImageData, error) {
	out, _, err := prepareGenerateImageResultsWithStrategyTiming(context.Background(), images, requestHost, strategy)
	return out, err
}

func prepareGenerateImageResultsWithStrategyTiming(ctx context.Context, images []dto.GenerateImageData, requestHost, strategy string) ([]dto.GenerateImageData, generateImageOutputTiming, error) {
	startedAt := time.Now()
	timing := generateImageOutputTiming{}
	out := make([]dto.GenerateImageData, 0, len(images))
	for _, image := range images {
		if image.Url != "" {
			timing.SourceURL++
			if dto.IsImageOutputStorageStrategy(strategy) {
				downloadStartedAt := time.Now()
				mimeType, base64Data, err := GetImageFromUrl(image.Url)
				timing.Download += time.Since(downloadStartedAt)
				if err != nil {
					timing.Total = time.Since(startedAt)
					return nil, timing, err
				}
				timing.OutputBytes += generateImageBase64DecodedSize(base64Data)
				url, err := timing.upload(ctx, mimeType, base64Data, strategy, requestHost)
				if err != nil {
					timing.Total = time.Since(startedAt)
					return nil, timing, err
				}
				out = append(out, dto.GenerateImageData{Url: url, MimeType: mimeType})
				continue
			}
			out = append(out, dto.GenerateImageData{
				Url:      image.Url,
				MimeType: image.MimeType,
			})
			continue
		}
		if image.B64Json == "" {
			continue
		}
		timing.SourceBase64++
		timing.OutputBytes += generateImageBase64DecodedSize(image.B64Json)
		mimeType := image.MimeType
		if mimeType == "" {
			mimeType = detectImageMimeType(image.B64Json)
		}
		if strategy == "" || strategy == dto.ImageOutputStrategyPassthrough {
			out = append(out, dto.GenerateImageData{
				B64Json:  image.B64Json,
				MimeType: mimeType,
			})
			continue
		}
		url, err := timing.upload(ctx, mimeType, image.B64Json, strategy, requestHost)
		if err != nil {
			timing.Total = time.Since(startedAt)
			return nil, timing, err
		}
		out = append(out, dto.GenerateImageData{
			Url:      url,
			MimeType: mimeType,
		})
	}
	if len(out) == 0 {
		timing.Total = time.Since(startedAt)
		return nil, timing, fmt.Errorf("上游未返回图片数据")
	}
	timing.Total = time.Since(startedAt)
	return out, timing, nil
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
	if resp.OutputTokensDetails != nil && resp.OutputTokensDetails.ImageTokens > 0 {
		details["image_output_tokens"] = resp.OutputTokensDetails.ImageTokens
	} else if resp.CompletionTokenDetails.ImageTokens > 0 {
		details["image_output_tokens"] = resp.CompletionTokenDetails.ImageTokens
	} else if resp.CompletionTokens > 0 {
		// gpt-image 图像端点输出全部是图像 token（无明细时兜底）
		details["image_output_tokens"] = resp.CompletionTokens
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
	finalizeStartedAt := time.Now()

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
	resultUpdateStartedAt := time.Now()
	_ = task.Update()
	resultUpdateDuration := time.Since(resultUpdateStartedAt)

	// 完成后用真实用量重新结算差额（tiered_expr 或按 token 模型）
	billingStartedAt := time.Now()
	SettleAsyncImageTaskBilling(ctx, task, promptTokens, completionTokens, tokenDetails)
	billingDuration := time.Since(billingStartedAt)

	// 零用量退款检查：仅在既无 token 用量又无返图时退款；正常返图但不回显 usage 的上游照常扣费
	refundCheckStartedAt := time.Now()
	RefundZeroUsageTaskQuota(ctx, task, promptTokens, completionTokens, len(images), "generate_image")
	refundCheckDuration := time.Since(refundCheckStartedAt)

	// 更新提交时的消费日志为完成态
	useTime := int(task.FinishTime - task.StartTime)
	updateContent := fmt.Sprintf("统一生图，生成 %d 张图片，异步任务 %s（已完成）", len(images), task.TaskID)
	otherUpdates := map[string]interface{}{
		"task_status":           "SUCCESS",
		"generated_image_count": len(images),
		"admin_info":            imageTaskAdminInfo(task),
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
	logUpdateStartedAt := time.Now()
	model.UpdateConsumeLogOnComplete(task.PrivateData.SubmitLogID, useTime, promptTokens, completionTokens, updateContent, otherUpdates)
	logUpdateDuration := time.Since(logUpdateStartedAt)

	task.Properties.UpstreamModelName = upstreamModelName
	metadataUpdateStartedAt := time.Now()
	_ = task.Update()
	metadataUpdateDuration := time.Since(metadataUpdateStartedAt)
	logger.LogInfo(ctx, fmt.Sprintf(
		"generate_image_timing: phase=finalize task=%s request_id=%s channel=%d result_update_ms=%.3f billing_ms=%.3f refund_check_ms=%.3f log_update_ms=%.3f metadata_update_ms=%.3f total_ms=%.3f",
		task.TaskID,
		task.PrivateData.RequestID,
		task.ChannelId,
		resultUpdateDuration.Seconds()*1000,
		billingDuration.Seconds()*1000,
		refundCheckDuration.Seconds()*1000,
		logUpdateDuration.Seconds()*1000,
		metadataUpdateDuration.Seconds()*1000,
		time.Since(finalizeStartedAt).Seconds()*1000,
	))

	logger.LogInfo(ctx, fmt.Sprintf("generate_image: task %s 完成，生成 %d 张图片", task.TaskID, len(images)))
}
