package relay

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsGptImage2 锁定隔离边界：只有 gpt-image-2 系列命中封装，
// 其他模型（尤其 gpt-image-1、gpt-image 前缀的其他模型）必须不命中，
// 以保证对现有模型行为零影响。
func TestIsGptImage2(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{"gpt-image-2", true},
		{"gpt-image-2-c", true},
		{"gpt-image-2-special", true},
		{"GPT-IMAGE-2", true},   // 大小写不敏感
		{" gpt-image-2 ", true}, // 两端空白
		{"gpt-image-1", false},
		{"gpt-image-1-mini", false},
		{"gpt-image", false},
		{"gpt-image-20", false}, // 无连字符，不属于 -2 系列
		{"nano-banana-2", false},
		{"gemini-3-pro-image-preview", false},
		{"", false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, isGptImage2(c.model), "model=%q", c.model)
	}
}

// TestClassifyGptImage2Error 用真实生产日志中的错误样本，锁定分类与回传状态码。
func TestClassifyGptImage2Error(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		wantCode    string
		wantStatus  int
		msgNotEmpty bool
	}{
		{
			name:        "html gateway bad request (占比最高)",
			status:      http.StatusBadRequest,
			body:        "400 ## Bad Request ## {\n  \"detail\": \"blocked\"\n}",
			wantCode:    "gpt_image_2_upstream_invalid_response",
			wantStatus:  http.StatusBadGateway,
			msgNotEmpty: true,
		},
		{
			name:        "doctype html page",
			status:      http.StatusBadRequest,
			body:        "<!DOCTYPE html><html><head><title>400</title></head><body>Bad Request</body></html>",
			wantCode:    "gpt_image_2_upstream_invalid_response",
			wantStatus:  http.StatusBadGateway,
			msgNotEmpty: true,
		},
		{
			name:        "no available image quota",
			status:      http.StatusTooManyRequests,
			body:        "no available image quota",
			wantCode:    "gpt_image_2_provider_quota_exceeded",
			wantStatus:  http.StatusTooManyRequests,
			msgNotEmpty: true,
		},
		{
			name:        "connection timed out",
			status:      http.StatusBadGateway,
			body:        "upstream connection timed out, please retry later",
			wantCode:    "gpt_image_2_upstream_timeout",
			wantStatus:  http.StatusGatewayTimeout,
			msgNotEmpty: true,
		},
		{
			name:        "exceeded max duration",
			status:      http.StatusGatewayTimeout,
			body:        "image request exceeded max duration (600 seconds)",
			wantCode:    "gpt_image_2_timeout",
			wantStatus:  http.StatusGatewayTimeout,
			msgNotEmpty: true,
		},
		{
			name:        "n out of range",
			status:      http.StatusBadRequest,
			body:        "n must be between 1 and 4",
			wantCode:    "gpt_image_2_invalid_parameter",
			wantStatus:  http.StatusBadRequest,
			msgNotEmpty: true,
		},
		{
			name:        "safety block chinese",
			status:      http.StatusBadRequest,
			body:        "非常抱歉，生成的图片可能违反了关于裸露、色情或情色内容的防护限制。",
			wantCode:    "gpt_image_2_safety_blocked",
			wantStatus:  http.StatusBadRequest,
			msgNotEmpty: true,
		},
		{
			name:        "non-utf8 corrupted body",
			status:      http.StatusInternalServerError,
			body:        "\xe5 invalid character looking for beginning of value",
			wantCode:    "gpt_image_2_upstream_corrupted",
			wantStatus:  http.StatusBadGateway,
			msgNotEmpty: true,
		},
		{
			name:        "generic 500 json body",
			status:      http.StatusInternalServerError,
			body:        `{"error":{"message":"server had an error"}}`,
			wantCode:    "gpt_image_2_upstream_error",
			wantStatus:  http.StatusBadGateway,
			msgNotEmpty: true,
		},
		{
			name:        "403 permission preserves status",
			status:      http.StatusForbidden,
			body:        `{"error":{"message":"organization must be verified"}}`,
			wantCode:    "gpt_image_2_provider_permission",
			wantStatus:  http.StatusForbidden,
			msgNotEmpty: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, msg, status := classifyGptImage2Error(c.status, c.body)
			assert.Equal(t, c.wantCode, code)
			assert.Equal(t, c.wantStatus, status)
			if c.msgNotEmpty {
				require.NotEmpty(t, msg)
			}
		})
	}
}

// TestIsHTMLBody 锁定 HTML/非结构化响应体的识别，确保正常 JSON 不被误判。
func TestIsHTMLBody(t *testing.T) {
	assert.True(t, isHTMLBody("<!DOCTYPE html>"))
	assert.True(t, isHTMLBody("<html><body>x</body></html>"))
	assert.True(t, isHTMLBody("400 ## Bad Request ## {}"))
	assert.True(t, isHTMLBody("Bad Gateway"))
	assert.False(t, isHTMLBody(`{"error":"x"}`))
	assert.False(t, isHTMLBody(`["a","b"]`))
	assert.False(t, isHTMLBody(""))
}
