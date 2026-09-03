package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLogTaskConsumptionRecordsKlingMotionControlSettings(t *testing.T) {
	truncate(t)
	seedUser(t, 1, 10_000_000)
	gin.SetMode(gin.TestMode)

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/kling/motion-control/kling-3.0", nil)
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt: "Follow the reference motion",
		Metadata: map[string]interface{}{
			"settings": map[string]interface{}{
				"character_orientation": "video",
				"audio":                 "original",
				"resolution":            "1080p",
			},
		},
	})
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1,
		OriginModelName: "kling-3.0",
		UserGroup:       "vip",
		UsingGroup:      "official",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 343},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: "motionControl30"},
		PriceData: types.PriceData{
			Quota:          2_250_000,
			ModelPrice:     0.05,
			ModelRatio:     1,
			OtherRatios:    map[string]float64{"resolution": 1.75, "seconds": 5},
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
	}

	logID := LogTaskConsumption(context, relayInfo, "task-audit-1")
	require.NotZero(t, logID)
	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, logID).Error)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, "Follow the reference motion", other["prompt"])
	assert.Equal(t, "video", other["character_orientation"])
	assert.Equal(t, "original", other["video_audio"])
	assert.Equal(t, "1080p", other["resolution"])
	assert.Equal(t, "task-audit-1", other["task_id"])
	assert.Equal(t, "SUBMITTED", other["task_status"])
	assert.Equal(t, "vip", other["user_group"])
	billing, ok := other["billing"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 1.75, billing["other_ratios"].(map[string]interface{})["resolution"])
	assert.Equal(t, "model_price(0.05) × group_ratio(1) × resolution(1.75) × seconds(5)", billing["formula"])
	assert.Contains(t, log.Content, "resolution: 1.75, seconds: 5")
}

func TestBuildTaskRequestSnapshotRecordsNativeVideoResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Model: "dreamina-seedance-2-5-ep",
		Metadata: map[string]interface{}{
			"resolution": " 480p ",
		},
	})
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "dreamina-seedance-2-5-ep",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
	}

	snapshot := BuildTaskRequestSnapshot(context, relayInfo)

	require.NotNil(t, snapshot)
	assert.Equal(t, "480p", snapshot.ResolutionRequested)
	assert.Equal(t, "480p", snapshot.ResolutionEffective)
}

func TestFinalizeTaskConsumptionLogRecordsTerminalLifecycle(t *testing.T) {
	truncate(t)
	const chargedQuota = 4000
	task := makeTask(1, 1, chargedQuota, 0, BillingSourceWallet, 0)
	task.Status = model.TaskStatusFailure
	task.SubmitTime = 100
	task.FinishTime = 220
	task.Data = []byte(`{"status":"failed"}`)
	task.PrivateData.SubmitLogID = seedConsumeLog(t, task, chargedQuota, map[string]interface{}{
		"task_status": "SUBMITTED",
	})

	FinalizeTaskConsumptionLog(task, chargedQuota, chargedQuota, "succeeded", &relaycommon.TaskInfo{
		VideoBilling: &relaycommon.VideoBillingDetails{
			BillingMode:      "video_per_second",
			Currency:         "CNY",
			ProviderCost:     2,
			GroupRatio:       10,
			FinalCost:        20,
			PreConsumedQuota: chargedQuota,
			FinalQuota:       chargedQuota,
		},
	})

	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, task.PrivateData.SubmitLogID).Error)
	assert.Equal(t, 120, log.UseTime)
	assert.Equal(t, chargedQuota, log.Quota)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, "FAILURE", other["task_status"])
	assert.Equal(t, float64(chargedQuota), other["refunded_quota"])
	assert.Equal(t, float64(0), other["net_quota"])
	assert.Equal(t, "succeeded", other["refund_status"])
	assert.Equal(t, true, other["response_body_available"])
	assert.Equal(t, "video_per_second", other["billing_mode"])
	videoBilling, ok := other["video_billing"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(2), videoBilling["provider_cost"])
	assert.Equal(t, float64(20), videoBilling["final_cost"])
	assert.Equal(t, float64(chargedQuota), videoBilling["final_quota"])
}

func TestSanitizeTaskAuditResponseRedactsSecretsAndMediaBodies(t *testing.T) {
	body := []byte(`{"request_id":"req-upstream","authorization":"Bearer secret","nested":{"api_key":"sk-secret"},"image":"data:image/png;base64,AAAA","usage":{"cost_in_usd_ticks":50000000}}`)

	sanitized := SanitizeTaskAuditResponse(body)
	var result map[string]interface{}
	require.NoError(t, common.Unmarshal(sanitized, &result))
	assert.Equal(t, "req-upstream", result["request_id"])
	assert.Equal(t, "[redacted]", result["authorization"])
	assert.Equal(t, "[redacted]", result["nested"].(map[string]interface{})["api_key"])
	assert.Equal(t, "[omitted 26 bytes]", result["image"])
	assert.Equal(t, float64(50_000_000), result["usage"].(map[string]interface{})["cost_in_usd_ticks"])
}

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true

	if err := db.AutoMigrate(
		&model.Task{},
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.Channel{},
		&model.Midjourney{},
		&model.TopUp{},
		&model.UserSubscription{},
		&model.SystemTask{},
		&model.SystemTaskLock{},
	); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Seed helpers
