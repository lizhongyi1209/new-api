package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestVideoRouterRegistersDoubaoFacade(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetVideoRouter(engine)

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	assert.True(t, routes["POST /api/v3/contents/generations/tasks"])
	assert.True(t, routes["GET /api/v3/contents/generations/tasks/:task_id"])
}
