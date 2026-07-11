package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
)

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
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if oaiError := usageResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	// When the client requests URL format (?image_format=url), upload every
	// returned image to R2 and rewrite the response so data[i] carries a short
	// url instead of a multi-MB b64_json blob. This mirrors the Gemini native
	// path (replaceInlineDataWithR2URLs): some upstreams (e.g. gpt-image) return
	// b64_json, others return a transient upstream url — both are normalized to a
	// stable R2 url here so the browser only ever holds a tiny string. On any
	// failure we fall back to the original body so a generation never fails just
	// because the rewrite/upload did.
	if strings.EqualFold(c.Query("image_format"), "url") {
		if rewritten, err := uploadOpenAIImagesToR2(c, responseBody); err != nil {
			logger.LogError(c, "openai image r2 upload failed, falling back to raw response: "+err.Error())
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
		return OpenaiImageJSONAsStreamHandler(c, info, resp)
	}
	// Reuse the shared streaming engine (helper.StreamScannerHandler) so the
	// image streaming path gets the same ping keepalive, streaming-timeout
	// watchdog, client-disconnect detection, panic recovery and goroutine
	// cleanup as every other relay stream. The scanner delivers only the
	// "data:" payload, so the SSE "event:" line is rebuilt from the JSON "type"
	// field (real OpenAI image events keep event == type).
	usage := &dto.Usage{}
	var lastStreamData []byte

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		raw := common.StringToByteSlice(data)
		lastStreamData = raw
		if isOpenAIImageStreamErrorEvent(raw) {
			// Record the error as a soft error; the scanner drives the final
			// EndReason. HasErrors() flags the failure for logging/handling.
			sr.Error(fmt.Errorf("%s", extractOpenAIImageStreamErrorMessage(raw)))
		}
		var usageResp dto.SimpleResponse
		if err := common.Unmarshal(raw, &usageResp); err == nil {
			normalizeOpenAIUsage(&usageResp.Usage)
			if service.ValidUsage(&usageResp.Usage) {
				usage = &usageResp.Usage
			}
		}
		writeOpenaiImageStreamChunk(c, raw)
	})

	// StreamScannerHandler consumes the upstream [DONE]; re-emit it so the
	// client still receives a terminal data: [DONE].
	if info != nil && info.StreamStatus != nil && info.StreamStatus.EndReason == relaycommon.StreamEndReasonDone {
		helper.Done(c)
	}

	applyUsagePostProcessing(info, usage, lastStreamData)
	return usage, nil
}

// writeOpenaiImageStreamChunk rebuilds the SSE frame for an image stream chunk:
// it emits an "event:" line derived from the JSON "type" field (when present)
// followed by the verbatim "data:" payload, mirroring helper.ResponseChunkData.
func writeOpenaiImageStreamChunk(c *gin.Context, data []byte) {
	var payload struct {
		Type string `json:"type"`
	}
	_ = common.Unmarshal(data, &payload)
	if eventName := strings.TrimSpace(payload.Type); eventName != "" {
		c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("event: %s\n", eventName)})
	}
	c.Render(-1, common.CustomEvent{Data: "data: " + string(data)})
	_ = helper.FlushWriter(c)
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

func OpenaiImageJSONAsStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var imageResp dto.ImageResponse
	if err := common.Unmarshal(responseBody, &imageResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var usageResp dto.SimpleResponse
	_ = common.Unmarshal(responseBody, &usageResp)
	if oaiError := usageResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}
	normalizeOpenAIUsage(&usageResp.Usage)
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)

	helper.SetEventStreamHeaders(c)
	c.Status(http.StatusOK)

	created := imageResp.Created
	if created == 0 {
		created = time.Now().Unix()
	}
	if info != nil {
		info.SetFirstResponseTime()
	}
	for _, image := range imageResp.Data {
		payload := map[string]any{
			"type":       "image_generation.completed",
			"created_at": created,
		}
		if image.Url != "" {
			payload["url"] = image.Url
		}
		if image.B64Json != "" {
			payload["b64_json"] = image.B64Json
		}
		if image.RevisedPrompt != "" {
			payload["revised_prompt"] = image.RevisedPrompt
		}
		if service.ValidUsage(&usageResp.Usage) {
			payload["usage"] = usageResp.Usage
		}
		if err := writeOpenaiImageStreamPayload(c, "image_generation.completed", payload); err != nil {
			if info != nil && info.StreamStatus != nil {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, err)
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
		info.ReceivedResponseCount += len(imageResp.Data)
		if info.StreamStatus == nil {
			info.StreamStatus = relaycommon.NewStreamStatus()
		}
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	}
	return &usageResp.Usage, nil
}

func writeOpenaiImageStreamPayload(c *gin.Context, eventName string, payload any) error {
	data, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	if eventName != "" {
		if _, err := fmt.Fprintf(c.Writer, "event: %s\n", eventName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
		return err
	}
	return helper.FlushWriter(c)
}

func writeOpenaiImageStreamDone(c *gin.Context) error {
	if _, err := fmt.Fprint(c.Writer, "data: [DONE]\n\n"); err != nil {
		return err
	}
	return helper.FlushWriter(c)
}

// uploadOpenAIImagesToR2 rewrites an OpenAI image response body so every image
// in data[] is served from R2 as a stable url. Two upstream shapes are handled:
//   - data[i].b64_json : decoded and uploaded to R2 directly.
//   - data[i].url      : a transient upstream url; downloaded then re-uploaded
//     to R2 so the link does not expire out from under the browser.
//
// On success the rewritten body carries data[i].url (pointing at R2) with
// b64_json cleared; all other top-level fields (created/usage/size/quality/...)
// are preserved verbatim by editing only the "data" field of the raw object.
// Compression follows the same image_compression query as the Gemini path
// (defaults to "origin"). Returns an error only when the body cannot be parsed
// or an upload fails, in which case the caller falls back to the raw response.
func uploadOpenAIImagesToR2(c *gin.Context, body []byte) ([]byte, error) {
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

	compression := c.Query("image_compression")
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

		url, err := service.UploadBase64ImageToR2Compressed(mimeType, b64, compression)
		if err != nil {
			return nil, fmt.Errorf("r2 upload: %w", err)
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
