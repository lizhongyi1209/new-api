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
		assert.Equal(t, 5, request.Duration)
		assert.Equal(t, "1:1", request.Size)

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

func TestKling30ConvertersPrepareTaskFetch(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		middleware gin.HandlerFunc
		model      string
	}{
		{name: "omni", path: "/kling/omni-video/kling-3.0-omni/task-1", middleware: KlingOmniVideo30Convert(), model: "kling-3.0-omni"},
		{name: "motion control", path: "/kling/motion-control/kling-3.0/task-1", middleware: KlingMotionControl30Convert(), model: "kling-3.0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(test.middleware)
			router.GET(test.path, func(c *gin.Context) {
				var request relaycommon.TaskSubmitReq
				require.NoError(t, common.UnmarshalBodyReusable(c, &request))
				assert.Equal(t, test.model, request.Model)
				assert.NotZero(t, c.GetInt("relay_mode"))
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assert.Equal(t, http.StatusNoContent, response.Code)
		})
	}
}

func TestKlingMotionControl30ConvertPreservesOfficialRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(KlingMotionControl30Convert())
	router.POST("/kling/motion-control/kling-3.0", func(c *gin.Context) {
		var request relaycommon.TaskSubmitReq
		require.NoError(t, common.UnmarshalBodyReusable(c, &request))
		assert.Equal(t, "kling-3.0", request.Model)
		assert.Equal(t, "Keep the outfit unchanged", request.Prompt)

		settings, ok := request.Metadata["settings"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "video", settings["character_orientation"])
		assert.Equal(t, "original", settings["audio"])
		c.Status(http.StatusNoContent)
	})

	body := `{
		"contents":[
			{"type":"prompt","text":"Keep the outfit unchanged"},
			{"type":"image","url":"https://example.com/character.png"},
			{"type":"video","url":"https://example.com/motion.mp4"}
		],
		"settings":{"character_orientation":"video","audio":"original","resolution":"1080p"}
	}`
	request := httptest.NewRequest(http.MethodPost, "/kling/motion-control/kling-3.0", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
}
