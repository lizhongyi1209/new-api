package relay

import (
	"fmt"
	"maps"
	"math"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/pkg/videoexpr"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type taskExpressionBillingResult struct {
	priceData    types.PriceData
	snapshot     *billingexpr.BillingSnapshot
	requestInput billingexpr.RequestInput
	clamp        *common.QuotaClamp
}

type taskExpressionBillingError struct {
	taskError              *dto.TaskError
	legacyFallbackEligible bool
}

func prepareTaskExpressionBilling(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	adaptor channel.TaskAdaptor,
	selectedPlugin *pluginruntime.LoadedPlugin,
	generation *pluginruntime.RoutingGeneration,
	modelName string,
	pluginKey string,
	expression string,
	configured bool,
) (taskExpressionBillingResult, *taskExpressionBillingError) {
	provider, supported := adaptor.(channel.TaskUsageFactsProvider)
	if !configured || !supported || strings.TrimSpace(expression) == "" || billingexpr.UsesFixedPricing(expression) {
		err := service.TaskErrorWrapperLocal(fmt.Errorf("task model %s has no usable billing expression or meter", modelName), "model_price_error", http.StatusBadRequest)
		return taskExpressionBillingResult{}, &taskExpressionBillingError{taskError: err, legacyFallbackEligible: true}
	}
	if selectedPlugin != nil && generation != nil && (generation.SharedModel(modelName) || generation.SharedModel(info.UpstreamModelName)) {
		schema, _ := selectedPlugin.Meta.UsageForModels(info.UpstreamModelName, modelName)
		if !billing_setting.TaskExprCompatible(expression, schema) {
			err := service.TaskErrorWrapperLocal(fmt.Errorf("task model %s pricing is not configured for plugin %s", modelName, pluginKey), "model_price_error", http.StatusBadRequest)
			return taskExpressionBillingResult{}, &taskExpressionBillingError{taskError: err, legacyFallbackEligible: true}
		}
	}

	var facts map[string]any
	if validated, ok := adaptor.(channel.TaskValidatedUsageFactsProvider); ok {
		var usageErr error
		facts, usageErr = validated.ExtractUsageFactsValidated(c, info)
		if usageErr != nil {
			err := service.TaskErrorWrapperLocal(usageErr, "plugin_usage_invalid", http.StatusBadRequest)
			return taskExpressionBillingResult{}, &taskExpressionBillingError{taskError: err}
		}
	} else {
		facts = provider.ExtractUsageFacts(c, info)
	}
	usageSchema := map[string]pluginruntime.UsageFieldSchema{}
	nativeMeter := ""
	if selectedPlugin != nil {
		usageSchema, _ = selectedPlugin.Meta.UsageForModels(info.UpstreamModelName, modelName)
	} else {
		meta := videoexpr.ForModel(info.ChannelType, info.UpstreamModelName)
		usageSchema = meta.Schema
		nativeMeter = meta.Meter
		if err := billing_setting.SmokeTestTaskExpr(expression, usageSchema); err != nil {
			taskErr := service.TaskErrorWrapperLocal(err, "model_price_error", http.StatusBadRequest)
			return taskExpressionBillingResult{}, &taskExpressionBillingError{taskError: taskErr, legacyFallbackEligible: true}
		}
	}

	billingInput, inputErr := helper.FreezeTaskBillingExprRequestInput(c, expression)
	if inputErr != nil {
		err := service.TaskErrorWrapperLocal(inputErr, "task_plugin_price_invalid", http.StatusBadRequest)
		// At this point a native expression has already passed its schema smoke
		// test. Remaining projection errors can be caused by request-controlled
		// values and must not select a cheaper compatibility price.
		return taskExpressionBillingResult{}, &taskExpressionBillingError{taskError: err}
	}
	billingInput.Usage = facts
	cost, trace, runErr := billingexpr.RunExprWithRequest(expression, billingexpr.TokenParams{}, billingInput)
	if runErr != nil || math.IsNaN(cost) || math.IsInf(cost, 0) || cost < 0 {
		if runErr == nil {
			runErr = fmt.Errorf("invalid task usage expression result")
		}
		err := service.TaskErrorWrapper(runErr, "model_price_error", http.StatusBadRequest)
		return taskExpressionBillingResult{}, &taskExpressionBillingError{taskError: err, legacyFallbackEligible: true}
	}

	groupRatio := helper.HandleGroupRatio(c, info)
	quota, clamp := common.QuotaRoundChecked(cost * common.QuotaPerUnit * groupRatio.GroupRatio)
	priceData := types.PriceData{Quota: quota, QuotaToPreConsume: quota, GroupRatioInfo: groupRatio}

	usedUsageFields := billingexpr.UsedUsageKeys(expression)
	usageDisplaySchema := make(map[string]billingexpr.UsageFieldSnapshot, len(usedUsageFields))
	for field, definition := range usageSchema {
		if !usedUsageFields[field] {
			continue
		}
		labels := make(map[string]map[string]string, len(definition.EnumLabels))
		for value, translations := range definition.EnumLabels {
			labels[value] = maps.Clone(translations)
		}
		usageDisplaySchema[field] = billingexpr.UsageFieldSnapshot{
			Type: definition.Type, Unit: definition.Unit,
			UnitLabel: maps.Clone(definition.UnitLabel), Enum: append([]string(nil), definition.Enum...),
			Description: maps.Clone(definition.Description), EnumLabels: labels,
		}
	}

	snapshot := &billingexpr.BillingSnapshot{
		BillingMode: billing_setting.BillingModeTieredExpr, ModelName: modelName,
		ExprString: expression, ExprHash: billingexpr.ExprHashString(expression),
		RequestHeaders: billingInput.Headers, RequestParams: billingInput.Params,
		RequestTimeUnix: billingInput.At.Unix(),
		GroupRatio:      groupRatio.GroupRatio, EstimatedQuotaBeforeGroup: cost * common.QuotaPerUnit,
		EstimatedQuotaAfterGroup: quota, EstimatedTier: trace.MatchedTier,
		QuotaPerUnit: common.QuotaPerUnit, ExprVersion: billingexpr.ExprVersion(expression),
		TaskUsageBilling: true, NativeVideoMeter: nativeMeter, UsageFacts: facts,
		UsageSchema: usageDisplaySchema,
	}
	return taskExpressionBillingResult{priceData: priceData, snapshot: snapshot, requestInput: billingInput, clamp: clamp}, nil
}

