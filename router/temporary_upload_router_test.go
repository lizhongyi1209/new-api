package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTemporaryAttachmentRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)
	SetStaticUploadRouter(engine)

	wantRoutes := map[string]bool{
		http.MethodPost + " /v1/o1key/uploads":    false,
		http.MethodGet + " /tmp/input/:filename":  false,
		http.MethodHead + " /tmp/input/:filename": false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wantRoutes[key]; ok {
			wantRoutes[key] = true
		}
	}
	for route, registered := range wantRoutes {
		assert.True(t, registered, route)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/o1key/uploads", nil)
	engine.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
