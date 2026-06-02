package dto

import "encoding/json"

type AsyncImageRequest struct {
	Model              string          `json:"model" binding:"required"`
	Prompt             string          `json:"prompt" binding:"required"`
	N                  *uint           `json:"n,omitempty"`
	Size               string          `json:"size,omitempty"`
	Quality            string          `json:"quality,omitempty"`
	ResponseFormat     string          `json:"response_format,omitempty"`
	OutputFormat       *string         `json:"output_format,omitempty"`
	Style              json.RawMessage `json:"style,omitempty"`
	User               json.RawMessage `json:"user,omitempty"`
	ImageCompression   string          `json:"image_compression,omitempty"`
	AspectRatio        string          `json:"aspect_ratio,omitempty"`
	ResponseModalities []string        `json:"response_modalities,omitempty"`
	Image              json.RawMessage `json:"image,omitempty"`
	Images             []string        `json:"images,omitempty"`
	Mask               *ImageReference `json:"mask,omitempty"`
}

const ImageCompressionWebP = "webp"
const ImageCompressionJPG = "jpg"
const ImageCompressionOrigin = "origin"

type AsyncTaskResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

type AsyncTaskFetchResponse struct {
	TaskID   string          `json:"task_id"`
	Status   string          `json:"status"`
	Progress string          `json:"progress,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type AsyncImageResponseData struct {
	URL string `json:"url"`
}

// Deprecated: use AsyncTaskResponse
type AsyncImageResponse = AsyncTaskResponse

// Deprecated: use AsyncTaskFetchResponse
type AsyncImageFetchResponse = AsyncTaskFetchResponse
