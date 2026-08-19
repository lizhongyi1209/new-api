package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
)

func TestFilterChannelsByRequestPathIsolatesILiuNativeRoutes(t *testing.T) {
	originalChannels := channelsIDM
	originalAdvancedConfigs := channel2advancedCustomConfig
	t.Cleanup(func() {
		channelsIDM = originalChannels
		channel2advancedCustomConfig = originalAdvancedConfigs
	})

	channelsIDM = map[int]*Channel{
		1: {Id: 1, Type: constant.ChannelTypeMidjourney},
		2: {Id: 2, Type: constant.ChannelTypeILiuMidjourney},
		3: {Id: 3, Type: constant.ChannelTypeOpenAI},
	}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{}

	assert.Equal(t, []int{2}, filterChannelsByRequestPath([]int{1, 2, 3}, "/v1/mj/submit/imagine"))
	assert.Equal(t, []int{1, 3}, filterChannelsByRequestPath([]int{1, 2, 3}, "/mj/submit/imagine"))
	assert.Equal(t, []int{1, 2, 3}, filterChannelsByRequestPath([]int{1, 2, 3}, "/v1/chat/completions"))
	assert.Equal(t, []int{1, 3}, filterChannelsByRequestPath([]int{1, 2, 3}, "/v1/images/generations"))
}
