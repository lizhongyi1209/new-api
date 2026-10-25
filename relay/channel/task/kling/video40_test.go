package kling

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKling40RequestValidation(t *testing.T) {
	require.NoError(t, validateVideo40Prompt(strings.Repeat("a", 26000)))
	require.ErrorContains(t, validateVideo40Prompt(strings.Repeat("中", 11001)), "character limit")

	tests := []struct {
		name    string
		body    string
		action  string
		wantErr string
	}{
		{"text valid", `{"prompt":"A lake at dawn","settings":{"duration":30,"resolution":"1080p","aspect_ratio":"21:9"}}`, "text", ""},
		{"text adaptive ratio", `{"prompt":"A lake at dawn","settings":{"aspect_ratio":"adaptive"}}`, "text", "settings.aspect_ratio adaptive requires omni-video"},
		{"text duration overflow", `{"prompt":"A lake at dawn","settings":{"duration":31}}`, "text", "settings.duration must be between 3 and 30 seconds"},
		{"image multiple keyframes", `{"contents":[{"type":"first_frame","url":"https://example.com/first.png"},{"type":"middle_frame","url":"https://example.com/middle.png","id":"middle"},{"type":"last_frame","url":"https://example.com/last.png"}]}`, "image", ""},
		{"image requires image", `{"contents":[{"type":"prompt","text":"A lake at dawn"}]}`, "image", "image-to-video requires at least one image"},
		{"image rejects video", `{"contents":[{"type":"feature_video","url":"https://example.com/a.mp4"}]}`, "image", "feature_video requires omni-video"},
		{"omni accepts voice and video", `{"contents":[{"type":"prompt","text":"Use @voice"},{"type":"feature_video","url":"https://example.com/a.mp4"},{"type":"voice","voice_id":"123","id":"voice"}],"settings":{"aspect_ratio":"adaptive"}}`, "omni", ""},
		{"omni duplicate ids", `{"contents":[{"type":"first_frame","url":"https://example.com/a.png","id":"same"},{"type":"voice","voice_id":"123","id":"same"}]}`, "omni", "duplicate content id \"same\""},
		{"omni requires ids for multiple references", `{"contents":[{"type":"refer_image","url":"https://example.com/a.png"},{"type":"refer_image","url":"https://example.com/b.png"}]}`, "omni", "id is required for multiple refer_image inputs"},
		{"omni rejects sixth video", `{"contents":[{"type":"feature_video","url":"1"},{"type":"feature_video","url":"2"},{"type":"feature_video","url":"3"},{"type":"feature_video","url":"4"},{"type":"feature_video","url":"5"},{"type":"feature_video","url":"6"}]}`, "omni", "contents exceeds Kling 4.0 material limits"},
		{"reject legacy audio setting", `{"prompt":"A lake at dawn","settings":{"audio":"native"}}`, "text", "unsupported settings field \"audio\""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var metadata map[string]any
			require.NoError(t, common.Unmarshal([]byte(test.body), &metadata))
			err := validateVideo40Shape(metadata, test.action)
			if err == nil {
				if test.action == "text" {
					var req video40TextRequest
					require.NoError(t, common.Unmarshal([]byte(test.body), &req))
					err = validateVideo40Prompt(req.Prompt)
					if err == nil {
						err = validateVideo40Settings(req.Settings, "text")
					}
				} else {
					var req video40ContentsRequest
					require.NoError(t, common.Unmarshal([]byte(test.body), &req))
					err = validateVideo40Contents(req, test.action == "image")
				}
			}
			if test.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, test.wantErr)
			}
		})
	}
}

func TestKling40PreservesExplicitFalseAndOfficialPath(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/kling/omni-video/kling-4.0", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{Metadata: map[string]any{
		"contents": []any{map[string]any{"type": "voice", "voice_id": "123", "id": "v"}},
		"settings": map[string]any{"duration": 30, "aspect_ratio": "adaptive"},
		"options":  map[string]any{"external_task_id": "", "watermark_info": map[string]any{"enabled": false}},
	}})
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: constant.TaskActionKling40Omni}, ChannelMeta: &relaycommon.ChannelMeta{}}
	adaptor := &TaskAdaptor{}
	requestURL, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "/omni-video/kling-4.0", requestURL)

	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, common.Unmarshal(data, &body))
	assert.NotContains(t, body, "model")
	assert.Equal(t, "", body["options"].(map[string]any)["external_task_id"])
	assert.Equal(t, false, body["options"].(map[string]any)["watermark_info"].(map[string]any)["enabled"])
	assert.Equal(t, "123", body["contents"].([]any)[0].(map[string]any)["voice_id"])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tasks", r.URL.Path)
		assert.Equal(t, "upstream-1", r.URL.Query().Get("task_ids"))
		_, _ = io.WriteString(w, `{"code":0,"data":[{"id":"upstream-1","status":"succeeded","outputs":[{"type":"video","url":"https://example.com/result.mp4"}],"usage":{"input_duration":"8.03"},"billing":[{"charge_type":"unit","amount":"1.5"}]}]}`)
	}))
	defer server.Close()
	resp, err := adaptor.FetchTask(server.URL, "direct-token", map[string]any{"task_id": "upstream-1", "action": constant.TaskActionKling40Omni}, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	response, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	taskInfo, err := adaptor.ParseTaskResult(response)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/result.mp4", taskInfo.Url)
	assert.InDelta(t, 1.5, taskInfo.ActualCost, 0.0001)
	assert.False(t, strings.Contains(string(response), "direct-token"))
}

func TestKling40UsesDistinctOfficialSubmissionPaths(t *testing.T) {
	for _, test := range []struct {
		action string
		path   string
	}{
		{constant.TaskActionKling40Text, "/text-to-video/kling-4.0"},
		{constant.TaskActionKling40Image, "/image-to-video/kling-4.0"},
		{constant.TaskActionKling40Omni, "/omni-video/kling-4.0"},
	} {
		info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: test.action}, ChannelMeta: &relaycommon.ChannelMeta{}}
		path, err := (&TaskAdaptor{}).BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, test.path, path)
	}
}

func TestKling40RejectsInvalidDurationBeforeSubmission(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/kling/omni-video/kling-4.0", strings.NewReader(`{"model":"kling-4.0","prompt":"A lake at dawn","metadata":{"contents":[{"type":"prompt","text":"A lake at dawn"}],"settings":{"duration":31}}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Contains(t, taskErr.Message, "duration")
	assert.Equal(t, constant.TaskActionKling40Omni, info.Action)
}
