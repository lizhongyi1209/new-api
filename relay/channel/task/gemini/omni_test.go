package gemini

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func omniTestContext(t *testing.T, body string) *gin.Context {
	t.Helper()
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	return context
}

func omniMultipartTestContext(t *testing.T, fields map[string]string, withImage bool) *gin.Context {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	if withImage {
		part, err := writer.CreateFormFile("input_reference", "reference.png")
		require.NoError(t, err)
		_, err = part.Write([]byte("\x89PNG\r\n\x1a\n"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return context
}

func TestOmniEndpointAndAuthentication(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://vg-api.aig-ai.com/v1/",
			UpstreamModelName: GeminiOmniFlashPreviewModel,
			ApiKey:            "test-key",
		},
	}
	adaptor.Init(info)

	requestURL, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://vg-api.aig-ai.com/v1/gemini-omni-flash-preview", requestURL)
	assert.Equal(t, "https://vg-api.aig-ai.com/v1/query/gemini-omni-flash-preview/video-123", omniQueryEndpoint(info.ChannelBaseUrl, info.UpstreamModelName, "video-123"))

	req := httptest.NewRequest(http.MethodPost, requestURL, nil)
	require.NoError(t, adaptor.BuildRequestHeader(&gin.Context{}, req, info))
	assert.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))
	assert.Empty(t, req.Header.Get("x-goog-api-key"))
}