// ---------------------------------------------------------------------------

func truncate(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM tasks")
		model.DB.Exec("DELETE FROM users")
		model.DB.Exec("DELETE FROM tokens")
		model.DB.Exec("DELETE FROM logs")
		model.DB.Exec("DELETE FROM channels")
		model.DB.Exec("DELETE FROM midjourneys")
		model.DB.Exec("DELETE FROM top_ups")
		model.DB.Exec("DELETE FROM user_subscriptions")
		model.DB.Exec("DELETE FROM system_task_locks")
		model.DB.Exec("DELETE FROM system_tasks")
	})
}

func seedUser(t *testing.T, id int, quota int) {
	t.Helper()
	user := &model.User{Id: id, Username: "test_user", Quota: quota, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
}

func seedToken(t *testing.T, id int, userId int, key string, remainQuota int) {
	t.Helper()
	token := &model.Token{
		Id:          id,
		UserId:      userId,
		Key:         key,
		Name:        "test_token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: remainQuota,
		UsedQuota:   0,
	}
	require.NoError(t, model.DB.Create(token).Error)
}

func seedSubscription(t *testing.T, id int, userId int, amountTotal int64, amountUsed int64) {
	t.Helper()
	sub := &model.UserSubscription{
		Id:          id,
		UserId:      userId,
		AmountTotal: amountTotal,
		AmountUsed:  amountUsed,
		Status:      "active",
		StartTime:   time.Now().Unix(),
		EndTime:     time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(sub).Error)
}

func seedChannel(t *testing.T, id int) {
	t.Helper()
	ch := &model.Channel{Id: id, Name: "test_channel", Key: "sk-test", Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(ch).Error)
}

func seedChargedAccounting(t *testing.T, userID, channelID, tokenID, quota, requestCount int) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"used_quota": quota, "request_count": requestCount,
	}).Error)
	if channelID > 0 {
		require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("used_quota", quota).Error)
	}
	if tokenID > 0 {
		require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", tokenID).Update("used_quota", quota).Error)
	}
}

