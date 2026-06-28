package kling

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// ============================
// Request / Response structures
// ============================

type TrajectoryPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type DynamicMask struct {
	Mask         string            `json:"mask,omitempty"`
	Trajectories []TrajectoryPoint `json:"trajectories,omitempty"`
}

type MultiPromptItem struct {
	Index    int    `json:"index"`
	Prompt   string `json:"prompt"`
	Duration string `json:"duration"`
}

type CameraConfig struct {
	Horizontal float64 `json:"horizontal,omitempty"`
	Vertical   float64 `json:"vertical,omitempty"`
	Pan        float64 `json:"pan,omitempty"`
	Tilt       float64 `json:"tilt,omitempty"`
	Roll       float64 `json:"roll,omitempty"`
	Zoom       float64 `json:"zoom,omitempty"`
}

type CameraControl struct {
	Type   string        `json:"type,omitempty"`
	Config *CameraConfig `json:"config,omitempty"`
}

type ElementItem struct {
	ElementId int64 `json:"element_id"`
}

type ImageItem struct {
	ImageUrl string `json:"image_url"`
	Type     string `json:"type,omitempty"` // first_frame or end_frame
}

type VideoItem struct {
	VideoUrl          string `json:"video_url"`
	ReferType         string `json:"refer_type,omitempty"`         // feature or base
	KeepOriginalSound string `json:"keep_original_sound,omitempty"` // yes or no
}

type WatermarkInfo struct {
	Enabled bool `json:"enabled"`
}

type requestPayload struct {
	Prompt         string         `json:"prompt,omitempty"`
	Image          string         `json:"image,omitempty"`
	ImageUrl       string         `json:"image_url,omitempty"`
	ImageTail      string         `json:"image_tail,omitempty"`
	NegativePrompt string         `json:"negative_prompt,omitempty"`
	Mode           string         `json:"mode,omitempty"`
	Duration       string         `json:"duration,omitempty"`
	AspectRatio    string         `json:"aspect_ratio,omitempty"`
	ModelName      string         `json:"model_name,omitempty"`
	Model          string         `json:"model,omitempty"` // Compatible with upstreams that only recognize "model"
	CfgScale       float64        `json:"cfg_scale,omitempty"`
	Sound          string            `json:"sound,omitempty"`
	MultiShot      *bool             `json:"multi_shot,omitempty"` // Use pointer to distinguish false from absent
	ShotType       string            `json:"shot_type,omitempty"`
	MultiPrompt    []MultiPromptItem `json:"multi_prompt,omitempty"`
	StaticMask     string            `json:"static_mask,omitempty"`
	DynamicMasks   []DynamicMask  `json:"dynamic_masks,omitempty"`
	CameraControl  *CameraControl `json:"camera_control,omitempty"`
	// motion-control specific fields
	VideoUrl             string        `json:"video_url,omitempty"`
	CharacterOrientation string        `json:"character_orientation,omitempty"`
	KeepOriginalSound    string        `json:"keep_original_sound,omitempty"`
	// Omni Video specific fields
	ImageList      []ImageItem    `json:"image_list,omitempty"`
	ElementList    []ElementItem  `json:"element_list,omitempty"`
	VideoList      []VideoItem    `json:"video_list,omitempty"`
	WatermarkInfo  *WatermarkInfo `json:"watermark_info,omitempty"`
	CallbackUrl    string         `json:"callback_url,omitempty"`
	ExternalTaskId string         `json:"external_task_id,omitempty"`
}

type responsePayload struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	TaskId    string `json:"task_id"`
	RequestId string `json:"request_id"`
	Data      struct {
		TaskId        string `json:"task_id"`
		TaskStatus    string `json:"task_status"`
		TaskStatusMsg string `json:"task_status_msg"`
		TaskInfo      struct {
			ExternalTaskId string `json:"external_task_id"`
		} `json:"task_info"`
		WatermarkInfo struct {
			Enabled bool `json:"enabled"`
		} `json:"watermark_info"`
		TaskResult struct {
			Videos []struct {
				Id           string `json:"id"`
				Url          string `json:"url"`
				WatermarkUrl string `json:"watermark_url"`
				Duration     string `json:"duration"`
			} `json:"videos"`
			Images []struct {
				Index        int    `json:"index"`
				Url          string `json:"url"`
				WatermarkUrl string `json:"watermark_url"`
			} `json:"images"`
		} `json:"task_result"`
		CreatedAt          int64  `json:"created_at"`
		UpdatedAt          int64  `json:"updated_at"`
		FinalUnitDeduction string `json:"final_unit_deduction"`
	} `json:"data"`
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
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey

	// apiKey format:
	// - New format (official): "api-key-kling-xxx" (used directly as Bearer token)
	// - Legacy format: "access_key|secret_key" (generates JWT token)
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	if strings.Contains(c.Request.URL.Path, "motion-control") {
		return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionMotionControl)
	}
	if strings.Contains(c.Request.URL.Path, "omni-video") {
		return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionOmniVideo)
	}
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

