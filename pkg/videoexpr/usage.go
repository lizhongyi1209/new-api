package videoexpr

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/tidwall/gjson"
)

// Estimate consumes the adaptor's validated, effective request rather than
// letting arbitrary client-supplied usage facts become billing multipliers.
func Estimate(meta Metadata, req relaycommon.TaskSubmitReq, action string) (map[string]any, error) {
	if meta.Meter == "" {
		return nil, fmt.Errorf("native video model has no usage meter")
	}
	encoded, err := common.Marshal(req.Metadata)
	if err != nil {
		return nil, err
	}
	body := gjson.ParseBytes(encoded)
	seconds := float64(req.Duration)
	if req.Seconds != "" {
		seconds, err = strconv.ParseFloat(req.Seconds, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid video seconds")
		}
	}
	if err = bounded("seconds", seconds, relaycommon.MaxTaskDurationSeconds); err != nil {
		return nil, err
	}
	// Check passthrough metadata even when a normalized top-level value exists.
	for _, path := range []string{"duration", "seconds", "durationSeconds", "parameters.durationSeconds", "parameters.duration"} {
		if value := body.Get(path); value.Exists() {
			n, parseErr := numeric(value)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid %s: %w", path, parseErr)
			}
			if err = bounded(path, n, relaycommon.MaxTaskDurationSeconds); err != nil {
				return nil, err
			}
			if seconds == 0 {
				seconds = n
			}
		}
	}
	if frames := body.Get("frames"); frames.Exists() {
		n, parseErr := numeric(frames)
		if parseErr != nil {
			return nil, parseErr
		}
		if err = bounded("frames", n, relaycommon.MaxTaskDurationSeconds*24); err != nil {
			return nil, err
		}
		if seconds == 0 {
			seconds = math.Ceil(n / 24)
		}
	}
	res := req.EffectiveResolution
	if res == "" {
		res = req.Resolution
	}
	if res == "" {
		res = firstString(body, "resolution", "parameters.resolution")
	}
	if res == "" && strings.HasSuffix(strings.ToLower(req.Size), "p") {
		res = req.Size
	}
	images := req.ImageCount
	if images == 0 {
		images = len(req.Images)
	}
	if images == 0 {
		images = len(req.ImageList)
	}
	if images == 0 && (req.Image != "" || req.ImageUrl != "" || req.InputReference != "") {
		images = 1
	}
	hasVideo := req.HasVideo
	contentImages := 0
	for _, item := range body.Get("content").Array() {
		switch item.Get("type").String() {
		case "video_url", "video":
			hasVideo = true
		case "image_url", "image":
			contentImages++
		}
	}
	if contentImages > images {
		images = contentImages
	}
	for _, path := range []string{"images", "reference_images"} {
		if count := len(body.Get(path).Array()); count > images {
			images = count
		}
	}
	if body.Get("image").Exists() && images == 0 {
		images = 1
	}
	if len(body.Get("videos").Array()) > 0 || body.Get("video").Exists() || body.Get("video_url").Exists() {
		hasVideo = true
	}
	if err = bounded("input_images", float64(images), dto.MaxImageN); err != nil {
		return nil, err
	}

	facts := map[string]any{}
	switch meta.Meter {
	case Seedance:
		if seconds == 0 {
			seconds = 15
		} // Automatic duration: reserve the output maximum.
		if res == "" {
			res = "720p"
		}
		res = normalizeEnum(res, meta.Schema["resolution"].Enum)
		width, height := 1280.0, 720.0
		switch res {
		case "480p":
			width, height = 864, 480
		case "1080p":
			width, height = 1920, 1080
		case "4k":
			width, height = 3840, 2160
		}
		input := "none"
		if hasVideo {
			input = "video"
			seconds += 15
		}
		facts = map[string]any{"tokens": math.Ceil(seconds * width * height * 24 / 1024), "resolution": res, "video_input": input}
	case Kling:
		if seconds == 0 {
			seconds = 5
		}
		// Resource deductions are unknown until completion. Reserve a conservative
		// credit estimate without consulting a mutable currency price or ratio.
		facts["units"] = seconds * 2
	case H3, H3Max, H3MaxTurbo:
		if seconds == 0 {
			seconds = 5
		}
		if res == "" {
			res = "768P"
		}
		inputSeconds := 0.0
		if hasVideo {
			inputSeconds = 15
		}
		facts = map[string]any{"seconds": seconds, "resolution": normalizeEnum(res, meta.Schema["resolution"].Enum), "input_images": images, "input_video_seconds": inputSeconds}
	case Grok:
		operation := "generate"
		if seconds == 0 {
			seconds = 8
		}
		if res == "" {
			res = "480p"
		}
		if action == constant.TaskActionVideoEdit {
			seconds, res, operation = 8.7, "auto", "edit" // Upstream caps edit duration at 8.7s; resolution follows input media.
		}
		if action == constant.TaskActionVideoExtend {
			operation = "extend"
			if req.Duration == 0 && req.Seconds == "" {
				seconds = 6
			}
		}
		facts = map[string]any{"seconds": seconds, "resolution": normalizeEnum(res, meta.Schema["resolution"].Enum), "operation": operation, "input_images": images}
	case Omni:
		facts = map[string]any{"input_tokens": 8192, "text_output_tokens": 8192, "video_output_tokens": 10 * 5792}
	case Veo:
		if seconds == 0 {
			seconds = 8
		}
		if res == "" {
			res = "720p"
		}
		facts = map[string]any{"seconds": seconds, "resolution": normalizeEnum(res, meta.Schema["resolution"].Enum)}
	case Sora:
		if seconds == 0 {
			seconds = 4
		}
		size := req.Size
		if size == "" {
			size = "720x1280"
		}
		facts = map[string]any{"seconds": seconds, "size": size}
	}
	return facts, Validate(meta.Schema, facts)
}