func makeTask(userId, channelId, quota, tokenId int, billingSource string, subscriptionId int) *model.Task {
	return &model.Task{
		TaskID:    "task_" + time.Now().Format("150405.000"),
		UserId:    userId,
		ChannelId: channelId,
		Quota:     quota,
		Status:    model.TaskStatus(model.TaskStatusInProgress),
		Group:     "default",
		Data:      json.RawMessage(`{}`),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
		Properties: model.Properties{
			OriginModelName: "test-model",
		},
		PrivateData: model.TaskPrivateData{
			BillingSource:  billingSource,
			SubscriptionId: subscriptionId,
			TokenId:        tokenId,
			BillingContext: &model.TaskBillingContext{
				ModelPrice:      0.02,
				GroupRatio:      1.0,
				OriginModelName: "test-model",
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Read-back helpers
// ---------------------------------------------------------------------------

func getUserQuota(t *testing.T, id int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", id).First(&user).Error)
	return user.Quota
}

func getUserUsageAccounting(t *testing.T, id int) (int, int) {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("used_quota", "request_count").Where("id = ?", id).First(&user).Error)
	return user.UsedQuota, user.RequestCount
}

func getChannelUsedQuota(t *testing.T, id int) int64 {
	t.Helper()
	var channel model.Channel
	require.NoError(t, model.DB.Select("used_quota").Where("id = ?", id).First(&channel).Error)
	return channel.UsedQuota
}

func getTokenRemainQuota(t *testing.T, id int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("remain_quota").Where("id = ?", id).First(&token).Error)
	return token.RemainQuota
}

func getTokenUsedQuota(t *testing.T, id int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("used_quota").Where("id = ?", id).First(&token).Error)
	return token.UsedQuota
}

func getSubscriptionUsed(t *testing.T, id int) int64 {
	t.Helper()
	var sub model.UserSubscription
	require.NoError(t, model.DB.Select("amount_used").Where("id = ?", id).First(&sub).Error)
	return sub.AmountUsed
}

func getTaskQuota(t *testing.T, id int64) int {
	t.Helper()
	var task model.Task
	require.NoError(t, model.DB.Select("quota").Where("id = ?", id).First(&task).Error)
	return task.Quota
}

func getMidjourneyTask(t *testing.T, id int) model.Midjourney {
	t.Helper()
	var task model.Midjourney
	require.NoError(t, model.DB.First(&task, id).Error)
	return task
}

func getLastLog(t *testing.T) *model.Log {
	t.Helper()
	var log model.Log
	err := model.LOG_DB.Order("id desc").First(&log).Error
	if err != nil {
		return nil
	}
	return &log
}

func seedConsumeLog(t *testing.T, task *model.Task, quota int, other map[string]interface{}) int {
	t.Helper()
	log := &model.Log{
		UserId:    task.UserId,
		Username:  "test_user",
		CreatedAt: common.GetTimestamp(),
		Type:      model.LogTypeConsume,
		Content:   "submitted",
		ModelName: taskModelName(task),
		Quota:     quota,
		ChannelId: task.ChannelId,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     common.MapToJsonStr(other),
	}
	require.NoError(t, model.LOG_DB.Create(log).Error)
	return log.Id
}

func countLogs(t *testing.T) int64 {
	t.Helper()
	var count int64
	model.LOG_DB.Model(&model.Log{}).Count(&count)
	return count
}

func TestPrepareMidjourneyTaskBillingKeepsUnbilledMarkerClear(t *testing.T) {
	task := &model.Midjourney{Quota: 900, TokenId: 7, BillingChannelId: 8}
	prepared, err := PrepareMidjourneyTaskBilling(&relaycommon.RelayInfo{}, task, 900, false)
	require.NoError(t, err)
	assert.False(t, prepared)
	assert.Zero(t, task.Quota)
	assert.Zero(t, task.TokenId)
	assert.Zero(t, task.BillingChannelId)
}

func TestMidjourneyRefundRestoresEveryAccountingElementOnBillingChannel(t *testing.T) {
	truncate(t)
	const userID, tokenID, billingChannelID, executionChannelID = 50, 50, 50, 51
	const initialUserQuota, initialTokenQuota, chargedQuota = 10000, 5000, 3000
	seedUser(t, userID, initialUserQuota)
	seedToken(t, tokenID, userID, "sk-midjourney", initialTokenQuota)
	seedChannel(t, billingChannelID)
	seedChannel(t, executionChannelID)
	relayInfo := &relaycommon.RelayInfo{
		UserId: userID, TokenId: tokenID, TokenKey: "sk-midjourney", UserQuota: initialUserQuota,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: billingChannelID},
	}
	task := &model.Midjourney{UserId: userID, Action: "IMAGINE", MjId: "mj-accounting-refund", ChannelId: executionChannelID, Progress: "0%"}
	prepared, err := PrepareMidjourneyTaskBilling(relayInfo, task, chargedQuota, true)
	require.NoError(t, err)
	require.True(t, prepared)
	require.NoError(t, task.Insert())
	billed, err := SettleMidjourneyTaskBilling(relayInfo, task, prepared)
	require.NoError(t, err)
	require.True(t, billed)
	assert.Equal(t, initialUserQuota-chargedQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenQuota-chargedQuota, getTokenRemainQuota(t, tokenID))
	persisted := getMidjourneyTask(t, task.Id)
	assert.Equal(t, tokenID, persisted.TokenId)
	assert.Equal(t, billingChannelID, persisted.BillingChannelId)

	seedChargedAccounting(t, userID, billingChannelID, tokenID, chargedQuota, 1)
	assert.True(t, RefundMidjourneyQuota(context.Background(), task, "构图失败"))
	assert.Equal(t, initialUserQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenQuota, getTokenRemainQuota(t, tokenID))
	assert.Zero(t, getTokenUsedQuota(t, tokenID))
	usedQuota, requestCount := getUserUsageAccounting(t, userID)
	assert.Zero(t, usedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Zero(t, getChannelUsedQuota(t, billingChannelID))
	assert.Zero(t, getChannelUsedQuota(t, executionChannelID))
	persisted = getMidjourneyTask(t, task.Id)
	assert.Zero(t, persisted.Quota)

	assert.True(t, RefundMidjourneyQuota(context.Background(), task, "duplicate poll"))
	assert.Equal(t, int64(1), countLogs(t))
}

func TestSettleMidjourneyTaskBillingTokenFailureKeepsFundingRefundable(t *testing.T) {
	truncate(t)
	const userID, tokenID, channelID = 53, 53, 53
	const initialUserQuota, initialTokenQuota, chargedQuota = 10000, 5000, 3000
	seedUser(t, userID, initialUserQuota)
	seedToken(t, tokenID, userID, "sk-midjourney-token-failure", initialTokenQuota)
	seedChannel(t, channelID)
	relayInfo := &relaycommon.RelayInfo{
		UserId: userID, TokenId: tokenID, TokenKey: "sk-midjourney-token-failure", UserQuota: initialUserQuota,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: channelID},
	}
	task := &model.Midjourney{UserId: userID, MjId: "mj-token-failure", ChannelId: channelID}
	prepared, err := PrepareMidjourneyTaskBilling(relayInfo, task, chargedQuota, true)
	require.NoError(t, err)
	require.NoError(t, task.Insert())
	require.NoError(t, model.DB.Exec(`
		CREATE TRIGGER fail_midjourney_token_update
		BEFORE UPDATE ON tokens WHEN OLD.id = 53
		BEGIN SELECT RAISE(ABORT, 'forced token quota failure'); END;
	`).Error)
	t.Cleanup(func() { model.DB.Exec("DROP TRIGGER IF EXISTS fail_midjourney_token_update") })

	billed, err := SettleMidjourneyTaskBilling(relayInfo, task, prepared)
	require.Error(t, err)
	assert.True(t, billed)
	assert.Equal(t, initialUserQuota-chargedQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenQuota, getTokenRemainQuota(t, tokenID))
	persisted := getMidjourneyTask(t, task.Id)
	assert.Equal(t, chargedQuota, persisted.Quota)
	assert.Zero(t, persisted.TokenId)

	seedChargedAccounting(t, userID, channelID, 0, chargedQuota, 1)
	assert.True(t, RefundMidjourneyQuota(context.Background(), task, "token settlement failed"))
	assert.Equal(t, initialUserQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenQuota, getTokenRemainQuota(t, tokenID))
}

