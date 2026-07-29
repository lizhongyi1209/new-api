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
