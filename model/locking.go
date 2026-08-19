package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockForUpdate makes the next query emit SELECT ... FOR UPDATE so the
// matched rows stay locked until the surrounding transaction ends.
//
// GORM v2 silently ignores the legacy Set("gorm:query_option", "FOR UPDATE")
// form. SQLite has no FOR UPDATE syntax, so its single-writer transaction
// behavior is used instead.
func lockForUpdate(tx *gorm.DB) *gorm.DB {
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}
