package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

// imageProvider 标识一个生图模型应走哪条处理路径。
type imageProvider string

const (
	imageProviderGeminiNative imageProvider = "gemini_native"
	imageProviderOpenAIImage  imageProvider = "openai_image"
)

// imageRoute 是一条分发规则：模型名匹配 match 时走对应 provider。
// provider 与 action 一起决定提交时如何构造任务、处理时走哪个分支。
// 扩展新 provider 只需在 imageRoutes 增加一条规则，并在 ProcessGenerateImageTask 增加一个 case。
type imageRoute struct {
	match    func(modelName string) bool
	provider imageProvider
	action   string
}

// imageRoutes 是统一生图端点的分发注册表，按顺序匹配，第一条命中即生效。
var imageRoutes = []imageRoute{
	{match: isGeminiImageModelName, provider: imageProviderGeminiNative, action: "generateContent"},
}

// isGeminiImageModelName 判定模型是否走 Gemini 原生生图路径。
// 规则与 controller/async_image.go 中既有的 isGeminiImageModel 保持一致。
func isGeminiImageModelName(modelName string) bool {
	return strings.HasPrefix(modelName, "nano-banana") ||
		modelName == "gemini-3-pro-image" ||
		modelName == "gemini-3.1-flash-image-preview"
}

// resolveImageRoute 根据模型名 + 渠道类型选出处理路径。
// Gemini 原生路径要求渠道本身是 Gemini/Vertex；否则即使模型名匹配也回退到通用 OpenAI image 路径。
func resolveImageRoute(modelName string, channelType int) imageRoute {
	isGeminiChannel := channelType == constant.ChannelTypeGemini || channelType == constant.ChannelTypeVertexAi
	for _, r := range imageRoutes {
		if !r.match(modelName) {
			continue
		}
		if r.provider == imageProviderGeminiNative && !isGeminiChannel {
			continue
		}
		return r
	}
	// 兜底：通用 OpenAI image 适配器路径。
	return imageRoute{provider: imageProviderOpenAIImage, action: "generate"}
}

// ResolveImageRoute 是 resolveImageRoute 的导出包装，供控制器在提交阶段决定任务构造方式。
// 返回 (action, isGeminiNative)。
func ResolveImageRoute(modelName string, channelType int) (action string, isGeminiNative bool) {
	r := resolveImageRoute(modelName, channelType)
	return r.action, r.provider == imageProviderGeminiNative
}

// failGenerateImageTask 把任务标记为失败并落库。退款由 ProcessGenerateImageTask 的 defer 统一处理。
func failGenerateImageTask(task *model.Task, reason string) {
	task.Status = model.TaskStatusFailure
	task.FailReason = reason
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	_ = task.Update()
}

// newAsyncGinContext 构造一个用于异步执行的最小 gin.Context，并写入用户名供日志使用。
func newAsyncGinContext(userId int) *gin.Context {
	c := &gin.Context{
		Request: &http.Request{
			Method: "POST",
			Header: http.Header{"Content-Type": []string{"application/json"}},
			Body:   http.NoBody,
		},
	}
	if user, err := model.GetUserById(userId, false); err == nil {
		c.Set("username", user.Username)
	}
	return c
}

// ProcessGenerateImageTask 是统一生图端点的异步处理入口，按 task.Action 分发到具体 provider。
func ProcessGenerateImageTask(ctx context.Context, task *model.Task) {
	defer func() {
		if r := recover(); r != nil {
			logger.LogError(ctx, fmt.Sprintf("generate_image: panic recovered: %v", r))
			failGenerateImageTask(task, fmt.Sprintf("内部错误 (panic): %v", r))
		}
		if task.Status == model.TaskStatusFailure {
			RefundTaskQuota(ctx, task, task.FailReason)
		}
	}()

	c := newAsyncGinContext(task.UserId)

	task.Status = model.TaskStatusInProgress
	task.StartTime = time.Now().Unix()
	task.Progress = "50%"
	_ = task.Update()

	switch task.Action {
	case "generateContent":
		processGenerateImageGemini(ctx, c, task)
	default:
		processGenerateImageOpenAI(ctx, c, task)
	}
}

