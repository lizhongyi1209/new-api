package kling

import (
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
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
		_, _ = io.WriteString(w, `{"code":0,"data":[{"id":"upstream-task-1","status":"succeeded","outputs":[{"type":"video","url":"https://example.com/result.mp4"}],"billing":[{"charge_type":"unit","amount":"3"}]}]}`)
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
	assert.InDelta(t, 3, taskInfo.ActualCost, 0.0001)
	assert.False(t, strings.Contains(string(responseBody), "direct-token"))

	task := &model.Task{Quota: 250000}
	task.PrivateData.BillingContext = &model.TaskBillingContext{GroupRatio: 1}
	expectedQuota := common.QuotaFromFloat(math.Ceil(3 * common.QuotaPerUnit / operation_setting.USDExchangeRate))
	assert.Equal(t, expectedQuota, adaptor.AdjustBillingOnComplete(task, taskInfo))
}

func TestKling30OmniTaskResultSumsCashAndUnitDeductions(t *testing.T) {
	taskInfo, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{"code":0,"data":[{"id":"upstream-task-1","status":"succeeded","outputs":[{"type":"video","url":"https://example.com/result.mp4"}],"billing":[{"charge_type":"cash","amount":"1.25"},{"charge_type":"unit","amount":"2"}]}]}`))
	require.NoError(t, err)
	assert.InDelta(t, 3.25, taskInfo.ActualCost, 0.0001)
}

func TestKling30OmniTaskQueryError(t *testing.T) {
	_, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{"code":1201,"message":"invalid task id","request_id":"request-1","data":[]}`))
	require.EqualError(t, err, "Kling task query failed: invalid task id")
}
