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
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeVideoExpressionReservesBeforeCallingUpstream(t *testing.T) {
	for _, tc := range []struct {
		name, expression, body, mapping, meter string
		wantCost                               float64
		wantPriceError                         bool
	}{
		{"valid Seedance", `tier("original", u("tokens") * 37 / 1000000)`, `{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":5}`, `{"video-alias":"dreamina-seedance-2-0-fast-hc"}`, videoexpr.Seedance, 3.996, false},
		{"valid H3 Max Turbo 1080P", `u("resolution") == "480P" ? tier("video_480p", u("seconds") * 0.025) : (u("resolution") == "768P" ? tier("video_768p", u("seconds") * 0.040) : tier("video_1080p", u("seconds") * 0.080))`, `{"model":"video-alias","content":[{"type":"text","text":"video"}],"resolution":"1080P","duration":5,"ratio":"16:9"}`, `{"video-alias":"minimax-h3-max-turbo"}`, videoexpr.H3MaxTurbo, 0.4, false},
		{"unknown meter field", `tier("bad", u("credits") * 1)`, `{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":5}`, `{"video-alias":"dreamina-seedance-2-0-fast-hc"}`, "", 0, true},
		{"ordinary token expression", `tier("bad", p * 37)`, `{"model":"video-alias","content":[{"type":"text","text":"video"}],"duration":5}`, `{"video-alias":"dreamina-seedance-2-0-fast-hc"}`, "", 0, true},
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
				assert.Equal(t, float64(108000), info.TieredBillingSnapshot.UsageFacts["tokens"])
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
