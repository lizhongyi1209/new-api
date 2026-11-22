package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSQLiteDSNPreservesDefaultsForBarePaths(t *testing.T) {
	assert.Equal(t, "custom.db?"+sqliteConcurrencyParams, sqliteDSN("custom.db"))
	assert.Equal(t, "custom.db?cache=shared", sqliteDSN("custom.db?cache=shared"))
}
