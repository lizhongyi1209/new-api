package gemini

import (
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

const (
	omniInputUSDPerMillion       = 1.50
	omniTextOutputUSDPerMillion  = 9.00
	omniVideoOutputUSDPerMillion = 17.50
)

func (a *TaskAdaptor) estimateOmniBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if info == nil || info.PriceData.UsePrice || info.PriceData.ModelRatio <= 0 {
		return nil
	}
	if _, err := relaycommon.GetTaskRequest(c); err != nil {
		return nil
	}

	videoTokens := omniMaxDurationSeconds * omniVideoTokensPerSecond
	weightedTokens := float64(omniEstimatedInputTokens)*info.PriceData.ModelRatio +
		float64(omniEstimatedThoughtTokens)*info.PriceData.ModelRatio*info.PriceData.CompletionRatio +
		float64(videoTokens)*info.PriceData.ModelRatio*info.PriceData.VideoCompletionRatio
	baseQuota := info.PriceData.ModelRatio / 2 * common.QuotaPerUnit
	reserveMultiplier := weightedTokens / baseQuota
	if reserveMultiplier <= 0 || math.IsNaN(reserveMultiplier) || math.IsInf(reserveMultiplier, 0) {
		return nil
	}

	groupRatio := info.PriceData.GroupRatioInfo.GroupRatio
	providerCost := omniProviderCost(omniEstimatedInputTokens, omniEstimatedThoughtTokens, videoTokens)
	info.VideoBilling = &relaycommon.VideoBillingDetails{
		BillingMode:         "video_modality_tokens",
		Currency:            "USD",
		Resolution:          "720p",
		OutputSeconds:       omniMaxDurationSeconds,
		ProviderCost:        providerCost,
		GroupRatio:          groupRatio,
		FinalCost:           providerCost * groupRatio,
		Estimated:           true,
		InputTokens:         omniEstimatedInputTokens,
		TextOutputTokens:    omniEstimatedThoughtTokens,
		VideoOutputTokens:   videoTokens,
		ThoughtTokens:       omniEstimatedThoughtTokens,
		InputUnitRate:       omniInputUSDPerMillion,
		TextOutputUnitRate:  omniTextOutputUSDPerMillion,
		VideoOutputUnitRate: omniVideoOutputUSDPerMillion,
	}
	return map[string]float64{"gemini_omni_token_reserve": reserveMultiplier}
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if task == nil || taskResult == nil || (!isGeminiOmniModel(task.Properties.OriginModelName) && !isGeminiOmniModel(task.Properties.UpstreamModelName)) {
		return 0
	}
	billing := task.PrivateData.BillingContext
	if billing == nil || billing.PerCallBilling || billing.ModelRatio <= 0 {
		return task.Quota
	}
	if billing.GroupRatio == 0 {
		return task.Quota
	}

	inputTokens := boundedOmniTokens(taskResult.PromptTokens)
	thoughtTokens := boundedOmniTokens(taskResult.ThoughtTokens)
	videoTokens := boundedOmniTokens(taskResult.OutputTokensByModality["video"])
	totalOutputTokens := taskResult.CompletionTokens - thoughtTokens
	if totalOutputTokens < 0 {
		totalOutputTokens = 0
	}
	textOutputTokens := totalOutputTokens - videoTokens
	if textOutputTokens < 0 {
		textOutputTokens = 0
	}
	textAndThoughtTokens := boundedOmniTokenSum(textOutputTokens, thoughtTokens)
	if inputTokens == 0 && textAndThoughtTokens == 0 && videoTokens == 0 {
		return task.Quota
	}

	completionRatio := billing.CompletionRatio
	if completionRatio < 0 || math.IsNaN(completionRatio) || math.IsInf(completionRatio, 0) {
		return task.Quota
	}
	videoCompletionRatio := billing.VideoCompletionRatio
	if videoCompletionRatio < 0 || math.IsNaN(videoCompletionRatio) || math.IsInf(videoCompletionRatio, 0) {
		return task.Quota
	}
	groupRatio := billing.GroupRatio
	weightedTokens := (float64(inputTokens)*billing.ModelRatio +
		float64(textAndThoughtTokens)*billing.ModelRatio*completionRatio +
		float64(videoTokens)*billing.ModelRatio*videoCompletionRatio) * groupRatio
	quota, clamp := common.QuotaFromFloatChecked(weightedTokens)
	if clamp != nil {
		taskResult.QuotaClamp = clamp
	}
	if quota <= 0 {
		return task.Quota
	}

	providerCost := omniProviderCost(inputTokens, textAndThoughtTokens, videoTokens)
	details := &relaycommon.VideoBillingDetails{
		BillingMode:          "video_modality_tokens",
		Currency:             "USD",
		Resolution:           "720p",
		OutputSeconds:        videoTokens / omniVideoTokensPerSecond,
		ProviderCost:         providerCost,
		GroupRatio:           groupRatio,
		FinalCost:            float64(quota) / common.QuotaPerUnit,
		PreConsumedQuota:     task.Quota,
		SettlementDeltaQuota: quota - task.Quota,
		FinalQuota:           quota,
		InputTokens:          inputTokens,
		TextOutputTokens:     textOutputTokens,
		VideoOutputTokens:    videoTokens,
		ThoughtTokens:        thoughtTokens,
		InputUnitRate:        omniInputUSDPerMillion,
		TextOutputUnitRate:   omniTextOutputUSDPerMillion,
		VideoOutputUnitRate:  omniVideoOutputUSDPerMillion,
	}
	taskResult.VideoBilling = details
	billing.VideoBilling = details
	return quota
}

func omniProviderCost(inputTokens int, textOutputTokens int, videoOutputTokens int) float64 {
	return (float64(boundedOmniTokens(inputTokens))*omniInputUSDPerMillion +
		float64(boundedOmniTokens(textOutputTokens))*omniTextOutputUSDPerMillion +
		float64(boundedOmniTokens(videoOutputTokens))*omniVideoOutputUSDPerMillion) / 1_000_000
}
