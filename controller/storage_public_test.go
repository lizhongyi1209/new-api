package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicR2PresignAcceptsAllowedSignedDownstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	clients := []setting.R2PublicUploadClient{{
		ID: "client-a", Name: "Client A", Origins: []string{"https://allowed.example.com"},
		Secret: "test-signing-secret", Enabled: true, MaxFileSizeMB: 100, RequestsPerMinute: 10,
	}}
	raw, err := setting.R2PublicUploadClientsToJSON(clients)
	require.NoError(t, err)
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap[setting.R2PublicUploadClientsOptionKey]
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[setting.R2PublicUploadClientsOptionKey] = raw
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap[setting.R2PublicUploadClientsOptionKey] = previous
		common.OptionMapRWMutex.Unlock()
	})

	previousGenerator := generatePublicR2PresignedUploadURL
	generatePublicR2PresignedUploadURL = func(clientID, filename, contentType string, size int64) (*service.PresignResult, error) {
		assert.Equal(t, "client-a", clientID)
		assert.Equal(t, "reference.png", filename)
		assert.Equal(t, "image/png", contentType)
		assert.EqualValues(t, 1024, size)
		return &service.PresignResult{UploadURL: "https://upload.example.com", PublicURL: "https://cdn.example.com/reference.png", ExpiresAt: 123}, nil
	}
	t.Cleanup(func() { generatePublicR2PresignedUploadURL = previousGenerator })

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	reqBody := publicR2PresignRequest{Filename: "reference.png", ContentType: "image/png", Size: 1024}
	nonce := "0123456789abcdef"
	mac := hmac.New(sha256.New, []byte("test-signing-secret"))
	_, _ = mac.Write([]byte(publicR2UploadSignaturePayload("client-a", "https://allowed.example.com", timestamp, nonce, reqBody)))

	request := httptest.NewRequest(http.MethodPost, "/v1/storage/public/presign", bytes.NewBufferString(`{"filename":"reference.png","content_type":"image/png","size":1024}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://allowed.example.com")
	request.Header.Set("X-Downstream-ID", "client-a")
	request.Header.Set("X-Timestamp", timestamp)
	request.Header.Set("X-Nonce", nonce)
	request.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request

	PublicR2Presign(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"upload_url":"https://upload.example.com","public_url":"https://cdn.example.com/reference.png","expires_at":123,"headers":{"Content-Type":"image/png","Content-Length":"1024"}}`, recorder.Body.String())
}

func TestPublicR2PresignRejectsUnlistedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodPost, "/v1/storage/public/presign", bytes.NewBufferString(`{"filename":"reference.png","content_type":"image/png","size":1024}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://attacker.example.com")
	request.Header.Set("X-Downstream-ID", "missing")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request

	PublicR2Presign(context)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
}
