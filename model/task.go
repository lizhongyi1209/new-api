package model

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	commonRelay "github.com/QuantumNous/new-api/relay/common"
)

type TaskStatus string

func (t TaskStatus) ToVideoStatus() string {
	var status string
	switch t {
	case TaskStatusQueued, TaskStatusSubmitted:
		status = dto.VideoStatusQueued
	case TaskStatusInProgress:
		status = dto.VideoStatusInProgress
	case TaskStatusSuccess:
		status = dto.VideoStatusCompleted
	case TaskStatusFailure:
		status = dto.VideoStatusFailed
	default:
		status = dto.VideoStatusUnknown // Default fallback
	}
	return status
}

const (
	TaskStatusNotStart   TaskStatus = "NOT_START"
	TaskStatusSubmitted             = "SUBMITTED"
	TaskStatusQueued                = "QUEUED"
	TaskStatusInProgress            = "IN_PROGRESS"
	TaskStatusFailure               = "FAILURE"
	TaskStatusSuccess               = "SUCCESS"
	TaskStatusUnknown               = "UNKNOWN"
)

// TaskRefundLegacyCutoff separates legacy timeout tasks that intentionally
// do not receive automatic refunds from tasks covered by reconciliation.
const TaskRefundLegacyCutoff int64 = 1740182400 // 2025-02-22 00:00:00 UTC

