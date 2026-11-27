package constant

const (
	KlingOfficialVideoPlugin = "kling-official-video"
	// KlingNativeVideoPlugin serves upstreams that expose only Kling's unified
	// task API (/{task}/{model} and /tasks) with a Bearer API key.
	KlingNativeVideoPlugin = "kling-native"
	WanOfficialVideoPlugin = "wan-official-video"
)

// OfficialVideoPluginAllowed limits an ordinary channel's explicit plugin
// binding to the provider contracts whose public routes it can normalize.
func OfficialVideoPluginAllowed(channelType int, name, key string) bool {
	switch channelType {
	case ChannelTypeKling:
		switch key {
		case KlingOfficialVideoPlugin:
			switch name {
			case "kling-v2-6", "kling-v3", "kling-v3-omni", "kling-3.0", "kling-3.0-omni", "kling-4.0", "kling-4.0-flash":
				return true
			}
		case KlingNativeVideoPlugin:
			switch name {
			case "kling-3.0", "kling-3.0-omni", "kling-4.0", "kling-4.0-flash":
				return true
			}
		}
	case ChannelTypeAli:
		return key == WanOfficialVideoPlugin && (name == "wan3.0-video" || name == "wan3.0-video-prime")
	}
	return false
}

func IsKlingOfficialVideoPlugin(platform TaskPlatform) bool {
	return platform == KlingOfficialVideoPlugin || platform == KlingNativeVideoPlugin
}

func IsOfficialVideoPlugin(platform TaskPlatform) bool {
	return IsKlingOfficialVideoPlugin(platform) || platform == WanOfficialVideoPlugin
}
