package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type imageGalleryItem struct {
	TaskID  string                         `json:"task_id"`
	Model   string                         `json:"model"`
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
			items = append(items, imageGalleryItem{
				TaskID:  task.TaskID,
				Model:   task.Properties.OriginModelName,
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
