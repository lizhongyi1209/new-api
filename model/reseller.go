package model

import (
	"context"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ResellerAccess is independent of the system administrator role. Granting it
// never grants user administration, API keys, or access to indirect referrals.
type ResellerAccess struct {
	UserId    int   `json:"user_id" gorm:"primaryKey;autoIncrement:false"`
	Enabled   bool  `json:"enabled"`
	UpdatedAt int64 `json:"updated_at"`
	UpdatedBy int   `json:"-"`
}

type ResellerCustomer struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	Quota       int64  `json:"quota"`
	UsedQuota   int64  `json:"used_quota"`
	CreatedAt   int64  `json:"created_at"`
	LastLoginAt int64  `json:"last_login_at"`
	Group       string `json:"user_group"`
}

type ResellerPricingGroup struct {
	UserGroup    string `json:"user_group"`
	BillingGroup string `json:"billing_group"`
	NetQuota     int64  `json:"net_quota"`
}

type ResellerPeriod struct {
	SpentQuota       int64                  `json:"spent_quota"`
	TopupQuota       int64                  `json:"topup_quota"`
	RefundQuota      int64                  `json:"refund_quota"`
	RemainingQuota   int64                  `json:"remaining_quota"`
	UsedQuota        int64                  `json:"used_quota"`
	EstimatedBalance bool                   `json:"estimated_balance"`
	AsOf             int64                  `json:"as_of"`
	PricingGroups    []ResellerPricingGroup `json:"pricing_groups"`
}

func CheckResellerAccess(ctx context.Context, userID int) (bool, error) {
	var user User
	if err := DB.WithContext(ctx).Select("id", "role", "status").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if user.Status != common.UserStatusEnabled {
		return false, nil
	}
	if user.Role == common.RoleRootUser {
		return true, nil
	}
	var count int64
	err := DB.WithContext(ctx).Model(&ResellerAccess{}).Where("user_id = ? AND enabled = ?", userID, true).Count(&count).Error
	return count > 0, err
}

func SetResellerAccess(ctx context.Context, username string, enabled bool, actor int) error {
	var user User
	if err := DB.WithContext(ctx).Select("id", "role").Where("username = ?", username).First(&user).Error; err != nil {
		return err
	}
	if user.Role == common.RoleRootUser {
		return errors.New("root access is implicit")
	}
	access := ResellerAccess{UserId: user.Id, Enabled: enabled, UpdatedAt: time.Now().Unix(), UpdatedBy: actor}
	return DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"enabled", "updated_at", "updated_by"})}).Create(&access).Error
}

func ListResellerAccess(ctx context.Context) ([]ResellerCustomer, error) {
	rows := make([]ResellerCustomer, 0)
	err := DB.WithContext(ctx).Model(&User{}).Select("users.id, users.username").
		Joins("JOIN reseller_accesses ON reseller_accesses.user_id = users.id AND reseller_accesses.enabled = ?", true).
		Order("users.id DESC").Scan(&rows).Error
	return rows, err
}

func GetResellerCustomers(ctx context.Context, owner int) ([]ResellerCustomer, error) {
	rows := make([]ResellerCustomer, 0)
	err := DB.WithContext(ctx).Model(&User{}).
		Select("id", "username", "quota", "used_quota", "created_at", "last_login_at", "group").
		Where("inviter_id = ? AND id <> ?", owner, owner).Order("id DESC").Scan(&rows).Error
	return rows, err
}

// Resolve membership on every request. Do not accept a reseller/owner ID from
// the browser, cache membership, or join users against a separate log database.
func resellerLogs(ctx context.Context, owner int) (*gorm.DB, error) {
	var ids []int
	if err := DB.WithContext(ctx).Model(&User{}).Where("inviter_id = ? AND id <> ?", owner, owner).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	query := LOG_DB.WithContext(ctx).Model(&Log{})
	if len(ids) == 0 {
		return query.Where("1 = 0"), nil
	}
	return query.Where("user_id IN ?", ids), nil
}

