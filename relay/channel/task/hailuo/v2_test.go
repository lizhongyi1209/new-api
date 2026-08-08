package hailuo

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMiniMaxH3RequestScenarios(t *testing.T) {
	validText := func() miniMaxH3Request {
		duration := 5
		return miniMaxH3Request{
			Model:      miniMaxH3Model,
			Content:    []taskcommon.VideoContentItem{{Type: "text", Text: "a quiet ocean at sunrise"}},
			Resolution: Resolution2K,
			Duration:   &duration,
			Ratio:      "16:9",
		}
	}

	t.Run("accepts text to video", func(t *testing.T) {
		request := validText()
		_, err := validateMiniMaxH3Request(&request)
		require.NoError(t, err)
	})
	t.Run("rejects adaptive ratio for text to video", func(t *testing.T) {
		request := validText()
		request.Ratio = "adaptive"
		_, err := validateMiniMaxH3Request(&request)
		require.Error(t, err)
	})
	t.Run("rejects frame and reference mixing", func(t *testing.T) {
		request := validText()
		request.Content = append(request.Content,
			taskcommon.VideoContentItem{Type: "image_url", ImageURL: &taskcommon.MediaURL{URL: "https://example.com/frame.png"}, Role: "first_frame"},
			taskcommon.VideoContentItem{Type: "audio_url", AudioURL: &taskcommon.MediaURL{URL: "https://example.com/ref.mp3"}, Role: "reference_audio"},
		)
		_, err := validateMiniMaxH3Request(&request)
		require.Error(t, err)
	})
	t.Run("bounds duration", func(t *testing.T) {
		request := validText()
		invalid := 16
		request.Duration = &invalid
		_, err := validateMiniMaxH3Request(&request)
		require.Error(t, err)
	})
}

func TestTaskAdaptorMiniMaxH3URLAndQuery(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/query/video_generation/task%2Fid", r.URL.EscapedPath())
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, `{"task":{"id":"task/id","status":"succeeded","content":{"url":"https://cdn.example.com/video.mp4"},"duration":5,"resolution":"2K","ratio":"16:9"}}`)
	})
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: handler}}
	server.Start()
	defer server.Close()

	adaptor := &TaskAdaptor{}
	resp, err := adaptor.FetchTask(server.URL, "key", map[string]any{
		"task_id":        "task/id",
		"model":          miniMaxH3Model,
		"upstream_model": miniMaxH3Model,
	}, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	result, err := adaptor.ParseTaskResult(body)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, "https://cdn.example.com/video.mp4", result.Url)
	assert.Equal(t, "2K", result.Metadata["resolution"])
}

func TestTaskAdaptorBuildsMiniMaxH3Payload(t *testing.T) {
	duration := 5
	request := miniMaxH3Request{
		Model:      miniMaxH3Model,
		Content:    []taskcommon.VideoContentItem{{Type: "text", Text: "a cinematic sunrise"}},
		Resolution: Resolution2K,
		Duration:   &duration,
		Ratio:      "16:9",
	}
	data, err := common.Marshal(request)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(string(data)))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		OriginModelName: miniMaxH3Model,
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api.minimax.chat"},
	}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
	info.UpstreamModelName = miniMaxH3Model
	info.PriceData.ModelRatio = 1
	ratios := adaptor.EstimateBilling(c, info)
	assert.InDelta(t, 4.0/miniMaxH3USDRMBRate()/0.5, ratios["minimax_h3_cost"], 1e-9)
	require.NotNil(t, info.VideoBilling)
	assert.Equal(t, "video_per_second", info.VideoBilling.BillingMode)
	assert.Equal(t, "CNY", info.VideoBilling.Currency)
	assert.Equal(t, 5, info.VideoBilling.OutputSeconds)
	assert.Equal(t, 0.8, info.VideoBilling.OutputUnitRate)
	assert.Equal(t, 4.0, info.VideoBilling.ProviderCost)
	assert.True(t, info.VideoBilling.Estimated)
	bodyReader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	body, err := io.ReadAll(bodyReader)
	require.NoError(t, err)
	var got miniMaxH3Request
	require.NoError(t, common.Unmarshal(body, &got))
	assert.Equal(t, miniMaxH3Model, got.Model)
	assert.Equal(t, "16:9", got.Ratio)
	require.NotNil(t, got.Duration)
	assert.Equal(t, 5, *got.Duration)
}

