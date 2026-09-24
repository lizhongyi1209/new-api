package controller

import (
 "net/http"
 "github.com/QuantumNous/new-api/common"
 "github.com/QuantumNous/new-api/model"
 "github.com/QuantumNous/new-api/service"
 "github.com/gin-gonic/gin"
)

func GetTaskAudit(c *gin.Context) {
	task, exists, err := model.GetByOnlyTaskId(c.Param("task_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists || task == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "task not found",
		})
		return
	}

	common.ApiSuccess(c, gin.H{
		"schema_version":   1,
		"request_id":       task.PrivateData.RequestID,
		"task_id":          task.TaskID,
		"upstream_task_id": task.PrivateData.UpstreamTaskID,
		"platform":         task.Platform,
		"model":            task.Properties.OriginModelName,
		"action":           task.Action,
		"status":           task.Status,
		"fail_reason":      task.FailReason,
		"channel_id":       task.ChannelId,
		"used_channels":    task.PrivateData.UsedChannels,
		"quota":            task.Quota,
		"submit_time":      task.SubmitTime,
		"start_time":       task.StartTime,
		"finish_time":      task.FinishTime,
		"request":          task.PrivateData.RequestSnapshot,
		"billing":          task.PrivateData.BillingContext,
		"responses": gin.H{
			"submit": task.PrivateData.SubmitResponse,
			"final":  service.SanitizeTaskAuditResponse(task.Data),
		},
	})
}
