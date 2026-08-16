package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	maxReferralLevel                   = 3
	levelOneReferralRewardRate   int64 = 10
	levelTwoReferralRewardRate   int64 = 3
	levelThreeReferralRewardRate int64 = 1
)

var ErrInvalidReferralLevel = errors.New("invalid referral level")

type ReferralLevelSummary struct {
	Level      int     `json:"level"`
	Count      int64   `json:"count"`
	TotalTopUp int64   `json:"total_top_up"`
	RewardRate int64   `json:"reward_rate"`
	Revenue    float64 `json:"revenue"`
}

type ReferralProgramSummary struct {
	ReferralCode string                 `json:"referral_code"`
	TotalCount   int64                  `json:"total_count"`
	TotalTopUp   int64                  `json:"total_top_up"`
	TotalRevenue float64                `json:"total_revenue"`
	Levels       []ReferralLevelSummary `json:"levels"`
}

type ReferralUser struct {
	Username   string  `json:"username"`
	CreatedAt  int64   `json:"created_at"`
	TotalTopUp int64   `json:"total_top_up"`
	Revenue    float64 `json:"revenue"`
}

func referralRevenue(totalTopUp int64, level int) (float64, error) {
	rewardRate, err := referralRewardRate(level)
	if err != nil {
		return 0, err
	}
	if totalTopUp <= 0 {
		return 0, nil
	}

	revenue := decimal.NewFromInt(totalTopUp).
		Mul(decimal.NewFromInt(rewardRate)).
		Div(decimal.NewFromInt(100))
	return revenue.InexactFloat64(), nil
}

func referralRewardRate(level int) (int64, error) {
	switch level {
	case 1:
		return levelOneReferralRewardRate, nil
	case 2:
		return levelTwoReferralRewardRate, nil
	case 3:
		return levelThreeReferralRewardRate, nil
	default:
		return 0, ErrInvalidReferralLevel
	}
}

// referralLevelQuery selects active users at a fixed depth below inviterId.
// Parent aliases intentionally include soft-deleted rows so an existing
// descendant does not change levels when an intermediate account is deleted.
func referralLevelQuery(db *gorm.DB, inviterId int, level int) (*gorm.DB, error) {
	query := db.Model(&User{})
	switch level {
	case 1:
		query = query.Where("users.inviter_id = ?", inviterId)
	case 2:
		query = query.
			Joins("JOIN users AS referral_parent_1 ON referral_parent_1.id = users.inviter_id").
			Where("referral_parent_1.inviter_id = ?", inviterId)
	case 3:
		query = query.
			Joins("JOIN users AS referral_parent_2 ON referral_parent_2.id = users.inviter_id").
			Joins("JOIN users AS referral_parent_1 ON referral_parent_1.id = referral_parent_2.inviter_id").
			Where("referral_parent_1.inviter_id = ?", inviterId)
	default:
		return nil, ErrInvalidReferralLevel
	}
	return query.Where("users.id <> ?", inviterId), nil
}

func GetReferralProgramSummary(inviterId int) (*ReferralProgramSummary, error) {
	summary := &ReferralProgramSummary{
		Levels: make([]ReferralLevelSummary, 0, maxReferralLevel),
	}
	if err := DB.Model(&User{}).
		Select("aff_code").
		Where("id = ?", inviterId).
		Scan(&summary.ReferralCode).Error; err != nil {
		return nil, err
	}

	for level := 1; level <= maxReferralLevel; level++ {
		query, err := referralLevelQuery(DB, inviterId, level)
		if err != nil {
			return nil, err
		}

		rewardRate, err := referralRewardRate(level)
		if err != nil {
			return nil, err
		}
		levelSummary := ReferralLevelSummary{Level: level, RewardRate: rewardRate}
		err = query.
			Joins("LEFT JOIN top_ups ON top_ups.user_id = users.id AND top_ups.status = ?", common.TopUpStatusSuccess).
			Select("COUNT(DISTINCT users.id) AS count, COALESCE(SUM(CASE WHEN top_ups.amount > 0 THEN top_ups.amount ELSE 0 END), 0) AS total_top_up").
			Scan(&levelSummary).Error
		if err != nil {
			return nil, err
		}
		levelSummary.Revenue, err = referralRevenue(levelSummary.TotalTopUp, level)
		if err != nil {
			return nil, err
		}

		summary.Levels = append(summary.Levels, levelSummary)
		summary.TotalCount += levelSummary.Count
		summary.TotalTopUp += levelSummary.TotalTopUp
		summary.TotalRevenue += levelSummary.Revenue
	}

	return summary, nil
}

func GetReferralUsers(inviterId int, level int) ([]ReferralUser, error) {
	query, err := referralLevelQuery(DB, inviterId, level)
	if err != nil {
		return nil, err
	}

	users := make([]ReferralUser, 0)
	err = query.
		Joins("LEFT JOIN top_ups ON top_ups.user_id = users.id AND top_ups.status = ?", common.TopUpStatusSuccess).
		Select("users.username, users.created_at, COALESCE(SUM(CASE WHEN top_ups.amount > 0 THEN top_ups.amount ELSE 0 END), 0) AS total_top_up").
		Group("users.id, users.username, users.created_at").
		Order("users.created_at DESC, users.id DESC").
		Scan(&users).Error
	if err != nil {
		return nil, err
	}
	for i := range users {
		users[i].Revenue, err = referralRevenue(users[i].TotalTopUp, level)
		if err != nil {
			return nil, err
		}
	}

	return users, nil
}
