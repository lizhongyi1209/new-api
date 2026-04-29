package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type ImageAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error)
	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
}

type GeminiAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
}

var GetImageAdaptorFunc func(apiType int) ImageAdaptor
var GetGeminiAdaptorFunc func(apiType int) GeminiAdaptor
var CalculatePriceFunc func(c *gin.Context, info *relaycommon.RelayInfo) (types.PriceData, error)

func applyModelMapping(originModelName string, modelMappingJSON *string) string {
	if modelMappingJSON == nil || *modelMappingJSON == "" || *modelMappingJSON == "{}" {
		return originModelName
	}

	var modelMap map[string]string
	if err := common.Unmarshal([]byte(*modelMappingJSON), &modelMap); err != nil {
		return originModelName
	}

	currentModel := originModelName
	visited := map[string]bool{currentModel: true}
	for {
		if mappedModel, exists := modelMap[currentModel]; exists && mappedModel != "" {
			if visited[mappedModel] {
				break
			}
			visited[mappedModel] = true
			currentModel = mappedModel
		} else {
			break
		}
	}
	return currentModel
}

// RecordAsyncImageSubmitLog records a usage log at task submission time.
// It returns the log ID so the task can update the log with actual data on completion.
func RecordAsyncImageSubmitLog(c *gin.Context, task *model.Task, imageReq *dto.AsyncImageRequest, relayInfo *relaycommon.RelayInfo) int {
	quota := relayInfo.PriceData.Quota

	model.UpdateUserUsedQuotaAndRequestCount(task.UserId, quota)
	model.UpdateChannelUsedQuota(task.ChannelId, quota)

	imageN := uint(1)
	if imageReq.N != nil {
		imageN = *imageReq.N
	}

	quality := "standard"
	if imageReq.Quality == "hd" {
		quality = "hd"
	}

	var logContent []string
	if len(imageReq.Size) > 0 {
		logContent = append(logContent, fmt.Sprintf("大小 %s", imageReq.Size))
	}
	if len(quality) > 0 {
		logContent = append(logContent, fmt.Sprintf("品质 %s", quality))
	}
	if imageN > 0 {
		logContent = append(logContent, fmt.Sprintf("生成数量 %d", imageN))
	}
	logContent = append(logContent, fmt.Sprintf("异步任务 %s（已提交）", task.TaskID))

	other := make(map[string]interface{})
	other["model_ratio"] = relayInfo.PriceData.ModelRatio
	other["group_ratio"] = relayInfo.PriceData.GroupRatioInfo.GroupRatio
	other["model_price"] = relayInfo.PriceData.ModelPrice
	other["user_group_ratio"] = relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio
	other["async_task_id"] = task.TaskID
	other["request_path"] = "/async/v1/images/generations"
	other["per_call_billing"] = relayInfo.PriceData.UsePrice
	other["completion_ratio"] = ratio_setting.GetCompletionRatio(imageReq.Model)

	adminInfo := make(map[string]interface{})
	adminInfo["use_channel"] = []string{fmt.Sprintf("%d", task.ChannelId)}
	other["admin_info"] = adminInfo

	tokenName := ""
	if task.Properties.TokenId > 0 {
		if token, err := model.GetTokenById(task.Properties.TokenId); err == nil {
			tokenName = token.Name
		}
	}

	logId := model.RecordConsumeLog(c, task.UserId, model.RecordConsumeLogParams{
		ChannelId:        task.ChannelId,
		PromptTokens:     0,
		CompletionTokens: 0,
		ModelName:        imageReq.Model,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          strings.Join(logContent, ", "),
		TokenId:          task.Properties.TokenId,
		UseTimeSeconds:   0,
		IsStream:         false,
		Group:            task.Group,
		Other:            other,
	})

	task.PrivateData.SubmitLogID = logId
	return logId
}

