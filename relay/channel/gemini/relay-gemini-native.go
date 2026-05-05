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

	// When the client requests URL format, upload inline_data images to R2 and replace them with fileData URLs.
	if strings.EqualFold(c.Query("image_format"), "url") {
		compression := c.Query("image_compression")
		if err := replaceInlineDataWithR2URLs(c, &geminiResponse, compression); err != nil {
			logger.LogError(c, "r2 upload failed, falling back to raw response: "+err.Error())
			service.IOCopyBytesGracefully(c, resp, responseBody)
			return &usage, nil
		}
		modifiedBody, err := common.Marshal(geminiResponse)
		if err != nil {
			service.IOCopyBytesGracefully(c, resp, responseBody)
			return &usage, nil
		}
		service.IOCopyBytesGracefully(c, resp, modifiedBody)
		return &usage, nil
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)

	return &usage, nil
}

// replaceInlineDataWithR2URLs uploads every image inline_data part to R2 and replaces it with a fileData URL.
// When compression is "webp", images are converted to WebP before upload.
func replaceInlineDataWithR2URLs(c *gin.Context, resp *dto.GeminiChatResponse, compression string) error {
	for ci := range resp.Candidates {
		for pi := range resp.Candidates[ci].Content.Parts {
			part := &resp.Candidates[ci].Content.Parts[pi]
			if part.InlineData == nil || !strings.HasPrefix(part.InlineData.MimeType, "image/") {
				continue
			}
			url, err := service.UploadBase64ImageToR2Compressed(part.InlineData.MimeType, part.InlineData.Data, compression)
			if err != nil {
				return err
			}
			mimeType := part.InlineData.MimeType
			if compression == service.ImageCompressionWebP {
				mimeType = "image/webp"
			}
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
		err := helper.StringData(c, data)
		if err != nil {
			logger.LogError(c, "failed to write stream data: "+err.Error())
			return false
		}
		info.SendResponseCount++
		return true
	})
}
