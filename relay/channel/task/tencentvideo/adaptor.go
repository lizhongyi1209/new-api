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
	"github.com/QuantumNous/new-api/logger"
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

	actionMotionSubmit   = "SubmitMotionControlKlingJob"
	actionMotionDescribe = "DescribeMotionControlKlingJob"

	actionOmniSubmit   = "SubmitVideoEditKlingJob"
	actionOmniDescribe = "DescribeVideoEditKlingJob"

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
	ElementId string `json:"ElementId,omitempty"`
}

type voiceItem struct {
	VoiceId string `json:"VoiceId,omitempty"`
}

type imageInfo struct {
	ImageUrl string `json:"ImageUrl,omitempty"`
	Type     string `json:"Type,omitempty"` // first_frame or end_frame
}

type referVideoInfo struct {
	VideoUrl          string `json:"VideoUrl,omitempty"`
	ReferType         string `json:"ReferType,omitempty"`         // feature or base
	KeepOriginalSound string `json:"KeepOriginalSound,omitempty"` // yes or no
}

type logoParam struct {
	LogoUrl  string `json:"LogoUrl,omitempty"`
	LogoRect string `json:"LogoRect,omitempty"`
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

// motionPayload is the SubmitMotionControlKlingJob request body. Unlike
// image2video, Model uses full names (kling-v2-6 / kling-v3) and Image is a
// plain URL string (not an {Url} object).
type motionPayload struct {
	Model                string        `json:"Model,omitempty"`
	Prompt               string        `json:"Prompt,omitempty"`
	Image                string        `json:"Image,omitempty"`
	Video                string        `json:"Video,omitempty"`
	Mode                 string        `json:"Mode,omitempty"`
	ElementList          []elementItem `json:"ElementList,omitempty"`
	KeepOriginalSound    string        `json:"KeepOriginalSound,omitempty"`
	CharacterOrientation string        `json:"CharacterOrientation,omitempty"`
	CallbackUrl          string        `json:"CallbackUrl,omitempty"`
	LogoAdd              *int          `json:"LogoAdd,omitempty"`
}

// omniPayload is the SubmitVideoEditKlingJob request body for omni video editing.
type omniPayload struct {
	Prompt         string            `json:"Prompt,omitempty"`
	Model          string            `json:"Model,omitempty"` // kling-video-o1 or kling-v3-omni
	ExternalTaskId string            `json:"ExternalTaskId,omitempty"`
	ImageList      []imageInfo       `json:"ImageList,omitempty"`
	AspectRatio    string            `json:"AspectRatio,omitempty"` // 16:9, 9:16, 1:1
	Duration       *int              `json:"Duration,omitempty"`    // 3-10 seconds
	LogoAdd        *int              `json:"LogoAdd,omitempty"`
	LogoParam      *logoParam        `json:"LogoParam,omitempty"`
	Mode           string            `json:"Mode,omitempty"` // std or pro
	VideoList      []referVideoInfo  `json:"VideoList,omitempty"`
	MultiShot      *bool             `json:"MultiShot,omitempty"`
	ShotType       string            `json:"ShotType,omitempty"` // customize
	MultiPrompt    []multiPromptItem `json:"MultiPrompt,omitempty"`
	ElementList    []elementItem     `json:"ElementList,omitempty"`
	CallbackUrl    string            `json:"CallbackUrl,omitempty"`
	Sound          string            `json:"Sound,omitempty"` // off or on
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

// GetModelList exposes the model names users configure on the channel.
// The "-t" suffix marks these as the Tencent-channel variants so they do NOT
// collide with the official Kling channel's model names (kling-v3 etc.), which
// keeps the official channel's pricing for online users untouched. The suffix
// is stripped before mapping to Tencent's short codes.
//
// "-motion-t" names route to the motion-control API (SubmitMotionControlKlingJob).
// "-omni-t" names route to the omni video editing API (SubmitVideoEditKlingJob).
func (a *TaskAdaptor) GetModelList() []string {
	return []string{
		"kling-v1-t", "kling-v1-5-t", "kling-v1-6-t",
		"kling-v2-master-t", "kling-v2-1-t", "kling-v2-1-master-t",
		"kling-v2-5-turbo-t", "kling-v2-6-t", "kling-v3-t",
		"kling-v2-6-motion-t", "kling-v3-motion-t",
		"kling-video-o1-omni-t", "kling-v3-omni-t",
	}
}

// isMotionControlModel reports whether the model name selects the
// motion-control API (carries a "-motion" marker before the "-t" suffix).
func isMotionControlModel(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, "-t")
	return strings.HasSuffix(n, "-motion")
}

// isOmniModel reports whether the model name selects the omni video editing
// API (carries an "-omni" marker before the "-t" suffix).
func isOmniModel(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, "-t")
	return strings.HasSuffix(n, "-omni")
}

