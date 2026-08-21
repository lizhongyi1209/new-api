package controller

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func ServeTemporaryOutputImage(c *gin.Context) {
	now := time.Now()
	file, info, err := service.OpenTemporaryOutputImage(c.Param("filename"), now)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) ||
			errors.Is(err, service.ErrTemporaryImageExpired) ||
			errors.Is(err, service.ErrTemporaryImageInvalidName) {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	expiresAt := info.ModTime().Add(service.TemporaryImageRetention)
	remainingSeconds := int64(expiresAt.Sub(now).Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, s-maxage=%d, must-revalidate", remainingSeconds, remainingSeconds))
	c.Header("Expires", expiresAt.UTC().Format(http.TimeFormat))
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), file)
}
