package serviceinference

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/grokvideo"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type MediaURL = taskcommon.MediaURL
type ContentItem = taskcommon.VideoContentItem

type requestPayload struct {
	Model           string        `json:"model"`
	Content         []ContentItem `json:"content,omitempty"`
	Duration        *int          `json:"duration,omitempty"`
	Resolution      string        `json:"resolution,omitempty"`
	Ratio           string        `json:"ratio,omitempty"`
	CallbackURL     string        `json:"callback_url,omitempty"`
	AIGCWatermark   *bool         `json:"aigc_watermark,omitempty"`
	GenerateAudio   *bool         `json:"generate_audio,omitempty"`
	Watermark       *bool         `json:"watermark,omitempty"`
	ReturnLastFrame *bool         `json:"return_last_frame,omitempty"`
}

type taskResponse struct {
	Task videoTask `json:"task"`
}

type videoTask struct {
	ID              string           `json:"id"`
	Status          string           `json:"status"`
	Model           string           `json:"model"`
	DurationSeconds int              `json:"duration_seconds"`
	Outputs         []string         `json:"outputs"`
	Error           any              `json:"error"`
	CreatedAt       string           `json:"created_at"`
	CompletedAt     string           `json:"completed_at"`
	Usage           *taskUsage       `json:"usage,omitempty"`
	Metadata        *taskMetadata    `json:"metadata,omitempty"`
	LastFrameURL    string           `json:"last_frame_url,omitempty"`
	Prep            map[string]any   `json:"prep,omitempty"`
	Resolution      string           `json:"resolution,omitempty"`
	Ratio           string           `json:"ratio,omitempty"`
	TaskType        string           `json:"task_type,omitempty"`
	Content         *metadataContent `json:"content,omitempty"`
}

type taskUsage struct {
	PromptTokens      int   `json:"prompt_tokens,omitempty"`
	CompletionTokens  int   `json:"completion_tokens,omitempty"`
	TotalTokens       int   `json:"total_tokens,omitempty"`
	CostInUSDTicks    int64 `json:"cost_in_usd_ticks,omitempty"`
	TotalSeconds      int   `json:"total_seconds,omitempty"`
	InputSeconds      int   `json:"input_seconds,omitempty"`
	InputAudioSeconds int   `json:"input_audio_seconds,omitempty"`
	OutputSeconds     int   `json:"output_seconds,omitempty"`
	InputImageCount   int   `json:"input_image_count,omitempty"`
}

type taskMetadata struct {
	Status     string           `json:"status,omitempty"`
	Model      string           `json:"model,omitempty"`
	Progress   int              `json:"progress,omitempty"`
	Video      *metadataVideo   `json:"video,omitempty"`
	Usage      *taskUsage       `json:"usage,omitempty"`
	Resolution string           `json:"resolution,omitempty"`
	Duration   int              `json:"duration,omitempty"`
	Ratio      string           `json:"ratio,omitempty"`
	TaskType   string           `json:"task_type,omitempty"`
	Content    *metadataContent `json:"content,omitempty"`
}

type metadataContent struct {
	URL string `json:"url,omitempty"`
}

type metadataVideo struct {
	URL               string  `json:"url,omitempty"`
	Duration          float64 `json:"duration,omitempty"`
	RespectModeration bool    `json:"respect_moderation,omitempty"`
}

type assetGroupResponse struct {
	ID string `json:"id"`
}

type createAssetResponse struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Error  any    `json:"error,omitempty"`
}

type getAssetResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  any    `json:"error,omitempty"`
}

type directAssetRecord struct {
	ID     string `json:"Id"`
	Status string `json:"Status"`
	Error  any    `json:"Error,omitempty"`
}

type directAssetAPIResponse struct {
	Success bool              `json:"success"`
	Data    directAssetRecord `json:"data"`
	Error   any               `json:"error,omitempty"`
}

type assetConfig struct {
	GroupID          string
	GroupName        string
	GroupDescription string
	PollAttempts     int
	PollInterval     time.Duration
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
	proxy       string
	channelID   int
	assetConfig assetConfig
}

