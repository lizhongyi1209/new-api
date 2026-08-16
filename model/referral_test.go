package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReferralProgramSummaryAndUsers(t *testing.T) {
	truncateTables(t)

	users := []*User{
		{Username: "root-referrer", Password: "password", AffCode: "root-referrer", CreatedAt: 100},
		{Username: "level-1-a", Password: "password", AffCode: "level-1-a", CreatedAt: 200},
		{Username: "level-1-b", Password: "password", AffCode: "level-1-b", CreatedAt: 210},
		{Username: "level-2", Password: "password", AffCode: "level-2", CreatedAt: 300},
		{Username: "level-3", Password: "password", AffCode: "level-3", CreatedAt: 400},
		{Username: "level-4", Password: "password", AffCode: "level-4", CreatedAt: 500},
		{Username: "unrelated", Password: "password", AffCode: "unrelated", CreatedAt: 600},
	}
	for _, user := range users {
		require.NoError(t, DB.Create(user).Error)
	}

	root := users[0]
	users[1].InviterId = root.Id
	users[2].InviterId = root.Id
	users[3].InviterId = users[1].Id
	users[4].InviterId = users[3].Id
	users[5].InviterId = users[4].Id
	for _, user := range users[1:6] {
		require.NoError(t, DB.Model(user).Update("inviter_id", user.InviterId).Error)
	}

	topUps := []*TopUp{
		{UserId: users[1].Id, Amount: 10, TradeNo: "l1-a-1", Status: common.TopUpStatusSuccess},
		{UserId: users[1].Id, Amount: 5, TradeNo: "l1-a-2", Status: common.TopUpStatusSuccess},
		{UserId: users[2].Id, Amount: 100, TradeNo: "l1-b-failed", Status: common.TopUpStatusFailed},
		{UserId: users[2].Id, Amount: -10, TradeNo: "l1-b-negative", Status: common.TopUpStatusSuccess},
		{UserId: users[3].Id, Amount: 20, TradeNo: "l2", Status: common.TopUpStatusSuccess},
		{UserId: users[4].Id, Amount: 30, TradeNo: "l3", Status: common.TopUpStatusSuccess},
		{UserId: users[5].Id, Amount: 40, TradeNo: "l4", Status: common.TopUpStatusSuccess},
	}
	for _, topUp := range topUps {
		require.NoError(t, DB.Create(topUp).Error)
	}

	summary, err := GetReferralProgramSummary(root.Id)
	require.NoError(t, err)
	require.Len(t, summary.Levels, 3)
	assert.Equal(t, int64(4), summary.TotalCount)
	assert.Equal(t, int64(65), summary.TotalTopUp)
	assert.Equal(t, ReferralLevelSummary{Level: 1, Count: 2, TotalTopUp: 15}, summary.Levels[0])
	assert.Equal(t, ReferralLevelSummary{Level: 2, Count: 1, TotalTopUp: 20}, summary.Levels[1])
	assert.Equal(t, ReferralLevelSummary{Level: 3, Count: 1, TotalTopUp: 30}, summary.Levels[2])

	levelOneUsers, err := GetReferralUsers(root.Id, 1)
	require.NoError(t, err)
	require.Len(t, levelOneUsers, 2)
	assert.Equal(t, ReferralUser{Username: "level-1-b", CreatedAt: 210, TotalTopUp: 0}, levelOneUsers[0])
	assert.Equal(t, ReferralUser{Username: "level-1-a", CreatedAt: 200, TotalTopUp: 15}, levelOneUsers[1])

	_, err = GetReferralUsers(root.Id, 4)
	assert.ErrorIs(t, err, ErrInvalidReferralLevel)
}