// BuildRequestURL constructs the upstream URL.
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	var path string
	switch info.Action {
	case constant.TaskActionMotionControl:
		path = "/v1/videos/motion-control"
	case constant.TaskActionOmniVideo:
		path = "/v1/videos/omni-video"
	case constant.TaskActionGenerate:
		path = "/v1/videos/image2video"
	default:
		path = "/v1/videos/text2video"
	}

	if isNewAPIRelay(info.ApiKey) {
		return fmt.Sprintf("%s/kling%s", a.baseURL, path), nil
	}

	return fmt.Sprintf("%s%s", a.baseURL, path), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	token, err := a.createJWTToken()
	if err != nil {
		return fmt.Errorf("failed to create JWT token: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "kling-sdk/1.0")
	return nil
}

// BuildRequestBody converts request into Kling specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req := v.(relaycommon.TaskSubmitReq)

	body, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		return nil, err
	}
	if info.Action != constant.TaskActionMotionControl && body.Image == "" && body.ImageTail == "" {
		c.Set("action", constant.TaskActionTextGenerate)
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	if action := c.GetString("action"); action != "" {
		info.Action = action
	}
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}

	var kResp responsePayload
	err = common.Unmarshal(responseBody, &kResp)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}
	if kResp.Code != 0 {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("%s", kResp.Message), "task_failed", http.StatusBadRequest)
		return
	}
	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)
	return kResp.Data.TaskId, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}
	action, ok := body["action"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid action")
	}
	var path string
	switch action {
	case constant.TaskActionMotionControl:
		path = "/v1/videos/motion-control"
	case constant.TaskActionOmniVideo:
		path = "/v1/videos/omni-video"
	case constant.TaskActionGenerate:
		path = "/v1/videos/image2video"
	default:
		path = "/v1/videos/text2video"
	}
	url := fmt.Sprintf("%s%s/%s", baseUrl, path, taskID)
	if isNewAPIRelay(key) {
		url = fmt.Sprintf("%s/kling%s/%s", baseUrl, path, taskID)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	token, err := a.createJWTTokenWithKey(key)
	if err != nil {
		token = key
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "kling-sdk/1.0")

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{"kling-v1", "kling-v1-6", "kling-v2-6", "kling-v2-master", "kling-v3"}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "kling"
}

