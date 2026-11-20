package service

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/videoexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWanMissingUsageWaitsForSettlementOrTimeout(t *testing.T) {
	for _, scenario := range []string{"native recovery", "pinned plugin recovery", "native timeout", "pinned plugin timeout"} {
		t.Run(scenario, func(t *testing.T) {
			truncate(t)
			const uid, tokenID, channelID = 44, 44, 144
			const reserve, balance = 3600000, 20000000
			seedUser(t, uid, balance)
			seedToken(t, tokenID, uid, "sk-wan-settlement", balance)
			seedTaskPollingChannel(t, channelID, true)
			task := makeTask(uid, channelID, reserve, tokenID, BillingSourceWallet, 0)
			task.TaskID = "task_wan_usage"
			task.Platform = "17"
			task.Status = model.TaskStatusInProgress
			task.SubmitTime = common.GetTimestamp()
			expr := `tier("video",u("billed_seconds") * 0.6)`
			snapshot := &billingexpr.BillingSnapshot{ExprString: expr, ExprHash: billingexpr.ExprHashString(expr), ExprVersion: 1, GroupRatio: 0.6, QuotaPerUnit: 500000, TaskUsageBilling: true, NativeVideoMeter: videoexpr.Wan30, UsageSchema: map[string]billingexpr.UsageFieldSnapshot{"seconds": {Type: "number"}, "input_video_seconds": {Type: "number"}, "billed_seconds": {Type: "number"}, "resolution": {Enum: []string{"480P", "720P", "1080P"}}}, UsageFacts: map[string]any{"seconds": float64(5), "input_video_seconds": float64(15), "billed_seconds": float64(20), "resolution": "720P"}}
			if scenario == "pinned plugin recovery" || scenario == "pinned plugin timeout" {
				task.Platform = constant.WanOfficialVideoPlugin
				snapshot.NativeVideoMeter = ""
				task.PrivateData.Execution = &model.TaskExecutionSnapshot{TaskPlugin: &model.TaskPluginSnapshot{Key: constant.WanOfficialVideoPlugin, Version: "1.0.0"}}
			}
			task.PrivateData.BillingContext.TieredSnapshot = billingSnapshotJSON(t, snapshot)
			require.NoError(t, model.DB.Create(task).Error)
			task.PrivateData.SubmitLogID = seedConsumeLog(t, task, reserve, map[string]interface{}{"async_task_id": task.TaskID})
			adaptor := &taskPollingFetchAdaptor{}
			result := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Url: "https://example.invalid/video.mp4"}
			require.NoError(t, ApplyVideoTaskPollingResult(context.Background(), adaptor, task, result, []byte(`{"output":{"task_status":"SUCCEEDED"},"usage":{"output_video_duration":5,"SR":720}}`)))
			var saved model.Task
			require.NoError(t, model.DB.First(&saved, task.ID).Error)
			require.EqualValues(t, model.TaskStatusInProgress, saved.Status)
			assert.Zero(t, saved.FinishTime)
			assert.Empty(t, saved.PrivateData.ResultURL)
			assert.Equal(t, reserve, saved.Quota)
			assert.Equal(t, balance, getUserQuota(t, uid))

			if scenario == "native timeout" || scenario == "pinned plugin timeout" {
				previous := constant.TaskTimeoutMinutes
				constant.TaskTimeoutMinutes = 1
				t.Cleanup(func() { constant.TaskTimeoutMinutes = previous })
				require.NoError(t, model.DB.Model(task).Update("submit_time", time.Now().Unix()-120).Error)
				sweepTimedOutTasks(context.Background())
				require.NoError(t, model.DB.First(&saved, task.ID).Error)
				assert.EqualValues(t, model.TaskStatusFailure, saved.Status)
				assert.Zero(t, saved.Quota)
				assert.Equal(t, balance+reserve, getUserQuota(t, uid))
				return
			}
			result = &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Url: "https://example.invalid/video.mp4", UsageFacts: map[string]any{"billed_seconds": float64(8)}}
			complete := []byte(`{"output":{"task_status":"SUCCEEDED"},"usage":{"output_video_duration":5,"input_video_duration":3,"SR":720}}`)
			require.NoError(t, ApplyVideoTaskPollingResult(context.Background(), adaptor, task, result, complete))
			require.NoError(t, model.DB.First(&saved, task.ID).Error)
			assert.EqualValues(t, model.TaskStatusSuccess, saved.Status)
			assert.Equal(t, 1440000, saved.Quota)
			assert.Equal(t, balance+reserve-1440000, getUserQuota(t, uid))
			assert.Equal(t, balance+reserve-1440000, getTokenRemainQuota(t, tokenID))
			require.NoError(t, ApplyVideoTaskPollingResult(context.Background(), adaptor, task, result, complete))
			assert.Equal(t, balance+reserve-1440000, getUserQuota(t, uid))
		})
	}
}