// buildGenerateImageRelayInfo 构造处理阶段所需的 RelayInfo（含渠道密钥、模型映射、价格）。
func buildGenerateImageRelayInfo(c *gin.Context, task *model.Task, relayMode int) (*relaycommon.RelayInfo, error) {
	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		return nil, fmt.Errorf("获取渠道信息失败: %v", err)
	}
	apiType, _ := common.ChannelType2APIType(channel.Type)
	key, keyIndex, keyErr := channel.GetNextEnabledKey()
	if keyErr != nil {
		return nil, fmt.Errorf("获取渠道密钥失败: %v", keyErr.Error())
	}
	upstreamModelName := ApplyModelMapping(task.Properties.OriginModelName, channel.ModelMapping)
	relayInfo := &relaycommon.RelayInfo{
		UserId:          task.UserId,
		UserGroup:       common.GetContextKeyString(c, constant.ContextKeyUserGroup),
		UsingGroup:      task.Group,
		RelayMode:       relayMode,
		OriginModelName: task.Properties.OriginModelName,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: task.TaskID},
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
			IsModelMapped:        upstreamModelName != task.Properties.OriginModelName,
		},
	}
	if CalculatePriceFunc != nil {
		if priceData, err := CalculatePriceFunc(c, relayInfo); err == nil {
			relayInfo.PriceData = priceData
		}
	}
	return relayInfo, nil
}

// processGenerateImageGemini 处理 Gemini 原生 generateContent 路径。
// 与 ProcessUnifiedImageTask 的区别：不上传 R2，直接把 inlineData(base64) 原样提取返回。
func processGenerateImageGemini(ctx context.Context, c *gin.Context, task *model.Task) {
	var requestBody map[string]interface{}
	if err := task.GetData(&requestBody); err != nil {
		failGenerateImageTask(task, fmt.Sprintf("解析请求数据失败: %v", err))
		return
	}
	delete(requestBody, "image_compression") // 客户端参数，不透传上游

	// 规范化：首个 content 缺 role 时补 user
	if contents, ok := requestBody["contents"].([]interface{}); ok && len(contents) > 0 {
		if first, ok := contents[0].(map[string]interface{}); ok {
			if _, has := first["role"]; !has {
				first["role"] = "user"
			}
		}
	}

	jsonData, err := common.Marshal(requestBody)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("序列化请求失败: %v", err))
		return
	}

	relayInfo, err := buildGenerateImageRelayInfo(c, task, relayconstant.RelayModeGemini)
	if err != nil {
		failGenerateImageTask(task, err.Error())
		return
	}

	if GetGeminiAdaptorFunc == nil {
		failGenerateImageTask(task, "内部错误：Gemini 适配器未初始化")
		return
	}
	adaptor := GetGeminiAdaptorFunc(relayInfo.ApiType)
	if adaptor == nil {
		failGenerateImageTask(task, fmt.Sprintf("不支持的 API 类型: %d", relayInfo.ApiType))
		return
	}
	adaptor.Init(relayInfo)

	logger.LogInfo(ctx, fmt.Sprintf("generate_image(gemini): model=%s, baseUrl=%s, bodyLen=%d",
		relayInfo.UpstreamModelName, relayInfo.ChannelBaseUrl, len(jsonData)))

	resp, err := adaptor.DoRequest(c, relayInfo, bytes.NewReader(jsonData))
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("请求上游失败: %v", err))
		return
	}
	httpResp := resp.(*http.Response)
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		httpResp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		logger.LogError(ctx, fmt.Sprintf("generate_image(gemini): upstream error status=%d body=%s", httpResp.StatusCode, string(bodyBytes)))
		relayErr := RelayErrorHandler(ctx, httpResp, false)
		failGenerateImageTask(task, fmt.Sprintf("上游返回错误: %s", relayErr.Error()))
		return
	}
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("读取响应失败: %v", err))
		return
	}

	var geminiResp map[string]interface{}
	if err := common.Unmarshal(bodyBytes, &geminiResp); err != nil {
		failGenerateImageTask(task, fmt.Sprintf("解析响应失败: %v", err))
		return
	}

	promptTokens, completionTokens, tokenDetails := extractGeminiUsage(geminiResp)
	images := extractGeminiImages(geminiResp)
	if len(images) == 0 {
		failGenerateImageTask(task, "上游未返回图片数据")
		return
	}

	finalizeGenerateImageTask(ctx, task, images, promptTokens, completionTokens, tokenDetails,
		relayInfo.UpstreamModelName, relayInfo.IsModelMapped, asString(geminiResp["modelVersion"]))
}

