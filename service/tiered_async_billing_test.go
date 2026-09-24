package service

import (
	"context"
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTieredAsyncSettlementBalancesFrozenSnapshots(t *testing.T) {
	for _, tc := range []struct {
		name, expression                         string
		unit                                     billingexpr.BillingUnit
		prompt, completion, count, expectedQuota int
		inputDetails                             string
	}{
		{"fixed without usage", `tier("request", fixed(0.025))`, billingexpr.BillingUnitRequest, 0, 0, 1, 12500, ""},
		{"explicit zero releases reservation", `tier("free", fixed(0))`, billingexpr.BillingUnitRequest, 0, 0, 1, 0, ""},
		{"actual image quantity", `tier("request", fixed(0.025))*image_count`, billingexpr.BillingUnitRequest, 0, 0, 2, 25000, ""},
		{"mixed actual token branch keeps unpriced image output", `len>500 ? tier("request", fixed(0.04)) : tier("tokens", p*2+c*10)`, billingexpr.BillingUnitRequest, 100, 100, 1, 600, ""},
		{"image cache overlap", `tier("image", p*2+cr*0.5+img*5+img_cr*0.1+img_o*10+c*0)`, billingexpr.BillingUnitToken, 1000, 200, 1, 2110, `{"cached_tokens":400,"image_tokens":500,"cached_tokens_details":{"image_tokens":200,"text_tokens":200}}`},
		{"historical snapshot keeps previous normalization", `len<=150 ? tier("low", p*2+c*10) : tier("high", p*4+c*20)`, "", 100, 100, 1, 200, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			const id, remaining, reserved = 71, 1000000, 50000
			seedUser(t, id, remaining)
			seedToken(t, id, id, "syntheticasyncbilling", remaining)
			seedChannel(t, id)
			seedChargedAccounting(t, id, id, id, reserved, 1)
			task := makeTask(id, id, reserved, id, BillingSourceWallet, 0)
			task.Status = model.TaskStatusSuccess
			task.PrivateData.SubmitLogID = seedConsumeLog(t, task, reserved, map[string]interface{}{})
			count := 4
			snapshot := makeSnapshot(tc.expression, 1, 1000, 0)
			snapshot.EstimatedBillingUnit = tc.unit
			if tc.name == "actual image quantity" {
				snapshot.EstimatedImageCount = &count
			}
			var err error
			task.PrivateData.BillingContext.TieredSnapshot, err = common.Marshal(snapshot)
			require.NoError(t, err)
			task.PrivateData.BillingContext.TieredRequestBody = []byte(`{"n":4}`)
			require.NoError(t, model.DB.Create(task).Error)
			facts := map[string]interface{}{"generated_image_count": tc.count, "image_output_tokens": tc.completion}
			if tc.inputDetails != "" {
				var details dto.InputTokenDetails
				require.NoError(t, common.UnmarshalJsonStr(tc.inputDetails, &details))
				facts["input_token_details"] = details
			}
			SettleAsyncImageTaskBilling(context.Background(), task, tc.prompt, tc.completion, facts)
			assert.Equal(t, tc.expectedQuota, task.Quota)
			assert.Equal(t, remaining+reserved-tc.expectedQuota, getUserQuota(t, id))
			assert.Equal(t, remaining+reserved-tc.expectedQuota, getTokenRemainQuota(t, id))
			assert.Equal(t, tc.expectedQuota, getTokenUsedQuota(t, id))
			used, requests := getUserUsageAccounting(t, id)
			assert.Equal(t, tc.expectedQuota, used)
			assert.Equal(t, 1, requests)
			assert.Equal(t, int64(tc.expectedQuota), getChannelUsedQuota(t, id))
			var log model.Log
			require.NoError(t, model.LOG_DB.First(&log, task.PrivateData.SubmitLogID).Error)
			assert.Equal(t, tc.expectedQuota, log.Quota)
			// Repeated completion must not deduct or refund the same difference twice.
			SettleAsyncImageTaskBilling(context.Background(), task, tc.prompt, tc.completion, facts)
			assert.Equal(t, remaining+reserved-tc.expectedQuota, getUserQuota(t, id))
			assert.Equal(t, remaining+reserved-tc.expectedQuota, getTokenRemainQuota(t, id))
		})
	}
}

