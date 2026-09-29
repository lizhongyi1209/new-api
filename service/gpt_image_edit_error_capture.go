package service

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

const imageEdit400CaptureUntilEnv = "GPT_IMAGE_EDIT_400_CAPTURE_UNTIL"

type imageEdit400CapturePart struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Value         string `json:"value,omitempty"`
	OriginalBytes int    `json:"original_bytes,omitempty"`
	ContentType   string `json:"content_type,omitempty"`
	Size          int64  `json:"size,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	OmittedReason string `json:"omitted_reason,omitempty"`
}

type imageEdit400Capture struct {
	SchemaVersion               int                       `json:"schema_version"`
	CapturedAt                  string                    `json:"captured_at"`
	UpstreamModel               string                    `json:"upstream_model"`
	UpstreamPath                string                    `json:"upstream_path"`
	ContentLength               int64                     `json:"content_length"`
	ClientResponseFormatPresent bool                      `json:"client_response_format_present"`
	ClientResponseFormat        string                    `json:"client_response_format,omitempty"`
	OutboundResponseFormatFound bool                      `json:"outbound_response_format_found"`
	Parts                       []imageEdit400CapturePart `json:"parts"`
	Truncated                   bool                      `json:"truncated,omitempty"`
}

// appendGPTImageEdit400Capture stores only bounded, non-secret form parameters
// from the effective upstream request. It automatically stops after the UTC
// deadline configured for a temporary diagnostic window.
func appendGPTImageEdit400Capture(c *gin.Context, info *relaycommon.RelayInfo, other *model.LogOther) {
	if c == nil || info == nil || other == nil ||
		info.RelayMode != relayconstant.RelayModeImagesEdits || !relaycommon.IsGPTImage(info) {
		return
	}
	until, err := time.Parse(time.RFC3339, os.Getenv(imageEdit400CaptureUntilEnv))
	if err != nil || !time.Now().UTC().Before(until) {
		return
	}
	snapshot := relaycommon.GetUpstreamRequestSnapshot(c)
	response := relaycommon.GetUpstreamResponseSnapshot(c)
	if snapshot == nil || response == nil || response.StatusCode != http.StatusBadRequest {
		return
	}

	capture := imageEdit400Capture{
		SchemaVersion: 1,
		CapturedAt:    time.Now().UTC().Format(time.RFC3339),
		UpstreamModel: info.UpstreamModelName,
		UpstreamPath:  snapshot.Path,
		ContentLength: snapshot.ContentLength,
		Parts:         make([]imageEdit400CapturePart, 0, min(len(snapshot.Parts), 128)),
	}
	if c.Request != nil && c.Request.MultipartForm != nil {
		for name, values := range c.Request.MultipartForm.Value {
			if !strings.EqualFold(strings.TrimSpace(name), "response_format") {
				continue
			}
			capture.ClientResponseFormatPresent = true
			if len(values) > 0 && len(values[0]) <= 256 {
				capture.ClientResponseFormat = values[0]
			}
			break
		}
	}
	for _, original := range snapshot.Parts {
		if original.Kind != "file" && strings.EqualFold(strings.TrimSpace(original.Name), "response_format") {
			capture.OutboundResponseFormatFound = true
		}
		if len(capture.Parts) == 128 {
			capture.Truncated = true
			continue
		}
		name := original.Name
		if len(name) > 128 {
			name = name[:128]
		}
		part := imageEdit400CapturePart{
			Kind:          original.Kind,
			Name:          name,
			OriginalBytes: original.OriginalBytes,
		}
		if original.Kind == "file" {
			part.ContentType = original.ContentType
			part.Size = original.Size
			part.SHA256 = original.SHA256
			part.OmittedReason = "binary_media"
		} else {
			key := strings.ToLower(strings.TrimSpace(original.Name))
			if original.Omitted {
				part.OmittedReason = original.OmittedReason
			} else if len(original.Value) > 256 {
				part.OmittedReason = "value_too_large"
			} else {
				switch key {
				case "model", "n", "size", "quality", "background", "moderation", "output_format", "output_compression", "input_fidelity", "partial_images", "stream", "response_format", "aspect_ratio", "style":
					part.Value = original.Value
				default:
					part.OmittedReason = "unlisted_field"
				}
			}
		}
		capture.Parts = append(capture.Parts, part)
	}
	other.SetAdmin("gpt_image_edit_400_capture", capture)
}
