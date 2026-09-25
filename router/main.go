package router

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"

	"github.com/gin-gonic/gin"
)

func SetRouter(router *gin.Engine, assets WebAssets) {
	SetApiRouter(router)
	SetDashboardRouter(router)
	SetRelayRouter(router)
	// The host-protocol routes are the plugin system's own entry points. With
	// the plugin system off, the legacy handlers registered by SetRelayRouter
	// and SetVideoRouter own these paths instead, and registering both would
	// collide on the same gin route.
	if pluginruntime.DefaultRegistry.Enabled() {
		SetTaskPluginProtocolRouter(router)
	}
	SetVideoRouter(router)
	SetTaskRouter(router)
	SetStaticUploadRouter(router) // Serve uploaded files
	pluginDispatcher := SetPluginRouter(router)
	frontendBaseUrl := os.Getenv("FRONTEND_BASE_URL")
	if common.IsMasterNode && frontendBaseUrl != "" {
		frontendBaseUrl = ""
		common.SysLog("FRONTEND_BASE_URL is ignored on master node")
	}
	if frontendBaseUrl == "" {
		SetWebRouter(router, assets, pluginDispatcher)
	} else {
		frontendBaseUrl = strings.TrimSuffix(frontendBaseUrl, "/")
		router.NoRoute(
			pluginDispatcher,
			middleware.RouteTag("web"),
			middleware.AccessTokenAudit(),
			func(c *gin.Context) {
				c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseUrl, c.Request.RequestURI))
			},
		)
	}
}

// SetStaticUploadRouter serves uploaded files from local storage
func SetStaticUploadRouter(router *gin.Engine) {
	uploadDir := os.Getenv("LOCAL_UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "uploads"
	}

	router.Static("/upload", uploadDir)
	router.GET("/tmp/input/:filename", controller.ServeTemporaryInputAttachment)
	router.HEAD("/tmp/input/:filename", controller.ServeTemporaryInputAttachment)
	router.GET("/tmp/output/:filename", controller.ServeTemporaryOutputImage)
	router.HEAD("/tmp/output/:filename", controller.ServeTemporaryOutputImage)
}
