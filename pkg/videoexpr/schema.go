// Package videoexpr defines usage meters for native asynchronous video adaptors.
// Prices belong exclusively to the saved billing expression, never to a meter.
package videoexpr

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
)

const (
	Seedance   = "seedance"
	Kling      = "kling"
	H3         = "minimax_h3"
	H3Max      = "minimax_h3_max"
	H3MaxTurbo = "minimax_h3_max_turbo"
	Grok       = "grok"
	Omni       = "gemini_omni"
	Veo        = "veo"
	Sora       = "sora"
)

type Metadata struct {
	Meter    string
	Schema   map[string]jsplugin.UsageFieldSchema
	Examples []jsplugin.UsageExample
}

// ForModel checks both the actual native adaptor and its mapped upstream model.
// In particular, an arbitrary text/image model does not acquire a video meter.
func ForModel(channelType int, model string) Metadata {
	name := strings.ToLower(strings.TrimSpace(model))
	meter := ""
	switch channelType {
	case constant.ChannelTypeServiceInferenceVideo:
		switch {
		case strings.Contains(name, "seedance"):
			meter = Seedance
		case name == "minimax-h3":
			meter = H3
		case name == "minimax-h3-max":
			meter = H3Max
		case name == "minimax-h3-max-turbo":
			meter = H3MaxTurbo
		case strings.HasPrefix(name, "grok-imagine") && strings.Contains(name, "video"):
			meter = Grok
		}
	case constant.ChannelTypeDoubaoVideo, constant.ChannelTypeVolcEngine, constant.ChannelTypeXinhankr:
		if strings.Contains(name, "seedance") {
			meter = Seedance
		}
	case constant.ChannelTypeKling, constant.ChannelTypeTencentVideo:
		if strings.HasPrefix(name, "kling-") {
			meter = Kling
		}
	case constant.ChannelTypeMiniMax:
		if name == "minimax-h3" {
			meter = H3
		}
		if name == "minimax-h3-max" {
			meter = H3Max
		}
	case constant.ChannelTypeXai:
		if strings.HasPrefix(name, "grok-imagine") && strings.Contains(name, "video") {
			meter = Grok
		}
	case constant.ChannelTypeGemini, constant.ChannelTypeVertexAi:
		if strings.HasPrefix(name, "gemini-omni-") {
			meter = Omni
		}
		if strings.HasPrefix(name, "veo-") {
			meter = Veo
		}
	case constant.ChannelTypeOpenAI, constant.ChannelTypeSora:
		switch {
		case strings.HasPrefix(name, "grok-imagine") && strings.Contains(name, "video"):
			meter = Grok
		case strings.HasPrefix(name, "veo-"):
			meter = Veo
		case strings.HasPrefix(name, "sora"), strings.HasPrefix(name, "omni_flash_"):
			meter = Sora
		}
	}
	if meter == "" {
		return Metadata{}
	}
	meta := ForMeter(meter)
	if meter == Seedance && (strings.Contains(name, "fast") || strings.Contains(name, "mini")) {
		field := meta.Schema["resolution"]
		field.Enum = []string{"480p", "720p"}
		meta.Schema["resolution"] = field
	}
	return meta
}