type Task struct {
	ID         int64                 `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	CreatedAt  int64                 `json:"created_at" gorm:"index"`
	UpdatedAt  int64                 `json:"updated_at"`
	TaskID     string                `json:"task_id" gorm:"type:varchar(191);index"` // 第三方id，不一定有/ song id\ Task id
	Platform   constant.TaskPlatform `json:"platform" gorm:"type:varchar(30);index"` // 平台
	UserId     int                   `json:"user_id" gorm:"index"`
	Group      string                `json:"group" gorm:"type:varchar(50)"` // 修正计费用
	ChannelId  int                   `json:"channel_id" gorm:"index"`
	Quota      int                   `json:"quota"`
	Action     string                `json:"action" gorm:"type:varchar(40);index"` // 任务类型, song, lyrics, description-mode
	Status     TaskStatus            `json:"status" gorm:"type:varchar(20);index"` // 任务状态
	FailReason string                `json:"fail_reason"`
	SubmitTime int64                 `json:"submit_time" gorm:"index"`
	StartTime  int64                 `json:"start_time" gorm:"index"`
	FinishTime int64                 `json:"finish_time" gorm:"index"`
	Progress   string                `json:"progress" gorm:"type:varchar(20);index"`
	Properties Properties            `json:"properties" gorm:"type:json"`
	Username   string                `json:"username,omitempty" gorm:"-"`
	ResultURL  string                `json:"result_url,omitempty" gorm:"-"`
	// 禁止返回给用户，内部可能包含key等隐私信息
	PrivateData TaskPrivateData `json:"-" gorm:"column:private_data;type:json"`
	Data        json.RawMessage `json:"data" gorm:"type:json"`
}

func (t *Task) SetData(data any) {
	b, _ := common.Marshal(data)
	t.Data = json.RawMessage(b)
}

func (t *Task) GetData(v any) error {
	return common.Unmarshal(t.Data, v)
}

type Properties struct {
	Input             string `json:"input"`
	UpstreamModelName string `json:"upstream_model_name,omitempty"`
	OriginModelName   string `json:"origin_model_name,omitempty"`
	TokenId           int    `json:"token_id,omitempty"`
	RequestHost       string `json:"request_host,omitempty"`
}

func (m *Properties) Scan(val interface{}) error {
	bytesValue, _ := val.([]byte)
	if len(bytesValue) == 0 {
		*m = Properties{}
		return nil
	}
	return common.Unmarshal(bytesValue, m)
}

func (m Properties) Value() (driver.Value, error) {
	if m == (Properties{}) {
		return nil, nil
	}
	return common.Marshal(m)
}

type TaskPrivateData struct {
	Key                 string                    `json:"key,omitempty"`
	UpstreamTaskID      string                    `json:"upstream_task_id,omitempty"`      // 上游真实 task ID
	RequestID           string                    `json:"request_id,omitempty"`            // 网关请求 ID，用于任务与消费日志关联
	RequestSnapshot     *TaskRequestSnapshot      `json:"request_snapshot,omitempty"`      // 脱敏后的原始/生效请求参数
	SubmitResponse      json.RawMessage           `json:"submit_response,omitempty"`       // 脱敏后的上游提交响应
	ResultURL           string                    `json:"result_url,omitempty"`            // 任务成功后的结果 URL（视频地址等）
	ErrorDetail         *TaskErrorDetail          `json:"error_detail,omitempty"`          // 任务失败时的结构化错误信息
	GenerateImageTiming *GenerateImageTimingAudit `json:"generate_image_timing,omitempty"` // 统一异步生图的分段耗时与流量审计
	// 计费上下文：用于异步退款/差额结算（轮询阶段读取）
	BillingSource  string              `json:"billing_source,omitempty"`  // "wallet" 或 "subscription"
	SubscriptionId int                 `json:"subscription_id,omitempty"` // 订阅 ID，用于订阅退款
	TokenId        int                 `json:"token_id,omitempty"`        // 令牌 ID，用于令牌额度退款
	NodeName       string              `json:"node_name,omitempty"`       // 发起任务的节点名，轮询结算阶段据此归属日志而非最后查询节点
	BillingContext *TaskBillingContext `json:"billing_context,omitempty"` // 计费参数快照（用于轮询阶段重新计算）
	SubmitLogID    int                 `json:"submit_log_id,omitempty"`   // 提交时消费日志的 ID，任务完成后更新
	UsedChannels   []string            `json:"used_channels,omitempty"`   // 每次真实上游尝试使用的渠道，保留重复项用于展示重试链路
}

// GenerateImageTimingAudit records only aggregate durations and byte counts.
// It intentionally excludes request bodies, image URLs, credentials, and other
// values that could expose user or upstream data in an administrator log.
type GenerateImageTimingAudit struct {
	ClientRequestBytes  int64   `json:"client_request_bytes,omitempty"`
	ClientBodyReceiveMs float64 `json:"client_body_receive_ms,omitempty"`
	LocalRequestMs      float64 `json:"local_request_ms,omitempty"`

	InputBytes        int64   `json:"input_bytes,omitempty"`
	InputPrepareMs    float64 `json:"input_prepare_ms,omitempty"`
	InputDownloadMs   float64 `json:"input_download_ms,omitempty"`
	InputDecodeMs     float64 `json:"input_decode_ms,omitempty"`
	InputLocalWriteMs float64 `json:"input_local_write_ms,omitempty"`

	UpstreamRequestBytes     int64   `json:"upstream_request_bytes,omitempty"`
	UpstreamTotalMs          float64 `json:"upstream_total_ms,omitempty"`
	UpstreamConnectionMs     float64 `json:"upstream_connection_ms,omitempty"`
	UpstreamRequestWriteMs   float64 `json:"upstream_request_write_ms,omitempty"`
	UpstreamWaitMs           float64 `json:"upstream_wait_ms,omitempty"`
	UpstreamResponseHeaderMs float64 `json:"upstream_response_header_ms,omitempty"`
	UpstreamResponseBytes    int64   `json:"upstream_response_bytes,omitempty"`
	UpstreamResponseReadMs   float64 `json:"upstream_response_read_ms,omitempty"`
	ResponseParseMs          float64 `json:"response_parse_ms,omitempty"`
	UpstreamAttempts         int     `json:"upstream_attempts,omitempty"`
	UpstreamStatus           int     `json:"upstream_status,omitempty"`

	OutputBytes      int64   `json:"output_bytes,omitempty"`
	OutputDownloadMs float64 `json:"output_download_ms,omitempty"`
	OutputUploadMs   float64 `json:"output_upload_ms,omitempty"`
	OutputTotalMs    float64 `json:"output_total_ms,omitempty"`
}

// TaskRequestSnapshot 是异步任务的审计请求快照，只保存计费与排障需要的参数，
// 不保存图片、音频、视频正文、上传地址或其他凭据。
type TaskRequestSnapshot struct {
	SchemaVersion       int    `json:"schema_version"`
	Model               string `json:"model,omitempty"`
	Action              string `json:"action,omitempty"`
	Prompt              string `json:"prompt,omitempty"`
	Mode                string `json:"mode,omitempty"`
	Size                string `json:"size,omitempty"`
	Duration            int    `json:"duration,omitempty"`
	AspectRatio         string `json:"aspect_ratio,omitempty"`
	ResolutionRequested string `json:"resolution_requested,omitempty"`
	ResolutionEffective string `json:"resolution_effective,omitempty"`
	ResolutionDefaulted bool   `json:"resolution_defaulted,omitempty"`
	ImageCount          int    `json:"image_count,omitempty"`
	ReferenceImageCount int    `json:"reference_image_count,omitempty"`
	ReferenceAudioCount int    `json:"reference_audio_count,omitempty"`
	HasVideo            bool   `json:"has_video,omitempty"`
}

type TaskErrorDetail struct {
	UpstreamStatus    int    `json:"upstream_status,omitempty"`
	UpstreamCode      string `json:"upstream_code,omitempty"`
	UpstreamType      string `json:"upstream_type,omitempty"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
	RetryAction       string `json:"retry_action,omitempty"`
	// UpstreamBilled 表示上游返回 200 却没给图：以 HTTP 200 为锚点，视为上游已在其侧受理并计费，
	// 失败退款据此判定不退（方案A），不依赖上游是否回显 token。
	UpstreamBilled bool `json:"upstream_billed,omitempty"`
	// 若 200 空图响应仍带用量信息，记录下来仅用于核对，不影响退款判定。
	UpstreamPromptTokens     int `json:"upstream_prompt_tokens,omitempty"`
	UpstreamCompletionTokens int `json:"upstream_completion_tokens,omitempty"`
}

