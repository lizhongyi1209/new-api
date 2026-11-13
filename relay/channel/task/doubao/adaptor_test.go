package doubao

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTaskResultTerminatesCancelledAndExpiredTasks(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus model.TaskStatus
		wantReason string
	}{
		{name: "running", body: `{"status":"running"}`, wantStatus: model.TaskStatusInProgress},
		{name: "failed", body: `{"status":"failed","error":{"code":"OutputVideoSensitiveContentDetected","message":"sensitive"}}`, wantStatus: model.TaskStatusFailure, wantReason: "sensitive"},
		{name: "cancelled", body: `{"status":"cancelled","error":null}`, wantStatus: model.TaskStatusFailure, wantReason: "task cancelled"},
		{name: "expired", body: `{"status":"expired","error":null}`, wantStatus: model.TaskStatusFailure, wantReason: "task expired"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info, err := (&TaskAdaptor{}).ParseTaskResult([]byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, model.TaskStatus(info.Status))
			assert.Equal(t, tc.wantReason, info.Reason)
		})
	}
}

func TestConvertToOpenAIVideoReportsStoredResultURL(t *testing.T) {
	upstreamURL := "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0-mini/v.mp4?X-Tos-Signature=abc"
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		CreatedAt:  100,
		FinishTime: 260,
		Properties: model.Properties{OriginModelName: "doubao-seedance-2-0-mini", UpstreamModelName: "doubao-seedance-2-0-mini-260615"},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "cgt-20261113180051-x3jui",
			ResultURL:      "https://oss.example.com/output/v.mp4",
		},
		Data: []byte(`{"id":"cgt-20261113180051-x3jui","model":"doubao-seedance-2-0-mini-260615","status":"succeeded",` +
			`"content":{"video_url":"` + upstreamURL + `"},"duration":4,"resolution":"480p","ratio":"16:9",` +
			`"usage":{"completion_tokens":40594,"total_tokens":40594}}`),
	}

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(body, &video))
	assert.Equal(t, "task_public", video.ID)
	assert.Equal(t, dto.VideoStatusCompleted, video.Status)
	assert.Equal(t, "doubao-seedance-2-0-mini", video.Model)
	assert.Equal(t, int64(260), video.CompletedAt)
	assert.Equal(t, "https://oss.example.com/output/v.mp4", video.ResultURL)
	assert.Equal(t, "https://oss.example.com/output/v.mp4", video.Metadata["url"])
	assert.Equal(t, "4", video.Seconds)
	assert.Equal(t, "480p", video.Size)
	assert.Equal(t, map[string]any{"completion_tokens": float64(40594), "total_tokens": float64(40594)}, video.Metadata["usage"])
	assert.NotContains(t, string(body), upstreamURL)
	assert.NotContains(t, string(body), "cgt-20261113180051-x3jui")
	assert.NotContains(t, string(body), "doubao-seedance-2-0-mini-260615")
}

// Under the default passthrough strategy polling stores the upstream URL, and the
// lookup keeps returning it unchanged.
func TestConvertToOpenAIVideoPassthroughKeepsUpstreamURL(t *testing.T) {
	upstreamURL := "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0-mini/v.mp4?X-Tos-Signature=abc"
	task := &model.Task{
		TaskID:      "task_public",
		Status:      model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{ResultURL: upstreamURL},
		Data:        []byte(`{"status":"succeeded","content":{"video_url":"` + upstreamURL + `"}}`),
	}

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(body, &video))
	assert.Equal(t, upstreamURL, video.ResultURL)
	assert.Equal(t, upstreamURL, video.Metadata["url"])
}

func TestConvertToOpenAIVideoReportsFailure(t *testing.T) {
	tests := []struct {
		name        string
		task        *model.Task
		wantCode    string
		wantMessage string
	}{
		{
			name: "upstream error",
			task: &model.Task{Status: model.TaskStatusFailure, FailReason: "sensitive",
				Data: []byte(`{"status":"failed","error":{"code":"OutputVideoSensitiveContentDetected","message":"sensitive"}}`)},
			wantCode: "OutputVideoSensitiveContentDetected", wantMessage: "sensitive",
		},
		{
			name:     "expired without error object",
			task:     &model.Task{Status: model.TaskStatusFailure, FailReason: "task expired", Data: []byte(`{"status":"expired","error":null}`)},
			wantCode: "upstream_error", wantMessage: "task expired",
		},
		{
			name:     "no upstream data",
			task:     &model.Task{Status: model.TaskStatusFailure},
			wantCode: "upstream_error", wantMessage: "task failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(tc.task)
			require.NoError(t, err)
			var video dto.OpenAIVideo
			require.NoError(t, common.Unmarshal(body, &video))
			assert.Equal(t, dto.VideoStatusFailed, video.Status)
			require.NotNil(t, video.Error)
			assert.Equal(t, tc.wantCode, video.Error.Code)
			assert.Equal(t, tc.wantMessage, video.Error.Message)
			assert.Empty(t, video.ResultURL)
		})
	}
}
