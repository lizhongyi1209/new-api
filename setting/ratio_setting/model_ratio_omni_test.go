package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiOmniDefaultModalityRatios(t *testing.T) {
	InitRatioSettings()

	modelRatio, configured, matchedModel := GetModelRatio("gemini-omni-flash-preview")
	require.True(t, configured)
	assert.Equal(t, "gemini-omni-flash-preview", matchedModel)
	assert.InDelta(t, 0.75, modelRatio, 1e-12)
	assert.InDelta(t, 6.0, GetCompletionRatio("gemini-omni-flash-preview"), 1e-12)
	assert.True(t, ContainsVideoCompletionRatio("gemini-omni-flash-preview"))
	assert.InDelta(t, 35.0/3.0, GetVideoCompletionRatio("gemini-omni-flash-preview"), 1e-12)
}