// TaskBillingContext 记录任务提交时的计费参数，以便轮询阶段可以重新计算额度。
type TaskBillingContext struct {
	ModelPrice           float64                          `json:"model_price,omitempty"`
	GroupRatio           float64                          `json:"group_ratio,omitempty"`
	ModelRatio           float64                          `json:"model_ratio,omitempty"`
	CompletionRatio      float64                          `json:"completion_ratio,omitempty"`
	VideoCompletionRatio float64                          `json:"video_completion_ratio,omitempty"`
	OtherRatios          map[string]float64               `json:"other_ratios,omitempty"`
	OriginModelName      string                           `json:"origin_model_name,omitempty"`
	PerCallBilling       bool                             `json:"per_call_billing,omitempty"`
	TieredSnapshot       json.RawMessage                  `json:"tiered_snapshot,omitempty"`
	TieredRequestBody    json.RawMessage                  `json:"tiered_request_body,omitempty"`
	VideoBilling         *commonRelay.VideoBillingDetails `json:"video_billing,omitempty"`
}

// GetUpstreamTaskID 获取上游真实 task ID（用于与 provider 通信）
// 旧数据没有 UpstreamTaskID 时，TaskID 本身就是上游 ID
func (t *Task) GetUpstreamTaskID() string {
	if t.PrivateData.UpstreamTaskID != "" {
		return t.PrivateData.UpstreamTaskID
	}
	return t.TaskID
}

