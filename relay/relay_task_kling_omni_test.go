package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
)

func TestBuildKlingOmniVideo30TaskResponseUsesSettledPlatformCharge(t *testing.T) {
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

	response := BuildKlingOmniVideo30TaskResponse(task)

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