// ===========================================================================
// RefundTaskQuota tests
// ===========================================================================

func TestRefundTaskQuota_Wallet(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 1, 1, 1
	const initQuota, preConsumed = 10000, 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-test-key", tokenRemain)
	seedChannel(t, channelID)
	seedChargedAccounting(t, userID, channelID, tokenID, preConsumed, 1)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	require.NoError(t, model.DB.Create(task).Error)

	assert.True(t, RefundTaskQuota(ctx, task, "task failed: upstream error"))

	// User quota should increase by preConsumed
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))

	// Token remain_quota should increase, used_quota should decrease
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Zero(t, getTokenUsedQuota(t, tokenID))
	usedQuota, requestCount := getUserUsageAccounting(t, userID)
	assert.Zero(t, usedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Zero(t, getChannelUsedQuota(t, channelID))

	// A refund log should be created
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed, log.Quota)
	assert.Equal(t, "test-model", log.ModelName)
	assert.Zero(t, task.Quota)
	assert.Zero(t, getTaskQuota(t, task.ID))
}

func TestRefundTaskQuota_Subscription(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 2, 2, 2, 1
	const preConsumed = 2000
	const subTotal, subUsed int64 = 100000, 50000
	const tokenRemain = 8000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-key", tokenRemain)
	seedChannel(t, channelID)
	seedSubscription(t, subID, userID, subTotal, subUsed)
	seedChargedAccounting(t, userID, channelID, tokenID, preConsumed, 1)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceSubscription, subID)
	require.NoError(t, model.DB.Create(task).Error)

	assert.True(t, RefundTaskQuota(ctx, task, "subscription task failed"))

	// Subscription used should decrease by preConsumed
	assert.Equal(t, subUsed-int64(preConsumed), getSubscriptionUsed(t, subID))

	// Token should also be refunded
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Zero(t, getTokenUsedQuota(t, tokenID))
	usedQuota, requestCount := getUserUsageAccounting(t, userID)
	assert.Zero(t, usedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Zero(t, getChannelUsedQuota(t, channelID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Zero(t, getTaskQuota(t, task.ID))
}

func TestRefundTaskQuota_ZeroQuota(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 3
	seedUser(t, userID, 5000)

	task := makeTask(userID, 0, 0, 0, BillingSourceWallet, 0)

	assert.True(t, RefundTaskQuota(ctx, task, "zero quota task"))

	// No change to user quota
	assert.Equal(t, 5000, getUserQuota(t, userID))

	// No log created
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRefundTaskQuota_NoToken(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, channelID = 4, 4
	const initQuota, preConsumed = 10000, 1500

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)
	seedChargedAccounting(t, userID, channelID, 0, preConsumed, 1)

	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceWallet, 0) // TokenId=0
	require.NoError(t, model.DB.Create(task).Error)

	assert.True(t, RefundTaskQuota(ctx, task, "no token task failed"))

	// User quota refunded
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	usedQuota, requestCount := getUserUsageAccounting(t, userID)
	assert.Zero(t, usedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Zero(t, getChannelUsedQuota(t, channelID))

	// Log created
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Zero(t, getTaskQuota(t, task.ID))
}

func TestRefundTaskQuota_FundingFailureKeepsPendingMarker(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, channelID, preConsumed = 5, 5, 1200
	seedUser(t, userID, 5000)
	seedChannel(t, channelID)
	seedChargedAccounting(t, userID, channelID, 0, preConsumed, 1)
	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceSubscription, 9999)
	task.Status = model.TaskStatusFailure
	require.NoError(t, model.DB.Create(task).Error)

	assert.False(t, RefundTaskQuota(ctx, task, "subscription missing"))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, preConsumed, getTaskQuota(t, task.ID))
	usedQuota, requestCount := getUserUsageAccounting(t, userID)
	assert.Equal(t, preConsumed, usedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, int64(preConsumed), getChannelUsedQuota(t, channelID))
	assert.Equal(t, int64(0), countLogs(t))
}

// ===========================================================================
// RecalculateTaskQuota tests
// ===========================================================================

