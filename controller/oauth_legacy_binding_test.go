package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubLegacyLoginNameCannotClaimNumericIdentity(t *testing.T) {
	setupAuthFlowControllerTest(t)
	legacy := model.User{Username: "legacy-github-user", GitHubId: "renamable-login"}
	require.NoError(t, model.DB.Create(&legacy).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	user, err := findOrCreateOAuthUser(c, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: "12345",
		Extra:          map[string]any{"legacy_id": "renamable-login"},
	}, "")
	require.Error(t, err)
	assert.Nil(t, user)

	var persisted model.User
	require.NoError(t, model.DB.First(&persisted, legacy.Id).Error)
	assert.Equal(t, "renamable-login", persisted.GitHubId)
	assert.False(t, model.IsGitHubIdAlreadyTaken("12345"))
}
