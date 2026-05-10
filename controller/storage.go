package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type presignRequest struct {
	Filename    string `json:"filename" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	Size        int64  `json:"size,omitempty"` // Optional: file size in bytes for validation
}

func GetPresignedURL(c *gin.Context) {
	var req presignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate content type (allow images and videos)
	if !strings.HasPrefix(req.ContentType, "image/") &&
	   !strings.HasPrefix(req.ContentType, "video/") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "只支持图片或视频类型 (image/*, video/*)",
		})
		return
	}

	// Validate file size if provided
	if req.Size > 0 {
		maxSize := int64(service.AsyncImageMaxURLSizeMB * 1024 * 1024)
		if req.Size > maxSize {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("文件大小 %.2f MB 超过限制 %d MB",
					float64(req.Size)/1024/1024, service.AsyncImageMaxURLSizeMB),
			})
			return
		}
	}

	result, err := service.GeneratePresignedUploadURL(req.Filename, req.ContentType, req.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate upload URL"})
		return
	}

	c.JSON(http.StatusOK, result)
}