func TestRecalculate_PositiveDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 10, 10, 10
	const initQuota, preConsumed = 10000, 2000
	const actualQuota = 3000 // under-charged by 1000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-pos", tokenRemain)
	seedChannel(t, channelID)
	seedChargedAccounting(t, userID, channelID, tokenID, preConsumed, 1)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	submitLogID := seedConsumeLog(t, task, preConsumed, map[string]interface{}{"async_task_id": task.TaskID})
	task.PrivateData.SubmitLogID = submitLogID

	RecalculateTaskQuota(ctx, task, actualQuota, "adaptor adjustment")

	// User quota should decrease by the delta (1000 additional charge)
	assert.Equal(t, initQuota-(actualQuota-preConsumed), getUserQuota(t, userID))

	// Token should also be charged the delta
	assert.Equal(t, tokenRemain-(actualQuota-preConsumed), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, getTokenUsedQuota(t, tokenID))
	usedQuota, requestCount := getUserUsageAccounting(t, userID)
	assert.Equal(t, actualQuota, usedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, int64(actualQuota), getChannelUsedQuota(t, channelID))

	// task.Quota should be updated to actualQuota
	assert.Equal(t, actualQuota, task.Quota)

	// 单日志结算：不新建日志行，原提交日志金额更新为实际额度
	assert.Equal(t, int64(1), countLogs(t))
	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, submitLogID).Error)
	assert.Equal(t, actualQuota, log.Quota)
}

// 差额结算契约：只更新原提交日志（不产生新日志行），并写入 tiered 计费信息。
// 原先由已删除的 SettleTaskQuotaInSubmitLog 承担，回退上游逻辑后由 RecalculateTaskQuota 承担。
func TestRecalculate_Tiered_UpdatesOriginalSubmitLogOnly(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 16, 16, 16
	const initQuota, preConsumed = 10000, 2000
	const actualQuota = 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-settle-tiered", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	submitLogID := seedConsumeLog(t, task, preConsumed, map[string]interface{}{"async_task_id": task.TaskID})
	task.PrivateData.SubmitLogID = submitLogID

	expr := `tier("base", p * 2 + c * 30 + img * 2.5)`
	snapBytes, err := common.Marshal(billingexpr.BillingSnapshot{
		BillingMode:   "tiered_expr",
		ModelName:     "test-model",
		ExprString:    expr,
		EstimatedTier: "base",
		QuotaPerUnit:  common.QuotaPerUnit,
	})
	require.NoError(t, err)
	task.PrivateData.BillingContext.ModelPrice = 0
	task.PrivateData.BillingContext.TieredSnapshot = snapBytes

	RecalculateTaskQuota(ctx, task, actualQuota,
		"tiered_expr重算：p=1548, c=3568, img=1530, tier=base")

	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, actualQuota, task.Quota)
	assert.Equal(t, initQuota-(actualQuota-preConsumed), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain-(actualQuota-preConsumed), getTokenRemainQuota(t, tokenID))

	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, submitLogID).Error)
	assert.Equal(t, actualQuota, log.Quota)
	assert.Equal(t, "submitted", log.Content)

	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, "tiered_expr", other["billing_mode"])
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte(expr)), other["expr_b64"])
	assert.Equal(t, "base", other["matched_tier"])
	assert.Equal(t, float64(preConsumed), other["pre_consumed_quota"])
	assert.Equal(t, float64(actualQuota), other["actual_quota"])
	assert.Equal(t, "tiered_expr重算：p=1548, c=3568, img=1530, tier=base", other["settlement_reason"])
}

func TestSettleAsyncImageTaskBillingUsesFrozenRequestParameters(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 17, 17, 17
	const preConsumed = 36000
	seedUser(t, userID, 1_000_000)
	seedToken(t, tokenID, userID, "sk-seedream-tiered", 1_000_000)
	seedChannel(t, channelID)
	seedChargedAccounting(t, userID, channelID, tokenID, preConsumed, 1)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Properties.OriginModelName = "dola-seedream-5-0-pro-260628-ep"
	task.PrivateData.SubmitLogID = seedConsumeLog(t, task, preConsumed, map[string]interface{}{"async_task_id": task.TaskID})

	expr := `(param("size") == "1K" || (img_o > 0 && img_o <= 9218)) ? tier("standard", 45000 + (param("image.#") - 1) * 3000) : tier("high_resolution", 90000 + (param("image.#") - 1) * 3000)`
	snapBytes, err := common.Marshal(billingexpr.BillingSnapshot{
		BillingMode:   "tiered_expr",
		ModelName:     task.Properties.OriginModelName,
		ExprString:    expr,
		ExprHash:      billingexpr.ExprHashString(expr),
		GroupRatio:    0.8,
		EstimatedTier: "high_resolution",
		QuotaPerUnit:  common.QuotaPerUnit,
	})
	require.NoError(t, err)
	task.PrivateData.BillingContext.TieredSnapshot = snapBytes
	task.PrivateData.BillingContext.TieredRequestBody = json.RawMessage(`{"size":"2K","image":[{},{}],"images":[{},{}]}`)

	SettleAsyncImageTaskBilling(ctx, task, 0, 16428, map[string]interface{}{
		"image_output_tokens": 16428,
	})

	assert.Equal(t, 37200, task.Quota)
	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, task.PrivateData.SubmitLogID).Error)
	assert.Equal(t, 37200, log.Quota)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, "high_resolution", other["matched_tier"])
}

