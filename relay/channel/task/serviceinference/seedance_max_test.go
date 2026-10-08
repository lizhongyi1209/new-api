package serviceinference

import (
	"io"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedanceMaxPreservesDocumentedParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		model         string
		upstreamModel string
	}{
		{"doubao-seedance-2-5-260628-max", "doubao-seedance-2-5-260628-max"},
		{"dreamina-seedance-2-5-260628-max", "dreamina-seedance-2-5-260628-max"},
		{"dreamina-seedance-2-5-hc", "dreamina-seedance-2-5-260628-max"},
		{"custom-seedance", "doubao-seedance-2-5-260628-max"},
	} {
		t.Run(test.model, func(t *testing.T) {
			info := newTestRelayInfo("https://model.service-inference.ai", kitdto.ChannelOtherSettings{})
			info.OriginModelName = test.model
			info.UpstreamModelName = test.upstreamModel
			info.IsModelMapped = test.model != test.upstreamModel
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			request := `{
				"model": "` + test.model + `",
				"content": [
					{"type":"text","text":"follow @Image1 and @Video1"},
					{"type":"image_url","image_url":{"url":"asset://mva-image"},"role":"reference_image"},
					{"type":"video_url","video_url":{"url":"https://cdn.example.com/ref.mp4"},"role":"reference_video"},
					{"type":"audio_url","audio_url":{"url":"https://cdn.example.com/ref.mp3"},"role":"reference_audio"}
				],
				"duration": -1,
				"resolution": "1080p",
				"aspect_ratio": "16:9",
				"generate_audio": false,
				"watermark": false,
				"return_last_frame": false,
				"seed": 0,
				"callback_url": "https://example.com/callback?job_id=123",
				"execution_expires_after": 3600,
				"output_format": "mov",
				"omni_reference_task_type": "edit",
				"aigc_watermark": true,
				"camera_fixed": true
			}`
			c := newTaskContextForPath("/v1/video/generations", request)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
			reader, err := adaptor.BuildRequestBody(c, info)
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)

			var expected map[string]any
			require.NoError(t, common.Unmarshal([]byte(request), &expected))
			expected["model"] = test.upstreamModel
			expected["ratio"] = expected["aspect_ratio"]
			delete(expected, "aspect_ratio")
			delete(expected, "aigc_watermark")
			delete(expected, "camera_fixed")
			expectedBody, err := common.Marshal(expected)
			require.NoError(t, err)
			assert.JSONEq(t, string(expectedBody), string(data))
		})
	}
}

func TestSeedanceMaxOmittedParametersRetainUpstreamDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, modelName := range []string{"doubao-seedance-2-0-260128-max", "dreamina-seedance-2-0-260128-max"} {
		t.Run(modelName, func(t *testing.T) {
			info := newTestRelayInfo("https://model.service-inference.ai", kitdto.ChannelOtherSettings{})
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			request := `{"model":"` + modelName + `","content":[{"type":"text","text":"a landscape"}]}`
			c := newTaskContext(request)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
			reader, err := adaptor.BuildRequestBody(c, info)
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.JSONEq(t, request, string(data))
		})
	}
}

func TestSeedanceMaxRatioOverridesAspectRatio(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := newTestRelayInfo("https://model.service-inference.ai", kitdto.ChannelOtherSettings{})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	c := newTaskContext(`{
		"model":"dreamina-seedance-2-0-260128-max",
		"content":[{"type":"text","text":"a landscape"}],
		"ratio":"9:16","aspect_ratio":"16:9"
	}`)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"model":"dreamina-seedance-2-0-260128-max",
		"content":[{"type":"text","text":"a landscape"}],"ratio":"9:16"
	}`, string(data))
}

func TestServiceInferenceRejectsInvalidDurationWithoutWeakeningLegacyBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name     string
		model    string
		duration int
	}{
		{"max negative", "doubao-seedance-2-0-260128-max", -2},
		{"max oversized", "dreamina-seedance-2-5-260628-max", relaycommon.MaxTaskDurationSeconds + 1},
		{"legacy negative", "dreamina-seedance-2-0-260128-df", -1},
		{"minimax negative", "minimax-h3", -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			info := newTestRelayInfo("https://model.service-inference.ai", kitdto.ChannelOtherSettings{})
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			body, err := common.Marshal(map[string]any{
				"model": test.model, "duration": test.duration,
				"content": []ContentItem{{Type: "text", Text: "a landscape"}},
			})
			require.NoError(t, err)
			taskErr := adaptor.ValidateRequestAndSetAction(newTaskContext(string(body)), info)
			require.NotNil(t, taskErr)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			assert.Equal(t, "invalid_duration", taskErr.Code)
		})
	}
}

func TestSeedanceMaxRejectsOversizedMetadataDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := newTestRelayInfo("https://model.service-inference.ai", kitdto.ChannelOtherSettings{})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	c := newTaskContext("{}")
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model: "doubao-seedance-2-0-260128-max", Prompt: "a landscape",
		Metadata: map[string]any{"duration": relaycommon.MaxTaskDurationSeconds + 1},
	})
	_, err := adaptor.BuildRequestBody(c, info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duration")
}

func TestSeedanceMaxTailFrameIsAvailableInPublicTaskResult(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ConvertToOpenAIVideo(&model.Task{
		TaskID: "task-public", Status: model.TaskStatusSuccess,
		Properties: model.Properties{OriginModelName: "doubao-seedance-2-0-260128-max"},
		Data: []byte(`{"task":{
			"id":"mvt-upstream","status":"completed",
			"outputs":["https://cdn.example.com/video.mp4"],
			"last_frame_url":"https://cdn.example.com/last-frame.png"
		}}`),
	})
	require.NoError(t, err)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(result, &video))
	assert.Equal(t, "task-public", video.ID)
	assert.Equal(t, "https://cdn.example.com/video.mp4", video.ResultURL)
	assert.Equal(t, "https://cdn.example.com/last-frame.png", video.Metadata["last_frame_url"])
}
