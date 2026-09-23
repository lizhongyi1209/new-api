package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetTask returns only the public lifecycle fields of a task owned by the
// authenticated API token's user. Provider IDs, plugin state, and task data
// remain private.
func GetTask(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	task, exists, err := model.GetByTaskId(c.GetInt("id"), c.Param("key"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to query task", "type": "server_error"}})
		return
	}
	if !exists || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "task not found", "type": "invalid_request_error"}})
		return
	}
	createdAt := task.CreatedAt
	if createdAt == 0 {
		createdAt = task.SubmitTime
	}
	c.JSON(http.StatusOK, gin.H{
		"task_id":     task.TaskID,
		"platform":    task.Platform,
		"status":      task.Status,
		"progress":    task.Progress,
		"fail_reason": task.FailReason,
		"created_at":  createdAt,
		"finished_at": task.FinishTime,
	})
}
