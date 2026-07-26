package controller

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

const publicR2UploadClockSkew = 5 * time.Minute

var (
	publicR2UploadRateLimiter          common.InMemoryRateLimiter
	publicR2UploadNonceMutex           sync.Mutex
	publicR2UploadNonces               = make(map[string]int64)
	generatePublicR2PresignedUploadURL = service.GeneratePublicR2PresignedUploadURL
)

type publicR2PresignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type r2PublicUploadClientView struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Origins           []string `json:"origins"`
	Enabled           bool     `json:"enabled"`
	MaxFileSizeMB     int      `json:"max_file_size_mb"`
	RequestsPerMinute int      `json:"requests_per_minute"`
	HasSecret         bool     `json:"has_secret"`
}

func PublicR2Presign(c *gin.Context) {
	var req publicR2PresignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	req.Filename = strings.TrimSpace(req.Filename)
	req.ContentType = strings.ToLower(strings.TrimSpace(req.ContentType))
	if req.Filename == "" || req.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename and positive size are required"})
		return
	}
	if !strings.HasPrefix(req.ContentType, "image/") && !strings.HasPrefix(req.ContentType, "video/") && !strings.HasPrefix(req.ContentType, "audio/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only image, video, or audio uploads are allowed"})
		return
	}

	clientID := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Downstream-ID")))
	origin, err := setting.NormalizeR2PublicUploadOrigin(c.GetHeader("Origin"))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "origin is not allowed"})
		return
	}
	client, ok := setting.FindR2PublicUploadClient(clientID, origin)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "downstream or origin is not allowed"})
		return
	}
	if req.Size > int64(client.MaxFileSizeMB)*1024*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fmt.Sprintf("file exceeds the %d MB downstream limit", client.MaxFileSizeMB)})
		return
	}

	timestampText := strings.TrimSpace(c.GetHeader("X-Timestamp"))
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil || time.Since(time.Unix(timestamp, 0)) > publicR2UploadClockSkew || time.Until(time.Unix(timestamp, 0)) > publicR2UploadClockSkew {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "request timestamp is invalid or expired"})
		return
	}
	nonce := strings.TrimSpace(c.GetHeader("X-Nonce"))
	providedSignature, err := hex.DecodeString(strings.TrimSpace(c.GetHeader("X-Signature")))
	if err != nil || len(nonce) < 16 || len(nonce) > 128 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid upload signature"})
		return
	}
	canonical := publicR2UploadSignaturePayload(clientID, origin, timestampText, nonce, req)
	mac := hmac.New(sha256.New, []byte(client.Secret))
	_, _ = mac.Write([]byte(canonical))
	if !hmac.Equal(providedSignature, mac.Sum(nil)) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid upload signature"})
		return
	}
	if !consumePublicR2UploadNonce(clientID, nonce) {
		c.JSON(http.StatusConflict, gin.H{"error": "upload request has already been used"})
		return
	}
	if !allowPublicR2UploadRequest(clientID, c.ClientIP(), client.RequestsPerMinute) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "upload signing rate limit exceeded"})
		return
	}

	filename := path.Base(strings.ReplaceAll(req.Filename, `\`, "/"))
	result, err := generatePublicR2PresignedUploadURL(clientID, filename, req.ContentType, req.Size)
	if err != nil {
		common.SysError("failed to generate public R2 upload URL: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate upload URL"})
		return
	}
	c.Header("Access-Control-Allow-Origin", origin)
	c.Header("Vary", "Origin")
	c.JSON(http.StatusOK, gin.H{
		"upload_url": result.UploadURL,
		"public_url": result.PublicURL,
		"expires_at": result.ExpiresAt,
		"headers":    gin.H{"Content-Type": req.ContentType, "Content-Length": strconv.FormatInt(req.Size, 10)},
	})
}

func publicR2UploadSignaturePayload(clientID, origin, timestamp, nonce string, req publicR2PresignRequest) string {
	return strings.Join([]string{clientID, origin, timestamp, nonce, req.Filename, req.ContentType, strconv.FormatInt(req.Size, 10)}, "\n")
}

func consumePublicR2UploadNonce(clientID, nonce string) bool {
	key := "r2-public-upload:nonce:" + clientID + ":" + nonce
	if common.RedisEnabled {
		accepted, err := common.RDB.SetNX(context.Background(), key, "1", 10*time.Minute).Result()
		return err == nil && accepted
	}
	publicR2UploadNonceMutex.Lock()
	defer publicR2UploadNonceMutex.Unlock()
	now := time.Now().Unix()
	for storedKey, expiresAt := range publicR2UploadNonces {
		if expiresAt <= now {
			delete(publicR2UploadNonces, storedKey)
		}
	}
	if _, exists := publicR2UploadNonces[key]; exists {
		return false
	}
	publicR2UploadNonces[key] = now + int64((10 * time.Minute).Seconds())
	return true
}

func allowPublicR2UploadRequest(clientID, ip string, limit int) bool {
	key := "r2-public-upload:rate:" + clientID + ":" + ip + ":" + time.Now().UTC().Format("200601021504")
	if common.RedisEnabled {
		ctx := context.Background()
		count, err := common.RDB.Incr(ctx, key).Result()
		if err != nil {
			return false
		}
		if count == 1 {
			common.RDB.Expire(ctx, key, 2*time.Minute)
		}
		return count <= int64(limit)
	}
	publicR2UploadRateLimiter.Init(10 * time.Minute)
	return publicR2UploadRateLimiter.Request(key, limit, 60)
}

func GetR2PublicUploadClients(c *gin.Context) {
	clients := setting.GetR2PublicUploadClients()
	views := make([]r2PublicUploadClientView, 0, len(clients))
	for _, client := range clients {
		views = append(views, r2PublicUploadClientView{
			ID: client.ID, Name: client.Name, Origins: client.Origins, Enabled: client.Enabled,
			MaxFileSizeMB: client.MaxFileSizeMB, RequestsPerMinute: client.RequestsPerMinute, HasSecret: client.Secret != "",
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": views})
}

func UpdateR2PublicUploadClients(c *gin.Context) {
	var requested []r2PublicUploadClientView
	if err := c.ShouldBindJSON(&requested); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid downstream configuration"})
		return
	}
	existingSecrets := make(map[string]string)
	for _, client := range setting.GetR2PublicUploadClients() {
		existingSecrets[client.ID] = client.Secret
	}
	clients := make([]setting.R2PublicUploadClient, 0, len(requested))
	for _, view := range requested {
		secret := existingSecrets[view.ID]
		if secret == "" {
			var err error
			secret, err = common.GenerateRandomKey(48)
			if err != nil {
				common.ApiError(c, err)
				return
			}
		}
		clients = append(clients, setting.R2PublicUploadClient{
			ID: view.ID, Name: view.Name, Origins: view.Origins, Secret: secret, Enabled: view.Enabled,
			MaxFileSizeMB: view.MaxFileSizeMB, RequestsPerMinute: view.RequestsPerMinute,
		})
	}
	if err := setting.ValidateR2PublicUploadClients(clients); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	value, err := setting.R2PublicUploadClientsToJSON(clients)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption(setting.R2PublicUploadClientsOptionKey, value); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "R2 public upload whitelist updated"})
}

func RotateR2PublicUploadClientSecret(c *gin.Context) {
	clientID := strings.ToLower(strings.TrimSpace(c.Param("client_id")))
	clients := setting.GetR2PublicUploadClients()
	secret, err := common.GenerateRandomKey(48)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	found := false
	for index := range clients {
		if clients[index].ID == clientID {
			clients[index].Secret = secret
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "downstream client not found"})
		return
	}
	value, err := setting.R2PublicUploadClientsToJSON(clients)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption(setting.R2PublicUploadClientsOptionKey, value); err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"secret": secret}})
}
