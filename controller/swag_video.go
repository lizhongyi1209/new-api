package controller

import (
	"github.com/gin-gonic/gin"
)

// VideoGenerations
// @Summary 生成视频
// @Description 调用视频生成接口生成视频
// @Description 支持多种视频生成服务：
// @Description - 可灵AI (Kling): https://app.klingai.com/cn/dev/document-api/apiReference/commonInfo
// @Description - 即梦 (Jimeng): https://www.volcengine.com/docs/85621/1538636
// @Tags Video
// @Accept json
// @Produce json
// @Param Authorization header string true "用户认证令牌 (Aeess-Token: sk-xxxx)"
// @Param request body dto.VideoRequest true "视频生成请求参数"
// @Failure 400 {object} dto.OpenAIError "请求参数错误"
// @Failure 401 {object} dto.OpenAIError "未授权"
// @Failure 403 {object} dto.OpenAIError "无权限"
// @Failure 500 {object} dto.OpenAIError "服务器内部错误"
// @Router /v1/video/generations [post]
func VideoGenerations(c *gin.Context) {
}

// VideoGenerationsTaskId
// @Summary 查询视频
// @Description 根据任务ID查询视频生成任务的状态和结果
// @Tags Video
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param task_id path string true "Task ID"
// @Success 200 {object} dto.VideoTaskResponse "任务状态和结果"
// @Failure 400 {object} dto.OpenAIError "请求参数错误"
// @Failure 401 {object} dto.OpenAIError "未授权"
// @Failure 403 {object} dto.OpenAIError "无权限"
// @Failure 500 {object} dto.OpenAIError "服务器内部错误"
// @Router /v1/video/generations/{task_id} [get]
func VideoGenerationsTaskId(c *gin.Context) {
}

// KlingText2VideoGenerations
// @Summary 可灵文生视频
// @Description 调用可灵AI文生视频接口，生成视频内容
// @Tags Video
// @Accept json
// @Produce json
// @Param Authorization header string true "用户认证令牌 (Aeess-Token: sk-xxxx)"
// @Param request body KlingText2VideoRequest true "视频生成请求参数"
// @Success 200 {object} dto.VideoTaskResponse "任务状态和结果"
// @Failure 400 {object} dto.OpenAIError "请求参数错误"
// @Failure 401 {object} dto.OpenAIError "未授权"
// @Failure 403 {object} dto.OpenAIError "无权限"
// @Failure 500 {object} dto.OpenAIError "服务器内部错误"
// @Router /kling/v1/videos/text2video [post]
func KlingText2VideoGenerations(c *gin.Context) {
}

type KlingText2VideoRequest struct {
	ModelName      string              `json:"model_name,omitempty" example:"kling-v1"`
	Prompt         string              `json:"prompt" binding:"required" example:"A cat playing piano in the garden"`
	NegativePrompt string              `json:"negative_prompt,omitempty" example:"blurry, low quality"`
	CfgScale       float64             `json:"cfg_scale,omitempty" example:"0.7"`
	Mode           string              `json:"mode,omitempty" example:"std"`
	CameraControl  *KlingCameraControl `json:"camera_control,omitempty"`
	AspectRatio    string              `json:"aspect_ratio,omitempty" example:"16:9"`
	Duration       string              `json:"duration,omitempty" example:"5"`
	CallbackURL    string              `json:"callback_url,omitempty" example:"https://your.domain/callback"`
	ExternalTaskId string              `json:"external_task_id,omitempty" example:"custom-task-001"`
}

type KlingCameraControl struct {
	Type   string             `json:"type,omitempty" example:"simple"`
	Config *KlingCameraConfig `json:"config,omitempty"`
}

type KlingCameraConfig struct {
	Horizontal float64 `json:"horizontal,omitempty" example:"2.5"`
	Vertical   float64 `json:"vertical,omitempty" example:"0"`
	Pan        float64 `json:"pan,omitempty" example:"0"`
	Tilt       float64 `json:"tilt,omitempty" example:"0"`
	Roll       float64 `json:"roll,omitempty" example:"0"`
	Zoom       float64 `json:"zoom,omitempty" example:"0"`
}

