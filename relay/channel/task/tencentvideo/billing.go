package tencentvideo

import (
	"math"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// Post-paid billing knobs (env, isolated from new-api's model price/ratio UI):
//
//	TENCENT_VIDEO_MARKUP       — markup multiplier over Tencent's actual cost.
//	                             FinalUnitDeduction is denominated in RMB
//	                             (1 credit = ¥1). 1.0 = sell at cost, 1.5 = +50%.
//	                             Default 1.0. RMB→USD uses the system
//	                             USDExchangeRate, consistent with top-up.
//	TENCENT_VIDEO_DEPOSIT_USD  — submit-time pre-charge in USD (default 0.5),
//	                             settled against actual cost on completion.
const (
	envMarkup     = "TENCENT_VIDEO_MARKUP"
	envDepositUSD = "TENCENT_VIDEO_DEPOSIT_USD"
)

// EstimateBilling overrides the submit-time pre-charge with a small fixed
// deposit (in USD), scaled by the user's group ratio. The real charge is
// reconciled in AdjustBillingOnComplete once Tencent reports actual usage.
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	depositUSD := common.GetEnvOrDefaultFloat64(envDepositUSD, 0.5)
	groupRatio := info.PriceData.GroupRatioInfo.GroupRatio
	if groupRatio <= 0 {
		groupRatio = 1
	}
	deposit := int(depositUSD * common.QuotaPerUnit * groupRatio)
	if deposit < 0 {
		deposit = 0
	}
	// Directly set the pre-charge; return nil so no extra ratios are applied.
	info.PriceData.Quota = deposit
	return nil
}

// AdjustBillingOnComplete computes the final quota from Tencent's actual
// FinalUnitDeduction. That value is the cost in RMB (1 credit = ¥1), so:
//
//	quota = ceil(FinalUnitDeduction(¥) × markup ÷ USDExchangeRate × QuotaPerUnit × groupRatio)
//
// It always returns a positive quota on success so the settlement flow never
// falls through to the generic token-recalc path (priority 2 in
// settleTaskBillingOnComplete), which would mis-price using the seeded model
// ratio. When usage is unavailable it returns the submit deposit unchanged.
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	credits := finalUnitDeduction(task, taskResult)
	if credits <= 0 {
		return task.Quota
	}

	markup := common.GetEnvOrDefaultFloat64(envMarkup, 1.0)
	if markup <= 0 {
		markup = 1.0
	}
	fx := operation_setting.USDExchangeRate
	if fx <= 0 {
		fx = 7.3
	}

	groupRatio := 1.0
	if bc := task.PrivateData.BillingContext; bc != nil && bc.GroupRatio > 0 {
		groupRatio = bc.GroupRatio
	}

	quota := int(math.Ceil(credits * markup / fx * common.QuotaPerUnit * groupRatio))
	if quota <= 0 {
		return task.Quota
	}
	return quota
}

// finalUnitDeduction extracts the precise FinalUnitDeduction float from the
// stored describe response, falling back to the rounded TotalTokens.
func finalUnitDeduction(task *model.Task, taskResult *relaycommon.TaskInfo) float64 {
	var dResp describeResponse
	if err := common.Unmarshal(task.Data, &dResp); err == nil {
		if f, err := strconv.ParseFloat(dResp.Response.FinalUnitDeduction, 64); err == nil && f > 0 {
			return f
		}
	}
	if taskResult != nil && taskResult.TotalTokens > 0 {
		return float64(taskResult.TotalTokens)
	}
	return 0
}