func TestRecalculate_NegativeDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 11, 11, 11
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000 // over-charged by 2000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-neg", tokenRemain)
	seedChannel(t, channelID)
	seedChargedAccounting(t, userID, channelID, tokenID, preConsumed, 1)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	submitLogID := seedConsumeLog(t, task, preConsumed, map[string]interface{}{"async_task_id": task.TaskID})
	task.PrivateData.SubmitLogID = submitLogID

	RecalculateTaskQuota(ctx, task, actualQuota, "adaptor adjustment")

	// User quota should increase by abs(delta) = 2000 (refund overpayment)
	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))

	// Token should be refunded the difference
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, getTokenUsedQuota(t, tokenID))
	usedQuota, requestCount := getUserUsageAccounting(t, userID)
	assert.Equal(t, actualQuota, usedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, int64(actualQuota), getChannelUsedQuota(t, channelID))

	// task.Quota updated
	assert.Equal(t, actualQuota, task.Quota)

	// 单日志结算：负差额也不新建退款日志行，只把原提交日志金额改小
	assert.Equal(t, int64(1), countLogs(t))
	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, submitLogID).Error)
	assert.Equal(t, actualQuota, log.Quota)
}

func TestRecalculate_ZeroDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 12
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)

	task := makeTask(userID, 0, preConsumed, 0, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, preConsumed, "exact match")

	// No change to user quota
	assert.Equal(t, initQuota, getUserQuota(t, userID))

	// No log created (delta is zero)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRecalculate_ActualQuotaZero(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 13
	const initQuota = 10000

	seedUser(t, userID, initQuota)

	task := makeTask(userID, 0, 5000, 0, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, 0, "zero actual")

	// No change (early return)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRecalculate_Subscription_NegativeDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 14, 14, 14, 2
	const preConsumed = 5000
	const actualQuota = 2000 // over-charged by 3000
	const subTotal, subUsed int64 = 100000, 50000
	const tokenRemain = 8000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-recalc", tokenRemain)
	seedChannel(t, channelID)
	seedSubscription(t, subID, userID, subTotal, subUsed)
	seedChargedAccounting(t, userID, channelID, tokenID, preConsumed, 1)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceSubscription, subID)

	RecalculateTaskQuota(ctx, task, actualQuota, "subscription over-charge")

	// Subscription used should decrease by delta (refund 3000)
	assert.Equal(t, subUsed-int64(preConsumed-actualQuota), getSubscriptionUsed(t, subID))

	// Token refunded
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, getTokenUsedQuota(t, tokenID))
	usedQuota, requestCount := getUserUsageAccounting(t, userID)
	assert.Equal(t, actualQuota, usedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, int64(actualQuota), getChannelUsedQuota(t, channelID))

	assert.Equal(t, actualQuota, task.Quota)

	// 单日志结算：差额调整不产生新日志行
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRecalculateTaskQuotaByTokensUsesFrozenGroupRatio(t *testing.T) {
	originalModelRatios := ratio_setting.ModelRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"dreamina-seedance-2-0-260128":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios))
	})

	cases := []struct {
		name              string
		billingContext    *model.TaskBillingContext
		wantQuota         int
		wantGroupInReason string
	}{
		{
			name: "discount special ratio",
			billingContext: &model.TaskBillingContext{
				GroupRatio:      0.85,
				OriginModelName: "dreamina-seedance-2-0-260128",
			},
			wantQuota:         850,
			wantGroupInReason: "groupRatio=0.85",
		},
		{
			name: "premium special ratio",
			billingContext: &model.TaskBillingContext{
				GroupRatio:      1.15,
				OriginModelName: "dreamina-seedance-2-0-260128",
			},
			wantQuota:         1150,
			wantGroupInReason: "groupRatio=1.15",
		},
		{
			name:              "legacy task without billing context",
			billingContext:    nil,
			wantQuota:         1000,
			wantGroupInReason: "groupRatio=1.00",
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			userID := 30 + i
			seedUser(t, userID, 10000)

			task := makeTask(userID, 0, 1000, 0, BillingSourceWallet, 0)
			task.Properties.OriginModelName = "dreamina-seedance-2-0-260128"
			task.PrivateData.BillingContext = tc.billingContext
			require.NoError(t, model.DB.Create(task).Error)
			task.PrivateData.SubmitLogID = seedConsumeLog(t, task, task.Quota, map[string]interface{}{})

			RecalculateTaskQuotaByTokens(context.Background(), task, 1000)

			assert.Equal(t, tc.wantQuota, task.Quota)
			assert.Equal(t, 10000-(tc.wantQuota-1000), getUserQuota(t, userID))

			var log model.Log
			require.NoError(t, model.LOG_DB.First(&log, task.PrivateData.SubmitLogID).Error)
			other, err := common.StrToMap(log.Other)
			require.NoError(t, err)
			assert.Contains(t, other["settlement_reason"], tc.wantGroupInReason)
		})
	}
}

// ===========================================================================
// CAS + Billing integration tests
// Simulates the flow in updateVideoSingleTask (service/task_polling.go)
// ===========================================================================

