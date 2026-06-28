package tencentvideo

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestBodyForcesLogoAddOff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request = &http.Request{Method: http.MethodPost}
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:   "cat running",
		Image:    "https://example.com/cat.png",
		Images:   []string{"https://example.com/cat.png"},
		Duration: 5,
		Mode:     "std",
		Metadata: map[string]interface{}{
			"Sound":   "off",
			"LogoAdd": 1,
		},
	})

	adaptor := &TaskAdaptor{}
	body, err := adaptor.BuildRequestBody(c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "tencent-v3"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action: "generate",
		},
	})
	require.NoError(t, err)

	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var payload submitPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	require.NotNil(t, payload.LogoAdd)
	assert.Equal(t, 0, *payload.LogoAdd)
	assert.Equal(t, "off", payload.Sound)
	assert.Equal(t, "v3.0", payload.Model)
	assert.Contains(t, string(data), `"LogoAdd":0`)
	assert.NotContains(t, strings.ToLower(string(data)), "logoadd\":1")
}
