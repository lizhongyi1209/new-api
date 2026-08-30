package openai

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertImageEditRequestMultipart verifies that ConvertImageRequest
// re-serializes multipart image edit requests with all fields (including
// stream) and the file intact, both when the form was already parsed and when
// it must be re-parsed from the reusable body.
func TestConvertImageEditRequestMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newMultipartContext := func(t *testing.T, model string, prompt string) *gin.Context {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", model))
		require.NoError(t, writer.WriteField("prompt", prompt))
		require.NoError(t, writer.WriteField("stream", "true"))
		require.NoError(t, writer.WriteField("partial_images", "3"))
		require.NoError(t, writer.WriteField("response_format", "b64_json"))
		require.NoError(t, writer.WriteField("quality", "low"))
		require.NoError(t, writer.WriteField("moderation", "low"))
		require.NoError(t, writer.WriteField("size", "1536x1024"))
		require.NoError(t, writer.WriteField("tag", "first"))
		require.NoError(t, writer.WriteField("tag", "second"))
		require.NoError(t, writer.WriteField("image_reference", "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB"))
		part, err := writer.CreateFormFile("image", "input.png")
		require.NoError(t, err)
		_, err = part.Write([]byte("fake image"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return c
	}

	convertAndReplay := func(t *testing.T, c *gin.Context, model string, prompt string, wantResponseFormat bool) {
		info := &relaycommon.RelayInfo{
			RelayMode:       relayconstant.RelayModeImagesEdits,
			RequestURLPath:  "/v1/images/edits",
			OriginModelName: model,
			ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: model},
		}
		request := dto.ImageRequest{
			Model:          model,
			Prompt:         prompt,
			Stream:         common.GetPointer(true),
			ResponseFormat: "b64_json",
		}

		converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
		require.NoError(t, err)
		convertedBody, ok := converted.(*bytes.Buffer)
		require.True(t, ok)

		replayedRequest := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(convertedBody.Bytes()))
		replayedRequest.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
		require.NoError(t, replayedRequest.ParseMultipartForm(32<<20))

		require.Equal(t, model, replayedRequest.PostForm.Get("model"))
		require.Equal(t, prompt, replayedRequest.PostForm.Get("prompt"))
		require.Equal(t, "true", replayedRequest.PostForm.Get("stream"))
		require.Equal(t, "3", replayedRequest.PostForm.Get("partial_images"))
		if wantResponseFormat {
			require.Equal(t, "b64_json", replayedRequest.PostForm.Get("response_format"))
		} else {
			require.False(t, replayedRequest.PostForm.Has("response_format"))
		}
		require.Equal(t, "low", replayedRequest.PostForm.Get("quality"))
		require.Equal(t, "low", replayedRequest.PostForm.Get("moderation"))
		require.Equal(t, "1536x1024", replayedRequest.PostForm.Get("size"))
		require.Equal(t, []string{"first", "second"}, replayedRequest.PostForm["tag"])
		require.Len(t, replayedRequest.MultipartForm.File["image"], 1)

		file, err := replayedRequest.MultipartForm.File["image"][0].Open()
		require.NoError(t, err)
		defer file.Close()
		fileBytes, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, []byte("fake image"), fileBytes)

		snapshot := relaycommon.GetUpstreamRequestSnapshot(c)
		require.NotNil(t, snapshot)
		require.Equal(t, http.MethodPost, snapshot.Method)
		require.Equal(t, "/v1/images/edits", snapshot.Path)
		require.Equal(t, int64(convertedBody.Len()), snapshot.ContentLength)
		require.Contains(t, snapshot.ContentType, "multipart/form-data; boundary=")

		values := make(map[string][]string)
		var imagePart *relaycommon.UpstreamRequestPart
		var base64Part *relaycommon.UpstreamRequestPart
		for i := range snapshot.Parts {
			part := &snapshot.Parts[i]
			if part.Kind == "field" && !part.Omitted {
				values[part.Name] = append(values[part.Name], part.Value)
			}
			if part.Kind == "file" && part.Name == "image" {
				imagePart = part
			}
			if part.Name == "image_reference" {
				base64Part = part
			}
		}
		if wantResponseFormat {
			require.Equal(t, []string{"b64_json"}, values["response_format"])
		} else {
			require.NotContains(t, values, "response_format")
		}
		require.Equal(t, []string{"first", "second"}, values["tag"])
		require.NotNil(t, imagePart)
		require.Equal(t, int64(len("fake image")), imagePart.Size)
		require.Equal(t, "input.png", imagePart.Filename)
		require.Equal(t, "image/png", imagePart.ContentType)
		require.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte("fake image"))), imagePart.SHA256)
		require.NotNil(t, base64Part)
		require.Equal(t, "base64_image", base64Part.OmittedReason)
		require.Empty(t, base64Part.Value)

		encodedSnapshot, err := common.Marshal(snapshot)
		require.NoError(t, err)
		require.NotContains(t, string(encodedSnapshot), "data:image/png;base64")
	}

	t.Run("with pre-parsed form", func(t *testing.T) {
		prompt := "edit this image"
		c := newMultipartContext(t, "gpt-image-1", prompt)
		require.NoError(t, c.Request.ParseMultipartForm(32<<20))

		convertAndReplay(t, c, "gpt-image-1", prompt, true)
	})

	t.Run("re-parses reusable body when form is missing", func(t *testing.T) {
		prompt := "edit without pre-parsed form"
		c := newMultipartContext(t, "gpt-image-1", prompt)

		storage, err := common.GetBodyStorage(c)
		require.NoError(t, err)
		c.Request.Body = io.NopCloser(storage)
		c.Request.MultipartForm = nil
		c.Request.PostForm = nil

		convertAndReplay(t, c, "gpt-image-1", prompt, true)
	})

	t.Run("drops response format for gpt image 2", func(t *testing.T) {
		prompt := "edit with ignored response format"
		c := newMultipartContext(t, "gpt-image-2-c", prompt)
		require.NoError(t, c.Request.ParseMultipartForm(32<<20))

		convertAndReplay(t, c, "gpt-image-2-c", prompt, false)
	})
}

func TestConvertImageJSONDropsResponseFormatForGPTImage2(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		OriginModelName: "gpt-image-2-c",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-image-2"},
	}
	moderation, err := common.Marshal("low")
	require.NoError(t, err)
	request := dto.ImageRequest{
		Model:          "gpt-image-2",
		Prompt:         "draw a test image",
		ResponseFormat: "invalid-client-value",
		Moderation:     moderation,
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
	require.NoError(t, err)
	convertedRequest, ok := converted.(dto.ImageRequest)
	require.True(t, ok)
	assert.Empty(t, convertedRequest.ResponseFormat)
	assert.JSONEq(t, `"low"`, string(convertedRequest.Moderation))
}
