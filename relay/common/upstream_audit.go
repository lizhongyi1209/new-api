package common

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const (
	UpstreamRequestSnapshotKey  = "upstream_request_snapshot"
	UpstreamResponseSnapshotKey = "upstream_response_snapshot"

	upstreamAuditMaxFieldBytes = 64 * 1024
	upstreamAuditMaxTotalBytes = 256 * 1024
	upstreamAuditMaxBodyBytes  = 128 * 1024
)

// UpstreamRequestSnapshot is an administrator-only audit representation of
// the effective request sent to an upstream provider. Multipart media content
// is represented by metadata and hashes instead of being persisted.
type UpstreamRequestSnapshot struct {
	SchemaVersion int                   `json:"schema_version"`
	Method        string                `json:"method"`
	Path          string                `json:"path"`
	ContentType   string                `json:"content_type"`
	ContentLength int64                 `json:"content_length"`
	Parts         []UpstreamRequestPart `json:"parts"`
	capturedBytes int
}

type UpstreamRequestPart struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Value         string `json:"value,omitempty"`
	Filename      string `json:"filename,omitempty"`
	ContentType   string `json:"content_type,omitempty"`
	Size          int64  `json:"size,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	Omitted       bool   `json:"omitted,omitempty"`
	OmittedReason string `json:"omitted_reason,omitempty"`
	OriginalBytes int    `json:"original_bytes,omitempty"`
}

type UpstreamResponseSnapshot struct {
	SchemaVersion int    `json:"schema_version"`
	StatusCode    int    `json:"status_code"`
	ContentType   string `json:"content_type,omitempty"`
	Body          any    `json:"body,omitempty"`
	OriginalBytes int    `json:"original_bytes"`
	Truncated     bool   `json:"truncated,omitempty"`
}

// UpstreamResponseCapture records a bounded copy while the provider adaptor
// consumes the response. The copy is persisted only if the request fails.
type UpstreamResponseCapture struct {
	body          io.ReadCloser
	preview       bytes.Buffer
	originalBytes int
}

func NewUpstreamMultipartRequestSnapshot(method string, path string) *UpstreamRequestSnapshot {
	return &UpstreamRequestSnapshot{
		SchemaVersion: 1,
		Method:        method,
		Path:          path,
		ContentType:   "multipart/form-data",
		Parts:         make([]UpstreamRequestPart, 0),
	}
}

func (s *UpstreamRequestSnapshot) AddField(name string, value string) {
	if s == nil {
		return
	}
	part := UpstreamRequestPart{
		Kind:          "field",
		Name:          name,
		OriginalBytes: len(value),
	}

	if isSensitiveUpstreamAuditField(name) {
		part.Omitted = true
		part.OmittedReason = "sensitive"
		s.Parts = append(s.Parts, part)
		return
	}

	if reason := imageValueOmissionReason(name, value); reason != "" {
		part.Omitted = true
		part.OmittedReason = reason
		part.SHA256 = upstreamAuditStringHash(value)
		s.Parts = append(s.Parts, part)
		return
	}

	if len(value) > upstreamAuditMaxFieldBytes {
		part.Omitted = true
		part.OmittedReason = "value_too_large"
		part.SHA256 = upstreamAuditStringHash(value)
		s.Parts = append(s.Parts, part)
		return
	}
	if s.capturedBytes+len(value) > upstreamAuditMaxTotalBytes {
		part.Omitted = true
		part.OmittedReason = "snapshot_limit"
		part.SHA256 = upstreamAuditStringHash(value)
		s.Parts = append(s.Parts, part)
		return
	}

	part.Value = value
	s.capturedBytes += len(value)
	s.Parts = append(s.Parts, part)
}

func (s *UpstreamRequestSnapshot) AddFile(name string, filename string, contentType string, size int64, sha256Hex string) {
	if s == nil {
		return
	}
	s.Parts = append(s.Parts, UpstreamRequestPart{
		Kind:          "file",
		Name:          name,
		Filename:      filename,
		ContentType:   contentType,
		Size:          size,
		SHA256:        sha256Hex,
		Omitted:       true,
		OmittedReason: "binary_media",
	})
}

func SetUpstreamRequestSnapshot(c *gin.Context, snapshot *UpstreamRequestSnapshot) {
	if c != nil {
		c.Set(UpstreamRequestSnapshotKey, snapshot)
	}
}

func GetUpstreamRequestSnapshot(c *gin.Context) *UpstreamRequestSnapshot {
	if c == nil {
		return nil
	}
	value, exists := c.Get(UpstreamRequestSnapshotKey)
	if !exists {
		return nil
	}
	snapshot, _ := value.(*UpstreamRequestSnapshot)
	return snapshot
}

func SetUpstreamResponseSnapshot(c *gin.Context, statusCode int, contentType string, body []byte) {
	setUpstreamResponseSnapshot(c, statusCode, contentType, body, len(body), false)
}

func NewUpstreamResponseCapture(body io.ReadCloser) *UpstreamResponseCapture {
	return &UpstreamResponseCapture{body: body}
}

func (capture *UpstreamResponseCapture) Read(p []byte) (int, error) {
	if capture == nil || capture.body == nil {
		return 0, io.EOF
	}
	n, err := capture.body.Read(p)
	capture.originalBytes += n
	remaining := upstreamAuditMaxBodyBytes - capture.preview.Len()
	if remaining > 0 && n > 0 {
		if n < remaining {
			remaining = n
		}
		_, _ = capture.preview.Write(p[:remaining])
	}
	return n, err
}

func (capture *UpstreamResponseCapture) Close() error {
	if capture == nil || capture.body == nil {
		return nil
	}
	return capture.body.Close()
}

func (capture *UpstreamResponseCapture) StoreSnapshot(c *gin.Context, statusCode int, contentType string) {
	if capture == nil {
		return
	}
	setUpstreamResponseSnapshot(
		c,
		statusCode,
		contentType,
		capture.preview.Bytes(),
		capture.originalBytes,
		capture.originalBytes > capture.preview.Len(),
	)
}

func setUpstreamResponseSnapshot(c *gin.Context, statusCode int, contentType string, body []byte, originalBytes int, truncated bool) {
	if c == nil {
		return
	}
	snapshot := &UpstreamResponseSnapshot{
		SchemaVersion: 1,
		StatusCode:    statusCode,
		ContentType:   contentType,
		OriginalBytes: originalBytes,
		Truncated:     truncated,
	}

	var value any
	if !snapshot.Truncated && len(body) <= upstreamAuditMaxBodyBytes && rootcommon.Unmarshal(body, &value) == nil {
		snapshot.Body = sanitizeUpstreamAuditValue(value)
	} else {
		preview := body
		if len(preview) > upstreamAuditMaxBodyBytes {
			preview = preview[:upstreamAuditMaxBodyBytes]
			snapshot.Truncated = true
		}
		snapshot.Body = strings.ToValidUTF8(string(preview), "�")
	}

	encoded, err := rootcommon.Marshal(snapshot.Body)
	if err != nil || len(encoded) > upstreamAuditMaxBodyBytes {
		snapshot.Body = fmt.Sprintf("[omitted %d bytes]", originalBytes)
		snapshot.Truncated = true
	}
	c.Set(UpstreamResponseSnapshotKey, snapshot)
}

func GetUpstreamResponseSnapshot(c *gin.Context) *UpstreamResponseSnapshot {
	if c == nil {
		return nil
	}
	value, exists := c.Get(UpstreamResponseSnapshotKey)
	if !exists {
		return nil
	}
	snapshot, _ := value.(*UpstreamResponseSnapshot)
	return snapshot
}

func ClearUpstreamAuditSnapshots(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(UpstreamRequestSnapshotKey, nil)
	c.Set(UpstreamResponseSnapshotKey, nil)
}

func isSensitiveUpstreamAuditField(name string) bool {
	normalized := strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(strings.TrimSpace(name)))
	switch normalized {
	case "authorization", "api_key", "apikey", "access_token", "refresh_token", "password", "secret", "token":
		return true
	}
	return strings.HasSuffix(normalized, "_api_key") ||
		strings.HasSuffix(normalized, "_access_token") ||
		strings.HasSuffix(normalized, "_refresh_token") ||
		strings.HasSuffix(normalized, "_password") ||
		strings.HasSuffix(normalized, "_secret")
}

func imageValueOmissionReason(name string, value string) string {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	if !strings.Contains(normalizedName, "image") && normalizedName != "mask" {
		return ""
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	prefix := trimmed
	if len(prefix) > 256 {
		prefix = prefix[:256]
	}
	lowerPrefix := strings.ToLower(prefix)
	if strings.Contains(lowerPrefix, ";base64,") || hasBase64ImageMagic(prefix) {
		return "base64_image"
	}
	if len(value) > 16*1024 {
		return "image_payload"
	}
	return ""
}

func hasBase64ImageMagic(value string) bool {
	sample := make([]byte, 0, 192)
	for i := 0; i < len(value) && len(sample) < 192; i++ {
		ch := value[i]
		if ch == '\r' || ch == '\n' || ch == '\t' || ch == ' ' {
			continue
		}
		if ch == ',' {
			sample = sample[:0]
			continue
		}
		sample = append(sample, ch)
	}
	if len(sample) < 32 {
		return false
	}
	sample = sample[:len(sample)-len(sample)%4]
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(sample)))
	n, err := base64.StdEncoding.Decode(decoded, sample)
	if err != nil {
		return false
	}
	decoded = decoded[:n]
	return hasImageMagic(decoded)
}

func hasImageMagic(data []byte) bool {
	return (len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n") ||
		(len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff) ||
		(len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a")) ||
		(len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP")
}

func upstreamAuditStringHash(value string) string {
	hasher := sha256.New()
	_, _ = io.WriteString(hasher, value)
	return hex.EncodeToString(hasher.Sum(nil))
}

func sanitizeUpstreamAuditValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(typed))
		for key, item := range typed {
			if isSensitiveUpstreamAuditField(key) {
				cleaned[key] = "[redacted]"
				continue
			}
			if text, ok := item.(string); ok && imageValueOmissionReason(key, text) != "" {
				cleaned[key] = fmt.Sprintf("[omitted %d bytes]", len(text))
				continue
			}
			cleaned[key] = sanitizeUpstreamAuditValue(item)
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(typed))
		for i := range typed {
			cleaned[i] = sanitizeUpstreamAuditValue(typed[i])
		}
		return cleaned
	case string:
		if strings.Contains(typed, ";base64,") || len(typed) > upstreamAuditMaxFieldBytes {
			return fmt.Sprintf("[omitted %d bytes]", len(typed))
		}
		return strings.ToValidUTF8(typed, "�")
	default:
		return value
	}
}
