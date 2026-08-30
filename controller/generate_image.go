package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// GenerateImageSubmit 是统一异步生图端点 POST /async/v1/generateImage 的入口。
// 收扁平参数 → 校验 → 预扣费 → 按模型分发 provider → 建任务 → 异步处理 → 返回 task_id。
func GenerateImageSubmit(c *gin.Context) {
	handlerStartedAt := time.Now()
	var rawReq map[string]json.RawMessage
	if err := unmarshalGenerateImageBody(c, &rawReq); err != nil {
		generateImageError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("请求参数错误: %v", err))
		return
	}
	if _, ok := rawReq["image"]; ok {
		generateImageError(c, http.StatusBadRequest, "invalid_request_error", "image 参数已废弃，请统一使用 images 字段；单张图也传入 images 数组")
		return
	}

	var req dto.GenerateImageRequest
	if err := unmarshalGenerateImageBody(c, &req); err != nil {
		generateImageError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("请求参数错误: %v", err))
		return
	}
	if err := service.ValidateGenerateImageRequest(&req); err != nil {
		generateImageError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	// 复用既有的图片格式与 URL 安全校验（images 字段）。Gemini 的大小限制
	// 在最终请求完成 Base64 编码和 JSON 序列化后按完整请求体检查。
	asyncReq := generateImageToAsyncRequest(&req)
	if err := service.ValidateAsyncImageReferences(asyncReq); err != nil {
		generateImageError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	userId := c.GetInt("id")
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	channelId := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	tokenId := c.GetInt("token_id")

	billingStartedAt := time.Now()
	relayInfo, priceData, billingErr := prepareAsyncBilling(c, userId, group, channelId, tokenId, req.Model)
	if billingErr != nil {
		generateImageError(c, billingErr.StatusCode, "billing_error", billingErr.Error())
		return
	}
	billingFinishedAt := time.Now()

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
			RequestHost:     c.Request.Host,
		},
	}
	task.PrivateData.RequestID = c.GetString(common.RequestIdKey)

	// 存储计费上下文，供后续退款/结算
	// tiered_expr 模型即使预扣为 0（信任用户）也必须存 BillingContext，否则完成时无法结算
	if relayInfo != nil && (priceData.Quota > 0 || c.GetBool("tiered_billing_active")) {
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = tokenId
		bc := &model.TaskBillingContext{
			ModelPrice:      priceData.ModelPrice,
			GroupRatio:      priceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      priceData.ModelRatio,
			OtherRatios:     priceData.OtherRatios,
			OriginModelName: req.Model,
			PerCallBilling:  priceData.UsePrice,
		}
		if snapBytes, ok := c.Get("tiered_snapshot_bytes"); ok {
			if b, ok := snapBytes.([]byte); ok {
				bc.TieredSnapshot = b
			}
		}
		task.PrivateData.BillingContext = bc
	}

	// 按 provider 决定任务数据形态：
	// - Gemini 原生：generateContent body 只在 goroutine 内存中使用，任务里只存省略占位
	// - 通用 OpenAI image：存扁平的 AsyncImageRequest
	var processData any
	if isGeminiNative {
		nativeReq, inputPreparation, convertErr := service.PrepareGenerateImageGeminiNative(
			context.Background(),
			asyncReq,
			req.Images,
			service.GeminiFileDataOptions{Enabled: relayInfo.ChannelOtherSettings.GeminiFileDataEnabled},
		)
		logger.LogInfo(c, fmt.Sprintf(
			"generate_image_timing: phase=input_prepare task=%s request_id=%s channel=%d client_format=%s upstream_format=%s conversion=%s image_count=%d input_bytes=%d download_ms=%.3f decode_ms=%.3f local_write_ms=%.3f prepare_ms=%.3f fallback=%s error=%t",
			task.TaskID,
			task.PrivateData.RequestID,
			task.ChannelId,
			inputPreparation.ClientFormat,
			inputPreparation.UpstreamFormat,
			inputPreparation.Conversion,
			inputPreparation.ImageCount,
			inputPreparation.InputBytes,
			inputPreparation.Download.Seconds()*1000,
			inputPreparation.Decode.Seconds()*1000,
			inputPreparation.LocalWrite.Seconds()*1000,
			inputPreparation.Total.Seconds()*1000,
			inputPreparation.Fallback,
			convertErr != nil,
		))
		if inputPreparation.Fallback != "none" {
			logger.LogWarn(c, fmt.Sprintf(
				"generate_image: task=%s request_id=%s channel=%d file_data_fallback=%s",
				task.TaskID, task.PrivateData.RequestID, task.ChannelId, inputPreparation.Fallback,
			))
		}
		if convertErr != nil {
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			generateImageError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("转换请求格式失败: %v", convertErr))
			return
		}
		applyGenerateImageGoogleSearchTool(nativeReq, req.GoogleSearch)
		if validateErr := service.ValidateGeminiGenerateContentRequestSize(nativeReq); validateErr != nil {
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			generateImageError(c, http.StatusBadRequest, "invalid_request_error", validateErr.Error())
			return
		}
		service.SetGenerateContentRequestOmittedData(task, "submitted")
		processData = nativeReq
	} else {
		task.SetData(asyncReq)
		processData = asyncReq
	}

	taskInsertStartedAt := time.Now()
	if err := task.Insert(); err != nil {
		if relayInfo != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		generateImageError(c, http.StatusInternalServerError, "internal_error", fmt.Sprintf("创建任务失败: %v", err))
		return
	}
	taskInsertFinishedAt := time.Now()

	// 记录提交时的消费日志，并持久化 log ID 供完成后更新
	submitLogStartedAt := time.Now()
	service.RecordAsyncImageSubmitLog(c, task, asyncReq, relayInfo)
	submitLogFinishedAt := time.Now()
	taskUpdateStartedAt := time.Now()
	_ = task.Update()
	taskUpdateFinishedAt := time.Now()

	requestBodyBytes := c.Request.ContentLength
	if storageValue, ok := c.Get(common.KeyBodyStorage); ok {
		if storage, ok := storageValue.(common.BodyStorage); ok {
			requestBodyBytes = storage.Size()
		}
	}
	requestReceivedAt := common.GetContextKeyTime(c, constant.ContextKeyRequestReceivedTime)
	requestBodyReadyAt := common.GetContextKeyTime(c, constant.ContextKeyRequestBodyReadyTime)
	if requestReceivedAt.IsZero() {
		requestReceivedAt = handlerStartedAt
	}
	if requestBodyReadyAt.IsZero() {
		requestBodyReadyAt = handlerStartedAt
	}
	logger.LogInfo(c, fmt.Sprintf(
		"generate_image_timing: phase=submit task=%s request_id=%s host=%s model=%s channel=%d request_bytes=%d body_receive_ms=%.3f decode_validate_ms=%.3f billing_ms=%.3f prepare_ms=%.3f task_insert_ms=%.3f submit_log_ms=%.3f task_update_ms=%.3f post_body_submit_ms=%.3f total_submit_ms=%.3f",
		task.TaskID,
		task.PrivateData.RequestID,
		task.Properties.RequestHost,
		task.Properties.OriginModelName,
		task.ChannelId,
		requestBodyBytes,
		requestBodyReadyAt.Sub(requestReceivedAt).Seconds()*1000,
		billingStartedAt.Sub(handlerStartedAt).Seconds()*1000,
		billingFinishedAt.Sub(billingStartedAt).Seconds()*1000,
		taskInsertStartedAt.Sub(billingFinishedAt).Seconds()*1000,
		taskInsertFinishedAt.Sub(taskInsertStartedAt).Seconds()*1000,
		submitLogFinishedAt.Sub(submitLogStartedAt).Seconds()*1000,
		taskUpdateFinishedAt.Sub(taskUpdateStartedAt).Seconds()*1000,
		time.Since(requestBodyReadyAt).Seconds()*1000,
		time.Since(requestReceivedAt).Seconds()*1000,
	))

	ctx := context.WithValue(context.Background(), "gin_context", c)
	gopool.Go(func() {
		service.ProcessGenerateImageTask(ctx, task, processData)
	})

	c.JSON(http.StatusOK, dto.AsyncTaskResponse{
		TaskID: task.TaskID,
		Status: string(task.Status),
	})
}

