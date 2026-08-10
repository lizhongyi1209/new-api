package dto

// GenerateImageRequest 是统一异步生图端点 /async/v1/generateImage 的入参。
// 设计原则：全部顶层传参、扁平、与 provider 无关。控制器根据 Model 分发到具体 provider。
//
// 遵守 CLAUDE.md Rule 6：可选标量用指针，保留显式零值语义。
type GenerateImageRequest struct {
	Model  string `json:"model" binding:"required"`
	Prompt string `json:"prompt" binding:"required"`
	N      *uint  `json:"n,omitempty"`

	Size        string `json:"size,omitempty"`         // 如 "auto" / "1024x1024" / "1K" / "2K" / "4K"
	AspectRatio string `json:"aspect_ratio,omitempty"` // 如 "16:9"
	Quality     string `json:"quality,omitempty"`      // "low" / "medium" / "high" / "auto"

	// Gemini 原生：响应模态，默认 ["TEXT","IMAGE"]。
	ResponseModalities []string `json:"response_modalities,omitempty"`

	// Gemini 原生：generationConfig.mediaResolution。
	MediaResolution string `json:"media_resolution,omitempty"`

	// Gemini 原生：开启 Google Search grounding。默认不传；仅 true 时启用。
	GoogleSearch *bool `json:"google_search,omitempty"`

	// Gemini 3.1 系列原生：thinkingConfig.thinkingLevel，仅支持小写 "minimal" / "high"。
	// 其他模型不应传入 thinking_level。
	ThinkingLevel   *string `json:"thinking_level,omitempty"`
	// Gemini 原生：thinkingConfig.includeThoughts。
	IncludeThoughts *bool   `json:"include_thoughts,omitempty"`

	OutputFormat *string         `json:"output_format,omitempty"` // "png" / "jpeg" / "webp"
	Mask         *ImageReference `json:"mask,omitempty"`

	// 参考图（img2img），统一使用 images；单张图也传单元素数组。
	Images []string `json:"images,omitempty"`
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
