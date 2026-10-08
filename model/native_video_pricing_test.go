package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/videoexpr"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeVideoPricingAliasSchemaAndVersionedSave(t *testing.T) {
	pricingPreviewDatabase(t)
	require.NoError(t, DB.AutoMigrate(&Vendor{}))
	oldOptions := common.OptionMap
	common.OptionMap = make(map[string]string)
	oldPrices := map[string]string{
		"ModelPrice": ratio_setting.ModelPrice2JSONString(), "ModelRatio": ratio_setting.ModelRatio2JSONString(),
		"CompletionRatio": ratio_setting.CompletionRatio2JSONString(), "CacheRatio": ratio_setting.CacheRatio2JSONString(),
		"CreateCacheRatio": ratio_setting.CreateCacheRatio2JSONString(), "ImageRatio": ratio_setting.ImageRatio2JSONString(),
		"AudioRatio": ratio_setting.AudioRatio2JSONString(), "AudioCompletionRatio": ratio_setting.AudioCompletionRatio2JSONString(),
		"VideoCompletionRatio": ratio_setting.VideoCompletionRatio2JSONString(),
	}
	oldBilling, err := config.ConfigToMap(config.GlobalConfig.Get("billing_setting"))
	require.NoError(t, err)
	t.Cleanup(func() {
		for key, value := range oldPrices {
			require.NoError(t, updateOptionMap(key, value))
		}
		require.NoError(t, config.UpdateConfigFromMap(config.GlobalConfig.Get("billing_setting"), oldBilling))
		common.OptionMap = oldOptions
		InvalidatePricingCache()
	})
	mapping := `{"my-video":"seedance-2.0-fast"}`
	require.NoError(t, DB.Create(&Channel{Id: 265, Type: 60, Models: "dreamina-seedance-2-0-fast-hc,my-video", ModelMapping: &mapping}).Error)
	snapshot, err := GetModelPricingSnapshot([]string{"my-video"})
	require.NoError(t, err)
	require.Len(t, snapshot.Entries, 1)
	entry := snapshot.Entries[0]
	assert.Equal(t, "token", entry.UsageSchema["tokens"].Unit)
	assert.Equal(t, []string{"480p", "720p"}, entry.UsageSchema["resolution"].Enum)
	require.NotEmpty(t, entry.UsageExamples)
	expression := `u("video_input") == "video" ? tier("video", u("tokens") * 22 / 1000000) : tier("text", u("tokens") * 37 / 1000000)`
	require.NoError(t, UpdateModelPricing([]ModelPricingChange{{ModelName: "my-video", ExpectedVersion: entry.Version, Pricing: PricingValues{
		"billing_setting.billing_mode": "tiered_expr", "billing_setting.billing_expr": expression,
	}}}))
	saved, err := GetModelPricingSnapshot([]string{"my-video"})
	require.NoError(t, err)
	assert.Equal(t, expression, saved.Entries[0].Configured["billing_setting.billing_expr"])
	assert.NotEqual(t, entry.Version, saved.Entries[0].Version)
	assert.ErrorIs(t, UpdateModelPricing([]ModelPricingChange{{ModelName: "my-video", ExpectedVersion: entry.Version, Reset: true}}), ErrModelPricingConflict)
	assert.ErrorContains(t, ValidateModelPricing("my-video", PricingValues{"billing_setting.billing_expr": `tier("bad", u("unknown") * 1)`}), "unknown")
	assert.Error(t, ValidateModelPricing("my-video", PricingValues{"billing_setting.billing_expr": `tier("bad", u("tokens") * -1)`}))
	assert.Error(t, ValidateModelPricing("gpt-4", PricingValues{"billing_setting.billing_expr": `tier("bad", u("tokens") * 1)`}))
}

func TestNativeVideoPricingRejectsIncompatibleMappedProviders(t *testing.T) {
	pricingPreviewDatabase(t)
	seedanceMap, grokMap := `{"shared-video":"seedance-2.0"}`, `{"shared-video":"grok-imagine-video-1.5"}`
	require.NoError(t, DB.Create(&Channel{Id: 1, Type: 60, Models: "shared-video", ModelMapping: &seedanceMap}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 2, Type: 48, Models: "shared-video", ModelMapping: &grokMap}).Error)
	assert.ErrorContains(t, ValidateModelPricing("shared-video", PricingValues{"billing_setting.billing_expr": `tier("bad", u("tokens") * 1)`}), "incompatible")
}

func TestNativeVideoPricingCatalogCoversRecordedFamilies(t *testing.T) {
	pricingPreviewDatabase(t)
	for _, channel := range []Channel{
		{Id: 1, Type: 60, Models: "dreamina-seedance-2-0-hc,dreamina-seedance-2-0-mini-hc,dreamina-seedance-2-5-hc,MiniMax-H3,MiniMax-H3-MAX,grok-imagine-video-1.5"},
		{Id: 2, Type: 54, Models: "doubao-seedance-2-0-260128,seedance-2.0-fast"},
		{Id: 3, Type: 61, Models: "seedance-2.0-mini"},
		{Id: 4, Type: 50, Models: "kling-v3,kling-v3-omni,kling-3.0,kling-v2-6-std-5s-novoice"},
		{Id: 5, Type: 58, Models: "kling-v3-t,kling-v2-6-motion-t"},
		{Id: 6, Type: 35, Models: "MiniMax-H3"},
		{Id: 7, Type: 48, Models: "grok-imagine-video"},
		{Id: 8, Type: 24, Models: "gemini-omni-flash-preview"},
		{Id: 9, Type: 1, Models: "veo-3.1,omni_flash_abra_edit,omni_flash_10s,grok-imagine-video-1.5-preview"},
	} {
		require.NoError(t, DB.Create(&channel).Error)
	}
	catalog, conflicts, err := nativeVideoPricingCatalog(DB)
	require.NoError(t, err)
	assert.Empty(t, conflicts)
	assert.Len(t, catalog, 21)
	assert.Equal(t, videoexpr.Omni, catalog["gemini-omni-flash-preview"].Meter)
	assert.Equal(t, videoexpr.Kling, catalog["kling-v2-6-motion-t"].Meter)
	// Reading the admin snapshot must not create or rewrite any price options.
	_, err = GetModelPricingSnapshot(nil)
	require.NoError(t, err)
	var options int64
	require.NoError(t, DB.Model(&Option{}).Count(&options).Error)
	assert.Zero(t, options)
	encoded, err := common.Marshal(catalog["gemini-omni-flash-preview"].Schema)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), "video_output_tokens")
}
