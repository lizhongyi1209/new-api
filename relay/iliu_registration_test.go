package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/iliu"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisteredILiuAdaptorUsesProviderURLBuilder(t *testing.T) {
	apiType, ok := common.ChannelType2APIType(constant.ChannelTypeILiuMidjourney)
	require.True(t, ok)
	adaptor := GetAdaptor(apiType)
	require.IsType(t, &iliu.Adaptor{}, adaptor)

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl: "https://iliu.ai/v1",
	}}
	info.RequestURLPath = "/v1/chat/completions"
	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://iliu.ai/v1/chat/completions", requestURL)
}
