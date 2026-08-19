package ollama

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIChatToOllamaPreservesReasoningAndToolContext(t *testing.T) {
	reasoning := "previous reasoning"
	assistant := dto.Message{Role: "assistant", Content: ""}
	assistant.ReasoningContent = &reasoning
	assistant.SetToolCalls([]dto.ToolCallRequest{{
		ID:   "call_weather",
		Type: "function",
		Function: dto.FunctionRequest{
			Name: "get_weather", Arguments: `{"city":"Paris"}`,
		},
	}})

	got, err := openAIChatToOllamaChat(nil, &dto.GeneralOpenAIRequest{
		Model:           "qwen3",
		ReasoningEffort: "high",
		Messages: []dto.Message{
			assistant,
			{Role: "tool", ToolCallId: "call_weather", Content: "sunny"},
		},
	})
	require.NoError(t, err)
	require.Len(t, got.Messages, 2)
	assert.JSONEq(t, `"high"`, string(got.Think))
	assert.JSONEq(t, `"previous reasoning"`, string(got.Messages[0].Thinking))
	require.Len(t, got.Messages[0].ToolCalls, 1)
	assert.Equal(t, "call_weather", got.Messages[0].ToolCalls[0].ID)
	assert.Equal(t, "call_weather", got.Messages[1].ToolCallID)
	assert.Equal(t, "get_weather", got.Messages[1].ToolName)

	encoded, err := common.Marshal(got)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"stream":false`)
}

func TestToOllamaResponseFormatUsesNestedSchema(t *testing.T) {
	format, err := toOllamaResponseFormat(&dto.ResponseFormat{
		Type:       "json_schema",
		JsonSchema: []byte(`{"name":"answer","schema":{"type":"object","required":["value"]}}`),
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"type": "object", "required": []any{"value"},
	}, format)
}