// simulatePollBilling reproduces the CAS + billing logic from updateVideoSingleTask.
// It takes a persisted task (already in DB), applies the new status, and performs
// the conditional update + billing exactly as the polling loop does.
func simulatePollBilling(ctx context.Context, task *model.Task, newStatus model.TaskStatus, actualQuota int) {
	snap := task.Snapshot()

	shouldRefund := false
	shouldSettle := false
	quota := task.Quota

	task.Status = newStatus
	switch string(newStatus) {
	case model.TaskStatusSuccess:
		task.Progress = "100%"
		task.FinishTime = 9999
		shouldSettle = true
	case model.TaskStatusFailure:
		task.Progress = "100%"
		task.FinishTime = 9999
		task.FailReason = "upstream error"
		if quota != 0 {
			shouldRefund = true
		}
	default:
		task.Progress = "50%"
	}

	isDone := task.Status == model.TaskStatus(model.TaskStatusSuccess) || task.Status == model.TaskStatus(model.TaskStatusFailure)
	if isDone && snap.Status != task.Status {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			shouldRefund = false
			shouldSettle = false
		} else if !won {
			shouldRefund = false
			shouldSettle = false
		}
	} else if !snap.Equal(task.Snapshot()) {
		_, _ = task.UpdateWithStatus(snap.Status)
	}

	if shouldSettle && actualQuota > 0 {
		RecalculateTaskQuota(ctx, task, actualQuota, "test settle")
	}
	if shouldRefund {
		RefundTaskQuota(ctx, task, task.FailReason)
	}
}

func TestCASGuardedRefund_Win(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 20, 20, 20
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusFailure), 0)

	// CAS wins: task in DB should now be FAILURE
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, reloaded.Status)
	assert.Zero(t, reloaded.Quota)

	// Refund should have happened
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestCASGuardedRefund_Lose(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 21, 21, 21
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-lose", tokenRemain)
	seedChannel(t, channelID)

	// Create task with IN_PROGRESS in DB
	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	// Simulate another process already transitioning to FAILURE
	model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Update("status", model.TaskStatusFailure)

	// Our process still has the old in-memory state (IN_PROGRESS) and tries to transition
	// task.Status is still IN_PROGRESS in the snapshot
	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusFailure), 0)

	// CAS lost: user quota should NOT change (no double refund)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))

	// No billing log should be created
	assert.Equal(t, int64(0), countLogs(t))
}

func TestCASGuardedSettle_Win(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 22, 22, 22
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000 // over-charged, should get partial refund
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-settle-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusSuccess), actualQuota)

	// CAS wins: task should be SUCCESS
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, reloaded.Status)

	// Settlement should refund the over-charge (5000 - 3000 = 2000 back to user)
	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	// task.Quota should be updated to actualQuota
	assert.Equal(t, actualQuota, task.Quota)
}

func TestNonTerminalUpdate_NoBilling(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, channelID = 23, 23
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	task.Progress = "20%"
	require.NoError(t, model.DB.Create(task).Error)

	// Simulate a non-terminal poll update (still IN_PROGRESS, progress changed)
	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusInProgress), 0)

	// User quota should NOT change
	assert.Equal(t, initQuota, getUserQuota(t, userID))

	// No billing log
	assert.Equal(t, int64(0), countLogs(t))

	// Task progress should be updated in DB
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.Equal(t, "50%", reloaded.Progress)
}

// ===========================================================================
// Mock adaptor for settleTaskBillingOnComplete tests
// ===========================================================================

type mockAdaptor struct {
	adjustReturn int
}

func (m *mockAdaptor) Init(_ *relaycommon.RelayInfo) {}
func (m *mockAdaptor) FetchTask(string, string, map[string]any, string) (*http.Response, error) {
	return nil, nil
}
func (m *mockAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) { return nil, nil }
func (m *mockAdaptor) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return m.adjustReturn
}

// ===========================================================================
// PerCallBilling tests — settleTaskBillingOnComplete
// ===========================================================================

func TestSettle_PerCallBilling_SkipsAdaptorAdjust(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 30, 30, 30
	const initQuota, preConsumed = 10000, 5000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-adaptor", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 2000}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Per-call: no adjustment despite adaptor returning 2000
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettle_PerCallBilling_SkipsTotalTokens(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 31, 31, 31
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 7000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-tokens", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 0}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, TotalTokens: 9999}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Per-call: no recalculation by tokens
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettle_RecordsPromptAndCompletionTokensInSubmitLog(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 33, 33, 33
	const preConsumed = 4000

	seedUser(t, userID, 10000)
	seedToken(t, tokenID, userID, "sk-settle-log-tokens", 7000)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true
	submitLogID := seedConsumeLog(t, task, preConsumed, map[string]interface{}{"async_task_id": task.TaskID})
	task.PrivateData.SubmitLogID = submitLogID

	settleTaskBillingOnComplete(ctx, &mockAdaptor{}, task, &relaycommon.TaskInfo{
		Status:           model.TaskStatusSuccess,
		CompletionTokens: 108872,
		TotalTokens:      108872,
	})

	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, submitLogID).Error)
	assert.Equal(t, 0, log.PromptTokens)
	assert.Equal(t, 108872, log.CompletionTokens)
	assert.Equal(t, preConsumed, log.Quota)
}

