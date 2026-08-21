package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func updateOpenAIImageCount(info *relaycommon.RelayInfo, count int64) {
	if info == nil || !info.PriceData.UsePrice || count <= 0 || count > int64(dto.MaxImageN) {
		return
	}
	info.PriceData.AddOtherRatio("n", float64(count))
}

func preserveGPTImage2UpstreamError(info *relaycommon.RelayInfo, resp *http.Response, body []byte, relayErr *types.NewAPIError) *types.NewAPIError {
	if relayErr != nil && resp != nil && relaycommon.IsGPTImage2(info) {
		relayErr.SetRawUpstreamResponse(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	return relayErr
}

// OpenaiImageHandler handles non-streaming OpenAI image responses
// (generations/edits), returning the parsed usage for billing.
func OpenaiImageHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var usageResp dto.SimpleResponse
	err = common.Unmarshal(responseBody, &usageResp)
	if err != nil {
		relayErr := types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		return nil, preserveGPTImage2UpstreamError(info, resp, responseBody, relayErr)
	}

	if oaiError := usageResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		relayErr := types.WithOpenAIError(*oaiError, resp.StatusCode)
		return nil, preserveGPTImage2UpstreamError(info, resp, responseBody, relayErr)
	}

	updateOpenAIImageCount(info, gjson.GetBytes(responseBody, "data.#").Int())

	// When a storage strategy is selected (or ?image_format=url requests the
	// historical R2 behavior), replace large base64/transient upstream values
	// with a storage URL. Durable storage failures keep the historical raw-body
	// fallback; local_temp fails closed so it never leaks a non-local image.
	strategy := info.ChannelOtherSettings.ImageOutputStrategy
	shouldRewrite := dto.IsImageOutputStorageStrategy(strategy) ||
		(strategy == "" && strings.EqualFold(c.Query("image_format"), "url"))
	if shouldRewrite {
		uploadStrategy := strategy
		if uploadStrategy == "" {
			uploadStrategy = dto.ImageOutputStrategyR2
		}
		if rewritten, err := uploadOpenAIImagesToStorage(c, responseBody, uploadStrategy); err != nil {
			if strategy == dto.ImageOutputStrategyLocalTemp {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
			logger.LogError(c, "openai image storage upload failed, falling back to raw response: "+err.Error())
			service.IOCopyBytesGracefully(c, resp, responseBody)
		} else {
			service.IOCopyBytesGracefully(c, resp, rewritten)
		}
		normalizeOpenAIUsage(&usageResp.Usage)
		applyUsagePostProcessing(info, &usageResp.Usage, responseBody)
		return &usageResp.Usage, nil
	}

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	normalizeOpenAIUsage(&usageResp.Usage)
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)
	return &usageResp.Usage, nil
}