func TestOmniFetchTaskUsesQueryEndpointAndBearerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodGet, request.Method)
		assert.Equal(t, "/v1/query/gemini-omni-flash-preview/video-123", request.URL.Path)
		assert.Equal(t, "Bearer test-key", request.Header.Get("Authorization"))
		writer.Header().Set("Content-Type", "application/json")
		_, err := writer.Write([]byte(`{"id":"video-123","status":"in_progress"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	response, err := (&TaskAdaptor{}).FetchTask(server.URL, "test-key", map[string]any{
		"task_id":        "video-123",
		"upstream_model": GeminiOmniFlashPreviewModel,
	}, "")
	require.NoError(t, err)
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	result, err := parseOmniTaskResult(body)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusInProgress, result.Status)
}

func TestOmniRequestConversion(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantTask   string
		wantAction string
		wantType   string
		wantPrompt bool
		wantAspect string
	}{
		{
			name:       "text to video",
			body:       `{"model":"gemini-omni-flash-preview","prompt":"ocean at sunrise","aspect_ratio":"9:16"}`,
			wantTask:   omniTaskTextToVideo,
			wantAction: constant.TaskActionTextGenerate,
			wantType:   "text",
			wantPrompt: true,
			wantAspect: "9:16",
		},
		{
			name:       "image to video",
			body:       `{"model":"gemini-omni-flash-preview","mode":"image_to_video","image":"data:image/png;base64,iVBORw0KGgo=","prompt":"move gently"}`,
			wantTask:   omniTaskImageToVideo,
			wantAction: constant.TaskActionGenerate,
			wantType:   "image",
			wantPrompt: true,
			wantAspect: "16:9",
		},
		{
			name:       "reference without prompt",
			body:       `{"model":"gemini-omni-flash-preview","mode":"reference_to_video","images":["data:image/png;base64,iVBORw0KGgo="]}`,
			wantTask:   omniTaskReferenceToVideo,
			wantAction: constant.TaskActionReferenceGenerate,
			wantType:   "image",
			wantAspect: "16:9",
		},
		{
			name:       "video edit",
			body:       `{"model":"gemini-omni-flash-preview","mode":"edit","video_url":"data:video/mp4;base64,AAAAIGZ0eXBpc29t","prompt":"make it cinematic"}`,
			wantTask:   omniTaskEdit,
			wantAction: constant.TaskActionVideoEdit,
			wantType:   "video",
			wantPrompt: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context := omniTestContext(t, test.body)
			info := &relaycommon.RelayInfo{
				ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: GeminiOmniFlashPreviewModel},
				TaskRelayInfo: &relaycommon.TaskRelayInfo{},
			}
			adaptor := &TaskAdaptor{}
			require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
			assert.Equal(t, test.wantAction, info.Action)

			reader, err := adaptor.BuildRequestBody(context, info)
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			var request omniRequest
			require.NoError(t, common.Unmarshal(data, &request))
			assert.Equal(t, GeminiOmniFlashPreviewModel, request.Model)
			assert.True(t, request.Background)
			assert.Equal(t, test.wantTask, request.GenerationConfig.VideoConfig.Task)
			assert.Equal(t, test.wantAspect, request.ResponseFormat.AspectRatio)
			assert.NotEmpty(t, request.Input)
			assert.Equal(t, test.wantType, request.Input[0].Content[0].Type)
			if test.wantPrompt {
				assert.Equal(t, "text", request.Input[0].Content[len(request.Input[0].Content)-1].Type)
			}
		})
	}
}

func TestOmniValidationRejectsUnsupportedDuration(t *testing.T) {
	context := omniTestContext(t, `{"model":"gemini-omni-flash-preview","prompt":"test","duration":11}`)
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: GeminiOmniFlashPreviewModel},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Equal(t, "invalid_duration", taskErr.Code)
}

func TestOmniValidationRejectsExplicitZeroAndMalformedDuration(t *testing.T) {
	tests := []string{
		`{"model":"gemini-omni-flash-preview","prompt":"test","duration":0}`,
		`{"model":"gemini-omni-flash-preview","prompt":"test","duration":"3 seconds"}`,
		`{"model":"gemini-omni-flash-preview","prompt":"test","duration":3.5}`,
	}
	for _, body := range tests {
		context := omniTestContext(t, body)
		info := &relaycommon.RelayInfo{
			ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: GeminiOmniFlashPreviewModel},
			TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		}
		taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		assert.Equal(t, "invalid_duration", taskErr.Code)
	}
}

func TestOmniValidationRejectsConflictingDurationAliases(t *testing.T) {
	context := omniTestContext(t, `{"model":"gemini-omni-flash-preview","prompt":"test","duration":3,"seconds":"4"}`)
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: GeminiOmniFlashPreviewModel},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Equal(t, "invalid_duration", taskErr.Code)
}

func TestOmniMultipartValidationAndConversion(t *testing.T) {
	context := omniMultipartTestContext(t, map[string]string{
		"model":   GeminiOmniFlashPreviewModel,
		"mode":    omniTaskImageToVideo,
		"prompt":  "move gently",
		"seconds": "3",
	}, true)
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: GeminiOmniFlashPreviewModel},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))

	reader, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	var request omniRequest
	require.NoError(t, common.Unmarshal(data, &request))
	require.Len(t, request.Input, 1)
	require.NotEmpty(t, request.Input[0].Content)
	assert.Equal(t, "image", request.Input[0].Content[0].Type)
	assert.Equal(t, "image/png", request.Input[0].Content[0].MIMEType)
}

func TestOmniMultipartRejectsMalformedDuration(t *testing.T) {
	context := omniMultipartTestContext(t, map[string]string{
		"model":    GeminiOmniFlashPreviewModel,
		"prompt":   "test",
		"duration": "3 seconds",
	}, false)
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: GeminiOmniFlashPreviewModel},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Equal(t, "invalid_duration", taskErr.Code)
}

func TestOmniValidationRejectsUnsupportedEditFormatFields(t *testing.T) {
	context := omniTestContext(t, `{"model":"gemini-omni-flash-preview","mode":"edit","video_url":"data:video/mp4;base64,AAAAIGZ0eXBpc29t","aspect_ratio":"16:9"}`)
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: GeminiOmniFlashPreviewModel},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Equal(t, "unsupported_parameter", taskErr.Code)
}

func TestOmniValidationRejectsInvalidMediaBeforeRelay(t *testing.T) {
	context := omniTestContext(t, `{"model":"gemini-omni-flash-preview","mode":"image_to_video","image":"https://example.com/image.png"}`)
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: GeminiOmniFlashPreviewModel},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Equal(t, "invalid_image", taskErr.Code)
}

func TestOmniValidationRejectsModeInputConflicts(t *testing.T) {
	tests := []string{
		`{"model":"gemini-omni-flash-preview","mode":"text_to_video","prompt":"test","image":"data:image/png;base64,iVBORw0KGgo="}`,
		`{"model":"gemini-omni-flash-preview","mode":"image_to_video","images":["data:image/png;base64,iVBORw0KGgo=","data:image/png;base64,iVBORw0KGgo="]}`,
		`{"model":"gemini-omni-flash-preview","mode":"edit","video_url":"data:video/mp4;base64,AAAAIGZ0eXBpc29t","image":"data:image/png;base64,iVBORw0KGgo="}`,
	}
	for _, body := range tests {
		context := omniTestContext(t, body)
		info := &relaycommon.RelayInfo{
			ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: GeminiOmniFlashPreviewModel},
			TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		}
		taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	}
}

func TestOmniValidationRejectsInvalidThinkingSettings(t *testing.T) {
	context := omniTestContext(t, `{"model":"gemini-omni-flash-preview","prompt":"test","metadata":{"thinking_level":"medium"}}`)
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: GeminiOmniFlashPreviewModel},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Equal(t, "invalid_thinking_level", taskErr.Code)
}

func TestParseOmniTaskResultAndModalityUsage(t *testing.T) {
	response := []byte(`{
		"id":"video-123","status":"completed","usage":{
			"total_tokens":47724,"total_input_tokens":908,
			"input_tokens_by_modality":[{"modality":"text","tokens":908}],
			"total_output_tokens":46336,
			"output_tokens_by_modality":[{"modality":"video","tokens":46336}],
			"total_thought_tokens":480
		},"steps":[{"type":"model_output","content":[{"type":"video","mime_type":"video/mp4","uri":"https://download-vod.aig-ai.com/result.mp4"}]}]
	}`)

	info, err := parseOmniTaskResult(response)
	require.NoError(t, err)
	assert.Equal(t, "video-123", info.TaskID)
	assert.Equal(t, model.TaskStatusSuccess, info.Status)
	assert.Equal(t, 908, info.PromptTokens)
	assert.Equal(t, 46816, info.CompletionTokens)
	assert.Equal(t, 480, info.ThoughtTokens)
	assert.Equal(t, 46336, info.OutputTokensByModality["video"])
	assert.Equal(t, "https://download-vod.aig-ai.com/result.mp4", info.RemoteUrl)
}

func TestParseOmniTaskResultTreatsErrorWithoutStatusAsFailure(t *testing.T) {
	info, err := parseOmniTaskResult([]byte(`{"id":"video-123","error":{"message":"content blocked"}}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, info.Status)
	assert.Equal(t, "100%", info.Progress)
	assert.Equal(t, "content blocked", info.Reason)
}

