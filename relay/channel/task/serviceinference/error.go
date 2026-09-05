package serviceinference

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// Error codes
const (
	// Balance errors
	ErrorCodeBalanceLocalInsufficient    = "BALANCE_LOCAL_INSUFFICIENT"
	ErrorCodeBalanceUpstreamInsufficient = "BALANCE_UPSTREAM_INSUFFICIENT"

	// Parameter errors
	ErrorCodeParamDurationInvalid   = "PARAM_DURATION_INVALID"
	ErrorCodeParamResolutionInvalid = "PARAM_RESOLUTION_INVALID"
	ErrorCodeParamImageURLInvalid   = "PARAM_IMAGE_URL_INVALID"
	ErrorCodeParamMissingRequired   = "PARAM_MISSING_REQUIRED"

	// Resource errors
	ErrorCodeAssetExpired     = "ASSET_EXPIRED"
	ErrorCodeAssetNotFound    = "ASSET_NOT_FOUND"
	ErrorCodeImageUnsupported = "IMAGE_FORMAT_UNSUPPORTED"

	// Model/Channel errors
	ErrorCodeModelNotAvailable       = "MODEL_NOT_AVAILABLE"
	ErrorCodeModelPriceNotConfigured = "MODEL_PRICE_NOT_CONFIGURED"
	ErrorCodeChannelUnavailable      = "CHANNEL_UNAVAILABLE"

	// Generic errors
	ErrorCodeUpstreamError = "UPSTREAM_ERROR"
	ErrorCodeInternalError = "INTERNAL_ERROR"
)

// Error types
const (
	ErrorTypeInsufficientBalance = "insufficient_balance"
	ErrorTypeInvalidParameter    = "invalid_parameter"
	ErrorTypeResourceNotFound    = "resource_not_found"
	ErrorTypePermissionDenied    = "permission_denied"
	ErrorTypeUpstreamService     = "upstream_service_error"
	ErrorTypeInternal            = "internal_error"
)

// ErrorDetails represents the structured error details
type ErrorDetails map[string]interface{}

// ServiceInferenceError represents a structured error for Seedance
type ServiceInferenceError struct {
	Code       string       `json:"code"`
	Message    string       `json:"message"`
	Type       string       `json:"type"`
	Details    ErrorDetails `json:"details,omitempty"`
	RequestID  string       `json:"request_id,omitempty"`
	StatusCode int          `json:"-"`
	// LocalError marks deterministic client-side errors (bad parameters, missing
	// fields, expired/invalid asset references, insufficient local balance) that
	// must NOT be retried on another channel and must NOT count against channel
	// health — the same request would fail on every channel. Genuine upstream or
	// transient errors leave this false so the caller may retry.
	LocalError bool `json:"-"`
}

// upstreamErrorResponse represents the upstream API error format
type upstreamErrorResponse struct {
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Param   string `json:"param"`
		Type    string `json:"type"`
	} `json:"error"`
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id"`
}

// WrapError converts various error formats into ServiceInferenceError
func WrapError(err error, statusCode int) *dto.TaskError {
	if err == nil {
		return nil
	}

	siErr := classifyError(err, statusCode)
	if siErr.RequestID == "" {
		var upstreamRef struct {
			RequestID string `json:"request_id"`
		}
		if unmarshalErr := common.Unmarshal([]byte(err.Error()), &upstreamRef); unmarshalErr == nil {
			siErr.RequestID = upstreamRef.RequestID
		}
	}

	// Build the final error response
	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    siErr.Code,
			"message": siErr.Message,
			"type":    siErr.Type,
		},
	}

	if len(siErr.Details) > 0 {
		errorResponse["error"].(map[string]interface{})["details"] = siErr.Details
	}

	// Marshal to JSON string for the message field
	jsonBytes, _ := common.Marshal(errorResponse)

	return &dto.TaskError{
		Code:       siErr.Code,
		Message:    string(jsonBytes),
		RequestID:  siErr.RequestID,
		StatusCode: siErr.StatusCode,
		Error:      err,
		LocalError: siErr.LocalError,
	}
}

