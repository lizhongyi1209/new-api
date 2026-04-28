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
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
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

	// Build relay info for billing
	relayInfo, priceData, billingErr := prepareAsyncBilling(c, userId, group, channelId, tokenId, req.Model)
	if billingErr != nil {
		c.JSON(billingErr.StatusCode, gin.H{
			"error": gin.H{
				"message": billingErr.Error(),
				"type":    "billing_error",
			},
		})
		return
	}

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
		Quota:      priceData.Quota,
		Properties: model.Properties{
			Input:           req.Prompt,
			OriginModelName: req.Model,
			TokenId:         tokenId,
		},
	}
	task.SetData(req)

	// Store billing context for later refund/settlement
	if relayInfo != nil && priceData.Quota > 0 {
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = tokenId
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:      priceData.ModelPrice,
			GroupRatio:      priceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      priceData.ModelRatio,
			OtherRatios:     priceData.OtherRatios,
			OriginModelName: req.Model,
			PerCallBilling:  true,
		}
	}

	if err := task.Insert(); err != nil {
		// Refund pre-consumed quota on insert failure
		if relayInfo != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("创建任务失败: %v", err),
				"type":    "internal_error",
			},
		})
		return
	}

	// Record usage log at submission time
	service.RecordAsyncImageSubmitLog(c, task, &req, relayInfo)

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

	// Build relay info for billing
	relayInfo, priceData, billingErr := prepareAsyncBilling(c, userId, group, channelId, tokenId, modelName)
	if billingErr != nil {
		c.JSON(billingErr.StatusCode, gin.H{
			"error": gin.H{
				"message": billingErr.Error(),
				"type":    "billing_error",
			},
		})
		return
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
		Quota:      priceData.Quota,
		Properties: model.Properties{
			OriginModelName: modelName,
			TokenId:         tokenId,
		},
	}
	task.SetData(req)

	// Store billing context for later refund/settlement
	if relayInfo != nil && priceData.Quota > 0 {
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = tokenId
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:      priceData.ModelPrice,
			GroupRatio:      priceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      priceData.ModelRatio,
			OtherRatios:     priceData.OtherRatios,
			OriginModelName: modelName,
			PerCallBilling:  true,
		}
	}

	if err := task.Insert(); err != nil {
		// Refund pre-consumed quota on insert failure
		if relayInfo != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("创建任务失败: %v", err),
				"type":    "internal_error",
			},
		})
		return
	}

	// Record usage log at submission time
	service.RecordAsyncGeminiSubmitLog(c, task, modelName, relayInfo)

	ctx := context.WithValue(context.Background(), "gin_context", c)
	gopool.Go(func() {
		service.ProcessAsyncGeminiTask(ctx, task)
	})

	c.JSON(http.StatusOK, dto.AsyncTaskResponse{
		TaskID: task.TaskID,
		Status: string(task.Status),
	})
}

// prepareAsyncBilling builds a RelayInfo, calculates price, and pre-consumes quota.
// Returns the relayInfo (with billing session attached) and priceData.
// Returns nil relayInfo and a NewAPIError if billing fails.
func prepareAsyncBilling(c *gin.Context, userId int, group string, channelId int, tokenId int, modelName string) (*relaycommon.RelayInfo, types.PriceData, *types.NewAPIError) {
	channel, err := model.CacheGetChannel(channelId)
	if err != nil {
		return nil, types.PriceData{}, types.NewError(
			fmt.Errorf("获取渠道信息失败: %v", err),
			types.ErrorCodeQueryDataError,
			types.ErrOptionWithSkipRetry(),
		)
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)

	relayInfo := &relaycommon.RelayInfo{
		UserId:          userId,
		UsingGroup:      group,
		OriginModelName: modelName,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: channel.Type,
			ChannelId:   channel.Id,
			ApiType:     apiType,
		},
		TokenId: tokenId,
	}

	// Populate user settings for billing preference
	if userSetting, ok := common.GetContextKeyType[dto.UserSetting](c, constant.ContextKeyUserSetting); ok {
		relayInfo.UserSetting = userSetting
	}

	if service.CalculatePriceFunc == nil {
		return nil, types.PriceData{}, types.NewError(
			fmt.Errorf("计费函数未初始化"),
			types.ErrorCodeInvalidApiType,
			types.ErrOptionWithSkipRetry(),
		)
	}

	priceData, err := service.CalculatePriceFunc(c, relayInfo)
	if err != nil {
		return nil, types.PriceData{}, types.NewError(
			fmt.Errorf("计算价格失败: %v", err),
			types.ErrorCodeModelPriceError,
			types.ErrOptionWithStatusCode(http.StatusBadRequest),
		)
	}
	relayInfo.PriceData = priceData

	// Pre-consume billing
	if !priceData.FreeModel && priceData.Quota > 0 {
		if apiErr := service.PreConsumeBilling(c, priceData.Quota, relayInfo); apiErr != nil {
			return nil, priceData, apiErr
		}
	}

	return relayInfo, priceData, nil
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
		if task.Platform == constant.TaskPlatformAsyncImage && task.PrivateData.ResultURL != "" {
			data := map[string]interface{}{
				"image_url": task.PrivateData.ResultURL,
			}
			dataBytes, _ := common.Marshal(data)
			resp.Data = dataBytes
		} else {
			resp.Data = task.Data
		}
	} else if task.Status == model.TaskStatusFailure {
		resp.Error = task.FailReason
	}

	c.JSON(http.StatusOK, resp)
}

// Deprecated: use AsyncTaskFetch
func AsyncImageFetch(c *gin.Context) {
	AsyncTaskFetch(c)
}