func TestSettle_NonPerCallBilling_AppliesAdaptorAdjustment(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 32, 32, 32
	const initQuota, preConsumed = 10000, 5000
	const adaptorQuota = 3000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-nonpercall-adj", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	// PerCallBilling defaults to false

	adaptor := &mockAdaptor{adjustReturn: adaptorQuota}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Non-per-call: adaptor adjustment applies (refund 2000)
	assert.Equal(t, initQuota+(preConsumed-adaptorQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-adaptorQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, adaptorQuota, task.Quota)

	// 单日志结算：差额调整不产生新日志行
	assert.Equal(t, int64(0), countLogs(t))
}

func TestApplyVideoTaskPollingResultSettlesRequestTimeTerminalTransitionOnce(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 34, 34, 34
	const initQuota, preConsumed, finalQuota = 10000, 5000, 3000
	const tokenRemain = 8000
	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "realtime-settle", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.SubmitLogID = seedConsumeLog(t, task, preConsumed, map[string]interface{}{"async_task_id": task.TaskID})
	require.NoError(t, model.DB.Create(task).Error)
	adaptor := &mockAdaptor{adjustReturn: finalQuota}
	result := &relaycommon.TaskInfo{
		Status:           model.TaskStatusSuccess,
		Progress:         "100%",
		PromptTokens:     100,
		CompletionTokens: 200,
		TotalTokens:      300,
	}

	require.NoError(t, ApplyVideoTaskPollingResult(ctx, adaptor, task, result, []byte(`{"status":"completed"}`)))
	assert.Equal(t, initQuota+(preConsumed-finalQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-finalQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, finalQuota, getTaskQuota(t, task.ID))

	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), stored.Status)
	assert.Equal(t, "100%", stored.Progress)

	require.NoError(t, ApplyVideoTaskPollingResult(ctx, adaptor, task, result, []byte(`{"status":"completed"}`)))
	assert.Equal(t, initQuota+(preConsumed-finalQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-finalQuota), getTokenRemainQuota(t, tokenID))
}

// ===========================================================================
// 统一退款原则：只看上游 token 消耗（上游是否已计费），不看是否返图
// ===========================================================================

func TestRefundFailedTask_UpstreamBilled_NoRefund(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 41, 41, 41
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-billed", 5000)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusFailure)
	task.FailReason = "上游未返回图片数据"
	// 方案A：上游返回 200 但没给图 → 已计费，即使一个 token 都没回显也不退款
	task.PrivateData.ErrorDetail = &model.TaskErrorDetail{UpstreamBilled: true}

	RefundFailedTaskQuotaByUpstreamUsage(ctx, task)

	// 上游已计费：不退款、不产生退款日志
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRefundFailedTask_NotBilled_Refunds(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 42, 42, 42
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-unbilled", 5000)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusFailure)
	task.FailReason = "上游未返回图片数据"
	// 非 200（真实上游错误）：UpstreamBilled 未置位 → 视为上游未计费 → 全额退款
	task.PrivateData.ErrorDetail = &model.TaskErrorDetail{UpstreamStatus: http.StatusBadGateway}

	RefundFailedTaskQuotaByUpstreamUsage(ctx, task)

	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed, log.Quota)
	assert.Contains(t, log.Other, "上游未返回图片数据")
}

func TestRefundZeroUsageTaskQuota_UsageOrImagesDecide(t *testing.T) {
	cases := []struct {
		name             string
		promptTokens     int
		completionTokens int
		imageCount       int
		wantRefund       bool
	}{
		{"零用量且无图_退款", 0, 0, 0, true},
		{"零用量但已返图_正常扣费不退款", 0, 0, 2, false},
		{"有输入消耗不退款_即使无图", 537, 0, 0, false},
		{"有输出消耗不退款", 0, 44, 0, false},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			ctx := context.Background()

			userID := 50 + i
			const preConsumed = 3000
			const initQuota = 10000
			seedUser(t, userID, initQuota)
			seedToken(t, userID, userID, "sk-zero-usage", 5000)
			seedChannel(t, userID)

			task := makeTask(userID, userID, preConsumed, userID, BillingSourceWallet, 0)
			task.Status = model.TaskStatus(model.TaskStatusSuccess)

			RefundZeroUsageTaskQuota(ctx, task, tc.promptTokens, tc.completionTokens, tc.imageCount, "test")

			if tc.wantRefund {
				assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
				log := getLastLog(t)
				require.NotNil(t, log)
				assert.Equal(t, model.LogTypeRefund, log.Type)
				assert.Contains(t, log.Other, "未计费")
			} else {
				assert.Equal(t, initQuota, getUserQuota(t, userID))
				assert.Equal(t, int64(0), countLogs(t))
			}
		})
	}
}
