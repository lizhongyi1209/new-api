package kling

import (
	"fmt"
	"strings"
)

type video40Content struct {
	Type      string  `json:"type"`
	Text      *string `json:"text,omitempty"`
	URL       *string `json:"url,omitempty"`
	ID        *string `json:"id,omitempty"`
	ElementID *string `json:"element_id,omitempty"`
	VoiceID   *string `json:"voice_id,omitempty"`
}

type video40Settings struct {
	Audio       *string `json:"audio,omitempty"`
	Resolution  *string `json:"resolution,omitempty"`
	AspectRatio *string `json:"aspect_ratio,omitempty"`
	Duration    *int    `json:"duration,omitempty"`
}

type video40Profile struct {
	maxDuration int
	resolutions map[string]struct{}
}

func video40ProfileForModel(model string) video40Profile {
	if model == "kling-4.0-flash" {
		return video40Profile{maxDuration: 20, resolutions: map[string]struct{}{"720p": {}}}
	}
	return video40Profile{maxDuration: 30, resolutions: map[string]struct{}{"720p": {}, "1080p": {}}}
}

func kling40ModelFromPath(path string) (string, bool) {
	switch {
	case strings.Contains(path, "/kling-4.0-flash"):
		return "kling-4.0-flash", true
	case strings.Contains(path, "/kling-4.0"):
		return "kling-4.0", true
	default:
		return "", false
	}
}

type video40Watermark struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type video40Options struct {
	CallbackURL    *string           `json:"callback_url,omitempty"`
	ExternalTaskID *string           `json:"external_task_id,omitempty"`
	WatermarkInfo  *video40Watermark `json:"watermark_info,omitempty"`
}

type video40TextRequest struct {
	Prompt   string           `json:"prompt"`
	Settings *video40Settings `json:"settings,omitempty"`
	Options  *video40Options  `json:"options,omitempty"`
}

type video40ContentsRequest struct {
	Contents []video40Content `json:"contents"`
	Settings *video40Settings `json:"settings,omitempty"`
	Options  *video40Options  `json:"options,omitempty"`
}

func validateVideo40Fields(fields map[string]any, scope string, allowed ...string) error {
	permitted := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		permitted[key] = struct{}{}
	}
	for key := range fields {
		if _, ok := permitted[key]; !ok {
			return fmt.Errorf("unsupported %s field %q", scope, key)
		}
	}
	return nil
}