// ForMeter returns detached definitions suitable for pricing APIs and snapshots.
func ForMeter(meter string) Metadata {
	meta := Metadata{Meter: meter, Schema: map[string]jsplugin.UsageFieldSchema{}}
	switch meter {
	case Seedance:
		meta.Schema["tokens"] = number("token", "Billing tokens", "计费 Token")
		meta.Schema["resolution"] = resolution("480p", "720p", "1080p", "4k")
		meta.Schema["video_input"] = jsplugin.UsageFieldSchema{Enum: []string{"none", "video"}, Description: jsplugin.LocalizedText{"en": "Reference video input", "zh": "参考视频输入"}, EnumLabels: map[string]jsplugin.LocalizedText{"none": {"en": "No reference video", "zh": "无参考视频"}, "video": {"en": "With reference video", "zh": "有参考视频"}}}
		meta.Examples = []jsplugin.UsageExample{
			{Label: "720p · 5s", Facts: map[string]any{"tokens": 108000, "resolution": "720p", "video_input": "none"}},
			{Label: "720p · 5s + 4s input", Facts: map[string]any{"tokens": 194400, "resolution": "720p", "video_input": "video"}},
		}
	case Kling:
		meta.Schema["units"] = number("credit", "Kling credit units", "可灵资源包单位")
		meta.Examples = []jsplugin.UsageExample{{Label: "1 credit", Facts: map[string]any{"units": 1}}, {Label: "3.5 credits", Facts: map[string]any{"units": 3.5}}}
	case H3, H3Max, H3MaxTurbo:
		meta.Schema["seconds"] = number("second", "Output video duration", "输出视频时长")
		meta.Schema["resolution"] = resolution("768P", "2K")
		meta.Schema["input_images"] = number("count", "Input images", "输入图片数量")
		meta.Schema["input_video_seconds"] = number("second", "Input video duration", "输入视频时长")
		if meter == H3Max {
			meta.Schema["resolution"] = resolution("480P", "768P")
		}
		if meter == H3MaxTurbo {
			meta.Schema["resolution"] = resolution("480P", "768P", "1080P")
		}
		meta.Examples = []jsplugin.UsageExample{{Label: "768P · 5s", Facts: map[string]any{"seconds": 5, "resolution": "768P", "input_images": 0, "input_video_seconds": 0}}}
	case Grok:
		meta.Schema["seconds"] = number("second", "Billable video duration", "计费视频时长")
		meta.Schema["resolution"] = resolution("480p", "720p", "1080p", "auto")
		meta.Schema["operation"] = jsplugin.UsageFieldSchema{Enum: []string{"generate", "edit", "extend"}, Description: jsplugin.LocalizedText{"en": "Video operation", "zh": "视频操作"}}
		meta.Schema["input_images"] = number("count", "Input images", "输入图片数量")
		meta.Examples = []jsplugin.UsageExample{{Label: "480p · 8s", Facts: map[string]any{"seconds": 8, "resolution": "480p", "operation": "generate", "input_images": 0}}}
	case Omni:
		meta.Schema["input_tokens"] = number("token", "Input tokens", "输入 Token")
		meta.Schema["text_output_tokens"] = number("token", "Text and thought output tokens", "文本及思考输出 Token")
		meta.Schema["video_output_tokens"] = number("token", "Video output tokens", "视频输出 Token")
		meta.Examples = []jsplugin.UsageExample{{Label: "720p · 5s", Facts: map[string]any{"input_tokens": 8192, "text_output_tokens": 8192, "video_output_tokens": 28960}}}
	case Veo:
		meta.Schema["seconds"] = number("second", "Output video duration", "输出视频时长")
		meta.Schema["resolution"] = resolution("720p", "1080p", "4k")
		meta.Examples = []jsplugin.UsageExample{{Label: "720p · 8s", Facts: map[string]any{"seconds": 8, "resolution": "720p"}}}
	case Sora:
		meta.Schema["seconds"] = number("second", "Output video duration", "输出视频时长")
		meta.Schema["size"] = jsplugin.UsageFieldSchema{Enum: []string{"720x1280", "1280x720", "1792x1024", "1024x1792"}, Description: jsplugin.LocalizedText{"en": "Output video dimensions", "zh": "输出视频尺寸"}}
		meta.Examples = []jsplugin.UsageExample{{Label: "720x1280 · 4s", Facts: map[string]any{"seconds": 4, "size": "720x1280"}}}
	default:
		return Metadata{}
	}
	return meta
}

func number(unit, en, zh string) jsplugin.UsageFieldSchema {
	return jsplugin.UsageFieldSchema{Type: "number", Unit: unit, Description: jsplugin.LocalizedText{"en": en, "zh": zh}}
}

func resolution(values ...string) jsplugin.UsageFieldSchema {
	return jsplugin.UsageFieldSchema{Enum: values, Description: jsplugin.LocalizedText{"en": "Output video resolution", "zh": "输出视频分辨率"}}
}