func TestParseOmniTaskResultPrioritizesErrorOverContradictoryStatus(t *testing.T) {
	info, err := parseOmniTaskResult([]byte(`{"id":"video-123","status":"completed","error":{"message":"content blocked"}}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, info.Status)
	assert.Equal(t, "content blocked", info.Reason)
}

func TestParseOmniTaskResultSaturatesOversizedTokenCounts(t *testing.T) {
	info, err := parseOmniTaskResult([]byte(`{
		"id":"video-123","status":"completed","usage":{
			"total_tokens":18446744073709551616,
			"total_input_tokens":18446744073709551616,
			"total_output_tokens":18446744073709551616,
			"total_thought_tokens":18446744073709551616,
			"output_tokens_by_modality":[{"modality":"video","tokens":18446744073709551616}]
		}
	}`))
	require.NoError(t, err)
	assert.Equal(t, int(^uint32(0)>>1), info.PromptTokens)
	assert.Equal(t, int(^uint32(0)>>1), info.CompletionTokens)
	assert.Equal(t, int(^uint32(0)>>1), info.OutputTokensByModality["video"])
}

func TestParseOmniTaskResultRejectsFractionalTokenCounts(t *testing.T) {
	_, err := parseOmniTaskResult([]byte(`{"id":"video-123","status":"completed","usage":{"total_input_tokens":1.5}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "total_input_tokens")
}

func TestOmniBillingUsesSeparateTextAndVideoOutputRates(t *testing.T) {
	task := &model.Task{
		Quota: 550000,
		Properties: model.Properties{
			OriginModelName:   GeminiOmniFlashPreviewModel,
			UpstreamModelName: GeminiOmniFlashPreviewModel,
		},
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
			ModelRatio:           0.75,
			CompletionRatio:      6,
			VideoCompletionRatio: 35.0 / 3.0,
			GroupRatio:           1,
		}},
	}
	result := &relaycommon.TaskInfo{
		PromptTokens:     908,
		CompletionTokens: 46816,
		ThoughtTokens:    480,
		OutputTokensByModality: map[string]int{
			"video": 46336,
		},
	}

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, result)
	assert.Equal(t, 408281, quota)
	require.NotNil(t, result.VideoBilling)
	assert.Equal(t, "video_modality_tokens", result.VideoBilling.BillingMode)
	assert.Equal(t, 46336, result.VideoBilling.VideoOutputTokens)
	assert.Equal(t, 480, result.VideoBilling.ThoughtTokens)
	assert.Equal(t, 8, result.VideoBilling.OutputSeconds)
	assert.InDelta(t, 0.816562, result.VideoBilling.ProviderCost, 1e-9)
}

