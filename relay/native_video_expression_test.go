package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/videoexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeVideoExpressionReservesBeforeCallingUpstream(t *testing.T) {
	for _, tc := range []struct {
		name, expression, body, mapping, meter string
		wantCost                               float64
		wantTokens                             float64
		wantPriceError                         bool
	}{
		{"valid Seedance", `tier("original", u("tokens") * 37 / 1000000)`, `{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":5}`, `{"video-alias":"dreamina-seedance-2-0-fast-hc"}`, videoexpr.Seedance, 3.996, 108000, false},
		{"valid Seedance 2.5 automatic duration", `tier("original", u("tokens") * 70 / 1000000)`, `{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":-1}`, `{"video-alias":"dreamina-seedance-2-5-260628-max"}`, videoexpr.Seedance, 45.36, 648000, false},
		{"valid H3 Max Turbo 1080P", `u("resolution") == "480P" ? tier("video_480p", u("seconds") * 0.025) : (u("resolution") == "768P" ? tier("video_768p", u("seconds") * 0.040) : tier("video_1080p", u("seconds") * 0.080))`, `{"model":"video-alias","content":[{"type":"text","text":"video"}],"resolution":"1080P","duration":5,"ratio":"16:9"}`, `{"video-alias":"minimax-h3-max-turbo"}`, videoexpr.H3MaxTurbo, 0.4, 0, false},
		{"unknown meter field", `tier("bad", u("credits") * 1)`, `{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":5}`, `{"video-alias":"dreamina-seedance-2-0-fast-hc"}`, "", 0, 0, true},
		{"ordinary token expression", `tier("bad", p * 37)`, `{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":5}`, `{"video-alias":"dreamina-seedance-2-0-fast-hc"}`, "", 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			previousRedis := common.RedisEnabled
			common.RedisEnabled = false
			t.Cleanup(func() { common.RedisEnabled = previousRedis })
			db := setupRelayChannelDB(t)
			require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSubscription{}))
			require.NoError(t, db.Create(&model.User{Id: 97, Username: "video-empty-wallet", Quota: 0, Status: common.UserStatusEnabled}).Error)
			settings := config.GlobalConfig.Get("billing_setting")
			saved, err := config.ConfigToMap(settings)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, config.UpdateConfigFromMap(settings, saved)) })
			raw, err := common.Marshal(map[string]string{"video-alias": tc.expression})
			require.NoError(t, err)
			require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"billing_mode": `{"video-alias":"tiered_expr"}`, "billing_expr": string(raw)}))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeServiceInferenceVideo)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "video-alias")
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://upstream-must-not-be-called.invalid")
			c.Set("model_mapping", tc.mapping)
			c.Set("group", "default")
			info := &relaycommon.RelayInfo{UserId: 97, UserGroup: "default", UsingGroup: "default", OriginModelName: "video-alias", TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
			_, taskErr := RelayTaskSubmit(c, info)
			require.NotNil(t, taskErr)
			if tc.wantPriceError {
				assert.Equal(t, "model_price_error", taskErr.Code)
				assert.Nil(t, info.TieredBillingSnapshot)
				return
			}
			require.NotNil(t, info.TieredBillingSnapshot, "%+v", taskErr)
			assert.NotEqual(t, "model_price_error", taskErr.Code)
			assert.NotEqual(t, "do_request_failed", taskErr.Code)
			assert.Contains(t, taskErr.Code, "quota")
			assert.Equal(t, tc.meter, info.TieredBillingSnapshot.NativeVideoMeter)
			if tc.meter == videoexpr.Seedance {
				assert.Equal(t, tc.wantTokens, info.TieredBillingSnapshot.UsageFacts["tokens"])
				assert.Equal(t, "token", info.TieredBillingSnapshot.UsageSchema["tokens"].Unit)
			} else {
				assert.Equal(t, float64(5), info.TieredBillingSnapshot.UsageFacts["seconds"])
				assert.Equal(t, "1080P", info.TieredBillingSnapshot.UsageFacts["resolution"])
				assert.Equal(t, "second", info.TieredBillingSnapshot.UsageSchema["seconds"].Unit)
			}
			cost, _, err := billingexpr.RunExprWithRequest(tc.expression, billingexpr.TokenParams{}, info.TieredBillingSnapshot.TaskRequestInput(info.TieredBillingSnapshot.UsageFacts))
			require.NoError(t, err)
			assert.InDelta(t, tc.wantCost, cost, 0.000000001)
			assert.Equal(t, common.QuotaRound(cost*common.QuotaPerUnit), info.PriceData.Quota)
		})
	}
}

