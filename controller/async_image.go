package controller

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

func AsyncImageSubmit(c *gin.Context) {
	var req dto.AsyncImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("请求参数错误: %v", err),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	userId := c.GetInt("id")
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	channelId := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	tokenId := c.GetInt("token_id")

	task := &model.Task{
		TaskID:     model.GenerateTaskID(),
		UserId:     userId,
		Group:      group,
		ChannelId:  channelId,
		Platform:   constant.TaskPlatformAsyncImage,
		Action:     "generate",
		Status:     model.TaskStatusSubmitted,
		Progress:   "0%",
		SubmitTime: time.Now().Unix(),
		Properties: model.Properties{
			Input:           req.Prompt,
			OriginModelName: req.Model,
			TokenId:         tokenId,
		},
	}
	task.SetData(req)

	if err := task.Insert(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("创建任务失败: %v", err),
				"type":    "internal_error",
			},
		})
		return
	}

	ctx := context.WithValue(context.Background(), "gin_context", c)
	gopool.Go(func() {
		service.ProcessAsyncImageTask(ctx, task)
	})

	c.JSON(http.StatusOK, dto.AsyncTaskResponse{
		TaskID: task.TaskID,
		Status: string(task.Status),
	})
}

func AsyncGeminiSubmit(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("请求参数错误: %v", err),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	userId := c.GetInt("id")
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	channelId := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	tokenId := c.GetInt("token_id")

	path := c.Param("path")
	modelName := ""
	if len(path) > 1 {
		parts := strings.Split(path[1:], ":")
		if len(parts) > 0 {
			modelName = parts[0]
		}
	}
	if modelName == "" {
		modelName = fmt.Sprintf("%v", req["model"])
	}

	task := &model.Task{
		TaskID:     model.GenerateTaskID(),
		UserId:     userId,
		Group:      group,
		ChannelId:  channelId,
		Platform:   constant.TaskPlatformAsyncImage,
		Action:     "generateContent",
		Status:     model.TaskStatusSubmitted,
		Progress:   "0%",
		SubmitTime: time.Now().Unix(),
		Properties: model.Properties{
			OriginModelName: modelName,
			TokenId:         tokenId,
		},
	}
	task.SetData(req)

	if err := task.Insert(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("创建任务失败: %v", err),
				"type":    "internal_error",
			},
		})
		return
	}

	ctx := context.WithValue(context.Background(), "gin_context", c)
	gopool.Go(func() {
		service.ProcessAsyncGeminiTask(ctx, task)
	})

	c.JSON(http.StatusOK, dto.AsyncTaskResponse{
		TaskID: task.TaskID,
		Status: string(task.Status),
	})
}

func AsyncTaskFetch(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		taskID = c.Query("task_id")
	}

	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "缺少 task_id 参数",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	userId := c.GetInt("id")
	task, exists, err := model.GetByTaskId(userId, taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("查询任务失败: %v", err),
				"type":    "internal_error",
			},
		})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "任务不存在",
				"type":    "not_found_error",
			},
		})
		return
	}

	resp := dto.AsyncTaskFetchResponse{
		TaskID:   task.TaskID,
		Status:   string(task.Status),
		Progress: task.Progress,
	}

	if task.Status == model.TaskStatusSuccess {
		resp.Data = task.Data
	} else if task.Status == model.TaskStatusFailure {
		resp.Error = task.FailReason
	}

	c.JSON(http.StatusOK, resp)
}

// Deprecated: use AsyncTaskFetch
func AsyncImageFetch(c *gin.Context) {
	AsyncTaskFetch(c)
}