// GetResultURL 获取任务结果 URL（视频地址等）
// 新数据存在 PrivateData.ResultURL 中；旧数据回退到 FailReason（历史兼容）
func (t *Task) GetResultURL() string {
	dataURL := t.GetDataResultURL()
	if dataURL != "" && t.isOwnVideoProxyURL(t.PrivateData.ResultURL) {
		return dataURL
	}
	if strings.TrimSpace(t.PrivateData.ResultURL) != "" {
		return strings.TrimSpace(t.PrivateData.ResultURL)
	}
	if dataURL != "" {
		return dataURL
	}
	return strings.TrimSpace(t.FailReason)
}

// GetDataResultURL extracts direct result URLs from provider response payloads.
func (t *Task) GetDataResultURL() string {
	if t == nil || len(t.Data) == 0 {
		return ""
	}
	var payload map[string]any
	if err := common.Unmarshal(t.Data, &payload); err != nil {
		return ""
	}
	return firstDirectURL(
		nestedString(payload, "output", "url"),
		stringField(payload, "video_url"),
		stringField(payload, "url"),
	)
}

func (t *Task) isOwnVideoProxyURL(rawURL string) bool {
	if t == nil || strings.TrimSpace(t.TaskID) == "" {
		return false
	}
	return strings.Contains(strings.TrimSpace(rawURL), "/v1/videos/"+t.TaskID+"/content")
}

func stringField(payload map[string]any, key string) string {
	value, ok := payload[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func nestedString(payload map[string]any, keys ...string) string {
	var current any = payload
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[key]
	}
	value, ok := current.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func firstDirectURL(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "data:") {
			return value
		}
	}
	return ""
}

// GenerateTaskID 生成对外暴露的 task_xxxx 格式 ID
func GenerateTaskID() string {
	key, _ := common.GenerateRandomCharsKey(32)
	return "task_" + key
}

func (p *TaskPrivateData) Scan(val interface{}) error {
	bytesValue, _ := val.([]byte)
	if len(bytesValue) == 0 {
		return nil
	}
	return common.Unmarshal(bytesValue, p)
}

func (p TaskPrivateData) Value() (driver.Value, error) {
	if reflect.ValueOf(p).IsZero() {
		return nil, nil
	}
	return common.Marshal(p)
}