// modelNameToMotionModel maps a motion-control model name to Tencent's Model
// value. Motion control uses full names (kling-v2-6 / kling-v3), not the short
// codes used by image2video.
func modelNameToMotionModel(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, "-t")
	n = strings.TrimSuffix(n, "-motion")
	switch n {
	case "kling-v2-6":
		return "kling-v2-6"
	case "kling-v3", "kling-v3-0":
		return "kling-v3"
	default:
		return n
	}
}

// modelNameToOmniModel maps an omni model name to Tencent's Model value.
// Omni supports kling-video-o1 and kling-v3-omni.
func modelNameToOmniModel(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, "-t")
	n = strings.TrimSuffix(n, "-omni")
	switch n {
	case "kling-video-o1":
		return "kling-video-o1"
	case "kling-v3", "kling-v3-0":
		return "kling-v3-omni"
	default:
		// If it already looks like a full omni model name, use it
		if n == "kling-v3-omni" {
			return "kling-v3-omni"
		}
		// Default to kling-video-o1
		return "kling-video-o1"
	}
}

// modelNameToTencentCode maps the model name to Tencent's Model code.
// The "-t" channel suffix is stripped first; unknown names pass through
// unchanged so a raw code (e.g. "v1.6") also works.
func modelNameToTencentCode(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, "-t")
	switch n {
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
	action := constant.TaskActionGenerate
	if isMotionControlModel(info.OriginModelName) {
		action = constant.TaskActionMotionControl
	} else if isOmniModel(info.OriginModelName) {
		action = constant.TaskActionOmniVideo
	}
	if err := relaycommon.ValidateBasicTaskRequest(c, info, action); err != nil {
		return err
	}
	info.Action = action
	return nil
}

// BuildRequestBody converts the unified task request into the Tencent submit
// payload (image2video, motion-control, or omni depending on action) and caches
// the exact bytes for TC3 signing.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req := v.(relaycommon.TaskSubmitReq)

	var payload any
	var err error
	switch info.Action {
	case constant.TaskActionMotionControl:
		payload, err = a.convertToMotionPayload(&req, info)
	case constant.TaskActionOmniVideo:
		payload, err = a.convertToOmniPayload(&req, info)
	default:
		payload, err = a.convertToSubmitPayload(&req, info)
	}
	if err != nil {
		return nil, err
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	// Cache the marshaled body so BuildRequestHeader can sign the exact bytes.
	c.Set(ctxKeyBody, data)
	switch p := payload.(type) {
	case *submitPayload:
		logoAdd := "<nil>"
		if p.LogoAdd != nil {
			logoAdd = strconv.Itoa(*p.LogoAdd)
		}
		logger.LogInfo(c.Request.Context(), fmt.Sprintf(
			"tencentvideo submit payload: action=%s model=%s mode=%s duration=%s sound=%s logo_add=%s has_image=%t has_image_tail=%t",
			actionSubmit, p.Model, p.Mode, p.Duration, p.Sound, logoAdd, p.Image != nil, p.ImageTail != nil,
		))
	case *motionPayload:
		logoAdd := "<nil>"
		if p.LogoAdd != nil {
			logoAdd = strconv.Itoa(*p.LogoAdd)
		}
		logger.LogInfo(c.Request.Context(), fmt.Sprintf(
			"tencentvideo submit payload: action=%s model=%s mode=%s logo_add=%s has_image=%t has_video=%t",
			actionMotionSubmit, p.Model, p.Mode, logoAdd, strings.TrimSpace(p.Image) != "", strings.TrimSpace(p.Video) != "",
		))
	case *omniPayload:
		logoAdd := "<nil>"
		if p.LogoAdd != nil {
			logoAdd = strconv.Itoa(*p.LogoAdd)
		}
		duration := "<nil>"
		if p.Duration != nil {
			duration = strconv.Itoa(*p.Duration)
		}
		logger.LogInfo(c.Request.Context(), fmt.Sprintf(
			"tencentvideo submit payload: action=%s model=%s mode=%s duration=%s aspect_ratio=%s logo_add=%s image_list_count=%d video_list_count=%d",
			actionOmniSubmit, p.Model, p.Mode, duration, p.AspectRatio, logoAdd, len(p.ImageList), len(p.VideoList),
		))
	}
	return bytes.NewReader(data), nil
}

