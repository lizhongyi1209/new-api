package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestVideoRouterRegistersSpecializedVideoRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetVideoRouter(engine)

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	assert.True(t, routes["POST /api/v3/contents/generations/tasks"])
	assert.True(t, routes["GET /api/v3/contents/generations/tasks/:task_id"])
	assert.True(t, routes["POST /kling/motion-control/kling-3.0"])
	assert.True(t, routes["GET /kling/motion-control/kling-3.0/:task_id"])
	assert.True(t, routes["POST /kling/v1/videos/motion-control"])
	assert.True(t, routes["GET /kling/v1/videos/motion-control/:task_id"])
	assert.True(t, routes["POST /grok/v1/videos/generations"])
	assert.True(t, routes["POST /grok/v1/videos/edits"])
	assert.True(t, routes["POST /grok/v1/videos/extensions"])
	assert.True(t, routes["GET /grok/v1/videos/:task_id"])
}
