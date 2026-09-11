package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccountPasswordRejectsCommonPasswordCorpus(t *testing.T) {
	require.Len(t, commonAccountPasswords, 3000)

	for _, password := range []string{"password", "password123", "admin1234"} {
		err := ValidateNewAccountPassword(password)
		assert.ErrorIs(t, err, ErrAccountPasswordCommon)
	}
	require.NoError(t, ValidateNewAccountPassword("correct horse battery staple 8472"))
}

func TestExistingCommonAccountPasswordStillAuthenticates(t *testing.T) {
	hash, err := Password2Hash("password123")
	require.NoError(t, err)
	assert.True(t, ValidatePasswordAndHash("password123", hash))
}
