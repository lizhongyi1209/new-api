package controller

import (
	"fmt"
	"net/http"
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
