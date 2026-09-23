package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIHandlersPreserveImageCacheBreakdown(t *testing.T) {
	const body = `{"data":[{"b64_json":"synthetic"}],"usage":{"input_tokens":1000,"output_tokens":200,"total_tokens":1200,"input_tokens_details":{"cached_tokens":400,"image_tokens":500,"cached_tokens_details":{"image_tokens":200,"text_tokens":200,"audio_tokens":0}},"output_tokens_details":{"image_tokens":200}}}`
	for _, path := range []string{"images", "responses", "compact responses"} {
		t.Run(path, func(t *testing.T) {
			c, _, response, info := newImageTestContext(t, body, "application/json", false)
			var usage *dto.Usage
			var err *types.NewAPIError
			switch path {
			case "images":
				usage, err = OpenaiImageHandler(c, info, response)
			case "responses":
				usage, err = OaiResponsesHandler(c, info, response)
			case "compact responses":
				usage, err = OaiResponsesCompactionHandler(c, response)
			}
			require.Nil(t, err)
			require.NotNil(t, usage)
			assert.Equal(t, 1000, usage.PromptTokens)
			assert.Equal(t, 400, usage.PromptTokensDetails.CachedTokens)
			assert.Equal(t, 500, usage.PromptTokensDetails.ImageTokens)
			details := usage.PromptTokensDetails.CachedTokensDetails
			require.NotNil(t, details)
			require.NotNil(t, details.ImageTokens)
			assert.Equal(t, 200, *details.ImageTokens)
			require.NotNil(t, details.AudioTokens)
			assert.Zero(t, *details.AudioTokens)
		})
	}
}

func TestResponsesHandlersPreserveOutputTokenDetails(t *testing.T) {
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	const body = `{"usage":{"input_tokens":100,"output_tokens":50,"total_tokens":150,"output_tokens_details":{"text_tokens":23,"audio_tokens":20,"reasoning_tokens":7}}}`
	for _, path := range []string{"responses", "compact", "stream"} {
		t.Run(path, func(t *testing.T) {
			payload := body
			if path == "stream" {
				payload = "data: {\"type\":\"response.completed\",\"response\":" + body + "}\n\n"
			}
			c, _, response, info := newImageTestContext(t, payload, "application/json", path == "stream")
			var usage *dto.Usage
			var err *types.NewAPIError
			switch path {
			case "responses":
				usage, err = OaiResponsesHandler(c, info, response)
			case "compact":
				usage, err = OaiResponsesCompactionHandler(c, response)
			case "stream":
				usage, err = OaiResponsesStreamHandler(c, info, response)
			}
			require.Nil(t, err)
			require.NotNil(t, usage)
			assert.Equal(t, 50, usage.CompletionTokens)
			assert.Equal(t, 20, usage.CompletionTokenDetails.AudioTokens)
			assert.Equal(t, 7, usage.CompletionTokenDetails.ReasoningTokens)
		})
	}
}
