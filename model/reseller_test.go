package model

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestBuildResellerPricingCatalogUsesUserGroupCostOverrides(t *testing.T) {
	cacheRatio := 0.5
	pricing := []Pricing{
		{
			ModelName:       "z-request-model",
			QuotaType:       1,
			ModelPrice:      4,
			EnableGroup:     []string{"vip", "default"},
			BillingMode:     "ratio",
		},
		{
			ModelName:       "a-token-model",
			ModelRatio:      1,
			CompletionRatio: 3,
			CacheRatio:      &cacheRatio,
			EnableGroup:     []string{"all"},
		},
		{
			ModelName:   "dynamic-model",
			BillingMode: "tiered_expr",
			EnableGroup: []string{"default"},
		},
	}
	catalog := BuildResellerPricingCatalog(
		pricing,
		"dealer-vip",
		"经销商专属",
		map[string]string{"default": "默认", "vip": "VIP"},
		map[string]float64{"default": 1, "vip": 1.5},
		map[string]float64{"default": 0.8, "vip": 1.2},
		map[string]bool{"default": true, "vip": true},
	)
	require.Equal(t, []string{"default", "vip"}, []string{catalog.Groups[0].Name, catalog.Groups[1].Name})
	assert.Equal(t, "dealer-vip", catalog.UserGroup)
	assert.Equal(t, "经销商专属", catalog.UserGroupDescription)
	require.NotNil(t, catalog.Groups[0].DiscountPercent)
	assert.InDelta(t, 20, *catalog.Groups[0].DiscountPercent, 0.0001)
	assert.InDelta(t, 20, *catalog.Groups[1].DiscountPercent, 0.0001)
	require.Len(t, catalog.Models, 3)
	assert.Equal(t, "a-token-model", catalog.Models[0].ModelName)
	assert.Equal(t, "dynamic-model", catalog.Models[1].ModelName)
	assert.Equal(t, "tiered_expr", catalog.Models[1].BillingMode)
	assert.Equal(t, "z-request-model", catalog.Models[2].ModelName)
	assert.Equal(t, []string{"default", "vip"}, catalog.Models[2].EnableGroups)
}

