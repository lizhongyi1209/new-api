package controller

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const klingTaskQueryLimit = 100

// RelayKling40TaskFetch returns one Kling 4.0 or Flash task in the official list envelope.
func RelayKling40TaskFetch(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	userID := c.GetInt("id")
	task, exist, err := model.GetByTaskId(userID, taskID)
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "get_task_failed", http.StatusInternalServerError))
		return
	}
	if !exist || !slices.Contains(model.Kling40TaskActions(), task.Action) {
		respondTaskError(c, service.TaskErrorWrapperLocal(errors.New("task_not_exist"), "task_not_exist", http.StatusBadRequest))
		return
	}
	c.JSON(http.StatusOK, relay.KlingOfficialTaskList(klingRequestID(c), []*model.Task{task}))
}

// RelayKling40TasksQuery serves official batch and cursor task queries under /kling/tasks.
func RelayKling40TasksQuery(c *gin.Context) {
	if c.Request.Method == http.MethodPost {
		relayKling40TaskCursor(c)
		return
	}
	taskIDs := splitKlingIDs(c.Query("task_ids"))
	externalIDs := splitKlingIDs(c.Query("external_task_ids"))
	if len(taskIDs) > 0 && len(externalIDs) > 0 {
		respondTaskError(c, service.TaskErrorWrapperLocal(fmt.Errorf("task_ids and external_task_ids cannot be used together"), "invalid_request", http.StatusBadRequest))
		return
	}
	if len(taskIDs) == 0 && len(externalIDs) == 0 {
		respondTaskError(c, service.TaskErrorWrapperLocal(fmt.Errorf("task_ids or external_task_ids is required"), "invalid_request", http.StatusBadRequest))
		return
	}
	if len(taskIDs) > klingTaskQueryLimit || len(externalIDs) > klingTaskQueryLimit {
		respondTaskError(c, service.TaskErrorWrapperLocal(fmt.Errorf("at most %d task ids can be queried", klingTaskQueryLimit), "invalid_request", http.StatusBadRequest))
		return
	}
	tasks, err := model.ListUserKling40Tasks(c.GetInt("id"), model.Kling40TaskQuery{
		TaskIDs:     taskIDs,
		ExternalIDs: externalIDs,
		Limit:       klingTaskQueryLimit,
	})
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "get_task_failed", http.StatusInternalServerError))
		return
	}
	c.JSON(http.StatusOK, relay.KlingOfficialTaskList(klingRequestID(c), orderKlingTasks(tasks, taskIDs, externalIDs)))
}

func relayKling40TaskCursor(c *gin.Context) {
	query, empty, err := parseKlingTaskCursor(c, time.Now())
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest))
		return
	}
	if empty {
		c.JSON(http.StatusOK, relay.KlingOfficialTaskCursor(klingRequestID(c), nil, false))
		return
	}
	pageSize := query.Limit
	query.Limit = pageSize + 1
	tasks, err := model.ListUserKling40Tasks(c.GetInt("id"), query)
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "get_task_failed", http.StatusInternalServerError))
		return
	}
	hasMore := len(tasks) > pageSize
	if hasMore {
		tasks = tasks[:pageSize]
	}
	c.JSON(http.StatusOK, relay.KlingOfficialTaskCursor(klingRequestID(c), tasks, hasMore))
}

