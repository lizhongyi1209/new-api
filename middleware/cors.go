package middleware

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	// IMPORTANT: When using credentials: 'include' in fetch requests,
	// CORS configuration MUST:
	// 1. Specify exact origins (cannot use wildcard '*')
	// 2. Explicitly list allowed headers (cannot use '*')
	// 3. Set AllowCredentials = true
	//
	// This is required for session-based authentication with cookies.
	// Without these settings, browsers will block requests with credentials.
	config.AllowOrigins = []string{
		"https://key.o1key.com",      // Token query tool frontend
		"http://15.204.107.201:3002", // TokenFlow frontend
		"http://localhost:3002",      // TokenFlow local dev
		"http://15.204.107.201:3000",
		"http://localhost:3000",
	}
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	// Must explicitly list headers when using credentials
	config.AllowHeaders = []string{"Origin", "Content-Type", "Content-Length", "Accept", "Authorization", "X-Requested-With"}
	config.ExposeHeaders = []string{"Content-Length", "Content-Type"}
	corsHandler := cors.New(config)
	return func(c *gin.Context) {
		// This endpoint is authenticated by a signed downstream identity and its
		// own origin allowlist. Its Origin header is part of the HMAC payload, so
		// the browser-oriented global CORS allowlist must not reject it first.
		if c.Request.URL.Path == "/v1/storage/public/presign" {
			c.Next()
			return
		}
		corsHandler(c)
	}
}

func Version() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
