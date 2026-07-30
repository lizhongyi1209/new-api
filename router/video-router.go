package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetVideoRouter(router *gin.Engine) {
	// Doubao-compatible facade for downstream New API sites. Their built-in
	// DoubaoVideo adaptor submits the official content-array request here; this
	// gateway converts it to the flat xinhankr-compatible task contract.
	doubaoVideoSubmitRouter := router.Group("/api/v3/contents/generations/tasks")
	doubaoVideoSubmitRouter.Use(middleware.RouteTag("relay"), middleware.DoubaoVideoRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	doubaoVideoSubmitRouter.POST("", controller.RelayTask)

	doubaoVideoFetchRouter := router.Group("/api/v3/contents/generations/tasks")
	doubaoVideoFetchRouter.Use(middleware.RouteTag("relay"), middleware.TokenAuth())
	doubaoVideoFetchRouter.GET("/:task_id", controller.RelayDoubaoVideoTaskFetch)

	// Video proxy: accepts either session auth (dashboard) or token auth (API clients)
	videoProxyRouter := router.Group("/v1")
	videoProxyRouter.Use(middleware.RouteTag("relay"))
	videoProxyRouter.Use(middleware.TokenOrUserAuth())
	{
		videoProxyRouter.GET("/videos/:task_id/content", controller.VideoProxy)
	}

	videoV1Router := router.Group("/v1")
	videoV1Router.Use(middleware.RouteTag("relay"))
	videoV1Router.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		videoV1Router.POST("/video/generations", controller.RelayTask)
		videoV1Router.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		videoV1Router.POST("/videos/:video_id/remix", controller.RelayTask)
	}

	// xAI Grok video API compatibility. The /grok prefix keeps xAI's response
	// contract independent from the OpenAI-compatible /v1/videos routes.
	grokVideoRouter := router.Group("/grok/v1")
	grokVideoRouter.Use(middleware.RouteTag("relay"))
	grokVideoRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		grokVideoRouter.POST("/videos/generations", controller.RelayTask)
		grokVideoRouter.POST("/videos/edits", controller.RelayTask)
		grokVideoRouter.POST("/videos/extensions", controller.RelayTask)
		grokVideoRouter.GET("/videos/:task_id", controller.RelayGrokVideoTaskFetch)
	}
	// openai compatible API video routes
	// docs: https://platform.openai.com/docs/api-reference/videos/create
	{
		videoV1Router.POST("/videos", controller.RelayTask)
		videoV1Router.GET("/videos/:task_id", controller.RelayTaskFetch)
	}

	// ServiceInference asset-management proxy for sub-stations. Their type-60
	// channel points here with a gateway token; the real upstream key stays on
	// this instance (see controller.ProxySeedanceAssetAPI).
	seedanceAssetProxyRouter := router.Group("/v1")
	seedanceAssetProxyRouter.Use(middleware.RouteTag("relay"))
	seedanceAssetProxyRouter.Use(middleware.TokenAuth())
	{
		seedanceAssetProxyRouter.POST("/asset-groups", controller.ProxySeedanceAssetAPI)
		seedanceAssetProxyRouter.GET("/asset-groups/:group_id", controller.ProxySeedanceAssetAPI)
		seedanceAssetProxyRouter.POST("/assets", controller.ProxySeedanceAssetAPI)
		seedanceAssetProxyRouter.POST("/assets/get", controller.ProxySeedanceAssetAPI)
	}

	klingV1Router := router.Group("/kling/v1")
	klingV1Router.Use(middleware.RouteTag("relay"))
	klingV1Router.Use(middleware.KlingRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingV1Router.POST("/videos/text2video", controller.RelayTask)
		klingV1Router.POST("/videos/image2video", controller.RelayTask)
		klingV1Router.GET("/videos/text2video/:task_id", controller.RelayTaskFetch)
		klingV1Router.GET("/videos/image2video/:task_id", controller.RelayTaskFetch)
	}

	klingMotionControlRouter := router.Group("/kling/v1")
	klingMotionControlRouter.Use(middleware.RouteTag("relay"))
	klingMotionControlRouter.Use(middleware.KlingMotionControlConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingMotionControlRouter.POST("/videos/motion-control", controller.RelayTask)
		klingMotionControlRouter.GET("/videos/motion-control/:task_id", controller.RelayTaskFetch)
	}

	klingOmniVideoRouter := router.Group("/kling/v1")
	klingOmniVideoRouter.Use(middleware.RouteTag("relay"))
	klingOmniVideoRouter.Use(middleware.KlingOmniVideoConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingOmniVideoRouter.POST("/videos/omni-video", controller.RelayTask)
		klingOmniVideoRouter.GET("/videos/omni-video/:task_id", controller.RelayTaskFetch)
	}

	// Kling 3.0 Omni official API. Keep the model in the path to match Kling's
	// current contract while reusing New API's task accounting and polling.
	klingOmniVideo30Router := router.Group("/kling")
	klingOmniVideo30Router.Use(middleware.RouteTag("relay"))
	klingOmniVideo30Router.Use(middleware.KlingOmniVideo30Convert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingOmniVideo30Router.POST("/omni-video/kling-3.0-omni", controller.RelayTask)
		klingOmniVideo30Router.GET("/omni-video/kling-3.0-omni/:task_id", controller.RelayKlingVideo30TaskFetch)
	}

	// Kling 3.0 Motion Control official API. This is additive; the legacy
	// /kling/v1/videos/motion-control contract remains available.
	klingMotionControl30Router := router.Group("/kling")
	klingMotionControl30Router.Use(middleware.RouteTag("relay"))
	klingMotionControl30Router.Use(middleware.KlingMotionControl30Convert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingMotionControl30Router.POST("/motion-control/kling-3.0", controller.RelayTask)
		klingMotionControl30Router.GET("/motion-control/kling-3.0/:task_id", controller.RelayKlingVideo30TaskFetch)
	}

	// Kling Official Element Management API (2024 New Version)
	klingElementRouter := router.Group("/kling/v1/general")
	klingElementRouter.Use(middleware.RouteTag("relay"))
	klingElementRouter.Use(middleware.TokenOrUserAuth())
	{
		klingElementRouter.POST("/advanced-custom-elements", controller.CreateKlingOfficialElement)
		klingElementRouter.GET("/advanced-custom-elements/:task_id", controller.QueryKlingOfficialElement)
		klingElementRouter.GET("/advanced-custom-elements", controller.ListKlingOfficialElements)
		klingElementRouter.GET("/advanced-presets-elements", controller.ListKlingOfficialPresetsElements)
		klingElementRouter.POST("/delete-advanced-elements", controller.DeleteKlingOfficialElementByBody)
		// Additional convenience routes for management
		klingElementRouter.POST("/upload", controller.UploadAigcElementImage) // Reuse existing upload handler
	}

	// Jimeng official API routes - direct mapping to official API format
	jimengOfficialGroup := router.Group("jimeng")
	jimengOfficialGroup.Use(middleware.RouteTag("relay"))
	jimengOfficialGroup.Use(middleware.JimengRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		// Maps to: /?Action=CVSync2AsyncSubmitTask&Version=2022-08-31 and /?Action=CVSync2AsyncGetResult&Version=2022-08-31
		jimengOfficialGroup.POST("/", controller.RelayTask)
	}
}
