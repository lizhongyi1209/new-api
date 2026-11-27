package constant

const (
	KlingOfficialVideoPlugin = "kling-official-video"
	// KlingOfficialPlugin serves upstreams that expose only Kling's unified
	// task API (/{task}/{model} and /tasks) with a Bearer API key.
	KlingOfficialPlugin    = "kling-official-plugin"
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
		case KlingOfficialPlugin:
			switch name {
			case "kling-3.0", "kling-3.0-omni", "kling-4.0", "kling-4.0-flash", KlingImage40Model:
				return true
			}
		}
	case ChannelTypeAli:
		return key == WanOfficialVideoPlugin && (name == "wan3.0-video" || name == "wan3.0-video-prime")
	}
	return false
}

// KlingImage40Model is Kling Image 4.0. Kling names it kling-4.0 inside its
// image paths; the gateway keeps it apart from the kling-4.0 video model.
const KlingImage40Model = "kling-image-4.0"

func IsKlingOfficialVideoPlugin(platform TaskPlatform) bool {
	return platform == KlingOfficialVideoPlugin || platform == KlingOfficialPlugin
}

func IsOfficialVideoPlugin(platform TaskPlatform) bool {
	return IsKlingOfficialVideoPlugin(platform) || platform == WanOfficialVideoPlugin
}
