package gemini

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	GeminiOmniFlashPreviewModel = "gemini-omni-flash-preview"
	omniMinDurationSeconds      = 3
	omniMaxDurationSeconds      = 10
	omniVideoTokensPerSecond    = 5792
	omniEstimatedThoughtTokens  = 8192
	omniEstimatedInputTokens    = 8192
)

const (
	omniTaskTextToVideo      = "text_to_video"
	omniTaskImageToVideo     = "image_to_video"
	omniTaskReferenceToVideo = "reference_to_video"
	omniTaskEdit             = "edit"
)

type omniContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	URI      string `json:"uri,omitempty"`
}

type omniInput struct {
	Type    string        `json:"type"`
	Content []omniContent `json:"content"`
}

type omniVideoConfig struct {
	Task string `json:"task"`
}

type omniGenerationConfig struct {
	ThinkingLevel     string          `json:"thinking_level,omitempty"`
	ThinkingSummaries string          `json:"thinking_summaries,omitempty"`
	VideoConfig       omniVideoConfig `json:"video_config"`
}

type omniResponseFormat struct {
	Type        string  `json:"type"`
	Delivery    string  `json:"delivery"`
	AspectRatio string  `json:"aspect_ratio,omitempty"`
	Duration    *string `json:"duration,omitempty"`
}

type omniRequest struct {
	Model            string               `json:"model"`
	Background       bool                 `json:"background"`
	Input            []omniInput          `json:"input"`
	GenerationConfig omniGenerationConfig `json:"generation_config"`
	ResponseFormat   omniResponseFormat   `json:"response_format"`
}

type omniModalityTokens struct {
	Modality string      `json:"modality"`
	Tokens   json.Number `json:"tokens"`
}

type omniUsage struct {
	TotalTokens            json.Number          `json:"total_tokens"`
	TotalInputTokens       json.Number          `json:"total_input_tokens"`
	InputTokensByModality  []omniModalityTokens `json:"input_tokens_by_modality"`
	TotalOutputTokens      json.Number          `json:"total_output_tokens"`
	OutputTokensByModality []omniModalityTokens `json:"output_tokens_by_modality"`
	TotalToolUseTokens     json.Number          `json:"total_tool_use_tokens"`
	TotalThoughtTokens     json.Number          `json:"total_thought_tokens"`
}

type omniStep struct {
	Type    string        `json:"type"`
	Content []omniContent `json:"content"`
}

type omniInteractionResponse struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Model  string      `json:"model"`
	Usage  omniUsage   `json:"usage"`
	Steps  []omniStep  `json:"steps"`
	Error  interface{} `json:"error,omitempty"`
}

func isGeminiOmniModel(modelName string) bool {
	return strings.EqualFold(strings.TrimSpace(modelName), GeminiOmniFlashPreviewModel)
}

func resolveOmniTask(req relaycommon.TaskSubmitReq) string {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	switch mode {
	case omniTaskTextToVideo, omniTaskImageToVideo, omniTaskReferenceToVideo, omniTaskEdit:
		return mode
	}
	if strings.TrimSpace(req.VideoUrl) != "" {
		return omniTaskEdit
	}
	if len(req.Images) > 0 || len(req.ImageList) > 0 || strings.TrimSpace(req.Image) != "" || strings.TrimSpace(req.ImageUrl) != "" || strings.TrimSpace(req.InputReference) != "" {
		return omniTaskImageToVideo
	}
	return omniTaskTextToVideo
}

func omniAction(task string) string {
	switch task {
	case omniTaskImageToVideo:
		return constant.TaskActionGenerate
	case omniTaskReferenceToVideo:
		return constant.TaskActionReferenceGenerate
	case omniTaskEdit:
		return constant.TaskActionVideoEdit
	default:
		return constant.TaskActionTextGenerate
	}
}

func omniTaskError(message string, code string) *dto.TaskError {
	err := fmt.Errorf("%s", message)
	return service.TaskErrorWrapperLocal(err, code, http.StatusBadRequest)
}

