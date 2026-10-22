package common

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpstreamMultipartRequestSnapshotOmitsMediaAndSecrets(t *testing.T) {
	snapshot := NewUpstreamMultipartRequestSnapshot(http.MethodPost, "/v1/images/edits")
	base64Image := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB"

	snapshot.AddField("model", "gpt-image-2")
	snapshot.AddField("response_format", "b64_json")
	snapshot.AddField("image", base64Image)
	snapshot.AddField("api_key", "sk-secret")
	snapshot.AddFile("mask", "mask.png", "image/png", 12, strings.Repeat("a", 64))

	require.Len(t, snapshot.Parts, 5)
	require.Equal(t, "b64_json", snapshot.Parts[1].Value)
	require.Equal(t, "base64_image", snapshot.Parts[2].OmittedReason)
	require.Equal(t, len(base64Image), snapshot.Parts[2].OriginalBytes)
	require.Len(t, snapshot.Parts[2].SHA256, 64)
	require.Equal(t, "sensitive", snapshot.Parts[3].OmittedReason)
	require.Empty(t, snapshot.Parts[3].Value)
	require.Equal(t, "binary_media", snapshot.Parts[4].OmittedReason)
	require.Equal(t, int64(12), snapshot.Parts[4].Size)

	encoded, err := rootcommon.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), base64Image)
	require.NotContains(t, string(encoded), "sk-secret")
	require.Contains(t, string(encoded), `"name":"response_format","value":"b64_json"`)
}

func TestUpstreamJSONRequestSnapshotKeepsSafeImageScalars(t *testing.T) {
	snapshot := NewUpstreamJSONRequestSnapshot(http.MethodPost, "/v1/images/generations", 256)
	body := []byte(`{"model":"gpt-image-2.5-flare","prompt":"private prompt","n":0,"size":"1024x1024","quality":"auto","parameters":{"n":3,"prompt_extend":true},"images":["data:image/png;base64,AAAA"],"api_key":"sk-secret","unknown":"private"}`)
	require.NoError(t, snapshot.AddJSONFields(body))

	values := make(map[string]string)
	for _, part := range snapshot.Parts {
		if !part.Omitted {
			values[part.Name] = part.Value
		}
	}
	require.Equal(t, "gpt-image-2.5-flare", values["model"])
	require.Equal(t, "0", values["n"])
	require.Equal(t, "1024x1024", values["size"])
	require.Equal(t, "auto", values["quality"])
	require.Equal(t, "3", values["parameters.n"])
	require.Equal(t, "true", values["parameters.prompt_extend"])
	encoded, err := rootcommon.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private prompt")
	require.NotContains(t, string(encoded), "data:image/png;base64")
	require.NotContains(t, string(encoded), "sk-secret")
	for _, name := range []string{"prompt", "images", "api_key", "unknown"} {
		require.Contains(t, string(encoded), `"name":"`+name+`"`)
	}
}

func TestSetUpstreamResponseSnapshotSanitizesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := []byte(`{"error":{"message":"invalid response_format","param":"response_format"},"authorization":"secret","image":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB"}`)

	SetUpstreamResponseSnapshot(c, http.StatusBadRequest, "application/json", body)

	snapshot := GetUpstreamResponseSnapshot(c)
	require.NotNil(t, snapshot)
	require.Equal(t, http.StatusBadRequest, snapshot.StatusCode)
	encoded, err := rootcommon.Marshal(snapshot)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "response_format")
	require.Contains(t, string(encoded), "[redacted]")
	require.Contains(t, string(encoded), "[omitted ")
	require.NotContains(t, string(encoded), "data:image/png;base64")
	require.NotContains(t, string(encoded), `"authorization":"secret"`)
}

func TestUpstreamResponseCaptureStoresConsumedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := `{"error":{"message":"invalid response_format","param":"response_format"}}`
	capture := NewUpstreamResponseCapture(io.NopCloser(strings.NewReader(body)))

	consumed, err := io.ReadAll(capture)
	require.NoError(t, err)
	require.Equal(t, body, string(consumed))
	require.NoError(t, capture.Close())
	capture.StoreSnapshot(c, http.StatusOK, "application/json")

	snapshot := GetUpstreamResponseSnapshot(c)
	require.NotNil(t, snapshot)
	require.Equal(t, len(body), snapshot.OriginalBytes)
	require.False(t, snapshot.Truncated)
	encoded, err := rootcommon.Marshal(snapshot.Body)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "response_format")
}
