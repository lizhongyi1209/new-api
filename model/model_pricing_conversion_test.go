package model

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func pricingPreviewDatabase(t *testing.T) {
	t.Helper()
	original := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&Option{}, &Channel{}, &Ability{}, &Model{}))
	DB = db
	t.Cleanup(func() {
		DB = original
		require.NoError(t, sqlDB.Close())
	})
}

func TestPricingPreviewDetachedDraftDoesNotWrite(t *testing.T) {
	pricingPreviewDatabase(t)
	const name = "preview-model"
	require.NoError(t, DB.Create(&Option{Key: "ModelRatio", Value: `{"preview-model":7}`}).Error)
	require.NoError(t, DB.Create(&Option{Key: "CompletionRatio", Value: `{"preview-model":9}`}).Error)
	originalCompletion := ratio_setting.CompletionRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"preview-model":9}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(originalCompletion)) })
	draft := PricingValues{"ModelRatio": float64(1)}
	preview, err := PreviewModelPricing(name, draft)
	require.NoError(t, err)
	assert.Equal(t, float64(1), preview["CompletionRatio"])
	assert.Equal(t, float64(1.25), preview["CreateCacheRatio"])
	_, err = PreviewModelPricingConversion(name, draft)
	require.NoError(t, err)
	assert.Equal(t, PricingValues{"ModelRatio": float64(1)}, draft)
	var rows []Option
	require.NoError(t, DB.Order(commonKeyCol).Find(&rows).Error)
	require.Len(t, rows, 2)
	assert.Equal(t, `{"preview-model":9}`, rows[0].Value)
	assert.Equal(t, `{"preview-model":7}`, rows[1].Value)
	assert.Equal(t, float64(9), ratio_setting.GetCompletionRatio(name))
	_, err = PreviewModelPricing(name, nil)
	assert.Error(t, err)
	_, err = PreviewModelPricing(name, PricingValues{"ModelRatio": math.Inf(1)})
	assert.Error(t, err)
}

func TestPricingConversionPreservesLegacyCostComponents(t *testing.T) {
	pricingPreviewDatabase(t)
	quotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = quotaPerUnit })
	count := 2
	for _, tc := range []struct {
		name, model string
		draft       PricingValues
		tokens      billingexpr.TokenParams
		request     billingexpr.RequestInput
		cost        float64
		unit        billingexpr.BillingUnit
	}{
		{"generic cache-write fallback", "preview-model", PricingValues{"ModelRatio": float64(1), "CompletionRatio": float64(3)}, billingexpr.TokenParams{P: 400, C: 50, CR: 200, CC: 100, Img: 300, Len: 1000}, billingexpr.RequestInput{}, 2350, billingexpr.BillingUnitToken},
		{"overlapping same-price input categories", "preview-model", PricingValues{"ModelRatio": float64(1)}, billingexpr.TokenParams{P: 0, CR: 400, CC: 100, Img: 800, Len: 1000}, billingexpr.RequestInput{}, 2650, billingexpr.BillingUnitToken},
		{"explicit free input", "preview-model", PricingValues{"ModelRatio": float64(0)}, billingexpr.TokenParams{P: 100, C: 200, CR: 20, CC: 30, Img: 50}, billingexpr.RequestInput{}, 0, billingexpr.BillingUnitToken},
		{"Claude dual TTL", "claude-preview-model", PricingValues{"ModelRatio": float64(1), "CompletionRatio": float64(3), "CacheRatio": float64(0.1), "CreateCacheRatio": float64(1.25)}, billingexpr.TokenParams{P: 100, C: 50, CR: 200, CC: 300, CC1h: 100, Len: 700}, billingexpr.RequestInput{}, 1690, billingexpr.BillingUnitToken},
		{"audio settlement bypasses text cache ratios", "gpt-4o-audio-preview", PricingValues{"ModelRatio": float64(1), "CompletionRatio": float64(3), "AudioRatio": float64(10), "AudioCompletionRatio": float64(2), "CacheRatio": float64(0.1)}, billingexpr.TokenParams{P: 40, C: 30, CR: 20, AI: 40, AO: 20, Len: 100}, billingexpr.RequestInput{}, 1900, billingexpr.BillingUnitToken},
		{"fixed request", "preview-model", PricingValues{"ModelPrice": float64(0.025)}, billingexpr.TokenParams{}, billingexpr.RequestInput{}, 25000, billingexpr.BillingUnitRequest},
		{"Dalle size quality and count", "dall-e-3", PricingValues{"ModelPrice": float64(0.04)}, billingexpr.TokenParams{}, billingexpr.RequestInput{Body: []byte(`{"size":"1024x1792","quality":"hd"}`), ImageCount: &count}, 240000, billingexpr.BillingUnitRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preview, err := PreviewModelPricingConversion(tc.model, tc.draft)
			require.NoError(t, err)
			require.Empty(t, preview.UnsupportedReason)
			cost, trace, err := billingexpr.RunExprWithRequest(preview.Expression, tc.tokens, tc.request)
			require.NoError(t, err)
			assert.InDelta(t, tc.cost, cost, 0.000001)
			assert.Equal(t, tc.unit, trace.BillingUnit)
			assert.NotEmpty(t, preview.Warnings)
		})
	}
}

func TestPricingConversionRefusesSpecialRouting(t *testing.T) {
	pricingPreviewDatabase(t)
	const name = "preview-alias"
	for _, tc := range []struct {
		name, mapping string
		channelType   int
	}{
		{"video channel", "{}", constant.ChannelTypeServiceInferenceVideo},
		{"mapped realtime", `{"preview-alias":"gpt-realtime"}`, constant.ChannelTypeOpenAI},
		{"mapped video", `{"preview-alias":"hop","hop":"veo-3"}`, constant.ChannelTypeGemini},
		{"mapping cycle", `{"preview-alias":"hop","hop":"preview-alias"}`, constant.ChannelTypeOpenAI},
		{"malformed mapping", "{", constant.ChannelTypeOpenAI},
		{"Claude semantic alias", "{}", constant.ChannelTypeAnthropic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, DB.Where("1 = 1").Delete(&Ability{}).Error)
			require.NoError(t, DB.Where("1 = 1").Delete(&Channel{}).Error)
			require.NoError(t, DB.Create(&Channel{Id: 1, Type: tc.channelType, Status: common.ChannelStatusEnabled, Models: name, ModelMapping: &tc.mapping}).Error)
			require.NoError(t, DB.Create(&Ability{Group: "default", Model: name, ChannelId: 1, Enabled: true}).Error)
			preview, err := PreviewModelPricingConversion(name, PricingValues{"ModelRatio": float64(1)})
			require.NoError(t, err)
			assert.Empty(t, preview.Expression)
			assert.NotEmpty(t, preview.UnsupportedReason)
		})
	}
}
