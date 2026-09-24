package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTieredAudioZeroUsageHonorsRequestUnitsAndLegacyRefund(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		unit             billingexpr.BillingUnit
		quota, requests  int
	}{
		{"fixed request", `tier("request", fixed(0.025))`, billingexpr.BillingUnitRequest, 12500, 1},
		{"free request", `tier("free", fixed(0))`, billingexpr.BillingUnitRequest, 0, 1},
		{"legacy token zero usage", `tier("tokens",p*2+c*10)`, billingexpr.BillingUnitToken, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			const id, remaining = 74, 1000000
			seedUser(t, id, remaining)
			seedToken(t, id, id, "synthetic-audio-zero", remaining)
			seedChannel(t, id)
			info := makeRelayInfo(tc.expression, 1, 100, 0)
			info.UserId = id
			info.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: id}
			info.TokenId = id
			info.TokenKey = "synthetic-audio-zero"
			info.OriginModelName = "synthetic-audio"
			info.StartTime = time.Now()
			info.ForcePreConsume = true
			info.UserSetting = dto.UserSetting{BillingPreference: "wallet_only"}
			info.TieredBillingSnapshot.EstimatedBillingUnit = tc.unit
			info.PriceData.GroupRatioInfo = types.GroupRatioInfo{GroupRatio: 1}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			require.Nil(t, PreConsumeBilling(c, info.TieredBillingSnapshot.EstimatedQuotaAfterGroup, info))
			PostAudioConsumeQuota(c, info, &dto.Usage{}, "")
			assert.Equal(t, remaining-tc.quota, getUserQuota(t, id))
			assert.Equal(t, remaining-tc.quota, getTokenRemainQuota(t, id))
			assert.Equal(t, tc.quota, getTokenUsedQuota(t, id))
			used, requests := getUserUsageAccounting(t, id)
			assert.Equal(t, tc.quota, used)
			assert.Equal(t, tc.requests, requests)
			assert.Equal(t, int64(tc.quota), getChannelUsedQuota(t, id))
			var log model.Log
			require.NoError(t, model.LOG_DB.Where("user_id = ?", id).First(&log).Error)
			assert.Equal(t, tc.quota, log.Quota)
		})
	}
}
