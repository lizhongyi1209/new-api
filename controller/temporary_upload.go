package controller

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

var (
	errMultipleTemporaryInputFiles = errors.New("multiple files are not supported")
	errInvalidTemporaryInputParts  = errors.New("invalid multipart request")
)

type remainingTemporaryUploadParts struct {
	reader *multipart.Reader
}

func (parts *remainingTemporaryUploadParts) Read(_ []byte) (int, error) {
	for {
		part, err := parts.reader.NextPart()
		if errors.Is(err, io.EOF) {
			return 0, io.EOF
		}
		if err != nil {
			return 0, errInvalidTemporaryInputParts
		}
		isFile := part.FormName() == "file" || part.FileName() != ""
		_ = part.Close()
		if isFile {
			return 0, errMultipleTemporaryInputFiles
		}
	}
}

func UploadTemporaryInputAttachment(c *gin.Context) {
	multipartReader, err := c.Request.MultipartReader()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request must be multipart/form-data"})
		return
	}

	for {
		part, err := multipartReader.NextPart()
		if errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
			return
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart request"})
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			_ = part.Close()
			continue
		}

		originalFilename := filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
		attachment, storeErr := service.StoreTemporaryInputAttachment(c.Request.Context(), io.MultiReader(part, &remainingTemporaryUploadParts{reader: multipartReader}), originalFilename)
		_ = part.Close()
		if storeErr != nil {
			switch {
			case errors.Is(storeErr, service.ErrTemporaryInputEmpty):
				c.JSON(http.StatusBadRequest, gin.H{"error": "file must not be empty"})
			case errors.Is(storeErr, service.ErrTemporaryInputTooLarge):
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": fmt.Sprintf("file exceeds the %d MiB limit", service.TemporaryInputMaxFileBytes/(1024*1024)),
				})
			case errors.Is(storeErr, service.ErrTemporaryInputUnsupportedType):
				c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported attachment type"})
			case errors.Is(storeErr, errMultipleTemporaryInputFiles):
				c.JSON(http.StatusBadRequest, gin.H{"error": "multiple files are not supported"})
			case errors.Is(storeErr, errInvalidTemporaryInputParts):
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart request"})
			default:
				logger.LogError(c, "temporary input upload failed: "+common.MaskSensitiveInfo(storeErr.Error()))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store attachment"})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"url":          attachment.URL,
			"filename":     originalFilename,
			"content_type": attachment.ContentType,
			"size":         attachment.Size,
			"expires_at":   attachment.ExpiresAt.Unix(),
		})
		return
	}
}

func ServeTemporaryInputAttachment(c *gin.Context) {
	now := time.Now()
	file, info, contentType, err := service.OpenTemporaryInputAttachment(c.Param("filename"), now)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) ||
			errors.Is(err, service.ErrTemporaryInputExpired) ||
			errors.Is(err, service.ErrTemporaryInputInvalidName) {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	expiresAt := info.ModTime().Add(service.TemporaryInputRetention)
	remainingSeconds := int64(expiresAt.Sub(now).Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, s-maxage=%d, must-revalidate", remainingSeconds, remainingSeconds))
	c.Header("Content-Type", contentType)
	c.Header("Expires", expiresAt.UTC().Format(http.TimeFormat))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	if !strings.HasPrefix(contentType, "image/") &&
		!strings.HasPrefix(contentType, "audio/") &&
		!strings.HasPrefix(contentType, "video/") {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
	}
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), file)
}