// classifyError analyzes the error and returns appropriate ServiceInferenceError
func classifyError(err error, statusCode int) *ServiceInferenceError {
	errText := err.Error()
	errLower := strings.ToLower(errText)

	// Try to parse as JSON error response
	var upstreamErr upstreamErrorResponse
	if jsonErr := common.Unmarshal([]byte(errText), &upstreamErr); jsonErr == nil {
		if parsed := parseUpstreamError(&upstreamErr, statusCode); parsed != nil {
			parsed.RequestID = upstreamErr.RequestID
			return parsed
		}
	}

	// Upstream proxies sometimes bury the real InvalidParameter error (a client
	// HTTP 400) inside a wrapper response carrying a different status code (e.g. a
	// 502 proxy_error) with an empty top-level error code, so the structured parse
	// above misses it. Detect the invalid-parameter signature from the raw text and
	// classify it as a deterministic client error (400, non-retryable) rather than a
	// retryable upstream failure.
	if strings.Contains(errLower, "invalidparameter") ||
		(strings.Contains(errLower, "not valid") &&
			(strings.Contains(errLower, "duration") || strings.Contains(errLower, "resolution"))) {
		return parseInvalidParameterError(errText, statusCode)
	}

	// Local balance errors (Chinese)
	if strings.Contains(errText, "预扣费额度失败") {
		return parseLocalBalanceError(errText)
	}

	// Local balance errors (English)
	if strings.Contains(errLower, "token quota is not enough") {
		return parseTokenQuotaError(errText)
	}

	// Model price not configured
	if strings.Contains(errText, "的价格未配置") || strings.Contains(errLower, "price not configured") {
		return parseModelPriceError(errText)
	}

	// Image URL format error
	if strings.Contains(errLower, "image asset url must be http") {
		return &ServiceInferenceError{
			Code:       ErrorCodeParamImageURLInvalid,
			Message:    "图片 URL 格式错误，仅支持 http://、https:// 或 asset:// 协议",
			Type:       ErrorTypeInvalidParameter,
			StatusCode: http.StatusBadRequest,
			LocalError: true,
			Details: ErrorDetails{
				"parameter":           "image_url",
				"supported_protocols": []string{"http://", "https://", "asset://"},
			},
		}
	}

	// Missing required field
	if strings.Contains(errLower, "missing required field") {
		return parseMissingFieldError(errText)
	}

	// Channel unavailable
	if strings.Contains(errLower, "no available channel") {
		return parseChannelUnavailableError(errText)
	}

	// Asset/resource errors in nested upstream response
	if strings.Contains(errLower, "invalid assets resources") {
		return parseInvalidAssetsError(errText)
	}

	// Image format unsupported
	if strings.Contains(errLower, "formatunsupported") || strings.Contains(errLower, "image format is not supported") {
		return &ServiceInferenceError{
			Code:       ErrorCodeImageUnsupported,
			Message:    "图片格式不支持，请上传 JPG、PNG 或 WebP 格式的图片",
			Type:       ErrorTypeInvalidParameter,
			StatusCode: http.StatusBadRequest,
			LocalError: true,
			Details: ErrorDetails{
				"supported_formats": []string{"jpg", "jpeg", "png", "webp"},
			},
		}
	}

	// Default: upstream error with original message
	return &ServiceInferenceError{
		Code:       ErrorCodeUpstreamError,
		Message:    fmt.Sprintf("上游服务返回错误：%s", errText),
		Type:       ErrorTypeUpstreamService,
		StatusCode: statusCode,
	}
}

// parseUpstreamError handles structured upstream API errors
func parseUpstreamError(upstreamErr *upstreamErrorResponse, statusCode int) *ServiceInferenceError {
	// Handle nested error structure
	if upstreamErr.Error != nil {
		return parseUpstreamErrorStruct(upstreamErr.Error, statusCode)
	}

	// Handle flat structure
	if upstreamErr.Code != "" || upstreamErr.Message != "" {
		code := upstreamErr.Code
		message := upstreamErr.Message

		// AccountOverdueError
		if code == "AccountOverdueError" || strings.Contains(message, "overdue balance") {
			return &ServiceInferenceError{
				Code:       ErrorCodeBalanceUpstreamInsufficient,
				Message:    "上游服务账号余额不足，请联系管理员充值",
				Type:       ErrorTypeUpstreamService,
				StatusCode: http.StatusServiceUnavailable,
			}
		}

		// InvalidParameter errors
		if code == "InvalidParameter" {
			return parseInvalidParameterError(message, statusCode)
		}

		// build_request_failed
		if code == "build_request_failed" {
			if strings.Contains(message, "invalid assets resources") {
				return parseInvalidAssetsError(message)
			}
		}

		// fail_to_fetch_task - this often wraps another error
		if code == "fail_to_fetch_task" {
			return parseFailToFetchTaskError(message, statusCode)
		}
	}

	return nil
}

