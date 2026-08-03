package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsGPTImage2(t *testing.T) {
	tests := []struct {
		name string
		info *RelayInfo
		want bool
	}{
		{name: "exact origin", info: &RelayInfo{OriginModelName: "gpt-image-2"}, want: true},
		{name: "origin alias", info: &RelayInfo{OriginModelName: "GPT-IMAGE-2-C"}, want: true},
		{name: "mapped upstream", info: &RelayInfo{OriginModelName: "custom-image", ChannelMeta: &ChannelMeta{UpstreamModelName: "gpt-image-2"}}, want: true},
		{name: "different model", info: &RelayInfo{OriginModelName: "gpt-image-1"}, want: false},
		{name: "similar prefix", info: &RelayInfo{OriginModelName: "gpt-image-20"}, want: false},
		{name: "nil info", info: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsGPTImage2(tt.info))
		})
	}
}