func TestResellerLedgerIsolationAndAccounting(t *testing.T) {
	cases := []struct {
		name   string
		kind   common.DatabaseType
		driver gorm.Dialector
	}{
		{"sqlite", common.DatabaseTypeSQLite, sqlite.Open(":memory:")},
	}
	if dsn := os.Getenv("RESELLER_TEST_POSTGRES_DSN"); dsn != "" {
		cases = append(cases, struct {
			name   string
			kind   common.DatabaseType
			driver gorm.Dialector
		}{"postgres", common.DatabaseTypePostgreSQL, postgres.Open(dsn)})
	}
	if dsn := os.Getenv("RESELLER_TEST_MYSQL_DSN"); dsn != "" {
		cases = append(cases, struct {
			name   string
			kind   common.DatabaseType
			driver gorm.Dialector
		}{"mysql", common.DatabaseTypeMySQL, mysql.Open(dsn)})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(tc.driver, &gorm.Config{})
			require.NoError(t, err)
			oldDB, oldLog, oldQuota := DB, LOG_DB, common.QuotaPerUnit
			oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
			DB, LOG_DB, common.QuotaPerUnit = db, db, 100
			common.SetDatabaseTypes(tc.kind, tc.kind)
			initCol()
			t.Cleanup(func() {
				DB, LOG_DB, common.QuotaPerUnit = oldDB, oldLog, oldQuota
				common.SetDatabaseTypes(oldMainType, oldLogType)
				initCol()
				sqlDB, _ := db.DB()
				_ = sqlDB.Close()
			})
			// Start with the existing application schema and data, then upgrade.
			require.NoError(t, db.AutoMigrate(&User{}, &Log{}, &TopUp{}, &Redemption{}, &SubscriptionOrder{}, &Checkin{}))
			for _, value := range []any{&Log{}, &TopUp{}, &Redemption{}, &SubscriptionOrder{}, &Checkin{}, &User{}} {
				require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(value).Error)
			}
			users := []User{
				{Id: 1, Username: "dealer", Role: 1, Quota: 9999},
				{Id: 2, Username: "direct-a", InviterId: 1, Quota: 1000, UsedQuota: 900},
				{Id: 3, Username: "direct-b", InviterId: 1, Quota: 500, UsedQuota: 300},
				{Id: 4, Username: "grandchild", InviterId: 2, Quota: 9999},
				{Id: 5, Username: "other-dealer", Quota: 9999},
				{Id: 6, Username: "deleted", InviterId: 1, Quota: 9999},
				{Id: 7, Username: "empty-dealer"},
				{Id: 9, Username: "root", Role: common.RoleRootUser},
			}
			for i := range users {
				users[i].AffCode = "reseller-" + strconv.Itoa(users[i].Id)
				users[i].CreatedAt = 10
				users[i].Status = common.UserStatusEnabled
				users[i].Group = "default"
				users[i].Password = "not-a-credential"
				require.NoError(t, db.Create(&users[i]).Error)
			}
			require.NoError(t, db.Delete(&users[5]).Error)
			require.NoError(t, db.AutoMigrate(&ResellerAccess{}))
			require.NoError(t, SetResellerAccess(context.Background(), "dealer", true, 9))
			require.NoError(t, db.AutoMigrate(&ResellerAccess{}))
			allowed, err := CheckResellerAccess(context.Background(), 1)
			require.NoError(t, err)
			assert.True(t, allowed)
			allowed, err = CheckResellerAccess(context.Background(), 5)
			require.NoError(t, err)
			assert.False(t, allowed)
			allowed, err = CheckResellerAccess(context.Background(), 9)
			require.NoError(t, err)
			assert.True(t, allowed)
			listed, err := ListResellerAccess(context.Background())
			require.NoError(t, err)
			require.Len(t, listed, 1)
			assert.Equal(t, 1, listed[0].Id)
			customers, err := GetResellerCustomers(context.Background(), 1)
			require.NoError(t, err)
			require.Len(t, customers, 2)
			assert.Equal(t, int64(1000), customers[1].Quota)
			logs := []Log{
				{UserId: 2, CreatedAt: 200, Type: 2, Quota: 120, ModelName: "visible-model", Group: "retail", Other: `{"admin_info":{"key":"hidden"},"prompt":"private","group_ratio":1.2,"provider_cost":77}`, Ip: "private-ip", Content: "private-content", TokenId: 99, ChannelId: 99},
				{UserId: 2, CreatedAt: 250, Type: 6, Quota: 20, Group: "retail"},
				{UserId: 3, CreatedAt: 210, Type: 2, Quota: 40, Group: "retail"},
				{UserId: 2, CreatedAt: 350, Type: 2, Quota: 60},
				{UserId: 3, CreatedAt: 360, Type: 2, Quota: 30, Other: `{"billing_source":"subscription"}`},
				{UserId: 3, CreatedAt: 370, Type: 6, Quota: 10, Other: `{"billing_source":"subscription"}`},
				{UserId: 2, CreatedAt: 380, Type: 1, Other: `{"op":{"action":"user.quota_add","params":{"from":0,"to":50}}}`},
				{UserId: 4, CreatedAt: 240, Type: 2, Quota: 9000, ModelName: "grandchild-model"},
				{UserId: 5, CreatedAt: 240, Type: 2, Quota: 9000, ModelName: "other-model"},
				{UserId: 6, CreatedAt: 240, Type: 2, Quota: 9000},
				{UserId: 1, CreatedAt: 240, Type: 2, Quota: 9000},
			}
			require.NoError(t, db.Create(&logs).Error)
			orders := []TopUp{
				{UserId: 2, CompleteTime: 220, Amount: 3, PaymentProvider: "epay", TradeNo: "reseller-epay", Status: "success"},
				{UserId: 3, CompleteTime: 230, Amount: 999, Money: 1.5, PaymentProvider: "stripe", TradeNo: "reseller-stripe", Status: "success"},
				{UserId: 3, CompleteTime: 240, Amount: 25, PaymentProvider: "creem", TradeNo: "reseller-creem", Status: "success"},
				{UserId: 2, CompleteTime: 240, Amount: 999, TradeNo: "reseller-failed", Status: "pending"},
				{UserId: 2, CompleteTime: 240, Amount: 999, TradeNo: "reseller-subscription", Status: "success"},
				{UserId: 4, CompleteTime: 240, Amount: 999, TradeNo: "reseller-grandchild", Status: "success"},
				{UserId: 2, CompleteTime: 390, Amount: 2, TradeNo: "reseller-later", Status: "success"},
			}
			require.NoError(t, db.Create(&orders).Error)
			require.NoError(t, db.Create(&SubscriptionOrder{TradeNo: "reseller-subscription", UserId: 2}).Error)
			require.NoError(t, db.Create(&Redemption{Key: "reseller-code", UsedUserId: 3, Status: 3, Quota: 15, RedeemedTime: 245}).Error)
			require.NoError(t, db.Create(&Checkin{UserId: 3, QuotaAwarded: 10, CreatedAt: 395, CheckinDate: "2026-11-21"}).Error)
			period, err := GetResellerPeriod(context.Background(), 1, 200, 300, 400)
			require.NoError(t, err)
			assert.Equal(t, int64(140), period.SpentQuota)
			assert.Equal(t, int64(20), period.RefundQuota)
			assert.Equal(t, int64(490), period.TopupQuota)
			assert.Equal(t, int64(1300), period.RemainingQuota)
			assert.Equal(t, int64(1120), period.UsedQuota)
			assert.True(t, period.EstimatedBalance)
			current, err := GetResellerPeriod(context.Background(), 1, 200, 400, 400)
			require.NoError(t, err)
			assert.Equal(t, int64(220), current.SpentQuota)
			assert.Equal(t, int64(30), current.RefundQuota)
			assert.Equal(t, int64(690), current.TopupQuota)
			assert.Equal(t, int64(1500), current.RemainingQuota)
			assert.Equal(t, int64(1200), current.UsedQuota)
			assert.False(t, current.EstimatedBalance)
			page, err := GetResellerLogs(context.Background(), 1, ResellerLogFilter{From: 200, To: 300, Model: "visible-model"})
			require.NoError(t, err)
			require.Len(t, page.Rows, 1)
			row := page.Rows[0]
			assert.Empty(t, row.Ip)
			assert.Empty(t, row.Content)
			assert.Zero(t, row.TokenId)
			assert.Zero(t, row.ChannelId)
			assert.JSONEq(t, `{"group_ratio":1.2}`, row.Other)
			for _, foreignID := range []int{1, 4, 5, 6} {
				page, err = GetResellerLogs(context.Background(), 1, ResellerLogFilter{From: 200, To: 400, User: foreignID})
				require.NoError(t, err)
				assert.Empty(t, page.Rows)
			}
			models, _, err := GetResellerLogOptions(context.Background(), 1)
			require.NoError(t, err)
			assert.Equal(t, []string{"visible-model"}, models)
			empty, err := GetResellerPeriod(context.Background(), 7, 200, 400, 400)
			require.NoError(t, err)
			assert.Zero(t, empty.SpentQuota)
			assert.Zero(t, empty.RemainingQuota)
			// Membership changes and revocation take effect without cache expiry.
			require.NoError(t, db.Model(&User{}).Where("id = ?", 2).Update("inviter_id", 5).Error)
			page, err = GetResellerLogs(context.Background(), 1, ResellerLogFilter{From: 200, To: 400, User: 2})
			require.NoError(t, err)
			assert.Empty(t, page.Rows)
			require.NoError(t, SetResellerAccess(context.Background(), "dealer", false, 9))
			allowed, err = CheckResellerAccess(context.Background(), 1)
			require.NoError(t, err)
			assert.False(t, allowed)
			require.NoError(t, db.AutoMigrate(&ResellerAccess{}))
			allowed, err = CheckResellerAccess(context.Background(), 1)
			require.NoError(t, err)
			assert.False(t, allowed)
		})
	}
}
