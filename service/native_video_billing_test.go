package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/videoexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeVideoExpressionSettlesFrozenWalletAndTokenLedger(t *testing.T) {
	for _, tc := range []struct {
		name, meter, expression, body string
		facts                         map[string]any
		actual                        int
	}{
		{"seedance fractional cost", videoexpr.Seedance, `tier("original", u("tokens") * 37 / 1000000)`, `{"status":"completed","usage":{"completion_tokens":100000}}`, map[string]any{"tokens": 150000, "resolution": "720p", "video_input": "none"}, 4440},
		{"seedance zero usage", videoexpr.Seedance, `tier("original", u("tokens") * 37 / 1000000)`, `{"status":"completed","usage":{"completion_tokens":0}}`, map[string]any{"tokens": 150000, "resolution": "720p", "video_input": "none"}, 0},
		{"precise Kling units with frozen request", videoexpr.Kling, `param("rate") == 2 ? tier("frozen", u("units") * 2) : tier("wrong", u("units") * 99)`, `{"data":{"final_unit_deduction":"1.25"}}`, map[string]any{"units": 5}, 3000},
		{"missing usage keeps reservation", videoexpr.Seedance, `tier("original", u("tokens") * 37 / 1000000)`, `{"status":"completed"}`, map[string]any{"tokens": 150000}, 5000},
		{"negative usage keeps reservation", videoexpr.Kling, `tier("original", u("units") * 2)`, `{"data":{"final_unit_deduction":"-3"}}`, map[string]any{"units": 5}, 5000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			const userID, channelID, tokenID = 91, 191, 91
			const preConsumed, initialWallet, initialToken = 5000, 20000, 15000
			seedUser(t, userID, initialWallet)
			seedToken(t, tokenID, userID, "test-native-video-token", initialToken)
			seedTaskPollingChannel(t, channelID, true)
			task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
			task.Platform = "60"
			task.TaskID = "native-expression-" + tc.meter
			task.Properties.OriginModelName = "billing-video"
			task.PrivateData.BillingContext.OtherRatios = map[string]float64{"seconds": 100, "video_input": 0.5}
			task.PrivateData.BillingContext.TieredSnapshot = billingSnapshotJSON(t, &billingexpr.BillingSnapshot{
				ExprString: tc.expression, ExprHash: billingexpr.ExprHashString(tc.expression),
				GroupRatio: 1.2, QuotaPerUnit: 1000, ExprVersion: 1,
				TaskUsageBilling: true, NativeVideoMeter: tc.meter, UsageFacts: tc.facts,
				RequestParams: map[string]any{"rate": float64(2)}, RequestTimeUnix: 1234567890,
			})
			require.NoError(t, model.DB.Create(task).Error)
			task.PrivateData.SubmitLogID = seedConsumeLog(t, task, preConsumed, map[string]any{})
			result := &relaycommon.TaskInfo{Status: string(model.TaskStatusSuccess), TotalTokens: 999999}
			adaptor := &taskPollingFetchAdaptor{}
			require.NoError(t, ApplyVideoTaskPollingResult(context.Background(), adaptor, task, result, []byte(tc.body)))
			assert.Equal(t, tc.actual, task.Quota)
			assert.Equal(t, initialWallet+preConsumed-tc.actual, getUserQuota(t, userID))
			assert.Equal(t, initialToken+preConsumed-tc.actual, getTokenRemainQuota(t, tokenID))
			var log model.Log
			require.NoError(t, model.LOG_DB.First(&log, task.PrivateData.SubmitLogID).Error)
			assert.Equal(t, tc.actual, log.Quota)
			snap := readBillingSnapshot(t, task.PrivateData.BillingContext.TieredSnapshot)
			assert.Equal(t, tc.expression, snap.ExprString)
			assert.Equal(t, float64(1.2), snap.GroupRatio)
			assert.Equal(t, float64(1000), snap.QuotaPerUnit)
			assert.Equal(t, int64(1234567890), snap.RequestTimeUnix)
			// A repeated successful poll must not change either ledger a second time.
			require.NoError(t, ApplyVideoTaskPollingResult(context.Background(), adaptor, task, result, []byte(tc.body)))
			assert.Equal(t, initialWallet+preConsumed-tc.actual, getUserQuota(t, userID))
			assert.Equal(t, initialToken+preConsumed-tc.actual, getTokenRemainQuota(t, tokenID))
		})
	}
}

func TestNativeVideoExpressionFailedTaskRefundsReservation(t *testing.T) {
	truncate(t)
	seedUser(t, 92, 20000)
	seedTaskPollingChannel(t, 192, true)
	task := makeTask(92, 192, 5000, 0, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.TieredSnapshot = billingSnapshotJSON(t, &billingexpr.BillingSnapshot{
		ExprString: `tier("video", u("tokens") * 37 / 1000000)`,
		GroupRatio: 1, QuotaPerUnit: 1000, ExprVersion: 1,
		TaskUsageBilling: true, NativeVideoMeter: videoexpr.Seedance, UsageFacts: map[string]any{"tokens": 150000},
	})
	require.NoError(t, model.DB.Create(task).Error)
	require.NoError(t, ApplyVideoTaskPollingResult(context.Background(), &taskPollingFetchAdaptor{}, task,
		&relaycommon.TaskInfo{Status: string(model.TaskStatusFailure)}, []byte(`{"status":"failed"}`)))
	assert.Equal(t, 25000, getUserQuota(t, 92))
	assert.Zero(t, task.Quota)
}
