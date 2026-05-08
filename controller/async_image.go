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
			PerCallBilling:  priceData.UsePrice,
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

	// Check if this should use Gemini native processing
	// Models with nano-banana prefix or specific Gemini image models use native format
	channelType := relayInfo.ChannelMeta.ChannelType
	isGeminiChannel := channelType == constant.ChannelTypeGemini || channelType == constant.ChannelTypeVertexAi
	isGeminiImageModel := strings.HasPrefix(req.Model, "nano-banana") ||
		req.Model == "gemini-3-pro-image" ||
		req.Model == "gemini-3.1-flash-image-preview"
	useGeminiNative := isGeminiChannel && isGeminiImageModel

	if useGeminiNative {
		// Convert OpenAI-format request to Gemini native format
		nativeReq, convertErr := service.ConvertAsyncImageToGeminiNative(context.Background(), &req)
		if convertErr != nil {
			// Refund on conversion failure
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("转换请求格式失败: %v", convertErr),
					"type":    "invalid_request_error",
				},
			})
			return
		}
		task.Action = "generateContent"
		task.SetData(nativeReq)
		_ = task.Update()

		service.RecordAsyncImageSubmitLog(c, task, &req, relayInfo)
		_ = task.Update()

		ctx := context.WithValue(context.Background(), "gin_context", c)
		gopool.Go(func() {
			service.ProcessAsyncGeminiTask(ctx, task)
		})
	} else {
		// Record usage log at submission time and persist the log ID for later update
		service.RecordAsyncImageSubmitLog(c, task, &req, relayInfo)
		_ = task.Update()

		ctx := context.WithValue(context.Background(), "gin_context", c)
		gopool.Go(func() {
			service.ProcessAsyncImageTask(ctx, task)
		})
	}

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
			PerCallBilling:  priceData.UsePrice,
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

	// Record usage log at submission time and persist the log ID for later update
	service.RecordAsyncGeminiSubmitLog(c, task, modelName, relayInfo)
	_ = task.Update()

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

	upstreamModelName := service.ApplyModelMapping(modelName, channel.ModelMapping)
	isModelMapped := upstreamModelName != modelName

	relayInfo := &relaycommon.RelayInfo{
		UserId:          userId,
		UserGroup:       common.GetContextKeyString(c, constant.ContextKeyUserGroup),
		UsingGroup:      group,
		OriginModelName: modelName,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       channel.Type,
			ChannelId:         channel.Id,
			ApiType:           apiType,
			UpstreamModelName: upstreamModelName,
			IsModelMapped:     isModelMapped,
		},
		TokenId: tokenId,
		TokenKey: common.GetContextKeyString(c, constant.ContextKeyTokenKey),
		TokenUnlimited: common.GetContextKeyBool(c, constant.ContextKeyTokenUnlimited),
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
	// 异步任务必须强制预扣，禁用信任额度旁路，确保余额实际扣除
	relayInfo.ForcePreConsume = true
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
		if task.Platform == constant.TaskPlatformAsyncImage {
			// Return standard OpenAI ImageResponse format
			imageResp := dto.ImageResponse{Created: task.FinishTime}
			var storedData map[string]interface{}
			if err := task.GetData(&storedData); err == nil {
				if urls, ok := storedData["urls"].([]interface{}); ok {
					for _, u := range urls {
						if urlStr, ok := u.(string); ok {
							imageResp.Data = append(imageResp.Data, dto.ImageData{Url: urlStr})
						}
					}
				} else if dataList, ok := storedData["data"].([]interface{}); ok {
					for _, d := range dataList {
						if b64Str, ok := d.(string); ok {
							imageResp.Data = append(imageResp.Data, dto.ImageData{B64Json: b64Str})
						}
					}
				}
			}
			// Fallback: single image via legacy ResultURL
			if len(imageResp.Data) == 0 && task.PrivateData.ResultURL != "" {
				imageResp.Data = append(imageResp.Data, dto.ImageData{Url: task.PrivateData.ResultURL})
			}
			if len(imageResp.Data) > 0 {
				dataBytes, _ := common.Marshal(imageResp)
				resp.Data = dataBytes
			} else {
				resp.Data = task.Data
			}
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
