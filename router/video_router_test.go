package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.True(t, routes["POST /v1/sd/assets"])
	assert.True(t, routes["GET /v1/sd/assets/:asset_id"])
	assert.True(t, routes["POST /v1/sd-5/assets"])
	assert.True(t, routes["GET /v1/sd-5/assets/:asset_id"])
	assert.True(t, routes["POST /v1/seedance/assets"])
	assert.True(t, routes["GET /v1/seedance/assets/:asset_id"])
	assert.True(t, routes["POST /v2/db-sd-max/assets"])
	assert.True(t, routes["GET /v2/db-sd-max/assets/:asset_id"])
	assert.True(t, routes["POST /v2/sd-max/assets"])
	assert.True(t, routes["GET /v2/sd-max/assets/:asset_id"])
}

func TestUnifiedKlingRoutesMatchOfficialPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetVideoRouter(engine)
	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, path := range []string{
		"/text-to-video/kling-4.0", "/text-to-video/kling-4.0-flash", "/text-to-video/kling-3.0",
		"/image-to-video/kling-4.0", "/image-to-video/kling-4.0-flash", "/image-to-video/kling-3.0",
		"/omni-video/kling-4.0", "/omni-video/kling-4.0-flash", "/omni-video/kling-3.0-omni",
	} {
		require.True(t, routes["POST "+path], path)
		require.True(t, routes["GET "+path+"/:task_id"], path)
		if path != "/omni-video/kling-3.0-omni" {
			require.True(t, routes["POST /kling"+path], path)
		}
	}
	require.True(t, routes["GET /tasks"])
	require.True(t, routes["POST /tasks"])
	require.True(t, routes["GET /kling/tasks"])
	// The prefixed 3.0 Omni route keeps its earlier task contract.
	require.True(t, routes["GET /kling/omni-video/kling-3.0-omni/:task_id"])
	require.False(t, routes["POST /motion-control/kling-3.0"])
}