// KlingImage2VideoGenerations
// @Summary 可灵官方-图生视频
// @Description 调用可灵AI图生视频接口，生成视频内容
// @Tags Video
// @Accept json
// @Produce json
// @Param Authorization header string true "用户认证令牌 (Aeess-Token: sk-xxxx)"
// @Param request body KlingImage2VideoRequest true "图生视频请求参数"
// @Success 200 {object} dto.VideoTaskResponse "任务状态和结果"
// @Failure 400 {object} dto.OpenAIError "请求参数错误"
// @Failure 401 {object} dto.OpenAIError "未授权"
// @Failure 403 {object} dto.OpenAIError "无权限"
// @Failure 500 {object} dto.OpenAIError "服务器内部错误"
// @Router /kling/v1/videos/image2video [post]
func KlingImage2VideoGenerations(c *gin.Context) {
}

type KlingImage2VideoRequest struct {
	ModelName      string              `json:"model_name,omitempty" example:"kling-v2-master"`
	Image          string              `json:"image" binding:"required" example:"https://h2.inkwai.com/bs2/upload-ylab-stunt/se/ai_portal_queue_mmu_image_upscale_aiweb/3214b798-e1b4-4b00-b7af-72b5b0417420_raw_image_0.jpg"`
	Prompt         string              `json:"prompt,omitempty" example:"A cat playing piano in the garden"`
	NegativePrompt string              `json:"negative_prompt,omitempty" example:"blurry, low quality"`
	CfgScale       float64             `json:"cfg_scale,omitempty" example:"0.7"`
	Mode           string              `json:"mode,omitempty" example:"std"`
	CameraControl  *KlingCameraControl `json:"camera_control,omitempty"`
	AspectRatio    string              `json:"aspect_ratio,omitempty" example:"16:9"`
	Duration       string              `json:"duration,omitempty" example:"5"`
	CallbackURL    string              `json:"callback_url,omitempty" example:"https://your.domain/callback"`
	ExternalTaskId string              `json:"external_task_id,omitempty" example:"custom-task-002"`
}

// KlingImage2videoTaskId godoc
// @Summary 可灵任务查询--图生视频
// @Description Query the status and result of a Kling video generation task by task ID
// @Tags Origin
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Router /kling/v1/videos/image2video/{task_id} [get]
func KlingImage2videoTaskId(c *gin.Context) {}

// KlingText2videoTaskId godoc
// @Summary 可灵任务查询--文生视频
// @Description Query the status and result of a Kling text-to-video generation task by task ID
// @Tags Origin
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Router /kling/v1/videos/text2video/{task_id} [get]
func KlingText2videoTaskId(c *gin.Context) {}

// KlingOmniVideoGenerations
// @Summary 可灵 Omni Video 生成
// @Description 调用可灵 Omni Video 接口，支持图片/元素引用、视频编辑、视频引用等功能
// @Tags Video
// @Accept json
// @Produce json
// @Param Authorization header string true "用户认证令牌 (Access-Token: sk-xxxx)"
// @Param request body KlingOmniVideoRequest true "Omni Video 生成请求参数"
// @Success 200 {object} dto.VideoTaskResponse "任务状态和结果"
// @Failure 400 {object} dto.OpenAIError "请求参数错误"
// @Failure 401 {object} dto.OpenAIError "未授权"
// @Failure 403 {object} dto.OpenAIError "无权限"
// @Failure 500 {object} dto.OpenAIError "服务器内部错误"
// @Router /kling/v1/videos/omni-video [post]
func KlingOmniVideoGenerations(c *gin.Context) {
}

