package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestAppliesModelCapabilities(t *testing.T) {
	tests := []struct {
		name            string
		model           string
		reasoningEffort string
		wantModel       string
		wantSampling    bool
		wantDeveloper   bool
	}{
		{name: "gpt 5.2 defaults to sampling", model: "gpt-5.2", wantModel: "gpt-5.2", wantSampling: true, wantDeveloper: true},
		{name: "gpt 5.2 high drops sampling", model: "gpt-5.2", reasoningEffort: "high", wantModel: "gpt-5.2", wantDeveloper: true},
		{name: "gpt 6 suffix resolves", model: "gpt-6-astra-high", wantModel: "gpt-6-astra", wantDeveloper: true},
		{name: "unknown future model unchanged", model: "gpt-6-preview", wantModel: "gpt-6-preview", wantSampling: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &dto.GeneralOpenAIRequest{
				Model:           tt.model,
				ReasoningEffort: tt.reasoningEffort,
				MaxTokens:       lo.ToPtr(uint(100)),
				Temperature:     lo.ToPtr(0.5),
				TopP:            lo.ToPtr(0.8),
				LogProbs:        lo.ToPtr(true),
				TopLogProbs:     lo.ToPtr(2),
				Messages:        []dto.Message{{Role: "system", Content: "prompt"}},
			}
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       constant.ChannelTypeOpenAI,
					UpstreamModelName: tt.model,
				},
			}

			converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
			require.NoError(t, err)
			assert.Same(t, request, converted)
			assert.Equal(t, tt.wantModel, request.Model)
			if tt.wantDeveloper {
				assert.Equal(t, "developer", request.Messages[0].Role)
			} else {
				assert.Equal(t, "system", request.Messages[0].Role)
			}
			if tt.wantSampling {
				assert.NotNil(t, request.Temperature)
				assert.NotNil(t, request.TopP)
				assert.NotNil(t, request.LogProbs)
				assert.NotNil(t, request.TopLogProbs)
			} else {
				assert.Nil(t, request.Temperature)
				assert.Nil(t, request.TopP)
				assert.Nil(t, request.LogProbs)
				assert.Nil(t, request.TopLogProbs)
			}
			if tt.wantDeveloper {
				assert.Nil(t, request.MaxTokens)
				assert.Equal(t, uint(100), *request.MaxCompletionTokens)
			} else {
				assert.NotNil(t, request.MaxTokens)
			}
		})
	}
}
