package tencentvideo

import (
	"math"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestAdjustBillingOnComplete(t *testing.T) {
	a := &TaskAdaptor{}
	fx := operation_setting.USDExchangeRate
	if fx <= 0 {
		fx = 7.3
	}

	// data carries the precise FinalUnitDeduction (RMB; 1 credit = ¥1)
	data := []byte(`{"Response":{"Status":"DONE","FinalUnitDeduction":"1.8","ResultVideoUrl":"x"}}`)
	task := &model.Task{Data: data}
	task.Quota = 250000 // submit deposit
	task.PrivateData.BillingContext = &model.TaskBillingContext{GroupRatio: 1}
	tr := &relaycommon.TaskInfo{TotalTokens: 2}

	// markup default 1.0: ¥1.8 ÷ fx × QuotaPerUnit × group 1
	os.Unsetenv(envMarkup)
	want := int(math.Ceil(1.8 * 1.0 / fx * common.QuotaPerUnit * 1))
	if got := a.AdjustBillingOnComplete(task, tr); got != want {
		t.Errorf("default markup: got %d, want %d", got, want)
	}

	// markup 1.5
	os.Setenv(envMarkup, "1.5")
	defer os.Unsetenv(envMarkup)
	want15 := int(math.Ceil(1.8 * 1.5 / fx * common.QuotaPerUnit * 1))
	if got := a.AdjustBillingOnComplete(task, tr); got != want15 {
		t.Errorf("markup 1.5: got %d, want %d", got, want15)
	}

	// group ratio applied (0.5×)
	task.PrivateData.BillingContext.GroupRatio = 0.5
	wantHalf := int(math.Ceil(1.8 * 1.5 / fx * common.QuotaPerUnit * 0.5))
	if got := a.AdjustBillingOnComplete(task, tr); got != wantHalf {
		t.Errorf("group ratio: got %d, want %d", got, wantHalf)
	}
}

func TestFinalUnitDeductionFallback(t *testing.T) {
	// no FinalUnitDeduction in data → fall back to TotalTokens
	task := &model.Task{Data: []byte(`{"Response":{"Status":"DONE"}}`)}
	tr := &relaycommon.TaskInfo{TotalTokens: 42}
	if got := finalUnitDeduction(task, tr); got != 42 {
		t.Errorf("fallback: got %v, want 42", got)
	}

	// precise float preferred over rounded tokens
	task.Data = []byte(`{"Response":{"FinalUnitDeduction":"12.7"}}`)
	if got := finalUnitDeduction(task, tr); got != 12.7 {
		t.Errorf("precise: got %v, want 12.7", got)
	}
}
