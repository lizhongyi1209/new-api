package model

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSeedanceRecoveryDB(t *testing.T) {
	t.Helper()
	previous := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&Task{}, &SeedanceAssetSource{}))
	DB = db
	t.Cleanup(func() { DB = previous; require.NoError(t, sqlDB.Close()) })
}

func TestSeedanceAssetSourceOwnerExpiryAndReplacement(t *testing.T) {
	setupSeedanceRecoveryDB(t)
	ctx := context.Background()
	source := SeedanceAssetRecreateRequest{Path: "/v1/sd/assets", Body: []byte(`{"URL":"https://media.example/a.png","AssetType":"Image"}`)}
	require.NoError(t, RememberSeedanceAssetSource(ctx, 7, "asset-a", source))
	other, err := FindSeedanceAssetSources(ctx, 8, []string{"asset-a"})
	require.NoError(t, err)
	assert.Empty(t, other)

	source.Body = []byte(`{"URL":"https://media.example/b.png","AssetType":"Image"}`)
	require.NoError(t, RememberSeedanceAssetSource(ctx, 7, "asset-a", source))
	got, err := FindSeedanceAssetSources(ctx, 7, []string{"asset-a"})
	require.NoError(t, err)
	assert.JSONEq(t, string(source.Body), string(got["asset-a"].Body))
	require.NoError(t, DB.Model(&SeedanceAssetSource{}).Where("user_id = ?", 7).Update("expires_at", time.Now().Unix()-1).Error)
	got, err = FindSeedanceAssetSources(ctx, 7, []string{"asset-a"})
	require.NoError(t, err)
	assert.Empty(t, got)
	require.NoError(t, RememberSeedanceAssetSource(ctx, 7, "asset-b", source))
	var count int64
	require.NoError(t, DB.Model(&SeedanceAssetSource{}).Where("asset_id = ?", "asset-a").Count(&count).Error)
	assert.Zero(t, count)
}

func TestSeedanceRecoveryRejectsStalePollAndErasesReplayOnCompletion(t *testing.T) {
	setupSeedanceRecoveryDB(t)
	task := &Task{TaskID: "task_public", Status: TaskStatusInProgress, Quota: 100,
		PrivateData: TaskPrivateData{UpstreamTaskID: "old-upstream", SeedanceAssetRecovery: &SeedanceAssetRecovery{
			Request: []byte(`{"model":"dreamina-seedance-2-0-hc"}`), Sources: map[string]SeedanceAssetRecreateRequest{
				"asset-old": {Path: "/v1/sd/assets", Body: []byte(`{"URL":"https://private.example/source"}`)},
			},
		}},
	}
	require.NoError(t, DB.Create(task).Error)
	var stale Task
	require.NoError(t, DB.First(&stale, task.ID).Error)
	updated, err := UpdateSeedanceRecoveryTask(context.Background(), task, func(current *Task) bool {
		current.PrivateData.SeedanceAssetRecovery.Attempts++
		current.PrivateData.UpstreamTaskID = "new-upstream"
		return true
	})
	require.NoError(t, err)
	require.True(t, updated)
	stale.Status = TaskStatusFailure
	stale.FailReason = "old task failed"
	updated, err = stale.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	assert.False(t, updated)
	assert.Equal(t, "new-upstream", stale.GetUpstreamTaskID())
	assert.Equal(t, 100, stale.Quota)

	task.Status = TaskStatusSuccess
	updated, err = task.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, updated)
	var stored Task
	require.NoError(t, DB.First(&stored, task.ID).Error)
	assert.Equal(t, 1, stored.PrivateData.SeedanceAssetRecovery.Attempts)
	assert.Empty(t, stored.PrivateData.SeedanceAssetRecovery.Request)
	assert.Empty(t, stored.PrivateData.SeedanceAssetRecovery.Sources)
	public, err := common.Marshal(stored)
	require.NoError(t, err)
	assert.NotContains(t, string(public), "seedance_asset_recovery")
	assert.NotContains(t, string(public), "private.example")
}
