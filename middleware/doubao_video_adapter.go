package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// DoubaoVideoRequestConvert converts Volcengine's content-array request into
// the flat /v1/video/generations contract used by the xinhankr adaptor.
func DoubaoVideoRequestConvert() func(c *gin.Context) {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		var original map[string]any
		if err := common.UnmarshalBodyReusable(c, &original); err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, "Invalid request body")
			return
		}

		converted := map[string]any{}
		for _, key := range []string{
			"model", "resolution", "ratio", "duration", "camera_fixed",
			"generate_audio", "web_search", "seed",
		} {
			if value, ok := original[key]; ok {
				converted[key] = value
			}
		}

		var promptParts []string
		var images []any
		var videos []string
		var audios []string
		if content, ok := original["content"].([]any); ok {
			for _, rawItem := range content {
				item, ok := rawItem.(map[string]any)
				if !ok {
					continue
				}
				typeName, _ := item["type"].(string)
				switch typeName {
				case "text":
					if text, _ := item["text"].(string); strings.TrimSpace(text) != "" {
						promptParts = append(promptParts, strings.TrimSpace(text))
					}
				case "image_url":
					if mediaURL := doubaoMediaURL(item["image_url"]); mediaURL != "" {
						image := map[string]any{"url": mediaURL}
						if role, _ := item["role"].(string); role != "" {
							image["role"] = role
						}
						images = append(images, image)
					}
				case "video_url":
					if mediaURL := doubaoMediaURL(item["video_url"]); mediaURL != "" {
						videos = append(videos, mediaURL)
					}
				case "audio_url":
					if mediaURL := doubaoMediaURL(item["audio_url"]); mediaURL != "" {
						audios = append(audios, mediaURL)
					}
				}
			}
		}

		converted["prompt"] = strings.Join(promptParts, "\n")
		if len(images) > 0 {
			converted["images"] = images
		}
		if len(videos) > 0 {
			converted["videos"] = videos
		}
		if len(audios) > 0 {
			converted["audios"] = audios
		}

		data, err := common.Marshal(converted)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "Failed to convert request body")
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(data))
		c.Request.ContentLength = int64(len(data))
		c.Request.URL.Path = "/v1/video/generations"
		if bodyStorage, err := common.CreateBodyStorage(data); err == nil {
			c.Set(common.KeyBodyStorage, bodyStorage)
		}
		c.Set(common.KeyRequestBody, data)
		c.Next()
	}
}

func doubaoMediaURL(value any) string {
	switch media := value.(type) {
	case string:
		return strings.TrimSpace(media)
	case map[string]any:
		url, _ := media["url"].(string)
		return strings.TrimSpace(url)
	default:
		return ""
	}
}
