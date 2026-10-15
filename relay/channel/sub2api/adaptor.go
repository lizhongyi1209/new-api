package sub2api

import (
	"io"
	"strings"

	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/newapi"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	newapi.Adaptor
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if isMultipartImageEdit(c, info) {
		imageEditAdaptor := &openai.Adaptor{}
		imageEditAdaptor.Init(info)
		return imageEditAdaptor.ConvertImageRequest(c, info, request)
	}
	return a.Adaptor.ConvertImageRequest(c, info, request)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if isMultipartImageEdit(c, info) {
		return channel.DoFormRequest(a, c, info, requestBody)
	}
	return a.Adaptor.DoRequest(c, info, requestBody)
}

func isMultipartImageEdit(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if c == nil || c.Request == nil || info == nil || info.RelayMode != relayconstant.RelayModeImagesEdits {
		return false
	}
	contentType := strings.ToLower(strings.TrimSpace(c.Request.Header.Get("Content-Type")))
	return strings.HasPrefix(contentType, "multipart/form-data")
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