func prepareLegacyTaskBilling(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.TaskAdaptor, modelName string) (types.PriceData, *common.QuotaClamp, error) {
	priceData, err := helper.ModelPriceHelperPerCall(c, info)
	if err != nil {
		return types.PriceData{}, nil, err
	}
	estimatedRatios := adaptor.EstimateBilling(c, info)
	for key, ratio := range estimatedRatios {
		priceData.AddOtherRatio(key, ratio)
	}
	if common.StringsContains(constant.TaskPricePatches, modelName) {
		return priceData, nil, nil
	}
	quota, clamp := common.QuotaFromFloatChecked(priceData.ApplyOtherRatiosToFloat(float64(priceData.Quota)))
	priceData.Quota = quota
	return priceData, clamp, nil
}

func legacyTaskBillingSnapshot(priceData types.PriceData) *billingexpr.LegacyTaskBillingSnapshot {
	return &billingexpr.LegacyTaskBillingSnapshot{
		Quota:       priceData.Quota,
		ModelPrice:  priceData.ModelPrice,
		ModelRatio:  priceData.ModelRatio,
		GroupRatio:  priceData.GroupRatioInfo.GroupRatio,
		OtherRatios: priceData.OtherRatios(),
		UsePrice:    priceData.UsePrice,
	}
}