type KlingOmniVideoRequest struct {
	ModelName      string                   `json:"model_name,omitempty" example:"kling-video-o1"`
	MultiShot      *bool                    `json:"multi_shot,omitempty" example:"false"`
	ShotType       string                   `json:"shot_type,omitempty" example:"customize"`
	Prompt         string                   `json:"prompt,omitempty" example:"Make the person in <<<image_1>>> wave to the camera"`
	MultiPrompt    []KlingMultiPromptItem   `json:"multi_prompt,omitempty"`
	ImageList      []KlingImageItem         `json:"image_list,omitempty"`
	ElementList    []KlingElementItem       `json:"element_list,omitempty"`
	VideoList      []KlingVideoItem         `json:"video_list,omitempty"`
	Sound          string                   `json:"sound,omitempty" example:"off"`
	Mode           string                   `json:"mode,omitempty" example:"pro"`
	AspectRatio    string                   `json:"aspect_ratio,omitempty" example:"16:9"`
	Duration       string                   `json:"duration,omitempty" example:"5"`
	WatermarkInfo  *KlingWatermarkInfo      `json:"watermark_info,omitempty"`
	CallbackURL    string                   `json:"callback_url,omitempty" example:"https://your.domain/callback"`
	ExternalTaskId string                   `json:"external_task_id,omitempty" example:"custom-task-003"`
}

type KlingMultiPromptItem struct {
	Index    int    `json:"index" example:"1"`
	Prompt   string `json:"prompt" example:"A person walking"`
	Duration string `json:"duration" example:"5"`
}

type KlingImageItem struct {
	ImageUrl string `json:"image_url" example:"https://example.com/image.jpg"`
	Type     string `json:"type,omitempty" example:"first_frame"`
}

type KlingElementItem struct {
	ElementId int64 `json:"element_id" example:"123456"`
}

type KlingVideoItem struct {
	VideoUrl          string `json:"video_url" example:"https://example.com/video.mp4"`
	ReferType         string `json:"refer_type,omitempty" example:"base"`
	KeepOriginalSound string `json:"keep_original_sound,omitempty" example:"yes"`
}

type KlingWatermarkInfo struct {
	Enabled bool `json:"enabled" example:"false"`
}

// KlingOmniVideoTaskId godoc
// @Summary 可灵任务查询--Omni Video
// @Description Query the status and result of a Kling Omni Video generation task by task ID
// @Tags Origin
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Router /kling/v1/videos/omni-video/{task_id} [get]
func KlingOmniVideoTaskId(c *gin.Context) {}

// KlingMotionControlGenerations
// @Summary 可灵动作控制
// @Description 调用可灵 Motion Control 接口，根据参考图片和动作视频生成视频
// @Tags Video
// @Accept json
// @Produce json
// @Param Authorization header string true "用户认证令牌 (Access-Token: sk-xxxx)"
// @Param request body KlingMotionControlRequest true "Motion Control 生成请求参数"
// @Success 200 {object} dto.VideoTaskResponse "任务状态和结果"
// @Failure 400 {object} dto.OpenAIError "请求参数错误"
// @Failure 401 {object} dto.OpenAIError "未授权"
// @Failure 403 {object} dto.OpenAIError "无权限"
// @Failure 500 {object} dto.OpenAIError "服务器内部错误"
// @Router /kling/v1/videos/motion-control [post]
func KlingMotionControlGenerations(c *gin.Context) {
}

type KlingMotionControlRequest struct {
	ModelName            string                 `json:"model_name,omitempty" example:"kling-v2-6"`
	Prompt               string                 `json:"prompt,omitempty" example:"The girl is wearing a loose gray T-shirt"`
	ImageUrl             string                 `json:"image_url" binding:"required" example:"https://example.com/reference.jpg"`
	VideoUrl             string                 `json:"video_url" binding:"required" example:"https://example.com/motion.mp4"`
	ElementList          []KlingElementItem     `json:"element_list,omitempty"`
	KeepOriginalSound    string                 `json:"keep_original_sound,omitempty" example:"yes"`
	CharacterOrientation string                 `json:"character_orientation" binding:"required" example:"image"`
	Mode                 string                 `json:"mode" binding:"required" example:"pro"`
	WatermarkInfo        *KlingWatermarkInfo    `json:"watermark_info,omitempty"`
	CallbackURL          string                 `json:"callback_url,omitempty" example:"https://your.domain/callback"`
	ExternalTaskId       string                 `json:"external_task_id,omitempty" example:"custom-task-004"`
}

// KlingMotionControlTaskId godoc
// @Summary 可灵任务查询--Motion Control
// @Description Query the status and result of a Kling Motion Control generation task by task ID
// @Tags Origin
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Router /kling/v1/videos/motion-control/{task_id} [get]
func KlingMotionControlTaskId(c *gin.Context) {}