// extractGeminiUsage 从 Gemini 响应的 usageMetadata 提取 token 用量。
func extractGeminiUsage(geminiResp map[string]interface{}) (promptTokens, completionTokens int, details map[string]interface{}) {
	details = map[string]interface{}{}
	usage, ok := geminiResp["usageMetadata"].(map[string]interface{})
	if !ok {
		return 0, 0, details
	}
	if pt, ok := usage["promptTokenCount"].(float64); ok {
		promptTokens = int(pt)
	}
	if ct, ok := usage["candidatesTokenCount"].(float64); ok {
		completionTokens = int(ct)
	}
	if tt, ok := usage["totalTokenCount"].(float64); ok {
		details["total_tokens"] = int(tt)
	}
	if th, ok := usage["thoughtsTokenCount"].(float64); ok {
		completionTokens += int(th)
		details["thought_tokens"] = int(th)
	}
	return promptTokens, completionTokens, details
}

// extractGeminiImages 从 Gemini 响应的 candidates.parts 提取图片（base64），跳过 thought 部分。
func extractGeminiImages(geminiResp map[string]interface{}) []dto.GenerateImageData {
	var images []dto.GenerateImageData
	candidates, ok := geminiResp["candidates"].([]interface{})
	if !ok {
		return images
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
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			if isThought, _ := partMap["thought"].(bool); isThought {
				continue
			}
			inlineData, ok := partMap["inlineData"].(map[string]interface{})
			if !ok {
				continue
			}
			b64, ok := inlineData["data"].(string)
			if !ok || b64 == "" {
				continue
			}
			mimeType := "image/png"
			if mt, ok := inlineData["mimeType"].(string); ok && mt != "" {
				mimeType = mt
			}
			images = append(images, dto.GenerateImageData{B64Json: b64, MimeType: mimeType})
		}
	}
	return images
}

// asString 安全地把 interface{} 转为 string。
func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

// processGenerateImageOpenAI 处理通用 OpenAI image 适配器路径（兜底 provider）。
// 任务数据存的是 dto.AsyncImageRequest。与 ProcessAsyncImageTask 的区别：不上传 R2，
// 直接把上游返回的 b64_json / url 原样提取返回。
func processGenerateImageOpenAI(ctx context.Context, c *gin.Context, task *model.Task) {
	var asyncReq dto.AsyncImageRequest
	if err := task.GetData(&asyncReq); err != nil {
		failGenerateImageTask(task, fmt.Sprintf("解析任务数据失败: %v", err))
		return
	}

	// 参考图：URL → base64 data-uri（供上游使用）
	resolvedImage, resolvedImages, err := resolveReferenceImagesForUpstream(asyncReq.Image, asyncReq.Images)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("下载参考图片失败: %v", err))
		return
	}

	imageReq := &dto.ImageRequest{
		Model:          asyncReq.Model,
		Prompt:         asyncReq.Prompt,
		N:              asyncReq.N,
		Size:           asyncReq.Size,
		AspectRatio:    asyncReq.AspectRatio,
		Quality:        asyncReq.Quality,
		ResponseFormat: asyncReq.ResponseFormat,
		Style:          asyncReq.Style,
		User:           asyncReq.User,
		Image:          resolvedImage,
		Images:         resolvedImages,
	}

	relayInfo, err := buildGenerateImageRelayInfo(c, task, relayconstant.RelayModeImagesGenerations)
	if err != nil {
		failGenerateImageTask(task, err.Error())
		return
	}
	relayInfo.Request = imageReq

	if GetImageAdaptorFunc == nil {
		failGenerateImageTask(task, "内部错误：适配器未初始化")
		return
	}
	adaptor := GetImageAdaptorFunc(relayInfo.ApiType)
	if adaptor == nil {
		failGenerateImageTask(task, fmt.Sprintf("不支持的 API 类型: %d", relayInfo.ApiType))
		return
	}
	adaptor.Init(relayInfo)

	convertedRequest, err := adaptor.ConvertImageRequest(c, relayInfo, *imageReq)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("转换请求失败: %v", err))
		return
	}
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("序列化请求失败: %v", err))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("generate_image(openai): model=%s, baseUrl=%s, bodyLen=%d",
		relayInfo.UpstreamModelName, relayInfo.ChannelBaseUrl, len(jsonData)))

	resp, err := adaptor.DoRequest(c, relayInfo, bytes.NewReader(jsonData))
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("请求上游失败: %v", err))
		return
	}
	httpResp := resp.(*http.Response)
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		httpResp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		logger.LogError(ctx, fmt.Sprintf("generate_image(openai): upstream error status=%d body=%s", httpResp.StatusCode, string(bodyBytes)))
		relayErr := RelayErrorHandler(ctx, httpResp, false)
		failGenerateImageTask(task, fmt.Sprintf("上游返回错误: %s", relayErr.Error()))
		return
	}
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		failGenerateImageTask(task, fmt.Sprintf("读取响应失败: %v", err))
		return
	}

	var imageResp dto.ImageResponse
	if err := common.Unmarshal(bodyBytes, &imageResp); err != nil {
		failGenerateImageTask(task, fmt.Sprintf("解析响应失败: %v", err))
		return
	}
	if len(imageResp.Data) == 0 {
		failGenerateImageTask(task, "上游未返回图片数据")
		return
	}

	images := make([]dto.GenerateImageData, 0, len(imageResp.Data))
	for _, d := range imageResp.Data {
		if d.B64Json != "" {
			images = append(images, dto.GenerateImageData{B64Json: d.B64Json, MimeType: detectImageMimeType(d.B64Json)})
		} else if d.Url != "" {
			images = append(images, dto.GenerateImageData{Url: d.Url})
		}
	}
	if len(images) == 0 {
		failGenerateImageTask(task, "上游未返回图片数据")
		return
	}

	promptTokens, completionTokens, tokenDetails, modelVersion := extractOpenAIImageUsage(bodyBytes)
	finalizeGenerateImageTask(ctx, task, images, promptTokens, completionTokens, tokenDetails,
		relayInfo.UpstreamModelName, relayInfo.IsModelMapped, modelVersion)
}

