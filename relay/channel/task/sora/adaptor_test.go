package sora

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestConvertToOpenAIVideoPreservesProviderFields(t *testing.T) {
	task := &model.Task{
		TaskID:    "video_public",
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		CreatedAt: 100,
		UpdatedAt: 200,
		Properties: model.Properties{
			OriginModelName: "sora-2",
		},
		Data: []byte(`{"id":"upstream","task_id":"upstream","status":"queued","provider_trace":"trace-1","url":"https://example.com/video.mp4"}`),
	}

	encoded, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	assert.Equal(t, "video_public", gjson.GetBytes(encoded, "id").String())
	assert.Equal(t, "video_public", gjson.GetBytes(encoded, "task_id").String())
	assert.Equal(t, "video", gjson.GetBytes(encoded, "object").String())
	assert.Equal(t, "sora-2", gjson.GetBytes(encoded, "model").String())
	assert.Equal(t, "completed", gjson.GetBytes(encoded, "status").String())
	assert.Equal(t, int64(100), gjson.GetBytes(encoded, "progress").Int())
	assert.Equal(t, int64(100), gjson.GetBytes(encoded, "created_at").Int())
	assert.Equal(t, int64(200), gjson.GetBytes(encoded, "completed_at").Int())
	assert.Equal(t, "trace-1", gjson.GetBytes(encoded, "provider_trace").String())
	assert.Equal(t, "https://example.com/video.mp4", gjson.GetBytes(encoded, "url").String())
}
