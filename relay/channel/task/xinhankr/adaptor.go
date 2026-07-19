package xinhankr

// xinhankr 视频生成渠道（type 61）：上游是 token.xinhankr.com 的
// OpenAI 对齐视频网关（火山方舟 Seedance 接入指南格式）。
//
// 与官方 DoubaoVideo（type 54，火山 /api/v3/contents/generations/tasks 的
// content 多模态数组格式）不同，xinhankr 的请求体是扁平的：
//   images: []string 或 [{url, role|type}]  — 图片参考（first_frame/last_frame/reference_image）
//   videos: []string                        — 视频参考直链
//   audios: []string                        — 音频参考直链
//   resolution / ratio / duration / camera_fixed / generate_audio / web_search / seed — 顶层标量
// 端点与本网关自身的 /v1/video/generations 完全同构，因此客户端请求原样透传即可。
// 计费口径与官方 DoubaoVideo 保持一致（复用其价格表，见 constants.go）。

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

// ============================
// Request / Response structures
// ============================

// ImageEntry 兼容字符串与对象两种形态：
//
//	"https://..."                              → 首帧/尾帧智能识别（由上游处理）
//	{"url": "https://...", "role": "..."}      → 指定角色
//	{"url": "https://...", "type": "..."}      → type 与 role 等价
type ImageEntry struct {
	URL  string `json:"url,omitempty"`
	Role string `json:"role,omitempty"`
	Type string `json:"type,omitempty"`
}

func (e *ImageEntry) UnmarshalJSON(data []byte) error {
	var s string
	if err := common.Unmarshal(data, &s); err == nil {
		e.URL = s
		return nil
	}
	type alias ImageEntry
	var a alias
	if err := common.Unmarshal(data, &a); err != nil {
		return err
	}
	*e = ImageEntry(a)
	return nil
}

func (e ImageEntry) MarshalJSON() ([]byte, error) {
	// 无角色时序列化回纯字符串，保持「智能首尾帧」语义不变。
	if e.Role == "" && e.Type == "" {
		return common.Marshal(e.URL)
	}
	type alias ImageEntry
	return common.Marshal(alias(e))
}

type requestPayload struct {
	Model         string         `json:"model"`
	Prompt        string         `json:"prompt"`
	Images        []ImageEntry   `json:"images,omitempty"`
	ImageUrls     []ImageEntry   `json:"image_urls,omitempty"` // images 的等价别名（上游文档）
	Videos        []string       `json:"videos,omitempty"`
	Audios        []string       `json:"audios,omitempty"`
	Resolution    string         `json:"resolution,omitempty"`
	Ratio         string         `json:"ratio,omitempty"`
	Duration      *dto.IntValue  `json:"duration,omitempty"`
	CameraFixed   *dto.BoolValue `json:"camera_fixed,omitempty"`
	GenerateAudio *dto.BoolValue `json:"generate_audio,omitempty"`
	WebSearch     *dto.BoolValue `json:"web_search,omitempty"`
	Seed          *dto.IntValue  `json:"seed,omitempty"`
}

// 提交响应：{"id": "...", "task_id": "...", "status": "pending", "message": "..."}
type responsePayload struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
}

// 查询响应：{"id","task_id","status","data":[{"url":"..."}],"error":{...}}
type responseTask struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Data   []struct {
		URL string `json:"url"`
	} `json:"data"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction 自行解析 xinhankr 原生格式。
// 不能走 ValidateBasicTaskRequest：TaskSubmitReq.Images 是 []string，
// 而本渠道的 images 允许 {url, role} 对象数组，直接解析会 400。
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var nativeReq requestPayload
	if err := common.UnmarshalBodyReusable(c, &nativeReq); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(nativeReq.Model) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest)
	}
	if strings.TrimSpace(nativeReq.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest)
	}

	// 原生请求整体塞进 metadata，BuildRequestBody 原样还原后透传。
	metadata := make(map[string]interface{})
	metadataBytes, err := common.Marshal(nativeReq)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if err := common.Unmarshal(metadataBytes, &metadata); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	req := relaycommon.TaskSubmitReq{
		Prompt:   nativeReq.Prompt,
		Model:    nativeReq.Model,
		Metadata: metadata,
	}
	if nativeReq.Duration != nil {
		req.Duration = int(*nativeReq.Duration)
	}
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	info.Action = constant.TaskActionGenerate
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/video/generations", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// EstimateBilling 计费口径与官方 DoubaoVideo 一致：按输出分辨率档 × 是否含视频输入取倍率。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	resolution, _ := req.Metadata["resolution"].(string)
	ratio, ok := videoInputRatio(info.OriginModelName, resolution, hasVideoInMetadata(req.Metadata))
	if !ok || ratio == 1.0 {
		return nil
	}
	return map[string]float64{"video_input": ratio}
}

// hasVideoInMetadata：xinhankr 格式的视频参考在顶层 videos 数组。
func hasVideoInMetadata(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	videosRaw, ok := metadata["videos"]
	if !ok {
		return false
	}
	videos, ok := videosRaw.([]interface{})
	return ok && len(videos) > 0
}

// BuildRequestBody 从 metadata 还原原生 payload 后原样透传（仅覆盖映射后的模型名）。
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	body := requestPayload{Model: req.Model, Prompt: req.Prompt}
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &body); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}
	// UnmarshalMetadata 为防计费绕过会删掉 metadata.model，这里补回。
	if body.Model == "" {
		body.Model = req.Model
	}
	if body.Prompt == "" {
		body.Prompt = req.Prompt
	}
	if body.Duration == nil && req.Duration > 0 {
		d := dto.IntValue(req.Duration)
		body.Duration = &d
	} else if body.Duration == nil {
		if sec, _ := strconv.Atoi(req.Seconds); sec > 0 {
			d := dto.IntValue(sec)
			body.Duration = &d
		}
	}

	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
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

	var dResp responsePayload
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	upstreamID := dResp.TaskID
	if upstreamID == "" {
		upstreamID = dResp.ID
	}
	if upstreamID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return upstreamID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := strings.TrimRight(baseUrl, "/") + "/v1/video/generations/" + url.PathEscape(taskID)

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
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{Code: 0}

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
		if len(resTask.Data) > 0 {
			taskResult.Url = resTask.Data[0].URL
		}
		taskResult.CompletionTokens = resTask.Usage.CompletionTokens
		taskResult.TotalTokens = resTask.Usage.TotalTokens
	case "failed", "failure", "error", "cancelled", "canceled":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = taskcommon.ProgressComplete
		taskResult.Reason = resTask.Error.Message
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

	if len(bytes.TrimSpace(originTask.Data)) > 0 {
		var dResp responseTask
		if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
			return nil, errors.Wrap(err, "unmarshal xinhankr task data failed")
		}
		if len(dResp.Data) > 0 {
			openAIVideo.SetMetadata("url", dResp.Data[0].URL)
		}
		if originTask.Status == model.TaskStatusFailure {
			message := strings.TrimSpace(dResp.Error.Message)
			if message == "" {
				message = strings.TrimSpace(originTask.FailReason)
			}
			if message == "" {
				message = "task failed"
			}
			openAIVideo.Error = &dto.OpenAIVideoError{
				Message: message,
				Code:    dResp.Error.Code,
			}
		}
	} else if originTask.Status == model.TaskStatusFailure {
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

// HandlesUpstreamErrorResponse：让非 2xx 的上游提交响应进入 DoResponse 之前
// 保持默认包装行为即可（上游错误体是标准 OpenAI error JSON，直接透传可读）。
func (a *TaskAdaptor) HandlesUpstreamErrorResponse() bool {
	return false
}
