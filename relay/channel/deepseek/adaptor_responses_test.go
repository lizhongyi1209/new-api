package deepseek

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLSupportsResponses(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.deepseek.com",
		},
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://api.deepseek.com/responses", requestURL)
}

func TestConvertOpenAIResponsesRequestAppliesDeepSeekV4Suffix(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		wantModel  string
		wantEffort string
	}{
		{
			name:       "max reasoning",
			model:      "deepseek-v4-preview-max",
			wantModel:  "deepseek-v4-preview",
			wantEffort: "max",
		},
		{
			name:       "reasoning disabled",
			model:      "deepseek-v4-preview-none",
			wantModel:  "deepseek-v4-preview",
			wantEffort: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName: tt.model,
			}}
			request := dto.OpenAIResponsesRequest{Model: "client-model"}

			convertedValue, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, request)

			require.NoError(t, err)
			converted, ok := convertedValue.(dto.OpenAIResponsesRequest)
			require.True(t, ok)
			require.NotNil(t, converted.Reasoning)
			assert.Equal(t, tt.wantModel, converted.Model)
			assert.Equal(t, tt.wantEffort, converted.Reasoning.Effort)
			assert.Equal(t, tt.wantModel, info.UpstreamModelName)
			assert.Equal(t, tt.wantEffort, info.ReasoningEffort)
		})
	}
}

func TestConvertOpenAIResponsesRequestPreservesClientReasoning(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		UpstreamModelName: "deepseek-chat",
	}}
	request := dto.OpenAIResponsesRequest{
		Model:     "deepseek-chat",
		Reasoning: &dto.Reasoning{Effort: "high"},
	}

	convertedValue, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, request)

	require.NoError(t, err)
	converted, ok := convertedValue.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.NotNil(t, converted.Reasoning)
	assert.Equal(t, "deepseek-chat", converted.Model)
	assert.Equal(t, "high", converted.Reasoning.Effort)
	assert.Equal(t, "high", info.ReasoningEffort)
}