// ============================
// helpers
// ============================

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*requestPayload, error) {
	modelName := info.UpstreamModelName
	if modelName == "" {
		modelName = "kling-v1"
	}
	// Support model_name field (Kling official) with fallback to model
	if req.ModelName != "" {
		modelName = req.ModelName
	}

	// Handle motion-control
	if info.Action == constant.TaskActionMotionControl {
		// Default model for motion-control
		if modelName == "kling-v1" || modelName == "" {
			modelName = "kling-v2-6"
		}

		r := requestPayload{
			Prompt:               req.Prompt,
			ModelName:            modelName,
			Model:                modelName,
			VideoUrl:             req.VideoUrl,
			CharacterOrientation: req.CharacterOrientation,
			KeepOriginalSound:    req.KeepOriginalSound,
			ImageUrl:             req.ImageUrl,
		}
		// metadata can still override (for backward compatibility)
		if err := taskcommon.UnmarshalMetadata(req.Metadata, &r); err != nil {
			return nil, errors.Wrap(err, "unmarshal metadata failed")
		}
		// Re-apply the channel-mapped model name
		r.ModelName = modelName
		r.Model = modelName
		return &r, nil
	}

	// Handle omni-video
	if info.Action == constant.TaskActionOmniVideo {
		// Default model for omni-video
		if modelName == "kling-v1" {
			modelName = "kling-video-o1"
		}

		r := requestPayload{
			Prompt:         req.Prompt,
			ModelName:      modelName,
			Model:          modelName,
			Mode:           taskcommon.DefaultString(req.Mode, "pro"),
			Duration:       fmt.Sprintf("%d", taskcommon.DefaultInt(req.Duration, 5)),
			AspectRatio:    a.getAspectRatio(req.Size, req.AspectRatio),
			ImageList:      convertImageList(req.ImageList),
			ElementList:    req.ElementList,
			VideoList:      req.VideoList,
			WatermarkInfo:  req.WatermarkInfo,
			CallbackUrl:    req.CallbackUrl,
			ExternalTaskId: req.ExternalTaskId,
		}

		// metadata can still override (for backward compatibility)
		if err := taskcommon.UnmarshalMetadata(req.Metadata, &r); err != nil {
			return nil, errors.Wrap(err, "unmarshal metadata failed")
		}

		// Re-apply the channel-mapped model name after metadata unmarshal
		r.ModelName = modelName
		r.Model = modelName

		return &r, nil
	}

	// Default: image2video / text2video
	r := requestPayload{
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Image:          req.Image,
		ImageUrl:       req.ImageUrl,
		ImageTail:      req.ImageTail,
		Mode:           taskcommon.DefaultString(req.Mode, "std"),
		Duration:       fmt.Sprintf("%d", taskcommon.DefaultInt(req.Duration, 5)),
		AspectRatio:    a.getAspectRatio(req.Size, req.AspectRatio),
		ModelName:      modelName,
		Model:          modelName,
		CfgScale:       0.5,
		Sound:          req.Sound,
		ShotType:       req.ShotType,
		StaticMask:     req.StaticMask,
		DynamicMasks:   req.DynamicMasks,
		CameraControl:  req.CameraControl,
		MultiPrompt:    req.MultiPrompt,
		CallbackUrl:    req.CallbackUrl,
		ExternalTaskId: req.ExternalTaskId,
	}

	// Handle multi_shot field (string "true"/"false" to *bool)
	if req.MultiShot != "" {
		multiShotBool := req.MultiShot == "true"
		r.MultiShot = &multiShotBool
	}

	// Handle cfg_scale from top-level
	if req.CfgScale != nil {
		r.CfgScale = *req.CfgScale
	}

	// Handle dynamic_masks - initialize empty array if nil
	if r.DynamicMasks == nil {
		r.DynamicMasks = []DynamicMask{}
	}

	// metadata can still override any field (for backward compatibility)
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	// Re-apply the channel-mapped model name after metadata unmarshal
	r.ModelName = modelName
	r.Model = modelName
	return &r, nil
}

// convertImageList converts TaskImageInfo to ImageItem
func convertImageList(taskImages []relaycommon.TaskImageInfo) []ImageItem {
	if len(taskImages) == 0 {
		return nil
	}
	result := make([]ImageItem, len(taskImages))
	for i, img := range taskImages {
		result[i] = ImageItem{
			ImageUrl: img.ImageURL,
			Type:     img.Type,
		}
	}
	return result
}

func (a *TaskAdaptor) getAspectRatio(size string, aspectRatio string) string {
	// If aspect_ratio is provided directly, use it
	if aspectRatio != "" {
		return aspectRatio
	}
	// Otherwise, derive from size
	switch size {
	case "1024x1024", "512x512":
		return "1:1"
	case "1280x720", "1920x1080":
		return "16:9"
	case "720x1280", "1080x1920":
		return "9:16"
	default:
		return "1:1"
	}
}

// ============================
// JWT helpers
// ============================

func (a *TaskAdaptor) createJWTToken() (string, error) {
	return a.createJWTTokenWithKey(a.apiKey)
}

