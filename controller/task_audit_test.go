package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTaskAuditReturnsSanitizedRetainedData(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Task{}))

	task := &model.Task{
		TaskID:    "task-public-audit",
		Platform:  constant.TaskPlatform("xai"),
		UserId:    1,
		ChannelId: 12,
		Action:    constant.TaskActionGenerate,
		Status:    model.TaskStatusSuccess,
		Properties: model.Properties{
			OriginModelName: "grok-imagine-video-1.5",
		},
		PrivateData: model.TaskPrivateData{
			RequestID:      "request-gateway",
			UpstreamTaskID: "request-upstream",
			RequestSnapshot: &model.TaskRequestSnapshot{
				SchemaVersion:       1,
				Prompt:              "animate the subject",
				ResolutionEffective: "480p",
				ResolutionDefaulted: true,
			},
			SubmitResponse: []byte(`{"request_id":"request-upstream"}`),
			BillingContext: &model.TaskBillingContext{
				ModelPrice:  0.05,
				GroupRatio:  1,
				OtherRatios: map[string]float64{"resolution": 1, "seconds": 8},
			},
		},
		Data: []byte(`{"status":"done","authorization":"Bearer secret","usage":{"cost_in_usd_ticks":50000000}}`),
	}
	require.NoError(t, db.Create(task).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/task/"+task.TaskID+"/audit", nil)

	GetTaskAudit(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, task.TaskID, response.Data["task_id"])
	assert.Equal(t, "request-upstream", response.Data["upstream_task_id"])
	request := response.Data["request"].(map[string]interface{})
	assert.Equal(t, "480p", request["resolution_effective"])
	responses := response.Data["responses"].(map[string]interface{})
	submitResponse := responses["submit"].(map[string]interface{})
	assert.Equal(t, "request-upstream", submitResponse["request_id"])
	finalResponse := responses["final"].(map[string]interface{})
	assert.Equal(t, "[redacted]", finalResponse["authorization"])
	assert.Equal(t, float64(50_000_000), finalResponse["usage"].(map[string]interface{})["cost_in_usd_ticks"])
}
