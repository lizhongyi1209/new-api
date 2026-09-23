package billingexpr_test

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixedPriceBranches(t *testing.T) {
	const expression = `(len <= 32000 ? tier("short", fixed(0.01)) : tier("long", p * 2 + c * 8)) * (param("fast") == true ? 2 : 1)`
	for _, tc := range []struct {
		name   string
		params billingexpr.TokenParams
		cost   float64
		tier   string
	}{
		{"zero usage still charges once", billingexpr.TokenParams{}, 20000, "short"},
		{"fixed branch ignores token prices", billingexpr.TokenParams{P: 32000, C: 9000, Len: 32000}, 20000, "short"},
		{"token branch uses actual usage", billingexpr.TokenParams{P: 32001, C: 100, Len: 32001}, 129604, "long"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cost, trace, err := billingexpr.RunExprWithRequest(expression, tc.params, billingexpr.RequestInput{Body: []byte(`{"fast":true}`)})
			require.NoError(t, err)
			assert.Equal(t, tc.cost, cost)
			assert.Equal(t, tc.tier, trace.MatchedTier)
			require.Len(t, trace.RequestRules, 1)
			assert.True(t, trace.RequestRules[0].Matched)
		})
	}
	cost, _, err := billingexpr.RunExpr(`tier("free", fixed(0))`, billingexpr.TokenParams{P: 1000})
	require.NoError(t, err)
	assert.Zero(t, cost)
	for _, count := range []int{-1, 0, 129} {
		_, _, err := billingexpr.RunExprWithRequest(`tier("image", fixed(0.04)) * image_count`, billingexpr.TokenParams{}, billingexpr.RequestInput{ImageCount: &count})
		require.ErrorContains(t, err, "image_count")
	}
}

func TestFixedPriceRejectsInvalidLeavesIncludingUnselectedBranches(t *testing.T) {
	for _, expression := range []string{
		`true ? tier("ok", p) : tier("bad", fixed(-0.01))`,
		`tier("bad", fixed(p))`,
		`tier("bad", fixed(0.01 + 0.02))`,
		`tier("bad", fixed(1e308))`,
		`tier("bad", fixed(0.01) + p * 2)`,
		`tier("bad", p * fixed(0.01))`,
		`tier("a", fixed(0.01)) + tier("b", p * 2)`,
		`fixed(0.01)`,
		`tier("bad", fixed(0.01)) * p`,
		`tier("bad", fixed(0.01)) * (param("fast") == true ? -2 : 1)`,
	} {
		t.Run(expression, func(t *testing.T) {
			_, err := billingexpr.CompileFromCache(expression)
			require.Error(t, err)
		})
	}
}

func TestFixedPriceQuotaBoundariesAndUnsupportedTaskSnapshots(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		group            float64
		want             int
		clamped          bool
	}{
		{"normal group", `tier("request", fixed(0.01))`, 1.5, 7500, false},
		{"free group", `tier("request", fixed(0.01))`, 0, 0, false},
		{"explicit free price", `tier("free", fixed(0))`, 1, 0, false},
		{"oversized price saturates instead of crediting", `tier("huge", fixed(1e20))`, 1, math.MaxInt32, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap := &billingexpr.BillingSnapshot{ExprString: tc.expression, ExprHash: billingexpr.ExprHashString(tc.expression), GroupRatio: tc.group, QuotaPerUnit: 500000}
			result, err := billingexpr.ComputeTieredQuota(snap, billingexpr.TokenParams{})
			require.NoError(t, err)
			assert.Equal(t, tc.want, result.ActualQuotaAfterGroup)
			assert.Equal(t, billingexpr.BillingUnitRequest, result.BillingUnit)
			assert.Equal(t, tc.clamped, result.Clamp != nil)
			_, err = billingexpr.QuotaRoundStrict(result.ActualQuotaBeforeGroup * tc.group)
			if tc.clamped {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			snap.TaskUsageBilling = true
			_, err = billingexpr.ComputeTieredQuota(snap, billingexpr.TokenParams{})
			require.ErrorContains(t, err, "task usage")
		})
	}
	const optimized = `true ? tier("tokens", p) : tier("request", fixed(0.01))`
	assert.True(t, billingexpr.UsesFixedPricing(optimized))
	_, trace, err := billingexpr.RunExpr(optimized, billingexpr.TokenParams{P: 10})
	require.NoError(t, err)
	assert.Equal(t, billingexpr.BillingUnitToken, trace.BillingUnit)
	assert.Nil(t, trace.FixedPrice)
}

func TestLegacySnapshotKeepsTokenScale(t *testing.T) {
	const old = `{"billing_mode":"tiered_expr","expr_string":"tier(\"base\", p * 2 + c * 10)","expr_hash":"","group_ratio":1.2,"quota_per_unit":500000,"expr_version":1,"estimated_tier":"base"}`
	var snap billingexpr.BillingSnapshot
	require.NoError(t, common.UnmarshalJsonStr(old, &snap))
	snap.ExprHash = billingexpr.ExprHashString(snap.ExprString)
	result, err := billingexpr.ComputeTieredQuota(&snap, billingexpr.TokenParams{P: 2500, C: 2000})
	require.NoError(t, err)
	assert.Equal(t, 15000, result.ActualQuotaAfterGroup)
	assert.Equal(t, billingexpr.BillingUnitToken, result.BillingUnit)
	assert.Nil(t, result.FixedPrice)
	assert.False(t, snap.TaskUsageBilling)
}

func TestRequestTraceKeepsLegacyArithmeticAndDoesNotLeakMatches(t *testing.T) {
	const expression = `tier("base", p * 3) * (header("X-Fixture") == "discount" ? 0.5 : 1) * (param("mode") == "fast" ? 2 : 1)`
	for _, tc := range []struct {
		name    string
		header  string
		body    string
		cost    float64
		matches []bool
	}{
		{"both match", "discount", `{"mode":"fast"}`, 300, []bool{true, true}},
		{"none match after cached matched evaluation", "", `{}`, 300, []bool{false, false}},
		{"discount only", "discount", `{}`, 150, []bool{true, false}},
		{"fast only", "", `{"mode":"fast"}`, 600, []bool{false, true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cost, trace, err := billingexpr.RunExprWithRequest(expression, billingexpr.TokenParams{P: 100}, billingexpr.RequestInput{Headers: map[string]string{"X-Fixture": tc.header}, Body: []byte(tc.body)})
			require.NoError(t, err)
			assert.Equal(t, tc.cost, cost)
			require.Len(t, trace.RequestRules, 2)
			for i, match := range tc.matches {
				assert.Equal(t, match, trace.RequestRules[i].Matched)
			}
		})
	}
}
