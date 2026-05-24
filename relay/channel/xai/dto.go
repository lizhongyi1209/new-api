package xai

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/dto"
)

// ChatCompletionResponse represents the response from XAI chat completion API
type ChatCompletionResponse struct {
	Id                string                         `json:"id"`
	Object            string                         `json:"object"`
	Created           int64                          `json:"created"`
	Model             string                         `json:"model"`
	Choices           []dto.OpenAITextResponseChoice `json:"choices"`
	Usage             *dto.Usage                     `json:"usage"`
	SystemFingerprint string                         `json:"system_fingerprint"`
}

// quality, size or style are not supported by xAI API at the moment.
type ImageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt" binding:"required"`
	N              int    `json:"n,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
}

type ImageEditRequest struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt"`
	N              int             `json:"n,omitempty"`
	ResponseFormat string          `json:"response_format,omitempty"`
	Image          json.RawMessage `json:"image"`
}

type ImageObject struct {
	Url  string `json:"url"`
	Type string `json:"type"`
}
