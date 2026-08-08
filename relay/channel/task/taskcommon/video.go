package taskcommon

import "strings"

// MediaURL is a provider-neutral media reference used by multimodal video
// generation APIs. Providers may apply their own URL validation or asset
// conversion after the request has been normalized.
type MediaURL struct {
	URL string `json:"url,omitempty"`
}

// VideoContentItem is the common content item shared by video providers that
// accept text, image, video, and audio inputs in one ordered array.
type VideoContentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

func FirstTextContent(content []VideoContentItem) string {
	for _, item := range content {
		if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
			return item.Text
		}
	}
	return ""
}

func HasTextContent(content []VideoContentItem) bool {
	return FirstTextContent(content) != ""
}
