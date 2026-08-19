package grokvideo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	RequestContextKey         = "grok_video_request"
	DefaultDuration           = 8
	DefaultResolution         = "480p"
	DefaultAspectRatio        = "16:9"
	MaxVideoEditingDuration   = 8.7 // Maximum input video duration for editing (seconds)
	MaxVideoEditingResolution = 720 // Maximum output resolution for video editing (capped at 720p)
)

var (
	validAspectRatios = map[string]bool{
		"1:1": true, "16:9": true, "9:16": true, "4:3": true,
		"3:4": true, "3:2": true, "2:3": true,
	}
	validResolutions = map[string]bool{"480p": true, "720p": true, "1080p": true}
)

type Request struct {
	Model           string          `json:"model"`
	Prompt          *string         `json:"prompt,omitempty"`
	Duration        *int            `json:"duration,omitempty"`
	AspectRatio     *string         `json:"aspect_ratio,omitempty"`
	Resolution      *string         `json:"resolution,omitempty"`
	Image           *MediaInput     `json:"image,omitempty"`
	ReferenceImages []MediaInput    `json:"reference_images,omitempty"`
	ReferenceAudios []AudioInput    `json:"reference_audios,omitempty"`
	Video           *MediaInput     `json:"video,omitempty"`
	StorageOptions  *StorageOptions `json:"storage_options,omitempty"`
	Output          *OutputOptions  `json:"output,omitempty"`
	User            *string         `json:"user,omitempty"`
}

type MediaInput struct {
	URL      *string `json:"url,omitempty"`
	ImageURL *string `json:"image_url,omitempty"`
	FileID   *string `json:"file_id,omitempty"`
}

func (input MediaInput) ResolvedURL() string {
	if input.URL != nil {
		if url := strings.TrimSpace(*input.URL); url != "" {
			return url
		}
	}
	if input.ImageURL != nil {
		return strings.TrimSpace(*input.ImageURL)
	}
	return ""
}

func (input MediaInput) ResolvedFileID() string {
	if input.FileID == nil {
		return ""
	}
	return strings.TrimSpace(*input.FileID)
}

func (request Request) PromptValue() string {
	if request.Prompt == nil {
		return ""
	}
	return *request.Prompt
}

func (request Request) AspectRatioValue() string {
	if request.AspectRatio == nil {
		return ""
	}
	return *request.AspectRatio
}

func (request Request) ResolutionValue() string {
	if request.Resolution == nil {
		return ""
	}
	return *request.Resolution
}

type AudioInput struct {
	URL     *string `json:"url,omitempty"`
	VoiceID *string `json:"voice_id,omitempty"`
}

func (input AudioInput) ResolvedURL() string {
	if input.URL == nil {
		return ""
	}
	return strings.TrimSpace(*input.URL)
}

func (input AudioInput) ResolvedVoiceID() string {
	if input.VoiceID == nil {
		return ""
	}
	return strings.TrimSpace(*input.VoiceID)
}

type OutputOptions struct {
	UploadURL string `json:"upload_url"`
}

type StorageOptions struct {
	Filename     string          `json:"filename"`
	ExpiresAfter *int            `json:"expires_after,omitempty"`
	PublicURL    json.RawMessage `json:"public_url,omitempty"`
}

type publicURLOptions struct {
	ExpiresAfter *int `json:"expires_after,omitempty"`
}

