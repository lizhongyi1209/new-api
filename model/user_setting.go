package model

import (
	"errors"
	"time"
)

// UserSetting stores user-specific settings like API keys
type UserSetting struct {
	UserId                   int    `json:"user_id" gorm:"primaryKey"`
	ServiceInferenceAPIKey   string `json:"serviceinference_api_key" gorm:"type:varchar(255)"` // encrypted
	CreatedAt                int64  `json:"created_at"`
	UpdatedAt                int64  `json:"updated_at"`
}

func (UserSetting) TableName() string {
	return "user_settings"
}

// GetUserAPIKeySetting retrieves API key settings for a user
func GetUserAPIKeySetting(userId int) (*UserSetting, error) {
	var setting UserSetting
	err := DB.Where("user_id = ?", userId).First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// UpdateUserAPIKeySetting updates or creates user API key settings
func UpdateUserAPIKeySetting(userId int, serviceInferenceKey string) error {
	if userId == 0 {
		return errors.New("user_id is required")
	}

	now := time.Now().Unix()

	// Check if settings exist
	var existing UserSetting
	err := DB.Where("user_id = ?", userId).First(&existing).Error

	if err != nil {
		// Create new
		setting := UserSetting{
			UserId:                 userId,
			ServiceInferenceAPIKey: serviceInferenceKey,
			CreatedAt:              now,
			UpdatedAt:              now,
		}
		return DB.Create(&setting).Error
	}

	// Update existing
	return DB.Model(&UserSetting{}).
		Where("user_id = ?", userId).
		Updates(map[string]interface{}{
			"serviceinference_api_key": serviceInferenceKey,
			"updated_at":               now,
		}).Error
}

// DeleteUserAPIKeySetting deletes settings for a user
func DeleteUserAPIKeySetting(userId int) error {
	return DB.Where("user_id = ?", userId).Delete(&UserSetting{}).Error
}