// extractOpenAIImageUsage 从 OpenAI-格式 image 响应原始体提取 token 用量与 modelVersion。
// gpt-image 系列上游返回 input_tokens/output_tokens（含 input_tokens_details.image_tokens），
// 而非 prompt_tokens/completion_tokens。这里复用 dto.SimpleResponse 同时兼容两套字段名，
// 归一逻辑与同步图像路径 OpenaiHandlerWithUsage 保持一致，避免按量计费取到 0 token。
func extractOpenAIImageUsage(bodyBytes []byte) (promptTokens, completionTokens int, details map[string]interface{}, modelVersion string) {
	details = map[string]interface{}{}

	var resp dto.SimpleResponse
	if err := common.Unmarshal(bodyBytes, &resp); err != nil {
		return 0, 0, details, ""
	}

	// 归一：把 input_tokens/output_tokens 累加进 prompt/completion（与同步路径一致）。
	if resp.InputTokens > 0 {
		resp.PromptTokens += resp.InputTokens
	}
	if resp.OutputTokens > 0 {
		resp.CompletionTokens += resp.OutputTokens
	}
	promptTokens = resp.PromptTokens
	completionTokens = resp.CompletionTokens

	if resp.TotalTokens > 0 {
		details["total_tokens"] = resp.TotalTokens
	}
	if resp.InputTokensDetails != nil && resp.InputTokensDetails.ImageTokens > 0 {
		details["image_tokens"] = resp.InputTokensDetails.ImageTokens
	}

	// modelVersion 在响应顶层，SimpleResponse 不含，单独提取。
	var raw map[string]interface{}
	if err := common.Unmarshal(bodyBytes, &raw); err == nil {
		modelVersion = asString(raw["modelVersion"])
	}

	return promptTokens, completionTokens, details, modelVersion
}

// resolveReferenceImagesForUpstream 把参考图的 http(s) URL 下载并转成 base64 data-uri，
// 非 URL（已是 base64 / data-uri）原样保留。供 OpenAI image 适配器使用。
func resolveReferenceImagesForUpstream(image json.RawMessage, images []string) (json.RawMessage, json.RawMessage, error) {
	var resolvedImage json.RawMessage
	if len(image) > 0 {
		var imageStr string
		if err := common.Unmarshal(image, &imageStr); err == nil {
			if strings.HasPrefix(imageStr, "http://") || strings.HasPrefix(imageStr, "https://") {
				mimeType, b64, err := GetImageFromUrlWithLimit(imageStr, AsyncImageMaxURLSizeMB)
				if err != nil {
					return nil, nil, err
				}
				resolved := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
				resolvedImage, _ = common.Marshal(resolved)
			} else {
				resolvedImage = image
			}
		} else {
			resolvedImage = image
		}
	}

	var resolvedImages json.RawMessage
	if len(images) > 0 {
		out := make([]string, len(images))
		var g errgroup.Group
		for i, imgURL := range images {
			i, imgURL := i, imgURL // 捕获循环变量
			if strings.HasPrefix(imgURL, "http://") || strings.HasPrefix(imgURL, "https://") {
				g.Go(func() error {
					mimeType, b64, err := GetImageFromUrlWithLimit(imgURL, AsyncImageMaxURLSizeMB)
					if err != nil {
						return err
					}
					out[i] = fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
					return nil
				})
			} else {
				out[i] = imgURL
			}
		}
		if err := g.Wait(); err != nil {
			return nil, nil, err
		}
		resolvedImages, _ = common.Marshal(out)
	}
	return resolvedImage, resolvedImages, nil
}

