package hailuo

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

const (
	miniMaxH3Rate768PRMB        = 0.50
	miniMaxH3Rate2KRMB          = 0.80
	miniMaxH3MaxRate480PRMB     = 0.33
	miniMaxH3MaxRate768PRMB     = 0.50
	miniMaxH3FreeImageCount     = 5
	miniMaxH3ExtraImageRMB      = 0.20
	miniMaxH3FallbackUSDRMBRate = 7.3
)

func miniMaxH3RateRMB(modelName, resolution string) float64 {
	if isMiniMaxH3MaxModel(modelName) {
		if strings.EqualFold(strings.TrimSpace(resolution), Resolution480P) {
			return miniMaxH3MaxRate480PRMB
		}
		return miniMaxH3MaxRate768PRMB
	}
	if strings.EqualFold(strings.TrimSpace(resolution), Resolution2K) {
		return miniMaxH3Rate2KRMB
	}
	return miniMaxH3Rate768PRMB
}

func miniMaxH3CostRMB(modelName string, outputSeconds, inputVideoSeconds, inputImageCount int, resolution string) float64 {
	if outputSeconds < 0 {
		outputSeconds = 0
	}
	if inputVideoSeconds < 0 {
		inputVideoSeconds = 0
	}
	if inputImageCount < 0 {
		inputImageCount = 0
	}
	rate := miniMaxH3RateRMB(modelName, resolution)
	isMaxModel := isMiniMaxH3MaxModel(modelName)
	cost := float64(outputSeconds) * rate
	if !isMaxModel {
		cost += float64(inputVideoSeconds) * rate
	}
	if !isMaxModel && inputImageCount > miniMaxH3FreeImageCount {
		cost += float64(inputImageCount-miniMaxH3FreeImageCount) * miniMaxH3ExtraImageRMB
	}
	return cost
}

func miniMaxH3BillingDetails(modelName string, outputSeconds, inputVideoSeconds, inputImageCount int, resolution string, groupRatio float64) *relaycommon.VideoBillingDetails {
	if outputSeconds < 0 {
		outputSeconds = 0
	}
	if inputVideoSeconds < 0 {
		inputVideoSeconds = 0
	}
	if inputImageCount < 0 {
		inputImageCount = 0
	}
	if groupRatio <= 0 {
		groupRatio = 1
	}
	rate := miniMaxH3RateRMB(modelName, resolution)
	freeImageCount := miniMaxH3FreeImageCount
	billedImageCount := inputImageCount - freeImageCount
	imageUnitRate := miniMaxH3ExtraImageRMB
	if isMiniMaxH3MaxModel(modelName) {
		inputVideoSeconds = 0
		freeImageCount = inputImageCount
		billedImageCount = 0
		imageUnitRate = 0
	} else if billedImageCount < 0 {
		billedImageCount = 0
	}
	outputCost := float64(outputSeconds) * rate
	referenceVideoCost := float64(inputVideoSeconds) * rate
	imageSurcharge := float64(billedImageCount) * imageUnitRate
	providerCost := outputCost + referenceVideoCost + imageSurcharge
	return &relaycommon.VideoBillingDetails{
		BillingMode:                "video_per_second",
		Currency:                   "CNY",
		Resolution:                 resolution,
		OutputUnitRate:             rate,
		OutputSeconds:              outputSeconds,
		OutputCost:                 outputCost,
		ReferenceVideoInputSeconds: inputVideoSeconds,
		ReferenceVideoCost:         referenceVideoCost,
		ImageCount:                 inputImageCount,
		FreeImageCount:             freeImageCount,
		BilledImageCount:           billedImageCount,
		ImageUnitRate:              imageUnitRate,
		ImageSurcharge:             imageSurcharge,
		ProviderCost:               providerCost,
		GroupRatio:                 groupRatio,
		FinalCost:                  providerCost * groupRatio,
	}
}

func miniMaxH3USDRMBRate() float64 {
	rate := operation_setting.USDExchangeRate
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return miniMaxH3FallbackUSDRMBRate
	}
	return rate
}