// normalizeOpenAIUsage maps the OpenAI Images usage shape (input_tokens /
// output_tokens / input_tokens_details) onto the canonical prompt/completion
// fields. It is used only on the OpenAI image relay paths (generations/edits,
// streaming and non-streaming): the image API never returns prompt_tokens /
// completion_tokens, so the overwrite (=) semantics here are equivalent to the
// previous additive (+=) behavior while avoiding any future double-counting if
// both field sets are ever populated. Do not reuse this on chat/embedding paths
// without revisiting the overwrite semantics.
func normalizeOpenAIUsage(usage *dto.Usage) {
	if usage == nil {
		return
	}
	if usage.InputTokens != 0 {
		usage.PromptTokens = usage.InputTokens
	}
	if usage.OutputTokens != 0 {
		usage.CompletionTokens = usage.OutputTokens
	}
	if usage.InputTokensDetails != nil {
		usage.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
		usage.PromptTokensDetails.CachedCreationTokens = usage.InputTokensDetails.CachedCreationTokens
		usage.PromptTokensDetails.CacheWriteTokens = usage.InputTokensDetails.CacheWriteTokens
		usage.PromptTokensDetails.ImageTokens = usage.InputTokensDetails.ImageTokens
		usage.PromptTokensDetails.TextTokens = usage.InputTokensDetails.TextTokens
		usage.PromptTokensDetails.AudioTokens = usage.InputTokensDetails.AudioTokens
	}
	if usage.OutputTokensDetails != nil {
		usage.CompletionTokenDetails.ImageTokens = usage.OutputTokensDetails.ImageTokens
		usage.CompletionTokenDetails.TextTokens = usage.OutputTokensDetails.TextTokens
		usage.CompletionTokenDetails.AudioTokens = usage.OutputTokensDetails.AudioTokens
	}
	// 图像端点的输出 token 全部是图像（无输出明细时兜底），
	// 标记为图像模态供 tiered_expr 的 img_o 变量取用；表达式未引用 img_o 时不影响任何计费。
	if usage.CompletionTokenDetails.ImageTokens == 0 && usage.CompletionTokens > 0 {
		usage.CompletionTokenDetails.ImageTokens = usage.CompletionTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
}

func OpenaiImageStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid image stream response")
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return OpenaiImageHandler(c, info, resp)
	}
	if !strings.Contains(contentType, "text/event-stream") {
		return openaiImageJSONAsStreamHandler(c, info, resp)
	}
	// Reuse the shared streaming engine (helper.StreamScannerHandler) so the
	// image streaming path gets the same ping keepalive, streaming-timeout
	// watchdog, client-disconnect detection, panic recovery and goroutine
	// cleanup as every other relay stream. The scanner delivers only the
	// "data:" payload, so the SSE "event:" line is rebuilt from the JSON "type"
	// field (real OpenAI image events keep event == type).
	usage := &dto.Usage{}
	var lastStreamData []byte
	var completedImages int64

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		raw := common.StringToByteSlice(data)
		lastStreamData = raw
		if isOpenAIImageStreamErrorEvent(raw) {
			// Record the error as a soft error; the scanner drives the final
			// EndReason. HasErrors() flags the failure for logging/handling.
			sr.Error(fmt.Errorf("%s", extractOpenAIImageStreamErrorMessage(raw)))
		}
		var chunk struct {
			Type  string    `json:"type"`
			Usage dto.Usage `json:"usage"`
		}
		if err := common.Unmarshal(raw, &chunk); err == nil {
			normalizeOpenAIUsage(&chunk.Usage)
			if service.ValidUsage(&chunk.Usage) {
				usage = &chunk.Usage
			}
			if chunk.Type == "image_generation.completed" || chunk.Type == "image_edit.completed" {
				completedImages++
				if info.ChannelOtherSettings.ImageOutputStrategy == dto.ImageOutputStrategyLocalTemp {
					rewritten, err := uploadOpenAIStreamImageToStorage(c, raw, dto.ImageOutputStrategyLocalTemp)
					if err != nil {
						sr.Stop(fmt.Errorf("temporary image storage upload: %w", err))
						return
					}
					raw = rewritten
				}
			}
			if info.ChannelOtherSettings.ImageOutputStrategy == dto.ImageOutputStrategyLocalTemp &&
				(chunk.Type == "image_generation.partial_image" || chunk.Type == "image_edit.partial_image") {
				return
			}
		}
		if err := writeOpenaiImageStreamChunk(c, raw); err != nil {
			sr.Stop(err)
		}
	})

	// StreamScannerHandler consumes the upstream [DONE]; re-emit it so the
	// client still receives a terminal data: [DONE].
	if info.StreamStatus != nil && info.StreamStatus.EndReason == relaycommon.StreamEndReasonDone {
		helper.Done(c)
	}

	applyUsagePostProcessing(info, usage, lastStreamData)
	// Only trust completedImages when upstream finished the stream (done/eof).
	// On client-side aborts (client_gone, or handler_stop from a failed client
	// write) the counter undercounts what upstream actually generated and
	// charged, so keep the requested n — otherwise a client could pay for one
	// image by disconnecting right after the first completed event. The abort
	// guard only blocks lowering the charge: if completed events already
	// exceed the recorded n, bill the higher actual count regardless.
	if info.StreamStatus != nil {
		upstreamFinished := info.StreamStatus.EndReason == relaycommon.StreamEndReasonDone ||
			info.StreamStatus.EndReason == relaycommon.StreamEndReasonEOF
		requestedN := 1.0
		if n, ok := info.PriceData.OtherRatios["n"]; ok {
			requestedN = n
		}
		if upstreamFinished || float64(completedImages) > requestedN {
			updateOpenAIImageCount(info, completedImages)
		}
	}
	return usage, nil
}