var assetGroupCache sync.Map

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
	a.channelID = info.ChannelId
	a.proxy = info.ChannelSetting.Proxy

	groupName := strings.TrimSpace(info.ChannelOtherSettings.ServiceInferenceAssetGroupName)
	if groupName == "" {
		groupName = defaultAssetGroupName
	}
	description := strings.TrimSpace(info.ChannelOtherSettings.ServiceInferenceAssetGroupDescription)
	if description == "" {
		description = defaultAssetGroupDescription
	}
	attempts := info.ChannelOtherSettings.ServiceInferenceAssetPollAttempts
	if attempts <= 0 {
		attempts = defaultAssetPollAttempts
	}
	intervalMS := info.ChannelOtherSettings.ServiceInferenceAssetPollIntervalMS
	if intervalMS < 0 {
		intervalMS = 0
	}
	if intervalMS == 0 && info.ChannelOtherSettings.ServiceInferenceAssetPollIntervalMS == 0 {
		intervalMS = defaultAssetPollIntervalMS
	}

	a.assetConfig = assetConfig{
		GroupID:          strings.TrimSpace(info.ChannelOtherSettings.ServiceInferenceAssetGroupID),
		GroupName:        groupName,
		GroupDescription: description,
		PollAttempts:     attempts,
		PollInterval:     time.Duration(intervalMS) * time.Millisecond,
	}
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var modelRequest struct {
		Model string `json:"model"`
	}
	if err := common.UnmarshalBodyReusable(c, &modelRequest); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.HasPrefix(modelRequest.Model, "grok-imagine-video") {
		if modelRequest.Model != "grok-imagine-video-1.5" {
			return service.TaskErrorWrapperLocal(fmt.Errorf("ServiceInference does not support model %s", modelRequest.Model), "channel_capability_mismatch", http.StatusServiceUnavailable)
		}
		_, taskErr := grokvideo.ParseAndValidate(c, info)
		return taskErr
	}
	var nativeReq requestPayload
	if err := common.UnmarshalBodyReusable(c, &nativeReq); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if len(nativeReq.Content) == 0 {
		return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
	}
	if strings.TrimSpace(nativeReq.Model) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest)
	}
	if isMiniMaxH3Model(nativeReq.Model) {
		if c.Request.ContentLength > tokenMartH3MaxBodyBytes {
			return service.TaskErrorWrapperLocal(fmt.Errorf("request body must not exceed 64 MB"), "invalid_request", http.StatusBadRequest)
		}
		summary, err := validateMiniMaxH3Payload(&nativeReq)
		if err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		metadata := make(map[string]interface{})
		metadataBytes, err := common.Marshal(nativeReq)
		if err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		if err := common.Unmarshal(metadataBytes, &metadata); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		request := relaycommon.TaskSubmitReq{
			Prompt:              taskcommon.FirstTextContent(nativeReq.Content),
			Model:               nativeReq.Model,
			Size:                nativeReq.Resolution,
			Duration:            *nativeReq.Duration,
			AspectRatio:         nativeReq.Ratio,
			Resolution:          nativeReq.Resolution,
			EffectiveResolution: nativeReq.Resolution,
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
	prompt := taskcommon.FirstTextContent(nativeReq.Content)
	if strings.TrimSpace(prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("content text is required"), "invalid_request", http.StatusBadRequest)
	}

	metadata := make(map[string]interface{})
	metadataBytes, err := common.Marshal(nativeReq)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if err := common.Unmarshal(metadataBytes, &metadata); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	req := relaycommon.TaskSubmitReq{
		Prompt:   prompt,
		Model:    nativeReq.Model,
		Metadata: metadata,
	}
	if nativeReq.Duration != nil {
		req.Duration = *nativeReq.Duration
	}
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	info.Action = constant.TaskActionGenerate
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if isSeedanceMaxModel(info.UpstreamModelName) {
		return a.baseURL + "/v2/video/generate", nil
	}
	return a.baseURL + "/v1/video/generate", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	if request, ok := grokvideo.GetRequest(c); ok {
		modelName := request.Model
		if info.IsModelMapped {
			modelName = info.UpstreamModelName
		} else {
			info.UpstreamModelName = modelName
		}
		request.Model = modelName
		data, err := common.Marshal(request)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	body, err := a.convertToRequestPayload(&req)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	if isMiniMaxH3Model(body.Model) {
		if _, err := validateMiniMaxH3Payload(body); err != nil {
			return nil, err
		}
	}
	// Seedance MAX v2 accepts public media URLs directly and performs its own
	// preparation. Existing v1 models keep the legacy asset conversion flow.
	if !isMiniMaxH3Model(body.Model) && !isSeedanceMaxModel(body.Model) {
		if err := a.prepareImageAssets(c.Request.Context(), body); err != nil {
			return nil, err
		}
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = WrapError(err, http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Check for HTTP error status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Parse upstream error response
		bodyText := string(responseBody)

		// Handle specific upstream errors
		if strings.Contains(bodyText, "Model not available") {
			taskErr = ParseModelNotAvailableError(info.OriginModelName)
			return
		}
		if strings.Contains(bodyText, "billing suspended") || strings.Contains(bodyText, "insufficient balance") {
			taskErr = ParseOrganizationBillingSuspendedError()
			return
		}

		// Generic upstream error handling
		taskErr = WrapError(fmt.Errorf("%s", bodyText), resp.StatusCode)
		return
	}

	var submitResp taskResponse
	if err := common.Unmarshal(responseBody, &submitResp); err != nil {
		taskErr = WrapError(errors.Wrapf(err, "body: %s", common.LocalLogPreview(string(responseBody))), http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(submitResp.Task.ID) == "" {
		taskErr = WrapError(fmt.Errorf("task id is empty"), http.StatusInternalServerError)
		return
	}

	if strings.HasPrefix(c.Request.URL.Path, "/grok/v1/") {
		c.JSON(http.StatusOK, map[string]string{"request_id": info.PublicTaskID})
	} else {
		ov := dto.NewOpenAIVideo()
		ov.ID = info.PublicTaskID
		ov.TaskID = info.PublicTaskID
		ov.CreatedAt = time.Now().Unix()
		ov.Model = info.OriginModelName
		applyTaskPreparationMetadata(ov, submitResp.Task)
		c.JSON(http.StatusOK, ov)
	}
	return submitResp.Task.ID, responseBody, nil
}

// HandlesUpstreamErrorResponse lets RelayTaskSubmit route non-2xx upstream submit
// responses through DoResponse (above), so the error classifier in error.go can turn
// proxy-wrapped upstream errors into properly typed, actionable *dto.TaskError values
// (e.g. an invalid video duration reported as a 502 becomes a non-retryable 400 with a
// readable message) instead of the generic fail_to_fetch_task wrapping.
func (a *TaskAdaptor) HandlesUpstreamErrorResponse() bool {
	return true
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	upstreamModel, _ := body["upstream_model"].(string)
	if strings.TrimSpace(upstreamModel) == "" {
		upstreamModel, _ = body["model"].(string)
	}
	apiVersion := "v1"
	if isSeedanceMaxModel(upstreamModel) {
		apiVersion = "v2"
	}
	uri := strings.TrimRight(baseUrl, "/") + "/" + apiVersion + "/video/tasks/" + url.PathEscape(taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask, err := parseVideoTask(respBody)
	if err != nil {
		return nil, err
	}
	taskResult := relaycommon.TaskInfo{
		Code:   0,
		TaskID: resTask.ID,
	}
	effectiveModel := resTask.Model
	if effectiveModel == "" && resTask.Metadata != nil {
		effectiveModel = resTask.Metadata.Model
	}
	if reason := formatUpstreamError(resTask.Error); isMiniMaxH3Model(effectiveModel) && reason != "" {
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = taskcommon.ProgressComplete
		taskResult.Reason = reason
		return &taskResult, nil
	}
	switch strings.ToLower(strings.TrimSpace(resTask.Status)) {
	case "pending", "queued", "submitted":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = taskcommon.ProgressQueued
	case "preparing":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "0%"
		taskResult.Metadata = map[string]interface{}{
			"upstream_status": "preparing",
		}
		if len(resTask.Prep) > 0 {
			taskResult.Metadata["prep"] = resTask.Prep
		}
	case "processing", "running", "in_progress":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "completed", "succeeded", "success":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = taskcommon.ProgressComplete
		if len(resTask.Outputs) > 0 {
			taskResult.Url = resTask.Outputs[0]
		}
		if taskResult.Url == "" && resTask.Content != nil {
			taskResult.Url = resTask.Content.URL
		}
		usage := effectiveTaskUsage(resTask)
		if usage != nil {
			taskResult.PromptTokens = usage.PromptTokens
			taskResult.CompletionTokens = usage.CompletionTokens
			taskResult.TotalTokens = usage.TotalTokens
		}
		taskResult.Metadata = map[string]interface{}{}
		if resTask.DurationSeconds > 0 {
			taskResult.Metadata["duration"] = resTask.DurationSeconds
		}
		if resTask.Resolution != "" {
			taskResult.Metadata["resolution"] = resTask.Resolution
		}
		if resTask.Ratio != "" {
			taskResult.Metadata["ratio"] = resTask.Ratio
		}
		if resTask.TaskType != "" {
			taskResult.Metadata["task_type"] = resTask.TaskType
		}
		if usage != nil {
			taskResult.Metadata["usage"] = usage
			if usage.CostInUSDTicks > 0 {
				taskResult.Metadata["cost_in_usd_ticks"] = usage.CostInUSDTicks
			}
		}
		if resTask.Metadata != nil {
			if taskResult.Url == "" && resTask.Metadata.Video != nil {
				taskResult.Url = resTask.Metadata.Video.URL
			}
			if taskResult.Url == "" && resTask.Metadata.Content != nil {
				taskResult.Url = resTask.Metadata.Content.URL
			}
			taskResult.Metadata["progress"] = resTask.Metadata.Progress
			if resTask.Metadata.Video != nil && resTask.Metadata.Video.Duration > 0 {
				taskResult.Metadata["duration"] = resTask.Metadata.Video.Duration
			}
			if resTask.Metadata.Duration > 0 {
				taskResult.Metadata["duration"] = resTask.Metadata.Duration
			}
			if resTask.Metadata.Resolution != "" {
				taskResult.Metadata["resolution"] = resTask.Metadata.Resolution
			}
			if resTask.Metadata.Ratio != "" {
				taskResult.Metadata["ratio"] = resTask.Metadata.Ratio
			}
			if resTask.Metadata.TaskType != "" {
				taskResult.Metadata["task_type"] = resTask.Metadata.TaskType
			}
		}
	case "failed", "failure", "error", "cancelled", "canceled":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = taskcommon.ProgressComplete
		taskResult.Reason = formatUpstreamError(resTask.Error)
		if taskResult.Reason == "" {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = taskcommon.ProgressInProgress
	}
	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.FinishTime
	openAIVideo.Model = originTask.Properties.OriginModelName
	if originTask.Status == model.TaskStatusSuccess {
		if resultURL := strings.TrimSpace(originTask.GetResultURL()); resultURL != "" {
			openAIVideo.ResultURL = resultURL
			openAIVideo.SetMetadata("url", resultURL)
		}
	}

	if len(bytes.TrimSpace(originTask.Data)) == 0 {
		if originTask.Status == model.TaskStatusFailure {
			message := strings.TrimSpace(originTask.FailReason)
			if message == "" {
				message = "task failed"
			}
			openAIVideo.Error = &dto.OpenAIVideoError{
				Message: message,
				Code:    "upstream_error",
			}
		}
		return common.Marshal(openAIVideo)
	}

	resTask, err := parseVideoTask(originTask.Data)
	if err != nil {
		return nil, err
	}
	if len(resTask.Outputs) > 0 {
		openAIVideo.ResultURL = resTask.Outputs[0]
		openAIVideo.SetMetadata("url", resTask.Outputs[0])
		openAIVideo.SetMetadata("outputs", resTask.Outputs)
	}
	if openAIVideo.ResultURL == "" && resTask.Metadata != nil && resTask.Metadata.Content != nil {
		openAIVideo.ResultURL = resTask.Metadata.Content.URL
		openAIVideo.SetMetadata("url", resTask.Metadata.Content.URL)
	}
	if openAIVideo.ResultURL == "" && resTask.Content != nil {
		openAIVideo.ResultURL = resTask.Content.URL
		openAIVideo.SetMetadata("url", resTask.Content.URL)
	}
	if resTask.LastFrameURL != "" {
		openAIVideo.SetMetadata("last_frame_url", resTask.LastFrameURL)
	}
	if usage := effectiveTaskUsage(resTask); usage != nil {
		openAIVideo.SetMetadata("usage", usage)
	}
	if resTask.DurationSeconds > 0 {
		openAIVideo.Seconds = strconv.Itoa(resTask.DurationSeconds)
	}
	if resTask.Resolution != "" {
		openAIVideo.Size = resTask.Resolution
		openAIVideo.SetMetadata("resolution", resTask.Resolution)
	}
	if resTask.Ratio != "" {
		openAIVideo.SetMetadata("ratio", resTask.Ratio)
	}
	if resTask.TaskType != "" {
		openAIVideo.SetMetadata("task_type", resTask.TaskType)
	}
	if resTask.Metadata != nil {
		if resTask.Metadata.Duration > 0 {
			openAIVideo.Seconds = strconv.Itoa(resTask.Metadata.Duration)
		}
		if resTask.Metadata.Resolution != "" {
			openAIVideo.Size = resTask.Metadata.Resolution
			openAIVideo.SetMetadata("resolution", resTask.Metadata.Resolution)
		}
		if resTask.Metadata.Ratio != "" {
			openAIVideo.SetMetadata("ratio", resTask.Metadata.Ratio)
		}
		if resTask.Metadata.TaskType != "" {
			openAIVideo.SetMetadata("task_type", resTask.Metadata.TaskType)
		}
	}
	applyTaskPreparationMetadata(openAIVideo, resTask)
	if originTask.Status == model.TaskStatusFailure {
		message := formatUpstreamError(resTask.Error)
		if message == "" {
			message = strings.TrimSpace(originTask.FailReason)
		}
		if message == "" {
			message = "task failed"
		}
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: message,
			Code:    "upstream_error",
		}
	}
	return common.Marshal(openAIVideo)
}

func applyTaskPreparationMetadata(video *dto.OpenAIVideo, task videoTask) {
	status := strings.ToLower(strings.TrimSpace(task.Status))
	if status == "preparing" {
		video.Status = dto.VideoStatusInProgress
	}
	if len(task.Prep) == 0 && status != "preparing" {
		return
	}
	video.SetMetadata("upstream_status", status)
	if len(task.Prep) > 0 {
		video.SetMetadata("prep", task.Prep)
	}
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	payload := &requestPayload{
		Model:   req.Model,
		Content: make([]ContentItem, 0),
	}
	for _, imgURL := range req.Images {
		if strings.TrimSpace(imgURL) == "" {
			continue
		}
		payload.Content = append(payload.Content, ContentItem{
			Type:     "image_url",
			ImageURL: &MediaURL{URL: imgURL},
		})
	}
	if err := taskcommon.UnmarshalMetadata(req.Metadata, payload); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}
	if payload.Duration == nil {
		if req.Duration > 0 {
			payload.Duration = &req.Duration
		} else if seconds := strings.TrimSpace(req.Seconds); seconds != "" {
			var duration int
			if _, err := fmt.Sscanf(seconds, "%d", &duration); err == nil {
				payload.Duration = &duration
			}
		}
	}
	if !taskcommon.HasTextContent(payload.Content) && strings.TrimSpace(req.Prompt) != "" {
		payload.Content = append(payload.Content, ContentItem{
			Type: "text",
			Text: req.Prompt,
		})
	}
	return payload, nil
}

func (a *TaskAdaptor) prepareImageAssets(ctx context.Context, payload *requestPayload) error {
	for i := range payload.Content {
		item := &payload.Content[i]
		if item.ImageURL == nil {
			continue
		}
		rawURL := strings.TrimSpace(item.ImageURL.URL)
		if rawURL == "" || strings.HasPrefix(rawURL, "asset://") {
			continue
		}
		if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
			return fmt.Errorf("service inference image asset url must be http(s) or asset://")
		}
		name := strings.TrimSpace(item.Role)
		if name == "" {
			name = "reference-image"
		}
		var assetID string
		var err error
		if workflow, basePath, ok := seedanceDirectAssetWorkflow(payload.Model); ok {
			assetID, err = a.ensureDirectImageAsset(ctx, workflow, basePath, rawURL, name)
		} else {
			assetID, err = a.ensureImageAsset(ctx, rawURL, name)
		}
		if err != nil {
			return err
		}
		item.ImageURL.URL = "asset://" + assetID
	}
	return nil
}

func (a *TaskAdaptor) ensureDirectImageAsset(ctx context.Context, workflow string, basePath string, imageURL string, name string) (string, error) {
	body := map[string]any{
		"URL":       imageURL,
		"AssetType": "Image",
	}
	if name != "" {
		body["Name"] = name
	}

	var createResponse directAssetAPIResponse
	_, _, err := a.doJSON(ctx, http.MethodPost, basePath, body, &createResponse)
	if err != nil {
		return "", err
	}
	if !createResponse.Success {
		errorDetail := createResponse.Data.Error
		if errorDetail == nil {
			errorDetail = createResponse.Error
		}
		if reason := formatUpstreamError(errorDetail); reason != "" {
			return "", fmt.Errorf("service inference %s asset failed: %s", workflow, reason)
		}
		return "", fmt.Errorf("service inference %s asset creation failed", workflow)
	}
	assetID := strings.TrimSpace(createResponse.Data.ID)
	if assetID == "" {
		return "", fmt.Errorf("service inference %s asset id is empty", workflow)
	}
	switch strings.ToLower(strings.TrimSpace(createResponse.Data.Status)) {
	case "active", "completed", "succeeded", "success":
		return assetID, nil
	}

	for attempt := 0; attempt < a.assetConfig.PollAttempts; attempt++ {
		var statusResponse directAssetAPIResponse
		_, _, err = a.doJSON(ctx, http.MethodGet, basePath+"/"+url.PathEscape(assetID), nil, &statusResponse)
		if err != nil {
			return "", err
		}
		if !statusResponse.Success {
			errorDetail := statusResponse.Data.Error
			if errorDetail == nil {
				errorDetail = statusResponse.Error
			}
			if reason := formatUpstreamError(errorDetail); reason != "" {
				return "", fmt.Errorf("service inference %s asset failed: %s", workflow, reason)
			}
			return "", fmt.Errorf("service inference %s asset status query failed", workflow)
		}
		switch strings.ToLower(strings.TrimSpace(statusResponse.Data.Status)) {
		case "active", "completed", "succeeded", "success":
			return assetID, nil
		case "failed", "failure", "error":
			if reason := formatUpstreamError(statusResponse.Data.Error); reason != "" {
				return "", fmt.Errorf("service inference %s asset failed: %s", workflow, reason)
			}
			return "", fmt.Errorf("service inference %s asset failed", workflow)
		}
		if attempt == a.assetConfig.PollAttempts-1 {
			break
		}
		if a.assetConfig.PollInterval > 0 {
			timer := time.NewTimer(a.assetConfig.PollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
	}
	return "", fmt.Errorf("service inference %s asset %s did not become active", workflow, assetID)
}

func (a *TaskAdaptor) ensureImageAsset(ctx context.Context, imageURL string, name string) (string, error) {
	groupID, err := a.ensureAssetGroup(ctx)
	if err != nil {
		return "", err
	}
	asset, status, err := a.createAsset(ctx, groupID, imageURL, name)
	if status == http.StatusNotFound {
		assetGroupCache.Delete(a.assetGroupCacheKey())
		groupID, err = a.createAssetGroup(ctx)
		if err != nil {
			return "", err
		}
		asset, _, err = a.createAsset(ctx, groupID, imageURL, name)
	}
	if err != nil {
		return "", err
	}
	if strings.EqualFold(asset.Status, "completed") {
		return asset.ID, nil
	}
	return a.waitAssetCompleted(ctx, asset)
}

func (a *TaskAdaptor) ensureAssetGroup(ctx context.Context) (string, error) {
	if cached, ok := assetGroupCache.Load(a.assetGroupCacheKey()); ok {
		if groupID, ok := cached.(string); ok && strings.TrimSpace(groupID) != "" {
			exists, err := a.assetGroupExists(ctx, groupID)
			if err == nil && exists {
				return groupID, nil
			}
			assetGroupCache.Delete(a.assetGroupCacheKey())
			if err != nil {
				return "", err
			}
		}
	}
	if a.assetConfig.GroupID != "" {
		exists, err := a.assetGroupExists(ctx, a.assetConfig.GroupID)
		if err != nil {
			return "", err
		}
		if exists {
			assetGroupCache.Store(a.assetGroupCacheKey(), a.assetConfig.GroupID)
			return a.assetConfig.GroupID, nil
		}
	}
	return a.createAssetGroup(ctx)
}

func (a *TaskAdaptor) assetGroupExists(ctx context.Context, groupID string) (bool, error) {
	var group assetGroupResponse
	status, _, err := a.doJSON(ctx, http.MethodGet, "/v1/asset-groups/"+url.PathEscape(groupID), nil, &group)
	if status == http.StatusNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(group.ID) != "", nil
}

func (a *TaskAdaptor) createAssetGroup(ctx context.Context) (string, error) {
	body := map[string]any{
		"name":        a.assetConfig.GroupName,
		"description": a.assetConfig.GroupDescription,
	}
	var group assetGroupResponse
	_, _, err := a.doJSON(ctx, http.MethodPost, "/v1/asset-groups", body, &group)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(group.ID) == "" {
		return "", fmt.Errorf("service inference asset group id is empty")
	}
	assetGroupCache.Store(a.assetGroupCacheKey(), group.ID)
	return group.ID, nil
}

func (a *TaskAdaptor) createAsset(ctx context.Context, groupID string, imageURL string, name string) (*createAssetResponse, int, error) {
	body := map[string]any{
		"group_id":   groupID,
		"url":        imageURL,
		"asset_type": "Image",
		"name":       name,
	}
	var asset createAssetResponse
	status, _, err := a.doJSON(ctx, http.MethodPost, "/v1/assets", body, &asset)
	if err != nil {
		return nil, status, err
	}
	if strings.TrimSpace(asset.ID) == "" {
		return nil, status, fmt.Errorf("service inference asset id is empty")
	}
	return &asset, status, nil
}

func (a *TaskAdaptor) waitAssetCompleted(ctx context.Context, asset *createAssetResponse) (string, error) {
	if asset == nil {
		return "", fmt.Errorf("service inference asset response is empty")
	}
	for attempt := 0; attempt < a.assetConfig.PollAttempts; attempt++ {
		current, err := a.getAsset(ctx, asset.ID, asset.TaskID)
		if err != nil {
			return "", err
		}
		switch strings.ToLower(strings.TrimSpace(current.Status)) {
		case "completed", "succeeded", "success":
			return current.ID, nil
		case "failed", "failure", "error":
			if reason := formatUpstreamError(current.Error); reason != "" {
				return "", fmt.Errorf("service inference asset failed: %s", reason)
			}
			return "", fmt.Errorf("service inference asset failed")
		}
		if attempt == a.assetConfig.PollAttempts-1 {
			break
		}
		if a.assetConfig.PollInterval > 0 {
			timer := time.NewTimer(a.assetConfig.PollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
	}
	return "", fmt.Errorf("service inference asset %s did not complete", asset.ID)
}

func (a *TaskAdaptor) getAsset(ctx context.Context, assetID string, taskID string) (*getAssetResponse, error) {
	body := map[string]any{
		"asset_id": assetID,
		"task_id":  taskID,
	}
	var asset getAssetResponse
	_, _, err := a.doJSON(ctx, http.MethodPost, "/v1/assets/get", body, &asset)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(asset.ID) == "" {
		return nil, fmt.Errorf("service inference asset id is empty")
	}
	return &asset, nil
}

func (a *TaskAdaptor) doJSON(ctx context.Context, method string, path string, body any, out any) (int, []byte, error) {
	var requestBody io.Reader
	if body != nil {
		data, err := common.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		requestBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, requestBody)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	client, err := service.GetHttpClientWithProxy(a.proxy)
	if err != nil {
		return 0, nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return resp.StatusCode, respBody, fmt.Errorf("service inference api status=%d body=%s", resp.StatusCode, common.LocalLogPreview(string(respBody)))
	}
	if out != nil {
		if err := common.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, respBody, err
		}
	}
	return resp.StatusCode, respBody, nil
}

func (a *TaskAdaptor) assetGroupCacheKey() string {
	return fmt.Sprintf("%d|%s|%s|%s", a.channelID, a.baseURL, a.assetConfig.GroupID, a.assetConfig.GroupName)
}

func parseVideoTask(respBody []byte) (videoTask, error) {
	var wrapped taskResponse
	if err := common.Unmarshal(respBody, &wrapped); err != nil {
		return videoTask{}, errors.Wrap(err, "unmarshal task result failed")
	}
	if strings.TrimSpace(wrapped.Task.ID) != "" || strings.TrimSpace(wrapped.Task.Status) != "" {
		return wrapped.Task, nil
	}
	var direct videoTask
	if err := common.Unmarshal(respBody, &direct); err != nil {
		return videoTask{}, errors.Wrap(err, "unmarshal direct task result failed")
	}
	return direct, nil
}

func formatUpstreamError(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case map[string]any:
		if msg, _ := v["message"].(string); strings.TrimSpace(msg) != "" {
			return msg
		}
		if errText, _ := v["error"].(string); strings.TrimSpace(errText) != "" {
			return errText
		}
	}
	data, err := common.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

// EstimateBilling 根据请求 metadata 中的输出分辨率与是否含视频输入，返回相对基准价的计费 OtherRatio。
// 具体价格规则由上游模型族决定，倍率为 1.0（基准价）时无需附加 OtherRatio。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if request, ok := grokvideo.GetRequest(c); ok {
		return grokvideo.EstimateBilling(request, info.Action)
	}
	if isMiniMaxH3Model(info.UpstreamModelName) || isMiniMaxH3Model(info.OriginModelName) {
		return a.estimateMiniMaxH3Billing(c, info)
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	resolution, _ := req.Metadata["resolution"].(string)
	hasVideo := hasVideoInMetadata(req.Metadata)
	pricingModelName := info.UpstreamModelName
	if pricingModelName == "" {
		pricingModelName = info.OriginModelName
	}
	ratio, ok := videoInputRatio(pricingModelName, resolution, hasVideo)
	if !ok || ratio == 1.0 {
		return nil
	}
	return map[string]float64{"video_input": ratio}
}

// hasVideoInMetadata 检查 metadata 的 content 数组是否包含 video_url 条目
func hasVideoInMetadata(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	contentRaw, ok := metadata["content"]
	if !ok {
		return false
	}
	contentList, ok := contentRaw.([]interface{})
	if !ok {
		return false
	}
	for _, item := range contentList {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if itemType, ok := itemMap["type"].(string); ok && itemType == "video_url" {
			return true
		}
		if _, hasVideoURL := itemMap["video_url"]; hasVideoURL {
			return true
		}
	}
	return false
}
