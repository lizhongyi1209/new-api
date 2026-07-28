package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKlingOmniVideo30ConvertPreservesOfficialRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(KlingOmniVideo30Convert())
	router.POST("/kling/omni-video/kling-3.0-omni", func(c *gin.Context) {
		var request relaycommon.TaskSubmitReq
		require.NoError(t, common.UnmarshalBodyReusable(c, &request))
		assert.Equal(t, "kling-3.0-omni", request.Model)
		assert.Equal(t, "A square-format product video", request.Prompt)

		settings, ok := request.Metadata["settings"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "1:1", settings["aspect_ratio"])
		assert.Equal(t, false, settings["multi_shot"])
		c.Status(http.StatusNoContent)
	})

	body := `{
		"contents":[{"type":"prompt","text":"A square-format product video"}],
		"settings":{"aspect_ratio":"1:1","multi_shot":false,"duration":5}
	}`
	request := httptest.NewRequest(http.MethodPost, "/kling/omni-video/kling-3.0-omni", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
}

func TestKlingOmniVideo30ConvertInjectsModelForTaskQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(KlingOmniVideo30Convert())
	router.GET("/kling/omni-video/kling-3.0-omni/:task_id", func(c *gin.Context) {
		var request relaycommon.TaskSubmitReq
		require.NoError(t, common.UnmarshalBodyReusable(c, &request))
		assert.Equal(t, "kling-3.0-omni", request.Model)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/kling/omni-video/kling-3.0-omni/task-1", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
}
