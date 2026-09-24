package zhipu_4v

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesRequestUsesGLMV1Endpoint(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://open.bigmodel.cn"},
		RelayMode:   relayconstant.RelayModeResponses,
	}

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://open.bigmodel.cn/api/v1/responses", requestURL)

	request := dto.OpenAIResponsesRequest{Model: "glm-5"}
	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, request)
	require.NoError(t, err)
	assert.Equal(t, request, converted)
}