// splitExcludePlatforms 把逗号分隔的 exclude_platform 查询值拆成切片，供 SQL NOT IN 使用。
// 兼容单值（如 "async_image"）和多值（如 "async_image,generate_image"）。
func splitExcludePlatforms(raw constant.TaskPlatform) []string {
	parts := strings.Split(string(raw), ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// SyncTaskQueryParams 用于包含所有搜索条件的结构体，可以根据需求添加更多字段
type SyncTaskQueryParams struct {
	Platform        constant.TaskPlatform
	ExcludePlatform constant.TaskPlatform
	ChannelID       string
	TaskID          string
	UserID          string
	Action          string
	Status          string
	StartTimestamp  int64
	EndTimestamp    int64
	UserIDs         []int
}

func InitTask(platform constant.TaskPlatform, relayInfo *commonRelay.RelayInfo) *Task {
	properties := Properties{}
	privateData := TaskPrivateData{}
	if relayInfo != nil && relayInfo.ChannelMeta != nil {
		if relayInfo.ChannelMeta.ChannelType == constant.ChannelTypeGemini ||
			relayInfo.ChannelMeta.ChannelType == constant.ChannelTypeVertexAi {
			privateData.Key = relayInfo.ChannelMeta.ApiKey
		}
		if relayInfo.UpstreamModelName != "" {
			properties.UpstreamModelName = relayInfo.UpstreamModelName
		}
		if relayInfo.OriginModelName != "" {
			properties.OriginModelName = relayInfo.OriginModelName
		}
	}

	// 使用预生成的公开 ID（如果有），否则新生成
	taskID := ""
	if relayInfo.TaskRelayInfo != nil && relayInfo.TaskRelayInfo.PublicTaskID != "" {
		taskID = relayInfo.TaskRelayInfo.PublicTaskID
	} else {
		taskID = GenerateTaskID()
	}

	t := &Task{
		TaskID:      taskID,
		UserId:      relayInfo.UserId,
		Group:       relayInfo.UsingGroup,
		SubmitTime:  time.Now().Unix(),
		Status:      TaskStatusNotStart,
		Progress:    "0%",
		ChannelId:   relayInfo.ChannelId,
		Platform:    platform,
		Properties:  properties,
		PrivateData: privateData,
	}
	return t
}

func TaskGetAllUserTask(userId int, startIdx int, num int, queryParams SyncTaskQueryParams) []*Task {
	var tasks []*Task
	var err error

	// 初始化查询构建器
	query := DB.Where("user_id = ?", userId)

	if queryParams.TaskID != "" {
		query = query.Where("task_id = ?", queryParams.TaskID)
	}
	if queryParams.Action != "" {
		query = query.Where("action = ?", queryParams.Action)
	}
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.Platform != "" {
		query = query.Where("platform = ?", queryParams.Platform)
	}
	if queryParams.ExcludePlatform != "" {
		query = query.Where("platform NOT IN ?", splitExcludePlatforms(queryParams.ExcludePlatform))
	}
	if queryParams.StartTimestamp != 0 {
		// 假设您已将前端传来的时间戳转换为数据库所需的时间格式，并处理了时间戳的验证和解析
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}

	// 获取数据
	err = query.Omit("channel_id").Order("id desc").Limit(num).Offset(startIdx).Find(&tasks).Error
	if err != nil {
		return nil
	}

	return tasks
}

func TaskGetAllTasks(startIdx int, num int, queryParams SyncTaskQueryParams) []*Task {
	var tasks []*Task
	var err error

	// 初始化查询构建器
	query := DB

	// 添加过滤条件
	if queryParams.ChannelID != "" {
		query = query.Where("channel_id = ?", queryParams.ChannelID)
	}
	if queryParams.Platform != "" {
		query = query.Where("platform = ?", queryParams.Platform)
	}
	if queryParams.ExcludePlatform != "" {
		query = query.Where("platform NOT IN ?", splitExcludePlatforms(queryParams.ExcludePlatform))
	}
	if queryParams.UserID != "" {
		query = query.Where("user_id = ?", queryParams.UserID)
	}
	if len(queryParams.UserIDs) != 0 {
		query = query.Where("user_id in (?)", queryParams.UserIDs)
	}
	if queryParams.TaskID != "" {
		query = query.Where("task_id = ?", queryParams.TaskID)
	}
	if queryParams.Action != "" {
		query = query.Where("action = ?", queryParams.Action)
	}
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.StartTimestamp != 0 {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}

	// 获取数据
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&tasks).Error
	if err != nil {
		return nil
	}

	return tasks
}

func GetTimedOutUnfinishedTasks(cutoffUnix int64, limit int) []*Task {
	var tasks []*Task
	err := DB.Where("progress != ?", "100%").
		Where("status NOT IN ?", []string{TaskStatusFailure, TaskStatusSuccess}).
		Where("submit_time < ?", cutoffUnix).
		Order("submit_time").
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil
	}
	return tasks
}

// GetUnrefundedFailedTasks returns failed tasks whose non-zero quota marks a
// pending refund. Legacy timeout tasks are excluded before LIMIT is applied so
// they cannot starve refundable tasks from the reconciliation sweep.
func GetUnrefundedFailedTasks(updatedBefore int64, limit int) []*Task {
	if limit <= 0 {
		return nil
	}

	var tasks []*Task
	err := DB.Where("status = ?", TaskStatusFailure).
		Where("quota != ?", 0).
		Where("updated_at <= ?", updatedBefore).
		Where("(submit_time <= ? OR submit_time >= ?)", 0, TaskRefundLegacyCutoff).
		Order("id").
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil
	}
	return tasks
}

