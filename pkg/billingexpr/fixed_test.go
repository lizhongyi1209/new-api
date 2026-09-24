package billingexpr_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