// finalizeGenerateImageTask 把生成结果写入任务、结算计费、更新提交日志，是两条 provider 路径的统一收尾。
func finalizeGenerateImageTask(ctx context.Context, task *model.Task, images []dto.GenerateImageData,
	promptTokens, completionTokens int, tokenDetails map[string]interface{},
	upstreamModelName string, isModelMapped bool, upstreamModelVersion string) {

	result := dto.GenerateImageResult{
		Model:   task.Properties.OriginModelName,
		Created: time.Now().Unix(),
		Images:  images,
	}
	task.SetData(result)
	if len(images) > 0 && images[0].Url != "" {
		task.PrivateData.ResultURL = images[0].Url
	}
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	_ = task.Update()

	// 按 token 计费的模型：完成后用真实用量重新结算差额
	if bc := task.PrivateData.BillingContext; bc != nil && !bc.PerCallBilling {
		if len(bc.TieredSnapshot) > 0 {
			// tiered_expr 模型：用冻结的 BillingSnapshot + 真实 token 重算
			var snap billingexpr.BillingSnapshot
			if err := common.Unmarshal(bc.TieredSnapshot, &snap); err == nil {
				params := billingexpr.TokenParams{
					P:   float64(promptTokens),
					C:   float64(completionTokens),
					Len: float64(promptTokens + completionTokens),
				}
				if imgTokens, ok := tokenDetails["image_tokens"]; ok {
					if v, ok := imgTokens.(int); ok {
						params.Img = float64(v)
						params.P -= params.Img
						if params.P < 0 {
							params.P = 0
						}
					}
				}
				tr, err := billingexpr.ComputeTieredQuota(&snap, params)
				if err == nil {
					RecalculateTaskQuota(ctx, task, tr.ActualQuotaAfterGroup,
						fmt.Sprintf("tiered_expr重算：p=%d, c=%d, img=%.0f, tier=%s",
							promptTokens, completionTokens, params.Img, tr.MatchedTier))
				} else {
					logger.LogError(ctx, fmt.Sprintf("generate_image: tiered settle failed: %v", err))
				}
			}
		} else {
			RecalculateTaskQuotaByTokens(ctx, task, promptTokens, completionTokens)
		}
	}

	// 0 输出 token（疑似风控）：全额退款
	if completionTokens == 0 && promptTokens > 0 && task.Quota > 0 {
		logger.LogWarn(ctx, fmt.Sprintf("generate_image: 上游返回0输出token（疑似风控），退还扣费，任务 %s，模型 %s",
			task.TaskID, task.Properties.OriginModelName))
		RefundTaskQuota(ctx, task, "上游返回0输出token（疑似风控），退还全部扣费")
	}

	// 更新提交时的消费日志为完成态
	useTime := int(task.FinishTime - task.StartTime)
	updateContent := fmt.Sprintf("统一生图，生成 %d 张图片，异步任务 %s（已完成）", len(images), task.TaskID)
	otherUpdates := map[string]interface{}{
		"task_status":           "SUCCESS",
		"generated_image_count": len(images),
	}
	for k, v := range tokenDetails {
		otherUpdates[k] = v
	}
	if isModelMapped {
		otherUpdates["is_model_mapped"] = true
		otherUpdates["upstream_model_name"] = upstreamModelName
	}
	if upstreamModelVersion != "" {
		otherUpdates["upstream_model_version"] = upstreamModelVersion
	}
	model.UpdateConsumeLogOnComplete(task.PrivateData.SubmitLogID, useTime, promptTokens, completionTokens, updateContent, otherUpdates)

	task.Properties.UpstreamModelName = upstreamModelName
	_ = task.Update()

	logger.LogInfo(ctx, fmt.Sprintf("generate_image: task %s 完成，生成 %d 张图片", task.TaskID, len(images)))
}
