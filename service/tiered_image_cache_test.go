package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTieredImageCacheSeparatesOverlapOnlyWhenExplicitlyPriced(t *testing.T) {
	const expression = `tier("image", p*2 + cr*0.5 + img*5 + img_cr*0.1)`
	for _, tc := range []struct {
		name                              string
		details                           string
		usedVars                          map[string]bool
		wantP, wantCR, wantImg, wantImgCR float64
		wantQuota                         int
	}{
		{"valid overlap", `{"image_tokens":200,"text_tokens":200}`, billingexpr.UsedVars(expression), 300, 200, 300, 200, 1110},
		{"missing breakdown retains old normalization", `null`, billingexpr.UsedVars(expression), 100, 400, 500, 0, 1450},
		{"explicit zero retains presence", `{"image_tokens":0}`, billingexpr.UsedVars(expression), 100, 400, 500, 0, 1450},
		{"missing image count is not inferred", `{"text_tokens":200}`, billingexpr.UsedVars(expression), 100, 400, 500, 0, 1450},
		{"negative overlap falls back", `{"image_tokens":-1}`, billingexpr.UsedVars(expression), 100, 400, 500, 0, 1450},
		{"overlap exceeds cache falls back", `{"image_tokens":401}`, billingexpr.UsedVars(expression), 100, 400, 500, 0, 1450},
		{"modality total exceeds cache falls back", `{"image_tokens":200,"text_tokens":201}`, billingexpr.UsedVars(expression), 100, 400, 500, 0, 1450},
		{"negative remaining modality falls back", `{"image_tokens":200,"audio_tokens":-1}`, billingexpr.UsedVars(expression), 100, 400, 500, 0, 1450},
		{"old expression keeps existing categories", `{"image_tokens":200}`, map[string]bool{"cr": true, "img": true}, 100, 400, 500, 0, 1450},
		{"unpriced remaining categories stay in prompt", `{"image_tokens":200}`, map[string]bool{"img_cr": true}, 800, 200, 300, 200, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var usage dto.Usage
			require.NoError(t, common.UnmarshalJsonStr(`{"prompt_tokens":1000,"prompt_tokens_details":{"cached_tokens":400,"image_tokens":500,"cached_tokens_details":`+tc.details+`}}`, &usage))
			params := BuildTieredTokenParams(&usage, false, tc.usedVars)
			assert.Equal(t, tc.wantP, params.P)
			assert.Equal(t, tc.wantCR, params.CR)
			assert.Equal(t, tc.wantImg, params.Img)
			assert.Equal(t, tc.wantImgCR, params.ImgCR)
			assert.Equal(t, float64(1000), params.Len)
			if tc.wantQuota != 0 {
				result, err := billingexpr.ComputeTieredQuota(makeSnapshot(expression, 1, 1000, 0), params)
				require.NoError(t, err)
				assert.Equal(t, tc.wantQuota, result.ActualQuotaAfterGroup)
				require.NotNil(t, result.BillingTokens)
				assert.Equal(t, params, *result.BillingTokens)
			}
		})
	}
}

func TestTieredImageCacheRejectsImpossibleUnionAndKeepsClaudeSemantics(t *testing.T) {
	image := 100
	usage := &dto.Usage{PromptTokens: 1000, PromptTokensDetails: dto.InputTokenDetails{
		CachedTokens: 500, ImageTokens: 900, CachedTokensDetails: &dto.CachedTokenDetails{ImageTokens: &image},
	}}
	params := BuildTieredTokenParams(usage, false, map[string]bool{"cr": true, "img": true, "img_cr": true})
	assert.Zero(t, params.ImgCR)
	assert.Zero(t, params.P)
	assert.Equal(t, float64(500), params.CR)
	assert.Equal(t, float64(900), params.Img)
	usage.UsageSemantic = "anthropic"
	usage.PromptTokensDetails.CachedCreationTokens = 50
	params = BuildTieredTokenParams(usage, true, map[string]bool{"cr": true, "img_cr": true})
	assert.Zero(t, params.ImgCR)
	assert.Equal(t, float64(1000), params.P)
	assert.Equal(t, float64(50), params.CC)
	assert.Equal(t, float64(1550), params.Len)
}
