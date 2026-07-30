package relay

import (
	"testing"

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
	assert.Equal(t, float64(100), response["progress"])
	assert.NotContains(t, video, "respect_moderation")
	assert.NotContains(t, response, "usage")
	assert.NotContains(t, response, "request_id")
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