func TestNativeSeedanceExpressionFailureFallsBackToLegacyRatio(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedis })

	db := setupRelayChannelDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSubscription{}))
	require.NoError(t, db.Create(&model.User{Id: 98, Username: "video-fallback-empty-wallet", Quota: 0, Status: common.UserStatusEnabled}).Error)

	settings := config.GlobalConfig.Get("billing_setting")
	savedBilling, err := config.ConfigToMap(settings)
	require.NoError(t, err)
	savedRatios := ratio_setting.ModelRatio2JSONString()
	savedPrices := ratio_setting.ModelPrice2JSONString()
	savedGroups := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(settings, savedBilling))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroups))
	})

	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"video-alias":35}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{
		"billing_mode": `{"video-alias":"tiered_expr"}`,
		"billing_expr": `{"video-alias":"tier(\"broken\", u(\"unknown\") * 70 / 1000000)"}`,
	}))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":5}`))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeServiceInferenceVideo)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "video-alias")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://upstream-must-not-be-called.invalid")
	c.Set("model_mapping", `{"video-alias":"dreamina-seedance-2-5-260628-max"}`)
	c.Set("group", "default")
	info := &relaycommon.RelayInfo{UserId: 98, UserGroup: "default", UsingGroup: "default", OriginModelName: "video-alias", TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	_, taskErr := RelayTaskSubmit(c, info)
	require.NotNil(t, taskErr)
	assert.NotEqual(t, "model_price_error", taskErr.Code)
	assert.Contains(t, taskErr.Code, "quota")
	assert.Nil(t, info.TieredBillingSnapshot)
	assert.Equal(t, float64(35), info.PriceData.ModelRatio)
	assert.NotEmpty(t, info.TaskBillingFallbackReason)

	// A request-side usage validation error must never enter the compatibility
	// path, even when the expression itself is broken and a legacy ratio exists.
	invalidContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	invalidContext.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":5,"resolution":"8k"}`))
	invalidContext.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(invalidContext, constant.ContextKeyChannelType, constant.ChannelTypeServiceInferenceVideo)
	common.SetContextKey(invalidContext, constant.ContextKeyOriginalModel, "video-alias")
	common.SetContextKey(invalidContext, constant.ContextKeyChannelBaseUrl, "https://upstream-must-not-be-called.invalid")
	invalidContext.Set("model_mapping", `{"video-alias":"dreamina-seedance-2-5-260628-max"}`)
	invalidContext.Set("group", "default")
	invalidInfo := &relaycommon.RelayInfo{UserId: 98, UserGroup: "default", UsingGroup: "default", OriginModelName: "video-alias", TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	_, invalidErr := RelayTaskSubmit(invalidContext, invalidInfo)
	require.NotNil(t, invalidErr)
	assert.Equal(t, "plugin_usage_invalid", invalidErr.Code)
	assert.Empty(t, invalidInfo.TaskBillingFallbackReason)
	assert.Nil(t, invalidInfo.TieredBillingSnapshot)

	// Request-controlled expression projection failures are also rejected. A
	// client must not be able to force fallback with an oversized scalar.
	require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{
		"billing_mode": `{"video-alias":"tiered_expr"}`,
		"billing_expr": `{"video-alias":"param(\"callback_url\") == \"preferred\" ? tier(\"preferred\", u(\"tokens\") * 70 / 1000000) : tier(\"standard\", u(\"tokens\") * 70 / 1000000)"}`,
	}))
	projectionContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	projectionBody := `{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":5,"callback_url":"` + strings.Repeat("x", 257) + `"}`
	projectionContext.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(projectionBody))
	projectionContext.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(projectionContext, constant.ContextKeyChannelType, constant.ChannelTypeServiceInferenceVideo)
	common.SetContextKey(projectionContext, constant.ContextKeyOriginalModel, "video-alias")
	common.SetContextKey(projectionContext, constant.ContextKeyChannelBaseUrl, "https://upstream-must-not-be-called.invalid")
	projectionContext.Set("model_mapping", `{"video-alias":"dreamina-seedance-2-5-260628-max"}`)
	projectionContext.Set("group", "default")
	projectionInfo := &relaycommon.RelayInfo{UserId: 98, UserGroup: "default", UsingGroup: "default", OriginModelName: "video-alias", TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	_, projectionErr := RelayTaskSubmit(projectionContext, projectionInfo)
	require.NotNil(t, projectionErr)
	assert.Equal(t, "task_plugin_price_invalid", projectionErr.Code)
	assert.Empty(t, projectionInfo.TaskBillingFallbackReason)
	assert.Nil(t, projectionInfo.TieredBillingSnapshot)

	// A valid expression keeps the current ratio frozen with the task so a
	// later settlement failure does not depend on mutable pricing settings.
	require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{
		"billing_mode": `{"video-alias":"tiered_expr"}`,
		"billing_expr": `{"video-alias":"tier(\"standard\", u(\"tokens\") * 70 / 1000000)"}`,
	}))
	frozenContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	frozenContext.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":5}`))
	frozenContext.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(frozenContext, constant.ContextKeyChannelType, constant.ChannelTypeServiceInferenceVideo)
	common.SetContextKey(frozenContext, constant.ContextKeyOriginalModel, "video-alias")
	common.SetContextKey(frozenContext, constant.ContextKeyChannelBaseUrl, "https://upstream-must-not-be-called.invalid")
	frozenContext.Set("model_mapping", `{"video-alias":"dreamina-seedance-2-5-260628-max"}`)
	frozenContext.Set("group", "default")
	frozenInfo := &relaycommon.RelayInfo{UserId: 98, UserGroup: "default", UsingGroup: "default", OriginModelName: "video-alias", TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	_, frozenErr := RelayTaskSubmit(frozenContext, frozenInfo)
	require.NotNil(t, frozenErr)
	require.NotNil(t, frozenInfo.TieredBillingSnapshot)
	require.NotNil(t, frozenInfo.TieredBillingSnapshot.LegacyTaskBilling)
	assert.Equal(t, float64(35), frozenInfo.TieredBillingSnapshot.LegacyTaskBilling.ModelRatio)
	assert.Equal(t, common.QuotaFromFloat(35.0/2*common.QuotaPerUnit), frozenInfo.TieredBillingSnapshot.LegacyTaskBilling.Quota)
}
