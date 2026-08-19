package relay

import (
	"bytes"
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
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ImageHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	relaycommon.ClearUpstreamAuditSnapshots(c)
	info.InitChannelMeta(c)

	imageReq, ok := info.Request.(*dto.ImageRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.ImageRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(imageReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to ImageRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	ignoreResponseFormat := relaycommon.IsGPTImage2(info)
	if ignoreResponseFormat {
		request.ResponseFormat = ""
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	var requestBody io.Reader

	passThroughEnabled := model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled
	if passThroughEnabled && !ignoreResponseFormat {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		info.UpstreamRequestBodySize = storage.Size()
		info.UpstreamRequestGetBody = storage.NewReader
		requestBody = common.ReaderOnly(storage)
	} else {
		convertedRequest, err := adaptor.ConvertImageRequest(c, info, *request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed)
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

		switch convertedRequest.(type) {
		case *bytes.Buffer:
			requestBody = convertedRequest.(io.Reader)
		default:
			jsonData, err := common.Marshal(convertedRequest)
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}

			// apply param override
			if len(info.ParamOverride) > 0 {
				jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
				if err != nil {
					return newAPIErrorFromParamOverride(err)
				}
			}

			logger.LogDebug(c, "image request body: %s", jsonData)
			body, size, getBody, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}
			defer closer.Close()
			jsonData = nil
			info.UpstreamRequestBodySize = size
			info.UpstreamRequestGetBody = getBody
			requestBody = body
		}
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	var httpResp *http.Response
	var responseCapture *relaycommon.UpstreamResponseCapture
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.Body != nil {
			responseCapture = relaycommon.NewUpstreamResponseCapture(httpResp.Body)
			httpResp.Body = responseCapture
		}
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			if httpResp.StatusCode == http.StatusCreated && info.ApiType == constant.APITypeReplicate {
				// replicate channel returns 201 Created when using Prefer: wait, treat it as success.
				httpResp.StatusCode = http.StatusOK
			} else {
				if relaycommon.IsGPTImage2(info) {
					statusCode := httpResp.StatusCode
					contentType := httpResp.Header.Get("Content-Type")
					body, readErr := io.ReadAll(httpResp.Body)
					service.CloseResponseBodyGracefully(httpResp)
					if readErr != nil {
						return types.NewOpenAIError(readErr, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
					}
					relaycommon.SetUpstreamResponseSnapshot(c, statusCode, contentType, body)
					httpResp.Body = io.NopCloser(bytes.NewReader(body))
					newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
					newAPIError.SetRawUpstreamResponse(statusCode, contentType, body)
				} else {
					newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
					responseCapture.StoreSnapshot(c, httpResp.StatusCode, httpResp.Header.Get("Content-Type"))
				}
				// reset status code 重置状态码
				service.ResetStatusCode(newAPIError, statusCodeMappingStr)
				return newAPIError
			}
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		if responseCapture != nil && httpResp != nil {
			responseCapture.StoreSnapshot(c, httpResp.StatusCode, httpResp.Header.Get("Content-Type"))
		}
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	imageN := uint(1)
	if request.N != nil {
		imageN = *request.N
	}

	if usage.(*dto.Usage).TotalTokens == 0 {
		usage.(*dto.Usage).TotalTokens = 1
	}
	if usage.(*dto.Usage).PromptTokens == 0 {
		usage.(*dto.Usage).PromptTokens = 1
	}

	quality := request.Quality
	if quality == "" {
		quality = "standard"
	}

	var logContent []string
	logImageN := imageN
	if actualN, ok := info.PriceData.OtherRatios["n"]; ok && actualN > 0 {
		logImageN = uint(actualN)
	}

	if len(request.Size) > 0 {
		logContent = append(logContent, fmt.Sprintf("大小 %s", request.Size))
	}
	if len(quality) > 0 {
		logContent = append(logContent, fmt.Sprintf("品质 %s", quality))
	}
	if logImageN > 0 {
		logContent = append(logContent, fmt.Sprintf("生成数量 %d", logImageN))
	}

	usageDto := usage.(*dto.Usage)
	service.PostTextConsumeQuota(c, info, usageDto, logContent)
	// Note: RefundIfZeroCompletionTokens removed for image generation.
	// Image generation providers (DALL-E, Gemini Imagen, etc.) return accurate usage.
	// For image tasks, completion_tokens is often 0 by design (output is an image, not text).
	return nil
}