// RecordAsyncGeminiSubmitLog records a usage log at Gemini task submission time.
// It returns the log ID so the task can update the log with actual data on completion.
func RecordAsyncGeminiSubmitLog(c *gin.Context, task *model.Task, modelName string, relayInfo *relaycommon.RelayInfo) int {
	quota := relayInfo.PriceData.Quota

	model.UpdateUserUsedQuotaAndRequestCount(task.UserId, quota)
	model.UpdateChannelUsedQuota(task.ChannelId, quota)

	logContent := fmt.Sprintf("Gemini 图片生成，异步任务 %s（已提交）", task.TaskID)

	other := make(map[string]interface{})
	other["model_ratio"] = relayInfo.PriceData.ModelRatio
	other["group_ratio"] = relayInfo.PriceData.GroupRatioInfo.GroupRatio
	other["model_price"] = relayInfo.PriceData.ModelPrice
	other["user_group_ratio"] = relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio
	other["async_task_id"] = task.TaskID
	other["request_path"] = "/async/v1beta/models/" + modelName + ":generateContent"
	other["per_call_billing"] = relayInfo.PriceData.UsePrice
	other["completion_ratio"] = ratio_setting.GetCompletionRatio(modelName)

	adminInfo := make(map[string]interface{})
	adminInfo["use_channel"] = []string{fmt.Sprintf("%d", task.ChannelId)}
	other["admin_info"] = adminInfo

	tokenName := ""
	if task.Properties.TokenId > 0 {
		if token, err := model.GetTokenById(task.Properties.TokenId); err == nil {
			tokenName = token.Name
		}
	}

	logId := model.RecordConsumeLog(c, task.UserId, model.RecordConsumeLogParams{
		ChannelId:        task.ChannelId,
		PromptTokens:     0,
		CompletionTokens: 0,
		ModelName:        modelName,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          task.Properties.TokenId,
		UseTimeSeconds:   0,
		IsStream:         false,
		Group:            task.Group,
		Other:            other,
	})

	task.PrivateData.SubmitLogID = logId
	return logId
}