// writeOpenaiImageStreamChunk rebuilds the SSE frame for an image stream chunk:
// it emits an "event:" line derived from the JSON "type" field (when present)
// followed by the verbatim "data:" payload, mirroring helper.ResponseChunkData.
func writeOpenaiImageStreamChunk(c *gin.Context, data []byte) error {
	var payload struct {
		Type string `json:"type"`
	}
	_ = common.Unmarshal(data, &payload)
	if eventName := strings.TrimSpace(payload.Type); eventName != "" {
		return helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: eventName}, string(data))
	}
	return helper.StringData(c, string(data))
}

// isOpenAIImageStreamErrorEvent detects upstream error chunks by JSON content
// only ("type" of error/upstream_error, or a non-empty "error" field). The SSE
// "event:" line is not available here: StreamScannerHandler delivers only the
// "data:" payload. A payload carrying just a "message" key is deliberately NOT
// treated as an error to avoid false positives.
func isOpenAIImageStreamErrorEvent(data []byte) bool {
	if !json.Valid(data) {
		return false
	}
	var payload struct {
		Type  string          `json:"type"`
		Error json.RawMessage `json:"error"`
	}
	if err := common.Unmarshal(data, &payload); err != nil {
		return false
	}
	payloadType := strings.ToLower(strings.TrimSpace(payload.Type))
	return payloadType == "error" || payloadType == "upstream_error" || len(payload.Error) > 0
}

func extractOpenAIImageStreamErrorMessage(data []byte) string {
	if len(data) == 0 || !json.Valid(data) {
		return "upstream image stream returned error event"
	}
	var payload struct {
		Message string          `json:"message"`
		Error   json.RawMessage `json:"error"`
	}
	if err := common.Unmarshal(data, &payload); err != nil {
		return "upstream image stream returned error event"
	}
	if msg := strings.TrimSpace(payload.Message); msg != "" {
		return msg
	}
	if len(payload.Error) > 0 {
		var nested struct {
			Message string `json:"message"`
		}
		if err := common.Unmarshal(payload.Error, &nested); err == nil {
			if msg := strings.TrimSpace(nested.Message); msg != "" {
				return msg
			}
		}
		if msg := strings.TrimSpace(common.JsonRawMessageToString(payload.Error)); msg != "" {
			return msg
		}
	}
	return "upstream image stream returned error event"
}

func openaiImageJSONAsStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if info.ChannelOtherSettings.ImageOutputStrategy == dto.ImageOutputStrategyLocalTemp {
		responseBody, err = uploadOpenAIImagesToStorage(c, responseBody, dto.ImageOutputStrategyLocalTemp)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	}

	// Only decode usage/error. Do not unmarshal data[] into dto.ImageResponse:
	// b64_json values are large and would be copied into Go strings and then
	// re-marshaled for every SSE event.
	var usageResp dto.SimpleResponse
	if err := common.Unmarshal(responseBody, &usageResp); err != nil {
		relayErr := types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		return nil, preserveGPTImage2UpstreamError(info, resp, responseBody, relayErr)
	}
	if oaiError := usageResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		relayErr := types.WithOpenAIError(*oaiError, resp.StatusCode)
		return nil, preserveGPTImage2UpstreamError(info, resp, responseBody, relayErr)
	}
	normalizeOpenAIUsage(&usageResp.Usage)
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)

	imageCount := gjson.GetBytes(responseBody, "data.#").Int()
	updateOpenAIImageCount(info, imageCount)

	helper.SetEventStreamHeaders(c)
	c.Status(http.StatusOK)

	created := gjson.GetBytes(responseBody, "created").Int()
	if created == 0 {
		created = time.Now().Unix()
	}
	if info != nil {
		info.SetFirstResponseTime()
	}

	validUsage := service.ValidUsage(&usageResp.Usage)
	var usageJSON []byte
	if validUsage {
		usageJSON, err = common.Marshal(usageResp.Usage)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	}

	for i := int64(0); i < imageCount; i++ {
		image := gjson.GetBytes(responseBody, "data."+strconv.FormatInt(i, 10))
		payload := []byte(`{"type":"image_generation.completed"}`)
		payload, err = sjson.SetBytes(payload, "created_at", created)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if validUsage {
			payload, err = sjson.SetRawBytes(payload, "usage", usageJSON)
			if err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		// b64_json goes last: every sjson.Set* reallocates the payload, so
		// inserting the large blob after small fields avoids repeated copies.
		for _, field := range []string{"url", "revised_prompt", "b64_json"} {
			value := image.Get(field)
			if value.Type != gjson.String || value.Raw == `""` {
				continue
			}
			raw := []byte(value.Raw)
			if value.Index > 0 {
				raw = responseBody[value.Index : value.Index+len(value.Raw)]
			}
			payload, err = sjson.SetRawBytes(payload, field, raw)
			if err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		if writeErr := helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: "image_generation.completed"}, string(payload)); writeErr != nil {
			if info != nil && info.StreamStatus != nil {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, writeErr)
			}
			return &usageResp.Usage, nil
		}
	}
	if err := writeOpenaiImageStreamDone(c); err != nil {
		if info != nil && info.StreamStatus != nil {
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, err)
		}
		return &usageResp.Usage, nil
	}
	if info != nil {
		info.ReceivedResponseCount += int(imageCount)
		if info.StreamStatus == nil {
			info.StreamStatus = relaycommon.NewStreamStatus()
		}
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	}
	return &usageResp.Usage, nil
}

