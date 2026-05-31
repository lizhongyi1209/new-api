package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// GenerateImageSubmit 是统一异步生图端点 POST /async/v1/generateImage 的入口。
// 收扁平参数 → 校验 → 预扣费 → 按模型分发 provider → 建任务 → 异步处理 → 返回 task_id。
func GenerateImageSubmit(c *gin.Context) {
	var req dto.GenerateImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		generateImageError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("请求参数错误: %v", err))
		return
	}

	// 复用既有的图片大小校验（image / images 字段）
	asyncReq := generateImageToAsyncRequest(&req)
	if err := service.ValidateAsyncImageSize(asyncReq); err != nil {
		generateImageError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	userId := c.GetInt("id")
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	channelId := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	tokenId := c.GetInt("token_id")

	relayInfo, priceData, billingErr := prepareAsyncBilling(c, userId, group, channelId, tokenId, req.Model)
	if billingErr != nil {
		generateImageError(c, billingErr.StatusCode, "billing_error", billingErr.Error())
		return
	}

	action, isGeminiNative := service.ResolveImageRoute(req.Model, relayInfo.ChannelMeta.ChannelType)

	task := &model.Task{
		TaskID:     model.GenerateTaskID(),
		UserId:     userId,
		Group:      group,
		ChannelId:  channelId,
		Platform:   constant.TaskPlatformGenerateImage,
		Action:     action,
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

	// 存储计费上下文，供后续退款/结算
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
			PerCallBilling:  priceData.UsePrice,
		}
	}

	// 按 provider 决定任务数据形态：
	// - Gemini 原生：转成 generateContent body 存入任务
	// - 通用 OpenAI image：存扁平的 AsyncImageRequest
	if isGeminiNative {
		nativeReq, convertErr := service.ConvertAsyncImageToGeminiNative(context.Background(), asyncReq)
		if convertErr != nil {
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			generateImageError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("转换请求格式失败: %v", convertErr))
			return
		}
		task.SetData(nativeReq)
	} else {
		task.SetData(asyncReq)
	}

	if err := task.Insert(); err != nil {
		if relayInfo != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		generateImageError(c, http.StatusInternalServerError, "internal_error", fmt.Sprintf("创建任务失败: %v", err))
		return
	}

	// 记录提交时的消费日志，并持久化 log ID 供完成后更新
	service.RecordAsyncImageSubmitLog(c, task, asyncReq, relayInfo)
	_ = task.Update()

	ctx := context.WithValue(context.Background(), "gin_context", c)
	gopool.Go(func() {
		service.ProcessGenerateImageTask(ctx, task)
	})

	c.JSON(http.StatusOK, dto.AsyncTaskResponse{
		TaskID: task.TaskID,
		Status: string(task.Status),
	})
}

// generateImageToAsyncRequest 把统一生图入参映射为既有的 AsyncImageRequest，
// 以复用现成的图片校验、Gemini 转换、提交日志逻辑。
func generateImageToAsyncRequest(req *dto.GenerateImageRequest) *dto.AsyncImageRequest {
	return &dto.AsyncImageRequest{
		Model:              req.Model,
		Prompt:             req.Prompt,
		N:                  req.N,
		Size:               req.Size,
		Quality:            req.Quality,
		AspectRatio:        req.AspectRatio,
		ImageCompression:   req.ImageCompression,
		ResponseModalities: req.ResponseModalities,
		Image:              req.Image,
		Images:             req.Images,
	}
}

// generateImageError 统一错误响应格式。
func generateImageError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}
