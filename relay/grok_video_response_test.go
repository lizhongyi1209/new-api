package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGrokVideoTaskResponse(t *testing.T) {
	task := &model.Task{
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://assetcache.o1key.com/output/cached-video.mp4",
		},
		Properties: model.Properties{
			OriginModelName: "grok-imagine-video",
		},
		Data: []byte(`{"request_id":"upstream-secret","status":"done","video":{"url":"https://vidgen.x.ai/video.mp4","duration":8,"respect_moderation":true,"file_output":{"file_id":"file-video"}},"usage":{"cost_in_usd_ticks":500000000},"progress":100}`),
	}

	response := BuildGrokVideoTaskResponse(task)
	assert.Equal(t, "done", response["status"])
	assert.Equal(t, "grok-imagine-video", response["model"])
	video, ok := response["video"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://assetcache.o1key.com/output/cached-video.mp4", video["url"])
	assert.Equal(t, float64(8), video["duration"])
	assert.Equal(t, map[string]any{"file_id": "file-video"}, video["file_output"])
	assert.Equal(t, true, video["respect_moderation"])
	assert.Equal(t, float64(100), response["progress"])
	assert.NotContains(t, response, "usage")
	assert.NotContains(t, response, "request_id")
}

func TestBuildGrokVideoTaskResponseFromServiceInferenceWrapper(t *testing.T) {
	task := &model.Task{
		Status:   model.TaskStatusSuccess,
		Progress: "100%",
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://cache.example/video.mp4",
		},
		Properties: model.Properties{
			OriginModelName: "grok-imagine-video-1.5",
		},
		Data: []byte(`{
			"task": {
				"id":"mvt-upstream",
				"status":"completed",
				"duration_seconds":5,
				"outputs":["https://upstream.example/video.mp4"],
				"metadata": {
					"progress":100,
					"video":{"url":"https://upstream.example/video.mp4","duration":5.04,"respect_moderation":true},
					"usage":{"cost_in_usd_ticks":7100000000}
				}
			}
		}`),
	}

	response := BuildGrokVideoTaskResponse(task)
	assert.Equal(t, "done", response["status"])
	assert.Equal(t, "grok-imagine-video-1.5", response["model"])
	assert.Equal(t, float64(100), response["progress"])
	video, ok := response["video"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://cache.example/video.mp4", video["url"])
	assert.Equal(t, float64(5.04), video["duration"])
	assert.Equal(t, true, video["respect_moderation"])
	assert.NotContains(t, response, "task")
	assert.NotContains(t, response, "usage")
}

func TestBuildGrokVideoTaskResponsePreservesExpired(t *testing.T) {
	task := &model.Task{
		Status:     model.TaskStatusFailure,
		FailReason: "expired",
		Data:       []byte(`{"status":"expired"}`),
	}

	response := BuildGrokVideoTaskResponse(task)
	assert.Equal(t, "expired", response["status"])
}

func TestTaskModel2DtoHidesGrokUpstreamDataFromUsers(t *testing.T) {
	task := &model.Task{
		TaskID: "task-public",
		Status: model.TaskStatusSuccess,
		Properties: model.Properties{
			OriginModelName: "grok-imagine-video-1.5",
		},
		PrivateData: model.TaskPrivateData{ResultURL: "https://cache.example/video.mp4"},
		Data:        []byte(`{"request_id":"upstream-secret","status":"done","video":{"url":"https://upstream.example/video.mp4","respect_moderation":true},"usage":{"cost_in_usd_ticks":50000000}}`),
	}

	userDto := TaskModel2Dto(task, false)
	var userData map[string]any
	require.NoError(t, common.Unmarshal(userDto.Data, &userData))
	assert.Equal(t, "done", userData["status"])
	assert.NotContains(t, userData, "request_id")
	assert.NotContains(t, userData, "usage")

	adminDto := TaskModel2Dto(task, true)
	assert.JSONEq(t, string(task.Data), string(adminDto.Data))
}
