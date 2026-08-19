package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// legacyTokenAutoGroupsMigration recreates the dropped auto_group_priority
// column. The historical declaration carried an empty-string DEFAULT clause,
// which MySQL rejects on TEXT columns (Error 1101); the default is irrelevant to
// the migration, which only reads values, so it is omitted to keep the fixture
// usable on all three supported databases.
type legacyTokenAutoGroupsMigration struct {
	Id                int    `gorm:"primaryKey"`
	Key               string `gorm:"type:varchar(128);uniqueIndex"`
	AutoGroupPriority string `gorm:"type:text"`
}

func (legacyTokenAutoGroupsMigration) TableName() string {
	return "tokens"
}

func TestMigrateTokenAutoGroupsFromLegacyPriority(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyTokenAutoGroupsMigration{}))

	legacyGroups := `["vip","default"]`
	require.NoError(t, db.Create(&legacyTokenAutoGroupsMigration{
		Key:               "legacy-auto-groups",
		AutoGroupPriority: legacyGroups,
	}).Error)
	require.NoError(t, db.Create(&legacyTokenAutoGroupsMigration{
		Key: "empty-legacy-auto-groups",
	}).Error)

	originalDB := DB
	originalMainType := common.MainDatabaseType()
	t.Cleanup(func() {
		DB = originalDB
		common.SetDatabaseTypes(originalMainType, common.LogDatabaseType())
	})
	DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.LogDatabaseType())

	require.NoError(t, db.AutoMigrate(&Token{}))
	require.NoError(t, migrateTokenAutoGroupsFromLegacyPriority())

	var migrated Token
	require.NoError(t, db.Where("key = ?", "legacy-auto-groups").First(&migrated).Error)
	assert.Equal(t, legacyGroups, migrated.AutoGroups)

	var empty Token
	require.NoError(t, db.Where("key = ?", "empty-legacy-auto-groups").First(&empty).Error)
	assert.Empty(t, empty.AutoGroups)

	require.NoError(t, db.Model(&Token{}).
		Where("key = ?", "legacy-auto-groups").
		UpdateColumn("auto_groups", `["custom"]`).Error)
	require.NoError(t, migrateTokenAutoGroupsFromLegacyPriority())
	require.NoError(t, db.Where("key = ?", "legacy-auto-groups").First(&migrated).Error)
	assert.Equal(t, `["custom"]`, migrated.AutoGroups)
}