func validateOmniRequest(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		var rawFields map[string]json.RawMessage
		if err := common.UnmarshalBodyReusable(c, &rawFields); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		var duration int
		var durationSet bool
		for _, field := range []string{"duration", "seconds"} {
			rawDuration, exists := rawFields[field]
			if !exists || strings.TrimSpace(string(rawDuration)) == "null" {
				continue
			}
			parsedDuration, err := parseOmniDuration(rawDuration)
			if err != nil {
				return omniTaskError(field+" must be an integer between 3 and 10", "invalid_duration")
			}
			if durationSet && parsedDuration != duration {
				return omniTaskError("duration and seconds must match when both are provided", "invalid_duration")
			}
			duration = parsedDuration
			durationSet = true
		}
		if durationSet && (duration < omniMinDurationSeconds || duration > omniMaxDurationSeconds) {
			return omniTaskError("duration must be between 3 and 10 seconds", "invalid_duration")
		}
	} else if strings.HasPrefix(contentType, "multipart/form-data") {
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_multipart_form", http.StatusBadRequest)
		}
		defer form.RemoveAll()
		var duration int
		var durationSet bool
		for _, field := range []string{"duration", "seconds"} {
			values := form.Value[field]
			if len(values) == 0 {
				continue
			}
			if len(values) != 1 {
				return omniTaskError(field+" must be provided at most once", "invalid_duration")
			}
			parsedDuration, parseErr := strconv.Atoi(values[0])
			if parseErr != nil || strconv.Itoa(parsedDuration) != values[0] {
				return omniTaskError(field+" must be an integer between 3 and 10", "invalid_duration")
			}
			if durationSet && parsedDuration != duration {
				return omniTaskError("duration and seconds must match when both are provided", "invalid_duration")
			}
			duration = parsedDuration
			durationSet = true
		}
		if durationSet && (duration < omniMinDurationSeconds || duration > omniMaxDurationSeconds) {
			return omniTaskError("duration must be between 3 and 10 seconds", "invalid_duration")
		}
	}

	var probe relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &probe); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	requestedMode := strings.ToLower(strings.TrimSpace(probe.Mode))
	if requestedMode != "" && requestedMode != omniTaskTextToVideo && requestedMode != omniTaskImageToVideo && requestedMode != omniTaskReferenceToVideo && requestedMode != omniTaskEdit {
		return omniTaskError("mode must be text_to_video, image_to_video, reference_to_video, or edit", "invalid_mode")
	}
	task := resolveOmniTask(probe)
	var taskErr *dto.TaskError
	if task == omniTaskTextToVideo {
		taskErr = relaycommon.ValidateBasicTaskRequest(c, info, omniAction(task))
	} else {
		taskErr = relaycommon.ValidateTaskRequestWithOptionalPrompt(c, info, omniAction(task))
	}
	if taskErr != nil {
		return taskErr
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	req.Mode = task
	if req.Duration == 0 && req.Seconds != "" {
		req.Duration, _ = strconv.Atoi(req.Seconds)
	}

	if req.Duration != 0 && (req.Duration < omniMinDurationSeconds || req.Duration > omniMaxDurationSeconds) {
		return omniTaskError("duration must be between 3 and 10 seconds", "invalid_duration")
	}
	if req.AspectRatio != "" && req.AspectRatio != "16:9" && req.AspectRatio != "9:16" {
		return omniTaskError("aspect_ratio must be 16:9 or 9:16", "invalid_aspect_ratio")
	}
	if task == omniTaskEdit && (req.Duration != 0 || req.AspectRatio != "" || req.Size != "") {
		return omniTaskError("duration, aspect_ratio, and size are not supported for edit", "unsupported_parameter")
	}
	if value, exists := req.Metadata["thinking_level"]; exists {
		level, ok := value.(string)
		if !ok || (level != "low" && level != "high") {
			return omniTaskError("metadata.thinking_level must be low or high", "invalid_thinking_level")
		}
	}
	if value, exists := req.Metadata["thinking_summaries"]; exists {
		summaries, ok := value.(string)
		if !ok || (summaries != "auto" && summaries != "none") {
			return omniTaskError("metadata.thinking_summaries must be auto or none", "invalid_thinking_summaries")
		}
	}

	images := req.Images
	if len(images) == 0 && strings.TrimSpace(req.Image) != "" {
		images = []string{req.Image}
	}
	if len(images) == 0 && strings.TrimSpace(req.ImageUrl) != "" {
		images = []string{req.ImageUrl}
	}
	if len(images) == 0 && strings.TrimSpace(req.InputReference) != "" {
		images = []string{req.InputReference}
	}
	if len(images) == 0 && len(req.ImageList) > 0 {
		images = make([]string, 0, len(req.ImageList))
		for _, image := range req.ImageList {
			if strings.TrimSpace(image.ImageURL) != "" {
				images = append(images, image.ImageURL)
			}
		}
	}

	multipartReferenceCount := multipartInputReferenceCount(c)
	if multipartReferenceCount > 1 {
		return omniTaskError("multipart input_reference supports exactly one image; use images for multiple references", "invalid_image")
	}
	if multipartReferenceCount > 0 && len(images) > 0 {
		return omniTaskError("provide images or multipart input_reference, not both", "invalid_image")
	}
	if multipartReferenceCount > 0 {
		image := ExtractMultipartImage(c, info)
		if image == nil || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(image.MimeType)), "image/") {
			return omniTaskError("multipart input_reference must be a valid image", "invalid_image")
		}
	}

	switch task {
	case omniTaskTextToVideo:
		if len(images) > 0 || multipartReferenceCount > 0 || strings.TrimSpace(req.VideoUrl) != "" {
			return omniTaskError("text_to_video only accepts a prompt", "invalid_mode_input")
		}
	case omniTaskImageToVideo:
		if strings.TrimSpace(req.VideoUrl) != "" {
			return omniTaskError("video_url is only supported for edit", "invalid_mode_input")
		}
		if len(images) == 0 && multipartReferenceCount == 0 {
			return omniTaskError("an image is required for image_to_video", "invalid_image")
		}
		if len(images) > 1 {
			return omniTaskError("image_to_video accepts exactly one image", "invalid_image")
		}
		if len(images) > 0 {
			if _, err := parseOmniMedia(images[0], "image/"); err != nil {
				return omniTaskError(err.Error(), "invalid_image")
			}
		}
		req.ImageCount = 1
	case omniTaskReferenceToVideo:
		if strings.TrimSpace(req.VideoUrl) != "" {
			return omniTaskError("video_url is only supported for edit", "invalid_mode_input")
		}
		if len(images) == 0 && multipartReferenceCount == 0 {
			return omniTaskError("at least one image is required for reference_to_video", "invalid_image")
		}
		for _, image := range images {
			if _, err := parseOmniMedia(image, "image/"); err != nil {
				return omniTaskError(err.Error(), "invalid_image")
			}
		}
		req.ReferenceImageCount = len(images)
		if req.ReferenceImageCount == 0 {
			req.ReferenceImageCount = multipartReferenceCount
		}
		req.ImageCount = req.ReferenceImageCount
	case omniTaskEdit:
		if len(images) > 0 || multipartReferenceCount > 0 {
			return omniTaskError("edit accepts a video and optional prompt, not images", "invalid_mode_input")
		}
		if strings.TrimSpace(req.VideoUrl) == "" {
			return omniTaskError("video_url is required for edit", "invalid_video")
		}
		if _, err := parseOmniMedia(req.VideoUrl, "video/"); err != nil {
			return omniTaskError(err.Error(), "invalid_video")
		}
		req.HasVideo = true
	}

	req.Images = images
	info.Action = omniAction(task)
	c.Set("task_request", req)
	return nil
}