// parseUpstreamErrorStruct handles the nested error.error structure
func parseUpstreamErrorStruct(err *struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
	Type    string `json:"type"`
}, statusCode int) *ServiceInferenceError {

	// AccountOverdueError
	if err.Code == "AccountOverdueError" {
		return &ServiceInferenceError{
			Code:       ErrorCodeBalanceUpstreamInsufficient,
			Message:    "上游服务账号余额不足，请联系管理员充值",
			Type:       ErrorTypeUpstreamService,
			StatusCode: http.StatusServiceUnavailable,
		}
	}

	// InvalidParameter
	if err.Code == "InvalidParameter" {
		return parseInvalidParameterError(err.Message, statusCode)
	}

	return nil
}

// parseInvalidParameterError handles various invalid parameter errors
func parseInvalidParameterError(message string, statusCode int) *ServiceInferenceError {
	messageLower := strings.ToLower(message)

	// Duration parameter error
	if strings.Contains(messageLower, "duration") && strings.Contains(messageLower, "not valid") {
		model := extractModelFromMessage(message)
		mode := extractModeFromMessage(message)

		return &ServiceInferenceError{
			Code:       ErrorCodeParamDurationInvalid,
			Message:    fmt.Sprintf("时长参数不合法，模型 %s 在当前模式下不支持该时长值。请参考文档了解支持的时长范围", model),
			Type:       ErrorTypeInvalidParameter,
			StatusCode: http.StatusBadRequest,
			LocalError: true,
			Details: ErrorDetails{
				"parameter":  "duration",
				"model":      model,
				"mode":       mode,
				"suggestion": "不同模型支持的时长范围不同，请查阅API文档",
			},
		}
	}

	// Resolution parameter error
	if strings.Contains(messageLower, "resolution") && strings.Contains(messageLower, "not valid") {
		model := extractModelFromMessage(message)
		mode := extractModeFromMessage(message)

		return &ServiceInferenceError{
			Code:       ErrorCodeParamResolutionInvalid,
			Message:    fmt.Sprintf("分辨率参数不合法，模型 %s 不支持该分辨率", model),
			Type:       ErrorTypeInvalidParameter,
			StatusCode: http.StatusBadRequest,
			LocalError: true,
			Details: ErrorDetails{
				"parameter":  "resolution",
				"model":      model,
				"mode":       mode,
				"suggestion": "请使用 720p、1080p 等支持的分辨率",
			},
		}
	}

	// Asset not found
	if strings.Contains(messageLower, "asset") && strings.Contains(messageLower, "not found") {
		assetID := extractAssetIDFromMessage(message)
		return &ServiceInferenceError{
			Code:       ErrorCodeAssetNotFound,
			Message:    "引用的图片资源不存在，请重新上传图片",
			Type:       ErrorTypeResourceNotFound,
			StatusCode: http.StatusNotFound,
			LocalError: true,
			Details: ErrorDetails{
				"asset_id":   assetID,
				"suggestion": "请确认资源ID是否正确，或重新上传图片",
			},
		}
	}

	// Generic parameter error
	return &ServiceInferenceError{
		Code:       ErrorCodeUpstreamError,
		Message:    fmt.Sprintf("参数错误：%s", message),
		Type:       ErrorTypeInvalidParameter,
		StatusCode: http.StatusBadRequest,
		LocalError: true,
	}
}

