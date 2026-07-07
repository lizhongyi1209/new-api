package relay

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// gpt-image-2 专属报错封装
//
// 背景：gpt-image-2 / gpt-image-2-c 跑在共用的 OpenAI 图片 relay 路径上
// (/v1/images/generations 与 /v1/images/edits)。上游返回非 200 时，标准链路
// (service.RelayErrorHandler) 会把 HTML 错误页、非 UTF-8 响应体等透传成
// "bad response status code 400" / "invalid character 'å'" 这类对用户毫无意义
// 的信息。本文件仅针对 gpt-image-2 系列模型，将这些原始错误改写为可读的中文提示。
//
// 隔离保证：仅当 isGptImage2(originModelName) 命中时才启用本封装；其他所有模型
// (gpt-image-1、nano-banana、gemini 等) 走原有 RelayErrorHandler 分支，行为零变化。
// 原始上游响应仍会写入日志，便于排查。

// isGptImage2 判断是否为 gpt-image-2 系列模型（含 gpt-image-2-c、gpt-image-2-special 等映射别名）。
func isGptImage2(originModelName string) bool {
	name := strings.ToLower(strings.TrimSpace(originModelName))
	return name == "gpt-image-2" || strings.HasPrefix(name, "gpt-image-2-")
}

// gptImage2UpstreamError 读取上游错误响应体、分类并构造用户友好的错误。
// 仅在 gpt-image-2 系列模型的图片 relay 路径中，替代 service.RelayErrorHandler 调用。
// 读取响应体后会关闭它，与 RelayErrorHandler 的行为保持一致。
func gptImage2UpstreamError(c *gin.Context, resp *http.Response) *types.NewAPIError {
	status := http.StatusInternalServerError
	var body []byte
	if resp != nil {
		status = resp.StatusCode
		if resp.Body != nil {
			body, _ = io.ReadAll(resp.Body)
			service.CloseResponseBodyGracefully(resp)
		}
	}

	bodyText := string(body)
	// 保留原始上游响应到日志，便于排查（不回传给客户端）。
	logger.LogError(c, fmt.Sprintf("gpt-image-2 upstream error: status=%d body=%s", status, common.LocalLogPreview(bodyText)))

	code, message, mappedStatus := classifyGptImage2Error(status, bodyText)

	oaiErr := types.OpenAIError{
		Message: message,
		Type:    string(types.ErrorTypeUpstreamError),
		Code:    code,
	}
	return types.WithOpenAIError(oaiErr, mappedStatus)
}