func multipartInputReferenceCount(c *gin.Context) int {
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		return 0
	}
	form, err := c.MultipartForm()
	if err != nil {
		return 0
	}
	return len(form.File["input_reference"])
}

func parseOmniDuration(raw json.RawMessage) (int, error) {
	var duration int
	if err := common.Unmarshal(raw, &duration); err == nil {
		return duration, nil
	}

	var durationText string
	if err := common.Unmarshal(raw, &durationText); err != nil {
		return 0, err
	}
	duration, err := strconv.Atoi(durationText)
	if err != nil || strconv.Itoa(duration) != durationText {
		return 0, fmt.Errorf("invalid integer duration")
	}
	return duration, nil
}

func parseOmniMedia(value string, requiredMIMEPrefix string) (*omniContent, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("media data is empty")
	}

	mimeType := ""
	data := value
	if strings.HasPrefix(value, "data:") {
		comma := strings.Index(value, ",")
		if comma < 0 {
			return nil, fmt.Errorf("invalid data URI")
		}
		meta := strings.TrimPrefix(value[:comma], "data:")
		parts := strings.Split(meta, ";")
		mimeType = parts[0]
		if !strings.EqualFold(parts[len(parts)-1], "base64") {
			return nil, fmt.Errorf("media data URI must use base64 encoding")
		}
		data = value[comma+1:]
	} else if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return nil, fmt.Errorf("remote media URLs are not supported; use a base64 data URI")
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("media data must be valid base64")
	}
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(decoded)
	}
	if !strings.HasPrefix(strings.ToLower(mimeType), requiredMIMEPrefix) {
		return nil, fmt.Errorf("media MIME type must start with %s", requiredMIMEPrefix)
	}

	mediaType := strings.TrimSuffix(requiredMIMEPrefix, "/")
	return &omniContent{Type: mediaType, Data: data, MIMEType: mimeType}, nil
}