// parseFailToFetchTaskError handles fail_to_fetch_task which wraps another error
func parseFailToFetchTaskError(message string, statusCode int) *ServiceInferenceError {
	// Try to extract the nested JSON error
	var nestedErr upstreamErrorResponse
	if err := common.Unmarshal([]byte(message), &nestedErr); err == nil {
		if parsed := parseUpstreamError(&nestedErr, statusCode); parsed != nil {
			return parsed
		}
	}

	// The wrapped payload frequently keeps the real InvalidParameter error as an
	// escaped string that does not cleanly re-parse as JSON. Detect it from the raw
	// text so a client parameter error stays a non-retryable 400 instead of being
	// surfaced as a retryable 502 upstream failure.
	messageLower := strings.ToLower(message)
	if strings.Contains(messageLower, "invalidparameter") ||
		(strings.Contains(messageLower, "not valid") &&
			(strings.Contains(messageLower, "duration") || strings.Contains(messageLower, "resolution"))) {
		return parseInvalidParameterError(message, statusCode)
	}

	return &ServiceInferenceError{
		Code:       ErrorCodeUpstreamError,
		Message:    fmt.Sprintf("任务获取失败：%s", message),
		Type:       ErrorTypeUpstreamService,
		StatusCode: http.StatusBadGateway,
	}
}

// parseInvalidAssetsError handles invalid asset resources errors
func parseInvalidAssetsError(errText string) *ServiceInferenceError {
	assetID := extractAssetIDFromMessage(errText)

	return &ServiceInferenceError{
		Code:       ErrorCodeAssetExpired,
		Message:    "引用的图片资源已过期或不存在，请重新上传图片",
		Type:       ErrorTypeResourceNotFound,
		StatusCode: http.StatusNotFound,
		LocalError: true,
		Details: ErrorDetails{
			"asset_id":   assetID,
			"suggestion": "图片资源有效期为7天，请重新上传",
		},
	}
}

// parseLocalBalanceError parses Chinese balance error
func parseLocalBalanceError(errText string) *ServiceInferenceError {
	// Extract amounts using regex
	reRemain := regexp.MustCompile(`剩余额度:\s*¥([\d.]+)`)
	reNeed := regexp.MustCompile(`需要预扣费额度:\s*¥([\d.]+)`)

	remain := "0.00"
	need := "0.00"

	if match := reRemain.FindStringSubmatch(errText); len(match) > 1 {
		remain = match[1]
	}
	if match := reNeed.FindStringSubmatch(errText); len(match) > 1 {
		need = match[1]
	}

	return &ServiceInferenceError{
		Code:       ErrorCodeBalanceLocalInsufficient,
		Message:    fmt.Sprintf("您的账户余额不足，当前余额 ¥%s，本次请求需要 ¥%s", remain, need),
		Type:       ErrorTypeInsufficientBalance,
		StatusCode: http.StatusPaymentRequired,
		LocalError: true,
	}
}

// parseTokenQuotaError parses English token quota error
func parseTokenQuotaError(errText string) *ServiceInferenceError {
	reRemain := regexp.MustCompile(`remain quota:\s*¥([\d.]+)`)
	reNeed := regexp.MustCompile(`need quota:\s*¥([\d.]+)`)

	remain := "0.00"
	need := "0.00"

	if match := reRemain.FindStringSubmatch(errText); len(match) > 1 {
		remain = match[1]
	}
	if match := reNeed.FindStringSubmatch(errText); len(match) > 1 {
		need = match[1]
	}

	return &ServiceInferenceError{
		Code:       ErrorCodeBalanceLocalInsufficient,
		Message:    fmt.Sprintf("您的账户余额不足，当前余额 ¥%s，本次请求需要 ¥%s", remain, need),
		Type:       ErrorTypeInsufficientBalance,
		StatusCode: http.StatusPaymentRequired,
		LocalError: true,
	}
}

// parseModelPriceError handles model price not configured errors
func parseModelPriceError(errText string) *ServiceInferenceError {
	// Extract model name
	model := ""
	re := regexp.MustCompile(`模型\s+(\S+)\s+的价格未配置`)
	if match := re.FindStringSubmatch(errText); len(match) > 1 {
		model = match[1]
	}

	return &ServiceInferenceError{
		Code:       ErrorCodeModelPriceNotConfigured,
		Message:    "模型价格未配置，请联系管理员",
		Type:       ErrorTypeUpstreamService,
		StatusCode: http.StatusInternalServerError,
		LocalError: true,
		Details: ErrorDetails{
			"model":      model,
			"admin_hint": "请在系统设置中配置该模型的价格",
		},
	}
}

