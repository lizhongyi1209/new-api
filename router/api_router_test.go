package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSeedanceElementRoutesDoNotExposeImageUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	assert.False(t, routes["POST /api/element/seedance/upload"])
	assert.True(t, routes["POST /api/element/seedance/"])
	assert.True(t, routes["POST /api/element/kling/upload"])
}