func validateVideo40Shape(metadata map[string]any, action string) error {
	var err error
	if action == "text" {
		err = validateVideo40Fields(metadata, "request", "prompt", "settings", "options")
	} else {
		err = validateVideo40Fields(metadata, "request", "contents", "settings", "options")
	}
	if err != nil {
		return err
	}
	if settings, ok := metadata["settings"].(map[string]any); ok {
		allowed := []string{"resolution", "duration"}
		if action != "image" {
			allowed = append(allowed, "aspect_ratio")
		}
		if action == "omni" {
			allowed = append(allowed, "audio")
		}
		if err := validateVideo40Fields(settings, "settings", allowed...); err != nil {
			return err
		}
	}
	if options, ok := metadata["options"].(map[string]any); ok {
		if err := validateVideo40Fields(options, "options", "callback_url", "external_task_id", "watermark_info"); err != nil {
			return err
		}
		if watermark, ok := options["watermark_info"].(map[string]any); ok {
			if err := validateVideo40Fields(watermark, "watermark_info", "enabled"); err != nil {
				return err
			}
		}
	}
	if contents, ok := metadata["contents"].([]any); ok {
		for _, raw := range contents {
			if item, ok := raw.(map[string]any); ok {
				if err := validateVideo40Fields(item, "content", "type", "text", "url", "id", "element_id", "voice_id"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateVideo40Prompt(prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("prompt text is required")
	}
	english, nonEnglish := 0, 0
	for _, r := range prompt {
		if r < 128 {
			english++
		} else {
			nonEnglish++
		}
	}
	if float64(english)/26000+float64(nonEnglish)/11000 > 1 {
		return fmt.Errorf("prompt exceeds Kling 4.0 character limit")
	}
	return nil
}

func validateVideo40Settings(settings *video40Settings, action string, profile video40Profile) error {
	if settings == nil {
		return nil
	}
	if settings.Duration != nil && (*settings.Duration < 3 || *settings.Duration > profile.maxDuration) {
		return fmt.Errorf("settings.duration must be between 3 and %d seconds", profile.maxDuration)
	}
	if settings.Resolution != nil {
		if _, ok := profile.resolutions[*settings.Resolution]; !ok {
			if profile.maxDuration == 20 {
				return fmt.Errorf("settings.resolution must be 720p")
			}
			return fmt.Errorf("settings.resolution must be 720p or 1080p")
		}
	}
	if action != "omni" && settings.Audio != nil {
		return fmt.Errorf("settings.audio requires omni-video")
	}
	if settings.Audio != nil && *settings.Audio != "native" && *settings.Audio != "original" {
		return fmt.Errorf("settings.audio must be native or original")
	}
	if action == "image" && settings.AspectRatio != nil {
		return fmt.Errorf("settings.aspect_ratio is not supported by image-to-video")
	}
	if settings.AspectRatio != nil {
		switch *settings.AspectRatio {
		case "21:9", "16:9", "9:16", "1:1":
		case "adaptive":
			if action != "omni" {
				return fmt.Errorf("settings.aspect_ratio adaptive requires omni-video")
			}
		default:
			return fmt.Errorf("invalid settings.aspect_ratio")
		}
	}
	return nil
}

func validateVideo40Contents(req video40ContentsRequest, imageOnly bool, profile video40Profile) error {
	if len(req.Contents) == 0 {
		return fmt.Errorf("contents is required")
	}
	action := "omni"
	if imageOnly {
		action = "image"
	}
	if err := validateVideo40Settings(req.Settings, action, profile); err != nil {
		return err
	}
	images, middleFrames, referenceImages := 0, 0, 0
	videos, baseVideos, elements, voices := 0, 0, 0, 0
	ids := make(map[string]struct{}, len(req.Contents))
	for _, content := range req.Contents {
		if content.ID != nil && *content.ID != "" {
			if _, exists := ids[*content.ID]; exists {
				return fmt.Errorf("duplicate content id %q", *content.ID)
			}
			ids[*content.ID] = struct{}{}
		}
		switch content.Type {
		case "prompt":
			if content.Text == nil {
				return fmt.Errorf("prompt text is required")
			}
			if err := validateVideo40Prompt(*content.Text); err != nil {
				return err
			}
		case "first_frame", "middle_frame", "last_frame", "refer_image":
			images++
			if content.Type == "middle_frame" {
				middleFrames++
			}
			if content.Type == "refer_image" {
				referenceImages++
			}
			if imageOnly && content.Type == "refer_image" {
				return fmt.Errorf("refer_image requires omni-video")
			}
			if content.URL == nil || strings.TrimSpace(*content.URL) == "" {
				return fmt.Errorf("url is required for %s", content.Type)
			}
		case "feature_video", "base_video":
			if imageOnly {
				return fmt.Errorf("%s requires omni-video", content.Type)
			}
			videos++
			if content.Type == "base_video" {
				baseVideos++
			}
			if content.URL == nil || strings.TrimSpace(*content.URL) == "" {
				return fmt.Errorf("url is required for %s", content.Type)
			}
		case "element":
			if imageOnly {
				return fmt.Errorf("element requires omni-video")
			}
			elements++
			if content.ElementID == nil || strings.TrimSpace(*content.ElementID) == "" || content.ID == nil || *content.ID == "" {
				return fmt.Errorf("element_id and id are required for element")
			}
		case "voice":
			if imageOnly {
				return fmt.Errorf("voice requires omni-video")
			}
			voices++
			if content.VoiceID == nil || strings.TrimSpace(*content.VoiceID) == "" || content.ID == nil || *content.ID == "" {
				return fmt.Errorf("voice_id and id are required for voice")
			}
		default:
			return fmt.Errorf("unsupported content type %q", content.Type)
		}
	}
	if req.Settings != nil && req.Settings.Audio != nil && *req.Settings.Audio == "original" {
		if voices > 0 {
			return fmt.Errorf("settings.audio original cannot be combined with voice")
		}
		if videos == 0 {
			return fmt.Errorf("settings.audio original requires base_video or feature_video")
		}
	}
	if imageOnly && images == 0 {
		return fmt.Errorf("image-to-video requires at least one image")
	}
	if images > 10 || videos > 5 || baseVideos > 1 || elements > 7 || voices > 3 || images+videos+elements+voices > 15 {
		return fmt.Errorf("contents exceeds Kling 4.0 material limits")
	}
	if middleFrames > 1 || referenceImages > 1 {
		for _, content := range req.Contents {
			if (content.Type == "middle_frame" && middleFrames > 1 || content.Type == "refer_image" && referenceImages > 1) && (content.ID == nil || *content.ID == "") {
				return fmt.Errorf("id is required for multiple %s inputs", content.Type)
			}
		}
	}
	return nil
}