func GetAllUnFinishSyncTasks(limit int) []*Task {
	var tasks []*Task
	var err error
	// get all tasks progress is not 100%
	err = DB.Where("progress != ?", "100%").Where("status != ?", TaskStatusFailure).Where("status != ?", TaskStatusSuccess).Limit(limit).Order("id").Find(&tasks).Error
	if err != nil {
		return nil
	}
	return tasks
}

// HasUnfinishedSyncTasks reports whether at least one async (Suno/video) task is
// still in progress. It is a cheap existence check (LIMIT 1) used to decide
// whether the async_task_poll system task needs to run; when no task is pending
// the scheduler skips creating a row entirely.
func HasUnfinishedSyncTasks() bool {
	var id int64
	err := DB.Model(&Task{}).
		Where("progress != ?", "100%").
		Where("status != ?", TaskStatusFailure).
		Where("status != ?", TaskStatusSuccess).
		Limit(1).
		Pluck("id", &id).Error
	return err == nil && id != 0
}

// HasTaskPollingWork reports whether an unfinished async task needs polling.
// A failed task's non-zero quota is historical billing data and must never be
// interpreted as a pending refund marker.
func HasTaskPollingWork() bool {
	return HasUnfinishedSyncTasks()
}

func GetByOnlyTaskId(taskId string) (*Task, bool, error) {
	if taskId == "" {
		return nil, false, nil
	}
	var task *Task
	var err error
	err = DB.Where("task_id = ?", taskId).First(&task).Error
	exist, err := RecordExist(err)
	if err != nil {
		return nil, false, err
	}
	return task, exist, err
}

func GetByTaskId(userId int, taskId string) (*Task, bool, error) {
	if taskId == "" {
		return nil, false, nil
	}
	var task *Task
	var err error

	// First query: fetch lightweight fields without the heavy 'data' column
	// This is critical for polling performance when tasks contain large base64 images in data
	err = DB.Select("id, created_at, updated_at, task_id, platform, user_id, "+
		commonGroupCol+", channel_id, quota, action, status, fail_reason, "+
		"submit_time, start_time, finish_time, progress, properties, private_data").
		Where("user_id = ? and task_id = ?", userId, taskId).
		First(&task).Error

	exist, err := RecordExist(err)
	if err != nil {
		return nil, false, err
	}
	if !exist {
		return nil, false, nil
	}

	// Only load the 'data' column when the task is in a terminal state
	// IN_PROGRESS/SUBMITTED/QUEUED tasks don't need data for polling responses
	if task.Status == TaskStatusSuccess || task.Status == TaskStatusFailure {
		var fullTask Task
		err = DB.Where("id = ?", task.ID).First(&fullTask).Error
		if err != nil {
			return nil, false, err
		}
		task.Data = fullTask.Data
	}

	if task != nil {
		task.ResultURL = task.GetResultURL()
	}
	return task, exist, err
}

