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
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
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

// TestVideoInputRatioMirrorsOfficialPricing 锁定本渠道计费与官方 doubao（火山引擎）完全一致：
// 按「输出分辨率 × 是否含视频输入」相对 720p/480p 不含视频基准价取倍率。
func TestVideoInputRatioMirrorsOfficialPricing(t *testing.T) {
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ratio, ok := videoInputRatio(tc.model, tc.resolution, tc.hasVideo)
			require.Equal(t, tc.wantOK, ok)
			assert.InDelta(t, tc.wantRatio, ratio, 1e-9)
		})
	}
}
