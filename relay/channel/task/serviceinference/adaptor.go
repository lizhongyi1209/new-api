package serviceinference

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type ContentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type requestPayload struct {
	Model           string        `json:"model"`
	Content         []ContentItem `json:"content,omitempty"`
	Duration        *int          `json:"duration,omitempty"`
	Resolution      string        `json:"resolution,omitempty"`
	Ratio           string        `json:"ratio,omitempty"`
	GenerateAudio   *bool         `json:"generate_audio,omitempty"`
	Watermark       *bool         `json:"watermark,omitempty"`
	ReturnLastFrame *bool         `json:"return_last_frame,omitempty"`
}

type taskResponse struct {
	Task videoTask `json:"task"`
}

type videoTask struct {
	ID              string     `json:"id"`
	Status          string     `json:"status"`
	Model           string     `json:"model"`
	DurationSeconds int        `json:"duration_seconds"`
	Outputs         []string   `json:"outputs"`
	Error           any        `json:"error"`
	CreatedAt       string     `json:"created_at"`
	CompletedAt     string     `json:"completed_at"`
	Usage           *taskUsage `json:"usage,omitempty"`
	LastFrameURL    string     `json:"last_frame_url,omitempty"`
}

type taskUsage struct {
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
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
	prompt := firstTextContent(nativeReq.Content)
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

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/video/generate", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
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
	if err := a.prepareImageAssets(c.Request.Context(), body); err != nil {
		return nil, err
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
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var submitResp taskResponse
	if err := common.Unmarshal(responseBody, &submitResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", common.LocalLogPreview(string(responseBody))), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(submitResp.Task.ID) == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return submitResp.Task.ID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	uri := strings.TrimRight(baseUrl, "/") + "/v1/video/tasks/" + url.PathEscape(taskID)
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
	switch strings.ToLower(strings.TrimSpace(resTask.Status)) {
	case "pending", "queued", "submitted":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = taskcommon.ProgressQueued
	case "processing", "running", "in_progress":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "completed", "succeeded", "success":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = taskcommon.ProgressComplete
		if len(resTask.Outputs) > 0 {
			taskResult.Url = resTask.Outputs[0]
		}
		if resTask.Usage != nil {
			taskResult.CompletionTokens = resTask.Usage.CompletionTokens
			taskResult.TotalTokens = resTask.Usage.TotalTokens
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
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

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
		openAIVideo.SetMetadata("url", resTask.Outputs[0])
		openAIVideo.SetMetadata("outputs", resTask.Outputs)
	}
	if resTask.LastFrameURL != "" {
		openAIVideo.SetMetadata("last_frame_url", resTask.LastFrameURL)
	}
	if resTask.Usage != nil {
		openAIVideo.SetMetadata("usage", resTask.Usage)
	}
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
	if !hasTextContent(payload.Content) && strings.TrimSpace(req.Prompt) != "" {
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
		assetID, err := a.ensureImageAsset(ctx, rawURL, name)
		if err != nil {
			return err
		}
		item.ImageURL.URL = "asset://" + assetID
	}
	return nil
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

func firstTextContent(content []ContentItem) string {
	for _, item := range content {
		if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
			return item.Text
		}
	}
	return ""
}

func hasTextContent(content []ContentItem) bool {
	return strings.TrimSpace(firstTextContent(content)) != ""
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
