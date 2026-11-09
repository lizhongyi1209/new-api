package relay

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// KlingOfficialTaskList is the official GET /tasks envelope.
// data is an array of task objects addressed by this gateway's public task id.
func KlingOfficialTaskList(requestID string, tasks []*model.Task) map[string]any {
	return map[string]any{
		"code":       0,
		"message":    "",
		"request_id": requestID,
		"data":       klingOfficialTaskItems(tasks),
	}
}

// KlingOfficialTaskCursor is the official POST /tasks cursor envelope.
func KlingOfficialTaskCursor(requestID string, tasks []*model.Task, hasMore bool) map[string]any {
	next := ""
	if hasMore && len(tasks) > 0 {
		next = strconv.FormatInt(tasks[len(tasks)-1].ID, 10)
	}
	return map[string]any{
		"code":       0,
		"message":    "",
		"request_id": requestID,
		"data": map[string]any{
			"result":      klingOfficialTaskItems(tasks),
			"count":       len(tasks),
			"next_cursor": next,
			"has_more":    hasMore,
		},
	}
}

func klingOfficialTaskItems(tasks []*model.Task) []any {
	items := make([]any, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		items = append(items, klingOfficialTaskItem(task))
	}
	return items
}

func klingOfficialTaskItem(task *model.Task) map[string]any {
	item := map[string]any{
		"id":          task.TaskID,
		"status":      klingOfficialStatus(task.Status),
		"message":     "",
		"create_time": klingUnixMillis(task.CreatedAt, task.SubmitTime),
		"update_time": klingUnixMillis(task.UpdatedAt, task.FinishTime, task.SubmitTime),
		"external_id": task.ExternalTaskID,
	}
	if task.Status == model.TaskStatusFailure {
		item["message"] = strings.TrimSpace(task.FailReason)
	}
	upstream := firstKlingUpstreamTask(task.Data)
	if upstream != nil {
		if message, _ := item["message"].(string); message == "" {
			if text, ok := upstream["message"].(string); ok {
				item["message"] = text
			}
		}
		if created, ok := jsonInt64(upstream["create_time"]); ok && created > 0 {
			item["create_time"] = created
		}
		if updated, ok := jsonInt64(upstream["update_time"]); ok && updated > 0 {
			item["update_time"] = updated
		}
		if item["external_id"] == "" {
			if ext, ok := upstream["external_id"].(string); ok {
				item["external_id"] = ext
			}
		}
		if outputs, ok := upstream["outputs"]; ok && outputs != nil {
			item["outputs"] = outputs
		}
		if usage, ok := upstream["usage"].(map[string]any); ok && len(usage) > 0 {
			item["usage"] = usage
		}
		if billing, ok := upstream["billing"].([]any); ok && len(billing) > 0 {
			item["billing"] = billing
		}
	}
	if _, ok := item["outputs"]; !ok && klingOfficialStatus(task.Status) == "succeeded" {
		if url := task.GetResultURL(); url != "" {
			item["outputs"] = []any{map[string]any{"type": "video", "url": url}}
		}
	}
	return item
}

func klingOfficialStatus(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusInProgress:
		return "processing"
	case model.TaskStatusSuccess:
		return "succeeded"
	case model.TaskStatusFailure:
		return "failed"
	default:
		return "submitted"
	}
}

func klingUnixMillis(values ...int64) int64 {
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if value < 1_000_000_000_000 {
			return value * 1000
		}
		return value
	}
	return time.Now().UnixMilli()
}

func firstKlingUpstreamTask(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var payload struct {
		Data []map[string]any `json:"data"`
	}
	if err := common.Unmarshal(data, &payload); err != nil || len(payload.Data) == 0 {
		return nil
	}
	return payload.Data[0]
}

func jsonInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}
