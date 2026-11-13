package ali

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToAliRequestWan30(t *testing.T) {
	for _, tc := range []struct {
		name        string
		req         relaycommon.TaskSubmitReq
		wantMedia   []AliVideoMedia
		wantRes     string
		wantSeconds int
		wantJSON    []string
		denyJSON    []string
	}{
		{
			name:        "text to video uses the official defaults",
			req:         relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "a cat"},
			wantRes:     "1080P",
			wantSeconds: 5,
			wantJSON:    []string{`"prompt_extend":true`},
			denyJSON:    []string{`"media"`, `"img_url"`, `"watermark"`, `"seed"`},
		},
		{
			name: "explicit false and zero values reach the upstream",
			req: relaycommon.TaskSubmitReq{Model: "wan3.0-video-prime", Prompt: "a cat", Size: "480p", Metadata: map[string]any{
				"parameters": map[string]any{"prompt_extend": false, "watermark": false, "seed": 0, "ratio": "21:9", "duration": -1, "audio": false},
			}},
			wantRes:     "480P",
			wantSeconds: -1,
			wantJSON:    []string{`"prompt_extend":false`, `"watermark":false`, `"seed":0`, `"ratio":"21:9"`, `"duration":-1`, `"audio":false`},
		},
		{
			name:        "top-level resolution is honored",
			req:         relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "a cat", Resolution: "720p"},
			wantRes:     "720P",
			wantSeconds: 5,
		},
		{
			name:        "images become first and last frames",
			req:         relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "a cat", Images: []string{"https://example.com/a.png", "https://example.com/b.png"}, Duration: 8},
			wantMedia:   []AliVideoMedia{{Type: "first_frame", URL: "https://example.com/a.png"}, {Type: "last_frame", URL: "https://example.com/b.png"}},
			wantRes:     "1080P",
			wantSeconds: 8,
			denyJSON:    []string{`"img_url"`, `"first_frame_url"`, `"last_frame_url"`},
		},
		{
			name: "explicit reference media is kept",
			req: relaycommon.TaskSubmitReq{Model: "wan3.0-video", Prompt: "图1 runs", Size: "1280*720", Image: "https://example.com/ignored.png", Metadata: map[string]any{
				"input": map[string]any{"media": []any{
					map[string]any{"type": "reference_image", "url": "https://example.com/girl.jpg"},
					map[string]any{"type": "reference_audio", "url": "https://example.com/voice.mp3"},
				}},
			}},
			wantMedia:   []AliVideoMedia{{Type: "reference_image", URL: "https://example.com/girl.jpg"}, {Type: "reference_audio", URL: "https://example.com/voice.mp3"}},
			wantRes:     "720P",
			wantSeconds: 5,
			denyJSON:    []string{`"img_url"`, `"size"`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			aliReq, err := (&TaskAdaptor{}).convertToAliRequest(testRelayInfo(), tc.req)
			require.NoError(t, err)
			assert.Equal(t, tc.wantMedia, aliReq.Input.Media)
			assert.Equal(t, tc.wantRes, aliReq.Parameters.Resolution)
			assert.Equal(t, tc.wantSeconds, aliReq.Parameters.Duration)
			body, err := common.Marshal(aliReq)
			require.NoError(t, err)
			for _, fragment := range tc.wantJSON {
				assert.Contains(t, string(body), fragment)
			}
			for _, fragment := range tc.denyJSON {
				assert.NotContains(t, string(body), fragment)
			}
		})
	}
}

