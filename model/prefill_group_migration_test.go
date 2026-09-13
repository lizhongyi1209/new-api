package model

import (
	"os"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMigratePrefillGroupSchemaSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, migratePrefillGroupSchema(db))
	require.NoError(t, migratePrefillGroupSchema(db))
	assert.True(t, db.Migrator().HasTable(&PrefillGroup{}))
	assert.True(t, db.Migrator().HasIndex(&PrefillGroup{}, "uk_prefill_name"))
}

func TestMigratePrefillGroupSchemaPreservesPostgresLegacyConstraint(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not configured")
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	const testSchema = "prefill_group_migration_test"
	require.NoError(t, db.Exec(`DROP SCHEMA IF EXISTS "prefill_group_migration_test" CASCADE`).Error)
	require.NoError(t, db.Exec(`CREATE SCHEMA "prefill_group_migration_test"`).Error)
	require.NoError(t, db.Exec(`SET search_path TO "prefill_group_migration_test"`).Error)
	t.Cleanup(func() {
		_ = db.Exec(`SET search_path TO public`).Error
		_ = db.Exec(`DROP SCHEMA IF EXISTS "prefill_group_migration_test" CASCADE`).Error
	})

	require.NoError(t, db.AutoMigrate(&PrefillGroup{}))
	require.NoError(t, db.Exec(`ALTER TABLE "prefill_groups" ADD CONSTRAINT "idx_prefill_groups_name" UNIQUE ("name")`).Error)
	require.NoError(t, db.Create(&[]PrefillGroup{
		{Name: "active-probe", Type: "model", Items: JSONValue(`["model-a"]`)},
		{Name: "deleted-probe", Type: "tag", Items: JSONValue(`["tag-a"]`), DeletedAt: gorm.DeletedAt{Valid: true}},
	}).Error)

	require.NoError(t, migratePrefillGroupSchema(db))
	require.NoError(t, migratePrefillGroupSchema(db))
	assert.True(t, db.Migrator().HasConstraint(&PrefillGroup{}, "idx_prefill_groups_name"))
	assert.True(t, db.Migrator().HasIndex(&PrefillGroup{}, "uk_prefill_name"))

	var groups []PrefillGroup
	require.NoError(t, db.Unscoped().Order("id").Find(&groups).Error)
	require.Len(t, groups, 2)
	assert.Equal(t, "active-probe", groups[0].Name)
	assert.Equal(t, "deleted-probe", groups[1].Name)
	assert.Error(t, db.Create(&PrefillGroup{Name: "deleted-probe", Type: "tag", Items: JSONValue(`[]`)}).Error)
}
