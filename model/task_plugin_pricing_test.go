package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPluginPricingKeepsSameNameOrdinaryAndPluginChargesIndependent(t *testing.T) {
	pricingPreviewDatabase(t)
	require.NoError(t, DB.AutoMigrate(&Vendor{}))
	originalRegistry := jsplugin.DefaultRegistry
	jsplugin.DefaultRegistry = jsplugin.NewRegistry()
	originalPrices := ratio_setting.ModelPrice2JSONString()
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		jsplugin.DefaultRegistry = originalRegistry
		require.NoError(t, billing_setting.SetTaskPluginPricingFromJsonString("{}"))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
		InvalidatePricingCache()
	})

	const name = "shared-task-pricing-test"
	for _, key := range []string{"scope-a", "scope-b"} {
		source := fmt.Sprintf(`
export const meta = {
  apiVersion: 1, key: %q, name: %q, version: "1.0.0",
  author: { name: "Test" }, fetchMode: "per_task", models: [%q],
  usageSchema: { seconds: { type: "number", unit: "second" } },
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {}; }
`, key, key, name)
		_, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
		require.NoError(t, err)
	}

	require.NoError(t, UpdateModelPricing([]ModelPricingChange{{
		ModelName: name, ExpectedVersion: ModelPricingVersion(PricingValues{}),
		Pricing: PricingValues{"ModelPrice": float64(0.5), "billing_setting.billing_mode": "ratio"},
	}}))
	for _, channel := range []Channel{
		{Id: 1, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: name},
		{Id: 2, Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Models: name},
	} {
		require.NoError(t, DB.Create(&channel).Error)
		require.NoError(t, DB.Create(&Ability{Group: "default", Model: name, ChannelId: channel.Id, Enabled: true}).Error)
	}
	for _, change := range []struct{ key, expression string }{
		{"scope-a", `tier("task", u("seconds") * 0.25)`},
		{"scope-b", `tier("task", u("seconds") * 0.5)`},
	} {
		require.NoError(t, UpdateTaskPluginPricing(TaskPluginPricingChange{
			PluginKey: change.key, ModelName: name,
			ExpectedVersion: TaskPluginPricingVersion(""), BillingExpr: change.expression,
		}))
	}

	assert.Equal(t, billing_setting.BillingModeRatio, billing_setting.GetBillingMode(name))
	ordinaryPrice, configured := ratio_setting.GetModelPrice(name, false)
	require.True(t, configured)
	assert.Equal(t, 0.5, ordinaryPrice)
	first, configured := billing_setting.GetTaskPluginBillingExpr("scope-a", name)
	require.True(t, configured)
	second, configured := billing_setting.GetTaskPluginBillingExpr("scope-b", name)
	require.True(t, configured)
	assert.NotEqual(t, first, second)

	snapshot, err := GetModelPricingSnapshot([]string{name})
	require.NoError(t, err)
	require.Len(t, snapshot.Entries, 1)
	require.Len(t, snapshot.Entries[0].UsageVariants, 2)
	assert.Equal(t, first, snapshot.Entries[0].UsageVariants[0].BillingExpr)
	assert.Equal(t, second, snapshot.Entries[0].UsageVariants[1].BillingExpr)
	assert.Equal(t, TaskPluginPricingVersion(first), snapshot.Entries[0].UsageVariants[0].Version)

	var public *Pricing
	for _, entry := range GetPricing() {
		if entry.ModelName == name {
			public = &entry
			break
		}
	}
	require.NotNil(t, public)
	assert.Equal(t, 0.5, public.ModelPrice)
	assert.True(t, public.HasOrdinaryChannel)
	assert.Empty(t, public.BillingUsageSchema)
	require.Len(t, public.BillingPluginVariants, 2)
	assert.NotEmpty(t, public.BillingPluginVariants[0].BillingUsageSchema)
	assert.Equal(t, first, public.BillingPluginVariants[0].BillingExpr)
	assert.Equal(t, second, public.BillingPluginVariants[1].BillingExpr)

	err = UpdateTaskPluginPricing(TaskPluginPricingChange{
		PluginKey: "scope-a", ModelName: name,
		ExpectedVersion: TaskPluginPricingVersion(""), BillingExpr: second,
	})
	require.ErrorIs(t, err, ErrModelPricingConflict)
	stillFirst, configured := billing_setting.GetTaskPluginBillingExpr("scope-a", name)
	require.True(t, configured)
	assert.Equal(t, first, stillFirst)

	require.NoError(t, UpdateTaskPluginPricing(TaskPluginPricingChange{
		PluginKey: "scope-a", ModelName: name,
		ExpectedVersion: TaskPluginPricingVersion(first), Reset: true,
	}))
	_, configured = billing_setting.GetTaskPluginBillingExpr("scope-a", name)
	assert.False(t, configured)
	remaining, configured := billing_setting.GetTaskPluginBillingExpr("scope-b", name)
	require.True(t, configured)
	assert.Equal(t, second, remaining)
	ordinaryPrice, configured = ratio_setting.GetModelPrice(name, false)
	require.True(t, configured)
	assert.Equal(t, 0.5, ordinaryPrice)

	require.NoError(t, DB.Model(&Ability{}).Where("channel_id = ?", 1).Update("enabled", false).Error)
	InvalidatePricingCache()
	for _, entry := range GetPricing() {
		if entry.ModelName != name {
			continue
		}
		assert.False(t, entry.HasOrdinaryChannel)
		assert.NotEmpty(t, entry.BillingUsageSchema)
		assert.Equal(t, second, entry.BillingExpr)
		return
	}
	t.Fatal("plugin-only model missing from public pricing")
}

func TestOrdinaryModelPricingRejectsTaskUsageExpression(t *testing.T) {
	pricingPreviewDatabase(t)
	err := ValidateModelPricing("ordinary-model", PricingValues{
		"billing_setting.billing_mode": "tiered_expr",
		"billing_setting.billing_expr": `tier("task", u("seconds") * 0.25)`,
	})
	require.ErrorContains(t, err, "configured usage schema")
}
