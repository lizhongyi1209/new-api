package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKlingCreditPrice = `u("units_source") == "estimated" ? tier("estimated", 0.5 + u("units") * 0) : tier("reported", u("units") * 1)`

func TestKlingRetailPricesFollowCustomerGroupAndHideSettlement(t *testing.T) {
	for _, name := range []string{"kling-v3-omni", "kling-3.0-omni"} {
		t.Run(name, func(t *testing.T) {
			private := Pricing{ModelName: name, BillingMode: "tiered_expr", BillingExpr: testKlingCreditPrice, EnableGroup: []string{"official", "free"},
				BillingPluginVariants: []PricingPluginVariant{{PluginKey: constant.KlingOfficialVideoPlugin, BillingExpr: testKlingCreditPrice}}}
			result := CustomerKlingPricing(private, map[string]float64{"official": .9, "free": 0, "unavailable": 1})
			require.Len(t, result.RetailVideoPrices.Groups, 2)
			free, paid := result.RetailVideoPrices.Groups[0], result.RetailVideoPrices.Groups[1]
			assert.Equal(t, "free", free.Group)
			assert.Zero(t, free.Rows[0].Prices["720P"])
			require.Len(t, paid.Rows, 3)
			for i, row := range [][3]float64{{.54, .72, 2.7}, {.72, .9, 2.7}, {.81, 1.08, 2.7}} {
				for j, size := range []string{"720P", "1080P", "4K"} {
					assert.InDelta(t, row[j], paid.Rows[i].Prices[size], 1e-10)
				}
			}
			serialized, err := common.Marshal(result)
			require.NoError(t, err)
			for _, secret := range []string{"units", "credit", "estimated", "reported", "billing_expr", "billing_usage", "billing_plugin"} {
				assert.NotContains(t, string(serialized), secret)
			}
			assert.Equal(t, testKlingCreditPrice, private.BillingPluginVariants[0].BillingExpr)
			private.BillingPluginVariants[0].BillingExpr = `u("units_source") == "reported" ? tier("reported", 0.4 + u("units") * 1.25) : tier("estimated", 0.5)`
			sales := CustomerKlingPricing(private, map[string]float64{"official": .6})
			require.Len(t, sales.RetailVideoPrices.Groups, 1)
			g := sales.RetailVideoPrices.Groups[0]
			units := 5 * .8
			charge, _, err := billingexpr.RunExprWithRequest(private.BillingPluginVariants[0].BillingExpr, billingexpr.TokenParams{}, billingexpr.RequestInput{Usage: map[string]any{"units_source": "reported", "units": units}})
			require.NoError(t, err)
			assert.InDelta(t, charge*.6, 5*g.Rows[0].Prices["1080P"]+g.PerRequest, 1e-10)
			private.BillingPluginVariants[0].BillingExpr = `tier("internal", u("units") * u("units"))`
			noQuote := CustomerKlingPricing(private, map[string]float64{"official": .9})
			assert.Empty(t, noQuote.RetailVideoPrices.Groups)
			assert.Empty(t, noQuote.BillingExpr)
			assert.Empty(t, noQuote.BillingPluginVariants)
		})
	}
	unrelated := Pricing{ModelName: "wan3.0-video", BillingExpr: "original"}
	assert.Equal(t, unrelated, CustomerKlingPricing(unrelated, nil))
}

func TestKlingCustomerLogHidesHistoricalAndPluginSettlement(t *testing.T) {
	private := `{"billing":{"other_ratios":{"units":3.6},"formula":"hidden"},"settlement_reason":"hidden","expr_b64":"c2VjcmV0","usage_schema":{"units":{}},"usage_facts":{"units":3.6},"actual_quota":1620000,"resolution":"720P"}`
	log := &Log{Type: LogTypeConsume, ModelName: "kling-v3-omni", Content: "操作，计算参数：units: 3.6", Other: private, Quota: 1620000}
	formatUserLogs([]*Log{log}, 0)
	assert.NotContains(t, log.Content, "units")
	assert.JSONEq(t, `{"actual_quota":1620000,"resolution":"720P"}`, log.Other)
	assert.Equal(t, 1620000, log.Quota)
}