// generateImageToAsyncRequest 把统一生图入参映射为既有的 AsyncImageRequest，
// 以复用现成的图片校验、Gemini 转换、提交日志逻辑。
func generateImageToAsyncRequest(req *dto.GenerateImageRequest) *dto.AsyncImageRequest {
	images := make([]string, 0, len(req.Images))
	for _, input := range req.Images {
		images = append(images, input.LegacyString())
	}
	return &dto.AsyncImageRequest{
		Model:              req.Model,
		Prompt:             req.Prompt,
		N:                  req.N,
		Size:               req.Size,
		Quality:            req.Quality,
		Background:         req.Background,
		Moderation:         req.Moderation,
		AspectRatio:        req.AspectRatio,
		OutputFormat:       req.OutputFormat,
		ResponseModalities: req.ResponseModalities,
		MediaResolution:    req.MediaResolution,
		ThinkingLevel:      req.ThinkingLevel,
		IncludeThoughts:    req.IncludeThoughts,
		Images:             images,
		Mask:               req.Mask,
	}
}

func applyGenerateImageGoogleSearchTool(nativeReq map[string]interface{}, googleSearch *bool) {
	if nativeReq == nil || googleSearch == nil || !*googleSearch {
		return
	}
	nativeReq["tools"] = []interface{}{
		map[string]interface{}{
			"googleSearch": map[string]interface{}{},
		},
	}
}

func unmarshalGenerateImageBody(c *gin.Context, v any) error {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return err
	}
	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if storage.IsDisk() {
		if err := common.DecodeJson(storage, v); err != nil {
			return err
		}
	} else {
		requestBody, err := storage.Bytes()
		if err != nil {
			return err
		}
		if err := common.Unmarshal(requestBody, v); err != nil {
			return err
		}
	}

	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return err
	}
	c.Request.Body = io.NopCloser(storage)
	return nil
}

// generateImageError 统一错误响应格式。错误信息原样透传，不做友好化改写。
func generateImageError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}
