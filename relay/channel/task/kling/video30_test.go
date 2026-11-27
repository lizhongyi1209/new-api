package kling

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUnifiedKlingRoute(t *testing.T) {
	tests := []struct {
		path, task, model string
		ok                bool
	}{
		{"/text-to-video/kling-3.0", "text-to-video", "kling-3.0", true},
		{"/kling/image-to-video/kling-3.0/123", "image-to-video", "kling-3.0", true},
		{"/omni-video/kling-3.0-omni", "omni-video", "kling-3.0-omni", true},
		{"/kling/omni-video/kling-3.0-omni", "", "", false},
		{"/omni-video/kling-3.0", "", "", false},
		{"/text-to-video/kling-3.0-omni", "", "", false},
		{"/motion-control/kling-3.0", "", "", false},
		{"/kling/omni-video/kling-4.0-flash/abc", "omni-video", "kling-4.0-flash", true},
		{"/v1/videos/text2video", "", "", false},
	}
	for _, test := range tests {
		task, model, ok := unifiedKlingRoute(test.path)
		require.Equal(t, test.ok, ok, test.path)
		require.Equal(t, test.task, task, test.path)
		require.Equal(t, test.model, model, test.path)
	}
}

func TestKling30RequestValidation(t *testing.T) {
	tests := []struct{ name, action, body, wantErr string }{
		{"text valid", "text", `{"prompt":"a cat","settings":{"multi_shot":false,"audio":"native","resolution":"4k","aspect_ratio":"9:16","duration":15},"options":{"external_task_id":"o1","watermark_info":{"enabled":true}}}`, ""},
		{"text requires prompt", "text", `{"settings":{}}`, "prompt is required"},
		{"text rejects 21:9", "text", `{"prompt":"a","settings":{"aspect_ratio":"21:9"}}`, "settings.aspect_ratio"},
		{"text rejects uppercase resolution", "text", `{"prompt":"a","settings":{"resolution":"1080P"}}`, "settings.resolution"},
		{"text rejects 16 seconds", "text", `{"prompt":"a","settings":{"duration":16}}`, "between 3 and 15"},
		{"text rejects HDR", "text", `{"prompt":"a","settings":{"dynamic_range":"hdr"}}`, "unsupported settings field \"dynamic_range\""},
		{"text rejects original audio", "text", `{"prompt":"a","settings":{"audio":"original"}}`, "settings.audio"},
		{"callback disabled", "text", `{"prompt":"a","options":{"callback_url":"https://x"}}`, CallbackDisabledMessage},
		{"image first and last frame", "image", `{"contents":[{"type":"prompt","text":"go"},{"type":"first_frame","url":"https://x/f"},{"type":"last_frame","url":"https://x/l"}],"settings":{"audio":"off","duration":5}}`, ""},
		{"image element", "image", `{"contents":[{"type":"first_frame","url":"https://x/f"},{"type":"element","element_id":123}]}`, ""},
		{"image requires first frame", "image", `{"contents":[{"type":"last_frame","url":"https://x/l"}]}`, "one first_frame"},
		{"image rejects aspect ratio", "image", `{"contents":[{"type":"first_frame","url":"https://x/f"}],"settings":{"aspect_ratio":"16:9"}}`, "unsupported settings field \"aspect_ratio\""},
		{"image rejects video", "image", `{"contents":[{"type":"first_frame","url":"https://x/f"},{"type":"feature_video","url":"https://x/v"}]}`, "unsupported content type"},
		{"image rejects four elements", "image", `{"contents":[{"type":"first_frame","url":"f"},{"type":"element","element_id":1},{"type":"element","element_id":2},{"type":"element","element_id":3},{"type":"element","element_id":4}]}`, "at most 3 elements"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var metadata map[string]any
			require.NoError(t, common.Unmarshal([]byte(test.body), &metadata))
			err := validateVideo30(metadata, test.action)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}
