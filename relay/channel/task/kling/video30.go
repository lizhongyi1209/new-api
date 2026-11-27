package kling

import (
	"fmt"
	"strings"
)

// validateVideo30 checks Kling 3.0 text-to-video and image-to-video bodies on
// the unified task API against the official field list. Accepted bodies are
// forwarded unchanged, so every key is checked against that list.
func validateVideo30(metadata map[string]any, action string) error {
	if HasCallback(metadata) {
		return fmt.Errorf("%s", CallbackDisabledMessage)
	}
	top := []string{"contents", "settings", "options"}
	if action == "text" {
		top = []string{"prompt", "settings", "options"}
	}
	if err := validateVideo40Fields(metadata, "request", top...); err != nil {
		return err
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
	if settings, ok := metadata["settings"].(map[string]any); ok {
		allowed := []string{"multi_shot", "audio", "resolution", "duration"}
		if action == "text" {
			allowed = append(allowed, "aspect_ratio")
		}
		if err := validateVideo40Fields(settings, "settings", allowed...); err != nil {
			return err
		}
		if v, ok := settings["multi_shot"]; ok {
			if _, isBool := v.(bool); !isBool {
				return fmt.Errorf("settings.multi_shot must be a boolean")
			}
		}
		if err := video30Enum(settings, "audio", "native", "off"); err != nil {
			return err
		}
		if err := video30Enum(settings, "resolution", "720p", "1080p", "4k"); err != nil {
			return err
		}
		if err := video30Enum(settings, "aspect_ratio", "16:9", "9:16", "1:1"); err != nil {
			return err
		}
		if v, ok := settings["duration"]; ok {
			d, isNumber := v.(float64)
			if !isNumber || d != float64(int(d)) || d < 3 || d > 15 {
				return fmt.Errorf("settings.duration must be an integer between 3 and 15 seconds")
			}
		}
	} else if v, ok := metadata["settings"]; ok && v != nil {
		return fmt.Errorf("settings must be an object")
	}
	if action == "text" {
		prompt, _ := metadata["prompt"].(string)
		if strings.TrimSpace(prompt) == "" {
			return fmt.Errorf("prompt is required")
		}
		if len([]rune(prompt)) > 3072 {
			return fmt.Errorf("prompt exceeds 3072 characters")
		}
		return nil
	}
	contents, _ := metadata["contents"].([]any)
	if len(contents) == 0 {
		return fmt.Errorf("contents is required")
	}
	firstFrames, lastFrames, elements := 0, 0, 0
	ids := make(map[string]struct{}, len(contents))
	for _, raw := range contents {
		item, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("contents items must be objects")
		}
		if err := validateVideo40Fields(item, "content", "type", "text", "url", "id", "element_id"); err != nil {
			return err
		}
		if id, _ := item["id"].(string); id != "" {
			if _, exists := ids[id]; exists {
				return fmt.Errorf("duplicate content id %q", id)
			}
			ids[id] = struct{}{}
		}
		kind, _ := item["type"].(string)
		switch kind {
		case "prompt":
			text, _ := item["text"].(string)
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("prompt text is required")
			}
			if len([]rune(text)) > 3072 {
				return fmt.Errorf("prompt exceeds 3072 characters")
			}
		case "first_frame", "last_frame":
			if url, _ := item["url"].(string); strings.TrimSpace(url) == "" {
				return fmt.Errorf("url is required for %s", kind)
			}
			if kind == "first_frame" {
				firstFrames++
			} else {
				lastFrames++
			}
		case "element":
			elementID := fmt.Sprint(item["element_id"])
			if item["element_id"] == nil || strings.TrimSpace(elementID) == "" {
				return fmt.Errorf("element_id is required for element")
			}
			elements++
		default:
			return fmt.Errorf("unsupported content type %q", kind)
		}
	}
	if firstFrames != 1 || lastFrames > 1 {
		return fmt.Errorf("image-to-video requires one first_frame and at most one last_frame")
	}
	if elements > 3 {
		return fmt.Errorf("image-to-video accepts at most 3 elements")
	}
	return nil
}

func video30Enum(settings map[string]any, key string, allowed ...string) error {
	v, ok := settings[key]
	if !ok {
		return nil
	}
	s, _ := v.(string)
	for _, value := range allowed {
		if s == value {
			return nil
		}
	}
	return fmt.Errorf("settings.%s must be one of %s", key, strings.Join(allowed, ", "))
}
