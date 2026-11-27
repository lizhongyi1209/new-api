package kling

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestKlingImage40Route(t *testing.T) {
	tests := []struct {
		path, action string
		ok           bool
	}{
		{"/image-generation/kling-4.0", constant.TaskActionKlingImage40, true},
		{"/kling/image-generation/kling-image-4.0/task_1", constant.TaskActionKlingImage40, true},
		{"/long-image-generation/kling-4.0", constant.TaskActionKlingLongImage40, true},
		{"/image-generation/kling-4.0-flash", "", false},
		{"/text-to-video/kling-4.0", "", false},
	}
	for _, test := range tests {
		action, ok := klingImage40Route(test.path)
		require.Equal(t, test.ok, ok, test.path)
		require.Equal(t, test.action, action, test.path)
	}
}

func TestKlingImage40RequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		long    bool
		body    string
		wantErr string
	}{
		{"single valid", false, `{"contents":[{"type":"prompt","text":"dog"},{"type":"refer_image","url":"https://x/a.png","id":"dog"}],"settings":{"result_type":"single","n":9,"resolution":"8k","aspect_ratio":"auto"},"options":{"external_task_id":"o1"}}`, ""},
		{"series auto", false, `{"contents":[{"type":"prompt","text":"dog"}],"settings":{"result_type":"series","series_amount":"auto"}}`, ""},
		{"element reference", false, `{"contents":[{"type":"prompt","text":"dog"},{"type":"element","element_id":12}]}`, ""},
		{"prompt required", false, `{"contents":[{"type":"refer_image","url":"https://x/a.png"}]}`, "exactly one prompt"},
		{"n out of range", false, `{"contents":[{"type":"prompt","text":"d"}],"settings":{"n":10}}`, "settings.n"},
		{"series out of range", false, `{"contents":[{"type":"prompt","text":"d"}],"settings":{"series_amount":1}}`, "settings.series_amount"},
		{"uppercase resolution", false, `{"contents":[{"type":"prompt","text":"d"}],"settings":{"resolution":"2K"}}`, "settings.resolution"},
		{"eleven references", false, `{"contents":[{"type":"prompt","text":"d"},{"type":"refer_image","url":"1"},{"type":"refer_image","url":"2"},{"type":"refer_image","url":"3"},{"type":"refer_image","url":"4"},{"type":"refer_image","url":"5"},{"type":"refer_image","url":"6"},{"type":"refer_image","url":"7"},{"type":"refer_image","url":"8"},{"type":"refer_image","url":"9"},{"type":"refer_image","url":"10"},{"type":"element","element_id":1}]}`, "at most 10"},
		{"callback disabled", false, `{"contents":[{"type":"prompt","text":"d"}],"options":{"callback_url":"https://x"}}`, CallbackDisabledMessage},
		{"ecommerce valid", true, `{"contents":[{"type":"prompt","text":"p"},{"type":"refer_image","url":"https://x/a.png"}],"settings":{"skill":"ecommerce","length":"long","resolution":"2k"}}`, ""},
		{"ecommerce needs reference", true, `{"contents":[{"type":"prompt","text":"p"}],"settings":{"skill":"ecommerce"}}`, "1 to 3 refer_image"},
		{"webpage style and subject", true, `{"contents":[{"type":"prompt","text":"p"},{"type":"style_refer_image","url":"s"},{"type":"subject_refer_image","url":"t"}],"settings":{"skill":"webpage"}}`, ""},
		{"webpage rejects refer_image", true, `{"contents":[{"type":"prompt","text":"p"},{"type":"refer_image","url":"s"}],"settings":{"skill":"webpage"}}`, "not refer_image"},
		{"long requires skill", true, `{"contents":[{"type":"prompt","text":"p"}]}`, "settings.skill"},
		{"long rejects element", true, `{"contents":[{"type":"prompt","text":"p"},{"type":"element","element_id":1}],"settings":{"skill":"webpage"}}`, "unsupported content type"},
		{"long rejects 4k", true, `{"contents":[{"type":"prompt","text":"p"}],"settings":{"skill":"webpage","resolution":"4k"}}`, "settings.resolution"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var metadata map[string]any
			require.NoError(t, common.Unmarshal([]byte(test.body), &metadata))
			err := validateImage40(metadata, test.long)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}
