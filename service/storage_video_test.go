package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

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

	publicURL, err := cacheRemoteVideoToR2(context.Background(), "https://vidgen.x.ai/video.mp4", client, uploader)
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
			ContentLength: maxR2VideoUploadBytes + 1,
			Body:          io.NopCloser(bytes.NewReader(nil)),
			Header:        make(http.Header),
		}, nil
	})}

	_, err := cacheRemoteVideoToR2(context.Background(), "https://vidgen.x.ai/video.mp4", client, &recordingR2Uploader{})
	require.ErrorContains(t, err, "exceeds")
}