// EstimateBilling reserves H3's known output and image cost, or H3 Max's
// output-only cost. H3 reference-video seconds are reported only after
// completion and are reconciled in AdjustBillingOnComplete.
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if !isMiniMaxH3Info(info) {
		return nil
	}
	request, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	modelName := info.UpstreamModelName
	if !isMiniMaxH3Model(modelName) {
		modelName = info.OriginModelName
	}
	costRMB := miniMaxH3CostRMB(modelName, request.Duration, 0, request.ImageCount, request.EffectiveResolution)
	if costRMB <= 0 || info.PriceData.ModelRatio <= 0 {
		return nil
	}
	// ModelPriceHelperPerCall seeds ratio-priced tasks at model_ratio / 2 USD.
	// This multiplier replaces that deposit with MiniMax's actual RMB estimate.
	baseCostUSD := info.PriceData.ModelRatio / 2
	ratio := costRMB / miniMaxH3USDRMBRate() / baseCostUSD
	if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 1) {
		return nil
	}
	info.VideoBilling = miniMaxH3BillingDetails(
		modelName,
		request.Duration,
		0,
		request.ImageCount,
		request.EffectiveResolution,
		info.PriceData.GroupRatioInfo.GroupRatio,
	)
	info.VideoBilling.Estimated = true
	return map[string]float64{
		"minimax_h3_cost": ratio,
	}
}

// AdjustBillingOnComplete settles against the usage returned by MiniMax. H3
// charges output seconds, reference-video seconds, and images beyond the first
// five; H3 Max charges output seconds only.
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if task == nil || (!isMiniMaxH3Model(task.Properties.OriginModelName) && !isMiniMaxH3Model(task.Properties.UpstreamModelName)) {
		return 0
	}

	var response miniMaxH3TaskResponse
	if err := common.Unmarshal(task.Data, &response); err != nil || response.Task == nil {
		return task.Quota
	}
	h3Task := response.Task
	modelName := task.Properties.UpstreamModelName
	if !isMiniMaxH3Model(modelName) {
		modelName = task.Properties.OriginModelName
	}
	if isMiniMaxH3Model(h3Task.Model) {
		modelName = h3Task.Model
	}
	outputSeconds := h3Task.Duration
	inputVideoSeconds := 0
	inputImageCount := 0
	if h3Task.Usage != nil {
		if h3Task.Usage.OutputSeconds > 0 {
			outputSeconds = h3Task.Usage.OutputSeconds
		}
		inputVideoSeconds = h3Task.Usage.InputSeconds
		if inputVideoSeconds == 0 && h3Task.Usage.TotalSeconds > outputSeconds {
			inputVideoSeconds = h3Task.Usage.TotalSeconds - outputSeconds
		}
		inputImageCount = h3Task.Usage.InputImageCount
	}

	resolution := h3Task.Resolution
	if snapshot := task.PrivateData.RequestSnapshot; snapshot != nil {
		if outputSeconds <= 0 {
			outputSeconds = snapshot.Duration
		}
		if inputImageCount == 0 {
			inputImageCount = snapshot.ImageCount
		}
		if resolution == "" {
			resolution = snapshot.ResolutionEffective
			if resolution == "" {
				resolution = snapshot.ResolutionRequested
			}
		}
	}
	if outputSeconds > miniMaxH3MaxDuration {
		outputSeconds = miniMaxH3MaxDuration
	}
	if inputVideoSeconds > miniMaxH3MaxDuration {
		inputVideoSeconds = miniMaxH3MaxDuration
	}
	if inputImageCount > miniMaxH3MaxImageCount {
		inputImageCount = miniMaxH3MaxImageCount
	}

	costRMB := miniMaxH3CostRMB(modelName, outputSeconds, inputVideoSeconds, inputImageCount, resolution)
	if costRMB <= 0 {
		return task.Quota
	}
	groupRatio := 1.0
	if billing := task.PrivateData.BillingContext; billing != nil {
		if billing.GroupRatio == 0 {
			return task.Quota
		}
		if billing.GroupRatio > 0 {
			groupRatio = billing.GroupRatio
		}
	}
	quota, clamp := common.QuotaRoundChecked(costRMB / miniMaxH3USDRMBRate() * common.QuotaPerUnit * groupRatio)
	if taskResult != nil && clamp != nil {
		taskResult.QuotaClamp = clamp
	}
	if quota <= 0 {
		return task.Quota
	}
	details := miniMaxH3BillingDetails(modelName, outputSeconds, inputVideoSeconds, inputImageCount, resolution, groupRatio)
	details.AspectRatio = h3Task.Ratio
	if details.AspectRatio == "" {
		if snapshot := task.PrivateData.RequestSnapshot; snapshot != nil {
			details.AspectRatio = snapshot.AspectRatio
		}
	}
	details.PreConsumedQuota = task.Quota
	details.SettlementDeltaQuota = quota - task.Quota
	details.FinalQuota = quota
	if taskResult != nil {
		taskResult.VideoBilling = details
	}
	if task.PrivateData.BillingContext != nil {
		task.PrivateData.BillingContext.VideoBilling = details
	}
	return quota
}