func (request *Request) UnmarshalJSON(data []byte) error {
	type requestAlias Request
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

func ParseAndValidate(c *gin.Context, info *relaycommon.RelayInfo) (Request, *dto.TaskError) {
	var request Request
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return request, service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(request.Model) == "" {
		return request, service.TaskErrorWrapperLocal(fmt.Errorf("model is required"), "missing_model", http.StatusBadRequest)
	}
	action, taskErr := validateRequest(c.Request.URL.Path, request)
	if taskErr != nil {
		return request, taskErr
	}
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	info.Action = action
	c.Set(RequestContextKey, request)
	resolution := request.ResolutionValue()
	taskRequest := relaycommon.TaskSubmitReq{
		Model:               request.Model,
		Prompt:              request.PromptValue(),
		AspectRatio:         request.AspectRatioValue(),
		Resolution:          resolution,
		EffectiveResolution: resolution,
		ImageCount:          len(request.ReferenceImages),
		ReferenceImageCount: len(request.ReferenceImages),
		ReferenceAudioCount: len(request.ReferenceAudios),
		HasVideo:            request.Video != nil,
	}
	if request.Image != nil {
		taskRequest.ImageCount++
	}
	if resolution == "" && action != constant.TaskActionVideoEdit && action != constant.TaskActionVideoExtend {
		taskRequest.EffectiveResolution = DefaultResolution
		taskRequest.ResolutionDefaulted = true
	}
	if request.Duration != nil {
		taskRequest.Duration = *request.Duration
	}
	c.Set("task_request", taskRequest)
	return request, nil
}

func GetRequest(c *gin.Context) (Request, bool) {
	value, ok := c.Get(RequestContextKey)
	if !ok {
		return Request{}, false
	}
	request, ok := value.(Request)
	return request, ok
}

func validateRequest(path string, request Request) (string, *dto.TaskError) {
	if request.Output != nil && strings.TrimSpace(request.Output.UploadURL) == "" {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("output.upload_url is required"), "invalid_output", http.StatusBadRequest)
	}
	if request.StorageOptions != nil {
		if strings.TrimSpace(request.StorageOptions.Filename) == "" {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("storage_options.filename is required"), "invalid_storage_options", http.StatusBadRequest)
		}
		if request.StorageOptions.ExpiresAfter != nil && (*request.StorageOptions.ExpiresAfter < 3600 || *request.StorageOptions.ExpiresAfter > 2592000) {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("storage_options.expires_after must be between 3600 and 2592000"), "invalid_storage_options", http.StatusBadRequest)
		}
		if len(request.StorageOptions.PublicURL) > 0 {
			switch common.GetJsonType(request.StorageOptions.PublicURL) {
			case "boolean":
				var enabled bool
				if err := common.Unmarshal(request.StorageOptions.PublicURL, &enabled); err != nil {
					return "", service.TaskErrorWrapperLocal(fmt.Errorf("storage_options.public_url must be a boolean or object"), "invalid_storage_options", http.StatusBadRequest)
				}
			case "object":
				var publicURL publicURLOptions
				if err := common.Unmarshal(request.StorageOptions.PublicURL, &publicURL); err != nil {
					return "", service.TaskErrorWrapperLocal(fmt.Errorf("invalid storage_options.public_url"), "invalid_storage_options", http.StatusBadRequest)
				}
				if publicURL.ExpiresAfter != nil && (*publicURL.ExpiresAfter < 3600 || *publicURL.ExpiresAfter > 2592000) {
					return "", service.TaskErrorWrapperLocal(fmt.Errorf("storage_options.public_url.expires_after must be between 3600 and 2592000"), "invalid_storage_options", http.StatusBadRequest)
				}
				if publicURL.ExpiresAfter != nil && request.StorageOptions.ExpiresAfter != nil && *publicURL.ExpiresAfter > *request.StorageOptions.ExpiresAfter {
					return "", service.TaskErrorWrapperLocal(fmt.Errorf("public URL expiration cannot exceed file expiration"), "invalid_storage_options", http.StatusBadRequest)
				}
			default:
				return "", service.TaskErrorWrapperLocal(fmt.Errorf("storage_options.public_url must be a boolean or object"), "invalid_storage_options", http.StatusBadRequest)
			}
		}
	}

	if strings.HasSuffix(path, "/edits") {
		if strings.TrimSpace(request.PromptValue()) == "" {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest)
		}
		if taskErr := validateMediaInput("video", request.Video); taskErr != nil {
			return "", taskErr
		}
		// Video editing does not support custom duration, aspect_ratio, or resolution parameters.
		// The output preserves the input's duration and aspect ratio, and matches its resolution
		// capped at 720p (e.g., a 1080p input will be downscaled to 720p output).
		// The upstream enforces a maximum input video duration of 8.7 seconds; videos longer
		// than that will be rejected by the provider.
		if request.Duration != nil || request.AspectRatio != nil || request.Resolution != nil {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("duration, aspect_ratio, and resolution are not supported for video editing"), "invalid_request", http.StatusBadRequest)
		}
		if request.Image != nil || len(request.ReferenceImages) > 0 || len(request.ReferenceAudios) > 0 {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("image inputs are not supported for video editing"), "invalid_request", http.StatusBadRequest)
		}
		return constant.TaskActionVideoEdit, nil
	}

	if strings.HasSuffix(path, "/extensions") {
		if strings.TrimSpace(request.PromptValue()) == "" {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest)
		}
		if taskErr := validateMediaInput("video", request.Video); taskErr != nil {
			return "", taskErr
		}
		// The duration parameter controls the length of the extended portion only, not the total output.
		// For example, if the input video is 10 seconds and duration is set to 5, the returned video
		// will be 15 seconds long (10s original + 5s extension).
		if request.Duration != nil && (*request.Duration < 2 || *request.Duration > 10) {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("extension duration must be between 2 and 10"), "invalid_duration", http.StatusBadRequest)
		}
		// Video extension does not support custom aspect_ratio or resolution parameters.
		// The output preserves the input video's aspect ratio and resolution.
		if request.AspectRatio != nil || request.Resolution != nil {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("aspect_ratio and resolution are not supported for video extension"), "invalid_request", http.StatusBadRequest)
		}
		if request.Image != nil || len(request.ReferenceImages) > 0 || len(request.ReferenceAudios) > 0 {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("image inputs are not supported for video extension"), "invalid_request", http.StatusBadRequest)
		}
		return constant.TaskActionVideoExtend, nil
	}

	if request.Video != nil {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("video is only supported for editing and extension"), "invalid_request", http.StatusBadRequest)
	}
	if request.Duration != nil && (*request.Duration < 1 || *request.Duration > 15) {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("duration must be between 1 and 15"), "invalid_duration", http.StatusBadRequest)
	}
	aspectRatio := request.AspectRatioValue()
	if request.AspectRatio != nil && !validAspectRatios[aspectRatio] {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("invalid aspect_ratio"), "invalid_aspect_ratio", http.StatusBadRequest)
	}
	resolution := request.ResolutionValue()
	if request.Resolution != nil && !validResolutions[resolution] {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("invalid resolution"), "invalid_resolution", http.StatusBadRequest)
	}
	if request.Image != nil && (len(request.ReferenceImages) > 0 || len(request.ReferenceAudios) > 0) {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("image and reference inputs are mutually exclusive"), "invalid_request", http.StatusBadRequest)
	}
	if request.Image != nil {
		if taskErr := validateMediaInput("image", request.Image); taskErr != nil {
			return "", taskErr
		}
		if resolution == "1080p" && !IsVideo15Model(request.Model) {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("1080p image-to-video requires grok-imagine-video-1.5"), "invalid_resolution", http.StatusBadRequest)
		}
		return constant.TaskActionGenerate, nil
	}
	if len(request.ReferenceImages) > 0 || len(request.ReferenceAudios) > 0 {
		if strings.TrimSpace(request.PromptValue()) == "" {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required for reference-to-video"), "invalid_request", http.StatusBadRequest)
		}
		if len(request.ReferenceImages) > 7 {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("a maximum of 7 reference images is supported"), "invalid_reference_images", http.StatusBadRequest)
		}
		maxDuration := 10
		if IsVideo15Model(request.Model) {
			maxDuration = 15
		}
		if request.Duration != nil && *request.Duration > maxDuration {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("reference-to-video duration must not exceed %d", maxDuration), "invalid_duration", http.StatusBadRequest)
		}
		if resolution == "1080p" {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("reference-to-video supports at most 720p"), "invalid_resolution", http.StatusBadRequest)
		}
		for i := range request.ReferenceImages {
			if taskErr := validateMediaInput("reference_images", &request.ReferenceImages[i]); taskErr != nil {
				return "", taskErr
			}
		}
		if len(request.ReferenceAudios) > 3 {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("a maximum of 3 reference audios is supported"), "invalid_reference_audios", http.StatusBadRequest)
		}
		for i := range request.ReferenceAudios {
			if taskErr := validateAudioInput(&request.ReferenceAudios[i]); taskErr != nil {
				return "", taskErr
			}
		}
		return constant.TaskActionReferenceGenerate, nil
	}
	if strings.TrimSpace(request.PromptValue()) == "" {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required for text-to-video"), "invalid_request", http.StatusBadRequest)
	}
	if resolution == "1080p" && !IsVideo15Model(request.Model) {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("1080p text-to-video requires grok-imagine-video-1.5"), "invalid_resolution", http.StatusBadRequest)
	}
	return constant.TaskActionTextGenerate, nil
}

