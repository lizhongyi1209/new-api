package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetAPIDocRouterServesEmbeddedPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	page := []byte("<!doctype html><title>API documentation marker</title>")
	SetAPIDocRouter(engine, page)

	request := httptest.NewRequest(http.MethodGet, "/docs/api-doc", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "text/html; charset=utf-8", response.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
	assert.Equal(t, string(page), response.Body.String())
}

func TestSetAPIDocRouterSupportsHead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetAPIDocRouter(engine, []byte("page body"))

	request := httptest.NewRequest(http.MethodHead, "/docs/api-doc", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "text/html; charset=utf-8", response.Header().Get("Content-Type"))
	assert.Empty(t, response.Body.String())
}
