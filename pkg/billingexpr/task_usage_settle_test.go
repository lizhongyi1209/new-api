package billingexpr_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskUsageSnapshotSettlesActualUsageWithFrozenPriceAndRequestConditions(t *testing.T) {
	const expression = `tier("seconds", u("seconds") * 0.25) * (param("quality") == "hd" ? 2 : 1)`
	initial := billingexpr.RequestInput{
		Params: map[string]any{"quality": "hd"},
		Usage:  map[string]any{"seconds": float64(4)},
	}
	estimatedCost, trace, err := billingexpr.RunExprWithRequest(expression, billingexpr.TokenParams{}, initial)
	require.NoError(t, err)
	assert.Equal(t, float64(2), estimatedCost)
	snapshot := billingexpr.BillingSnapshot{
		BillingMode: "tiered_expr", ModelName: "shared-model",
		ExprString: expression, ExprHash: billingexpr.ExprHashString(expression),
		TaskUsageBilling: true, RequestParams: initial.Params, UsageFacts: initial.Usage,
		GroupRatio: 1.5, QuotaPerUnit: 500000,
		EstimatedQuotaAfterGroup: 1500000, EstimatedTier: trace.MatchedTier,
	}
	encoded, err := common.Marshal(snapshot)
	require.NoError(t, err)
	var persisted billingexpr.BillingSnapshot
	require.NoError(t, common.Unmarshal(encoded, &persisted))

	actual, err := billingexpr.ComputeTieredQuotaWithRequest(
		&persisted, billingexpr.TokenParams{},
		persisted.TaskRequestInput(map[string]any{"seconds": float64(6)}),
	)
	require.NoError(t, err)
	assert.Equal(t, 2250000, actual.ActualQuotaAfterGroup)
	assert.Equal(t, "seconds", actual.MatchedTier)
	assert.False(t, actual.CrossedTier)

	free, err := billingexpr.ComputeTieredQuotaWithRequest(
		&persisted, billingexpr.TokenParams{},
		persisted.TaskRequestInput(map[string]any{"seconds": float64(0)}),
	)
	require.NoError(t, err)
	assert.Zero(t, free.ActualQuotaAfterGroup)
	assert.Equal(t, expression, persisted.ExprString)
}