func ProcessAsyncImageTask(ctx context.Context, task *model.Task) {
	// Refund pre-consumed quota if task ends in failure
	defer func() {
		if task.Status == model.TaskStatusFailure {
			RefundTaskQuota(ctx, task, task.FailReason)
		}
	}()

	var asyncReq dto.AsyncImageRequest
	if err := task.GetData(&asyncReq); err != nil {
		logger.LogError(ctx, fmt.Sprintf("async_image: failed to parse task data: %v", err))
		task.Status = model.TaskStatusFailure
		task.FailReason = "解析任务数据失败"
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	imageReq := &dto.ImageRequest{
		Model:          asyncReq.Model,
		Prompt:         asyncReq.Prompt,
		N:              asyncReq.N,
		Size:           asyncReq.Size,
		Quality:        asyncReq.Quality,
		ResponseFormat: asyncReq.ResponseFormat,
		Style:          asyncReq.Style,
		User:           asyncReq.User,
	}

	// Create a new gin context for async execution
	c := &gin.Context{
		Request: &http.Request{
			Method: "POST",
			Header: make(http.Header),
			Body:   http.NoBody,
		},
	}

	// Get username for logging
	username := ""
	if user, err := model.GetUserById(task.UserId, false); err == nil {
		username = user.Username
	}
	c.Set("username", username)

	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("async_image: failed to get channel: %v", err))
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("获取渠道信息失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)
	key, keyIndex, keyErr := channel.GetNextEnabledKey()
	if keyErr != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("获取渠道密钥失败: %v", keyErr.Error())
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	upstreamModelName := applyModelMapping(imageReq.Model, channel.ModelMapping)
	isModelMapped := upstreamModelName != imageReq.Model

	relayInfo := &relaycommon.RelayInfo{
		UserId:     task.UserId,
		UsingGroup: task.Group,
		Request:    imageReq,
		RelayMode:  relayconstant.RelayModeImagesGenerations,
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: task.TaskID,
		},
		OriginModelName: imageReq.Model,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          channel.Type,
			ChannelId:            channel.Id,
			ChannelIsMultiKey:    channel.ChannelInfo.IsMultiKey,
			ChannelMultiKeyIndex: keyIndex,
			ChannelBaseUrl:       channel.GetBaseURL(),
			ApiType:              apiType,
			ApiKey:               key,
			UpstreamModelName:    upstreamModelName,
			IsModelMapped:        isModelMapped,
		},
	}

	task.Status = model.TaskStatusInProgress
	task.StartTime = time.Now().Unix()
	task.Progress = "50%"
	_ = task.Update()

	if GetImageAdaptorFunc == nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = "内部错误：适配器未初始化"
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	adaptor := GetImageAdaptorFunc(relayInfo.ApiType)
	if adaptor == nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("不支持的 API 类型: %d", relayInfo.ApiType)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}
	adaptor.Init(relayInfo)

	// Calculate price for billing
	if CalculatePriceFunc != nil {
		if priceData, err := CalculatePriceFunc(c, relayInfo); err == nil {
			relayInfo.PriceData = priceData
		}
	}

	convertedRequest, err := adaptor.ConvertImageRequest(c, relayInfo, *imageReq)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("转换请求失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("序列化请求失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	resp, err := adaptor.DoRequest(c, relayInfo, bytes.NewReader(jsonData))
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("请求上游失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	httpResp := resp.(*http.Response)
	if httpResp.StatusCode != http.StatusOK {
		relayErr := RelayErrorHandler(ctx, httpResp, false)
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("上游返回错误: %s", relayErr.Error())
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("读取响应失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	var imageResp dto.ImageResponse
	if err := common.Unmarshal(bodyBytes, &imageResp); err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("解析响应失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	if len(imageResp.Data) == 0 {
		task.Status = model.TaskStatusFailure
		task.FailReason = "上游未返回图片数据"
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	// Process response based on response_format
	responseFormat := imageReq.ResponseFormat
	if responseFormat == "" {
		responseFormat = "url" // default
	}

	var uploadedURLs []string
	var resultData map[string]interface{}

	if responseFormat == "b64_json" {
		// Return base64 directly
		b64List := []string{}
		for _, imgData := range imageResp.Data {
			if imgData.B64Json != "" {
				b64List = append(b64List, imgData.B64Json)
			}
		}
		resultData = map[string]interface{}{
			"data": b64List,
		}
	} else {
		// Upload to R2 and return URLs
		for _, imgData := range imageResp.Data {
			if imgData.B64Json != "" {
				publicURL, err := UploadBase64ImageToR2("image/png", imgData.B64Json)
				if err != nil {
					logger.LogError(ctx, fmt.Sprintf("async_image: R2 upload failed: %v", err))
					task.Status = model.TaskStatusFailure
					task.FailReason = fmt.Sprintf("上传图片到 R2 失败: %v", err)
					task.Progress = "100%"
					task.FinishTime = time.Now().Unix()
					_ = task.Update()
					return
				}
				uploadedURLs = append(uploadedURLs, publicURL)
			} else if imgData.Url != "" {
				uploadedURLs = append(uploadedURLs, imgData.Url)
			}
		}
		resultData = map[string]interface{}{
			"urls": uploadedURLs,
		}
		if len(uploadedURLs) > 0 {
			task.PrivateData.ResultURL = uploadedURLs[0]
		}
	}

	task.SetData(resultData)
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	_ = task.Update()

	// Update submission-time log with actual completion data
	useTime := int(task.FinishTime - task.StartTime)
	actualImageCount := len(imageResp.Data)
	updateContent := buildAsyncImageCompleteContent(imageReq, actualImageCount, task.TaskID)
	otherUpdates := map[string]interface{}{
		"task_status":          "SUCCESS",
		"generated_image_count": actualImageCount,
	}
	model.UpdateConsumeLogOnComplete(task.PrivateData.SubmitLogID, useTime, 0, 0, updateContent, otherUpdates)

	logger.LogInfo(ctx, fmt.Sprintf("async_image: task %s completed, generated %d images", task.TaskID, actualImageCount))
}

// stripThoughtSignature removes thoughtSignature from all parts in a Gemini response.
// Gemini thinking models include a massive base64-encoded thoughtSignature in each part,
// which can be several megabytes and bloats the stored task data.
func stripThoughtSignature(geminiResp map[string]interface{}) {
	candidates, ok := geminiResp["candidates"].([]interface{})
	if !ok {
		return
	}
	for _, candidate := range candidates {
		candidateMap, ok := candidate.(map[string]interface{})
		if !ok {
			continue
		}
		content, ok := candidateMap["content"].(map[string]interface{})
		if !ok {
			continue
		}
		parts, ok := content["parts"].([]interface{})
		if !ok {
			continue
		}
		for _, part := range parts {
			if partMap, ok := part.(map[string]interface{}); ok {
				delete(partMap, "thoughtSignature")
			}
		}
	}
}

func buildAsyncImageCompleteContent(imageReq *dto.ImageRequest, actualImageCount int, taskID string) string {
	quality := "standard"
	if imageReq.Quality == "hd" {
		quality = "hd"
	}
	var parts []string
	if len(imageReq.Size) > 0 {
		parts = append(parts, fmt.Sprintf("大小 %s", imageReq.Size))
	}
	parts = append(parts, fmt.Sprintf("品质 %s", quality))
	parts = append(parts, fmt.Sprintf("生成 %d 张图片", actualImageCount))
	parts = append(parts, fmt.Sprintf("异步任务 %s（已完成）", taskID))
	return strings.Join(parts, ", ")
}

func ProcessAsyncGeminiTask(ctx context.Context, task *model.Task) {
	// Refund pre-consumed quota if task ends in failure
	defer func() {
		if task.Status == model.TaskStatusFailure {
			RefundTaskQuota(ctx, task, task.FailReason)
		}
	}()

	// Create a new gin context for async execution
	c := &gin.Context{
		Request: &http.Request{
			Method: "POST",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: http.NoBody,
		},
	}

	// Get username for logging
	username := ""
	if user, err := model.GetUserById(task.UserId, false); err == nil {
		username = user.Username
	}
	c.Set("username", username)

	task.Status = model.TaskStatusInProgress
	task.StartTime = time.Now().Unix()
	task.Progress = "50%"
	_ = task.Update()

	var requestBody map[string]interface{}
	if err := task.GetData(&requestBody); err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("解析请求数据失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	// Apply Gemini request normalization (set default role for first content)
	if contents, ok := requestBody["contents"].([]interface{}); ok && len(contents) > 0 {
		if firstContent, ok := contents[0].(map[string]interface{}); ok {
			if _, hasRole := firstContent["role"]; !hasRole {
				firstContent["role"] = "user"
				logger.LogInfo(ctx, fmt.Sprintf("async_gemini: set default role=user for first content"))
			}
		}
	} else {
		logger.LogError(ctx, fmt.Sprintf("async_gemini: failed to normalize contents, type=%T", requestBody["contents"]))
	}

	jsonData, err := common.Marshal(requestBody)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("序列化请求失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("async_gemini: request body after normalization: %s", string(jsonData)))

	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("获取渠道信息失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)
	key, keyIndex, keyErr := channel.GetNextEnabledKey()
	if keyErr != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("获取渠道密钥失败: %v", keyErr.Error())
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	upstreamModelName := applyModelMapping(task.Properties.OriginModelName, channel.ModelMapping)
	isModelMapped := upstreamModelName != task.Properties.OriginModelName

	relayInfo := &relaycommon.RelayInfo{
		UserId:          task.UserId,
		UsingGroup:      task.Group,
		RelayMode:       relayconstant.RelayModeGemini,
		OriginModelName: task.Properties.OriginModelName,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          channel.Type,
			ChannelId:            channel.Id,
			ChannelIsMultiKey:    channel.ChannelInfo.IsMultiKey,
			ChannelMultiKeyIndex: keyIndex,
			ChannelBaseUrl:       channel.GetBaseURL(),
			ApiType:              apiType,
			ApiVersion:           channel.Other,
			ApiKey:               key,
			UpstreamModelName:    upstreamModelName,
			IsModelMapped:        isModelMapped,
		},
	}

	if GetGeminiAdaptorFunc == nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = "内部错误：Gemini 适配器未初始化"
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	adaptor := GetGeminiAdaptorFunc(relayInfo.ApiType)
	if adaptor == nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("不支持的 API 类型: %d", relayInfo.ApiType)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}
	adaptor.Init(relayInfo)

	// Calculate price for billing
	if CalculatePriceFunc != nil {
		if priceData, err := CalculatePriceFunc(c, relayInfo); err == nil {
			relayInfo.PriceData = priceData
		}
	}

	// Debug: log request details
	logger.LogInfo(ctx, fmt.Sprintf("async_gemini: calling upstream with model=%s, baseUrl=%s, apiType=%d",
		relayInfo.ChannelMeta.UpstreamModelName, relayInfo.ChannelMeta.ChannelBaseUrl, relayInfo.ChannelMeta.ApiType))
	logger.LogInfo(ctx, fmt.Sprintf("async_gemini: request body length=%d bytes", len(jsonData)))

	resp, err := adaptor.DoRequest(c, relayInfo, bytes.NewReader(jsonData))
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("async_gemini: DoRequest failed: %v", err))
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("请求上游失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	httpResp := resp.(*http.Response)
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		// Debug: log response details
		bodyPreview, _ := io.ReadAll(httpResp.Body)
		httpResp.Body = io.NopCloser(bytes.NewReader(bodyPreview))
		logger.LogError(ctx, fmt.Sprintf("async_gemini: upstream error status=%d, body=%s", httpResp.StatusCode, string(bodyPreview)))

		relayErr := RelayErrorHandler(ctx, httpResp, false)
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("上游返回错误: %s", relayErr.Error())
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("读取响应失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	var geminiResp map[string]interface{}
	if err := common.Unmarshal(bodyBytes, &geminiResp); err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("解析响应失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	// Extract token usage from response
	promptTokens := 0
	completionTokens := 0
	var tokenDetails map[string]interface{}
	if usageMetadata, ok := geminiResp["usageMetadata"].(map[string]interface{}); ok {
		if pt, ok := usageMetadata["promptTokenCount"].(float64); ok {
			promptTokens = int(pt)
		}
		if ct, ok := usageMetadata["candidatesTokenCount"].(float64); ok {
			completionTokens = int(ct)
		}
		tokenDetails = map[string]interface{}{}
		if tt, ok := usageMetadata["totalTokenCount"].(float64); ok {
			tokenDetails["total_tokens"] = int(tt)
		}
		if tt, ok := usageMetadata["thoughtsTokenCount"].(float64); ok {
			tokenDetails["thought_tokens"] = int(tt)
		}
	}

	// Strip thoughtSignature from parts before storing (it can be megabytes of base64)
	stripThoughtSignature(geminiResp)

	// Extract and upload images to R2
	var firstImageURL string
	imageCount := 0
	if candidates, ok := geminiResp["candidates"].([]interface{}); ok && len(candidates) > 0 {
		for _, candidate := range candidates {
			if candidateMap, ok := candidate.(map[string]interface{}); ok {
				if content, ok := candidateMap["content"].(map[string]interface{}); ok {
					if parts, ok := content["parts"].([]interface{}); ok {
						for _, part := range parts {
							if partMap, ok := part.(map[string]interface{}); ok {
								if inlineData, ok := partMap["inlineData"].(map[string]interface{}); ok {
									if base64Data, ok := inlineData["data"].(string); ok {
										mimeType := "image/png"
										if mt, ok := inlineData["mimeType"].(string); ok {
											mimeType = mt
										}
										publicURL, err := UploadBase64ImageToR2(mimeType, base64Data)
										if err != nil {
											logger.LogError(ctx, fmt.Sprintf("async_gemini: R2 upload failed: %v", err))
											task.Status = model.TaskStatusFailure
											task.FailReason = fmt.Sprintf("上传图片到 R2 失败: %v", err)
											task.Progress = "100%"
											task.FinishTime = time.Now().Unix()
											_ = task.Update()
											return
										}
										// Skip thought images (intermediate model refinements, not final output)
										isThought, _ := partMap["thought"].(bool)
										if !isThought {
											imageCount++
										}
										// Replace inline data with URL
										delete(partMap, "inlineData")
										partMap["imageUrl"] = publicURL
										if firstImageURL == "" && !isThought {
											firstImageURL = publicURL
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	task.SetData(geminiResp)
	if firstImageURL != "" {
		task.PrivateData.ResultURL = firstImageURL
	}
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	_ = task.Update()

	// Update submission-time log with actual completion data
	useTime := int(task.FinishTime - task.StartTime)
	updateContent := fmt.Sprintf("Gemini 图片生成，生成 %d 张图片，异步任务 %s（已完成）", imageCount, task.TaskID)
	otherUpdates := map[string]interface{}{
		"task_status":           "SUCCESS",
		"generated_image_count": imageCount,
	}
	for k, v := range tokenDetails {
		otherUpdates[k] = v
	}
	model.UpdateConsumeLogOnComplete(task.PrivateData.SubmitLogID, useTime, promptTokens, completionTokens, updateContent, otherUpdates)

	// Settle billing: per-token models need post-completion recalculation with actual token counts
	if bc := task.PrivateData.BillingContext; bc != nil && !bc.PerCallBilling {
		RecalculateTaskQuotaByTokens(ctx, task, promptTokens, completionTokens)
	}

	logger.LogInfo(ctx, fmt.Sprintf("async_gemini: task %s completed, generated %d images, tokens: p=%d c=%d", task.TaskID, imageCount, promptTokens, completionTokens))
}