func parseKlingTaskCursor(c *gin.Context, now time.Time) (model.Kling40TaskQuery, bool, error) {
	body := map[string]any{}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return model.Kling40TaskQuery{}, false, err
	}
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := common.Unmarshal(raw, &body); err != nil {
			return model.Kling40TaskQuery{}, false, fmt.Errorf("invalid task query body")
		}
	}
	limit := 100
	if value, ok := body["limit"]; ok {
		parsed, ok := jsonPositiveInt(value)
		if !ok || parsed > 500 {
			return model.Kling40TaskQuery{}, false, fmt.Errorf("limit must be between 1 and 500")
		}
		limit = parsed
	}
	cursorID := int64(0)
	if value, ok := body["cursor"]; ok && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
		parsed, ok := jsonPositiveInt(value)
		if !ok {
			return model.Kling40TaskQuery{}, false, fmt.Errorf("cursor is invalid")
		}
		cursorID = int64(parsed)
	}
	statuses, includeVideo, err := klingCursorFilters(body["filters"])
	if err != nil {
		return model.Kling40TaskQuery{}, false, err
	}
	if !includeVideo {
		return model.Kling40TaskQuery{}, true, nil
	}
	query := model.Kling40TaskQuery{Statuses: statuses, Limit: limit, CursorID: cursorID}
	if cursorID > 0 {
		return query, false, nil
	}
	endMs := now.UnixMilli()
	if value, ok := body["end_time"]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
		parsed, ok := jsonPositiveInt(value)
		if !ok {
			return model.Kling40TaskQuery{}, false, fmt.Errorf("end_time must be a unix millisecond timestamp")
		}
		endMs = int64(parsed)
	}
	startMs := endMs - int64(30*24*time.Hour/time.Millisecond)
	if value, ok := body["start_time"]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
		parsed, ok := jsonPositiveInt(value)
		if !ok {
			return model.Kling40TaskQuery{}, false, fmt.Errorf("start_time must be a unix millisecond timestamp")
		}
		startMs = int64(parsed)
	}
	if startMs >= endMs {
		return model.Kling40TaskQuery{}, false, fmt.Errorf("start_time must be earlier than end_time")
	}
	query.UseTime = true
	query.StartSec = startMs / 1000
	query.EndSec = endMs / 1000
	return query, false, nil
}

func klingCursorFilters(raw any) ([]model.TaskStatus, bool, error) {
	if raw == nil {
		return nil, true, nil
	}
	filters, ok := raw.([]any)
	if !ok {
		return nil, false, fmt.Errorf("filters must be an array")
	}
	var statuses []model.TaskStatus
	includeVideo := true
	sawProduct := false
	for _, item := range filters {
		filter, ok := item.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("filters must contain key and values")
		}
		key, _ := filter["key"].(string)
		values, ok := stringList(filter["values"])
		if !ok || len(values) == 0 {
			return nil, false, fmt.Errorf("filters.values is required")
		}
		switch key {
		case "status":
			for _, value := range values {
				switch value {
				case "submitted":
					statuses = append(statuses, model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued)
				case "processing":
					statuses = append(statuses, model.TaskStatusInProgress)
				case "succeeded":
					statuses = append(statuses, model.TaskStatusSuccess)
				case "failed":
					statuses = append(statuses, model.TaskStatusFailure)
				default:
					return nil, false, fmt.Errorf("invalid status filter %q", value)
				}
			}
		case "product_type":
			sawProduct = true
			includeVideo = false
			for _, value := range values {
				switch value {
				case "video":
					includeVideo = true
				case "image", "try_on":
				default:
					return nil, false, fmt.Errorf("invalid product_type filter %q", value)
				}
			}
		default:
			return nil, false, fmt.Errorf("invalid filter key %q", key)
		}
	}
	if sawProduct && !includeVideo {
		return statuses, false, nil
	}
	return statuses, true, nil
}

func orderKlingTasks(tasks []*model.Task, taskIDs []string, externalIDs []string) []*model.Task {
	if len(taskIDs) == 0 && len(externalIDs) == 0 {
		return tasks
	}
	byTask := map[string]*model.Task{}
	byExternal := map[string]*model.Task{}
	for _, task := range tasks {
		byTask[task.TaskID] = task
		if task.ExternalTaskID != "" {
			byExternal[task.ExternalTaskID] = task
		}
	}
	ordered := make([]*model.Task, 0, len(tasks))
	keys := taskIDs
	lookup := byTask
	if len(externalIDs) > 0 {
		keys = externalIDs
		lookup = byExternal
	}
	for _, key := range keys {
		if task, ok := lookup[key]; ok {
			ordered = append(ordered, task)
		}
	}
	return ordered
}

func splitKlingIDs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func stringList(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			values = append(values, text)
		}
		return values, true
	case []string:
		return typed, true
	default:
		return nil, false
	}
}

func jsonPositiveInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		if typed != float64(int(typed)) || typed < 1 || typed > 1e15 {
			return 0, false
		}
		return int(typed), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil || parsed < 1 {
			return 0, false
		}
		return int(parsed), true
	default:
		return 0, false
	}
}

func klingRequestID(c *gin.Context) string {
	if id := strings.TrimSpace(c.GetString(common.RequestIdKey)); id != "" {
		return id
	}
	return common.NewRequestId()
}
