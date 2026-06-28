package kling

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestBodyKeepsOmniActionWithoutLegacyImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/kling/v1/videos/omni-video", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt: "Make the person in <<<image_1>>> wave to the camera",
		ImageList: []relaycommon.TaskImageInfo{
			{ImageURL: "https://example.com/image.png"},
		},
		Duration: 5,
		Mode:     "pro",
		Size:     "16:9",
	})
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "kling-v3-omni",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action: constant.TaskActionOmniVideo,
		},
	}

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var payload requestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, constant.TaskActionOmniVideo, info.Action)
	assert.Empty(t, c.GetString("action"))
	assert.Equal(t, "kling-v3-omni", payload.ModelName)
	assert.Equal(t, []ImageItem{{ImageUrl: "https://example.com/image.png"}}, payload.ImageList)
}
