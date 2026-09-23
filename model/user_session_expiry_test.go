package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshRejectsSessionAtAbsoluteExpiry(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		offset int64
	}{{"before expiry", -1}, {"at expiry", 0}, {"after expiry", 1}} {
		t.Run(scenario.name, func(t *testing.T) {
			setupUserSessionTest(t)
			createUserSessionTestUser(t, 1100, 1)
			session := newTestUserSession("expiry-boundary", 1100, time.Now().Unix())
			require.NoError(t, CreateUserSession(session))
			_, err := RotateUserSessionRefresh(1100, session.SID, session.RefreshHash, "new-hash", session.ExpiresAt+scenario.offset, 30*time.Second)
			stored, readErr := GetUserSessionBySID(session.SID)
			require.NoError(t, readErr)
			if scenario.offset < 0 {
				require.NoError(t, err)
				assert.Equal(t, "new-hash", stored.RefreshHash)
			} else {
				assert.ErrorIs(t, err, ErrUserSessionInactive)
				assert.Equal(t, session.RefreshHash, stored.RefreshHash)
			}
			assert.Equal(t, session.ExpiresAt, stored.ExpiresAt, "refresh must not extend absolute lifetime")
		})
	}
}
