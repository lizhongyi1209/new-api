package hailuo

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	miniMaxH3Model          = "MiniMax-H3"
	miniMaxH3MinDuration    = 4
	miniMaxH3MaxDuration    = 15
	miniMaxH3MaxPromptRunes = 7000
	miniMaxH3MaxImageCount  = 9
	miniMaxH3MaxBodyBytes   = 64 << 20
)

type miniMaxH3Request struct {
	Model       string                        `json:"model"`
	Content     []taskcommon.VideoContentItem `json:"content"`
	Resolution  string                        `json:"resolution"`
	Duration    *int                          `json:"duration"`
	Ratio       string                        `json:"ratio,omitempty"`
	CallbackURL string                        `json:"callback_url,omitempty"`
}

type miniMaxH3CreateResponse struct {
	TaskID string `json:"task_id"`
}

type miniMaxH3TaskResponse struct {
	Task *miniMaxH3Task `json:"task"`
}

type miniMaxH3Task struct {
	ID         string               `json:"id"`
	Model      string               `json:"model"`
	Status     string               `json:"status"`
	Error      *miniMaxH3TaskError  `json:"error,omitempty"`
	CreatedAt  int64                `json:"created_at,omitempty"`
	UpdatedAt  int64                `json:"updated_at,omitempty"`
	Content    miniMaxH3TaskContent `json:"content,omitempty"`
	Resolution string               `json:"resolution,omitempty"`
	Duration   int                  `json:"duration,omitempty"`
	Usage      *miniMaxH3TaskUsage  `json:"usage,omitempty"`
	Ratio      string               `json:"ratio,omitempty"`
	TaskType   string               `json:"task_type,omitempty"`
	Modality   string               `json:"modality,omitempty"`
}

type miniMaxH3TaskError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type miniMaxH3TaskContent struct {
	URL    string `json:"url,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

type miniMaxH3TaskUsage struct {
	TotalSeconds     int `json:"total_seconds,omitempty"`
	InputSeconds     int `json:"input_seconds,omitempty"`
	OutputSeconds    int `json:"output_seconds,omitempty"`
	InputImageCount  int `json:"input_image_count,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
}

