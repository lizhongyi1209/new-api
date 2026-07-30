package xai

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
)

type videoGenerationRequest struct {
	Model           string          `json:"model"`
	Prompt          string          `json:"prompt"`
	Duration        *int            `json:"duration,omitempty"`
	AspectRatio     string          `json:"aspect_ratio,omitempty"`
	Resolution      string          `json:"resolution,omitempty"`
	Image           *imageInput     `json:"image,omitempty"`
	ReferenceImages []imageInput    `json:"reference_images,omitempty"`
	ReferenceAudios []audioInput    `json:"reference_audios,omitempty"`
	Video           *mediaInput     `json:"video,omitempty"`
	StorageOptions  *storageOptions `json:"storage_options,omitempty"`
	Output          *outputOptions  `json:"output,omitempty"`
	User            *string         `json:"user,omitempty"`
}

type mediaInput struct {
	URL      string `json:"url,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
}

type imageInput = mediaInput

type audioInput struct {
	URL string `json:"url"`
}

type outputOptions struct {
	UploadURL string `json:"upload_url"`
}

type storageOptions struct {
	Filename     string          `json:"filename"`
	ExpiresAfter *int            `json:"expires_after,omitempty"`
	PublicURL    json.RawMessage `json:"public_url,omitempty"`
}

type publicURLOptions struct {
	ExpiresAfter *int `json:"expires_after,omitempty"`
}

func (request *videoGenerationRequest) UnmarshalJSON(data []byte) error {
	type requestAlias videoGenerationRequest
	var fields struct {
		*requestAlias
		Duration json.RawMessage `json:"duration,omitempty"`
		Seconds  json.RawMessage `json:"seconds,omitempty"`
	}
	fields.requestAlias = (*requestAlias)(request)
	if err := common.Unmarshal(data, &fields); err != nil {
		return err
	}
	rawDuration := fields.Duration
	if len(rawDuration) == 0 {
		rawDuration = fields.Seconds
	}
	if len(rawDuration) == 0 || common.GetJsonType(rawDuration) == "null" {
		return nil
	}
	var duration int
	if err := common.Unmarshal(rawDuration, &duration); err == nil {
		request.Duration = &duration
		return nil
	}
	var durationString string
	if err := common.Unmarshal(rawDuration, &durationString); err != nil {
		return fmt.Errorf("duration must be an integer or integer string")
	}
	duration, err := strconv.Atoi(durationString)
	if err != nil {
		return fmt.Errorf("duration must be an integer or integer string")
	}
	request.Duration = &duration
	return nil
}

type submitResponse struct {
	RequestID string `json:"request_id"`
}

type videoResponse struct {
	Status string       `json:"status"`
	Video  *videoOutput `json:"video,omitempty"`
	Model  string       `json:"model,omitempty"`
	Error  *videoError  `json:"error,omitempty"`
}

type videoOutput struct {
	URL               string  `json:"url"`
	Duration          float64 `json:"duration"`
	RespectModeration bool    `json:"respect_moderation"`
}

type videoError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
