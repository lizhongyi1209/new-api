package relayconvert

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsRequestToResponsesPreservesOptionalFields(t *testing.T) {
	zero := 0.0
	key := "session-\"quoted\"\\path\n世界"
	got, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
		Model:            "gpt-test",
		Messages:         []dto.Message{{Role: "user", Content: "hello"}},
		FrequencyPenalty: &zero,
		PresencePenalty:  &zero,
		PromptCacheKey:   key,
	})
	require.NoError(t, err)

	encoded, err := common.Marshal(got)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(encoded, &payload))
	assert.Equal(t, float64(0), payload["frequency_penalty"])
	assert.Equal(t, float64(0), payload["presence_penalty"])
	assert.Equal(t, key, payload["prompt_cache_key"])
}

func TestChatCompletionsRequestToResponsesOmitsAbsentOptionalFields(t *testing.T) {
	got, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
		Model:    "gpt-test",
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	})
	require.NoError(t, err)

	encoded, err := common.Marshal(got)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(encoded, &payload))
	assert.NotContains(t, payload, "frequency_penalty")
	assert.NotContains(t, payload, "presence_penalty")
	assert.NotContains(t, payload, "prompt_cache_key")
}
