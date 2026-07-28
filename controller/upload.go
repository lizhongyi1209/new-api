package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// FileInfo represents a file in the upload directory
type FileInfo struct {
	Path         string `json:"path"`          // Relative path from uploads root
	Name         string `json:"name"`          // Filename
	Size         int64  `json:"size"`          // File size in bytes
	ModTime      int64  `json:"mod_time"`      // Last modified timestamp
	Category     string `json:"category"`      // uploads/elements/temp
	URL          string `json:"url"`           // Public URL
	ThumbnailURL string `json:"thumbnail_url"` // Thumbnail URL (same as URL for now)
	IsImage      bool   `json:"is_image"`      // Whether it's an image file
}

// ListUploadedFiles lists all files in the uploads directory
func ListUploadedFiles(c *gin.Context) {
	category := c.Query("category") // optional filter: uploads/elements/temp
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 50

	uploadDir := getUploadDir()
	publicBase := strings.TrimRight(getLocalPublicBase(), "/")

	var allFiles []FileInfo

	// Walk through upload directory
	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}
		if info.IsDir() {
			return nil
		}

		// Get relative path from upload root
		relPath, err := filepath.Rel(uploadDir, path)
		if err != nil {
			return nil
		}

		// Determine category from path
		parts := strings.Split(filepath.ToSlash(relPath), "/")
		fileCategory := "uploads"
		if len(parts) > 1 {
			fileCategory = parts[0]
		}

		// Filter by category if specified
		if category != "" && fileCategory != category {
			return nil
		}

		// Check if it's an image
		ext := strings.ToLower(filepath.Ext(info.Name()))
		isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"

		urlPath := filepath.ToSlash(relPath)
		fileInfo := FileInfo{
			Path:         relPath,
			Name:         info.Name(),
			Size:         info.Size(),
			ModTime:      info.ModTime().Unix(),
			Category:     fileCategory,
			URL:          fmt.Sprintf("%s/upload/%s", publicBase, urlPath),
			ThumbnailURL: fmt.Sprintf("%s/upload/%s", publicBase, urlPath),
			IsImage:      isImage,
		}

		allFiles = append(allFiles, fileInfo)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list files"})
		return
	}

	// Sort by modification time (newest first).
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].ModTime > allFiles[j].ModTime
	})

	// Calculate pagination
	total := len(allFiles)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		start = 0
		end = 0
	}
	if end > total {
		end = total
	}

	pageFiles := allFiles[start:end]

	// Calculate category statistics
	stats := map[string]interface{}{
		"uploads":  map[string]interface{}{"count": 0, "size": int64(0)},
		"elements": map[string]interface{}{"count": 0, "size": int64(0)},
		"temp":     map[string]interface{}{"count": 0, "size": int64(0)},
	}
	for _, f := range allFiles {
		if s, ok := stats[f.Category].(map[string]interface{}); ok {
			s["count"] = s["count"].(int) + 1
			s["size"] = s["size"].(int64) + f.Size
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pageFiles,
		"total":   total,
		"page":    page,
		"stats":   stats,
	})
}

// DeleteUploadedFile deletes a file from the uploads directory
func DeleteUploadedFile(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	// Security: prevent directory traversal
	if strings.Contains(req.Path, "..") || filepath.IsAbs(req.Path) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}

	uploadDir := getUploadDir()
	fullPath := filepath.Join(uploadDir, req.Path)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	// Delete file
	if err := os.Remove(fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "file deleted",
	})
}

// BatchDeleteUploadedFiles deletes multiple files
func BatchDeleteUploadedFiles(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "paths is required"})
		return
	}

	uploadDir := getUploadDir()
	deleted := 0
	failed := 0

	for _, path := range req.Paths {
		// Security check
		if strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
			failed++
			continue
		}

		fullPath := filepath.Join(uploadDir, path)
		if err := os.Remove(fullPath); err != nil {
			failed++
		} else {
			deleted++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"deleted": deleted,
		"failed":  failed,
	})
}

// GetUploadStats returns storage statistics
func GetUploadStats(c *gin.Context) {
	uploadDir := getUploadDir()

	stats := map[string]interface{}{
		"uploads": map[string]interface{}{
			"count": 0,
			"size":  int64(0),
		},
		"elements": map[string]interface{}{
			"count": 0,
			"size":  int64(0),
		},
		"temp": map[string]interface{}{
			"count": 0,
			"size":  int64(0),
		},
		"total": map[string]interface{}{
			"count": 0,
			"size":  int64(0),
		},
	}

	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(uploadDir, path)
		parts := strings.Split(filepath.ToSlash(relPath), "/")
		category := "uploads"
		if len(parts) > 1 {
			category = parts[0]
		}

		if s, ok := stats[category].(map[string]interface{}); ok {
			s["count"] = s["count"].(int) + 1
			s["size"] = s["size"].(int64) + info.Size()
		}

		totalStats := stats["total"].(map[string]interface{})
		totalStats["count"] = totalStats["count"].(int) + 1
		totalStats["size"] = totalStats["size"].(int64) + info.Size()

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// CleanOldFiles cleans files older than specified days in a category
func CleanOldFiles(c *gin.Context) {
	var req struct {
		Category string `json:"category"`
		Days     int    `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category and days are required"})
		return
	}

	// Prevent cleaning elements directory
	if req.Category == "elements" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot auto-clean elements directory"})
		return
	}

	if req.Days < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid days value"})
		return
	}

	uploadDir := getUploadDir()
	categoryPath := filepath.Join(uploadDir, req.Category)

	cutoffTime := time.Now().AddDate(0, 0, -req.Days)
	deleted := 0
	var deletedSize int64

	err := filepath.Walk(categoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err == nil {
				deleted++
				deletedSize += info.Size()
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clean files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"deleted": deleted,
		"size":    deletedSize,
		"message": fmt.Sprintf("Deleted %d files older than %d days", deleted, req.Days),
	})
}

// ClearUploadedFiles removes every file managed by the local upload directory.
// The upload root itself is preserved so subsequent uploads continue to work.
func ClearUploadedFiles(c *gin.Context) {
	uploadDir := getUploadDir()
	deleted := 0
	failed := 0
	var deletedSize int64

	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			failed++
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if err := os.Remove(path); err != nil {
			failed++
			return nil
		}
		deleted++
		deletedSize += info.Size()
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear uploaded files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"deleted": deleted,
		"failed":  failed,
		"size":    deletedSize,
	})
}
