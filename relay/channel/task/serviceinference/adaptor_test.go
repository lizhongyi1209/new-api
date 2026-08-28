package serviceinference

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetAssetGroupCache() {
	assetGroupCache = sync.Map{}
}

func newTestRelayInfo(serverURL string, other dto.ChannelOtherSettings) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          constant.ChannelTypeServiceInferenceVideo,
			ChannelId:            1001,
			ChannelBaseUrl:       serverURL,
			ApiKey:               "test-key",
			ChannelOtherSettings: other,
		},
	}
}

func newTaskContext(body string) *gin.Context {
	return newTaskContextForPath("/v1/videos", body)
}

func newTaskContextForPath(path string, body string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	data, err := common.Marshal(v)
	require.NoError(t, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(data)
	require.NoError(t, err)
}

func requireBearer(t *testing.T, r *http.Request) {
	t.Helper()
	assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
}

func TestGrokServiceInferencePreservesOfficialImageRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := newTestRelayInfo("https://example.com", dto.ChannelOtherSettings{})
	info.OriginModelName = "grok-imagine-video-1.5"
	requestBody := `{
		"model":"grok-imagine-video-1.5",
		"prompt":"make the light move",
		"image":{"image_url":"https://cdn.example.com/reference.png"},
		"duration":15,
		"resolution":"720p",
		"aspect_ratio":"9:16"
	}`
	context := newTaskContextForPath("/grok/v1/videos/generations", requestBody)

	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
	assert.Equal(t, constant.TaskActionGenerate, info.Action)
	assert.Equal(t, map[string]float64{
		"seconds":     15,
		"resolution":  1.75,
		"image_input": 2.11 / 2.1,
	}, adaptor.EstimateBilling(context, info))

	reader, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.JSONEq(t, requestBody, string(data))
	assert.NotContains(t, string(data), "reference_images")

	taskRequest, err := relaycommon.GetTaskRequest(context)
	require.NoError(t, err)
	assert.Equal(t, 1, taskRequest.ImageCount)
	assert.Equal(t, "720p", taskRequest.EffectiveResolution)
}

func TestGrokServiceInferenceDoesNotInjectWrapperDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := newTestRelayInfo("https://example.com", dto.ChannelOtherSettings{})
	info.OriginModelName = "grok-imagine-video-1.5"
	requestBody := `{
		"model":"grok-imagine-video-1.5",
		"prompt":"move",
		"image":{"url":"https://cdn.example.com/reference.png"}
	}`
	context := newTaskContextForPath("/grok/v1/videos/generations", requestBody)

	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
	reader, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.JSONEq(t, requestBody, string(data))
	assert.NotContains(t, string(data), "duration")
	assert.NotContains(t, string(data), "resolution")
	assert.NotContains(t, string(data), "aspect_ratio")
	assert.Equal(t, map[string]float64{
		"seconds":     8,
		"resolution":  1,
		"image_input": 0.65 / 0.64,
	}, adaptor.EstimateBilling(context, info))
}

func TestGrokServiceInferenceSupportsOfficialTextAndReferenceModes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		body   string
		action string
	}{
		{
			name:   "text to video",
			body:   `{"model":"grok-imagine-video-1.5","prompt":"a rocket launch"}`,
			action: constant.TaskActionTextGenerate,
		},
		{
			name:   "reference to video",
			body:   `{"model":"grok-imagine-video-1.5","prompt":"the subject walks on stage","reference_images":[{"url":"https://cdn.example.com/subject.png"},{"image_url":"https://cdn.example.com/outfit.png"}],"duration":15,"resolution":"720p"}`,
			action: constant.TaskActionReferenceGenerate,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := newTestRelayInfo("https://example.com", dto.ChannelOtherSettings{})
			context := newTaskContextForPath("/grok/v1/videos/generations", test.body)
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
			assert.Equal(t, test.action, info.Action)

			reader, err := adaptor.BuildRequestBody(context, info)
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.JSONEq(t, test.body, string(data))
		})
	}
}

func TestGrokServiceInferenceAcceptsOfficialDurationAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		field    string
		expected int
	}{
		{name: "duration string", field: `"duration":"5"`, expected: 5},
		{name: "seconds alias", field: `"seconds":"6"`, expected: 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := newTestRelayInfo("https://example.com", dto.ChannelOtherSettings{})
			context := newTaskContextForPath("/grok/v1/videos/generations", `{
				"model":"grok-imagine-video-1.5",
				"prompt":"move",
				"image":{"url":"https://cdn.example.com/reference.png"},
				"aspect_ratio":"16:9",
				`+test.field+`
			}`)
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
			reader, err := adaptor.BuildRequestBody(context, info)
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(data, &payload))
			assert.Equal(t, float64(test.expected), payload["duration"])
			assert.NotContains(t, payload, "seconds")
		})
	}
}

