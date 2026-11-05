package common

import (
	"strings"

	"github.com/QuantumNous/new-api/relaykit/dto"
)

// IsGPTImage reports whether the client-facing or mapped upstream model
// starts with gpt-image.
func IsGPTImage(info *RelayInfo) bool {
	if info == nil {
		return false
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(info.OriginModelName)), "gpt-image") {
		return true
	}
	return info.ChannelMeta != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(info.UpstreamModelName)), "gpt-image")
}

// IsGPTImage2 reports whether either the client-facing or its mapped upstream
// model belongs to the gpt-image-2 family: gpt-image-2 and its aliases, plus
// the 2.5 and 2.6 series.
func IsGPTImage2(info *RelayInfo) bool {
	if info == nil {
		return false
	}
	if dto.IsGPTImage2FamilyModel(info.OriginModelName) {
		return true
	}
	return info.ChannelMeta != nil && dto.IsGPTImage2FamilyModel(info.UpstreamModelName)
}

// RequiresOmittedInputFidelity reports whether input_fidelity must be dropped
// before forwarding. The base gpt-image-2 model always processes image inputs at
// high fidelity and rejects an explicit value.
func RequiresOmittedInputFidelity(info *RelayInfo) bool {
	if info == nil {
		return false
	}
	if dto.RequiresOmittedInputFidelity(info.OriginModelName) {
		return true
	}
	return info.ChannelMeta != nil && dto.RequiresOmittedInputFidelity(info.UpstreamModelName)
}
