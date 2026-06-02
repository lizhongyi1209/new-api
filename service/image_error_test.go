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
