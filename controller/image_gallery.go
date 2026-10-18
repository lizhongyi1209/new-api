package controller

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type imageGalleryItem struct {
	TaskID  string                         `json:"task_id"`
	Model   string                         `json:"model"`
	Prompt  string                         `json:"prompt"`
	Options *model.TaskImageGalleryOptions `json:"options,omitempty"`
	Created int64                          `json:"created"`
	Images  []model.TaskImageGalleryResult `json:"images"`
}

// ImageGenerationHistory returns only the current user's durable image URLs.
// Task data is deliberately omitted: it may contain large legacy Base64 bodies.
func ImageGenerationHistory(c *gin.Context) {
	beforeID := int64(0)
	if raw := c.Query("before_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid before_id"})
			return
		}
		beforeID = parsed
	}

	const pageSize = 24
	const scanSize = 100
	items := make([]imageGalleryItem, 0, pageSize)
	for {
		tasks, err := model.ListImageGalleryCandidates(c.GetInt("id"), beforeID, scanSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load image history"})
			return
		}
		for _, task := range tasks {
			beforeID = task.ID
			if !task.PrivateData.ImageGallery || len(task.PrivateData.ImageGalleryResults) == 0 {
				continue
			}
			hasImage := false
			for _, image := range task.PrivateData.ImageGalleryResults {
				if image.URL != "" {
					hasImage = true
					break
				}
			}
			if !hasImage {
				continue
			}
			items = append(items, imageGalleryItem{
				TaskID:  task.TaskID,
				Model:   task.Properties.OriginModelName,
				Prompt:  task.Properties.Input,
				Options: task.PrivateData.ImageGalleryOptions,
				Created: task.FinishTime,
				Images:  task.PrivateData.ImageGalleryResults,
			})
			if len(items) == pageSize {
				c.JSON(http.StatusOK, gin.H{"items": items, "next_before_id": beforeID})
				return
			}
		}
		if len(tasks) < scanSize {
			c.JSON(http.StatusOK, gin.H{"items": items, "next_before_id": 0})
			return
		}
	}
}

// DeleteImageGenerationGalleryImages removes selected results from the current
// user's gallery. Original indices remain reserved for surviving downloads.
func DeleteImageGenerationGalleryImages(c *gin.Context) {
	var request struct {
		Items []model.ImageGallerySelection `json:"items"`
	}
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024), &request); err != nil || len(request.Items) == 0 || len(request.Items) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image selection"})
		return
	}
	for _, item := range request.Items {
		if item.TaskID == "" || len(item.TaskID) > 191 || item.Index < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image selection"})
			return
		}
	}
	deleted, err := model.DeleteImageGallerySelections(c.GetInt("id"), request.Items)
	if errors.Is(err, model.ErrImageGalleryImageNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete images"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

// ImageGenerationDownload returns a saved image as an attachment after
// checking that the requested gallery task belongs to the current user.
func ImageGenerationDownload(c *gin.Context) {
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil || index < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image index"})
		return
	}

	task, exists, err := model.GetByTaskId(c.GetInt("id"), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load image"})
		return
	}
	if !exists || task.Platform != constant.TaskPlatformGenerateImage ||
		task.Status != model.TaskStatusSuccess || !task.PrivateData.ImageGallery ||
		index >= len(task.PrivateData.ImageGalleryResults) {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}

	image := task.PrivateData.ImageGalleryResults[index]
	if image.URL == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}
	contentType, imageBytes, err := service.GetImageBytesFromUrlWithLimit(image.URL, service.AsyncImageMaxURLSizeMB)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to download image"})
		return
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid image content type"})
		return
	}
	if !strings.HasPrefix(mediaType, "image/") {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid image content type"})
		return
	}

	extension := ".img"
	if extensions, _ := mime.ExtensionsByType(mediaType); len(extensions) > 0 {
		extension = extensions[0]
	}
	filename := fmt.Sprintf("generated-image-%d-%d%s", task.ID, index+1, extension)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, mediaType, imageBytes)
}