// Complete returns only authoritative completion quantities. Missing usage is
// an error for token/credit meters; explicit zero remains a valid actual value.
// Request-priced video APIs may omit duration, retaining the frozen request.
func Complete(meter string, submitted map[string]any, body []byte, result *relaycommon.TaskInfo) (map[string]any, error) {
	meta := ForMeter(meter)
	if meta.Meter == "" {
		return nil, fmt.Errorf("unknown native video meter %q", meter)
	}
	root := gjson.ParseBytes(body)
	if task := root.Get("task"); task.IsObject() {
		root = task
	}
	usage := root.Get("metadata.usage")
	if !usage.IsObject() {
		usage = root.Get("usage")
	}
	facts := map[string]any{}
	switch meter {
	case Seedance:
		tokens := usage.Get("completion_tokens")
		if !tokens.Exists() {
			tokens = usage.Get("total_tokens")
		}
		if !tokens.Exists() {
			return nil, fmt.Errorf("upstream video billing tokens are missing")
		}
		n, err := numeric(tokens)
		if err != nil {
			return nil, err
		}
		facts["tokens"] = n
	case Kling:
		deduction := root.Get("data.final_unit_deduction")
		if !deduction.Exists() {
			deduction = root.Get("Response.FinalUnitDeduction")
		}
		if deduction.Exists() {
			n, err := numeric(deduction)
			if err != nil {
				return nil, err
			}
			facts["units"] = n
		} else {
			// Kling 3.0 reports one bill per charge source; preserve fractional amounts.
			found, sum := false, 0.0
			for _, bill := range root.Get("data.0.billing").Array() {
				kind := bill.Get("charge_type").String()
				if kind != "unit" && kind != "cash" {
					continue
				}
				n, err := numeric(bill.Get("amount"))
				if err != nil {
					return nil, err
				}
				if err = bounded("units", n, math.MaxInt32); err != nil {
					return nil, err
				}
				found, sum = true, sum+n
			}
			if !found {
				// The existing unified Kling protocol reports credit consumption in
				// usage.completion_tokens (total_tokens is its compatibility fallback).
				credits := usage.Get("completion_tokens")
				if !credits.Exists() {
					credits = usage.Get("total_tokens")
				}
				if !credits.Exists() {
					return nil, fmt.Errorf("upstream video resource deduction is missing")
				}
				n, err := numeric(credits)
				if err != nil {
					return nil, err
				}
				sum = n
			}
			facts["units"] = sum
		}
	case H3, H3Max, H3MaxTurbo:
		if !usage.IsObject() {
			return nil, fmt.Errorf("upstream MiniMax video usage is missing")
		}
		for key, path := range map[string]string{"seconds": "output_seconds", "input_video_seconds": "input_seconds", "input_images": "input_image_count"} {
			if value := usage.Get(path); value.Exists() {
				n, err := numeric(value)
				if err != nil {
					return nil, err
				}
				facts[key] = n
			}
		}
		if _, found := facts["input_video_seconds"]; !found {
			if total := usage.Get("total_seconds"); total.Exists() {
				n, err := numeric(total)
				if err != nil {
					return nil, err
				}
				if output, exists := facts["seconds"].(float64); exists && n >= output {
					facts["input_video_seconds"] = n - output
				}
			}
		}
	case Omni:
		if !usage.IsObject() || (!usage.Get("total_input_tokens").Exists() && !usage.Get("input_tokens_by_modality").Exists()) || (!usage.Get("total_output_tokens").Exists() && !usage.Get("output_tokens_by_modality").Exists()) {
			return nil, fmt.Errorf("upstream Omni video usage is missing")
		}
		if result.CompletionTokens > result.ThoughtTokens && len(result.OutputTokensByModality) == 0 {
			return nil, fmt.Errorf("upstream Omni output modality usage is missing")
		}
		video := result.OutputTokensByModality["video"]
		if result.CompletionTokens < video {
			return nil, fmt.Errorf("video tokens exceed total output tokens")
		}
		facts = map[string]any{"input_tokens": result.PromptTokens, "text_output_tokens": result.CompletionTokens - video, "video_output_tokens": video}
	case Grok, Veo, Sora:
		durationPaths := []string{"video.duration", "metadata.video.duration", "duration_seconds", "seconds", "duration", "metadata.duration"}
		if meter == Grok && submitted["operation"] == "extend" {
			// Extension bills the generated portion. video.duration includes the
			// original input and cannot replace the frozen requested extension.
			durationPaths = []string{"usage.output_seconds"}
		}
		for _, path := range durationPaths {
			if value := root.Get(path); value.Exists() {
				n, err := numeric(value)
				if err != nil {
					return nil, err
				}
				facts["seconds"] = n
				break
			}
		}
		if meter == Grok && submitted["operation"] != "extend" && result.Metadata != nil {
			if value, exists := result.Metadata["duration"]; exists && facts["seconds"] == nil {
				facts["seconds"] = value
			}
		}
	}
	if _, hasResolution := meta.Schema["resolution"]; hasResolution {
		if res := firstString(root, "metadata.resolution", "video.resolution", "resolution"); res != "" {
			facts["resolution"] = normalizeEnum(res, meta.Schema["resolution"].Enum)
		}
	}
	if meter == Sora {
		if size := root.Get("size").String(); size != "" {
			facts["size"] = size
		}
	}
	return facts, Validate(meta.Schema, facts)
}

