package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsResponsesEventStreamContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{name: "plain", contentType: "text/event-stream", want: true},
		{name: "mixed case with charset", contentType: "Text/Event-Stream; charset=utf-8", want: true},
		{name: "json", contentType: "application/json", want: false},
		{name: "empty", contentType: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isResponsesEventStreamContentType(tt.contentType))
		})
	}
}

func TestApplySystemPromptSkipsKimiDynamicToolMessage(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	info.ChannelSetting.SystemPrompt = "site prompt"
	info.ChannelSetting.SystemPromptOverride = true
	request := &dto.GeneralOpenAIRequest{
		Model: "kimi-k3",
		Messages: []dto.Message{
			{Role: "system", Tools: []byte(`[{"type":"function","function":{"name":"lookup"}}]`)},
			{Role: "system", Content: "client prompt"},
		},
	}

	applySystemPromptIfNeeded(c, info, request)

	require.Len(t, request.Messages, 2)
	assert.Nil(t, request.Messages[0].Content)
	assert.Equal(t, "site prompt\nclient prompt", request.Messages[1].Content)
}