// convertToMotionPayload builds a SubmitMotionControlKlingJob body. Image and
// Video are required; both are plain URL strings. Video comes from metadata
// ("video" or "Video") since the unified request has no top-level video field.
func (a *TaskAdaptor) convertToMotionPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*motionPayload, error) {
	p := motionPayload{
		Model:  modelNameToMotionModel(info.UpstreamModelName),
		Prompt: req.Prompt,
		Mode:   taskcommon.DefaultString(req.Mode, "std"),
	}
	if len(req.Images) > 0 && strings.TrimSpace(req.Images[0]) != "" {
		p.Image = req.Images[0]
	}
	// Optional region override (used in TC3 credential scope, not the body).
	if v, ok := req.Metadata["region"]; ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			a.region = strings.TrimSpace(s)
		}
	}
	// metadata supplies Video and any other PascalCase Tencent fields
	// (Video, KeepOriginalSound, CharacterOrientation, ElementList, LogoAdd).
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &p); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}
	// Re-apply the channel-mapped model so metadata cannot bypass billing.
	p.Model = modelNameToMotionModel(info.UpstreamModelName)

	// Always disable Tencent's AI-generated watermark. Tencent defaults LogoAdd
	// to 1 (adds a logo) when omitted, so we force 0 regardless of client input.
	zero := 0
	p.LogoAdd = &zero

	if strings.TrimSpace(p.Image) == "" {
		return nil, fmt.Errorf("image is required for motion control")
	}
	if strings.TrimSpace(p.Video) == "" {
		return nil, fmt.Errorf("video is required for motion control (pass it in metadata.Video)")
	}
	return &p, nil
}

// convertToOmniPayload builds a SubmitVideoEditKlingJob body for omni video editing.
func (a *TaskAdaptor) convertToOmniPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*omniPayload, error) {
	modelName := modelNameToOmniModel(info.UpstreamModelName)

	p := omniPayload{
		Model:       modelName,
		Prompt:      req.Prompt,
		Mode:        taskcommon.DefaultString(req.Mode, "pro"),
		AspectRatio: a.getAspectRatio(req.Size),
	}

	// Duration - convert to pointer
	if req.Duration > 0 {
		duration := taskcommon.DefaultInt(req.Duration, 5)
		p.Duration = &duration
	}

	// Process images into ImageList format
	// Prefer ImageList (with type info) over Images (plain URLs)
	if len(req.ImageList) > 0 {
		for _, img := range req.ImageList {
			if strings.TrimSpace(img.ImageURL) != "" {
				p.ImageList = append(p.ImageList, imageInfo{
					ImageUrl: img.ImageURL,
					Type:     img.Type, // Preserve type (first_frame, end_frame)
				})
			}
		}
	} else if len(req.Images) > 0 {
		// Fallback: if only plain URLs provided, use them without type
		for _, imgUrl := range req.Images {
			if strings.TrimSpace(imgUrl) != "" {
				p.ImageList = append(p.ImageList, imageInfo{
					ImageUrl: imgUrl,
					// Type not set - may cause issues if provider requires it
				})
			}
		}
	}

	// Optional region override (used in TC3 credential scope, not the body).
	if v, ok := req.Metadata["region"]; ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			a.region = strings.TrimSpace(s)
		}
	}

	// metadata may override / supply any PascalCase Tencent field
	// (e.g. VideoList, ElementList, MultiShot, ShotType, MultiPrompt, Sound, LogoParam, CallbackUrl).
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &p); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	// Re-apply the channel-mapped model so metadata cannot bypass billing.
	p.Model = modelName

	// Always disable Tencent's AI-generated watermark.
	zero := 0
	p.LogoAdd = &zero

	return &p, nil
}

// getAspectRatio converts size string to Tencent's AspectRatio format.
func (a *TaskAdaptor) getAspectRatio(size string) string {
	switch size {
	case "1024x1024", "512x512":
		return "1:1"
	case "1280x720", "1920x1080":
		return "16:9"
	case "720x1280", "1080x1920":
		return "9:16"
	default:
		return "16:9" // Default for omni
	}
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

	// Always disable Tencent's AI-generated watermark. Tencent defaults LogoAdd
	// to 1 (adds a logo) when omitted, so we force 0 regardless of client input.
	zero := 0
	p.LogoAdd = &zero

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
	action := actionSubmit
	switch info.Action {
	case constant.TaskActionMotionControl:
		action = actionMotionSubmit
	case constant.TaskActionOmniVideo:
		action = actionOmniSubmit
	}
	a.applyTC3Headers(req, action, payload, secretId, secretKey, a.region)
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

// FetchTask queries the Describe endpoint (image2video, motion-control, or omni,
// chosen by the task's action) for the given JobId.
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

	action := actionDescribe
	if act, ok := body["action"].(string); ok {
		switch act {
		case constant.TaskActionMotionControl:
			action = actionMotionDescribe
		case constant.TaskActionOmniVideo:
			action = actionOmniDescribe
		}
	}

	payload, err := common.Marshal(map[string]string{"JobId": taskID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://%s/", a.host()), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	a.applyTC3Headers(req, action, payload, secretId, secretKey, region)

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
