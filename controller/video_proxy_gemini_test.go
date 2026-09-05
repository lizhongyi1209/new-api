package controller

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	geminitask "github.com/QuantumNous/new-api/relay/channel/task/gemini"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractGeminiOmniVideoURL(t *testing.T) {
	payload := []byte(`{"id":"video-123","steps":[{"type":"model_output","content":[{"type":"video","uri":"https://download-vod.aig-ai.com/result.mp4"}]}]}`)
	assert.Equal(t, "https://download-vod.aig-ai.com/result.mp4", extractGeminiVideoURLFromPayload(payload))
}

func TestGeminiOmniVideoURLDoesNotExposeAPIKey(t *testing.T) {
	task := &model.Task{
		Data: json.RawMessage(`{"steps":[{"content":[{"type":"video","uri":"https://download-vod.aig-ai.com/result.mp4"}]}]}`),
		Properties: model.Properties{
			OriginModelName:   geminitask.GeminiOmniFlashPreviewModel,
			UpstreamModelName: geminitask.GeminiOmniFlashPreviewModel,
		},
	}
	channel := &model.Channel{Type: constant.ChannelTypeGemini}

	videoURL, err := getGeminiVideoURL(channel, task, "secret-upstream-key")
	require.NoError(t, err)
	assert.Equal(t, "https://download-vod.aig-ai.com/result.mp4", videoURL)
	assert.NotContains(t, videoURL, "secret-upstream-key")
}

func TestGeminiVideoURLPrefersStoredOutput(t *testing.T) {
	task := &model.Task{
		TaskID: "task-stored-output",
		Data:   json.RawMessage(`{"response":{"video":"https://upstream.example.com/result.mp4"}}`),
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://media.example.com/output/stored.mp4",
		},
	}
	channel := &model.Channel{Type: constant.ChannelTypeGemini}

	videoURL, err := getGeminiVideoURL(channel, task, "secret-upstream-key")

	require.NoError(t, err)
	assert.Equal(t, "https://media.example.com/output/stored.mp4", videoURL)
	assert.NotContains(t, videoURL, "secret-upstream-key")
}
