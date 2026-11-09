package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
)

func TestBuildKlingVideo30TaskResponseUsesSettledPlatformCharge(t *testing.T) {
	originalExchangeRate := operation_setting.USDExchangeRate
	operation_setting.USDExchangeRate = 1
	t.Cleanup(func() { operation_setting.USDExchangeRate = originalExchangeRate })

	task := &model.Task{
		TaskID: "task_public",
		Status: model.TaskStatusSuccess,
		Quota:  common.QuotaFromFloat(3.825 * common.QuotaPerUnit),
		Properties: model.Properties{
			OriginModelName: "kling-3.0-omni",
		},
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/result.mp4"},
		Data:        []byte(`{"code":0,"request_id":"request-upstream","data":[{"billing":[{"amount":"4.5","charge_type":"unit"}],"outputs":[{"type":"video","url":"https://example.com/result.mp4","duration":"5.041"}]}]}`),
	}

	response := BuildKlingVideo30TaskResponse(task)

	assert.Equal(t, "task_public", response["task_id"])
	assert.Equal(t, "SUCCESS", response["status"])
	assert.Equal(t, "https://example.com/result.mp4", response["video_url"])
	assert.Equal(t, 5.041, response["duration"])
	assert.Equal(t, "kling-3.0-omni", response["model"])
	assert.Equal(t, "3.825", response["cost"])
	assert.Equal(t, "request-upstream", response["request_id"])
	assert.NotEqual(t, "4.5", response["cost"])
	assert.Len(t, response, 7)
}

func TestBuildKlingMotionControl30TaskResponseHidesInternalData(t *testing.T) {
	originalExchangeRate := operation_setting.USDExchangeRate
	operation_setting.USDExchangeRate = 1
	t.Cleanup(func() { operation_setting.USDExchangeRate = originalExchangeRate })

	task := &model.Task{
		ID:        1220052,
		TaskID:    "task_public",
		UserId:    1,
		ChannelId: 343,
		Status:    model.TaskStatusSuccess,
		Quota:     common.QuotaFromFloat(4.5 * common.QuotaPerUnit),
		Properties: model.Properties{
			OriginModelName: "kling-3.0",
		},
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/motion.mp4"},
		Data:        []byte(`{"code":0,"request_id":"request-motion","data":[{"billing":[{"amount":"4.5","charge_type":"unit"}],"outputs":[{"type":"video","url":"https://example.com/motion.mp4","duration":"4.966"}]}]}`),
	}

	response := BuildKlingVideo30TaskResponse(task)

	assert.Equal(t, "task_public", response["task_id"])
	assert.Equal(t, "SUCCESS", response["status"])
	assert.Equal(t, "https://example.com/motion.mp4", response["video_url"])
	assert.Equal(t, 4.966, response["duration"])
	assert.Equal(t, "kling-3.0", response["model"])
	assert.Equal(t, "4.5", response["cost"])
	assert.Equal(t, "request-motion", response["request_id"])
	assert.NotContains(t, response, "user_id")
	assert.NotContains(t, response, "channel_id")
	assert.NotContains(t, response, "quota")
	assert.NotContains(t, response, "data")
	assert.Len(t, response, 7)
}

func TestKling40OfficialTaskResponseUsesUpstreamFields(t *testing.T) {
	task := &model.Task{
		TaskID:         "task_public",
		ExternalTaskID: "caller-1",
		Status:         model.TaskStatusSuccess,
		SubmitTime:     1781080778,
		Properties:     model.Properties{OriginModelName: "kling-4.0"},
		PrivateData:    model.TaskPrivateData{ResultURL: "https://example.com/result.mp4"},
		Data:           []byte(`{"request_id":"request-40","data":[{"id":"upstream-1","status":"succeeded","create_time":1781080778802,"update_time":1781080794151,"outputs":[{"type":"video","id":"vid","url":"https://example.com/result.mp4","duration":"10.2"}],"usage":{"input_duration":"8.03","output_duration":"10"},"billing":[{"charge_type":"cash","amount":"1.57","currency":"USD"}]}]}`),
	}
	response := KlingOfficialTaskList("query-1", []*model.Task{task})
	assert.Equal(t, 0, response["code"])
	assert.Equal(t, "query-1", response["request_id"])
	items := response["data"].([]any)
	requireItem := items[0].(map[string]any)
	assert.Equal(t, "task_public", requireItem["id"])
	assert.Equal(t, "succeeded", requireItem["status"])
	assert.Equal(t, "caller-1", requireItem["external_id"])
	assert.Equal(t, int64(1781080778802), requireItem["create_time"])
	usage := requireItem["usage"].(map[string]any)
	assert.Equal(t, "8.03", usage["input_duration"])
	assert.Equal(t, "10", usage["output_duration"])
	assert.NotContains(t, requireItem, "task_id")
	assert.NotContains(t, requireItem, "cost")
	assert.Len(t, requireItem["billing"].([]any), 1)
}
