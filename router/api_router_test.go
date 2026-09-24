package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedanceElementRoutesDoNotExposeImageUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	assert.False(t, routes["POST /api/element/seedance/upload"])
	assert.True(t, routes["POST /api/element/seedance/"])
	assert.True(t, routes["POST /api/element/kling/upload"])
}

func TestTaskLogListRoutesRequireAdminAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	for _, path := range []string{"/api/task", "/api/task/"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusUnauthorized, response.Code)
		})
	}
}