func TestGrokServiceInferencePassesOfficialOnlyInputs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		path   string
		body   string
		action string
	}{
		{
			name:   "file id image",
			body:   `{"model":"grok-imagine-video-1.5","prompt":"move","image":{"file_id":"file-image"}}`,
			action: constant.TaskActionGenerate,
		},
		{
			name:   "image without prompt or aspect ratio",
			body:   `{"model":"grok-imagine-video-1.5","image":{"url":"https://cdn.example.com/reference.png"}}`,
			action: constant.TaskActionGenerate,
		},
		{
			name:   "reference audio voice",
			body:   `{"model":"grok-imagine-video-1.5","prompt":"speak","reference_audios":[{"voice_id":"eve"}]}`,
			action: constant.TaskActionReferenceGenerate,
		},
		{
			name:   "storage and user",
			body:   `{"model":"grok-imagine-video-1.5","prompt":"orbit","storage_options":{"filename":"result.mp4"},"user":"user-1"}`,
			action: constant.TaskActionTextGenerate,
		},
		{
			name:   "video edit",
			path:   "/grok/v1/videos/edits",
			body:   `{"model":"grok-imagine-video-1.5","prompt":"add snow","video":{"url":"https://cdn.example.com/video.mp4"}}`,
			action: constant.TaskActionVideoEdit,
		},
		{
			name:   "video extension",
			path:   "/grok/v1/videos/extensions",
			body:   `{"model":"grok-imagine-video-1.5","prompt":"continue","video":{"file_id":"file-video"},"duration":6}`,
			action: constant.TaskActionVideoExtend,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := newTestRelayInfo("https://example.com", dto.ChannelOtherSettings{})
			path := test.path
			if path == "" {
				path = "/grok/v1/videos/generations"
			}
			context := newTaskContextForPath(path, test.body)
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
			assert.Equal(t, test.action, info.Action)

			reader, err := adaptor.BuildRequestBody(context, info)
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.JSONEq(t, test.body, string(data))
		})
	}
}

