package common

import "strings"

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

// IsGPTImage2 reports whether either the client-facing model or its mapped
// upstream model belongs to the gpt-image-2 family.
func IsGPTImage2(info *RelayInfo) bool {
	if info == nil {
		return false
	}
	models := []string{info.OriginModelName}
	if info.ChannelMeta != nil {
		models = append(models, info.UpstreamModelName)
	}
	for _, model := range models {
		model = strings.ToLower(strings.TrimSpace(model))
		if model == "gpt-image-2" || strings.HasPrefix(model, "gpt-image-2-") {
			return true
		}
	}
	return false
}
