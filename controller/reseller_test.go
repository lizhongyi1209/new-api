package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResellerAuthenticationAndRevocation(t *testing.T) {
	previousDB, previousRedis, previousSecret := model.DB, common.RedisEnabled, common.SessionSecret
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.ResellerAccess{}))
	model.DB, common.RedisEnabled, common.SessionSecret = db, false, "reseller-tests-only-secret"
	t.Cleanup(func() {
		model.DB, common.RedisEnabled, common.SessionSecret = previousDB, previousRedis, previousSecret
	})
	user := model.User{Username: "reseller-auth", Password: "unused", AffCode: "reseller-auth", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1}
	require.NoError(t, db.Create(&user).Error)
	session, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "test")
	require.NoError(t, err)
	router := gin.New()
	router.GET("/reseller", middleware.UserAuth(), ResellerAccessRequired, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.PUT("/access", middleware.UserAuth(), middleware.RootAuth(), SetResellerAccess)
	request := func(method, path, token string) int {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		router.ServeHTTP(recorder, req)
		return recorder.Code
	}
	assert.Equal(t, http.StatusUnauthorized, request("GET", "/reseller", ""))
	assert.Equal(t, http.StatusUnauthorized, request("GET", "/reseller", "invalid-token"))
	assert.Equal(t, http.StatusForbidden, request("GET", "/reseller", session.AccessToken))
	require.NoError(t, model.SetResellerAccess(context.Background(), user.Username, true, 99))
	assert.Equal(t, http.StatusNoContent, request("GET", "/reseller", session.AccessToken))
	assert.Equal(t, http.StatusForbidden, request("PUT", "/access", session.AccessToken))
	require.NoError(t, model.SetResellerAccess(context.Background(), user.Username, false, 99))
	assert.Equal(t, http.StatusForbidden, request("GET", "/reseller", session.AccessToken))
	require.NoError(t, model.SetResellerAccess(context.Background(), user.Username, true, 99))
	require.NoError(t, db.Model(&model.UserSession{}).Where("sid = ?", session.Session.SID).Update("status", model.UserSessionStatusRevoked).Error)
	assert.Equal(t, http.StatusUnauthorized, request("GET", "/reseller", session.AccessToken))
}
