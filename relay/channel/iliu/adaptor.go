package iliu

import (
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

const ChannelName = "iLiu Midjourney"

var ModelList = []string{
	"mj_imagine",
	"mj_blend",
	"mj_describe",
	"mj_upscale",
	"mj_variation",
	"mj_reroll",
	"mj_modal",
	"mj_zoom",
	"mj_custom_zoom",
	"mj_high_variation",
	"mj_low_variation",
	"mj_pan",
	"mj_shorten",
	"mj_edits",
	"mj_video",
	"mj_upload",
	"mj_action",
	"swap_face",
}

// Adaptor serves iLiu's OpenAI-compatible endpoint. Native Midjourney calls
// use the independent /v1/mj router and TaskAdaptor in this package.
type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", fmt.Errorf("relay info is nil")
	}
	if info.RequestURLPath != "/v1/chat/completions" {
		return "", fmt.Errorf("iLiu OpenAI compatibility only supports /v1/chat/completions")
	}
	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	requestPath := info.RequestURLPath
	if strings.HasSuffix(baseURL, "/v1") && strings.HasPrefix(requestPath, "/v1/") {
		requestPath = strings.TrimPrefix(requestPath, "/v1")
	}
	return baseURL + requestPath, nil
}

// DoRequest must remain on the outer adaptor. Calling the embedded OpenAI
// implementation would pass *openai.Adaptor to the request helper and bypass
// this provider's /v1 de-duplication in GetRequestURL.
func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