func TestConvertToAliRequestWan30RejectsUnsupportedInput(t *testing.T) {
	media := func(types ...string) map[string]any {
		items := make([]any, 0, len(types))
		for _, mediaType := range types {
			items = append(items, map[string]any{"type": mediaType, "url": "https://example.com/" + mediaType})
		}
		return map[string]any{"input": map[string]any{"media": items}}
	}
	for _, tc := range []struct {
		name string
		req  relaycommon.TaskSubmitReq
	}{
		{name: "duration below minimum", req: relaycommon.TaskSubmitReq{Duration: 1}},
		{name: "duration above maximum", req: relaycommon.TaskSubmitReq{Duration: 31}},
		{name: "metadata duration out of range", req: relaycommon.TaskSubmitReq{Metadata: map[string]any{"parameters": map[string]any{"duration": 120}}}},
		{name: "unsupported resolution", req: relaycommon.TaskSubmitReq{Size: "4k"}},
		{name: "document media", req: relaycommon.TaskSubmitReq{Metadata: media("file")}},
		{name: "frames with references", req: relaycommon.TaskSubmitReq{Metadata: media("first_frame", "reference_image")}},
		{name: "last frame without first frame", req: relaycommon.TaskSubmitReq{Metadata: media("last_frame")}},
		{name: "two first frames", req: relaycommon.TaskSubmitReq{Metadata: media("first_frame", "first_frame")}},
		{name: "legacy audio url", req: relaycommon.TaskSubmitReq{Metadata: map[string]any{"input": map[string]any{"audio_url": "https://example.com/a.mp3"}}}},
		{name: "prompt cleared by metadata", req: relaycommon.TaskSubmitReq{Metadata: map[string]any{"input": map[string]any{"prompt": " "}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.Model = "wan3.0-video"
			if tc.req.Prompt == "" {
				tc.req.Prompt = "a cat"
			}
			_, err := (&TaskAdaptor{}).convertToAliRequest(testRelayInfo(), tc.req)
			assert.Error(t, err)
		})
	}
}

func TestConvertToOpenAIVideoWan30(t *testing.T) {
	upstreamURL := "https://dashscope-result.oss-cn-beijing.aliyuncs.com/video.mp4?Expires=1"
	for _, tc := range []struct {
		name      string
		task      *model.Task
		wantURL   string
		wantError *dto.OpenAIVideoError
	}{
		{
			name: "passthrough keeps the upstream URL",
			task: &model.Task{TaskID: "task_public", Status: model.TaskStatusSuccess, Progress: "100%", FinishTime: 260,
				PrivateData: model.TaskPrivateData{ResultURL: upstreamURL},
				Data:        []byte(`{"output":{"task_status":"SUCCEEDED","video_url":"` + upstreamURL + `"},"usage":{"output_video_duration":5,"SR":720}}`)},
			wantURL: upstreamURL,
		},
		{
			name: "oss returns the cached copy",
			task: &model.Task{TaskID: "task_public", Status: model.TaskStatusSuccess, Progress: "100%", FinishTime: 260,
				PrivateData: model.TaskPrivateData{ResultURL: "https://oss.example.com/output/v.mp4"},
				Data:        []byte(`{"output":{"task_status":"SUCCEEDED","video_url":"` + upstreamURL + `"},"usage":{"output_video_duration":5,"SR":720}}`)},
			wantURL: "https://oss.example.com/output/v.mp4",
		},
		{
			name: "failure reports the upstream code",
			task: &model.Task{TaskID: "task_public", Status: model.TaskStatusFailure, FailReason: "task failed, code: DataInspectionFailed",
				Data: []byte(`{"output":{"task_status":"FAILED","code":"DataInspectionFailed","message":"inspection failed"}}`)},
			wantError: &dto.OpenAIVideoError{Code: "DataInspectionFailed", Message: "inspection failed"},
		},
		{
			name:      "failure without upstream data",
			task:      &model.Task{TaskID: "task_public", Status: model.TaskStatusFailure, FailReason: "task failed"},
			wantError: &dto.OpenAIVideoError{Code: "upstream_error", Message: "task failed"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(tc.task)
			require.NoError(t, err)
			var video dto.OpenAIVideo
			require.NoError(t, common.Unmarshal(body, &video))
			assert.Equal(t, "task_public", video.ID)
			assert.Equal(t, tc.wantURL, video.ResultURL)
			assert.Equal(t, tc.wantError, video.Error)
			if tc.wantError == nil {
				assert.Equal(t, "5", video.Seconds)
				assert.Equal(t, "720P", video.Size)
				assert.Equal(t, int64(260), video.CompletedAt)
			}
			if tc.wantURL != upstreamURL {
				assert.NotContains(t, string(body), "aliyuncs.com")
			}
		})
	}
}
