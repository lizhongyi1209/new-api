package kling

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestBodyKeepsOmniActionWithoutLegacyImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/kling/v1/videos/omni-video", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt: "Make the person in <<<image_1>>> wave to the camera",
		ImageList: []relaycommon.TaskImageInfo{
			{ImageURL: "https://example.com/image.png"},
		},
		Duration: 5,
		Mode:     "pro",
		Size:     "16:9",
	})
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "kling-v3-omni",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action: constant.TaskActionOmniVideo,
		},
	}

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var payload requestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, constant.TaskActionOmniVideo, info.Action)
	assert.Empty(t, c.GetString("action"))
	assert.Equal(t, "kling-v3-omni", payload.ModelName)
	assert.Equal(t, []ImageItem{{ImageUrl: "https://example.com/image.png"}}, payload.ImageList)
}

func TestBuildRequestBodyForKling30OmniPreservesOfficialContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/kling/omni-video/kling-3.0-omni", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt: "A product rotates on a pedestal",
		Metadata: map[string]interface{}{
			"contents": []interface{}{
				map[string]interface{}{"type": "prompt", "text": "A product rotates on a pedestal"},
				map[string]interface{}{"type": "refer_image", "url": "https://example.com/product.png", "id": "image_1"},
			},
			"settings": map[string]interface{}{
				"multi_shot":   false,
				"audio":        "off",
				"resolution":   "1080p",
				"aspect_ratio": "1:1",
				"duration":     5,
			},
			"options": map[string]interface{}{
				"watermark_info": map[string]interface{}{"enabled": false},
			},
		},
	})
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "kling-3.0-omni"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: constant.TaskActionOmniVideo30},
	}

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var payload map[string]interface{}
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.NotContains(t, payload, "model_name")
	settings := payload["settings"].(map[string]interface{})
	assert.Equal(t, false, settings["multi_shot"])
	assert.Equal(t, "1:1", settings["aspect_ratio"])
	options := payload["options"].(map[string]interface{})
	assert.Equal(t, false, options["watermark_info"].(map[string]interface{})["enabled"])
}

func TestKling30OmniUpstreamPathsAndTaskResult(t *testing.T) {
	service.InitHttpClient()
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: constant.TaskActionOmniVideo30},
	}

	requestURL, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "/omni-video/kling-3.0-omni", requestURL)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tasks", r.URL.Path)
		assert.Equal(t, "upstream-task-1", r.URL.Query().Get("task_ids"))
		_, _ = io.WriteString(w, `{"code":0,"data":[{"id":"upstream-task-1","status":"succeeded","outputs":[{"type":"video","url":"https://example.com/result.mp4"}],"billing":[{"charge_type":"cash","amount":"1.25"}]}]}`)
	}))
	defer server.Close()

	resp, err := adaptor.FetchTask(server.URL, "direct-token", map[string]any{
		"task_id": "upstream-task-1",
		"action":  constant.TaskActionOmniVideo30,
	}, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	taskInfo, err := adaptor.ParseTaskResult(responseBody)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, taskInfo.Status)
	assert.Equal(t, "https://example.com/result.mp4", taskInfo.Url)
	assert.InDelta(t, 1.25, taskInfo.ActualCost, 0.0001)
	assert.False(t, strings.Contains(string(responseBody), "direct-token"))
}

func TestKling30OmniTaskQueryError(t *testing.T) {
	_, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{"code":1201,"message":"invalid task id","request_id":"request-1","data":[]}`))
	require.EqualError(t, err, "Kling task query failed: invalid task id")
}

func TestBuildRequestBodyForKling30MotionControlPreservesOfficialContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/kling/motion-control/kling-3.0", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{Metadata: map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{"type": "image", "url": "https://example.com/character.png"},
			map[string]interface{}{"type": "video", "url": "https://example.com/motion.mp4"},
		},
		"settings": map[string]interface{}{
			"character_orientation": "video",
			"audio":                 "off",
			"resolution":            "1080p",
		},
		"options": map[string]interface{}{
			"watermark_info": map[string]interface{}{"enabled": false},
		},
	}})
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "kling-3.0"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: constant.TaskActionMotionControl30},
	}

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var payload map[string]interface{}
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.NotContains(t, payload, "model")
	settings := payload["settings"].(map[string]interface{})
	assert.Equal(t, "video", settings["character_orientation"])
	assert.Equal(t, "off", settings["audio"])
	options := payload["options"].(map[string]interface{})
	assert.Equal(t, false, options["watermark_info"].(map[string]interface{})["enabled"])
}

func TestKling30MotionControlUpstreamPaths(t *testing.T) {
	service.InitHttpClient()
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: constant.TaskActionMotionControl30},
	}

	requestURL, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "/motion-control/kling-3.0", requestURL)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tasks", r.URL.Path)
		assert.Equal(t, "motion-task-1", r.URL.Query().Get("task_ids"))
		_, _ = io.WriteString(w, `{"code":0,"data":[{"id":"motion-task-1","status":"processing"}]}`)
	}))
	defer server.Close()

	resp, err := adaptor.FetchTask(server.URL, "direct-token", map[string]any{
		"task_id": "motion-task-1",
		"action":  constant.TaskActionMotionControl30,
	}, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	taskInfo, err := adaptor.ParseTaskResult(responseBody)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusInProgress, taskInfo.Status)
}

func TestValidateMotionControl30Request(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "image and video without optional prompt",
			body: `{"contents":[{"type":"image","url":"https://example.com/a.png"},{"type":"video","url":"https://example.com/a.mp4"}],"settings":{"character_orientation":"image"}}`,
		},
		{
			name:    "element requires video orientation",
			body:    `{"contents":[{"type":"element","element_id":"162","id":"role_1"},{"type":"video","url":"https://example.com/a.mp4"}],"settings":{"character_orientation":"image"}}`,
			wantErr: "element content requires video character orientation",
		},
		{
			name:    "rejects unsupported resolution",
			body:    `{"contents":[{"type":"image","url":"https://example.com/a.png"},{"type":"video","url":"https://example.com/a.mp4"}],"settings":{"character_orientation":"video","resolution":"4k"}}`,
			wantErr: "settings.resolution must be 720p or 1080p",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var req motionControl30Request
			require.NoError(t, common.Unmarshal([]byte(test.body), &req))
			err := validateMotionControl30Request(req)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, test.wantErr)
		})
	}
}
