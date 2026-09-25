package router

import (
	"testing"

	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setPluginSystemEnabled flips the master switch and restores it afterwards.
func setPluginSystemEnabled(t *testing.T, enabled bool) {
	t.Helper()
	registry := pluginruntime.DefaultRegistry
	previous := registry.Enabled()
	registry.SetEnabled(enabled)
	t.Cleanup(func() { registry.SetEnabled(previous) })
	require.Equal(t, enabled, registry.Enabled(), "the plugin master switch did not reach the requested state")
}

// protocolOwnedPaths are the routes whose only other owner is the plugin
// host-protocol router. With the plugin system off they must still be served,
// by the legacy handlers.
var protocolOwnedPaths = []string{
	"POST /v1/responses",
	"POST /v1/videos",
	"GET /v1/videos/:task_id",
	"GET /v1/videos/:task_id/content",
	"POST /v1/images/generations",
	"POST /v1/images/edits",
}

func registerAllRouters() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	SetApiRouter(engine)
	SetDashboardRouter(engine)
	SetRelayRouter(engine)
	if pluginruntime.DefaultRegistry.Enabled() {
		SetTaskPluginProtocolRouter(engine)
	}
	SetVideoRouter(engine)
	SetTaskRouter(engine)
	return engine
}

func registeredRoutes(engine *gin.Engine) map[string]int {
	routes := make(map[string]int)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path]++
	}
	return routes
}

// Disabling the plugin system must not remove the host-protocol paths: they
// fall back to the legacy handlers instead.
func TestPluginSystemDisabledKeepsLegacyProtocolPaths(t *testing.T) {
	setPluginSystemEnabled(t, false)
	routes := registeredRoutes(registerAllRouters())

	for _, path := range protocolOwnedPaths {
		assert.Equal(t, 1, routes[path], "%s must be owned exactly once while the plugin system is off", path)
	}
	assert.Equal(t, 1, routes["POST /v1/video/generations"], "the legacy MiniMax submit path is not a protocol route")
}

// While the plugin system is on, the protocol router owns those paths and the
// legacy registrations must stay out of the way.
func TestPluginSystemEnabledKeepsSingleProtocolOwner(t *testing.T) {
	setPluginSystemEnabled(t, true)
	routes := registeredRoutes(registerAllRouters())

	for _, path := range protocolOwnedPaths {
		assert.Equal(t, 1, routes[path], "%s must be owned exactly once while the plugin system is on", path)
	}
	assert.Equal(t, 1, routes["POST /v1/video/generations"], "the legacy MiniMax submit path is not a protocol route")
}