func (a *TaskAdaptor) createJWTTokenWithKey(apiKey string) (string, error) {
	if isNewAPIRelay(apiKey) {
		return apiKey, nil // new api relay
	}

	// Check if it's the new official API key format (Bearer token directly)
	// New format example: api-key-kling-mykbR7zMT1pcFcJrY5um1UA_tretIj9ba2W-8xsgBrQ
	if strings.HasPrefix(apiKey, "api-key-kling-") {
		return apiKey, nil // Use the API key directly as Bearer token
	}

	// Check if it's the old format with pipe separator
	if !strings.Contains(apiKey, "|") {
		// If no pipe separator and not prefixed with api-key-kling-, assume it's a direct token
		return apiKey, nil
	}

	// Legacy format: AccessKey|SecretKey - generate JWT token
	keyParts := strings.Split(apiKey, "|")
	if len(keyParts) != 2 {
		return "", errors.New("invalid api_key, required format is accessKey|secretKey or api-key-kling-xxx")
	}
	accessKey := strings.TrimSpace(keyParts[0])
	secretKey := strings.TrimSpace(keyParts[1])

	if secretKey == "" {
		// Only access key provided, use it directly
		return accessKey, nil
	}

	// Generate JWT token for legacy format
	now := time.Now().Unix()
	claims := jwt.MapClaims{
		"iss": accessKey,
		"exp": now + 1800, // 30 minutes
		"nbf": now - 5,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["typ"] = "JWT"
	return token.SignedString([]byte(secretKey))
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	taskInfo := &relaycommon.TaskInfo{}
	resPayload := responsePayload{}
	err := common.Unmarshal(respBody, &resPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}
	taskInfo.Code = resPayload.Code
	taskInfo.TaskID = resPayload.Data.TaskId
	taskInfo.Reason = resPayload.Data.TaskStatusMsg
	//任务状态，枚举值：submitted（已提交）、processing（处理中）、succeed（成功）、failed（失败）
	status := resPayload.Data.TaskStatus
	switch status {
	case "submitted":
		taskInfo.Status = model.TaskStatusSubmitted
	case "processing":
		taskInfo.Status = model.TaskStatusInProgress
	case "succeed":
		taskInfo.Status = model.TaskStatusSuccess
		if videos := resPayload.Data.TaskResult.Videos; len(videos) > 0 {
			video := videos[0]
			taskInfo.Url = video.Url
		}
		if tokens, err := strconv.ParseFloat(resPayload.Data.FinalUnitDeduction, 64); err == nil {
			rounded := int(math.Ceil(tokens))
			if rounded > 0 {
				taskInfo.CompletionTokens = rounded
				taskInfo.TotalTokens = rounded
			}
		}
	case "failed":
		taskInfo.Status = model.TaskStatusFailure
	default:
		return nil, fmt.Errorf("unknown task status: %s", status)
	}
	return taskInfo, nil
}

func isNewAPIRelay(apiKey string) bool {
	return strings.HasPrefix(apiKey, "sk-")
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var klingResp responsePayload
	if err := common.Unmarshal(originTask.Data, &klingResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal kling task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = klingResp.Data.CreatedAt
	openAIVideo.CompletedAt = klingResp.Data.UpdatedAt

	if len(klingResp.Data.TaskResult.Videos) > 0 {
		video := klingResp.Data.TaskResult.Videos[0]
		if video.Url != "" {
			openAIVideo.SetMetadata("url", video.Url)
		}
		if video.Duration != "" {
			openAIVideo.Seconds = video.Duration
		}
	}

	if klingResp.Code != 0 && klingResp.Message != "" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: klingResp.Message,
			Code:    fmt.Sprintf("%d", klingResp.Code),
		}
	}

	// https://app.klingai.com/cn/dev/document-api/apiReference/model/textToVideo
	if data := klingResp.Data; data.TaskStatus == "failed" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: data.TaskStatusMsg,
		}
	}
	return common.Marshal(openAIVideo)
}

// AdjustBillingOnComplete implements differential settlement for Kling tasks.
// Kling returns final_unit_deduction (in RMB) which represents the actual cost.
// We convert this to quota units and return it to trigger delta settlement.
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	// Only adjust billing for successful tasks
	if taskResult.Status != model.TaskStatusSuccess {
		return 0
	}

	// Kling returns final_unit_deduction as the actual cost in RMB (1 credit = ¥1)
	// This value is already stored in taskResult.CompletionTokens
	if taskResult.CompletionTokens <= 0 {
		return 0
	}

	// Convert RMB to quota units using the system's exchange rate
	// quota = RMB × (QuotaPerUnit ÷ USDExchangeRate) × groupRatio
	//
	// Example with USDExchangeRate=1:
	//   - RMB cost: 3 yuan
	//   - quota = 3 × (500,000 ÷ 1) × 1 = 1,500,000
	//   - Display: 1,500,000 ÷ 500,000 × 1 = 3 CNY ✓
	//
	// Example with USDExchangeRate=7.3 (realistic rate):
	//   - RMB cost: 3 yuan
	//   - quota = 3 × (500,000 ÷ 7.3) × 1 ≈ 205,479
	//   - Display: 205,479 ÷ 500,000 × 7.3 ≈ 3 CNY ✓
	rmbCost := float64(taskResult.CompletionTokens)

	usdExchangeRate := operation_setting.USDExchangeRate
	if usdExchangeRate <= 0 {
		usdExchangeRate = 1.0 // fallback to 1:1 if not set
	}

	// Get group ratio from billing context
	groupRatio := 1.0
	if bc := task.PrivateData.BillingContext; bc != nil && bc.GroupRatio > 0 {
		groupRatio = bc.GroupRatio
	}

	actualQuota := int(math.Ceil(rmbCost * common.QuotaPerUnit / usdExchangeRate * groupRatio))

	return actualQuota
}