// classifyGptImage2Error 把 (状态码, 响应体) 映射为 (错误码, 中文提示, 回传状态码)。
// 分类规则来自真实生产日志（/v1/images/generations 与 /v1/images/edits）。
func classifyGptImage2Error(status int, body string) (code string, message string, mappedStatus int) {
	lower := strings.ToLower(body)

	switch {
	// 内容安全：上游文案已是清晰中文，保留其语义，统一归类。
	case containsAnyStr(lower, "content_policy", "content policy", "safety", "violat", "违反", "裸露", "色情", "情色", "防护"):
		return "gpt_image_2_safety_blocked",
			"请求内容可能不符合安全策略，请修改提示词或图片后重试。",
			http.StatusBadRequest

	// 上游图片配额耗尽。
	case containsAnyStr(lower, "no available image quota", "image quota", "quota exhausted", "insufficient", "配额"):
		return "gpt_image_2_provider_quota_exceeded",
			"当前 gpt-image-2 服务额度不足，请联系管理员处理。",
			http.StatusTooManyRequests

	// 请求超时 / 上游连接超时。
	case containsAnyStr(lower, "max duration", "exceeded max duration"):
		return "gpt_image_2_timeout",
			"图片生成耗时过长（超过最大等待时间），请重试或精简提示词后再试。",
			http.StatusGatewayTimeout
	case containsAnyStr(lower, "timed out", "timeout", "connection timed out"):
		return "gpt_image_2_upstream_timeout",
			"连接 gpt-image-2 上游服务超时，请稍后重试。",
			http.StatusGatewayTimeout

	// 限流。
	case containsAnyStr(lower, "rate limit", "too many requests", "concurrency limit") || status == http.StatusTooManyRequests:
		return "gpt_image_2_rate_limited",
			"当前 gpt-image-2 请求较多，请稍后重试。",
			http.StatusTooManyRequests

	// 参数错误：数量 n 越界。
	case containsAnyStr(lower, "n must be between"):
		return "gpt_image_2_invalid_parameter",
			"生成数量 n 参数不合法，n 必须在 1-4 之间。",
			http.StatusBadRequest

	// 响应体编码损坏 / 非 UTF-8（"invalid character 'å'" 一类）。
	// 必须排在 HTML 判断之前：损坏的 body 首字节常非 JSON 起始符，
	// 否则会被 isHTMLBody 误判为“非结构化 HTML 响应”。
	case !utf8.ValidString(body) || containsAnyStr(lower, "invalid character", "looking for beginning of value"):
		return "gpt_image_2_upstream_corrupted",
			"gpt-image-2 上游返回的数据格式异常，请重试；若持续失败请联系管理员。",
			http.StatusBadGateway

	// 响应体为 HTML / 非结构化（网关拦截、上游维护、Web 服务器直接拒绝）。
	// 这是 "bad response status code 400" 的主因：上游返回 HTML 错误页而非 JSON。
	case isHTMLBody(body):
		return "gpt_image_2_upstream_invalid_response",
			"gpt-image-2 上游服务返回了异常响应（可能正在维护，或请求被网关拦截），请稍后重试；若持续失败请联系管理员。",
			http.StatusBadGateway
	}

	// 兜底：按状态码归类。
	switch {
	case status == http.StatusBadRequest:
		return "gpt_image_2_upstream_bad_request",
			"gpt-image-2 上游拒绝了本次请求（400），请检查参数或稍后重试。",
			http.StatusBadGateway
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "gpt_image_2_provider_permission",
			"当前上游账号暂无 gpt-image-2 权限或鉴权失败，请联系管理员处理。",
			status
	case status >= http.StatusInternalServerError:
		return "gpt_image_2_upstream_error",
			"gpt-image-2 上游服务异常，请稍后重试。",
			http.StatusBadGateway
	default:
		return "gpt_image_2_upstream_error",
			fmt.Sprintf("gpt-image-2 上游返回错误（%d），请稍后重试。", status),
			http.StatusBadGateway
	}
}

// isHTMLBody 判断响应体是否为 HTML 或非 JSON 的错误页文本。
func isHTMLBody(body string) bool {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "<!doctype") || strings.HasPrefix(lower, "<html") || strings.Contains(lower, "<head>") || strings.Contains(lower, "<body") {
		return true
	}
	// 形如 "400 ## Bad Request ## { ... }" 的网关错误页。
	if strings.Contains(body, "## Bad Request ##") || strings.Contains(body, "## ") {
		return true
	}
	// 首字符不是 JSON 起始符号，视为非结构化响应。
	first := trimmed[0]
	if first != '{' && first != '[' && first != '"' {
		return true
	}
	return false
}

func containsAnyStr(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

// rewriteGptImage2ResponseError 处理上游返回 200、但响应体解析失败的场景
// （DoResponse 阶段，body 已被消费，只能基于已生成的 error 文本分类）。
// 典型为 "invalid character 'å' looking for beginning of value" 一类的编码/JSON 损坏错误。
// 若无法归类则原样返回，不改变行为。
func rewriteGptImage2ResponseError(c *gin.Context, apiErr *types.NewAPIError) *types.NewAPIError {
	if apiErr == nil {
		return nil
	}
	errText := strings.ToLower(apiErr.Error())
	if !containsAnyStr(errText, "invalid character", "looking for beginning of value", "unexpected end of json", "cannot unmarshal") {
		// 非解析类错误，保持原样（可能是 adaptor 明确返回的业务错误）。
		return apiErr
	}

	logger.LogError(c, fmt.Sprintf("gpt-image-2 response parse error: %s", apiErr.Error()))

	oaiErr := types.OpenAIError{
		Message: "gpt-image-2 上游返回的数据格式异常，请重试；若持续失败请联系管理员。",
		Type:    string(types.ErrorTypeUpstreamError),
		Code:    "gpt_image_2_upstream_corrupted",
	}
	return types.WithOpenAIError(oaiErr, http.StatusBadGateway)
}