type miniMaxV2ErrorResponse struct {
	Type  string `json:"type,omitempty"`
	Error struct {
		Type     string `json:"type,omitempty"`
		Message  string `json:"message,omitempty"`
		HTTPCode string `json:"http_code,omitempty"`
	} `json:"error,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type miniMaxH3InputSummary struct {
	ImageCount          int
	ReferenceImageCount int
	ReferenceAudioCount int
	HasVideo            bool
}

func isMiniMaxH3Model(modelName string) bool {
	return strings.EqualFold(strings.TrimSpace(modelName), miniMaxH3Model)
}

func isMiniMaxH3Info(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	return isMiniMaxH3Model(info.UpstreamModelName) || isMiniMaxH3Model(info.OriginModelName)
}

func miniMaxH3BaseURL(configuredBaseURL string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(configuredBaseURL), "/")
	if baseURL == "" || baseURL == strings.TrimRight(constant.ChannelBaseURLs[constant.ChannelTypeMiniMax], "/") || baseURL == MiniMaxLegacyBaseURL {
		return MiniMaxV2BaseURL
	}
	return baseURL
}

func validateMiniMaxH3Request(request *miniMaxH3Request) (*miniMaxH3InputSummary, error) {
	if request == nil {
		return nil, fmt.Errorf("request is required")
	}
	if strings.TrimSpace(request.Model) == "" {
		return nil, fmt.Errorf("model field is required")
	}
	if len(request.Content) == 0 {
		return nil, fmt.Errorf("content field is required")
	}
	if request.Duration == nil || *request.Duration < miniMaxH3MinDuration || *request.Duration > miniMaxH3MaxDuration {
		return nil, fmt.Errorf("duration must be between %d and %d", miniMaxH3MinDuration, miniMaxH3MaxDuration)
	}
	if request.Resolution != Resolution768P && request.Resolution != Resolution2K {
		return nil, fmt.Errorf("resolution must be %s or %s", Resolution768P, Resolution2K)
	}
	validRatios := map[string]bool{
		"adaptive": true,
		"21:9":     true,
		"16:9":     true,
		"4:3":      true,
		"1:1":      true,
		"3:4":      true,
		"9:16":     true,
	}
	if request.Ratio != "" && !validRatios[request.Ratio] {
		return nil, fmt.Errorf("invalid ratio")
	}

	summary := &miniMaxH3InputSummary{}
	textCount := 0
	firstFrameCount := 0
	lastFrameCount := 0
	referenceVideoCount := 0
	frameMode := false
	referenceMode := false

	for i, item := range request.Content {
		switch item.Type {
		case "text":
			text := strings.TrimSpace(item.Text)
			if text == "" {
				return nil, fmt.Errorf("content[%d].text is required", i)
			}
			if len([]rune(text)) > miniMaxH3MaxPromptRunes {
				return nil, fmt.Errorf("content[%d].text exceeds %d characters", i, miniMaxH3MaxPromptRunes)
			}
			textCount++
		case "image_url":
			if item.ImageURL == nil || !validMiniMaxMediaURL(item.ImageURL.URL, "image") {
				return nil, fmt.Errorf("content[%d].image_url.url is invalid", i)
			}
			summary.ImageCount++
			role := item.Role
			if role == "" {
				role = "first_frame"
			}
			switch role {
			case "first_frame":
				firstFrameCount++
				frameMode = true
			case "last_frame":
				lastFrameCount++
				frameMode = true
			case "reference_image":
				summary.ReferenceImageCount++
				referenceMode = true
			default:
				return nil, fmt.Errorf("content[%d].role is invalid for image_url", i)
			}
		case "video_url":
			if item.VideoURL == nil || !validMiniMaxMediaURL(item.VideoURL.URL, "video") {
				return nil, fmt.Errorf("content[%d].video_url.url is invalid", i)
			}
			if item.Role != "reference_video" {
				return nil, fmt.Errorf("content[%d].role must be reference_video", i)
			}
			referenceVideoCount++
			summary.HasVideo = true
			referenceMode = true
		case "audio_url":
			if item.AudioURL == nil || !validMiniMaxMediaURL(item.AudioURL.URL, "audio") {
				return nil, fmt.Errorf("content[%d].audio_url.url is invalid", i)
			}
			if item.Role != "reference_audio" {
				return nil, fmt.Errorf("content[%d].role must be reference_audio", i)
			}
			summary.ReferenceAudioCount++
			referenceMode = true
		default:
			return nil, fmt.Errorf("content[%d].type is invalid", i)
		}
	}

	if textCount == 0 {
		return nil, fmt.Errorf("content must include a non-empty text item")
	}
	if firstFrameCount > 1 || lastFrameCount > 1 || (lastFrameCount == 1 && firstFrameCount == 0) {
		return nil, fmt.Errorf("first_frame and last_frame inputs are invalid")
	}
	if summary.ReferenceImageCount > miniMaxH3MaxImageCount {
		return nil, fmt.Errorf("a maximum of %d reference images is supported", miniMaxH3MaxImageCount)
	}
	if referenceVideoCount > 3 {
		return nil, fmt.Errorf("a maximum of 3 reference videos is supported")
	}
	if summary.ReferenceAudioCount > 3 {
		return nil, fmt.Errorf("a maximum of 3 reference audios is supported")
	}
	if frameMode && referenceMode {
		return nil, fmt.Errorf("frame inputs and reference inputs are mutually exclusive")
	}
	if !frameMode && !referenceMode && (request.Ratio == "" || request.Ratio == "adaptive") {
		return nil, fmt.Errorf("ratio is required for text-to-video and cannot be adaptive")
	}
	return summary, nil
}

func validMiniMaxMediaURL(rawURL string, mediaType string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") || strings.HasPrefix(rawURL, "mm_file://") {
		return true
	}
	switch mediaType {
	case "image":
		for _, format := range []string{"jpg", "jpeg", "png", "webp", "heic", "heif"} {
			if strings.HasPrefix(rawURL, "data:image/"+format+";base64,") {
				return true
			}
		}
		return false
	case "video":
		return strings.HasPrefix(rawURL, "data:video/mp4;base64,")
	case "audio":
		return strings.HasPrefix(rawURL, "data:audio/wav;base64,") || strings.HasPrefix(rawURL, "data:audio/mp3;base64,")
	default:
		return false
	}
}

func (a *TaskAdaptor) validateMiniMaxH3Request(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if c.Request.ContentLength > miniMaxH3MaxBodyBytes {
		return service.TaskErrorWrapperLocal(fmt.Errorf("request body must not exceed 64 MB"), "invalid_request", http.StatusBadRequest)
	}
	var nativeRequest miniMaxH3Request
	if err := common.UnmarshalBodyReusable(c, &nativeRequest); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	summary, err := validateMiniMaxH3Request(&nativeRequest)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	metadata := make(map[string]any)
	data, err := common.Marshal(nativeRequest)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if err := common.Unmarshal(data, &metadata); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	request := relaycommon.TaskSubmitReq{
		Prompt:              taskcommon.FirstTextContent(nativeRequest.Content),
		Model:               nativeRequest.Model,
		Size:                nativeRequest.Resolution,
		Duration:            *nativeRequest.Duration,
		AspectRatio:         nativeRequest.Ratio,
		Resolution:          nativeRequest.Resolution,
		EffectiveResolution: nativeRequest.Resolution,
		ImageCount:          summary.ImageCount,
		ReferenceImageCount: summary.ReferenceImageCount,
		ReferenceAudioCount: summary.ReferenceAudioCount,
		HasVideo:            summary.HasVideo,
		Metadata:            metadata,
	}
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	info.Action = constant.TaskActionGenerate
	c.Set("task_request", request)
	return nil
}

func miniMaxV2TaskError(responseBody []byte, statusCode int) *dto.TaskError {
	var response miniMaxV2ErrorResponse
	_ = common.Unmarshal(responseBody, &response)
	message := strings.TrimSpace(response.Error.Message)
	if message == "" {
		message = strings.TrimSpace(string(responseBody))
	}
	if message == "" {
		message = http.StatusText(statusCode)
	}
	code := strings.TrimSpace(response.Error.Type)
	if code == "" {
		code = "minimax_upstream_error"
	}
	err := fmt.Errorf("%s", message)
	if statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError {
		return service.TaskErrorWrapperLocal(err, code, statusCode)
	}
	return service.TaskErrorWrapper(err, code, statusCode)
}

func parseMiniMaxH3TaskResult(responseBody []byte) (*relaycommon.TaskInfo, bool, error) {
	var upstreamError miniMaxV2ErrorResponse
	if err := common.Unmarshal(responseBody, &upstreamError); err == nil && strings.TrimSpace(upstreamError.Error.Message) != "" {
		return nil, true, fmt.Errorf("minimax query error: %s", upstreamError.Error.Message)
	}
	var response miniMaxH3TaskResponse
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return nil, false, err
	}
	if response.Task == nil {
		return nil, false, nil
	}

	task := response.Task
	result := &relaycommon.TaskInfo{Code: 0, TaskID: task.ID}
	switch strings.ToLower(strings.TrimSpace(task.Status)) {
	case "queued":
		result.Status = model.TaskStatusQueued
		result.Progress = taskcommon.ProgressQueued
	case "running":
		result.Status = model.TaskStatusInProgress
		result.Progress = "50%"
	case "succeeded":
		result.Status = model.TaskStatusSuccess
		result.Progress = taskcommon.ProgressComplete
		result.Url = strings.TrimSpace(task.Content.URL)
	case "failed", "cancelled":
		result.Status = model.TaskStatusFailure
		result.Progress = taskcommon.ProgressComplete
		if task.Error != nil {
			result.Code, _ = strconv.Atoi(task.Error.Code)
			result.Reason = strings.TrimSpace(task.Error.Message)
		}
		if result.Reason == "" {
			result.Reason = "task " + strings.ToLower(strings.TrimSpace(task.Status))
		}
	default:
		result.Status = model.TaskStatusInProgress
		result.Progress = taskcommon.ProgressInProgress
	}
	if task.Resolution != "" || task.Duration > 0 || task.Ratio != "" {
		result.Metadata = make(map[string]interface{})
		if task.Resolution != "" {
			result.Metadata["resolution"] = task.Resolution
		}
		if task.Duration > 0 {
			result.Metadata["duration"] = task.Duration
		}
		if task.Ratio != "" {
			result.Metadata["ratio"] = task.Ratio
		}
	}
	if task.Usage != nil {
		if result.Metadata == nil {
			result.Metadata = make(map[string]interface{})
		}
		result.Metadata["usage"] = task.Usage
	}
	return result, true, nil
}
