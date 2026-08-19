package xai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newVideoContext(body string) (*gin.Context, *relaycommon.RelayInfo) {
	return newVideoContextForPath("/grok/v1/videos/generations", body)
}

func newVideoContextForPath(path string, body string) (*gin.Context, *relaycommon.RelayInfo) {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	return context, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
}

func TestValidateVideoGenerationRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		body   string
		code   string
		action string
	}{
		{name: "text to video", body: `{"model":"grok-imagine-video","prompt":"orbit","duration":15,"aspect_ratio":"16:9","resolution":"720p"}`, action: constant.TaskActionTextGenerate},
		{name: "1.5 text to video at 1080p", body: `{"model":"grok-imagine-video-1.5","prompt":"orbit","resolution":"1080p"}`, action: constant.TaskActionTextGenerate},
		{name: "image to video", body: `{"model":"grok-imagine-video","prompt":"move","image":{"url":"https://example.com/a.png"}}`, action: constant.TaskActionGenerate},
		{name: "image to video optional prompt", body: `{"model":"grok-imagine-video","image":{"image_url":"https://example.com/a.png"},"seconds":"8"}`, action: constant.TaskActionGenerate},
		{name: "reference to video", body: `{"model":"grok-imagine-video","prompt":"scene","reference_images":[{"file_id":"file-1"}]}`, action: constant.TaskActionReferenceGenerate},
		{name: "1.5 reference image and preset voice", body: `{"model":"grok-imagine-video-1.5","prompt":"scene","reference_images":[{"file_id":"file-1"}],"reference_audios":[{"voice_id":"eve"}],"duration":15,"resolution":"720p"}`, action: constant.TaskActionReferenceGenerate},
		{name: "1.5 audio only reference", body: `{"model":"grok-imagine-video-1.5","prompt":"speak","reference_audios":[{"voice_id":"ara"}]}`, action: constant.TaskActionReferenceGenerate},
		{name: "duration overflow", body: `{"model":"grok-imagine-video","prompt":"orbit","duration":16}`, code: "invalid_duration"},
		{name: "mutually exclusive inputs", body: `{"model":"grok-imagine-video","prompt":"orbit","image":{"url":"https://example.com/a.png"},"reference_images":[{"url":"https://example.com/b.png"}]}`, code: "invalid_request"},
		{name: "legacy model cannot use 1080p", body: `{"model":"grok-imagine-video","prompt":"orbit","resolution":"1080p"}`, code: "invalid_resolution"},
		{name: "1080p image to video", body: `{"model":"grok-imagine-video-1.5","prompt":"orbit","image":{"url":"https://example.com/a.png"},"resolution":"1080p"}`, action: constant.TaskActionGenerate},
		{name: "reference mode cannot use 1080p", body: `{"model":"grok-imagine-video-1.5","prompt":"orbit","reference_images":[{"url":"https://example.com/a.png"}],"resolution":"1080p"}`, code: "invalid_resolution"},
		{name: "reference image limit", body: `{"model":"grok-imagine-video","prompt":"orbit","reference_images":[{"file_id":"1"},{"file_id":"2"},{"file_id":"3"},{"file_id":"4"},{"file_id":"5"},{"file_id":"6"},{"file_id":"7"},{"file_id":"8"}]}`, code: "invalid_reference_images"},
		{name: "reference duration limit", body: `{"model":"grok-imagine-video","prompt":"orbit","reference_images":[{"file_id":"1"}],"duration":11}`, code: "invalid_duration"},
		{name: "audio reference", body: `{"model":"grok-imagine-video","prompt":"speak","reference_audios":[{"url":"data:audio/wav;base64,AAAA"}]}`, action: constant.TaskActionReferenceGenerate},
		{name: "audio reference requires prompt", body: `{"model":"grok-imagine-video","reference_audios":[{"url":"data:audio/wav;base64,AAAA"}]}`, code: "invalid_request"},
		{name: "audio reference requires one source", body: `{"model":"grok-imagine-video-1.5","prompt":"speak","reference_audios":[{}]}`, code: "invalid_reference_audio"},
		{name: "audio URL and voice are exclusive", body: `{"model":"grok-imagine-video-1.5","prompt":"speak","reference_audios":[{"url":"data:audio/wav;base64,AAAA","voice_id":"eve"}]}`, code: "invalid_reference_audio"},
		{name: "image URL aliases are exclusive", body: `{"model":"grok-imagine-video-1.5","image":{"url":"https://example.com/a.png","image_url":"https://example.com/b.png"}}`, code: "invalid_image"},
		{name: "output requires upload URL", body: `{"model":"grok-imagine-video","prompt":"orbit","output":{}}`, code: "invalid_output"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, info := newVideoContext(test.body)
			taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
			if test.code != "" {
				require.NotNil(t, taskErr)
				assert.Equal(t, test.code, taskErr.Code)
				return
			}
			require.Nil(t, taskErr)
			assert.Equal(t, test.action, info.Action)
		})
	}
}

func TestValidateVideoEditAndExtensionRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		path   string
		body   string
		code   string
		action string
	}{
		{name: "edit", path: "/grok/v1/videos/edits", body: `{"model":"grok-imagine-video","prompt":"add snow","video":{"url":"https://example.com/source.mp4"}}`, action: constant.TaskActionVideoEdit},
		{name: "edit rejects duration", path: "/grok/v1/videos/edits", body: `{"model":"grok-imagine-video","prompt":"add snow","video":{"url":"https://example.com/source.mp4"},"duration":5}`, code: "invalid_request"},
		{name: "extension", path: "/grok/v1/videos/extensions", body: `{"model":"grok-imagine-video","prompt":"zoom out","video":{"file_id":"file-video"},"duration":10}`, action: constant.TaskActionVideoExtend},
		{name: "extension requires video", path: "/grok/v1/videos/extensions", body: `{"model":"grok-imagine-video","prompt":"zoom out"}`, code: "invalid_video"},
		{name: "extension duration minimum", path: "/grok/v1/videos/extensions", body: `{"model":"grok-imagine-video","prompt":"zoom out","video":{"url":"https://example.com/source.mp4"},"duration":1}`, code: "invalid_duration"},
		{name: "storage expiry", path: "/grok/v1/videos/extensions", body: `{"model":"grok-imagine-video","prompt":"zoom out","video":{"url":"https://example.com/source.mp4"},"storage_options":{"filename":"result.mp4","expires_after":3599}}`, code: "invalid_storage_options"},
		{name: "public URL expiry exceeds file", path: "/grok/v1/videos/extensions", body: `{"model":"grok-imagine-video","prompt":"zoom out","video":{"url":"https://example.com/source.mp4"},"storage_options":{"filename":"result.mp4","expires_after":7200,"public_url":{"expires_after":10800}}}`, code: "invalid_storage_options"},
		{name: "persistent output", path: "/grok/v1/videos/extensions", body: `{"model":"grok-imagine-video","prompt":"zoom out","video":{"url":"https://example.com/source.mp4"},"storage_options":{"filename":"result.mp4","public_url":true}}`, action: constant.TaskActionVideoExtend},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, info := newVideoContextForPath(test.path, test.body)
			taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)
			if test.code != "" {
				require.NotNil(t, taskErr)
				assert.Equal(t, test.code, taskErr.Code)
				return
			}
			require.Nil(t, taskErr)
			assert.Equal(t, test.action, info.Action)
		})
	}
}

func TestBuildRequestURLUsesOfficialEndpoint(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://api.x.ai"}
	tests := []struct {
		action string
		url    string
	}{
		{action: constant.TaskActionTextGenerate, url: "https://api.x.ai/v1/videos/generations"},
		{action: constant.TaskActionVideoEdit, url: "https://api.x.ai/v1/videos/edits"},
		{action: constant.TaskActionVideoExtend, url: "https://api.x.ai/v1/videos/extensions"},
	}
	for _, test := range tests {
		url, err := adaptor.BuildRequestURL(&relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: test.action}})
		require.NoError(t, err)
		assert.Equal(t, test.url, url)
	}
}

func TestEstimateBillingUsesOfficialDurationAndResolutionDefaults(t *testing.T) {
	tests := []struct {
		body string
		want map[string]float64
	}{
		{body: `{"model":"grok-imagine-video","prompt":"orbit"}`, want: map[string]float64{"seconds": 8, "resolution": 1}},
		{body: `{"model":"grok-imagine-video","prompt":"orbit","duration":5,"resolution":"720p"}`, want: map[string]float64{"seconds": 5, "resolution": 1.4}},
		{body: `{"model":"grok-imagine-video-1.5","image":{"url":"https://example.com/a.png"},"duration":5,"resolution":"720p"}`, want: map[string]float64{"seconds": 5, "resolution": 1.75, "image_input": 0.71 / 0.7}},
	}
	for _, test := range tests {
		context, info := newVideoContext(test.body)
		adaptor := &TaskAdaptor{}
		require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
		assert.Equal(t, test.want, adaptor.EstimateBilling(context, info))
	}
}

