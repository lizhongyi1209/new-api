package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

var imageErrorStatusCodePatterns = []*regexp.Regexp{
	regexp.MustCompile(`upstream error status=(\d{3})`),
	regexp.MustCompile(`status_code=(\d{3})`),
	regexp.MustCompile(`bad response status code (\d{3})`),
	regexp.MustCompile(`status code[:= ]+(\d{3})`),
}

func BuildFriendlyImageError(reason, requestID, taskID string) (string, *dto.ImageErrorDetail) {
	reason = strings.TrimSpace(reason)
	lower := strings.ToLower(reason)
	status := extractImageErrorStatus(reason)

	message := "生成失败，请稍后重试。如多次失败请联系管理员。"
	detail := &dto.ImageErrorDetail{
		Code:           "image_unknown_error",
		Category:       "unknown",
		Retryable:      true,
		RequestID:      requestID,
		TaskID:         taskID,
		UpstreamStatus: status,
	}

	// Gemini/Nano Banana and GPT image providers can reuse the same HTTP status
	// for different failure causes. Keep reason-specific checks before status-code
	// fallbacks so 503 timeout and 429 resolution mismatch are not misreported.
	switch {
	case containsAny(lower, "image 参数已废弃", "image field is required", "the 'image' field is required"):
		detail.Code = "image_missing_input"
		detail.Category = "invalid_request"
		detail.Retryable = false
		message = "请使用 images 数组传入图片，单张图也放在数组中。"
	case containsAny(lower, "model is required"):
		detail.Code = "image_model_required"
		detail.Category = "invalid_request"
		detail.Retryable = false
		message = "缺少模型名称，请传入 model 参数。"
	case containsAny(lower, "prompt is required"):
		detail.Code = "image_prompt_required"
		detail.Category = "invalid_request"
		detail.Retryable = false
		message = "缺少提示词，请传入 prompt 参数。"
	case containsAny(lower, "invalid size", "pixel budget", "divisible by 16", "size an unexpected", "resolution mismatch", "尺寸", "文件大小") || status == 413:
		detail.Code = "image_invalid_size"
		detail.Category = "invalid_request"
		detail.Retryable = false
		if status == 413 || containsAny(lower, "文件大小") {
			detail.Code = "image_payload_too_large"
			detail.Category = "payload_too_large"
			message = "图片文件过大，请压缩后重试。"
		} else {
			message = "图片尺寸不支持，请调整为常见尺寸或比例后重试。"
		}
	case containsAny(lower, "mask image", "mask.file_id", "mask.image_url", "mask must provide"):
		detail.Code = "image_invalid_mask"
		detail.Category = "invalid_request"
		detail.Retryable = false
		message = "Mask 图片格式不正确，请使用有效的 mask 图片后重试。"
	case containsAny(lower, "too many input images", "input images; max is"):
		detail.Code = "image_too_many_input_images"
		detail.Category = "invalid_request"
		detail.Retryable = false
		message = "参考图片数量超出当前模型限制，请减少图片数量后重试。"
	case containsAny(lower, "quality must be", "output_format must be", "unrecognized request argument", "partial images", "tool choice", "input_fidelity", "requested operation is unsupported", "转换请求失败"):
		detail.Code = "image_invalid_parameter"
		detail.Category = "invalid_request"
		detail.Retryable = false
		message = "当前模型不支持部分请求参数，请调整参数后重试。"
	case status == 451 || containsAny(lower, "content_policy", "image_unsafe", "prompt_unsafe", "policy", "safety", "violat", "rejected"):
		detail.Code = "image_safety_blocked"
		detail.Category = "safety"
		detail.Retryable = false
		message = "请求内容可能不符合安全策略，请修改提示词或图片后重试。"
	case isImageTimeoutError(lower, status):
		detail.Code = "image_timeout"
		detail.Category = "timeout"
		detail.Retryable = true
		message = "图片生成耗时过长或连接中断，请稍后重试。"
	case isImageProviderQuotaError(lower):
		detail.Code = "image_provider_quota_exceeded"
		detail.Category = "provider_quota"
		detail.Retryable = false
		message = "当前模型服务额度不足，请联系管理员处理。"
	case isImageModelUnavailableError(lower, status):
		detail.Code = "image_model_unavailable"
		detail.Category = "model_unavailable"
		detail.Retryable = false
		message = "当前模型暂不可用，请稍后重试或切换模型。"
	case isImagePermissionError(lower, status):
		detail.Code = "image_provider_permission_required"
		detail.Category = "provider_permission"
		detail.Retryable = false
		message = "当前上游账号暂无该模型权限，请联系管理员处理。"
	case containsAny(lower, "rate limit", "too many requests", "cooling down", "concurrency limit", "pending requests") || status == 429:
		detail.Code = "image_rate_limited"
		detail.Category = "rate_limit"
		detail.Retryable = true
		message = "当前生图服务请求较多，请稍后重试。"
	case containsAny(lower, "high load", "负载", "no available compatible accounts", "overloaded", "usage limit", "当前模型", "系统繁忙") || status == 503:
		detail.Code = "image_upstream_busy"
		detail.Category = "upstream_busy"
		detail.Retryable = true
		message = "当前模型服务繁忙，请稍后重试。"
	case containsAny(lower, "对象存储", "storage upload", "oss upload", "r2 upload", "aliyun oss"):
		detail.Code = "image_storage_failed"
		detail.Category = "storage"
		detail.Retryable = true
		message = "图片已生成但保存失败，请稍后重试。"
	case containsAny(lower, "下载参考图片", "download reference image", "download upstream image"):
		detail.Code = "image_reference_download_failed"
		detail.Category = "invalid_request"
		detail.Retryable = false
		message = "参考图片无法访问或下载失败，请检查图片链接后重试。"
	case containsAny(lower, "上游未返回图片数据", "no image", "未返回图片"):
		detail.Code = "image_empty_result"
		detail.Category = "upstream_error"
		detail.Retryable = true
		message = "上游未返回图片结果，请稍后重试。"
	case status == 500 || status == 502 || containsAny(lower, "internal server", "server had an error", "upstream error", "openai_error", "system error", "unexpected end of json", "do request failed", "请求上游失败", "上游返回错误"):
		detail.Code = "image_upstream_error"
		detail.Category = "upstream_error"
		detail.Retryable = true
		message = "上游服务异常，请稍后重试。"
	case containsAny(lower, "解析", "序列化", "内部错误", "适配器", "渠道"):
		detail.Code = "image_internal_error"
		detail.Category = "internal_error"
		detail.Retryable = true
		message = "服务内部处理异常，请稍后重试。"
	}

	return message, detail
}

