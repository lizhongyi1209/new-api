package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDoubaoVideoTaskResponseExposesSuccessfulResultAndUsage(t *testing.T) {
	task := &model.Task{
		TaskID:    "task_public",
		Status:    model.TaskStatusSuccess,
		CreatedAt: 100,
		UpdatedAt: 200,
		Properties: model.Properties{
			OriginModelName: "seedance-2.0-mini",
		},
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/result.mp4"},
		Data:        []byte(`{"usage":{"completion_tokens":123,"total_tokens":456}}`),
	}

	response := BuildDoubaoVideoTaskResponse(task)

	assert.Equal(t, "task_public", response["id"])
	assert.Equal(t, "succeeded", response["status"])
	assert.Equal(t, "seedance-2.0-mini", response["model"])
	assert.Equal(t, map[string]any{"video_url": "https://example.com/result.mp4"}, response["content"])
	usage, ok := response["usage"].(struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	})
	require.True(t, ok)
	assert.Equal(t, 123, usage.CompletionTokens)
	assert.Equal(t, 456, usage.TotalTokens)
}

func TestBuildDoubaoVideoTaskResponseExposesFailure(t *testing.T) {
	task := &model.Task{TaskID: "task_failed", Status: model.TaskStatusFailure, FailReason: "provider rejected request"}

	response := BuildDoubaoVideoTaskResponse(task)

	assert.Equal(t, "failed", response["status"])
	assert.Equal(t, map[string]any{
		"code": "generation_failed", "message": "provider rejected request",
	}, response["error"])
	assert.NotContains(t, response, "content")
}