func GetByTaskIds(userId int, taskIds []any) ([]*Task, error) {
	if len(taskIds) == 0 {
		return nil, nil
	}
	var task []*Task
	var err error
	err = DB.Where("user_id = ? and task_id in (?)", userId, taskIds).
		Find(&task).Error
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (Task *Task) Insert() error {
	var err error
	err = DB.Create(Task).Error
	return err
}

type taskSnapshot struct {
	Status     TaskStatus
	Progress   string
	StartTime  int64
	FinishTime int64
	FailReason string
	ResultURL  string
	Data       json.RawMessage
}

func (s taskSnapshot) Equal(other taskSnapshot) bool {
	return s.Status == other.Status &&
		s.Progress == other.Progress &&
		s.StartTime == other.StartTime &&
		s.FinishTime == other.FinishTime &&
		s.FailReason == other.FailReason &&
		s.ResultURL == other.ResultURL &&
		bytes.Equal(s.Data, other.Data)
}

func (t *Task) Snapshot() taskSnapshot {
	return taskSnapshot{
		Status:     t.Status,
		Progress:   t.Progress,
		StartTime:  t.StartTime,
		FinishTime: t.FinishTime,
		FailReason: t.FailReason,
		ResultURL:  t.PrivateData.ResultURL,
		Data:       t.Data,
	}
}

func (Task *Task) Update() error {
	var err error
	err = DB.Save(Task).Error
	return err
}

func (t *Task) UpdateQuota() error {
	return DB.Model(t).Update("quota", t.Quota).Error
}

func (t *Task) UpdatePrivateData() error {
	return DB.Model(t).Update("private_data", t.PrivateData).Error
}

// ClaimQuotaForRefund atomically clears an expected non-zero quota. A true
// result grants the caller ownership of the corresponding refund attempt.
func ClaimQuotaForRefund(id int64, expectedQuota int) (bool, error) {
	if expectedQuota == 0 {
		return false, nil
	}

	result := DB.Model(&Task{}).
		Where("id = ? AND quota = ?", id, expectedQuota).
		Update("quota", 0)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// RestoreQuotaAfterFailedRefund restores a claimed quota marker only while it
// is still zero. It is used when the observable funding adjustment fails, so a
// later reconciliation pass can retry without overwriting another writer.
func RestoreQuotaAfterFailedRefund(id int64, quota int) (bool, error) {
	if quota == 0 {
		return false, nil
	}

	result := DB.Model(&Task{}).
		Where("id = ? AND quota = ?", id, 0).
		Update("quota", quota)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// UpdateWithStatus performs a conditional UPDATE guarded by fromStatus (CAS).
// Returns (true, nil) if this caller won the update, (false, nil) if
// another process already moved the task out of fromStatus. MySQL commonly
// reports changed rows rather than matched rows, so a same-value no-op update
// can also return false even when the status predicate still matched.
//
// Uses Model().Select("*").Updates() instead of Save() because GORM's Save
// falls back to INSERT ON CONFLICT when the WHERE-guarded UPDATE matches
// zero rows, which silently bypasses the CAS guard.
func (t *Task) UpdateWithStatus(fromStatus TaskStatus) (bool, error) {
	result := DB.Model(t).Where("status = ?", fromStatus).Select("*").Updates(t)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// TaskBulkUpdate performs an unconditional bulk UPDATE by upstream task_id strings.
// Same caveats as TaskBulkUpdateByID — no CAS guard.
func TaskBulkUpdate(taskIds []string, params map[string]any) error {
	if len(taskIds) == 0 {
		return nil
	}
	return DB.Model(&Task{}).
		Where("task_id in (?)", taskIds).
		Updates(params).Error
}

// TaskBulkUpdateByID performs an unconditional bulk UPDATE by primary key IDs.
// WARNING: This function has NO CAS (Compare-And-Swap) guard — it will overwrite
// any concurrent status changes. DO NOT use in billing/quota lifecycle flows
// (e.g., timeout, success, failure transitions that trigger refunds or settlements).
// For status transitions that involve billing, use Task.UpdateWithStatus() instead.
func TaskBulkUpdateByID(ids []int64, params map[string]any) error {
	if len(ids) == 0 {
		return nil
	}
	return DB.Model(&Task{}).
		Where("id in (?)", ids).
		Updates(params).Error
}

type TaskQuotaUsage struct {
	Mode  string  `json:"mode"`
	Count float64 `json:"count"`
}

// TaskCountAllTasks returns total tasks that match the given query params (admin usage)
func TaskCountAllTasks(queryParams SyncTaskQueryParams) int64 {
	var total int64
	query := DB.Model(&Task{})
	if queryParams.ChannelID != "" {
		query = query.Where("channel_id = ?", queryParams.ChannelID)
	}
	if queryParams.Platform != "" {
		query = query.Where("platform = ?", queryParams.Platform)
	}
	if queryParams.UserID != "" {
		query = query.Where("user_id = ?", queryParams.UserID)
	}
	if len(queryParams.UserIDs) != 0 {
		query = query.Where("user_id in (?)", queryParams.UserIDs)
	}
	if queryParams.TaskID != "" {
		query = query.Where("task_id = ?", queryParams.TaskID)
	}
	if queryParams.Action != "" {
		query = query.Where("action = ?", queryParams.Action)
	}
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.StartTimestamp != 0 {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}
	if queryParams.ExcludePlatform != "" {
		query = query.Where("platform NOT IN ?", splitExcludePlatforms(queryParams.ExcludePlatform))
	}
	_ = query.Count(&total).Error
	return total
}

// TaskCountAllUserTask returns total tasks for given user
func TaskCountAllUserTask(userId int, queryParams SyncTaskQueryParams) int64 {
	var total int64
	query := DB.Model(&Task{}).Where("user_id = ?", userId)
	if queryParams.TaskID != "" {
		query = query.Where("task_id = ?", queryParams.TaskID)
	}
	if queryParams.Action != "" {
		query = query.Where("action = ?", queryParams.Action)
	}
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.Platform != "" {
		query = query.Where("platform = ?", queryParams.Platform)
	}
	if queryParams.StartTimestamp != 0 {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}
	if queryParams.ExcludePlatform != "" {
		query = query.Where("platform NOT IN ?", splitExcludePlatforms(queryParams.ExcludePlatform))
	}
	_ = query.Count(&total).Error
	return total
}
func (t *Task) ToOpenAIVideo() *dto.OpenAIVideo {
	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = t.TaskID
	openAIVideo.Status = t.Status.ToVideoStatus()
	openAIVideo.Model = t.Properties.OriginModelName
	openAIVideo.SetProgressStr(t.Progress)
	openAIVideo.CreatedAt = t.CreatedAt
	openAIVideo.CompletedAt = t.UpdatedAt
	openAIVideo.SetMetadata("url", t.GetResultURL())
	return openAIVideo
}

// ClearGenerateImageDataWindow blanks the heavy `data` column (which holds
// returned base64 image payloads for passthrough or unconfigured channels)
// for terminal generate_image tasks whose finish_time falls in the window
// (since, cutoff]. Rows are kept for billing/audit; only the consumed base64 is
// dropped.
//
// The window is driven entirely off the indexed finish_time bigint and an id
// cursor — the json `data` column is never referenced in a predicate. This is
// deliberate and load-bearing for cross-DB safety: PostgreSQL's json type has
// no equality operator, and SQLite stores json.RawMessage as a BLOB whose
// storage class differs from a TEXT literal, so `data != '{}'` is unusable on
// both. Callers must advance `since` to the previous `cutoff` between runs (see
// the cleanup task watermark) so each row is blanked at most once, avoiding
// MVCC dead-tuple churn from repeated rewrites.
//
// Returns the number of rows blanked across all internal batches.
func ClearGenerateImageDataWindow(since, cutoff int64, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 500
	}
	var total int64
	lastID := int64(0)
	for {
		var ids []int64
		err := DB.Model(&Task{}).
			Where("platform = ?", constant.TaskPlatformGenerateImage).
			Where("status IN ?", []string{TaskStatusSuccess, TaskStatusFailure}).
			Where("finish_time > ? AND finish_time <= ?", since, cutoff).
			Where("id > ?", lastID).
			Order("id").
			Limit(batchSize).
			Pluck("id", &ids).Error
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			break
		}

		result := DB.Model(&Task{}).
			Where("id IN ?", ids).
			Update("data", json.RawMessage("{}"))
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected

		lastID = ids[len(ids)-1]
		if len(ids) < batchSize {
			break
		}
	}
	return total, nil
}
