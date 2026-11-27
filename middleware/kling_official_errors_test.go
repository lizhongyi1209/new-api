package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestKlingOfficialErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		status     int
		body       any
		wantStatus int
		wantCode   float64
		wantMsg    string
	}{
		{"validation", http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "settings.resolution must be 720p"}, 400, 1201, "settings.resolution must be 720p"},
		{"missing key", http.StatusUnauthorized, gin.H{"error": gin.H{"message": "未提供令牌"}}, 401, 1000, "未提供令牌"},
		{"quota", http.StatusForbidden, gin.H{"message": "user quota is not enough"}, 402, 1101, "user quota is not enough"},
		{"rate limit", http.StatusTooManyRequests, gin.H{"message": "slow down"}, 429, 1302, "slow down"},
		{"unavailable", http.StatusServiceUnavailable, gin.H{"message": "no channel"}, 503, 5001, "no channel"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(func(c *gin.Context) { c.Set(common.RequestIdKey, "req-1") }, KlingOfficialErrors())
			engine.POST("/x", func(c *gin.Context) { c.AbortWithStatusJSON(test.status, test.body) })
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/x", nil))
			require.Equal(t, test.wantStatus, recorder.Code)
			var got map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &got))
			require.Equal(t, map[string]any{"code": test.wantCode, "message": test.wantMsg, "request_id": "req-1"}, got)
		})
	}
	engine := gin.New()
	engine.Use(KlingOfficialErrors())
	engine.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"code": 0, "message": "SUCCEED"}) })
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ok", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"code":0,"message":"SUCCEED"}`, recorder.Body.String())
}
