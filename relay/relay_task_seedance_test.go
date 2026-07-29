package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestBuildSeedance20VideoTaskResponseFiltersInternalFields(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		CreatedAt:  1785340426,
		FinishTime: 1785340613,
		Progress:   "100%",
		Properties: model.Properties{OriginModelName: "seedance-2.0-mini"},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://example.com/video.mp4",
		},
		Data: []byte(`{"id":"cgt-20260729235346-67rgg","status":"completed","usage":{"total_tokens":170648}}`),
	}

	response := BuildSeedance20VideoTaskResponse(task)

	assert.Equal(t, map[string]any{
		"status":       "success",
		"task_id":      "task_public",
		"video_id":     "cgt-20260729235346-67rgg",
		"model":        "seedance-2.0-mini",
		"progress":     100,
		"video_url":    "https://example.com/video.mp4",
		"created_at":   int64(1785340426),
		"completed_at": int64(1785340613),
	}, response)
	assert.NotContains(t, response, "quota")
	assert.NotContains(t, response, "channel_id")
	assert.NotContains(t, response, "data")
}
