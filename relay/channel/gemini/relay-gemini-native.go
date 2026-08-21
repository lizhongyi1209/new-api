package gemini

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func GeminiTextGenerationHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// 读取响应体
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	logger.LogDebug(c, "Gemini native response body: %s", responseBody)

	// 解析为 Gemini 原生响应格式
	var geminiResponse dto.GeminiChatResponse
	err = common.Unmarshal(responseBody, &geminiResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if len(geminiResponse.Candidates) == 0 && geminiResponse.PromptFeedback != nil && geminiResponse.PromptFeedback.BlockReason != nil {
		common.SetContextKey(c, constant.ContextKeyAdminRejectReason, fmt.Sprintf("gemini_block_reason=%s", *geminiResponse.PromptFeedback.BlockReason))
	}

	// 计算使用量（基于 UsageMetadata）
	usage := buildUsageFromGeminiMetadata(geminiResponse.UsageMetadata, info.GetEstimatePromptTokens())

	// Upload inline_data images to the selected storage and return fileData URLs.
	strategy := info.ChannelOtherSettings.ImageOutputStrategy
	shouldRewrite := dto.IsImageOutputStorageStrategy(strategy) ||
		(strategy == "" && strings.EqualFold(c.Query("image_format"), "url"))
	if shouldRewrite {
		uploadStrategy := strategy
		if uploadStrategy == "" {
			uploadStrategy = dto.ImageOutputStrategyR2
		}
		if err := replaceInlineDataWithStorageURLs(c, &geminiResponse, uploadStrategy); err != nil {
			if dto.IsImageOutputTemporaryStrategy(strategy) {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
			logger.LogError(c, "image storage upload failed, falling back to raw response: "+err.Error())
			service.IOCopyBytesGracefully(c, resp, responseBody)
			return &usage, nil
		}
		modifiedBody, err := common.Marshal(geminiResponse)
		if err != nil {
			if dto.IsImageOutputTemporaryStrategy(strategy) {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
			service.IOCopyBytesGracefully(c, resp, responseBody)
			return &usage, nil
		}
		service.IOCopyBytesGracefully(c, resp, modifiedBody)
		return &usage, nil
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)

	return &usage, nil
}

// replaceInlineDataWithStorageURLs uploads every image inline_data part without re-encoding it
// and replaces the part with a fileData URL.
func replaceInlineDataWithStorageURLs(c *gin.Context, resp *dto.GeminiChatResponse, strategy string) error {
	for ci := range resp.Candidates {
		for pi := range resp.Candidates[ci].Content.Parts {
			part := &resp.Candidates[ci].Content.Parts[pi]
			if part.InlineData == nil || !strings.HasPrefix(part.InlineData.MimeType, "image/") {
				continue
			}
			url, err := service.UploadBase64ImageWithOutputStrategy(part.InlineData.MimeType, part.InlineData.Data, strategy, c.Request.Host)
			if err != nil {
				return err
			}
			mimeType := part.InlineData.MimeType
			part.FileData = &dto.GeminiFileData{
				MimeType: mimeType,
				FileUri:  url,
			}
			part.InlineData = nil
		}
	}
	return nil
}

func NativeGeminiEmbeddingHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	logger.LogDebug(c, "Gemini native embedding response body: %s", responseBody)

	usage := service.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())

	if info.IsGeminiBatchEmbedding {
		var geminiResponse dto.GeminiBatchEmbeddingResponse
		err = common.Unmarshal(responseBody, &geminiResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	} else {
		var geminiResponse dto.GeminiEmbeddingResponse
		err = common.Unmarshal(responseBody, &geminiResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)

	return usage, nil
}

func GeminiTextGenerationStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	helper.SetEventStreamHeaders(c)

	return geminiStreamHandler(c, info, resp, func(data string, geminiResponse *dto.GeminiChatResponse) bool {
		if dto.IsImageOutputTemporaryStrategy(info.ChannelOtherSettings.ImageOutputStrategy) {
			if err := replaceInlineDataWithStorageURLs(c, geminiResponse, info.ChannelOtherSettings.ImageOutputStrategy); err != nil {
				logger.LogError(c, "temporary image storage upload failed: "+err.Error())
				return false
			}
			rewritten, err := common.Marshal(geminiResponse)
			if err != nil {
				logger.LogError(c, "failed to marshal temporary image stream response: "+err.Error())
				return false
			}
			data = string(rewritten)
		}
		err := helper.StringData(c, data)
		if err != nil {
			logger.LogError(c, "failed to write stream data: "+err.Error())
			return false
		}
		info.SendResponseCount++
		return true
	})
}
