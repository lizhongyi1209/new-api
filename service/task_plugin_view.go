package service

import (
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// BuildTaskPluginView converts a persisted task into the deliberately narrow
// public shape permitted at JavaScript plugin boundaries.
func BuildTaskPluginView(task *model.Task) (dto.TaskView, error) {
	createdAt := task.CreatedAt
	if createdAt == 0 {
		createdAt = task.SubmitTime
	}
	view := dto.TaskView{
		TaskID:     task.TaskID,
		Platform:   string(task.Platform),
		Status:     string(task.Status),
		Progress:   task.Progress,
		FailReason: task.FailReason,
		CreatedAt:  createdAt,
		UpdatedAt:  task.UpdatedAt,
		FinishedAt: task.FinishTime,
	}
	if len(task.Data) > 0 {
		data := task.Data
		if model.HasPrivateKlingPricing(task.Properties.OriginModelName) {
			data, _ = RedactUpstreamCharges(data)
		}
		if err := common.Unmarshal(data, &view.Data); err != nil {
			return dto.TaskView{}, err
		}
		view.Data = replacePrivateTaskID(view.Data, task.PrivateData.UpstreamTaskID, task.TaskID)
	}
	return view, nil
}

// replacePrivateTaskID rewrites exact private IDs only in known task-ID fields.
// Map keys and opaque strings, including URLs containing the ID, are preserved.
func replacePrivateTaskID(value any, privateTaskID, publicTaskID string) any {
	if privateTaskID == "" || privateTaskID == publicTaskID {
		return value
	}
	switch typed := value.(type) {
	case []any:
		replaced := make([]any, len(typed))
		for index, item := range typed {
			replaced[index] = replacePrivateTaskID(item, privateTaskID, publicTaskID)
		}
		return replaced
	case map[string]any:
		replaced := make(map[string]any, len(typed))
		for key, item := range typed {
			if (key == "id" || key == "task_id" || key == "taskId") && item == privateTaskID {
				replaced[key] = publicTaskID
				continue
			}
			replaced[key] = replacePrivateTaskID(item, privateTaskID, publicTaskID)
		}
		return replaced
	default:
		return value
	}
}

// upstreamChargeKeys name provider settlement fields: what the gateway pays
// upstream (Kling billing and unit/balance deductions, Tencent VOD deductions).
// Clients are billed by the gateway, so these never appear in client payloads.
var upstreamChargeKeys = []string{"billing", "final_unit_deduction", "final_balance_deduction", "FinalUnitDeduction"}

// RedactUpstreamCharges removes upstream settlement fields at any depth while
// leaving every other raw value byte-for-byte intact. It reports whether the
// payload changed; a payload that cannot be re-encoded is dropped rather than
// leaked.
func RedactUpstreamCharges(data common.RawMessage) (common.RawMessage, bool) {
	var encoded []byte
	var err error
	switch common.GetJsonType(data) {
	case "object":
		var object map[string]common.RawMessage
		if common.Unmarshal(data, &object) != nil {
			return data, false
		}
		changed := false
		for key, value := range object {
			if slices.Contains(upstreamChargeKeys, key) {
				delete(object, key)
				changed = true
				continue
			}
			if redacted, ok := RedactUpstreamCharges(value); ok {
				object[key] = redacted
				changed = true
			}
		}
		if !changed {
			return data, false
		}
		encoded, err = common.Marshal(object)
	case "array":
		var items []common.RawMessage
		if common.Unmarshal(data, &items) != nil {
			return data, false
		}
		changed := false
		for index, value := range items {
			if redacted, ok := RedactUpstreamCharges(value); ok {
				items[index] = redacted
				changed = true
			}
		}
		if !changed {
			return data, false
		}
		encoded, err = common.Marshal(items)
	default:
		return data, false
	}
	if err != nil {
		return nil, true
	}
	return encoded, true
}
