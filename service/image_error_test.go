package service

import "testing"

func TestBuildFriendlyImageErrorClassifiesCommonErrors(t *testing.T) {
	tests := []struct {
		name        string
		reason      string
		wantCode    string
		wantCat     string
		wantRetry   bool
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "invalid size",
			reason:      "status_code=400, Invalid size '2560x3840'. Requested resolution exceeds the current pixel budget.",
			wantCode:    "image_invalid_size",
			wantCat:     "invalid_request",
			wantRetry:   false,
			wantStatus:  400,
			wantMessage: "图片尺寸不支持，请调整为常见尺寸或比例后重试。",
		},
		{
			name:        "rate limited",
			reason:      "status_code=429, We are currently servicing too many requests at the moment.",
			wantCode:    "image_rate_limited",
			wantCat:     "rate_limit",
			wantRetry:   true,
			wantStatus:  429,
			wantMessage: "当前生图服务请求较多，请稍后重试。",
		},
		{
			name:        "nano banana timeout with service unavailable status",
			reason:      `async_gemini: upstream error status=503, body={"error":{"message":"generation timed out","type":"server_error","param":"","code":"generation_timeout"}}`,
			wantCode:    "image_timeout",
			wantCat:     "timeout",
			wantRetry:   true,
			wantStatus:  503,
			wantMessage: "图片生成耗时过长或连接中断，请稍后重试。",
		},
		{
			name:        "nano banana poll timeout with service unavailable status",
			reason:      `async_gemini: upstream error status=503, body={"error":{"message":"poll failed (transient): 408 {\"error_code\":\"timeout_error\",\"message\":\"Gateway timeout from fal-nanobanana: {'detail': 'Request timed out'}\"}","type":"server_error","param":"","code":"poll_timeout"}}`,
			wantCode:    "image_timeout",
			wantCat:     "timeout",
			wantRetry:   true,
			wantStatus:  503,
			wantMessage: "图片生成耗时过长或连接中断，请稍后重试。",
		},
		{
			name:        "nano banana safety status",
			reason:      `async_gemini: upstream error status=451, body={"error":{"message":"content policy violation: {\"error_code\":\"image_unsafe\",\"message\":\"The generated images appear to be unsafe.\"}","type":"content_policy_violation","param":"","code":"content_policy"}}`,
			wantCode:    "image_safety_blocked",
			wantCat:     "safety",
			wantRetry:   false,
			wantStatus:  451,
			wantMessage: "请求内容可能不符合安全策略，请修改提示词或图片后重试。",
		},
		{
			name:        "nano banana resolution mismatch with rate limit status",
			reason:      `async_gemini: upstream error status=429, body={"error":{"message":"resolution mismatch: want 4096x4096, got 2880x5908","type":"upstream_error","param":"","code":429}}`,
			wantCode:    "image_invalid_size",
			wantCat:     "invalid_request",
			wantRetry:   false,
			wantStatus:  429,
			wantMessage: "图片尺寸不支持，请调整为常见尺寸或比例后重试。",
		},
		{
			name:        "nano banana no active upstream tokens with rate limit status",
			reason:      `async_gemini: upstream error status=429, body={"error":{"message":"no active tokens available","type":"rate_limit_error","param":"","code":"token_unavailable"}}`,
			wantCode:    "image_provider_quota_exceeded",
			wantCat:     "provider_quota",
			wantRetry:   false,
			wantStatus:  429,
			wantMessage: "当前模型服务额度不足，请联系管理员处理。",
		},
		{
			name:        "nano banana too many input images",
			reason:      `async_gemini: upstream error status=400, body={"error":{"message":"too many input images; max is 4","type":"invalid_request_error","param":"","code":"invalid_request"}}`,
			wantCode:    "image_too_many_input_images",
			wantCat:     "invalid_request",
			wantRetry:   false,
			wantStatus:  400,
			wantMessage: "参考图片数量超出当前模型限制，请减少图片数量后重试。",
		},
		{
			name:        "safety blocked",
			reason:      "status_code=400, Your request was rejected by the safety system.",
			wantCode:    "image_safety_blocked",
			wantCat:     "safety",
			wantRetry:   false,
			wantStatus:  400,
			wantMessage: "请求内容可能不符合安全策略，请修改提示词或图片后重试。",
		},
		{
			name:        "storage failed",
			reason:      "上传图片到对象存储失败: aliyun oss upload failed",
			wantCode:    "image_storage_failed",
			wantCat:     "storage",
			wantRetry:   true,
			wantStatus:  0,
			wantMessage: "图片已生成但保存失败，请稍后重试。",
		},
		{
			name:        "bad response status",
			reason:      "上游返回错误: bad response status code 405",
			wantCode:    "image_model_unavailable",
			wantCat:     "model_unavailable",
			wantRetry:   false,
			wantStatus:  405,
			wantMessage: "当前模型暂不可用，请稍后重试或切换模型。",
		},
		{
			name:        "gpt busy status remains busy",
			reason:      "status_code=503, upstream overloaded",
			wantCode:    "image_upstream_busy",
			wantCat:     "upstream_busy",
			wantRetry:   true,
			wantStatus:  503,
			wantMessage: "当前模型服务繁忙，请稍后重试。",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message, detail := BuildFriendlyImageError(test.reason, "req-test", "task-test")
			if message != test.wantMessage {
				t.Fatalf("message = %q, want %q", message, test.wantMessage)
			}
			if detail == nil {
				t.Fatalf("detail is nil")
			}
			if detail.Code != test.wantCode || detail.Category != test.wantCat || detail.Retryable != test.wantRetry || detail.UpstreamStatus != test.wantStatus {
				t.Fatalf("detail = %#v, want code=%s category=%s retry=%v status=%d", detail, test.wantCode, test.wantCat, test.wantRetry, test.wantStatus)
			}
			if detail.RequestID != "req-test" || detail.TaskID != "task-test" {
				t.Fatalf("trace fields = (%q, %q), want req-test/task-test", detail.RequestID, detail.TaskID)
			}
		})
	}
}