func TestRequestAuditCapturesRequestedAndEffectiveResolution(t *testing.T) {
	tests := []struct {
		name            string
		body            string
		requested       string
		effective       string
		defaulted       bool
		imageCount      int
		referenceAudios int
	}{
		{
			name:      "official 480p default",
			body:      `{"model":"grok-imagine-video","prompt":"orbit"}`,
			effective: "480p",
			defaulted: true,
		},
		{
			name:       "explicit 720p image input",
			body:       `{"model":"grok-imagine-video-1.5","image":{"url":"https://example.com/a.png"},"resolution":"720p"}`,
			requested:  "720p",
			effective:  "720p",
			imageCount: 1,
		},
		{
			name:            "reference counts",
			body:            `{"model":"grok-imagine-video-1.5","prompt":"orbit","reference_images":[{"file_id":"1"},{"file_id":"2"}],"reference_audios":[{"voice_id":"eve"}]}`,
			effective:       "480p",
			defaulted:       true,
			imageCount:      2,
			referenceAudios: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, info := newVideoContext(test.body)
			require.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info))
			request, err := relaycommon.GetTaskRequest(context)
			require.NoError(t, err)
			assert.Equal(t, test.requested, request.Resolution)
			assert.Equal(t, test.effective, request.EffectiveResolution)
			assert.Equal(t, test.defaulted, request.ResolutionDefaulted)
			assert.Equal(t, test.imageCount, request.ImageCount)
			assert.Equal(t, test.referenceAudios, request.ReferenceAudioCount)
		})
	}
}

func TestVideoBillingMatchesOfficialPerSecondPrices(t *testing.T) {
	tests := []struct {
		model string
		body  string
		price float64
	}{
		{model: "grok-imagine-video", body: `{"model":"grok-imagine-video","prompt":"orbit","duration":5,"resolution":"480p"}`, price: 0.25},
		{model: "grok-imagine-video", body: `{"model":"grok-imagine-video","prompt":"orbit","duration":5,"resolution":"720p"}`, price: 0.35},
		{model: "grok-imagine-video-1.5", body: `{"model":"grok-imagine-video-1.5","prompt":"orbit","duration":5,"resolution":"1080p"}`, price: 1.25},
		{model: "grok-imagine-video-1.5", body: `{"model":"grok-imagine-video-1.5","image":{"url":"https://example.com/a.png"},"duration":5,"resolution":"480p"}`, price: 0.41},
		{model: "grok-imagine-video-1.5", body: `{"model":"grok-imagine-video-1.5","prompt":"scene","reference_images":[{"url":"https://example.com/a.png"},{"file_id":"file-2"}],"duration":15,"resolution":"720p"}`, price: 2.12},
		{model: "grok-imagine-video-1.5-preview", body: `{"model":"grok-imagine-video-1.5-preview","image":{"url":"https://example.com/a.png"},"duration":5,"resolution":"720p"}`, price: 0.71},
		{model: "grok-imagine-video-1.5-2026-05-30", body: `{"model":"grok-imagine-video-1.5-2026-05-30","image":{"url":"https://example.com/a.png"},"duration":5,"resolution":"1080p"}`, price: 1.26},
	}

	for _, test := range tests {
		context, info := newVideoContext(test.body)
		adaptor := &TaskAdaptor{}
		require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
		ratios := adaptor.EstimateBilling(context, info)
		basePrice := ratio_setting.GetDefaultModelPriceMap()[test.model]
		total := basePrice
		for _, ratio := range ratios {
			total *= ratio
		}
		assert.InDelta(t, test.price, total, 1e-9)
	}
}

func TestBuildRequestBodyPreservesOfficialExtensionFields(t *testing.T) {
	context, info := newVideoContextForPath(
		"/grok/v1/videos/extensions",
		`{"model":"mapped-client-model","prompt":"zoom out","video":{"file_id":"file-video"},"duration":6,"storage_options":{"filename":"result.mp4","public_url":true}}`,
	)
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
	info.ChannelMeta = &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-video"}

	body, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var request map[string]any
	require.NoError(t, common.Unmarshal(data, &request))
	assert.Equal(t, "grok-imagine-video", request["model"])
	assert.Equal(t, float64(6), request["duration"])
	assert.Equal(t, map[string]any{"file_id": "file-video"}, request["video"])
	assert.Equal(t, map[string]any{"filename": "result.mp4", "public_url": true}, request["storage_options"])
}