func TestMiniMaxH3BaseURLDefaultsToOfficialAPI(t *testing.T) {
	assert.Equal(t, MiniMaxV2BaseURL, miniMaxH3BaseURL(""))
	assert.Equal(t, MiniMaxV2BaseURL, miniMaxH3BaseURL("https://api.minimaxi.com"))
	assert.Equal(t, MiniMaxV2BaseURL, miniMaxH3BaseURL(MiniMaxLegacyBaseURL))
	assert.Equal(t, "https://proxy.example/v2/video_generation", miniMaxH3BaseURL("https://proxy.example")+VideoGenerationV2Endpoint)
}

func TestMiniMaxH3TaskErrorResponse(t *testing.T) {
	err := miniMaxV2TaskError([]byte(`{"type":"error","error":{"type":"bad_request_error","message":"invalid duration"}}`), http.StatusBadRequest)
	require.NotNil(t, err)
	assert.True(t, err.LocalError)
	assert.Equal(t, "bad_request_error", err.Code)
	assert.True(t, strings.Contains(err.Message, "invalid duration"))
}

func TestMiniMaxH3BillingUsesOfficialRates(t *testing.T) {
	assert.InDelta(t, 4.2, miniMaxH3CostRMB(5, 0, 6, Resolution2K), 1e-9)
	assert.Equal(t, 3.0, miniMaxH3CostRMB(6, 0, 0, Resolution768P))

	task := &model.Task{
		Quota:      1,
		Properties: model.Properties{OriginModelName: miniMaxH3Model},
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{GroupRatio: 1},
		},
		Data: []byte(`{"task":{"status":"succeeded","resolution":"2K","duration":5,"usage":{"input_seconds":3,"output_seconds":5,"input_image_count":6}}}`),
	}
	// (8 seconds * 0.80 + 1 extra image * 0.20) RMB, converted to quota.
	want := common.QuotaRound(6.6 / miniMaxH3USDRMBRate() * common.QuotaPerUnit)
	taskResult := &relaycommon.TaskInfo{}
	assert.Equal(t, want, (&TaskAdaptor{}).AdjustBillingOnComplete(task, taskResult))
	require.NotNil(t, taskResult.VideoBilling)
	assert.Equal(t, 5, taskResult.VideoBilling.OutputSeconds)
	assert.Equal(t, 3, taskResult.VideoBilling.ReferenceVideoInputSeconds)
	assert.Equal(t, 6, taskResult.VideoBilling.ImageCount)
	assert.Equal(t, 1, taskResult.VideoBilling.BilledImageCount)
	assert.InDelta(t, 6.6, taskResult.VideoBilling.ProviderCost, 1e-9)
	assert.Equal(t, task.Quota, taskResult.VideoBilling.PreConsumedQuota)
	assert.Equal(t, want-task.Quota, taskResult.VideoBilling.SettlementDeltaQuota)
	assert.Equal(t, want, taskResult.VideoBilling.FinalQuota)
}

func TestConvertMiniMaxH3TaskToOpenAIVideo(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		Properties: model.Properties{OriginModelName: miniMaxH3Model},
		Data:       []byte(`{"task":{"id":"upstream","status":"succeeded","content":{"url":"https://cdn.example.com/video.mp4"},"resolution":"2K","duration":5,"ratio":"16:9"}}`),
	}
	data, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	var response map[string]any
	require.NoError(t, common.Unmarshal(data, &response))
	assert.Equal(t, miniMaxH3Model, response["model"])
	assert.Equal(t, "5", response["seconds"])
}
