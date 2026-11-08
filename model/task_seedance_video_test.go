package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	seedanceProviderURL = "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/output/video.mp4?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Signature=abc"
	seedanceMirroredURL = "https://oss.o1key.cn/output/8f0c0b1e-1111-2222-3333-444455556666.mp4"
)

func seedancePlatform() constant.TaskPlatform {
	return constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeServiceInferenceVideo))
}

func TestGetResultURLPrefersSeedanceProviderURLOverMirroredResult(t *testing.T) {
	task := &Task{
		Platform: seedancePlatform(),
		Properties: Properties{
			OriginModelName:   "dreamina-seedance-2-0-hc",
			UpstreamModelName: "dreamina-seedance-2-0-260128-max",
		},
		PrivateData: TaskPrivateData{ResultURL: seedanceMirroredURL},
		Data: []byte(`{
			"task": {
				"id": "mvt-upstream",
				"status": "completed",
				"model": "dreamina-seedance-2-0-260128-max",
				"outputs": ["` + seedanceProviderURL + `"],
				"metadata": {
					"content": {"video_url": "` + seedanceProviderURL + `"}
				}
			}
		}`),
	}

	assert.Equal(t, seedanceProviderURL, task.GetResultURL())
}

func TestGetResultURLReadsSeedanceVideoURLShapes(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string
	}{
		{
			name: "metadata content video_url string",
			data: `{"task":{"model":"doubao-seedance-2-0-260128-max","metadata":{"content":{"video_url":"` + seedanceProviderURL + `"}}}}`,
			want: seedanceProviderURL,
		},
		{
			name: "metadata content video_url object",
			data: `{"task":{"metadata":{"model":"dreamina-seedance-2-5-260628-max","content":{"video_url":{"url":"` + seedanceProviderURL + `"}}}}}`,
			want: seedanceProviderURL,
		},
		{
			name: "metadata content url",
			data: `{"task":{"model":"dreamina-seedance-2-0-260128","metadata":{"content":{"url":"` + seedanceProviderURL + `"}}}}`,
			want: seedanceProviderURL,
		},
		{
			name: "top-level content video_url",
			data: `{"model":"doubao-seedance-2-0-fast-260128-max","content":{"video_url":"` + seedanceProviderURL + `"}}`,
			want: seedanceProviderURL,
		},
		{
			name: "metadata video url",
			data: `{"task":{"model":"dreamina-seedance-2-0-hc","metadata":{"video":{"url":"` + seedanceProviderURL + `"}}}}`,
			want: seedanceProviderURL,
		},
		{
			name: "output object url",
			data: `{"task":{"model":"dreamina-seedance-2-0-260128-max","outputs":[{"url":"` + seedanceProviderURL + `"}]}}`,
			want: seedanceProviderURL,
		},
		{
			name: "outputs win over metadata video_url",
			data: `{"task":{"model":"dreamina-seedance-2-0-hc","outputs":["https://provider.example/from-outputs.mp4"],"metadata":{"content":{"video_url":"https://provider.example/from-metadata.mp4"}}}}`,
			want: "https://provider.example/from-outputs.mp4",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &Task{
				Platform:    seedancePlatform(),
				PrivateData: TaskPrivateData{ResultURL: seedanceMirroredURL},
				Data:        []byte(tc.data),
			}
			assert.Equal(t, tc.want, task.GetResultURL())
		})
	}
}

func TestGetResultURLKeepsMirroredSeedanceURLWhenProviderURLIsGone(t *testing.T) {
	task := &Task{
		Platform: seedancePlatform(),
		Properties: Properties{
			OriginModelName: "dreamina-seedance-2-0-hc",
		},
		PrivateData: TaskPrivateData{ResultURL: seedanceMirroredURL},
		Data:        []byte(`{"task":{"id":"mvt-old","status":"completed","model":"dreamina-seedance-2-0-260128-max","outputs":[]}}`),
	}

	assert.Equal(t, seedanceMirroredURL, task.GetResultURL())
}

func TestGetResultURLDoesNotReplaceOtherPlatformMirrors(t *testing.T) {
	grok := &Task{
		Platform: seedancePlatform(),
		Properties: Properties{
			OriginModelName: "grok-imagine-video-1.5",
		},
		PrivateData: TaskPrivateData{ResultURL: "https://assetcache.o1key.com/output/cached-video.mp4"},
		Data:        []byte(`{"task":{"model":"grok-imagine-video-1.5","outputs":["https://upstream.example/video.mp4"]}}`),
	}
	kling := &Task{
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeKling)),
		Properties: Properties{
			OriginModelName: "dreamina-seedance-2-0-hc",
		},
		PrivateData: TaskPrivateData{ResultURL: seedanceMirroredURL},
		Data:        []byte(`{"task":{"outputs":["` + seedanceProviderURL + `"]}}`),
	}
	minimax := &Task{
		Platform: seedancePlatform(),
		Properties: Properties{
			OriginModelName:   "MiniMax-H3",
			UpstreamModelName: "minimax-h3",
		},
		PrivateData: TaskPrivateData{ResultURL: seedanceMirroredURL},
		Data:        []byte(`{"task":{"model":"minimax-h3","metadata":{"content":{"url":"https://cdn.example.com/output.mp4"}}}}`),
	}

	assert.Equal(t, "https://assetcache.o1key.com/output/cached-video.mp4", grok.GetResultURL())
	assert.Equal(t, seedanceMirroredURL, kling.GetResultURL())
	assert.Equal(t, seedanceMirroredURL, minimax.GetResultURL())
	require.False(t, minimax.PassesThroughSeedanceVideo())
	require.True(t, (&Task{
		Platform:   seedancePlatform(),
		Properties: Properties{OriginModelName: "public-seedance-2-5"},
	}).PassesThroughSeedanceVideo())
}
