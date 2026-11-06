package tencentvod

import (
	"fmt"
	"strings"
)

// Model constants for the Midjourney family. Only v8.2 is exposed by the
// gateway today; the others exist so a channel can be pointed at them without
// a code change.
const (
	ModelNameMJ = "MJ"

	ModelVersionV82 = "v8.2"
)

// Task states reported by DescribeTaskDetail. The outer task object and the
// nested AigcImageTask carry the same enum.
const (
	TaskStatusWaiting    = "WAITING"
	TaskStatusProcessing = "PROCESSING"
	TaskStatusFinish     = "FINISH"
	TaskStatusAborted    = "ABORTED"
)

// Storage modes for OutputConfig.
const (
	StorageModeTemporary = "Temporary"
	StorageModePermanent = "Permanent"
)

// CreateImageTaskRequest is the body of CreateAigcImageTask. Every field is
// PascalCase because the VOD API uses that convention throughout; renaming one
// would silently drop it from the request.
//
// The API has no image-count parameter. Every accepted task produces four
// images regardless of the request.
type CreateImageTaskRequest struct {
	SubAppID     int64             `json:"SubAppId"`
	ModelName    string            `json:"ModelName"`
	ModelVersion string            `json:"ModelVersion"`
	FileInfos    []InputFileInfo   `json:"FileInfos,omitempty"`
	Prompt       string            `json:"Prompt,omitempty"`
	OutputConfig *ImageOutputConfig `json:"OutputConfig,omitempty"`
	SessionID    string            `json:"SessionId,omitempty"`
}

// InputFileInfo is one reference image. Type selects which of FileID, URL or
// Base64 carries the content.
type InputFileInfo struct {
	Type   string `json:"Type,omitempty"`
	FileID string `json:"FileId,omitempty"`
	URL    string `json:"Url,omitempty"`
	Base64 string `json:"Base64,omitempty"`
}

// ImageOutputConfig controls where results land.
//
// Resolution is accepted by the API and echoed back, but it does not take
// effect for MJ: the tier is chosen by the "--hd" prompt flag instead. Do not
// use it for billing decisions.
type ImageOutputConfig struct {
	StorageMode string `json:"StorageMode,omitempty"`
}

// CreateImageTaskResponse is the submit reply. A TaskId means the task was
// accepted, not that generation succeeded.
type CreateImageTaskResponse struct {
	TaskID    string `json:"TaskId"`
	RequestID string `json:"RequestId"`
	Error     *APIError `json:"Error,omitempty"`
}

// DescribeTaskRequest is the body of DescribeTaskDetail.
type DescribeTaskRequest struct {
	SubAppID int64  `json:"SubAppId"`
	TaskID   string `json:"TaskId"`
}

// TaskDetailResponse mirrors the parts of DescribeTaskDetail the gateway uses.
//
// CreateTime and FinishTime live on the outer object, not inside
// AigcImageTask. Reading them from the nested task always yields zero.
type TaskDetailResponse struct {
	TaskType       string          `json:"TaskType"`
	Status         string          `json:"Status"`
	CreateTime     string          `json:"CreateTime"`
	BeginProcessTime string        `json:"BeginProcessTime"`
	FinishTime     string          `json:"FinishTime"`
	RequestID      string          `json:"RequestId"`
	AigcImageTask  AigcImageTask   `json:"AigcImageTask"`
	Error          *APIError       `json:"Error,omitempty"`
}

// AigcImageTask is the nested task object.
type AigcImageTask struct {
	TaskID       string            `json:"TaskId"`
	Status       string            `json:"Status"`
	ErrCode      int               `json:"ErrCode"`
	ErrCodeExt   string            `json:"ErrCodeExt"`
	Message      string            `json:"Message"`
	Progress     int               `json:"Progress"`
	SessionID    string            `json:"SessionId"`
	Input        AigcImageTaskInput `json:"Input"`
	Output       AigcImageTaskOutput `json:"Output"`
}

// AigcImageTaskInput echoes the submitted parameters.
type AigcImageTaskInput struct {
	ModelName    string             `json:"ModelName"`
	ModelVersion string             `json:"ModelVersion"`
	Prompt       string             `json:"Prompt"`
	OutputConfig *ImageOutputConfig `json:"OutputConfig"`
}

