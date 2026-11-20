package constant

const (
	KlingOfficialVideoPlugin = "kling-official-video"
	WanOfficialVideoPlugin   = "wan-official-video"
)

// OfficialVideoPluginForModel limits an ordinary channel's explicit plugin
// binding to the provider contracts whose public routes it can normalize.
func OfficialVideoPluginForModel(channelType int, name string) string {
	switch channelType {
	case ChannelTypeKling:
		switch name {
		case "kling-v2-6", "kling-v3", "kling-v3-omni", "kling-3.0", "kling-3.0-omni", "kling-4.0", "kling-4.0-flash":
			return KlingOfficialVideoPlugin
		}
	case ChannelTypeAli:
		if name == "wan3.0-video" || name == "wan3.0-video-prime" {
			return WanOfficialVideoPlugin
		}
	}
	return ""
}

func IsOfficialVideoPlugin(platform TaskPlatform) bool {
	return platform == KlingOfficialVideoPlugin || platform == WanOfficialVideoPlugin
}