func TestGrokDoResponseKeepsPublicContract(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/grok/v1/videos/generations", nil)
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"task":{"id":"mvt-upstream","status":"pending"}}`)),
	}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
		OriginModelName: "grok-imagine-video-1.5",
	}

	upstreamID, data, taskErr := (&TaskAdaptor{}).DoResponse(context, response, info)
	require.Nil(t, taskErr)
	assert.Equal(t, "mvt-upstream", upstreamID)
	assert.JSONEq(t, `{"task":{"id":"mvt-upstream","status":"pending"}}`, string(data))
	assert.JSONEq(t, `{"request_id":"task_public"}`, recorder.Body.String())
}

func TestBuildRequestBodyNativeContentUploadsImageAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetAssetGroupCache()

	var createdAsset map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		switch r.URL.Path {
		case "/v1/asset-groups":
			require.Equal(t, http.MethodPost, r.Method)
			var body map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &body))
			assert.Equal(t, defaultAssetGroupName, body["name"])
			writeJSON(t, w, http.StatusOK, map[string]any{
				"id":          "group-1",
				"name":        body["name"],
				"description": body["description"],
			})
		case "/v1/assets":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, common.DecodeJson(r.Body, &createdAsset))
			writeJSON(t, w, http.StatusOK, map[string]any{
				"id":      "asset-1",
				"task_id": "asset-task-1",
				"status":  "processing",
			})
		case "/v1/assets/get":
			require.Equal(t, http.MethodPost, r.Method)
			writeJSON(t, w, http.StatusOK, map[string]any{
				"id":     "asset-1",
				"status": "completed",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	info := newTestRelayInfo(server.URL, dto.ChannelOtherSettings{
		ServiceInferenceAssetPollAttempts:   1,
		ServiceInferenceAssetPollIntervalMS: -1,
	})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	c := newTaskContext(`{
		"model": "dreamina-seedance-2-0-260128",
		"content": [
			{"type": "text", "text": "singing"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example.com/ref.png"}, "role": "reference_image"},
			{"type": "audio_url", "audio_url": {"url": "https://cdn.example.com/ref.mp3"}, "role": "reference_audio"}
		],
		"duration": 4,
		"resolution": "480p",
		"ratio": "16:9",
		"generate_audio": true,
		"watermark": false,
		"return_last_frame": true
	}`)

	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload requestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	require.Len(t, payload.Content, 3)
	assert.Equal(t, "asset://asset-1", payload.Content[1].ImageURL.URL)
	assert.Equal(t, "https://cdn.example.com/ref.mp3", payload.Content[2].AudioURL.URL)
	require.NotNil(t, payload.Duration)
	assert.Equal(t, 4, *payload.Duration)
	assert.Equal(t, "480p", payload.Resolution)
	assert.Equal(t, "16:9", payload.Ratio)
	require.NotNil(t, payload.Watermark)
	assert.False(t, *payload.Watermark)
	assert.Equal(t, "group-1", createdAsset["group_id"])
	assert.Equal(t, "https://cdn.example.com/ref.png", createdAsset["url"])
	assert.Equal(t, "reference_image", createdAsset["name"])
}

func TestBuildRequestBodyDFContentUsesDirectAssetWorkflow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var createdAsset map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		switch r.URL.Path {
		case "/v1/sd-5/assets":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, common.DecodeJson(r.Body, &createdAsset))
			writeJSON(t, w, http.StatusOK, map[string]any{
				"success": true,
				"data": map[string]any{
					"Id":     "asset-df-1",
					"Status": nil,
					"Error":  nil,
				},
			})
		case "/v1/sd-5/assets/asset-df-1":
			require.Equal(t, http.MethodGet, r.Method)
			writeJSON(t, w, http.StatusOK, map[string]any{
				"success": true,
				"data": map[string]any{
					"Id":     "asset-df-1",
					"Status": "Active",
					"Error":  nil,
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	info := newTestRelayInfo(server.URL, dto.ChannelOtherSettings{
		ServiceInferenceAssetPollAttempts:   1,
		ServiceInferenceAssetPollIntervalMS: -1,
	})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	c := newTaskContext(`{
		"model": "dreamina-seedance-2-0-260128-df",
		"content": [
			{"type": "text", "text": "dancing"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example.com/ref.png"}, "role": "reference_image"}
		]
	}`)

	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload requestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	require.Len(t, payload.Content, 2)
	assert.Equal(t, "asset://asset-df-1", payload.Content[1].ImageURL.URL)
	assert.Equal(t, "https://cdn.example.com/ref.png", createdAsset["URL"])
	assert.Equal(t, "reference_image", createdAsset["Name"])
	assert.Equal(t, "Image", createdAsset["AssetType"])
}

func TestBuildRequestBodyHCContentUsesHCAssetWorkflow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var createdAsset map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		switch r.URL.Path {
		case "/v1/sd/assets":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, common.DecodeJson(r.Body, &createdAsset))
			writeJSON(t, w, http.StatusOK, map[string]any{
				"success": true,
				"data": map[string]any{
					"Id":     "asset-hc-1",
					"Status": nil,
					"Error":  nil,
				},
			})
		case "/v1/sd/assets/asset-hc-1":
			require.Equal(t, http.MethodGet, r.Method)
			writeJSON(t, w, http.StatusOK, map[string]any{
				"success": true,
				"data": map[string]any{
					"Id":     "asset-hc-1",
					"Status": "Active",
					"Error":  nil,
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	info := newTestRelayInfo(server.URL, dto.ChannelOtherSettings{
		ServiceInferenceAssetPollAttempts:   1,
		ServiceInferenceAssetPollIntervalMS: -1,
	})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	c := newTaskContext(`{
		"model": "dreamina-seedance-2-0-hc",
		"content": [
			{"type": "text", "text": "dancing"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example.com/ref.png"}, "role": "reference_image"}
		]
	}`)

	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload requestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	require.Len(t, payload.Content, 2)
	assert.Equal(t, "asset://asset-hc-1", payload.Content[1].ImageURL.URL)
	assert.Equal(t, "https://cdn.example.com/ref.png", createdAsset["URL"])
	assert.Equal(t, "reference_image", createdAsset["Name"])
	assert.Equal(t, "Image", createdAsset["AssetType"])
}

func TestSeedanceDFModelClassification(t *testing.T) {
	for _, modelName := range []string{
		"dreamina-seedance-2-0-260128-df",
		"dreamina-seedance-2-0-fast-260128-df",
		"dreamina-seedance-2-0-mini-260615-df",
		"dreamina-seedance-2-5-260628-df",
	} {
		assert.True(t, isSeedanceDFModel(modelName), modelName)
		assert.Contains(t, ModelList, modelName)
	}
	assert.False(t, isSeedanceDFModel("dreamina-seedance-2-0-hc"))
	assert.False(t, isSeedanceDFModel("dreamina-seedance-2-0-260128"))
}

func TestSeedanceHCModelClassification(t *testing.T) {
	for _, modelName := range []string{
		"dreamina-seedance-2-0-hc",
		"dreamina-seedance-2-0-fast-hc",
		"dreamina-seedance-2-0-mini-hc",
		"dreamina-seedance-2-5-hc",
	} {
		assert.True(t, isSeedanceHCModel(modelName), modelName)
		workflow, basePath, ok := seedanceDirectAssetWorkflow(modelName)
		assert.True(t, ok, modelName)
		assert.Equal(t, "hc", workflow)
		assert.Equal(t, "/v1/sd/assets", basePath)
	}
	assert.False(t, isSeedanceHCModel("dreamina-seedance-2-0-260128-df"))
	assert.False(t, isSeedanceHCModel("dreamina-seedance-2-0-260128"))
}

func TestBuildRequestBodyRecreatesMissingConfiguredAssetGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetAssetGroupCache()

	var assetGroupUsed string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		switch r.URL.Path {
		case "/v1/asset-groups/group-old":
			require.Equal(t, http.MethodGet, r.Method)
			writeJSON(t, w, http.StatusNotFound, map[string]any{"error": "not found"})
		case "/v1/asset-groups":
			require.Equal(t, http.MethodPost, r.Method)
			writeJSON(t, w, http.StatusOK, map[string]any{"id": "group-new"})
		case "/v1/assets":
			var body map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &body))
			assetGroupUsed, _ = body["group_id"].(string)
			writeJSON(t, w, http.StatusOK, map[string]any{
				"id":      "asset-new",
				"task_id": "asset-task-new",
				"status":  "completed",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	info := newTestRelayInfo(server.URL, dto.ChannelOtherSettings{
		ServiceInferenceAssetGroupID:        "group-old",
		ServiceInferenceAssetPollAttempts:   1,
		ServiceInferenceAssetPollIntervalMS: -1,
	})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	c := newTaskContext(`{
		"model": "dreamina-seedance-2-0-260128",
		"content": [
			{"type": "text", "text": "singing"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example.com/ref.png"}}
		]
	}`)

	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload requestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, "asset://asset-new", payload.Content[1].ImageURL.URL)
	assert.Equal(t, "group-new", assetGroupUsed)
}

func TestParseTaskResultCompletedUsage(t *testing.T) {
	adaptor := &TaskAdaptor{}
	taskInfo, err := adaptor.ParseTaskResult([]byte(`{
		"task": {
			"id": "mvt-1",
			"status": "completed",
			"model": "dreamina-seedance-2-0-260128",
			"duration_seconds": 4,
			"outputs": ["https://cdn.example.com/video.mp4"],
			"usage": {
				"completion_tokens": 40594,
				"total_tokens": 40594
			},
			"last_frame_url": "https://model.service-inference.ai/v1/video/files/mvt-1/last-frame"
		}
	}`))

	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, taskInfo.Status)
	assert.Equal(t, "https://cdn.example.com/video.mp4", taskInfo.Url)
	assert.Equal(t, 40594, taskInfo.CompletionTokens)
	assert.Equal(t, 40594, taskInfo.TotalTokens)
}

func TestParseGrokTaskResultReadsMetadataUsage(t *testing.T) {
	adaptor := &TaskAdaptor{}
	taskInfo, err := adaptor.ParseTaskResult([]byte(`{
		"task": {
			"id": "mvt-1",
			"status": "completed",
			"model": "grok-imagine-video-1.5",
			"duration_seconds": 5,
			"outputs": ["https://cdn.example.com/video.mp4"],
			"metadata": {
				"progress": 100,
				"video": {"url":"https://cdn.example.com/video.mp4","duration":5.04,"respect_moderation":true},
				"usage": {"cost_in_usd_ticks":7100000000}
			}
		}
	}`))

	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, taskInfo.Status)
	assert.Equal(t, "https://cdn.example.com/video.mp4", taskInfo.Url)
	require.NotNil(t, taskInfo.Metadata)
	assert.Equal(t, float64(5.04), taskInfo.Metadata["duration"])
	assert.Equal(t, 100, taskInfo.Metadata["progress"])
	assert.Equal(t, int64(7100000000), taskInfo.Metadata["cost_in_usd_ticks"])
}

func TestFetchTaskUsesTaskEndpointAndBearer(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer fetch-key", r.Header.Get("Authorization"))
		writeJSON(t, w, http.StatusOK, map[string]any{
			"task": map[string]any{
				"id":     "mvt-1",
				"status": "pending",
			},
		})
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{}
	resp, err := adaptor.FetchTask(server.URL, "fetch-key", map[string]any{"task_id": "mvt-1"}, "")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "/v1/video/tasks/mvt-1", requestedPath)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestConvertToOpenAIVideoQueuedTaskWithoutData(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{
		TaskID:    "task-public",
		Status:    model.TaskStatusQueued,
		Progress:  "20%",
		CreatedAt: 1782551878,
		UpdatedAt: 1782551878,
		Properties: model.Properties{
			OriginModelName: "dreamina-seedance-2-0-260128",
		},
	}

	data, err := adaptor.ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(data, &video))
	assert.Equal(t, "task-public", video.ID)
	assert.Equal(t, dto.VideoStatusQueued, video.Status)
	assert.Equal(t, 20, video.Progress)
	assert.Equal(t, "dreamina-seedance-2-0-260128", video.Model)
	assert.Nil(t, video.Error)
	assert.Nil(t, video.Metadata)
}

// TestVideoInputRatio 锁定 TokenMartSeedance 各模型的视频输入计费倍率。
func TestVideoInputRatio(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		resolution string
		hasVideo   bool
		wantRatio  float64
		wantOK     bool
	}{
		// 标准版：720p 基准 46；含视频 28；1080p 51/含视频 31；4k 26/含视频 16。
		{"standard 720p base", "dreamina-seedance-2-0-260128", "720p", false, 1.0, true},
		{"standard 720p video", "dreamina-seedance-2-0-260128", "720p", true, 28.0 / 46.0, true},
		{"standard 1080p", "dreamina-seedance-2-0-260128", "1080p", false, 51.0 / 46.0, true},
		{"standard 1080p video", "dreamina-seedance-2-0-260128", "1080p", true, 31.0 / 46.0, true},
		{"standard 4k", "dreamina-seedance-2-0-260128", "4k", false, 26.0 / 46.0, true},
		{"standard 4k video", "dreamina-seedance-2-0-260128", "4k", true, 16.0 / 46.0, true},
		// fast 版：基准 37/含视频 22；官方未配置 1080p/4k，回落基准价（倍率 1.0）。
		{"fast 720p base", "dreamina-seedance-2-0-fast-260128", "720p", false, 1.0, true},
		{"fast 720p video", "dreamina-seedance-2-0-fast-260128", "720p", true, 22.0 / 37.0, true},
		{"fast 1080p falls back to base", "dreamina-seedance-2-0-fast-260128", "1080p", false, 1.0, true},
		// Seedance 2.5：480p/720p token 单价相同；含视频输入价格为 42/70。
		{"2.5 480p base", "dreamina-seedance-2-5-ep", "480p", false, 1.0, true},
		{"2.5 480p video", "dreamina-seedance-2-5-ep", "480p", true, 42.0 / 70.0, true},
		{"2.5 variant 720p base", "future-provider-seedance-2-5-preview", "720p", false, 1.0, true},
		{"2.5 variant 720p video", "future-provider-seedance-2-5-preview", "720p", true, 42.0 / 70.0, true},
		{"unknown future model has no implicit pricing", "future-video-model", "720p", true, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ratio, ok := videoInputRatio(tc.model, tc.resolution, tc.hasVideo)
			require.Equal(t, tc.wantOK, ok)
			assert.InDelta(t, tc.wantRatio, ratio, 1e-9)
		})
	}
}

func TestEstimateBillingUsesMappedSeedance25UpstreamModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := newTestRelayInfo("https://example.com", dto.ChannelOtherSettings{})
	info.OriginModelName = "public-seedance-2-5"
	info.UpstreamModelName = "dreamina-seedance-2-5-ep"

	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	context := newTaskContext(`{
		"model": "public-seedance-2-5",
		"content": [
			{"type": "text", "text": "extend the scene"},
			{"type": "video_url", "video_url": {"url": "https://example.com/input.mp4"}}
		],
		"resolution": "720p"
	}`)

	taskErr := adaptor.ValidateRequestAndSetAction(context, info)
	require.Nil(t, taskErr)
	assert.Equal(t, map[string]float64{"video_input": 42.0 / 70.0}, adaptor.EstimateBilling(context, info))
}
