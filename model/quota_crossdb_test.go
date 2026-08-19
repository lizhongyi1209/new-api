package model

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestQuotaAndSettlementCrossDatabase runs only when the isolated test stack
// supplies both DSNs. It protects SQL dialect behavior that SQLite cannot
// exercise, including row locks and concurrent payment callbacks.
func TestQuotaAndSettlementCrossDatabase(t *testing.T) {
	postgresDSN := os.Getenv("UPSTREAM_TEST_POSTGRES_DSN")
	mysqlDSN := os.Getenv("UPSTREAM_TEST_MYSQL_DSN")
	if postgresDSN == "" || mysqlDSN == "" {
		t.Skip("isolated PostgreSQL and MySQL DSNs are not configured")
	}

	originalDB := DB
	originalLogDB := LOG_DB
	originalMainType := common.MainDatabaseType()
	originalLogType := common.LogDatabaseType()
	originalRedisEnabled := common.RedisEnabled
	originalBatchEnabled := common.BatchUpdateEnabled
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
		common.RedisEnabled = originalRedisEnabled
		common.BatchUpdateEnabled = originalBatchEnabled
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	testCases := []struct {
		name         string
		databaseType common.DatabaseType
		open         func() (*gorm.DB, error)
	}{
		{"postgres", common.DatabaseTypePostgreSQL, func() (*gorm.DB, error) {
			return gorm.Open(postgres.Open(postgresDSN), &gorm.Config{})
		}},
		{"mysql", common.DatabaseTypeMySQL, func() (*gorm.DB, error) {
			return gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{})
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db, err := testCase.open()
			require.NoError(t, err)
			if testCase.databaseType == common.DatabaseTypePostgreSQL {
				require.NoError(t, db.Exec("CREATE SCHEMA IF NOT EXISTS crossdb").Error)
			}
			DB = db
			LOG_DB = db
			common.SetDatabaseTypes(testCase.databaseType, testCase.databaseType)
			common.RedisEnabled = false
			common.BatchUpdateEnabled = false
			common.QuotaPerUnit = 10
			initCol()

			require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &TopUp{}, &Redemption{}, &Log{}))
			if !db.Migrator().HasColumn(&legacyTokenAutoGroupsMigration{}, "AutoGroupPriority") {
				require.NoError(t, db.Migrator().AddColumn(&legacyTokenAutoGroupsMigration{}, "AutoGroupPriority"))
			}
			columnTypes, err := db.Migrator().ColumnTypes(&Token{})
			require.NoError(t, err)
			foundAutoGroups := false
			for _, columnType := range columnTypes {
				if !strings.EqualFold(columnType.Name(), "auto_groups") {
					continue
				}
				foundAutoGroups = true
				assert.Contains(t, strings.ToUpper(columnType.DatabaseTypeName()), "TEXT")
			}
			assert.True(t, foundAutoGroups)
			for _, value := range []interface{}{&Redemption{}, &TopUp{}, &Token{}, &Log{}, &User{}} {
				require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(value).Error)
			}

			user := User{Username: "crossdb-" + testCase.name, Password: "unused", Status: common.UserStatusEnabled, Group: "default", Quota: 100}
			require.NoError(t, db.Create(&user).Error)
			legacyToken := Token{UserId: user.Id, Key: "crossdb-legacy-token-" + testCase.name, Name: "legacy", Status: common.TokenStatusEnabled, ExpiredTime: -1, Group: "auto"}
			require.NoError(t, legacyToken.Insert())
			legacyGroups := `["vip","default"]`
			require.NoError(t, db.Model(&Token{}).Where("id = ?", legacyToken.Id).UpdateColumn("auto_group_priority", legacyGroups).Error)
			require.NoError(t, migrateTokenAutoGroupsFromLegacyPriority())
			require.NoError(t, db.First(&legacyToken, legacyToken.Id).Error)
			assert.Equal(t, legacyGroups, legacyToken.AutoGroups)

			reserved, err := TryReserveUserQuota(user.Id, 60)
			require.NoError(t, err)
			assert.True(t, reserved)
			reserved, err = TryReserveUserQuota(user.Id, 41)
			require.NoError(t, err)
			assert.False(t, reserved)

			token := Token{UserId: user.Id, Key: "crossdb-token-" + testCase.name, Name: "crossdb", Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 80, Group: "auto", AutoGroups: `["vip","default"]`}
			require.NoError(t, token.Insert())
			reserved, err = TryReserveTokenQuota(token.Id, token.Key, 25, false)
			require.NoError(t, err)
			assert.True(t, reserved)
			reserved, err = TryReserveTokenQuota(token.Id, token.Key, 56, false)
			require.NoError(t, err)
			assert.False(t, reserved)

			order := TopUp{UserId: user.Id, Amount: 2, Money: 2, TradeNo: "CROSSDB-" + testCase.name, PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay, CreateTime: common.GetTimestamp(), Status: common.TopUpStatusPending}
			require.NoError(t, order.Insert())
			const callbacks = 5
			results := make(chan bool, callbacks)
			errorsSeen := make(chan error, callbacks)
			var waitGroup sync.WaitGroup
			waitGroup.Add(callbacks)
			for i := 0; i < callbacks; i++ {
				go func() {
					defer waitGroup.Done()
					alreadyDone, callbackErr := RechargeEpay(order.TradeNo, "alipay", "127.0.0.1")
					results <- alreadyDone
					errorsSeen <- callbackErr
				}()
			}
			waitGroup.Wait()
			close(results)
			close(errorsSeen)
			for callbackErr := range errorsSeen {
				require.NoError(t, callbackErr)
			}
			completed := 0
			for alreadyDone := range results {
				if !alreadyDone {
					completed++
				}
			}
			assert.Equal(t, 1, completed, fmt.Sprintf("%s must credit exactly once", testCase.name))

			var reloadedUser User
			require.NoError(t, db.First(&reloadedUser, user.Id).Error)
			assert.Equal(t, 60, reloadedUser.Quota)
			var reloadedToken Token
			require.NoError(t, db.First(&reloadedToken, token.Id).Error)
			assert.Equal(t, 55, reloadedToken.RemainQuota)
			assert.Equal(t, 25, reloadedToken.UsedQuota)
			assert.JSONEq(t, `["vip","default"]`, reloadedToken.AutoGroups)
		})
	}
}
