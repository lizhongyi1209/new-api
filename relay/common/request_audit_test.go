package common

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func TestCaptureClientRequestSnapshotRedactsBase64SecretsAndLargeValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","n":0,"prompt":"draw a red fox","image":"data:image/png;base64,`+strings.Repeat("A", 256)+`","api_key":"secret","messages":[{"role":"user","content":"hello"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	snapshot := CaptureClientRequestSnapshot(c)
	require.NotNil(t, snapshot)
	require.NotEmpty(t, snapshot.Fields)

	fields := make(map[string]ClientRequestField, len(snapshot.Fields))
	for _, field := range snapshot.Fields {
		fields[field.Path] = field
	}
	assert.Equal(t, "gpt-image-2", fields["model"].Value)
	assert.Equal(t, "0", fields["n"].Value)
	assert.Equal(t, "draw a red fox", fields["prompt"].Value)
	assert.Equal(t, "base64_payload", fields["image"].OmittedReason)
	assert.Equal(t, "sensitive", fields["api_key"].OmittedReason)
	assert.Equal(t, "hello", fields["messages[0].content"].Value)
	encoded, err := rootcommon.Marshal(snapshot)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), strings.Repeat("A", 64))
}
