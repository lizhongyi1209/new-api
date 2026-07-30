package xai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var (
	validAspectRatios = map[string]bool{
		"1:1": true, "16:9": true, "9:16": true, "4:3": true,
		"3:4": true, "3:2": true, "2:3": true,
	}
	validResolutions = map[string]bool{"480p": true, "720p": true, "1080p": true}
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.apiKey = info.ApiKey
	a.baseURL = info.ChannelBaseUrl
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var request videoGenerationRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(request.Model) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model is required"), "missing_model", http.StatusBadRequest)
	}
	action, taskErr := validateVideoRequest(c.Request.URL.Path, request)
	if taskErr != nil {
		return taskErr
	}
	info.Action = action
	c.Set("xai_video_request", request)
	taskRequest := relaycommon.TaskSubmitReq{
		Model:       request.Model,
		Prompt:      request.Prompt,
		AspectRatio: request.AspectRatio,
	}
	if request.Duration != nil {
		taskRequest.Duration = *request.Duration
	}
	c.Set("task_request", taskRequest)
	return nil
}

func validateVideoRequest(path string, request videoGenerationRequest) (string, *dto.TaskError) {
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
		if strings.TrimSpace(request.Prompt) == "" {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest)
		}
		if taskErr := validateMediaInput("video", request.Video); taskErr != nil {
			return "", taskErr
		}
		if request.Duration != nil || request.AspectRatio != "" || request.Resolution != "" {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("duration, aspect_ratio, and resolution are not supported for video editing"), "invalid_request", http.StatusBadRequest)
		}
		if request.Image != nil || len(request.ReferenceImages) > 0 || len(request.ReferenceAudios) > 0 {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("image inputs are not supported for video editing"), "invalid_request", http.StatusBadRequest)
		}
		return constant.TaskActionVideoEdit, nil
	}

	if strings.HasSuffix(path, "/extensions") {
		if strings.TrimSpace(request.Prompt) == "" {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest)
		}
		if taskErr := validateMediaInput("video", request.Video); taskErr != nil {
			return "", taskErr
		}
		if request.Duration != nil && (*request.Duration < 2 || *request.Duration > 10) {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("extension duration must be between 2 and 10"), "invalid_duration", http.StatusBadRequest)
		}
		if request.AspectRatio != "" || request.Resolution != "" {
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
	if request.AspectRatio != "" && !validAspectRatios[request.AspectRatio] {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("invalid aspect_ratio"), "invalid_aspect_ratio", http.StatusBadRequest)
	}
	if request.Resolution != "" && !validResolutions[request.Resolution] {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("invalid resolution"), "invalid_resolution", http.StatusBadRequest)
	}
	if request.Resolution == "1080p" && (!isVideo15Model(request.Model) || request.Image == nil) {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("1080p is only supported by grok-imagine-video-1.5 for image-to-video generation"), "invalid_resolution", http.StatusBadRequest)
	}
	if request.Image != nil && (len(request.ReferenceImages) > 0 || len(request.ReferenceAudios) > 0) {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("image and reference inputs are mutually exclusive"), "invalid_request", http.StatusBadRequest)
	}
	if request.Image != nil {
		if taskErr := validateMediaInput("image", request.Image); taskErr != nil {
			return "", taskErr
		}
		return constant.TaskActionGenerate, nil
	}
	if len(request.ReferenceImages) > 0 || len(request.ReferenceAudios) > 0 {
		if strings.TrimSpace(request.Prompt) == "" {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required for reference-to-video"), "invalid_request", http.StatusBadRequest)
		}
		if len(request.ReferenceImages) > 7 {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("a maximum of 7 reference images is supported"), "invalid_reference_images", http.StatusBadRequest)
		}
		if request.Duration != nil && *request.Duration > 10 {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("reference-to-video duration must not exceed 10"), "invalid_duration", http.StatusBadRequest)
		}
		if isVideo15Model(request.Model) {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("grok-imagine-video-1.5 does not support reference-to-video"), "invalid_model", http.StatusBadRequest)
		}
		for i := range request.ReferenceImages {
			if taskErr := validateMediaInput("reference_images", &request.ReferenceImages[i]); taskErr != nil {
				return "", taskErr
			}
		}
		if len(request.ReferenceAudios) > 3 {
			return "", service.TaskErrorWrapperLocal(fmt.Errorf("a maximum of 3 reference audios is supported"), "invalid_reference_audios", http.StatusBadRequest)
		}
		for _, audio := range request.ReferenceAudios {
			if strings.TrimSpace(audio.URL) == "" {
				return "", service.TaskErrorWrapperLocal(fmt.Errorf("each reference audio requires url"), "invalid_reference_audio", http.StatusBadRequest)
			}
		}
		return constant.TaskActionReferenceGenerate, nil
	}
	if isVideo15Model(request.Model) {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("grok-imagine-video-1.5 only supports image-to-video generation"), "invalid_model", http.StatusBadRequest)
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return "", service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required for text-to-video"), "invalid_request", http.StatusBadRequest)
	}
	return constant.TaskActionTextGenerate, nil
}