func buildOmniRequest(c *gin.Context, info *relaycommon.RelayInfo) (omniRequest, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return omniRequest{}, err
	}
	if req.Duration == 0 && req.Seconds != "" {
		req.Duration, _ = strconv.Atoi(req.Seconds)
	}
	task := resolveOmniTask(req)
	contents := make([]omniContent, 0, len(req.Images)+1)

	if task == omniTaskImageToVideo || task == omniTaskReferenceToVideo {
		if image := ExtractMultipartImage(c, info); image != nil {
			contents = append(contents, omniContent{Type: "image", Data: image.BytesBase64Encoded, MIMEType: image.MimeType})
		} else {
			images := req.Images
			if task == omniTaskImageToVideo && len(images) > 1 {
				images = images[:1]
			}
			for _, imageValue := range images {
				image, parseErr := parseOmniMedia(imageValue, "image/")
				if parseErr != nil {
					return omniRequest{}, parseErr
				}
				contents = append(contents, *image)
			}
		}
	}
	if task == omniTaskEdit {
		video, parseErr := parseOmniMedia(req.VideoUrl, "video/")
		if parseErr != nil {
			return omniRequest{}, parseErr
		}
		contents = append(contents, *video)
	}
	if strings.TrimSpace(req.Prompt) != "" {
		contents = append(contents, omniContent{Type: "text", Text: req.Prompt})
	}

	thinkingLevel := "low"
	thinkingSummaries := "auto"
	if value, ok := req.Metadata["thinking_level"].(string); ok && strings.TrimSpace(value) != "" {
		thinkingLevel = value
	}
	if value, ok := req.Metadata["thinking_summaries"].(string); ok && strings.TrimSpace(value) != "" {
		thinkingSummaries = value
	}
	aspectRatio := req.AspectRatio
	if aspectRatio == "" && req.Size != "" {
		aspectRatio = SizeToVeoAspectRatio(req.Size)
	}
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}

	responseFormat := omniResponseFormat{
		Type:     "video",
		Delivery: "uri",
	}
	if task != omniTaskEdit {
		responseFormat.AspectRatio = aspectRatio
		if req.Duration > 0 {
			duration := strconv.Itoa(req.Duration) + "s"
			responseFormat.Duration = &duration
		}
	}

	return omniRequest{
		Model:      info.UpstreamModelName,
		Background: true,
		Input: []omniInput{{
			Type:    "user_input",
			Content: contents,
		}},
		GenerationConfig: omniGenerationConfig{
			ThinkingLevel:     thinkingLevel,
			ThinkingSummaries: thinkingSummaries,
			VideoConfig:       omniVideoConfig{Task: task},
		},
		ResponseFormat: responseFormat,
	}, nil
}

func omniEndpoint(baseURL string, modelName string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/" + modelName
	}
	return baseURL + "/v1/" + modelName
}

func omniQueryEndpoint(baseURL string, modelName string, taskID string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(baseURL, "/v1") {
		return fmt.Sprintf("%s/query/%s/%s", baseURL, modelName, taskID)
	}
	return fmt.Sprintf("%s/v1/query/%s/%s", baseURL, modelName, taskID)
}

func parseOmniTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var response omniInteractionResponse
	if err := common.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("unmarshal Gemini Omni response failed: %w", err)
	}

	inputTokens, err := parseOmniTokenCount(response.Usage.TotalInputTokens)
	if err != nil {
		return nil, fmt.Errorf("invalid total_input_tokens: %w", err)
	}
	outputTokens, err := parseOmniTokenCount(response.Usage.TotalOutputTokens)
	if err != nil {
		return nil, fmt.Errorf("invalid total_output_tokens: %w", err)
	}
	thoughtTokens, err := parseOmniTokenCount(response.Usage.TotalThoughtTokens)
	if err != nil {
		return nil, fmt.Errorf("invalid total_thought_tokens: %w", err)
	}
	totalTokens, err := parseOmniTokenCount(response.Usage.TotalTokens)
	if err != nil {
		return nil, fmt.Errorf("invalid total_tokens: %w", err)
	}
	completionTokens := boundedOmniTokenSum(outputTokens, thoughtTokens)
	info := &relaycommon.TaskInfo{
		TaskID:           response.ID,
		PromptTokens:     inputTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		ThoughtTokens:    thoughtTokens,
	}
	info.InputTokensByModality, err = modalityTokenMap(response.Usage.InputTokensByModality)
	if err != nil {
		return nil, fmt.Errorf("invalid input_tokens_by_modality: %w", err)
	}
	info.OutputTokensByModality, err = modalityTokenMap(response.Usage.OutputTokensByModality)
	if err != nil {
		return nil, fmt.Errorf("invalid output_tokens_by_modality: %w", err)
	}
	if info.PromptTokens == 0 {
		for _, tokens := range info.InputTokensByModality {
			info.PromptTokens = boundedOmniTokenSum(info.PromptTokens, tokens)
		}
	}
	if outputTokens == 0 {
		for _, tokens := range info.OutputTokensByModality {
			info.CompletionTokens = boundedOmniTokenSum(info.CompletionTokens, tokens)
		}
	}
	if info.TotalTokens == 0 {
		info.TotalTokens = boundedOmniTokenSum(info.PromptTokens, info.CompletionTokens)
	}

	status := strings.ToLower(strings.TrimSpace(response.Status))
	if response.Error != nil {
		info.Status = model.TaskStatusFailure
		info.Progress = "100%"
		info.Reason = omniErrorMessage(response.Error)
		return info, nil
	}

	switch status {
	case "completed", "succeeded", "success", "done":
		info.Status = model.TaskStatusSuccess
		info.Progress = "100%"
	case "failed", "cancelled", "canceled", "expired":
		info.Status = model.TaskStatusFailure
		info.Progress = "100%"
		info.Reason = omniErrorMessage(response.Error)
	case "queued", "pending", "submitted":
		info.Status = model.TaskStatusSubmitted
		info.Progress = "10%"
	default:
		info.Status = model.TaskStatusInProgress
		info.Progress = "50%"
	}

	for _, step := range response.Steps {
		for _, content := range step.Content {
			if content.Type == "video" && strings.TrimSpace(content.URI) != "" {
				info.Url = content.URI
				info.RemoteUrl = content.URI
				return info, nil
			}
		}
	}
	return info, nil
}

func modalityTokenMap(items []omniModalityTokens) (map[string]int, error) {
	if len(items) == 0 {
		return nil, nil
	}
	result := make(map[string]int, len(items))
	for _, item := range items {
		tokens, err := parseOmniTokenCount(item.Tokens)
		if err != nil {
			return nil, fmt.Errorf("%s tokens: %w", item.Modality, err)
		}
		if tokens <= 0 || strings.TrimSpace(item.Modality) == "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(item.Modality))
		result[key] = boundedOmniTokenSum(result[key], tokens)
	}
	return result, nil
}

func parseOmniTokenCount(value json.Number) (int, error) {
	text := strings.TrimSpace(string(value))
	if text == "" {
		return 0, nil
	}
	parsed := new(big.Int)
	if _, ok := parsed.SetString(text, 10); !ok {
		return 0, fmt.Errorf("must be a base-10 integer")
	}
	if parsed.Sign() <= 0 {
		return 0, nil
	}
	if parsed.Cmp(big.NewInt(math.MaxInt32)) > 0 {
		return math.MaxInt32, nil
	}
	return int(parsed.Int64()), nil
}

func boundedOmniTokens(tokens int) int {
	if tokens <= 0 {
		return 0
	}
	return min(tokens, math.MaxInt32)
}

func boundedOmniTokenSum(left int, right int) int {
	total := int64(boundedOmniTokens(left)) + int64(boundedOmniTokens(right))
	if total > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(total)
}

func omniErrorMessage(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]interface{}:
		if message, ok := typed["message"].(string); ok {
			return message
		}
	}
	return "Gemini Omni video generation failed"
}
