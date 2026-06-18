// Package tencentvideo implements a task adaptor for Tencent Cloud's
// image-to-video (Kling) service exposed via the VCLM API
// (vclm.tencentcloudapi.com). It authenticates each request with the
// TC3-HMAC-SHA256 signature scheme and bridges Tencent's
// SubmitImageToVideoJob / DescribeImageToVideoJob pair to the unified
// async-task / OpenAI video flow used across new-api.
package tencentvideo

import (
	"bytes"
	"fmt"
	"io"
	"math"
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
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const (
	tcService       = "vclm"
	tcVersion       = "2024-05-23"
	tcDefaultHost   = "vclm.tencentcloudapi.com"
	tcDefaultRegion = "ap-guangzhou"

	actionSubmit   = "SubmitImageToVideoJob"
	actionDescribe = "DescribeImageToVideoJob"

	// gin context keys
	ctxKeyBody   = "tencentvideo_body_bytes"
	ctxKeyRegion = "tencentvideo_region"
)

// ============================
// Request / Response structures (Tencent VCLM uses PascalCase JSON keys)
// ============================

// imageRef matches Tencent's Image / ImageTail input object.
type imageRef struct {
	Url string `json:"Url,omitempty"`
}

type cameraConfig struct {
	Horizontal *float64 `json:"Horizontal,omitempty"`
	Vertical   *float64 `json:"Vertical,omitempty"`
	Pan        *float64 `json:"Pan,omitempty"`
	Tilt       *float64 `json:"Tilt,omitempty"`
	Roll       *float64 `json:"Roll,omitempty"`
	Zoom       *float64 `json:"Zoom,omitempty"`
}

type cameraControl struct {
	Type   string        `json:"Type,omitempty"`
	Config *cameraConfig `json:"Config,omitempty"`
}

type multiPromptItem struct {
	Index    int    `json:"Index,omitempty"`
	Prompt   string `json:"Prompt,omitempty"`
	Duration string `json:"Duration,omitempty"`
}

type elementItem struct {
	ElementId int64 `json:"ElementId,omitempty"`
}

type voiceItem struct {
	VoiceId string `json:"VoiceId,omitempty"`
}

// submitPayload is the SubmitImageToVideoJob request body.
type submitPayload struct {
	Model          string            `json:"Model,omitempty"`
	Image          *imageRef         `json:"Image,omitempty"`
	ImageTail      *imageRef         `json:"ImageTail,omitempty"`
	Prompt         string            `json:"Prompt,omitempty"`
	NegativePrompt string            `json:"NegativePrompt,omitempty"`
	Duration       string            `json:"Duration,omitempty"`
	Mode           string            `json:"Mode,omitempty"`
	CfgScale       *float64          `json:"CfgScale,omitempty"`
	Sound          string            `json:"Sound,omitempty"`
	MultiShot      *bool             `json:"MultiShot,omitempty"`
	ShotType       string            `json:"ShotType,omitempty"`
	MultiPrompt    []multiPromptItem `json:"MultiPrompt,omitempty"`
	ElementList    []elementItem     `json:"ElementList,omitempty"`
	StaticMask     string            `json:"StaticMask,omitempty"`
	CameraControl  *cameraControl    `json:"CameraControl,omitempty"`
	VoiceList      []voiceItem       `json:"VoiceList,omitempty"`
	CallbackUrl    string            `json:"CallbackUrl,omitempty"`
	LogoAdd        *int              `json:"LogoAdd,omitempty"`
}

// submitResponse is the {"Response":{...}} envelope for SubmitImageToVideoJob.
type submitResponse struct {
	Response struct {
		JobId     string        `json:"JobId"`
		RequestId string        `json:"RequestId"`
		Error     *tencentError `json:"Error,omitempty"`
	} `json:"Response"`
}

// describeResponse is the {"Response":{...}} envelope for DescribeImageToVideoJob.
type describeResponse struct {
	Response struct {
		Status             string        `json:"Status"`
		ErrorCode          string        `json:"ErrorCode"`
		ErrorMessage       string        `json:"ErrorMessage"`
		ResultVideoUrl     string        `json:"ResultVideoUrl"`
		VideoId            string        `json:"VideoId"`
		Duration           string        `json:"Duration"`
		FinalUnitDeduction string        `json:"FinalUnitDeduction"`
		RequestId          string        `json:"RequestId"`
		Error              *tencentError `json:"Error,omitempty"`
	} `json:"Response"`
}

type tencentError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

// ============================
// Adaptor
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
	region      string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey // format: SecretId|SecretKey
	a.region = tcDefaultRegion
}

