package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// SetTaskRouter exposes generic task-plugin submission. The plugin master
// switch defaults off, and PrepareTaskPluginSubmit pins an active generation.
func SetTaskRouter(router *gin.Engine) {
	tasks := router.Group("/v1/tasks")
	tasks.Use(middleware.RouteTag("relay"), middleware.SystemPerformanceCheck(), middleware.TokenAuth())
	tasks.POST("/:key", middleware.PrepareTaskPluginSubmit(), middleware.Distribute(), controller.RelayTask)
	tasks.GET("/:key", controller.GetTask)
	tasks.GET("/:key/artifacts", controller.GetTaskArtifacts)

	content := router.Group("/v1/tasks")
	content.Use(middleware.RouteTag("relay"), middleware.TokenOrTaskArtifactAccessAuth("key", "artifact_key"))
	content.GET("/:key/artifacts/:artifact_key/content", controller.TaskArtifactContent)
	content.HEAD("/:key/artifacts/:artifact_key/content", controller.TaskArtifactContent)
}