func TestTieredAsyncZeroPriceRestoresSubscriptionAndToken(t *testing.T) {
	truncate(t)
	const id, remaining, reserved = 72, 10000, 5000
	seedUser(t, id, remaining)
	seedToken(t, id, id, "syntheticsubscriptionbilling", remaining)
	seedChannel(t, id)
	seedSubscription(t, id, id, 100000, reserved)
	seedChargedAccounting(t, id, id, id, reserved, 1)
	task := makeTask(id, id, reserved, id, BillingSourceSubscription, id)
	task.Status = model.TaskStatusSuccess
	task.PrivateData.SubmitLogID = seedConsumeLog(t, task, reserved, map[string]interface{}{})
	snapshot := makeSnapshot(`tier("free", fixed(0))`, 1, 500, 0)
	snapshot.EstimatedBillingUnit = billingexpr.BillingUnitRequest
	var err error
	task.PrivateData.BillingContext.TieredSnapshot, err = common.Marshal(snapshot)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(task).Error)
	SettleAsyncImageTaskBilling(context.Background(), task, 0, 0, map[string]interface{}{"generated_image_count": 1})
	assert.Zero(t, task.Quota)
	assert.Zero(t, getSubscriptionUsed(t, id))
	assert.Equal(t, remaining, getUserQuota(t, id))
	assert.Equal(t, remaining+reserved, getTokenRemainQuota(t, id))
	assert.Zero(t, getTokenUsedQuota(t, id))
	used, requests := getUserUsageAccounting(t, id)
	assert.Zero(t, used)
	assert.Equal(t, 1, requests)
	assert.Zero(t, getChannelUsedQuota(t, id))
	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, task.PrivateData.SubmitLogID).Error)
	assert.Zero(t, log.Quota)
}

func TestTieredAsyncClampKeepsAccountingAndCompletionAudit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reserved int
	}{
		{"charge difference", 5000},
		{"already reserved bounded amount", math.MaxInt32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			const id = 73
			reserved := tc.reserved
			seedUser(t, id, math.MaxInt32-reserved)
			seedToken(t, id, id, "syntheticclampaudit", math.MaxInt32-reserved)
			seedChannel(t, id)
			seedChargedAccounting(t, id, id, id, reserved, 1)
			task := makeTask(id, id, reserved, id, BillingSourceWallet, 0)
			task.Status = model.TaskStatusSuccess
			task.PrivateData.RequestID = "synthetic-clamp-request"
			task.PrivateData.SubmitLogID = seedConsumeLog(t, task, reserved, map[string]interface{}{"admin_info": map[string]interface{}{"original_audit": true}})
			snap := makeSnapshot(`len>700 ? tier("huge", fixed(100000)) : tier("reservation", fixed(0.01))`, 1, 500, 0)
			snap.EstimatedBillingUnit = billingexpr.BillingUnitRequest
			var err error
			task.PrivateData.BillingContext.TieredSnapshot, err = common.Marshal(snap)
			require.NoError(t, err)
			require.NoError(t, model.DB.Create(task).Error)
			SettleAsyncImageTaskBilling(context.Background(), task, 1000, 200, nil)
			assert.Equal(t, math.MaxInt32, task.Quota)
			assert.Zero(t, getUserQuota(t, id))
			assert.Zero(t, getTokenRemainQuota(t, id))
			assert.Equal(t, math.MaxInt32, getTokenUsedQuota(t, id))
			used, requests := getUserUsageAccounting(t, id)
			assert.Equal(t, math.MaxInt32, used)
			assert.Equal(t, 1, requests)
			assert.Equal(t, int64(math.MaxInt32), getChannelUsedQuota(t, id))
			model.UpdateConsumeLogOnComplete(task.PrivateData.SubmitLogID, 1, 1000, 200, "completed", map[string]interface{}{"admin_info": map[string]interface{}{"completed_audit": true}})
			model.UpdateConsumeLogOther(task.PrivateData.SubmitLogID, map[string]interface{}{"admin_info": map[string]interface{}{"later_audit": true}})
			var log model.Log
			require.NoError(t, model.LOG_DB.First(&log, task.PrivateData.SubmitLogID).Error)
			assert.Equal(t, math.MaxInt32, log.Quota)
			other, err := common.StrToMap(log.Other)
			require.NoError(t, err)
			admin, ok := other["admin_info"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, true, admin["original_audit"])
			assert.Equal(t, true, admin["completed_audit"])
			assert.Equal(t, true, admin["later_audit"])
			marker, ok := admin["quota_saturation"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, float64(math.MaxInt32), marker["clamped"])
			assert.Equal(t, string(common.QuotaClampOverflow), marker["kind"])
		})
	}
}
