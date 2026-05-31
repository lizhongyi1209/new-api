package dto

import "encoding/json"

// GenerateImageRequest 是统一异步生图端点 /async/v1/model/generateImage 的入参。
// 设计原则：全部顶层传参、扁平、与 provider 无关。控制器根据 Model 分发到具体 provider。
//
// 遵守 CLAUDE.md Rule 6：可选标量用指针，保留显式零值语义。
type GenerateImageRequest struct {
	Model  string `json:"model" binding:"required"`
	Prompt string `json:"prompt" binding:"required"`
	N      *uint  `json:"n,omitempty"`

	Size        string `json:"size,omitempty"`         // 如 "1024x1024" / "1K" / "2K" / "4K"
	AspectRatio string `json:"aspect_ratio,omitempty"` // 如 "16:9"
	Quality     string `json:"quality,omitempty"`      // "standard" / "hd"

	// 客户端可控的图片压缩策略，透传给处理函数（webp/jpg/origin）。
	ImageCompression string `json:"image_compression,omitempty"`

	// Gemini 原生：响应模态，默认 ["TEXT","IMAGE"]。
	ResponseModalities []string `json:"response_modalities,omitempty"`

	// 参考图（img2img）。Image 为单图，可为 url / base64 / data-uri 字符串；
	// Images 为多图数组。两者可同时存在。
	Image  json.RawMessage `json:"image,omitempty"`
	Images []string        `json:"images,omitempty"`
}

// GenerateImageData 是单张生成结果。原样返回：上游给 base64 则填 B64Json，
// 上游给 url 则填 Url（不经 R2，url 可能有时效性）。
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
