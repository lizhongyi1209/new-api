package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestGeminiImageUsageFeedsCacheModalitiesAndBounds(t *testing.T) {
	var response map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(`{"usageMetadata":{"promptTokenCount":1000,"candidatesTokenCount":200,"thoughtsTokenCount":20,"cachedContentTokenCount":400,"promptTokensDetails":[{"modality":"TEXT","tokenCount":500},{"modality":"IMAGE","tokenCount":300},{"modality":"IMAGE","tokenCount":200}],"cacheTokensDetails":[{"modality":"TEXT","tokenCount":200},{"modality":"IMAGE","tokenCount":200},{"modality":"AUDIO","tokenCount":0}],"candidatesTokensDetails":[{"modality":"IMAGE","tokenCount":120},{"modality":"IMAGE","tokenCount":80}]}}`, &response))
	p, c, facts := extractGeminiUsage(response)
	assert.Equal(t, 1000, p)
	assert.Equal(t, 220, c)
	input, ok := facts["input_token_details"].(dto.InputTokenDetails)
	require.True(t, ok)
	require.NotNil(t, input.CachedTokensDetails)
	require.NotNil(t, input.CachedTokensDetails.AudioTokens)
	assert.Zero(t, *input.CachedTokensDetails.AudioTokens)
	assert.Equal(t, 500, input.ImageTokens)
	assert.Equal(t, 400, input.CachedTokens)
	params := BuildTieredTokenParams(&dto.Usage{PromptTokens: p, CompletionTokens: c, PromptTokensDetails: input, CompletionTokenDetails: dto.OutputTokenDetails{ImageTokens: facts["image_output_tokens"].(int)}}, false, map[string]bool{"cr": true, "img": true, "img_cr": true, "img_o": true})
	assert.Equal(t, float64(300), params.P)
	assert.Equal(t, float64(20), params.C)
	assert.Equal(t, float64(200), params.CR)
	assert.Equal(t, float64(300), params.Img)
	assert.Equal(t, float64(200), params.ImgCR)
	assert.Equal(t, float64(200), params.ImgO)
	delete(response["usageMetadata"].(map[string]interface{}), "cacheTokensDetails")
	_, _, facts = extractGeminiUsage(response)
	input = facts["input_token_details"].(dto.InputTokenDetails)
	assert.Nil(t, input.CachedTokensDetails)
	p, c, facts = extractGeminiUsage(map[string]interface{}{"usageMetadata": map[string]interface{}{"promptTokenCount": float64(-1), "candidatesTokenCount": float64(math.MaxInt32), "thoughtsTokenCount": float64(math.MaxInt32)}})
	assert.Zero(t, p)
	assert.Equal(t, math.MaxInt32, c)
}
