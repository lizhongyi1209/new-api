package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundImageRoutesAreRegisteredAndRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	wantRoutes := map[string]bool{
		http.MethodPost + " /pg/images/uploads":     false,
		http.MethodPost + " /pg/images/generations": false,
		http.MethodGet + " /pg/images/history":      false,
		http.MethodGet + " /pg/images/tasks/:id":    false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wantRoutes[key]; ok {
			wantRoutes[key] = true
		}
	}
	for route, registered := range wantRoutes {
		require.True(t, registered, route)
	}

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/pg/images/uploads"},
		{method: http.MethodPost, path: "/pg/images/generations"},
		{method: http.MethodGet, path: "/pg/images/history"},
		{method: http.MethodGet, path: "/pg/images/tasks/task_example"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, nil)
		engine.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code, test.path)
	}
}