// Validate bounds every multiplier before expression evaluation. Unknown fields
// and enum values fail closed, preventing cross-provider schema mismatches.
func Validate(schema map[string]jsplugin.UsageFieldSchema, facts map[string]any) error {
	for key, value := range facts {
		field, exists := schema[key]
		if !exists {
			return fmt.Errorf("unknown video usage field %s", key)
		}
		if len(field.Enum) > 0 {
			text, ok := value.(string)
			if !ok || !slices.Contains(field.Enum, text) {
				return fmt.Errorf("invalid video usage %s", key)
			}
			continue
		}
		var n float64
		switch v := value.(type) {
		case int:
			n = float64(v)
		case float64:
			n = v
		default:
			return fmt.Errorf("invalid numeric video usage %s", key)
		}
		limit := float64(math.MaxInt32)
		switch field.Unit {
		case "second":
			limit = relaycommon.MaxTaskDurationSeconds
		case "count":
			limit = dto.MaxImageN
		}
		if err := bounded(key, n, limit); err != nil {
			return err
		}
		if (field.Unit == "token" || field.Unit == "count") && n != math.Trunc(n) {
			return fmt.Errorf("%s must be an integer", key)
		}
	}
	return nil
}

func bounded(key string, value, maximum float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maximum {
		return fmt.Errorf("%s must be finite and between 0 and %g", key, maximum)
	}
	return nil
}

func numeric(value gjson.Result) (float64, error) {
	if value.Type != gjson.Number && value.Type != gjson.String {
		return 0, fmt.Errorf("video usage must be numeric")
	}
	n, err := strconv.ParseFloat(value.String(), 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("invalid numeric video usage")
	}
	return n, nil
}

func firstString(body gjson.Result, paths ...string) string {
	for _, path := range paths {
		if value := body.Get(path).String(); value != "" {
			return value
		}
	}
	return ""
}

func normalizeEnum(value string, values []string) string {
	for _, allowed := range values {
		if strings.EqualFold(value, allowed) {
			return allowed
		}
	}
	return value
}
