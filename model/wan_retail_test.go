package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWanCustomerPricingPublishesOnlyGroupedSales(t *testing.T) {
	for _, name := range []string{"wan3.0-video", "wan3.0-video-prime"} {
		t.Run(name, func(t *testing.T) {
			expression := `u("resolution") == "480P" ? tier("480P", u("billed_seconds") * 0.3) : u("resolution") == "720P" ? tier("720P", u("billed_seconds") * 0.6) : tier("1080P", u("billed_seconds") * 1.2)`
			schema := map[string]jsplugin.UsageFieldSchema{"resolution": {Enum: []string{"480P", "720P", "1080P"}}, "billed_seconds": {Type: "number", Unit: "second"}}
			private := Pricing{ModelName: name, BillingExpr: expression, EnableGroup: []string{"wan", "free"}, BillingPluginVariants: []PricingPluginVariant{{PluginKey: constant.WanOfficialVideoPlugin, BillingExpr: expression, BillingUsageSchema: schema}}}
			public := CustomerVideoPricing(private, map[string]float64{"wan": 0.6, "free": 0, "hidden": 2})
			require.NotNil(t, public.RetailVideoPrices)
			assert.Equal(t, "billable", public.RetailVideoPrices.Basis)
			require.Len(t, public.RetailVideoPrices.Groups, 2)
			assert.Equal(t, map[string]float64{"480P": 0.18, "720P": 0.36, "1080P": 0.72}, public.RetailVideoPrices.Groups[1].Rows[0].Prices)
			assert.Zero(t, public.RetailVideoPrices.Groups[0].Rows[0].Prices["480P"])
			serialized, err := common.Marshal(public)
			require.NoError(t, err)
			for _, secret := range []string{"billing_expr", "billing_usage", "billing_plugin", "billed_seconds", "tier("} {
				assert.NotContains(t, string(serialized), secret)
			}
			assert.Equal(t, expression, private.BillingPluginVariants[0].BillingExpr)
			private.BillingPluginVariants[0].BillingExpr = `tier("custom",u("billed_seconds") * u("billed_seconds"))`
			hidden := CustomerVideoPricing(private, map[string]float64{"wan": 0.6})
			require.NotNil(t, hidden.RetailVideoPrices)
			assert.Empty(t, hidden.RetailVideoPrices.Groups)
			assert.Empty(t, hidden.BillingPluginVariants)
			assert.Empty(t, hidden.BillingExpr)
		})
	}
}
func TestWanCustomerLogsKeepOrderAmountAndHidePricing(t *testing.T) {
	for _, name := range []string{"wan3.0-video", "wan3.0-video-prime"} {
		log := &Log{Type: LogTypeConsume, ModelName: name, Quota: 1800000, Content: "internal formula", Other: `{"expr_b64":"hidden","usage_facts":{"billed_seconds":5},"billing":{"price":1.2},"actual_quota":1800000,"resolution":"1080P"}`}
		formatUserLogs([]*Log{log}, 0)
		assert.Equal(t, "视频任务消费", log.Content)
		assert.JSONEq(t, `{"actual_quota":1800000,"resolution":"1080P"}`, log.Other)
		assert.Equal(t, 1800000, log.Quota)
	}
}
