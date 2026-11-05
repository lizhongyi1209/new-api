package openai

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMarkupEditContext builds an image-edit multipart request carrying an image,
// a mask and a markup annotation layer.
func newMarkupEditContext(t *testing.T, model string) *gin.Context {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", model))
	require.NoError(t, writer.WriteField("prompt", "recolor the vase"))
	require.NoError(t, writer.WriteField("complexity", "high"))
	require.NoError(t, writer.WriteField("frame_count", "9"))
	require.NoError(t, writer.WriteField("input_fidelity", "high"))
	require.NoError(t, writer.WriteField("safety_identifier", "user-hash"))
	for _, name := range []string{"image", "mask", "markup"} {
		part, err := writer.CreateFormFile(name, name+".png")
		require.NoError(t, err)
		_, err = part.Write([]byte(name + " bytes"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return c
}

func replayMultipart(t *testing.T, c *gin.Context, converted *bytes.Buffer) *http.Request {
	t.Helper()
	replayed := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(converted.Bytes()))
	replayed.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	require.NoError(t, replayed.ParseMultipartForm(32<<20))
	return replayed
}

func TestConvertImageEditRequestForwardsMarkupLayer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const model = "gpt-image-2.6-flare"

	c := newMarkupEditContext(t, model)
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesEdits,
		RequestURLPath:  "/v1/images/edits",
		OriginModelName: model,
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: model},
	}
	request := dto.ImageRequest{Model: model, Prompt: "recolor the vase"}

	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
	require.NoError(t, err)
	body, ok := converted.(*bytes.Buffer)
	require.True(t, ok)

	replayed := replayMultipart(t, c, body)
	for _, name := range []string{"image", "mask", "markup"} {
		files := replayed.MultipartForm.File[name]
		require.Len(t, files, 1, "field %s", name)
		file, err := files[0].Open()
		require.NoError(t, err)
		content, err := io.ReadAll(file)
		require.NoError(t, err)
		require.NoError(t, file.Close())
		require.Equal(t, name+" bytes", string(content))
	}
	// The 2.6 series keeps both scalars: markup guidance and per-frame timing
	// reach the provider untouched.
	require.Equal(t, "high", replayed.PostForm.Get("complexity"))
	require.Equal(t, "9", replayed.PostForm.Get("frame_count"))
}

func TestConvertImageEditRequestStripsProviderRestrictedFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name              string
		model             string
		allowSafetyID     bool
		wantInputFidelity bool
		wantSafetyID      bool
	}{
		// gpt-image-2 always runs image input at high fidelity and rejects an
		// explicit value; safety_identifier is opt-in per channel.
		{name: "gpt-image-2 drops both", model: "gpt-image-2"},
		{name: "gpt-image-2 keeps safety id when the channel allows it", model: "gpt-image-2", allowSafetyID: true, wantSafetyID: true},
		// The 2.5 and 2.6 series accept an explicit input_fidelity.
		{name: "gpt-image-2.6 keeps input fidelity", model: "gpt-image-2.6-flare", wantInputFidelity: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newMarkupEditContext(t, tc.model)
			info := &relaycommon.RelayInfo{
				RelayMode:       relayconstant.RelayModeImagesEdits,
				RequestURLPath:  "/v1/images/edits",
				OriginModelName: tc.model,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName:    tc.model,
					ChannelOtherSettings: dto.ChannelOtherSettings{AllowSafetyIdentifier: tc.allowSafetyID},
				},
			}
			request := dto.ImageRequest{Model: tc.model, Prompt: "recolor the vase"}

			converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
			require.NoError(t, err)
			body, ok := converted.(*bytes.Buffer)
			require.True(t, ok)

			replayed := replayMultipart(t, c, body)
			assert.Equal(t, tc.wantInputFidelity, replayed.PostForm.Has("input_fidelity"))
			assert.Equal(t, tc.wantSafetyID, replayed.PostForm.Has("safety_identifier"))
		})
	}
}

// TestConvertImageRequestStripsRestrictedJSONFields covers the JSON generations
// path, where the same two scalars travel as struct fields instead of form parts.
func TestConvertImageRequestStripsRestrictedJSONFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name              string
		model             string
		allowSafetyID     bool
		wantInputFidelity bool
		wantSafetyID      bool
	}{
		{name: "gpt-image-2 drops both", model: "gpt-image-2"},
		{name: "gpt-image-2 keeps safety id when allowed", model: "gpt-image-2", allowSafetyID: true, wantSafetyID: true},
		{name: "gpt-image-2.5 keeps input fidelity", model: "gpt-image-2.5-flare", wantInputFidelity: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			info := &relaycommon.RelayInfo{
				RelayMode:       relayconstant.RelayModeImagesGenerations,
				RequestURLPath:  "/v1/images/generations",
				OriginModelName: tc.model,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName:    tc.model,
					ChannelOtherSettings: dto.ChannelOtherSettings{AllowSafetyIdentifier: tc.allowSafetyID},
				},
			}
			fidelity := "high"
			fidelityRaw, err := common.Marshal(fidelity)
			require.NoError(t, err)
			safetyRaw, err := common.Marshal("user-hash")
			require.NoError(t, err)
			request := dto.ImageRequest{
				Model:            tc.model,
				Prompt:           "a vase",
				InputFidelity:    fidelityRaw,
				SafetyIdentifier: safetyRaw,
			}

			converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
			require.NoError(t, err)
			result, ok := converted.(dto.ImageRequest)
			require.True(t, ok)
			assert.Equal(t, tc.wantInputFidelity, len(result.InputFidelity) > 0)
			assert.Equal(t, tc.wantSafetyID, len(result.SafetyIdentifier) > 0)
		})
	}
}
