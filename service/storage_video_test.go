package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type videoRoundTripper func(*http.Request) (*http.Response, error)

func (fn videoRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type recordingR2Uploader struct {
	input *s3.PutObjectInput
	body  []byte
}

func (uploader *recordingR2Uploader) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	uploader.input = input
	var err error
	uploader.body, err = io.ReadAll(input.Body)
	return &s3.PutObjectOutput{}, err
}

func TestCacheRemoteVideoToR2UploadsUnderOutputPrefix(t *testing.T) {
	t.Setenv("R2_BUCKET", "api-o1key")
	t.Setenv("R2_PUBLIC_BASE_URL", "https://assetcache.o1key.com/")

	videoBytes := []byte("fake mp4 content")
	client := &http.Client{Transport: videoRoundTripper(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, "https://vidgen.x.ai/video.mp4", request.URL.String())
		return &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: int64(len(videoBytes)),
			Body:          io.NopCloser(bytes.NewReader(videoBytes)),
			Header:        make(http.Header),
		}, nil
	})}
	uploader := &recordingR2Uploader{}

	publicURL, err := cacheVideoOutput(
		context.Background(),
		"https://vidgen.x.ai/video.mp4",
		client,
		nil,
		uploader,
		"api-o1key",
		"https://assetcache.o1key.com",
		"R2",
	)
	require.NoError(t, err)
	require.NotNil(t, uploader.input)
	assert.Equal(t, "api-o1key", *uploader.input.Bucket)
	assert.Regexp(t, `^output/[0-9a-f-]+\.mp4$`, *uploader.input.Key)
	assert.Equal(t, "https://assetcache.o1key.com/"+*uploader.input.Key, publicURL)
	assert.Equal(t, videoBytes, uploader.body)
	assert.Equal(t, "video/mp4", *uploader.input.ContentType)
	assert.Equal(t, int64(len(videoBytes)), *uploader.input.ContentLength)
}

func TestCacheRemoteVideoToR2RejectsOversizedResponse(t *testing.T) {
	t.Setenv("R2_BUCKET", "api-o1key")
	t.Setenv("R2_PUBLIC_BASE_URL", "https://assetcache.o1key.com")

	client := &http.Client{Transport: videoRoundTripper(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: maxVideoOutputBytes + 1,
			Body:          io.NopCloser(bytes.NewReader(nil)),
			Header:        make(http.Header),
		}, nil
	})}

	_, err := cacheVideoOutput(
		context.Background(),
		"https://vidgen.x.ai/video.mp4",
		client,
		nil,
		&recordingR2Uploader{},
		"api-o1key",
		"https://assetcache.o1key.com",
		"R2",
	)
	require.ErrorContains(t, err, "exceeds")
}

func TestApplyVideoOutputStrategyUploadsAnyChannelVideoToOSS(t *testing.T) {
	var requestPath string
	var requestBody []byte
	var requestBodyErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestPath = request.URL.Path
		requestBody, requestBodyErr = io.ReadAll(request.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	previousClient := ossClient
	previousPresignClient := ossPresignClient
	ossClient = nil
	ossPresignClient = nil
	t.Cleanup(func() {
		ossClient = previousClient
		ossPresignClient = previousPresignClient
	})
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_SECRET", "test-secret-key")
	t.Setenv("ALIYUN_OSS_REGION", "test-region")
	t.Setenv("ALIYUN_OSS_ENDPOINT", server.URL)
	t.Setenv("ALIYUN_OSS_FORCE_PATH_STYLE", "true")
	t.Setenv("ALIYUN_OSS_BUCKET", "test-bucket")
	t.Setenv("ALIYUN_OSS_PUBLIC_BASE_URL", "https://media.example.com")

	channel := &model.Channel{Type: constant.ChannelTypeKling}
	channel.SetOtherSettings(dto.ChannelOtherSettings{ImageOutputStrategy: dto.ImageOutputStrategyOSS})
	task := &model.Task{TaskID: "task_video_output"}
	result := &relaycommon.TaskInfo{
		Status: model.TaskStatusSuccess,
		Url:    "data:video/mp4;base64,ZmFrZSBtcDQ=",
	}

	err := ApplyVideoOutputStrategy(context.Background(), channel, task, result)

	require.NoError(t, err)
	require.NoError(t, requestBodyErr)
	assert.True(t, strings.HasPrefix(requestPath, "/test-bucket/output/"), requestPath)
	assert.True(t, strings.HasSuffix(requestPath, ".mp4"), requestPath)
	assert.Equal(t, []byte("fake mp4"), requestBody)
	assert.True(t, strings.HasPrefix(result.Url, "https://media.example.com/output/"), result.Url)
	assert.Empty(t, result.RemoteUrl)
}

func TestApplyVideoOutputStrategyDefaultsToUpstreamPassthrough(t *testing.T) {
	for _, strategy := range []string{"", dto.ImageOutputStrategyPassthrough} {
		t.Run(strategy, func(t *testing.T) {
			channel := &model.Channel{Type: constant.ChannelTypeKling}
			channel.SetOtherSettings(dto.ChannelOtherSettings{ImageOutputStrategy: strategy})
			task := &model.Task{TaskID: "task_video_passthrough"}
			result := &relaycommon.TaskInfo{
				Status: model.TaskStatusSuccess,
				Url:    "https://upstream.example.com/video.mp4",
			}

			err := ApplyVideoOutputStrategy(context.Background(), channel, task, result)

			require.NoError(t, err)
			assert.Equal(t, "https://upstream.example.com/video.mp4", result.Url)
		})
	}
}
