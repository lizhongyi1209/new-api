package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxVideoUploadSizeMB = 100

type presignRequest struct {
	Filename    string `json:"filename" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	Size        int64  `json:"size,omitempty"` // Optional: file size in bytes for validation
}

func GetPresignedURL(c *gin.Context) {
	var req presignRequest
	if !bindAndValidatePresignRequest(c, &req) {
		return
	}

	result, err := service.GeneratePresignedUploadURLForHost(c.Request.Host, req.Filename, req.ContentType, req.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate upload URL"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func GetOSSPresignedURL(c *gin.Context) {
	var req presignRequest
	if !bindAndValidatePresignRequest(c, &req) {
		return
	}

	result, err := service.GenerateOSSPresignedUploadURL(req.Filename, req.ContentType, req.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate OSS upload URL"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func bindAndValidatePresignRequest(c *gin.Context, req *presignRequest) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}

	contentType := strings.ToLower(strings.TrimSpace(req.ContentType))
	if !strings.HasPrefix(contentType, "image/") &&
		!strings.HasPrefix(contentType, "video/") &&
		!strings.HasPrefix(contentType, "audio/") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "只支持图片、视频或音频类型 (image/*, video/*, audio/*)",
		})
		return false
	}

	if req.Size > 0 {
		maxSizeMB := service.AsyncImageMaxURLSizeMB
		if strings.HasPrefix(contentType, "video/") {
			maxSizeMB = maxVideoUploadSizeMB
		}
		maxSize := int64(maxSizeMB * 1024 * 1024)
		if req.Size > maxSize {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("文件大小 %.2f MB 超过限制 %d MB",
					float64(req.Size)/1024/1024, maxSizeMB),
			})
			return false
		}
	}

	return true
}

// UploadLocalFile handles direct file upload to local storage
func UploadLocalFile(c *gin.Context) {
	objectKey := c.Query("object_key")
	if objectKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "object_key is required"})
		return
	}

	// Security: validate object_key format
	if !isValidObjectKey(objectKey) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid object_key format"})
		return
	}

	uploadDir := getUploadDir()
	filePath := filepath.Join(uploadDir, objectKey)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
		return
	}

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Check file size
	contentType := c.GetHeader("Content-Type")
	maxSizeMB := service.AsyncImageMaxURLSizeMB
	if strings.HasPrefix(strings.ToLower(contentType), "video/") {
		maxSizeMB = maxVideoUploadSizeMB
	}
	maxSize := int64(maxSizeMB * 1024 * 1024)
	if int64(len(body)) > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("文件大小 %.2f MB 超过限制 %d MB",
				float64(len(body))/1024/1024, maxSizeMB),
		})
		return
	}

	// Write file
	if err := os.WriteFile(filePath, body, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	localPublicBase := strings.TrimRight(getLocalPublicBase(), "/")
	publicURL := fmt.Sprintf("%s/upload/%s", localPublicBase, objectKey)

	c.JSON(http.StatusOK, gin.H{
		"public_url":  publicURL,
		"object_key":  objectKey,
		"size":        len(body),
		"content_type": contentType,
	})
}

func isValidObjectKey(objectKey string) bool {
	// Must start with "uploads/" and contain only safe characters
	if !strings.HasPrefix(objectKey, "uploads/") {
		return false
	}
	// No directory traversal
	if strings.Contains(objectKey, "..") {
		return false
	}
	// Must have a filename after uploads/
	parts := strings.Split(objectKey, "/")
	if len(parts) < 2 || parts[len(parts)-1] == "" {
		return false
	}
	return true
}

func getUploadDir() string {
	if dir := os.Getenv("LOCAL_UPLOAD_DIR"); dir != "" {
		return dir
	}
	return "uploads"
}

func getLocalPublicBase() string {
	if base := os.Getenv("LOCAL_PUBLIC_BASE_URL"); base != "" {
		return base
	}
	return "https://api.o1key.cn"
}
