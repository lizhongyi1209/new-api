package dto

import (
	"bytes"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// GenerateImageRequest 是统一异步生图端点 /async/v1/generateImage 的入参。
// 设计原则：全部顶层传参、扁平、与 provider 无关。控制器根据 Model 分发到具体 provider。
//
// 遵守 CLAUDE.md Rule 6：可选标量用指针，保留显式零值语义。
type GenerateImageRequest struct {
	Model  string `json:"model" binding:"required"`
	Prompt string `json:"prompt" binding:"required"`
	N      *uint  `json:"n,omitempty"`

	Size        string  `json:"size,omitempty"`         // 如 "auto" / "1024x1024" / "1K" / "2K" / "4K"
	AspectRatio string  `json:"aspect_ratio,omitempty"` // 如 "16:9"
	Quality     string  `json:"quality,omitempty"`      // "low" / "medium" / "high" / "auto"
	Background  *string `json:"background,omitempty"`   // gpt-image*: "auto" / "transparent"
	Moderation  *string `json:"moderation,omitempty"`   // gpt-image*: "auto" / "low"

	// Gemini 原生：响应模态，默认 ["TEXT","IMAGE"]。
	ResponseModalities []string `json:"response_modalities,omitempty"`

	// Gemini 原生：generationConfig.mediaResolution。
	MediaResolution string `json:"media_resolution,omitempty"`

	// Gemini 原生：开启 Google Search grounding。默认不传；仅 true 时启用。
	GoogleSearch *bool `json:"google_search,omitempty"`

	// Gemini 3.1 系列原生：thinkingConfig.thinkingLevel，仅支持小写 "minimal" / "high"。
	// 其他模型不应传入 thinking_level。
	ThinkingLevel *string `json:"thinking_level,omitempty"`
	// Gemini 原生：thinkingConfig.includeThoughts。
	IncludeThoughts *bool `json:"include_thoughts,omitempty"`

	OutputFormat *string         `json:"output_format,omitempty"` // "png" / "jpeg" / "webp"
	Mask         *ImageReference `json:"mask,omitempty"`

	// 参考图（img2img），统一使用 images；单张图也传单元素数组。
	// 兼容历史字符串以及 Gemini inlineData/fileData 显式对象。
	Images []GenerateImageInput `json:"images,omitempty"`
}

// GenerateImageInput 是统一生图参考图的兼容输入。
// 每项只能是历史字符串、inlineData 或 fileData 三种形式之一。
type GenerateImageInput struct {
	Value      *string                  `json:"-"`
	InlineData *GenerateImageInlineData `json:"inlineData,omitempty"`
	FileData   *GenerateImageFileData   `json:"fileData,omitempty"`
}

type GenerateImageInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type GenerateImageFileData struct {
	MimeType string `json:"mimeType"`
	FileURI  string `json:"fileUri"`
}

func (input *GenerateImageInput) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("image input must not be empty")
	}

	switch trimmed[0] {
	case '"':
		var value string
		if err := common.Unmarshal(trimmed, &value); err != nil {
			return err
		}
		input.Value = &value
		input.InlineData = nil
		input.FileData = nil
		return nil
	case '{':
		var object struct {
			InlineData *GenerateImageInlineData `json:"inlineData"`
			FileData   *GenerateImageFileData   `json:"fileData"`
		}
		if err := common.Unmarshal(trimmed, &object); err != nil {
			return err
		}
		if (object.InlineData == nil) == (object.FileData == nil) {
			return fmt.Errorf("image input must provide exactly one of inlineData or fileData")
		}
		input.Value = nil
		input.InlineData = object.InlineData
		input.FileData = object.FileData
		return nil
	default:
		return fmt.Errorf("image input must be a string or an inlineData/fileData object")
	}
}

func (input GenerateImageInput) MarshalJSON() ([]byte, error) {
	switch {
	case input.Value != nil:
		return common.Marshal(*input.Value)
	case input.InlineData != nil && input.FileData == nil:
		return common.Marshal(struct {
			InlineData *GenerateImageInlineData `json:"inlineData"`
		}{InlineData: input.InlineData})
	case input.FileData != nil && input.InlineData == nil:
		return common.Marshal(struct {
			FileData *GenerateImageFileData `json:"fileData"`
		}{FileData: input.FileData})
	default:
		return nil, fmt.Errorf("image input must provide exactly one input type")
	}
}

// LegacyString returns the equivalent historical string representation.
// It is used by non-Gemini image paths, whose existing contract remains string-based.
func (input GenerateImageInput) LegacyString() string {
	switch {
	case input.Value != nil:
		return *input.Value
	case input.InlineData != nil:
		return fmt.Sprintf("data:%s;base64,%s", input.InlineData.MimeType, input.InlineData.Data)
	case input.FileData != nil:
		return input.FileData.FileURI
	default:
		return ""
	}
}

// ImageReference 通过已上传文件 ID 或图片 URL 引用输入图片。
// 两个字段必须二选一，不能同时提供。
type ImageReference struct {
	FileID   *string `json:"file_id,omitempty"`
	ImageURL *string `json:"image_url,omitempty"`
}

// GenerateImageData 是单张生成结果。上游给 url 时原样填 Url；
// 上游给 base64 时处理阶段会上传对象存储后填 Url。
type GenerateImageData struct {
	B64Json  string `json:"b64_json,omitempty"`
	Url      string `json:"url,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// GenerateImageResult 是任务成功后存入 task.Data、并由 fetch 端点原样返回的统一结构。
type GenerateImageResult struct {
	Model   string              `json:"model"`
	Created int64               `json:"created"`
	Images  []GenerateImageData `json:"images"`
}