func IsVideo15Model(modelName string) bool {
	return modelName == "grok-imagine-video-1.5" ||
		modelName == "grok-imagine-video-1.5-preview" ||
		modelName == "grok-imagine-video-1.5-2026-05-30"
}

func validateMediaInput(field string, input *MediaInput) *dto.TaskError {
	if input == nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("%s is required", field), "invalid_"+field, http.StatusBadRequest)
	}
	if input.URL != nil && input.ImageURL != nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("%s cannot contain both url and image_url", field), "invalid_"+field, http.StatusBadRequest)
	}
	url := input.ResolvedURL()
	fileID := input.ResolvedFileID()
	if url == "" && fileID == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("%s requires url or file_id", field), "invalid_"+field, http.StatusBadRequest)
	}
	if url != "" && fileID != "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("%s cannot contain both url and file_id", field), "invalid_"+field, http.StatusBadRequest)
	}
	return nil
}

func validateAudioInput(input *AudioInput) *dto.TaskError {
	if input == nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("reference audio is required"), "invalid_reference_audio", http.StatusBadRequest)
	}
	if input.URL != nil && input.VoiceID != nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("reference audio cannot contain both url and voice_id"), "invalid_reference_audio", http.StatusBadRequest)
	}
	if input.ResolvedURL() == "" && input.ResolvedVoiceID() == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("reference audio requires url or voice_id"), "invalid_reference_audio", http.StatusBadRequest)
	}
	return nil
}

func EstimateBilling(request Request, action string) map[string]float64 {
	if action == constant.TaskActionVideoEdit {
		return nil
	}
	if action == constant.TaskActionVideoExtend && request.Duration == nil {
		return map[string]float64{"seconds": 6}
	}
	seconds := DefaultDuration
	if request.Duration != nil {
		seconds = *request.Duration
	}
	ratio := 1.0
	resolution := request.ResolutionValue()
	if IsVideo15Model(request.Model) {
		switch resolution {
		case "720p":
			ratio = 1.75
		case "1080p":
			ratio = 3.125
		}
	} else if resolution == "720p" {
		ratio = 1.4
	}
	ratios := map[string]float64{"seconds": float64(seconds), "resolution": ratio}
	imageCount := len(request.ReferenceImages)
	if request.Image != nil {
		imageCount++
	}
	if imageCount == 0 {
		return ratios
	}
	basePrice := 0.05
	imagePrice := 0.002
	if IsVideo15Model(request.Model) {
		basePrice = 0.08
		imagePrice = 0.01
	}
	outputPrice := basePrice * float64(seconds) * ratio
	ratios["image_input"] = (outputPrice + imagePrice*float64(imageCount)) / outputPrice
	return ratios
}