func TestOmniBillingEstimateReservesTenSecondMaximum(t *testing.T) {
	context := omniTestContext(t, `{"model":"gemini-omni-flash-preview","prompt":"test","duration":3}`)
	context.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "test", Duration: 3})
	info := &relaycommon.RelayInfo{PriceData: types.PriceData{
		ModelRatio:           0.75,
		CompletionRatio:      6,
		VideoCompletionRatio: 35.0 / 3.0,
		GroupRatioInfo:       types.GroupRatioInfo{GroupRatio: 1},
	}}

	ratios := (&TaskAdaptor{}).estimateOmniBilling(context, info)
	require.Contains(t, ratios, "gemini_omni_token_reserve")
	assert.Greater(t, ratios["gemini_omni_token_reserve"], 1.0)
	require.NotNil(t, info.VideoBilling)
	assert.True(t, info.VideoBilling.Estimated)
	assert.Equal(t, 10, info.VideoBilling.OutputSeconds)
	assert.Equal(t, 57920, info.VideoBilling.VideoOutputTokens)
}

func TestOmniConvertToOpenAIVideoSurfacesFailReason(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_omni_fail",
		Status:     model.TaskStatusFailure,
		Progress:   "100%",
		FailReason: "safety filter triggered",
	}
	task.Properties.OriginModelName = "gemini-omni-flash-preview"

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(body, &video))
	assert.Equal(t, dto.VideoStatusFailed, video.Status)
	require.NotNil(t, video.Error)
	assert.Equal(t, "safety filter triggered", video.Error.Message)
}

func TestOmniConvertToOpenAIVideoFallsBackWhenReasonMissing(t *testing.T) {
	task := &model.Task{TaskID: "task_omni_fail2", Status: model.TaskStatusFailure}
	task.Properties.OriginModelName = "gemini-omni-flash-preview"

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(body, &video))
	require.NotNil(t, video.Error)
	assert.Equal(t, "task failed", video.Error.Message)
}

func TestOmniConvertToOpenAIVideoLeavesErrorNilOnSuccess(t *testing.T) {
	task := &model.Task{TaskID: "task_omni_ok", Status: model.TaskStatusSuccess, Progress: "100%"}
	task.Properties.OriginModelName = "gemini-omni-flash-preview"

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(body, &video))
	assert.Equal(t, dto.VideoStatusCompleted, video.Status)
	assert.Nil(t, video.Error)
}