// parseMissingFieldError handles missing required field errors
func parseMissingFieldError(errText string) *ServiceInferenceError {
	field := ""
	re := regexp.MustCompile(`Missing required field:\s*(\w+)`)
	if match := re.FindStringSubmatch(errText); len(match) > 1 {
		field = match[1]
	}

	return &ServiceInferenceError{
		Code:       ErrorCodeParamMissingRequired,
		Message:    fmt.Sprintf("缺少必填参数：%s", field),
		Type:       ErrorTypeInvalidParameter,
		StatusCode: http.StatusBadRequest,
		LocalError: true,
		Details: ErrorDetails{
			"missing_field": field,
		},
	}
}

// parseChannelUnavailableError handles channel unavailable errors
func parseChannelUnavailableError(errText string) *ServiceInferenceError {
	model := ""
	group := ""

	reModel := regexp.MustCompile(`model\s+(\S+)\s+under`)
	reGroup := regexp.MustCompile(`group\s+(\S+)`)

	if match := reModel.FindStringSubmatch(errText); len(match) > 1 {
		model = match[1]
	}
	if match := reGroup.FindStringSubmatch(errText); len(match) > 1 {
		group = match[1]
	}

	return &ServiceInferenceError{
		Code:       ErrorCodeChannelUnavailable,
		Message:    "当前没有可用的服务渠道，所有渠道均不可用或已达到限流上限",
		Type:       ErrorTypeUpstreamService,
		StatusCode: http.StatusServiceUnavailable,
		Details: ErrorDetails{
			"model":      model,
			"group":      group,
			"suggestion": "请稍后重试或联系管理员",
		},
	}
}

// Helper functions to extract information from error messages

func extractModelFromMessage(message string) string {
	re := regexp.MustCompile(`model\s+([a-zA-Z0-9_-]+)`)
	if match := re.FindStringSubmatch(message); len(match) > 1 {
		return match[1]
	}
	return ""
}

func extractModeFromMessage(message string) string {
	re := regexp.MustCompile(`in\s+(t2v|i2v|r2v)`)
	if match := re.FindStringSubmatch(message); len(match) > 1 {
		return match[1]
	}
	return ""
}

func extractAssetIDFromMessage(message string) string {
	re := regexp.MustCompile(`(?:asset|mva)-[\w-]+`)
	if match := re.FindString(message); match != "" {
		return match
	}
	return ""
}

// ParseModelNotAvailableError handles "Model not available" error
func ParseModelNotAvailableError(model string) *dto.TaskError {
	siErr := &ServiceInferenceError{
		Code:       ErrorCodeModelNotAvailable,
		Message:    "该模型暂不可用，可能是上游账号权限不足或模型已下线",
		Type:       ErrorTypePermissionDenied,
		StatusCode: http.StatusForbidden,
		Details: ErrorDetails{
			"model":      model,
			"suggestion": "请联系管理员检查上游账号权限或更换其他模型",
		},
	}

	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    siErr.Code,
			"message": siErr.Message,
			"type":    siErr.Type,
			"details": siErr.Details,
		},
	}

	jsonBytes, _ := common.Marshal(errorResponse)

	return &dto.TaskError{
		Code:       siErr.Code,
		Message:    string(jsonBytes),
		StatusCode: siErr.StatusCode,
		Error:      fmt.Errorf("model not available: %s", model),
		LocalError: false,
	}
}

// ParseOrganizationBillingSuspendedError handles upstream billing suspended error
func ParseOrganizationBillingSuspendedError() *dto.TaskError {
	siErr := &ServiceInferenceError{
		Code:       ErrorCodeBalanceUpstreamInsufficient,
		Message:    "上游服务账号余额不足，请联系管理员充值",
		Type:       ErrorTypeUpstreamService,
		StatusCode: http.StatusServiceUnavailable,
	}

	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    siErr.Code,
			"message": siErr.Message,
			"type":    siErr.Type,
		},
	}

	jsonBytes, _ := common.Marshal(errorResponse)

	return &dto.TaskError{
		Code:       siErr.Code,
		Message:    string(jsonBytes),
		StatusCode: siErr.StatusCode,
		Error:      fmt.Errorf("organization billing suspended"),
		LocalError: false,
	}
}
