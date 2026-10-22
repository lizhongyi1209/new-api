package model

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const SeedanceAssetRecoveryLimit = 3

const SeedanceAssetRecoveryContextKey = "seedance_asset_recovery"

// SeedanceAssetSource is private, short-lived replay data, never an API response
// or an audit snapshot. Ownership is checked before attaching it to a task.
type SeedanceAssetSource struct {
	UserID    int    `gorm:"primaryKey;autoIncrement:false" json:"-"`
	AssetID   string `gorm:"primaryKey;type:varchar(191)" json:"-"`
	Path      string `gorm:"type:varchar(64)" json:"-"`
	Body      string `gorm:"type:text" json:"-"`
	ExpiresAt int64  `gorm:"index" json:"-"`
}

type SeedanceAssetRecreateRequest struct {
	Path string          `json:"path"`
	Body json.RawMessage `json:"body"`
}

// SeedanceAssetRecovery lives only in Task.PrivateData. Media URLs and the
// replay body are erased at terminal state; the attempt count remains private.
type SeedanceAssetRecovery struct {
	Request   json.RawMessage                         `json:"request,omitempty"`
	Sources   map[string]SeedanceAssetRecreateRequest `json:"sources,omitempty"`
	Attempts  int                                     `json:"attempts,omitempty"`
	Revision  int                                     `json:"revision,omitempty"`
	StartedAt int64                                   `json:"started_at,omitempty"`
	Stopped   bool                                    `json:"stopped,omitempty"`
}

func RememberSeedanceAssetSource(ctx context.Context, userID int, assetID string, source SeedanceAssetRecreateRequest) error {
	if DB == nil || userID <= 0 || assetID == "" || len(assetID) > 191 {
		return nil
	}
	now := time.Now().Unix()
	// The expiry index also keeps cleanup bounded to expired source records.
	if err := DB.WithContext(ctx).Where("expires_at <= ?", now).Delete(&SeedanceAssetSource{}).Error; err != nil {
		return err
	}
	record := SeedanceAssetSource{
		UserID: userID, AssetID: assetID, Path: source.Path,
		Body: string(source.Body), ExpiresAt: now + 24*60*60,
	}
	return DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "asset_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"path", "body", "expires_at"}),
	}).Create(&record).Error
}

func FindSeedanceAssetSources(ctx context.Context, userID int, assetIDs []string) (map[string]SeedanceAssetRecreateRequest, error) {
	sources := make(map[string]SeedanceAssetRecreateRequest)
	if DB == nil || userID <= 0 || len(assetIDs) == 0 {
		return sources, nil
	}
	var records []SeedanceAssetSource
	err := DB.WithContext(ctx).Where("user_id = ? AND asset_id IN ? AND expires_at > ?", userID, assetIDs, time.Now().Unix()).Find(&records).Error
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		sources[record.AssetID] = SeedanceAssetRecreateRequest{Path: record.Path, Body: json.RawMessage(record.Body)}
	}
	return sources, nil
}

// UpdateSeedanceRecoveryTask serializes recovery claims and completion with
// normal polling/timeout transitions. Revision and upstream ID reject stale
// polls, including an old FAILURE received after a successful resubmission.
func UpdateSeedanceRecoveryTask(ctx context.Context, task *Task, change func(*Task) bool) (bool, error) {
	expectedID := task.GetUpstreamTaskID()
	expectedRevision := task.PrivateData.SeedanceAssetRecovery.Revision
	updated := false
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current Task
		if err := lockForUpdate(tx).First(&current, task.ID).Error; err != nil {
			return err
		}
		recovery := current.PrivateData.SeedanceAssetRecovery
		if recovery == nil || recovery.Revision != expectedRevision || current.GetUpstreamTaskID() != expectedID ||
			current.Status == TaskStatusFailure || current.Status == TaskStatusSuccess {
			*task = current
			return nil
		}
		if !change(&current) {
			*task = current
			return nil
		}
		current.PrivateData.SeedanceAssetRecovery.Revision++
		result := tx.Model(&current).Select("*").Updates(&current)
		if result.Error != nil {
			return result.Error
		}
		updated = result.RowsAffected > 0
		*task = current
		return nil
	})
	return updated, err
}

func (t *Task) updateWithSeedanceRecoveryStatus(fromStatus TaskStatus) (bool, error) {
	desired := *t
	if desired.Status == TaskStatusSuccess || desired.Status == TaskStatusFailure {
		desired.PrivateData.SeedanceAssetRecovery.Request = nil
		desired.PrivateData.SeedanceAssetRecovery.Sources = nil
		desired.PrivateData.SeedanceAssetRecovery.StartedAt = 0
	}
	return UpdateSeedanceRecoveryTask(context.Background(), t, func(current *Task) bool {
		if current.Status != fromStatus {
			return false
		}
		*current = desired
		return true
	})
}