func isVideo15Model(modelName string) bool {
	return modelName == "grok-imagine-video-1.5" ||
		modelName == "grok-imagine-video-1.5-preview" ||
		modelName == "grok-imagine-video-1.5-2026-05-30"
}

func validateMediaInput(field string, input *mediaInput) *dto.TaskError {
	if input == nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("%s is required", field), "invalid_"+field, http.StatusBadRequest)
	}
	url := strings.TrimSpace(input.URL)
	if url == "" {
		url = strings.TrimSpace(input.ImageURL)
	}
	if url == "" && strings.TrimSpace(input.FileID) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("%s requires url or file_id", field), "invalid_"+field, http.StatusBadRequest)
	}
	if url != "" && input.FileID != "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("%s cannot contain both url and file_id", field), "invalid_"+field, http.StatusBadRequest)
	}
	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	request, ok := c.Get("xai_video_request")
	if !ok {
		return nil
	}
	duration := request.(videoGenerationRequest).Duration
	requestData := request.(videoGenerationRequest)
	if info.Action == constant.TaskActionVideoExtend && duration == nil {
		return map[string]float64{"seconds": 6}
	}
	if info.Action == constant.TaskActionVideoEdit {
		return nil
	}
	seconds := 8
	if duration != nil {
		seconds = *duration
	}
	ratio := 1.0
	if isVideo15Model(requestData.Model) {
		switch requestData.Resolution {
		case "720p":
			ratio = 1.75
		case "1080p":
			ratio = 3.125
		}
	} else if requestData.Resolution == "720p" {
		ratio = 1.4
	}
	ratios := map[string]float64{"seconds": float64(seconds), "resolution": ratio}
	imageCount := len(requestData.ReferenceImages)
	if requestData.Image != nil {
		imageCount++
	}
	if imageCount > 0 {
		basePrice := 0.05
		imagePrice := 0.002
		if isVideo15Model(requestData.Model) {
			basePrice = 0.08
			imagePrice = 0.01
		}
		outputPrice := basePrice * float64(seconds) * ratio
		ratios["image_input"] = (outputPrice + imagePrice*float64(imageCount)) / outputPrice
	}
	return ratios
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	path := "/v1/videos/generations"
	switch info.Action {
	case constant.TaskActionVideoEdit:
		path = "/v1/videos/edits"
	case constant.TaskActionVideoExtend:
		path = "/v1/videos/extensions"
	}
	return strings.TrimRight(a.baseURL, "/") + path, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, request *http.Request, _ *relaycommon.RelayInfo) error {
	request.Header.Set("Authorization", "Bearer "+a.apiKey)
	request.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	value, ok := c.Get("xai_video_request")
	if !ok {
		return nil, fmt.Errorf("xAI video request not found")
	}
	request := value.(videoGenerationRequest)
	request.Model = info.UpstreamModelName
	body, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, body)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, response *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("%s", body), "fail_to_fetch_task", response.StatusCode)
	}
	var result submitResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if strings.TrimSpace(result.RequestID) == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("request_id is empty"), "invalid_response", http.StatusInternalServerError)
	}
	c.JSON(response.StatusCode, submitResponse{RequestID: info.PublicTaskID})
	return result.RequestID, body, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/videos/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	return client.Do(request)
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var response videoResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	result := &relaycommon.TaskInfo{}
	switch response.Status {
	case "pending":
		result.Status = model.TaskStatusInProgress
	case "done":
		result.Status = model.TaskStatusSuccess
		if response.Video != nil {
			result.Url = response.Video.URL
			result.Metadata = map[string]any{"duration": response.Video.Duration}
		}
	case "failed", "expired":
		result.Status = model.TaskStatusFailure
		result.Reason = response.Status
		if response.Error != nil && response.Error.Message != "" {
			result.Reason = response.Error.Message
		}
	default:
		return nil, fmt.Errorf("unknown xAI video status %q", response.Status)
	}
	return result, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{
		"grok-imagine-video",
		"grok-imagine-video-1.5",
		"grok-imagine-video-1.5-preview",
		"grok-imagine-video-1.5-2026-05-30",
	}
}
func (a *TaskAdaptor) GetChannelName() string { return "xai-video" }
