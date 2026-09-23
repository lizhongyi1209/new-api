package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

// PrepareTaskPluginSubmit pins one active plugin generation before channel
// selection. The declared plugin key and model are both checked server-side;
// a token bound to a different channel cannot cross this boundary.
func PrepareTaskPluginSubmit() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		if !jsplugin.ValidPluginKey(key) || !jsplugin.DefaultRegistry.Enabled() {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "task plugin not found", "type": "invalid_request_error"}})
			return
		}
		generation := jsplugin.DefaultRegistry.Generation()
		if generation == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "task plugin unavailable", "type": "server_error"}})
			return
		}
		plugin, found := generation.Get(key)
		if !found || plugin == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "task plugin not found", "type": "invalid_request_error"}})
			return
		}
		if c.ContentType() != gin.MIMEJSON {
			c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{"error": gin.H{"message": "a JSON request body is required", "type": "invalid_request_error"}})
			return
		}
		var body map[string]any
		if err := common.UnmarshalBodyReusable(c, &body); err != nil || body == nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid JSON request body", "type": "invalid_request_error"}})
			return
		}
		modelName, ok := body["model"].(string)
		if !ok || strings.TrimSpace(modelName) != modelName || !slices.Contains(plugin.Meta.Models, modelName) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "model is not served by this task plugin", "type": "invalid_request_error"}})
			return
		}
		c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: generation, Plugin: plugin})
		c.Set("task_request", body)
		c.Set("resolved_task_model", modelName)
		c.Set("expected_task_plugin_key", key)
		c.Set("task_plugin_key", key)
		c.Set("platform", key)
		c.Set("relay_mode", relayconstant.RelayModeVideoSubmit)
		c.Next()
	}
}
