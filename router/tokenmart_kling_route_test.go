package router

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenMartKlingOfficialPathsLeaveBuiltinRoutesAlone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	outer := gin.New()
	outer.POST("/kling/text-to-video/kling-4.0", func(c *gin.Context) { c.String(http.StatusOK, "kling-builtin") })
	outer.POST("/kling/v1/videos/text2video", func(c *gin.Context) { c.String(http.StatusOK, "kling-legacy") })
	outer.GET("/kling/tasks", func(c *gin.Context) { c.String(http.StatusOK, "kling-tasks") })
	outer.POST("/kling/v1/general/advanced-custom-elements", func(c *gin.Context) { c.String(http.StatusOK, "kling-element") })
	outer.POST("/v1/videos", func(c *gin.Context) { c.String(http.StatusOK, "openai-videos") })
	outer.GET("/v1/videos/:task_id", func(c *gin.Context) { c.String(http.StatusOK, "openai-video-task") })
	outer.POST("/v1/images/generations", func(c *gin.Context) { c.String(http.StatusOK, "openai-images") })
	outer.GET("/v1/tasks/:key", func(c *gin.Context) { c.String(http.StatusOK, "gateway-tasks") })

	source, err := builtinplugins.Source("tokenmart-kling")
	require.NoError(t, err)
	plugin, err := jsplugin.CompilePlugin(source, jsplugin.Options{Key: "tokenmart-kling"})
	require.NoError(t, err)
	assert.Empty(t, plugin.Meta.ChannelTypes)

	registry := jsplugin.NewRegistry()
	require.NoError(t, registry.ReplaceOverrides([]*jsplugin.LoadedPlugin{plugin}))
	handlers := testPluginRouteHandlers(func(c *gin.Context, _ *jsplugin.RoutingGeneration, binding jsplugin.RouteBinding) {
		c.String(http.StatusOK, binding.Plugin.Meta.Key)
	})
	builder := newPluginGenerationBuilder(outer.Routes(), nil, handlers)
	require.NoError(t, registry.SetGenerationPreparer(builder.prepare))
	require.Empty(t, registry.RoutingErrors())
	_, found := registry.Generation().Get("tokenmart-kling")
	require.True(t, found)

	outer.NoRoute(
		(&pluginRouteDispatcher{registry: registry}).dispatch,
		func(c *gin.Context) { c.String(http.StatusNotFound, "fallback") },
	)

	cases := []struct {
		method string
		path   string
		status int
		want   string
	}{
		{http.MethodPost, "/text-to-video/kling-4.0-cn", http.StatusOK, "tokenmart-kling"},
		{http.MethodPost, "/image-to-video/kling-4.0-cn", http.StatusOK, "tokenmart-kling"},
		{http.MethodPost, "/omni-video/kling-4.0-cn", http.StatusOK, "tokenmart-kling"},
		{http.MethodPost, "/motion-control/kling-3.0-cn", http.StatusOK, "tokenmart-kling"},
		{http.MethodPost, "/image-generation/kling-4.0-cn", http.StatusOK, "tokenmart-kling"},
		{http.MethodPost, "/long-image-generation/kling-4.0-cn", http.StatusOK, "tokenmart-kling"},
		{http.MethodGet, "/tasks?task_ids=950079555921253320", http.StatusOK, "tokenmart-kling"},
		{http.MethodPost, "/tasks", http.StatusOK, "tokenmart-kling"},
		{http.MethodPost, "/v1/general/advanced-custom-elements", http.StatusOK, "tokenmart-kling"},
		{http.MethodGet, "/v1/general/presets-voices", http.StatusOK, "tokenmart-kling"},
		{http.MethodPost, "/kling/text-to-video/kling-4.0", http.StatusOK, "kling-builtin"},
		{http.MethodPost, "/kling/v1/videos/text2video", http.StatusOK, "kling-legacy"},
		{http.MethodGet, "/kling/tasks", http.StatusOK, "kling-tasks"},
		{http.MethodPost, "/kling/v1/general/advanced-custom-elements", http.StatusOK, "kling-element"},
		{http.MethodPost, "/v1/videos", http.StatusOK, "openai-videos"},
		{http.MethodGet, "/v1/videos/task_123", http.StatusOK, "openai-video-task"},
		{http.MethodGet, "/v1/videos/text2video", http.StatusOK, "openai-video-task"},
		{http.MethodPost, "/v1/videos/text2video", http.StatusNotFound, "fallback"},
		{http.MethodPost, "/v1/images/generations", http.StatusOK, "openai-images"},
		{http.MethodGet, "/v1/tasks/task_123", http.StatusOK, "gateway-tasks"},
	}
	for _, testCase := range cases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			response := performPluginRequest(outer, testCase.method, testCase.path)
			assert.Equal(t, testCase.status, response.Code)
			assert.Equal(t, testCase.want, response.Body.String())
		})
	}
}