func (a *TaskAdaptor) GetChannelName() string {
	return "tencentvideo"
}

// GetModelList exposes the friendly model names users configure on the channel.
// They are mapped to Tencent's short codes (v1.0, v1.6, v3.0, …) at request time.
func (a *TaskAdaptor) GetModelList() []string {
	return []string{
		"kling-v1", "kling-v1-5", "kling-v1-6",
		"kling-v2-master", "kling-v2-1", "kling-v2-1-master",
		"kling-v2-5-turbo", "kling-v2-6", "kling-v3",
	}
}

// modelNameToTencentCode maps the friendly model name to Tencent's Model code.
// Unknown names are passed through unchanged so a raw code (e.g. "v1.6") also works.
func modelNameToTencentCode(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "kling-v1", "kling-v1-0":
		return "v1.0"
	case "kling-v1-5":
		return "v1.5"
	case "kling-v1-6":
		return "v1.6"
	case "kling-v2-master", "kling-v2-0":
		return "v2.0"
	case "kling-v2-1":
		return "v2.1"
	case "kling-v2-1-master":
		return "v2.1m"
	case "kling-v2-5-turbo":
		return "v2.5"
	case "kling-v2-6":
		return "v2.6"
	case "kling-v3", "kling-v3-0":
		return "v3.0"
	default:
		return name
	}
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if err := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); err != nil {
		return err
	}
	info.Action = constant.TaskActionGenerate
	return nil
}

// BuildRequestBody converts the unified task request into a Tencent
// SubmitImageToVideoJob payload and caches the exact bytes for TC3 signing.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req := v.(relaycommon.TaskSubmitReq)

	payload, err := a.convertToSubmitPayload(&req, info)
	if err != nil {
		return nil, err
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	// Cache the marshaled body so BuildRequestHeader can sign the exact bytes.
	c.Set(ctxKeyBody, data)
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) convertToSubmitPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*submitPayload, error) {
	modelCode := modelNameToTencentCode(info.UpstreamModelName)

	p := submitPayload{
		Model:    modelCode,
		Prompt:   req.Prompt,
		Duration: strconv.Itoa(taskcommon.DefaultInt(req.Duration, 5)),
		Mode:     taskcommon.DefaultString(req.Mode, "std"),
	}
	// Tencent's Image parameter only accepts a URL; the first image is treated as Image.
	if len(req.Images) > 0 && strings.TrimSpace(req.Images[0]) != "" {
		p.Image = &imageRef{Url: req.Images[0]}
	}
	// Optional negative prompt at the top level of metadata.
	if v, ok := req.Metadata["negative_prompt"]; ok {
		if s, ok := v.(string); ok {
			p.NegativePrompt = s
		}
	}
	// Optional region override (used in TC3 credential scope, not the body).
	if v, ok := req.Metadata["region"]; ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			a.region = strings.TrimSpace(s)
		}
	}
	// metadata may override / supply any PascalCase Tencent field
	// (e.g. ImageTail, CameraControl, MultiShot, Sound, CfgScale, LogoAdd, region).
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &p); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}
	// Re-apply the channel-mapped model so metadata cannot bypass billing.
	p.Model = modelCode

	if p.Image == nil && p.ImageTail == nil {
		return nil, fmt.Errorf("either image or image_tail (ImageTail) is required")
	}
	return &p, nil
}

// hostFromBaseURL extracts the host used for signing and the Host header.
func (a *TaskAdaptor) host() string {
	if a.baseURL == "" {
		return tcDefaultHost
	}
	if u, err := url.Parse(a.baseURL); err == nil && u.Host != "" {
		return u.Host
	}
	return tcDefaultHost
}

