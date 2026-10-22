package common

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/url"
	"sort"
	"strings"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const ClientRequestSnapshotKey = "client_request_snapshot"

const (
	clientRequestAuditMaxBodyBytes  = 512 * 1024
	clientRequestAuditMaxFieldBytes = 4 * 1024
	clientRequestAuditMaxTotalBytes = 32 * 1024
	clientRequestAuditMaxFields     = 128
	clientRequestAuditMaxDepth      = 8
)

// ClientRequestSnapshot is a bounded, administrator-only representation of a
// client request body. Values that can contain credentials, media, Base64, or
// large user content are represented by an omission reason and hash only.
type ClientRequestSnapshot struct {
	SchemaVersion int                  `json:"schema_version"`
	Method        string               `json:"method"`
	Path          string               `json:"path"`
	ContentType   string               `json:"content_type,omitempty"`
	ContentLength int64                `json:"content_length"`
	Fields        []ClientRequestField `json:"fields"`
	Truncated     bool                 `json:"truncated,omitempty"`
	capturedBytes int
}

type ClientRequestField struct {
	Kind          string `json:"kind"`
	Path          string `json:"path"`
	Value         string `json:"value,omitempty"`
	Filename      string `json:"filename,omitempty"`
	ContentType   string `json:"content_type,omitempty"`
	Size          int64  `json:"size,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	OriginalBytes int    `json:"original_bytes,omitempty"`
	OmittedReason string `json:"omitted_reason,omitempty"`
}

// CaptureClientRequestSnapshot captures the request body after the relay has
// parsed it. It is deliberately bounded so request diagnostics cannot turn
// usage logs into a second body store.
func CaptureClientRequestSnapshot(c *gin.Context) *ClientRequestSnapshot {
	if c == nil {
		return nil
	}
	if existing := GetClientRequestSnapshot(c); existing != nil {
		return existing
	}

	snapshot := &ClientRequestSnapshot{
		SchemaVersion: 1,
		Method:        "",
		Path:          "",
		Fields:        make([]ClientRequestField, 0),
	}
	if c.Request != nil {
		snapshot.Method = c.Request.Method
		if c.Request.URL != nil {
			snapshot.Path = c.Request.URL.Path
		}
		snapshot.ContentType = c.Request.Header.Get("Content-Type")
		snapshot.ContentLength = c.Request.ContentLength
	}

	if c.Request == nil || c.Request.Body == nil {
		SetClientRequestSnapshot(c, snapshot)
		return snapshot
	}
	storage, err := rootcommon.GetBodyStorage(c)
	if err != nil {
		snapshot.Truncated = true
		snapshot.addOmitted("$body", "body_unavailable", 0, "")
		SetClientRequestSnapshot(c, snapshot)
		return snapshot
	}
	if snapshot.ContentLength < 0 {
		snapshot.ContentLength = storage.Size()
	}

	contentType := strings.ToLower(snapshot.ContentType)
	if strings.Contains(contentType, "multipart/form-data") {
		captureClientMultipart(snapshot, c)
	} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		captureClientForm(snapshot, storage)
	} else if strings.Contains(contentType, "json") {
		captureClientJSON(snapshot, storage)
	} else {
		snapshot.addOmitted("$body", "unsupported_content_type", 0, "")
	}
	if snapshot.ContentLength > clientRequestAuditMaxBodyBytes {
		snapshot.Truncated = true
	}
	SetClientRequestSnapshot(c, snapshot)
	return snapshot
}

func captureClientMultipart(snapshot *ClientRequestSnapshot, c *gin.Context) {
	if c.Request.MultipartForm == nil {
		snapshot.addOmitted("$body", "multipart_not_parsed", 0, "")
		return
	}
	form := c.Request.MultipartForm
	keys := make([]string, 0, len(form.Value))
	for key := range form.Value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range form.Value[key] {
			snapshot.addString(key, value)
		}
	}
	fileKeys := make([]string, 0, len(form.File))
	for key := range form.File {
		fileKeys = append(fileKeys, key)
	}
	sort.Strings(fileKeys)
	for _, key := range fileKeys {
		for _, file := range form.File[key] {
			if len(snapshot.Fields) >= clientRequestAuditMaxFields {
				snapshot.Truncated = true
				return
			}
			filename := file.Filename
			if len(filename) > 256 {
				filename = filename[:256]
			}
			contentType := ""
			if file.Header != nil {
				contentType = file.Header.Get("Content-Type")
			}
			snapshot.Fields = append(snapshot.Fields, ClientRequestField{
				Kind:          "file",
				Path:          key,
				Filename:      filename,
				ContentType:   contentType,
				Size:          file.Size,
				OriginalBytes: int(file.Size),
				OmittedReason: "binary_media",
			})
		}
	}
}

func captureClientForm(snapshot *ClientRequestSnapshot, storage rootcommon.BodyStorage) {
	if storage.Size() > clientRequestAuditMaxBodyBytes {
		snapshot.addOmitted("$body", "body_too_large", int(storage.Size()), "")
		return
	}
	data, err := readClientAuditBody(storage)
	if err != nil {
		snapshot.addOmitted("$body", "body_unavailable", 0, "")
		return
	}
	values, err := url.ParseQuery(string(data))
	if err != nil {
		snapshot.addOmitted("$body", "invalid_form", len(data), "")
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range values[key] {
			snapshot.addString(key, value)
		}
	}
}

func captureClientJSON(snapshot *ClientRequestSnapshot, storage rootcommon.BodyStorage) {
	if storage.Size() > clientRequestAuditMaxBodyBytes {
		snapshot.addOmitted("$body", "body_too_large", int(storage.Size()), "")
		return
	}
	data, err := readClientAuditBody(storage)
	if err != nil {
		snapshot.addOmitted("$body", "body_unavailable", 0, "")
		return
	}
	var object map[string]json.RawMessage
	if err := rootcommon.Unmarshal(data, &object); err == nil {
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			snapshot.addJSONValue(key, object[key], 0)
		}
		return
	}
	var array []json.RawMessage
	if err := rootcommon.Unmarshal(data, &array); err == nil {
		for index, value := range array {
			snapshot.addJSONValue("["+itoa(index)+"]", value, 0)
		}
		return
	}
	snapshot.addOmitted("$body", "invalid_json", len(data), "")
}

func readClientAuditBody(storage rootcommon.BodyStorage) ([]byte, error) {
	reader, err := storage.NewReader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, clientRequestAuditMaxBodyBytes+1))
}

func (s *ClientRequestSnapshot) addJSONValue(path string, raw json.RawMessage, depth int) {
	if s == nil || len(s.Fields) >= clientRequestAuditMaxFields {
		s.Truncated = true
		return
	}
	if depth > clientRequestAuditMaxDepth {
		s.addOmitted(path, "nesting_too_deep", len(raw), "")
		return
	}
	if isSensitiveRequestAuditPath(path) {
		s.addOmitted(path, "sensitive", len(raw), "")
		return
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		s.addOmitted(path, "empty_value", 0, "")
		return
	}
	switch trimmed[0] {
	case '{':
		var object map[string]json.RawMessage
		if rootcommon.Unmarshal(raw, &object) != nil {
			s.addOmitted(path, "invalid_object", len(raw), "")
			return
		}
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			s.addJSONValue(path+"."+key, object[key], depth+1)
		}
	case '[':
		var array []json.RawMessage
		if rootcommon.Unmarshal(raw, &array) != nil {
			s.addOmitted(path, "invalid_array", len(raw), "")
			return
		}
		for index, value := range array {
			s.addJSONValue(path+"["+itoa(index)+"]", value, depth+1)
		}
	default:
		if trimmed == "null" || trimmed == "true" || trimmed == "false" || isJSONNumber(trimmed) {
			s.addScalar(path, trimmed, len(raw))
			return
		}
		var value string
		if rootcommon.Unmarshal(raw, &value) != nil {
			s.addOmitted(path, "unsupported_value", len(raw), "")
			return
		}
		s.addString(path, value)
	}
}

func (s *ClientRequestSnapshot) addString(path, value string) {
	if s == nil || len(s.Fields) >= clientRequestAuditMaxFields {
		if s != nil {
			s.Truncated = true
		}
		return
	}
	if isSensitiveRequestAuditPath(path) {
		s.addOmitted(path, "sensitive", len(value), "")
		return
	}
	if isLikelyBase64Payload(value) {
		s.addOmitted(path, "base64_payload", len(value), upstreamAuditStringHash(value))
		return
	}
	if len(value) > clientRequestAuditMaxFieldBytes {
		s.addOmitted(path, "value_too_large", len(value), upstreamAuditStringHash(value))
		return
	}
	if s.capturedBytes+len(value) > clientRequestAuditMaxTotalBytes {
		s.addOmitted(path, "snapshot_limit", len(value), upstreamAuditStringHash(value))
		return
	}
	s.Fields = append(s.Fields, ClientRequestField{Kind: "scalar", Path: path, Value: value, OriginalBytes: len(value)})
	s.capturedBytes += len(value)
}

func (s *ClientRequestSnapshot) addScalar(path, value string, originalBytes int) {
	if value == "null" {
		s.addOmitted(path, "null", originalBytes, "")
		return
	}
	s.addString(path, value)
	if len(s.Fields) > 0 && s.Fields[len(s.Fields)-1].Path == path {
		s.Fields[len(s.Fields)-1].OriginalBytes = originalBytes
	}
}

func (s *ClientRequestSnapshot) addOmitted(path, reason string, originalBytes int, hash string) {
	if s == nil || len(s.Fields) >= clientRequestAuditMaxFields {
		if s != nil {
			s.Truncated = true
		}
		return
	}
	s.Fields = append(s.Fields, ClientRequestField{
		Kind:          "omitted",
		Path:          path,
		OriginalBytes: originalBytes,
		SHA256:        hash,
		OmittedReason: reason,
	})
}

func isSensitiveRequestAuditPath(path string) bool {
	last := path
	if index := strings.LastIndexAny(last, ".["); index >= 0 {
		last = last[index+1:]
	}
	last = strings.Trim(last, "] ")
	if isSensitiveUpstreamAuditField(last) {
		return true
	}
	normalized := strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(strings.TrimSpace(last)))
	return normalized == "cookie" || normalized == "credential" ||
		normalized == "private_key" || strings.Contains(normalized, "signature")
}

func isLikelyBase64Payload(value string) bool {
	trimmed := strings.TrimSpace(value)
	if strings.Contains(strings.ToLower(trimmed), ";base64,") {
		return true
	}
	if len(trimmed) < 256 {
		return false
	}
	compact := strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t', ' ':
			return -1
		default:
			return r
		}
	}, trimmed)
	if len(compact) < 256 {
		return false
	}
	for _, ch := range compact {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
			(ch >= '0' && ch <= '9') || ch == '+' || ch == '/' || ch == '=' || ch == '-' || ch == '_' {
			continue
		}
		return false
	}
	var decoded []byte
	var err error
	if strings.ContainsAny(compact, "-_") {
		decoded, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(compact, "="))
	} else if len(compact)%4 == 0 {
		decoded, err = base64.StdEncoding.DecodeString(compact)
	} else {
		decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(compact, "="))
	}
	return err == nil && len(decoded) >= 128
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}

func SetClientRequestSnapshot(c *gin.Context, snapshot *ClientRequestSnapshot) {
	if c != nil {
		c.Set(ClientRequestSnapshotKey, snapshot)
	}
}

func GetClientRequestSnapshot(c *gin.Context) *ClientRequestSnapshot {
	if c == nil {
		return nil
	}
	value, exists := c.Get(ClientRequestSnapshotKey)
	if !exists {
		return nil
	}
	snapshot, _ := value.(*ClientRequestSnapshot)
	return snapshot
}