// AigcImageTaskOutput carries the generated files and the billing facts.
type AigcImageTaskOutput struct {
	FileInfos []OutputFileInfo `json:"FileInfos"`
	Usage     *OutputUsage     `json:"Usage"`
}

// OutputFileInfo is one generated image. FileType is empty for MJ, so callers
// must derive the format from the URL extension or MetaData.Container.
type OutputFileInfo struct {
	StorageMode string             `json:"StorageMode"`
	ExpireTime  string             `json:"ExpireTime"`
	FileType    string             `json:"FileType"`
	FileID      string             `json:"FileId"`
	FileURL     string             `json:"FileUrl"`
	MetaData    *OutputFileMetaData `json:"MetaData"`
}

// OutputFileMetaData describes the produced image.
type OutputFileMetaData struct {
	Width     int    `json:"Width"`
	Height    int    `json:"Height"`
	Size      int64  `json:"Size"`
	Container string `json:"Container"`
}

// OutputUsage reports what the provider bills. SpecificationUsages has one
// entry per generated image, so a four-image MJ task lists four identical
// entries.
type OutputUsage struct {
	SpecificationUsages []SpecificationUsage `json:"SpecificationUsages"`
}

// SpecificationUsage is one billed unit. Specification encodes the model
// version and resolution tier, e.g. "MJv8.2_1k" or "MJv8.2_2k".
type SpecificationUsage struct {
	Specification string `json:"Specification"`
	Category      string `json:"Category"`
}

// APIError is the error envelope VOD returns inside an HTTP 200 body.
type APIError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Resolution tiers the gateway prices differently. The tier is decided by the
// "--hd" prompt flag, not by any request parameter.
const (
	ResolutionTier1K = "1k"
	ResolutionTier2K = "2k"
)

// ResolutionTier extracts the tier from a Specification value such as
// "MJv8.2_1k". It returns an empty string for an unrecognised shape so the
// caller can fall back to its own pre-charge instead of guessing.
func (u OutputUsage) ResolutionTier() string {
	if len(u.SpecificationUsages) == 0 {
		return ""
	}
	spec := strings.ToLower(strings.TrimSpace(u.SpecificationUsages[0].Specification))
	if spec == "" {
		return ""
	}
	if strings.HasSuffix(spec, "_"+ResolutionTier1K) {
		return ResolutionTier1K
	}
	if strings.HasSuffix(spec, "_"+ResolutionTier2K) {
		return ResolutionTier2K
	}
	return ""
}

// Succeeded reports whether the task reached a usable terminal state. A
// FINISH status alone is not enough: the task can finish with a non-zero
// ErrCode or an empty ErrCodeExt and no files.
func (t *AigcImageTask) Succeeded() bool {
	return t.Status == TaskStatusFinish &&
		t.ErrCode == 0 &&
		strings.TrimSpace(t.ErrCodeExt) == "" &&
		len(t.Output.FileInfos) > 0
}

// Terminal reports whether polling can stop. ABORTED is terminal even though
// the documented enum only lists WAITING/PROCESSING/FINISH for this task type.
func (t *AigcImageTask) Terminal() bool {
	switch t.Status {
	case TaskStatusFinish, TaskStatusAborted:
		return true
	}
	return false
}

// FailureReason builds a diagnostic message from the task error fields.
func (t *AigcImageTask) FailureReason() string {
	parts := make([]string, 0, 3)
	if code := strings.TrimSpace(t.ErrCodeExt); code != "" {
		parts = append(parts, code)
	} else if t.ErrCode != 0 {
		parts = append(parts, fmt.Sprintf("ErrCode=%d", t.ErrCode))
	}
	if msg := strings.TrimSpace(t.Message); msg != "" {
		parts = append(parts, msg)
	}
	if t.Status == TaskStatusFinish && len(t.Output.FileInfos) == 0 {
		parts = append(parts, "上游未返回图片")
	}
	if len(parts) == 0 {
		parts = append(parts, "上游任务失败")
	}
	return strings.Join(parts, ": ")
}
