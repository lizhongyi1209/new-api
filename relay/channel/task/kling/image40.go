package kling

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// klingImage40Route parses Kling Image 4.0 paths, /{image-generation|
// long-image-generation}/{kling-4.0|kling-image-4.0}[/{task_id}], at the root
// or under /kling, and returns the task action.
func klingImage40Route(path string) (string, bool) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(path, "/kling"), "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || (parts[1] != "kling-4.0" && parts[1] != constant.KlingImage40Model) {
		return "", false
	}
	switch parts[0] {
	case "image-generation":
		return constant.TaskActionKlingImage40, true
	case "long-image-generation":
		return constant.TaskActionKlingLongImage40, true
	}
	return "", false
}

func isKlingImage40Action(action string) bool {
	return action == constant.TaskActionKlingImage40 || action == constant.TaskActionKlingLongImage40
}

// validateImage40 checks an Image 4.0 body against the official field list.
// Accepted bodies are forwarded unchanged, so every key is checked.
func validateImage40(metadata map[string]any, long bool) error {
	if HasCallback(metadata) {
		return fmt.Errorf("%s", CallbackDisabledMessage)
	}
	if err := validateVideo40Fields(metadata, "request", "contents", "settings", "options"); err != nil {
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
	settings, _ := metadata["settings"].(map[string]any)
	if settings == nil {
		if v, ok := metadata["settings"]; ok && v != nil {
			return fmt.Errorf("settings must be an object")
		}
		settings = map[string]any{}
	}
	skill := ""
	if long {
		if err := validateVideo40Fields(settings, "settings", "skill", "length", "resolution"); err != nil {
			return err
		}
		skill, _ = settings["skill"].(string)
		if skill != "ecommerce" && skill != "webpage" {
			return fmt.Errorf("settings.skill must be ecommerce or webpage")
		}
		if err := video30Enum(settings, "length", "short", "medium", "long"); err != nil {
			return err
		}
		if err := video30Enum(settings, "resolution", "1k", "2k"); err != nil {
			return err
		}
	} else {
		if err := validateVideo40Fields(settings, "settings", "result_type", "n", "series_amount", "resolution", "aspect_ratio"); err != nil {
			return err
		}
		if err := video30Enum(settings, "result_type", "single", "series"); err != nil {
			return err
		}
		if err := video30Enum(settings, "resolution", "2k", "4k", "8k"); err != nil {
			return err
		}
		if err := video30Enum(settings, "aspect_ratio", "16:9", "9:16", "1:1", "4:3", "3:4", "3:2", "2:3", "21:9", "auto"); err != nil {
			return err
		}
		if v, ok := settings["n"]; ok {
			n, isNumber := v.(float64)
			if !isNumber || n != float64(int(n)) || n < 1 || n > 9 {
				return fmt.Errorf("settings.n must be an integer between 1 and 9")
			}
		}
		if v, ok := settings["series_amount"]; ok && v != "auto" {
			n, isNumber := v.(float64)
			if !isNumber || n != float64(int(n)) || n < 2 || n > 10 {
				return fmt.Errorf("settings.series_amount must be auto or an integer between 2 and 10")
			}
		}
	}

	contents, _ := metadata["contents"].([]any)
	prompts, references, styles, subjects := 0, 0, 0, 0
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
		switch {
		case kind == "prompt":
			text, _ := item["text"].(string)
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("prompt text is required")
			}
			prompts++
		case kind == "refer_image" || (long && (kind == "style_refer_image" || kind == "subject_refer_image")):
			if url, _ := item["url"].(string); strings.TrimSpace(url) == "" {
				return fmt.Errorf("url is required for %s", kind)
			}
			switch kind {
			case "refer_image":
				references++
			case "style_refer_image":
				styles++
			default:
				subjects++
			}
		case kind == "element" && !long:
			if item["element_id"] == nil || strings.TrimSpace(fmt.Sprint(item["element_id"])) == "" {
				return fmt.Errorf("element_id is required for element")
			}
			references++
		default:
			return fmt.Errorf("unsupported content type %q", kind)
		}
	}
	if prompts != 1 {
		return fmt.Errorf("contents requires exactly one prompt")
	}
	if !long {
		if references > 10 {
			return fmt.Errorf("contents accepts at most 10 reference images and elements")
		}
		return nil
	}
	if skill == "ecommerce" {
		if styles > 0 || subjects > 0 {
			return fmt.Errorf("ecommerce long images take refer_image references only")
		}
		if references < 1 || references > 3 {
			return fmt.Errorf("ecommerce long images require 1 to 3 refer_image inputs")
		}
		return nil
	}
	if references > 0 {
		return fmt.Errorf("webpage long images take style_refer_image and subject_refer_image, not refer_image")
	}
	if styles > 1 || subjects > 1 {
		return fmt.Errorf("webpage long images accept at most one style_refer_image and one subject_refer_image")
	}
	return nil
}
