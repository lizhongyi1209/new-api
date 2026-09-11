package ollama

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaRequestURLs(t *testing.T) {
	tests := []struct {
		name        string
		relayFormat types.RelayFormat
		relayMode   int
		want        string
	}{
		{name: "chat", relayMode: relayconstant.RelayModeChatCompletions, want: "http://ollama/api/chat"},
		{name: "completions", relayMode: relayconstant.RelayModeCompletions, want: "http://ollama/api/generate"},
		{name: "responses", relayMode: relayconstant.RelayModeResponses, want: "http://ollama/v1/responses"},
		{name: "responses compact", relayMode: relayconstant.RelayModeResponsesCompact, want: "http://ollama/v1/responses/compact"},
		{name: "claude messages", relayFormat: types.RelayFormatClaude, want: "http://ollama/v1/messages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestURL, err := (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "http://ollama"},
				RelayFormat: tt.relayFormat,
				RelayMode:   tt.relayMode,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, requestURL)
		})
	}
}

func TestOllamaPreservesNativeClaudeRequestAndHeaders(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "ollama-key"},
	}
	request := &dto.ClaudeRequest{Model: "qwen3"}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, request)
	require.NoError(t, err)
	assert.Same(t, request, converted)

	header := http.Header{}
	require.NoError(t, (&Adaptor{}).SetupRequestHeader(c, &header, info))
	assert.Equal(t, "2023-06-01", header.Get("anthropic-version"))
	assert.Equal(t, "prompt-caching-2024-07-31", header.Get("anthropic-beta"))
}
