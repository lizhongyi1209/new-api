package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRedeemFixture(t *testing.T, quota int) (int, string) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Redemption{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{}).Error)
		DB.Exec("DELETE FROM users")
		DB.Exec("DELETE FROM logs")
	})
	user := &User{Username: "redeem-user", Password: "password", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	key := "10000000000000000000000000000001"
	redemption := &Redemption{Name: "redeem-test", Key: key, Status: common.RedemptionCodeStatusEnabled, Quota: quota, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(redemption).Error)
	return user.Id, key
}

func TestRedeemCreditsQuotaExactlyOnce(t *testing.T) {
	userId, key := setupRedeemFixture(t, 500)
	quota, err := Redeem(key, userId)
	require.NoError(t, err)
	assert.Equal(t, 500, quota)
	_, err = Redeem(key, userId)
	require.Error(t, err)
	var user User
	require.NoError(t, DB.First(&user, "id = ?", userId).Error)
	assert.Equal(t, 500, user.Quota)
}

func TestRedeemConcurrentSingleSuccess(t *testing.T) {
	userId, key := setupRedeemFixture(t, 300)
	const goroutines = 5
	successes := make([]bool, goroutines)
	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(index int) {
			defer waitGroup.Done()
			if _, err := Redeem(key, userId); err == nil {
				successes[index] = true
			}
		}(i)
	}
	waitGroup.Wait()
	successCount := 0
	for _, succeeded := range successes {
		if succeeded {
			successCount++
		}
	}
	assert.Equal(t, 1, successCount)
	var user User
	require.NoError(t, DB.First(&user, "id = ?", userId).Error)
	assert.Equal(t, 300, user.Quota)
}