func BuildFriendlyImageErrorWithDetail(reason, requestID, taskID string, stored *model.TaskErrorDetail) (string, *dto.ImageErrorDetail) {
	classifyReason := reason
	if stored != nil {
		classifyReason = enrichImageErrorReason(classifyReason, stored)
	}
	message, detail := BuildFriendlyImageError(classifyReason, requestID, taskID)
	return message, mergeStoredImageErrorDetail(detail, stored)
}

func enrichImageErrorReason(reason string, stored *model.TaskErrorDetail) string {
	parts := make([]string, 0, 4)
	if reason != "" {
		parts = append(parts, reason)
	}
	if stored.UpstreamStatus > 0 && extractImageErrorStatus(reason) == 0 {
		parts = append(parts, fmt.Sprintf("status_code=%d", stored.UpstreamStatus))
	}
	if stored.UpstreamCode != "" {
		parts = append(parts, "upstream_code="+stored.UpstreamCode)
	}
	if stored.UpstreamType != "" {
		parts = append(parts, "upstream_type="+stored.UpstreamType)
	}
	return strings.Join(parts, ", ")
}

func mergeStoredImageErrorDetail(detail *dto.ImageErrorDetail, stored *model.TaskErrorDetail) *dto.ImageErrorDetail {
	if detail == nil || stored == nil {
		return detail
	}
	if stored.UpstreamStatus > 0 {
		detail.UpstreamStatus = stored.UpstreamStatus
	}
	detail.UpstreamCode = stored.UpstreamCode
	detail.UpstreamType = stored.UpstreamType
	if detail.Retryable {
		detail.RetryAfterSeconds = stored.RetryAfterSeconds
		detail.RetryAction = stored.RetryAction
	}
	return detail
}

func extractImageErrorStatus(reason string) int {
	for _, pattern := range imageErrorStatusCodePatterns {
		if matches := pattern.FindStringSubmatch(reason); len(matches) == 2 {
			status, err := strconv.Atoi(matches[1])
			if err == nil {
				return status
			}
		}
	}
	return 0
}

func isImageTimeoutError(lower string, status int) bool {
	return containsAny(lower, "timeout", "timed out", "poll_timeout", "timeout_error", "stream disconnected", "socket hang up", "eof") ||
		status == 408 || status == 504 || status == 524
}

func isImageProviderQuotaError(lower string) bool {
	return containsAny(lower,
		"billing hard limit",
		"insufficient credits",
		"insufficient_user_quota",
		"用户额度不足",
		"quota exhausted",
		"current quota",
		"token quota",
		"quota is not enough",
		"no active tokens available",
		"token_unavailable",
		"系统网关次数不足",
		"额度",
	)
}

func isImageModelUnavailableError(lower string, status int) bool {
	return containsAny(lower,
		"unknown model",
		"model was not found",
		"not found",
		"only supported on",
		"no available ai provider",
		"no available channel for model",
	) || status == 404 || status == 405
}

func isImagePermissionError(lower string, status int) bool {
	return containsAny(lower,
		"organization must be verified",
		"auth_unavailable",
		"authentication_error",
		"no auth",
		"api key",
		"token invalid",
		"invalid_grant",
		"failed to exchange jwt",
		"account not found",
		"无效的令牌",
	) || status == 401 || status == 403
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