func TestBuildRequestBodyPreservesCompleteOfficialReferenceRequest(t *testing.T) {
	context, info := newVideoContext(`{
		"model":"grok-imagine-video-1.5",
		"prompt":"the subject speaks to camera",
		"reference_images":[
			{"url":"https://example.com/subject.png"},
			{"file_id":"file-outfit"}
		],
		"reference_audios":[
			{"voice_id":"eve"},
			{"url":"data:audio/wav;base64,AAAA"}
		],
		"duration":"15",
		"aspect_ratio":"9:16",
		"resolution":"720p",
		"output":{"upload_url":"https://uploads.example.com/video"},
		"storage_options":{"filename":"result.mp4","expires_after":7200,"public_url":{"expires_after":3600}},
		"user":""
	}`)
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
	assert.Equal(t, constant.TaskActionReferenceGenerate, info.Action)
	info.ChannelMeta = &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-video-1.5"}

	body, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var request map[string]any
	require.NoError(t, common.Unmarshal(data, &request))
	assert.Equal(t, "grok-imagine-video-1.5", request["model"])
	assert.Equal(t, float64(15), request["duration"])
	assert.Equal(t, "9:16", request["aspect_ratio"])
	assert.Equal(t, "720p", request["resolution"])
	assert.Equal(t, "", request["user"])
	assert.Equal(t, []any{
		map[string]any{"url": "https://example.com/subject.png"},
		map[string]any{"file_id": "file-outfit"},
	}, request["reference_images"])
	assert.Equal(t, []any{
		map[string]any{"voice_id": "eve"},
		map[string]any{"url": "data:audio/wav;base64,AAAA"},
	}, request["reference_audios"])
	assert.Equal(t, map[string]any{"upload_url": "https://uploads.example.com/video"}, request["output"])
	assert.Equal(t, map[string]any{
		"filename":      "result.mp4",
		"expires_after": float64(7200),
		"public_url":    map[string]any{"expires_after": float64(3600)},
	}, request["storage_options"])
}

func TestParseTaskResult(t *testing.T) {
	adaptor := &TaskAdaptor{}
	tests := []struct {
		body   string
		status model.TaskStatus
		url    string
		reason string
	}{
		{body: `{"status":"pending"}`, status: model.TaskStatusInProgress},
		{body: `{"status":"done","progress":100,"video":{"url":"https://vidgen.x.ai/video.mp4","duration":8,"respect_moderation":true,"file_output":{"file_id":"file-video","filename":"video.mp4"}},"usage":{"cost_in_usd_ticks":8100000000,"input_tokens":11,"input_tokens_details":{"cached_tokens":2,"image_tokens":4,"text_tokens":7},"output_tokens":22,"output_tokens_details":{"image_tokens":10,"reasoning_tokens":5,"text_tokens":7},"total_tokens":33}}`, status: model.TaskStatusSuccess, url: "https://vidgen.x.ai/video.mp4"},
		{body: `{"status":"expired"}`, status: model.TaskStatusFailure, reason: "expired"},
		{body: `{"status":"failed","error":{"code":"generation_failed","message":"blocked"}}`, status: model.TaskStatusFailure, reason: "blocked"},
	}

	for _, test := range tests {
		result, err := adaptor.ParseTaskResult([]byte(test.body))
		require.NoError(t, err)
		assert.Equal(t, string(test.status), result.Status)
		assert.Equal(t, test.url, result.Url)
		assert.Equal(t, test.reason, result.Reason)
		if test.status == model.TaskStatusSuccess {
			assert.Equal(t, "100%", result.Progress)
			assert.Equal(t, 11, result.PromptTokens)
			assert.Equal(t, 22, result.CompletionTokens)
			assert.Equal(t, 33, result.TotalTokens)
			assert.Equal(t, int64(8100000000), result.Metadata["cost_in_usd_ticks"])
			assert.Equal(t, 2, result.Metadata["cached_tokens"])
			assert.Equal(t, map[string]int{"text": 7, "image": 4}, result.InputTokensByModality)
			assert.Equal(t, map[string]int{"text": 7, "image": 10}, result.OutputTokensByModality)
			assert.Equal(t, 5, result.ThoughtTokens)
			assert.Equal(t, true, result.Metadata["respect_moderation"])
			require.NotNil(t, result.Metadata["file_output"])
		}
	}
}

func TestDoResponseReturnsPublicRequestID(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"upstream-request"}`)),
	}
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"}}

	upstreamID, data, taskErr := (&TaskAdaptor{}).DoResponse(context, response, info)
	require.Nil(t, taskErr)
	assert.Equal(t, "upstream-request", upstreamID)
	assert.JSONEq(t, `{"request_id":"upstream-request"}`, string(data))
	assert.JSONEq(t, `{"request_id":"task_public"}`, recorder.Body.String())
}
