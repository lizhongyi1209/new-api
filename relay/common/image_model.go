package common

import "strings"

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