func writeOpenaiImageStreamDone(c *gin.Context) error {
	return helper.StringData(c, "[DONE]")
}

// uploadOpenAIImagesToStorage rewrites an OpenAI image response body so every image
// in data[] is served from the configured storage URL.
//   - data[i].url      : a transient upstream url; downloaded then re-uploaded
//     so the upstream link does not expire out from under the browser.
//
// On success the rewritten body carries data[i].url with
// b64_json cleared; all other top-level fields (created/usage/size/quality/...)
// are preserved verbatim by editing only the "data" field of the raw object.
// Images are uploaded without re-encoding. Returns an error only when the body
// cannot be parsed or an upload fails; the caller decides whether fallback is
// allowed for the selected strategy.
func uploadOpenAIImagesToStorage(c *gin.Context, body []byte, strategy string) ([]byte, error) {
	var root map[string]json.RawMessage
	if err := common.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("parse image response: %w", err)
	}
	rawData, ok := root["data"]
	if !ok {
		return nil, fmt.Errorf("image response has no data field")
	}
	var items []dto.ImageData
	if err := common.Unmarshal(rawData, &items); err != nil {
		return nil, fmt.Errorf("parse image data: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("image response data is empty")
	}

	for i := range items {
		var mimeType, b64 string
		switch {
		case items[i].B64Json != "":
			// output_format defaults to png for gpt-image; the "origin" upload
			// path re-detects the real format from the decoded bytes anyway, so
			// this mime is only a content-type hint/fallback.
			mimeType = "image/png"
			b64 = items[i].B64Json
		case items[i].Url != "":
			mt, data, err := service.GetImageFromUrl(items[i].Url)
			if err != nil {
				return nil, fmt.Errorf("download upstream image url: %w", err)
			}
			mimeType = mt
			b64 = data
		default:
			continue // nothing to upload for this entry
		}

		url, err := service.UploadBase64ImageWithOutputStrategy(mimeType, b64, strategy, c.Request.Host)
		if err != nil {
			return nil, fmt.Errorf("image storage upload: %w", err)
		}
		items[i].Url = url
		items[i].B64Json = ""
	}

	newData, err := common.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("marshal rewritten data: %w", err)
	}
	root["data"] = newData
	out, err := common.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("marshal rewritten response: %w", err)
	}
	return out, nil
}

func uploadOpenAIStreamImageToStorage(c *gin.Context, body []byte, strategy string) ([]byte, error) {
	b64 := gjson.GetBytes(body, "b64_json").String()
	mimeType := "image/png"
	if b64 == "" {
		upstreamURL := gjson.GetBytes(body, "url").String()
		if upstreamURL == "" {
			return nil, fmt.Errorf("completed image event has no b64_json or url")
		}
		var err error
		mimeType, b64, err = service.GetImageFromUrl(upstreamURL)
		if err != nil {
			return nil, fmt.Errorf("download completed image: %w", err)
		}
	}

	url, err := service.UploadBase64ImageWithOutputStrategy(mimeType, b64, strategy, c.Request.Host)
	if err != nil {
		return nil, err
	}
	rewritten, err := sjson.SetBytes(body, "url", url)
	if err != nil {
		return nil, err
	}
	rewritten, err = sjson.DeleteBytes(rewritten, "b64_json")
	if err != nil {
		return nil, err
	}
	return rewritten, nil
}
