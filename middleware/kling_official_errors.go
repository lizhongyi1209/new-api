package middleware

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// klingErrorWriter holds the response until the handler chain finishes so a
// failure can be rewritten. The root Kling routes answer with small JSON bodies.
type klingErrorWriter struct {
	gin.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *klingErrorWriter) WriteHeader(code int)              { w.status = code }
func (w *klingErrorWriter) WriteHeaderNow()                   {}
func (w *klingErrorWriter) Write(b []byte) (int, error)       { return w.body.Write(b) }
func (w *klingErrorWriter) WriteString(s string) (int, error) { return w.body.WriteString(s) }
func (w *klingErrorWriter) Written() bool                     { return w.status != 0 || w.body.Len() > 0 }
func (w *klingErrorWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// KlingOfficialErrors gives the root Kling routes Kling's error contract,
// {"code": <business code>, "message", "request_id"}, for every failure,
// including those raised before the request reaches Kling (missing key, quota).
func KlingOfficialErrors() gin.HandlerFunc {
	return func(c *gin.Context) {
		original := c.Writer
		buffered := &klingErrorWriter{ResponseWriter: original}
		c.Writer = buffered
		c.Next()
		c.Writer = original

		status := buffered.Status()
		if status < http.StatusBadRequest {
			original.WriteHeader(status)
			_, _ = original.Write(buffered.body.Bytes())
			return
		}
		var failure struct {
			Message string `json:"message"`
			Error   struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = common.Unmarshal(buffered.body.Bytes(), &failure)
		message := failure.Message
		if message == "" {
			message = failure.Error.Message
		}
		if message == "" {
			message = http.StatusText(status)
		}
		status, code := klingErrorCode(status, message)
		original.Header().Set("Content-Type", "application/json; charset=utf-8")
		original.WriteHeader(status)
		body, _ := common.Marshal(map[string]any{"code": code, "message": message, "request_id": c.GetString(common.RequestIdKey)})
		_, _ = original.Write(body)
	}
}

// klingErrorCode maps a gateway failure to Kling's HTTP status and business code.
func klingErrorCode(status int, message string) (int, int) {
	lower := strings.ToLower(message)
	if status == http.StatusPaymentRequired || strings.Contains(lower, "quota") || strings.Contains(message, "额度") || strings.Contains(message, "余额") {
		return http.StatusPaymentRequired, 1101
	}
	switch {
	case status == http.StatusUnauthorized:
		return status, 1000
	case status == http.StatusForbidden:
		return status, 1103
	case status == http.StatusNotFound:
		return status, 1203
	case status == http.StatusTooManyRequests:
		return status, 1302
	case status == http.StatusServiceUnavailable:
		return status, 5001
	case status == http.StatusGatewayTimeout:
		return status, 5002
	case status >= http.StatusInternalServerError:
		return status, 5000
	}
	return http.StatusBadRequest, 1201
}
