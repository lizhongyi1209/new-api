package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedCustomValidateResponsesToChatConverterPath(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
			},
		},
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name         string
		incomingPath string
	}{
		{name: "chat completions", incomingPath: "/v1/chat/completions"},
		{name: "responses compact", incomingPath: "/v1/responses/compact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdvancedCustomConfig{
				Routes: []AdvancedCustomRoute{
					{
						IncomingPath: tt.incomingPath,
						UpstreamPath: "/v1/chat/completions",
						Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
					},
				},
			}
			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "converter does not match incoming_path")
		})
	}
}

func TestAdvancedCustomRoutesSplitByClientModel(t *testing.T) {
	config := &AdvancedCustomConfig{Routes: []AdvancedCustomRoute{
		{IncomingPath: "/v1/chat/completions", UpstreamPath: "/special", Converter: AdvancedCustomConverterNone, Models: []string{"gpt-4o", "re:^claude-"}},
		{IncomingPath: "/v1/chat/completions", UpstreamPath: "/fallback", Converter: AdvancedCustomConverterNone},
	}}
	require.NoError(t, config.Validate())

	route, ok := config.MatchPathForModel("/v1/chat/completions", "claude-sonnet-4")
	require.True(t, ok)
	assert.Equal(t, "/special", route.UpstreamPath)
	route, ok = config.MatchPathForModel("/v1/chat/completions", "unknown-model")
	require.True(t, ok)
	assert.Equal(t, "/fallback", route.UpstreamPath)
}

func TestAdvancedCustomRoutesRejectSplitAfterFallback(t *testing.T) {
	config := &AdvancedCustomConfig{Routes: []AdvancedCustomRoute{
		{IncomingPath: "/v1/chat/completions", UpstreamPath: "/fallback", Converter: AdvancedCustomConverterNone},
		{IncomingPath: "/v1/chat/completions", UpstreamPath: "/special", Converter: AdvancedCustomConverterNone, Models: []string{"gpt-4o"}},
	}}
	assert.ErrorContains(t, config.Validate(), "catch-all route must be last")
}

func TestChannelOtherSettingsGeminiFileDataCapabilityDefaultsOff(t *testing.T) {
	var defaultSettings ChannelOtherSettings
	require.NoError(t, common.Unmarshal([]byte(`{}`), &defaultSettings))
	assert.False(t, defaultSettings.GeminiFileDataEnabled)

	var enabledSettings ChannelOtherSettings
	require.NoError(t, common.Unmarshal([]byte(`{"gemini_file_data_enabled":true}`), &enabledSettings))
	assert.True(t, enabledSettings.GeminiFileDataEnabled)
}

func TestChannelOtherSettingsValidateImageOutputStrategy(t *testing.T) {
	for _, strategy := range []string{
		"",
		ImageOutputStrategyOSS,
		ImageOutputStrategyR2,
		ImageOutputStrategyLocalTemp,
		ImageOutputStrategyLocalTempCF,
		ImageOutputStrategyLocalTempESA,
		ImageOutputStrategyPassthrough,
	} {
		t.Run(strategy, func(t *testing.T) {
			settings := ChannelOtherSettings{ImageOutputStrategy: strategy}
			require.NoError(t, settings.ValidateImageOutputStrategy())
		})
	}

	settings := ChannelOtherSettings{ImageOutputStrategy: "local"}
	err := settings.ValidateImageOutputStrategy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image_output_strategy")
}

func TestImageOutputTemporaryStrategyIncludesExplicitCDNDomains(t *testing.T) {
	for _, strategy := range []string{
		ImageOutputStrategyLocalTemp,
		ImageOutputStrategyLocalTempCF,
		ImageOutputStrategyLocalTempESA,
	} {
		assert.True(t, IsImageOutputTemporaryStrategy(strategy))
		assert.True(t, IsImageOutputStorageStrategy(strategy))
	}
	assert.False(t, IsImageOutputTemporaryStrategy(ImageOutputStrategyR2))
}