func GetResellerLogOptions(ctx context.Context, owner int) (models []string, groups []string, err error) {
	models, groups = []string{}, []string{}
	query, err := resellerLogs(ctx, owner)
	if err != nil {
		return models, groups, err
	}
	if err = query.Where("type IN ? AND model_name <> ?", []int{2, 5, 6}, "").Distinct("model_name").Order("model_name").Pluck("model_name", &models).Error; err != nil {
		return models, groups, err
	}
	query, err = resellerLogs(ctx, owner)
	if err != nil {
		return models, groups, err
	}
	err = query.Where("type IN ?", []int{2, 5, 6}).Distinct(logGroupCol).Pluck(logGroupCol, &groups).Error
	return models, groups, err
}

type ResellerLogFilter struct {
	From, To                     int64
	User, Type, Page             int
	Model, Group, Token, Request string
}

type ResellerLogPage struct {
	Rows  []*Log `json:"rows"`
	Total int64  `json:"total"`
	Page  int    `json:"page"`
}

func GetResellerLogs(ctx context.Context, owner int, filter ResellerLogFilter) (*ResellerLogPage, error) {
	query, err := resellerLogs(ctx, owner)
	if err != nil {
		return nil, err
	}
	query = query.Where("created_at >= ? AND created_at <= ? AND type IN ?", filter.From, filter.To, []int{2, 5, 6})
	if filter.User > 0 {
		query = query.Where("user_id = ?", filter.User)
	}
	if filter.Type > 0 {
		query = query.Where("type = ?", filter.Type)
	}
	for _, pair := range [][2]string{{"model_name", filter.Model}, {logGroupCol, filter.Group}, {"token_name", filter.Token}, {"request_id", filter.Request}} {
		if pair[1] != "" {
			query = query.Where(pair[0]+" = ?", pair[1])
		}
	}
	result := &ResellerLogPage{Rows: []*Log{}, Page: max(1, filter.Page)}
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, err
	}
	result.Page = min(result.Page, max(1, int((result.Total+9)/10)))
	// The narrow projection excludes IPs, prompts, channel routing and tokens.
	err = query.Select("id", "user_id", "created_at", "type", "username", "token_name", "model_name", "quota", "prompt_tokens", "completion_tokens", "use_time", "is_stream", "group", "request_id", "other").
		Order("created_at DESC, id DESC").Limit(10).Offset((result.Page - 1) * 10).Find(&result.Rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range result.Rows {
		var raw map[string]any
		_ = common.UnmarshalJsonStr(row.Other, &raw)
		safe := make(map[string]any)
		for _, key := range []string{"group_ratio", "user_group_ratio", "frt", "is_task", "is_async_task", "billing_source", "billing_mode", "model_price", "model_ratio", "completion_ratio", "per_call_billing", "generated_image_count", "input_image_count", "total_tokens", "cache_tokens", "actual_quota", "billing_unit", "duration", "seconds", "image_count", "task_status"} {
			if value, ok := raw[key]; ok {
				safe[key] = value
			}
		}
		row.Other = common.MapToJsonStr(safe)
	}
	return result, nil
}

// Monetary totals use int64/decimal rather than the single-charge int32 clamp.
// These are read-only reports and never modify charging or settlement.
func resellerTopups(ctx context.Context, ids []int, from, to int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var orders []TopUp
	err := DB.WithContext(ctx).Model(&TopUp{}).
		Select("amount", "money", "payment_method", "payment_provider").
		Where("user_id IN ? AND status = ? AND complete_time >= ? AND complete_time <= ?", ids, common.TopUpStatusSuccess, from, to).
		Where("NOT EXISTS (SELECT 1 FROM subscription_orders WHERE subscription_orders.trade_no = top_ups.trade_no)").Find(&orders).Error
	if err != nil {
		return 0, err
	}
	var total int64
	for _, order := range orders {
		provider := order.PaymentProvider
		if provider == "" {
			provider = order.PaymentMethod
		}
		quota := decimal.NewFromInt(order.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit))
		if provider == PaymentProviderStripe {
			quota = decimal.NewFromFloat(order.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit))
		}
		if provider == PaymentProviderCreem {
			quota = decimal.NewFromInt(order.Amount)
		}
		total += quota.Round(0).IntPart()
	}
	var redeemed int64
	// Deleted redemption codes remain successful wallet credits.
	err = DB.WithContext(ctx).Unscoped().Model(&Redemption{}).Where("used_user_id IN ? AND status = ? AND redeemed_time >= ? AND redeemed_time <= ?", ids, common.RedemptionCodeStatusUsed, from, to).Select("COALESCE(SUM(quota),0)").Scan(&redeemed).Error
	return total + redeemed, err
}

