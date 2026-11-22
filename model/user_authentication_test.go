package model

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type failCommittedAuthPublishHook struct {
	userID      int
	evalCount   atomic.Int64
	afterCommit atomic.Bool
}

func (hook *failCommittedAuthPublishHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if cmd.Name() != "eval" || hook.evalCount.Add(1) != 2 {
		return ctx, nil
	}
	var count int64
	if err := DB.Unscoped().Model(&User{}).Where("id = ?", hook.userID).Count(&count).Error; err == nil && count == 0 {
		hook.afterCommit.Store(true)
	}
	return ctx, errors.New("forced redis failure")
}

func (*failCommittedAuthPublishHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (*failCommittedAuthPublishHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (*failCommittedAuthPublishHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func TestHardDeleteUserPurgesAuthenticationDataWhenRedisFails(t *testing.T) {
	truncateTables(t)

	user := User{Username: "hard-delete-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&Token{UserId: user.Id, Key: "hard-delete-token"}).Error)
	require.NoError(t, DB.Create(&TwoFA{UserId: user.Id, Secret: "secret", IsEnabled: true}).Error)
	require.NoError(t, DB.Create(&TwoFABackupCode{UserId: user.Id, CodeHash: "hash"}).Error)
	require.NoError(t, DB.Create(&PasskeyCredential{UserID: user.Id, CredentialID: "credential", PublicKey: "public-key"}).Error)
	require.NoError(t, DB.Create(&UserOAuthBinding{UserId: user.Id, ProviderId: 1, ProviderUserId: "provider-user"}).Error)

	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled = true
	redisServer := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	hook := &failCommittedAuthPublishHook{userID: user.Id}
	common.RDB.AddHook(hook)
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB
	})

	require.NoError(t, HardDeleteUserById(user.Id))
	assert.True(t, hook.afterCommit.Load())

	var count int64
	require.NoError(t, DB.Unscoped().Model(&User{}).Where("id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
	for _, record := range []any{
		&Token{},
		&TwoFA{},
		&TwoFABackupCode{},
		&PasskeyCredential{},
		&UserOAuthBinding{},
	} {
		require.NoError(t, DB.Unscoped().Model(record).Where("user_id = ?", user.Id).Count(&count).Error)
		assert.Zero(t, count)
	}
}

func TestIncrementFailedAttemptsCountsConcurrentFailures(t *testing.T) {
	truncateTables(t)

	user := User{Username: "twofa-cas-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	twoFA := TwoFA{UserId: user.Id, Secret: "secret", IsEnabled: true}
	require.NoError(t, DB.Create(&twoFA).Error)

	const attempts = 4
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- (&TwoFA{Id: twoFA.Id}).IncrementFailedAttempts()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	var reloaded TwoFA
	require.NoError(t, DB.First(&reloaded, twoFA.Id).Error)
	assert.Equal(t, attempts, reloaded.FailedAttempts)
}

func TestIncrementFailedAttemptsRestartsWindowAfterLockoutExpires(t *testing.T) {
	for _, tc := range []struct {
		name            string
		lockedUntil     time.Duration
		failures        int
		wantAttempts    int
		wantLocked      bool
		keepLockedUntil bool
	}{
		{name: "first failure after expiry", lockedUntil: -time.Minute, failures: 1, wantAttempts: 1},
		{name: "max failures after expiry", lockedUntil: -time.Minute, failures: common.MaxFailAttempts, wantAttempts: common.MaxFailAttempts, wantLocked: true},
		{name: "failure during active lockout", lockedUntil: time.Minute, failures: 1, wantAttempts: common.MaxFailAttempts, wantLocked: true, keepLockedUntil: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncateTables(t)

			user := User{Username: "twofa-expiry-user", Password: "password"}
			require.NoError(t, DB.Create(&user).Error)
			lockedUntil := time.Now().Add(tc.lockedUntil)
			twoFA := TwoFA{UserId: user.Id, Secret: "secret", IsEnabled: true, FailedAttempts: common.MaxFailAttempts, LockedUntil: &lockedUntil}
			require.NoError(t, DB.Create(&twoFA).Error)

			for range tc.failures {
				require.NoError(t, (&TwoFA{Id: twoFA.Id}).IncrementFailedAttempts())
			}

			var reloaded TwoFA
			require.NoError(t, DB.First(&reloaded, twoFA.Id).Error)
			assert.Equal(t, tc.wantAttempts, reloaded.FailedAttempts)
			assert.Equal(t, tc.wantLocked, reloaded.IsLocked())
			if !tc.wantLocked {
				assert.Nil(t, reloaded.LockedUntil)
			}
			if tc.keepLockedUntil {
				require.NotNil(t, reloaded.LockedUntil)
				assert.WithinDuration(t, lockedUntil, *reloaded.LockedUntil, time.Second)
			}
		})
	}
}

func TestValidateBackupCodeCanOnlySucceedOnce(t *testing.T) {
	truncateTables(t)

	const code = "ABCD-1234"
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return replaceBackupCodesWithTx(tx, 123, []string{code})
	}))

	const attempts = 2
	results := make(chan bool, attempts)
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			valid, err := ValidateBackupCode(123, code)
			results <- valid
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	wins := 0
	for valid := range results {
		if valid {
			wins++
		}
	}
	assert.Equal(t, 1, wins)

	remaining, err := GetUnusedBackupCodeCount(123)
	require.NoError(t, err)
	assert.Zero(t, remaining)
}