// BuildRequestURL — Tencent VCLM is a single POST endpoint at the root path.
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("https://%s/", a.host()), nil
}

// BuildRequestHeader applies the TC3-HMAC-SHA256 signature over the cached body.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	body, _ := c.Get(ctxKeyBody)
	payload, _ := body.([]byte)
	secretId, secretKey, err := splitCredential(a.apiKey)
	if err != nil {
		return err
	}
	a.applyTC3Headers(req, actionSubmit, payload, secretId, secretKey, a.region)
	return nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse parses the SubmitImageToVideoJob envelope and returns the JobId.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}

	var sResp submitResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "%s", responseBody), "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}
	if sResp.Response.Error != nil && sResp.Response.Error.Code != "" {
		taskErr = service.TaskErrorWrapperLocal(
			fmt.Errorf("%s: %s", sResp.Response.Error.Code, sResp.Response.Error.Message),
			"task_failed", http.StatusBadRequest)
		return
	}
	if sResp.Response.JobId == "" {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("empty JobId in response: %s", responseBody), "task_failed", http.StatusBadRequest)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)
	return sResp.Response.JobId, responseBody, nil
}

// FetchTask queries DescribeImageToVideoJob for the given JobId.
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	secretId, secretKey, err := splitCredential(key)
	if err != nil {
		return nil, err
	}

	a.baseURL = baseUrl
	region := tcDefaultRegion
	if r, ok := body["region"].(string); ok && strings.TrimSpace(r) != "" {
		region = strings.TrimSpace(r)
	}

	payload, err := common.Marshal(map[string]string{"JobId": taskID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://%s/", a.host()), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	a.applyTC3Headers(req, actionDescribe, payload, secretId, secretKey, region)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

// ParseTaskResult maps DescribeImageToVideoJob status to the unified TaskInfo.
// Tencent statuses: WAIT (queued), RUN (processing), DONE (success), FAIL (failure).
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var dResp describeResponse
	if err := common.Unmarshal(respBody, &dResp); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}
	r := dResp.Response
	if r.Error != nil && r.Error.Code != "" {
		return nil, fmt.Errorf("%s: %s", r.Error.Code, r.Error.Message)
	}

	taskInfo := &relaycommon.TaskInfo{}
	switch r.Status {
	case "WAIT":
		taskInfo.Status = model.TaskStatusSubmitted
		taskInfo.Progress = taskcommon.ProgressQueued
	case "RUN":
		taskInfo.Status = model.TaskStatusInProgress
		taskInfo.Progress = taskcommon.ProgressInProgress
	case "DONE":
		taskInfo.Status = model.TaskStatusSuccess
		taskInfo.Progress = taskcommon.ProgressComplete
		taskInfo.Url = r.ResultVideoUrl
		if v, err := strconv.ParseFloat(r.FinalUnitDeduction, 64); err == nil {
			if rounded := int(math.Ceil(v)); rounded > 0 {
				taskInfo.CompletionTokens = rounded
				taskInfo.TotalTokens = rounded
			}
		}
	case "FAIL":
		taskInfo.Status = model.TaskStatusFailure
		taskInfo.Reason = strings.TrimSpace(r.ErrorCode + " " + r.ErrorMessage)
	default:
		return nil, fmt.Errorf("unknown task status: %s", r.Status)
	}
	return taskInfo, nil
}

// ConvertToOpenAIVideo renders the stored describe response as an OpenAI video object.
func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt

	var dResp describeResponse
	if err := common.Unmarshal(originTask.Data, &dResp); err == nil {
		r := dResp.Response
		if r.ResultVideoUrl != "" {
			openAIVideo.SetMetadata("url", r.ResultVideoUrl)
		}
		if r.Duration != "" {
			openAIVideo.Seconds = r.Duration
		}
		if r.Status == "FAIL" {
			openAIVideo.Error = &dto.OpenAIVideoError{
				Message: strings.TrimSpace(r.ErrorMessage),
				Code:    r.ErrorCode,
			}
		}
	}
	return common.Marshal(openAIVideo)
}
