package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"

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

	// Legacy video routes. The plugin host-protocol router owns these paths
	// while the plugin system is enabled; with it off, these keep serving them
	// exactly as they did before the protocol routes existed.
	if !pluginruntime.DefaultRegistry.Enabled() {
		// Video proxy: accepts either session auth (dashboard) or token auth (API clients)
		videoProxyRouter := router.Group("/v1")
		videoProxyRouter.Use(middleware.RouteTag("relay"))
		videoProxyRouter.Use(middleware.TokenOrUserAuth())
		{
			videoProxyRouter.GET("/videos/:task_id/content", controller.VideoProxy)
		}
	}

	videoV1Router := router.Group("/v1")
	videoV1Router.Use(middleware.RouteTag("relay"))
	videoV1Router.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		// Legacy MiniMax/Hailuo submit path. It predates the plugin host-protocol
		// routes and has no entry in the host-protocol table, so it must stay
		// registered here.
		videoV1Router.POST("/video/generations", controller.RelayTask)
		videoV1Router.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		videoV1Router.POST("/videos/:video_id/remix", controller.RelayTask)
		if !pluginruntime.DefaultRegistry.Enabled() {
			// openai compatible API video routes
			// docs: https://platform.openai.com/docs/api-reference/videos/create
			// Keep this endpoint switchable after an off-at-boot start. Its
			// unclaimed-model branch is the same legacy RelayTask handler.
			createHandlers, err := taskPluginProtocolHandlers("openai_video", "create")
			if err != nil {
				panic(err)
			}
			router.POST("/v1/videos", createHandlers...)
			videoV1Router.GET("/videos/:task_id", controller.RelayTaskFetch)
		}
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
	// ServiceInference asset-management proxy for sub-stations. Their type-60
	// channel points here with a gateway token; the real upstream key stays on
	// this instance (see controller.ProxySeedanceAssetAPI). The /v1/sd/assets
	// routes expose the HC-model asset contract alongside the existing grouped
	// asset workflow. DF models use the parallel /v1/sd-5/assets contract.
	seedanceAssetProxyRouter := router.Group("/v1")
	seedanceAssetProxyRouter.Use(middleware.RouteTag("relay"))
	seedanceAssetProxyRouter.Use(middleware.TokenAuth())
	{
		seedanceAssetProxyRouter.POST("/asset-groups", controller.ProxySeedanceAssetAPI)
		seedanceAssetProxyRouter.GET("/asset-groups/:group_id", controller.ProxySeedanceAssetAPI)
		seedanceAssetProxyRouter.POST("/assets", controller.ProxySeedanceAssetAPI)
		seedanceAssetProxyRouter.POST("/assets/get", controller.ProxySeedanceAssetAPI)
		seedanceAssetProxyRouter.POST("/sd/assets", controller.ProxySeedanceAssetAPI)
		seedanceAssetProxyRouter.GET("/sd/assets/:asset_id", controller.ProxySeedanceAssetAPI)
		seedanceAssetProxyRouter.POST("/sd-5/assets", controller.ProxySeedanceAssetAPI)
		seedanceAssetProxyRouter.GET("/sd-5/assets/:asset_id", controller.ProxySeedanceAssetAPI)
		// Stable dispatcher for downstream New API sites. Provider-specific routes
		// above remain available and unchanged.
		seedanceAssetProxyRouter.POST("/seedance/assets", controller.CreateUnifiedSeedanceAsset)
		seedanceAssetProxyRouter.GET("/seedance/assets/:asset_id", controller.GetUnifiedSeedanceAsset)
	}

	// Doubao Seedance MAX v2 asset contract. The handler selects a type-60
	// channel that serves Doubao models, not an arbitrary type-60 channel.
	doubaoSeedanceAssetRouter := router.Group("/v2/db-sd-max")
	doubaoSeedanceAssetRouter.Use(middleware.RouteTag("relay"))
	doubaoSeedanceAssetRouter.Use(middleware.TokenAuth())
	{
		doubaoSeedanceAssetRouter.POST("/assets", controller.ProxySeedanceAssetAPI)
		doubaoSeedanceAssetRouter.GET("/assets/:asset_id", controller.ProxySeedanceAssetAPI)
	}

	// Dreamina Seedance MAX v2 asset contract. Same selection rule, bound to
	// channels that serve Dreamina models.
	dreaminaSeedanceAssetRouter := router.Group("/v2/sd-max")
	dreaminaSeedanceAssetRouter.Use(middleware.RouteTag("relay"))
	dreaminaSeedanceAssetRouter.Use(middleware.TokenAuth())
	{
		dreaminaSeedanceAssetRouter.POST("/assets", controller.ProxySeedanceAssetAPI)
		dreaminaSeedanceAssetRouter.GET("/assets/:asset_id", controller.ProxySeedanceAssetAPI)
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

	// Kling's unified task API (/{task}/{model}). The root routes match the
	// official paths exactly, so a client only changes its base URL and key;
	// the /kling routes remain for existing clients. /kling/omni-video/kling-3.0-omni
	// keeps its earlier contract above, so 3.0 Omni is unified at the root only.
	unifiedKlingRoutes := map[string][]string{
		"text-to-video":  {"kling-4.0", "kling-4.0-flash", "kling-3.0"},
		"image-to-video": {"kling-4.0", "kling-4.0-flash", "kling-3.0"},
		"omni-video":     {"kling-4.0", "kling-4.0-flash", "kling-3.0-omni"},
	}
	for _, prefix := range []string{"/kling", ""} {
		kling40Router := router.Group(prefix)
		kling40Router.Use(middleware.RouteTag("relay"))
		if prefix == "" {
			kling40Router.Use(middleware.KlingOfficialErrors())
		}
		kling40Router.Use(middleware.KlingVideo40Convert(), middleware.TokenAuth(), middleware.Distribute())
		for task, models := range unifiedKlingRoutes {
			for _, name := range models {
				if prefix != "" && name == "kling-3.0-omni" {
					continue
				}
				kling40Router.POST("/"+task+"/"+name, controller.RelayTask)
				kling40Router.GET("/"+task+"/"+name+"/:task_id", controller.RelayKling40TaskFetch)
			}
		}

		// Official batch and cursor task queries: Kling's GET/POST /tasks.
		klingTaskQueryRouter := router.Group(prefix + "/tasks")
		klingTaskQueryRouter.Use(middleware.RouteTag("relay"))
		if prefix == "" {
			klingTaskQueryRouter.Use(middleware.KlingOfficialErrors())
		}
		klingTaskQueryRouter.Use(middleware.TokenAuth())
		klingTaskQueryRouter.GET("", controller.RelayKling40TasksQuery)
		klingTaskQueryRouter.POST("", controller.RelayKling40TasksQuery)
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
		klingElementRouter.POST("/custom-voices", controller.ProxyKlingGeneralAPI)
		klingElementRouter.GET("/custom-voices", controller.ProxyKlingGeneralAPI)
		klingElementRouter.GET("/custom-voices/:id", controller.ProxyKlingGeneralAPI)
		klingElementRouter.GET("/presets-voices", controller.ProxyKlingGeneralAPI)
		klingElementRouter.POST("/delete-voices", controller.ProxyKlingGeneralAPI)
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
