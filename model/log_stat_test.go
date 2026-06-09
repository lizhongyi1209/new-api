package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSumQuotaByLogTypeSumsRefundsSeparately(t *testing.T) {
	truncateTables(t)

	now := time.Now().Unix()
	logs := []Log{
		{
			UserId:    1,
			Username:  "alice",
			CreatedAt: now,
			Type:      LogTypeConsume,
			ModelName: "gpt-test",
			TokenName: "token-a",
			Quota:     1000,
			ChannelId: 7,
			Group:     "default",
		},
		{
			UserId:    1,
			Username:  "alice",
			CreatedAt: now,
			Type:      LogTypeConsume,
			ModelName: "gpt-test",
			TokenName: "token-a",
			Quota:     500,
			ChannelId: 7,
			Group:     "default",
		},
		{
			UserId:    1,
			Username:  "alice",
			CreatedAt: now,
			Type:      LogTypeRefund,
			ModelName: "gpt-test",
			TokenName: "token-a",
			Quota:     300,
			ChannelId: 7,
			Group:     "default",
		},
		{
			UserId:    1,
			Username:  "alice",
			CreatedAt: now,
			Type:      LogTypeRefund,
			ModelName: "gpt-test",
			TokenName: "token-a",
			Quota:     200,
			ChannelId: 7,
			Group:     "default",
		},
		{
			UserId:    2,
			Username:  "bob",
			CreatedAt: now,
			Type:      LogTypeRefund,
			ModelName: "gpt-test",
			TokenName: "token-a",
			Quota:     700,
			ChannelId: 7,
			Group:     "default",
		},
		{
			UserId:    1,
			Username:  "alice",
			CreatedAt: now - 100,
			Type:      LogTypeRefund,
			ModelName: "gpt-test",
			TokenName: "token-a",
			Quota:     900,
			ChannelId: 7,
			Group:     "default",
		},
	}
	require.NoError(t, DB.Create(&logs).Error)

	stat, err := SumUsedQuota(LogTypeUnknown, now-10, now+10, "gpt-test", "alice", "token-a", 7, "default")
	require.NoError(t, err)
	require.Equal(t, 1500, stat.Quota)

	refundQuota, err := SumQuotaByLogType(LogTypeRefund, now-10, now+10, "gpt-test", "alice", "token-a", 7, "default")
	require.NoError(t, err)
	require.Equal(t, 500, refundQuota)
}