func GetResellerPeriod(ctx context.Context, owner int, from, to, now int64) (*ResellerPeriod, error) {
	to = min(to, now)
	customers, err := GetResellerCustomers(ctx, owner)
	if err != nil {
		return nil, err
	}
	result := &ResellerPeriod{AsOf: now, EstimatedBalance: to < now, PricingGroups: []ResellerPricingGroup{}}
	ids, historicalIDs := []int{}, []int{}
	byUser := map[int]ResellerCustomer{}
	for _, customer := range customers {
		ids = append(ids, customer.Id)
		byUser[customer.Id] = customer
		if customer.CreatedAt <= to {
			historicalIDs = append(historicalIDs, customer.Id)
			result.RemainingQuota += customer.Quota
			result.UsedQuota += customer.UsedQuota
		}
	}
	if len(ids) == 0 {
		return result, nil
	}
	type groupTotal struct {
		UserId      int
		Group       string
		Net, Refund int64
	}
	var totals []groupTotal
	err = LOG_DB.WithContext(ctx).Model(&Log{}).Where("user_id IN ? AND created_at >= ? AND created_at <= ? AND type IN ?", ids, from, to, []int{2, 6}).
		Select("user_id, " + logGroupCol + ", SUM(CASE WHEN type = 2 THEN quota ELSE -quota END) AS net, SUM(CASE WHEN type = 6 THEN quota ELSE 0 END) AS refund").Group("user_id, " + logGroupCol).Scan(&totals).Error
	if err != nil {
		return nil, err
	}
	groups := map[[2]string]int64{}
	for _, row := range totals {
		result.SpentQuota += row.Net
		result.RefundQuota += row.Refund
		groups[[2]string{byUser[row.UserId].Group, row.Group}] += row.Net
	}
	for key, net := range groups {
		result.PricingGroups = append(result.PricingGroups, ResellerPricingGroup{UserGroup: key[0], BillingGroup: key[1], NetQuota: net})
	}
	result.TopupQuota, err = resellerTopups(ctx, ids, from, to)
	if err != nil {
		return nil, err
	}
	if !result.EstimatedBalance || len(historicalIDs) == 0 {
		return result, nil
	}
	// Historical closing balances are estimates reconstructed from recorded
	// movements; legacy adjustments may be incomplete. The API labels them so.
	rows, err := LOG_DB.WithContext(ctx).Model(&Log{}).Select("type", "quota", "other").
		Where("user_id IN ? AND created_at > ? AND created_at <= ? AND type IN ?", historicalIDs, to, now, []int{1, 2, 6}).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row Log
		if err := LOG_DB.ScanRows(rows, &row); err != nil {
			return nil, err
		}
		var other struct {
			BillingSource string `json:"billing_source"`
			Op            struct {
				Action string                   `json:"action"`
				Params struct{ From, To int64 } `json:"params"`
			} `json:"op"`
		}
		_ = common.UnmarshalJsonStr(row.Other, &other)
		if row.Type == 1 {
			switch other.Op.Action {
			case "user.quota_add", "user.quota_subtract", "user.quota_override":
				result.RemainingQuota -= other.Op.Params.To - other.Op.Params.From
			}
			continue
		}
		delta := int64(row.Quota)
		if row.Type == 6 {
			delta = -delta
		}
		result.UsedQuota -= delta
		if other.BillingSource != "subscription" {
			result.RemainingQuota += delta
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	credits, err := resellerTopups(ctx, historicalIDs, to+1, now)
	if err != nil {
		return nil, err
	}
	var bonus int64
	err = DB.WithContext(ctx).Model(&Checkin{}).Where("user_id IN ? AND created_at > ? AND created_at <= ?", historicalIDs, to, now).Select("COALESCE(SUM(quota_awarded),0)").Scan(&bonus).Error
	result.RemainingQuota -= credits + bonus
	return result, err
}
