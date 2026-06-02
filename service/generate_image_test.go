package service

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

func testStringPtr(value string) *string {
	return &value
}

func TestIsGeminiImageModelName(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{model: "nano-banana-pro", want: true},
		{model: "nano-banana-2-2k", want: true},
		{model: "gemini-3-pro-image-preview", want: true},
		{model: "gemini-3.1-flash-image-preview", want: true},
		{model: "gpt-image-1", want: false},
	}

	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			if got := isGeminiImageModelName(test.model); got != test.want {
				t.Fatalf("isGeminiImageModelName(%q) = %v, want %v", test.model, got, test.want)
			}
		})
	}
}

func TestValidateGenerateImageRequestNormalizesNewParameters(t *testing.T) {
	req := &dto.GenerateImageRequest{
		Model:        "gpt-image-1",
		Prompt:       "draw a cat",
		Size:         " AUTO ",
		Quality:      " HIGH ",
		OutputFormat: testStringPtr(" WEBP "),
		Mask: &dto.ImageReference{
			ImageURL: testStringPtr(" data:image/png;base64,AAAA "),
		},
		Images: []string{"data:image/png;base64,BBBB"},
	}

	if err := ValidateGenerateImageRequest(req); err != nil {
		t.Fatalf("ValidateGenerateImageRequest returned error: %v", err)
	}
	if req.Size != "auto" {
		t.Fatalf("Size = %q, want auto", req.Size)
	}
	if req.Quality != "high" {
		t.Fatalf("Quality = %q, want high", req.Quality)
	}
	if req.OutputFormat == nil || *req.OutputFormat != "webp" {
		t.Fatalf("OutputFormat = %v, want webp", req.OutputFormat)
	}
	if req.Mask == nil || req.Mask.ImageURL == nil || *req.Mask.ImageURL != "data:image/png;base64,AAAA" {
		t.Fatalf("Mask.ImageURL = %v, want trimmed data URL", req.Mask)
	}
}

func TestValidateGenerateImageRequestRejectsInvalidEnums(t *testing.T) {
	tests := []struct {
		name    string
		req     *dto.GenerateImageRequest
		wantErr string
	}{
		{
			name: "quality",
			req: &dto.GenerateImageRequest{
				Model:   "gpt-image-1",
				Prompt:  "draw a cat",
				Quality: "best",
			},
			wantErr: "quality must be one of",
		},
		{
			name: "output format",
			req: &dto.GenerateImageRequest{
				Model:        "gpt-image-1",
				Prompt:       "draw a cat",
				OutputFormat: testStringPtr("gif"),
			},
			wantErr: "output_format must be one of",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateGenerateImageRequest(test.req)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateGenerateImageRequestValidatesMask(t *testing.T) {
	tests := []struct {
		name    string
		mask    *dto.ImageReference
		wantErr string
	}{
		{
			name:    "neither field",
			mask:    &dto.ImageReference{},
			wantErr: "exactly one",
		},
		{
			name: "both fields",
			mask: &dto.ImageReference{
				FileID:   testStringPtr("file-123"),
				ImageURL: testStringPtr("https://example.com/mask.png"),
			},
			wantErr: "exactly one",
		},
		{
			name: "invalid image url",
			mask: &dto.ImageReference{
				ImageURL: testStringPtr("not-a-url"),
			},
			wantErr: "mask.image_url",
		},
		{
			name: "empty file id",
			mask: &dto.ImageReference{
				FileID: testStringPtr(" "),
			},
			wantErr: "mask.file_id",
		},
		{
			name: "empty image url",
			mask: &dto.ImageReference{
				ImageURL: testStringPtr(" "),
			},
			wantErr: "mask.image_url",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &dto.GenerateImageRequest{
				Model:  "gpt-image-1",
				Prompt: "draw a cat",
				Mask:   test.mask,
			}
			err := ValidateGenerateImageRequest(req)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestGenerateImageRawMessageHelpers(t *testing.T) {
	outputFormatRaw := stringPtrToRawMessage(testStringPtr("png"))
	var outputFormat string
	if err := common.Unmarshal(outputFormatRaw, &outputFormat); err != nil {
		t.Fatalf("unmarshal output_format: %v", err)
	}
	if outputFormat != "png" {
		t.Fatalf("outputFormat = %q, want png", outputFormat)
	}

	maskRaw := imageReferenceToRawMessage(&dto.ImageReference{FileID: testStringPtr("file-123")})
	var mask map[string]string
	if err := common.Unmarshal(maskRaw, &mask); err != nil {
		t.Fatalf("unmarshal mask: %v", err)
	}
	if mask["file_id"] != "file-123" {
		t.Fatalf("mask[file_id] = %q, want file-123", mask["file_id"])
	}
	if _, ok := mask["image_url"]; ok {
		t.Fatalf("mask should not include empty image_url: %v", mask)
	}
}

func TestMappedAsyncImageRequest(t *testing.T) {
	req := &dto.ImageRequest{Model: "gpt-image-2-c"}
	mappedReq := mappedAsyncImageRequest(req, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-image-2"},
	})
	if req.Model != "gpt-image-2-c" {
		t.Fatalf("original Model = %q, want unchanged original model", req.Model)
	}
	if mappedReq == req {
		t.Fatalf("mapped request should be a copy, got same pointer")
	}
	if mappedReq.Model != "gpt-image-2" {
		t.Fatalf("mapped Model = %q, want mapped upstream model", mappedReq.Model)
	}

	mappedReq = mappedAsyncImageRequest(req, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	})
	if mappedReq.Model != "gpt-image-2-c" {
		t.Fatalf("empty upstream model should not overwrite request model, got %q", mappedReq.Model)
	}

	mappedReq = mappedAsyncImageRequest(req, &relaycommon.RelayInfo{})
	if mappedReq.Model != "gpt-image-2-c" {
		t.Fatalf("nil channel meta should not overwrite request model, got %q", mappedReq.Model)
	}

	mappedReq = mappedAsyncImageRequest(nil, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-image-2"},
	})
	if mappedReq != nil {
		t.Fatalf("nil image request should return nil, got %#v", mappedReq)
	}
}

func TestAsyncOpenAIImageRelayMode(t *testing.T) {
	if got := asyncOpenAIImageRelayMode(&dto.ImageRequest{}); got != relayconstant.RelayModeImagesGenerations {
		t.Fatalf("empty request relay mode = %d, want generations", got)
	}

	imagesRaw, err := common.Marshal([]string{"data:image/png;base64,AAAA"})
	if err != nil {
		t.Fatalf("marshal images: %v", err)
	}
	if got := asyncOpenAIImageRelayMode(&dto.ImageRequest{Images: imagesRaw}); got != relayconstant.RelayModeImagesEdits {
		t.Fatalf("images relay mode = %d, want edits", got)
	}

	imageRaw, err := common.Marshal("data:image/png;base64,AAAA")
	if err != nil {
		t.Fatalf("marshal image: %v", err)
	}
	if got := asyncOpenAIImageRelayMode(&dto.ImageRequest{Image: imageRaw}); got != relayconstant.RelayModeImagesEdits {
		t.Fatalf("image relay mode = %d, want edits", got)
	}

	maskRaw, err := common.Marshal(map[string]string{"image_url": "data:image/png;base64,BBBB"})
	if err != nil {
		t.Fatalf("marshal mask: %v", err)
	}
	if got := asyncOpenAIImageRelayMode(&dto.ImageRequest{Mask: maskRaw}); got != relayconstant.RelayModeImagesEdits {
		t.Fatalf("mask relay mode = %d, want edits", got)
	}
}

func TestAsyncImageRequestURLPath(t *testing.T) {
	if got := asyncImageRequestURLPath(relayconstant.RelayModeImagesGenerations, "gpt-image-2"); got != "/v1/images/generations" {
		t.Fatalf("generations path = %q", got)
	}
	if got := asyncImageRequestURLPath(relayconstant.RelayModeImagesEdits, "gpt-image-2"); got != "/v1/images/edits" {
		t.Fatalf("edits path = %q", got)
	}
	if got := asyncImageRequestURLPath(relayconstant.RelayModeGemini, "gemini-3-pro-image"); got != "/v1beta/models/gemini-3-pro-image:generateContent" {
		t.Fatalf("gemini path = %q", got)
	}
}

func TestPrepareAsyncOpenAIImageRequestMapsImagesToImage(t *testing.T) {
	imagesRaw, err := common.Marshal([]string{"data:image/png;base64,AAAA"})
	if err != nil {
		t.Fatalf("marshal images: %v", err)
	}
	req := &dto.ImageRequest{
		Model:  "gpt-image-2-c",
		Images: imagesRaw,
	}
	upstreamReq := prepareAsyncOpenAIImageRequest(req, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-image-2"},
	})

	if req.Model != "gpt-image-2-c" || len(req.Image) != 0 || len(req.Images) == 0 {
		t.Fatalf("original request should stay unchanged: %#v", req)
	}
	if upstreamReq.Model != "gpt-image-2" {
		t.Fatalf("upstream model = %q, want mapped model", upstreamReq.Model)
	}
	var image string
	if err := common.Unmarshal(upstreamReq.Image, &image); err != nil {
		t.Fatalf("unmarshal upstream image: %v", err)
	}
	if image != "data:image/png;base64,AAAA" {
		t.Fatalf("upstream image = %q", image)
	}
	if len(upstreamReq.Images) != 0 {
		t.Fatalf("upstream images should be cleared after mapping to image: %s", string(upstreamReq.Images))
	}
}

func TestPrepareAsyncOpenAIImageRequestKeepsMultipleImages(t *testing.T) {
	imagesRaw, err := common.Marshal([]string{
		"data:image/png;base64,AAAA",
		"data:image/png;base64,BBBB",
	})
	if err != nil {
		t.Fatalf("marshal images: %v", err)
	}
	upstreamReq := prepareAsyncOpenAIImageRequest(&dto.ImageRequest{Images: imagesRaw}, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
	})

	var images []string
	if err := common.Unmarshal(upstreamReq.Image, &images); err != nil {
		t.Fatalf("unmarshal upstream image array: %v", err)
	}
	if len(images) != 2 || images[0] != "data:image/png;base64,AAAA" || images[1] != "data:image/png;base64,BBBB" {
		t.Fatalf("upstream image array = %#v", images)
	}
	if !json.Valid(upstreamReq.Image) {
		t.Fatalf("upstream image should be valid json: %s", string(upstreamReq.Image))
	}
}

func TestBuildAsyncOpenAIImageEditMultipartBody(t *testing.T) {
	imageRaw, err := common.Marshal("data:image/png;base64,AQID")
	if err != nil {
		t.Fatalf("marshal image: %v", err)
	}
	outputFormatRaw, err := common.Marshal("png")
	if err != nil {
		t.Fatalf("marshal output format: %v", err)
	}
	req := &dto.ImageRequest{
		Model:        "gpt-image-2",
		Prompt:       "draw a cat",
		Size:         "auto",
		Quality:      "auto",
		OutputFormat: outputFormatRaw,
		Image:        imageRaw,
	}
	c := &gin.Context{Request: &http.Request{Header: make(http.Header)}}

	body, err := buildAsyncOpenAIImageEditMultipartBody(c, req)
	if err != nil {
		t.Fatalf("build multipart body: %v", err)
	}
	contentType := c.Request.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type %q: %v", contentType, err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("content type = %q, want multipart/form-data", mediaType)
	}

	form := readMultipartForm(t, body, params["boundary"])
	if got := form.Value["model"]; len(got) != 1 || got[0] != "gpt-image-2" {
		t.Fatalf("model field = %#v", got)
	}
	if got := form.Value["prompt"]; len(got) != 1 || got[0] != "draw a cat" {
		t.Fatalf("prompt field = %#v", got)
	}
	if got := form.Value["output_format"]; len(got) != 1 || got[0] != "png" {
		t.Fatalf("output_format field = %#v", got)
	}
	imageFiles := form.File["image"]
	if len(imageFiles) != 1 {
		t.Fatalf("image files = %d, want 1", len(imageFiles))
	}
	file, err := imageFiles[0].Open()
	if err != nil {
		t.Fatalf("open image file: %v", err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read image file: %v", err)
	}
	if !bytes.Equal(data, []byte{1, 2, 3}) {
		t.Fatalf("image bytes = %v", data)
	}
}

func readMultipartForm(t *testing.T, body *bytes.Buffer, boundary string) *multipart.Form {
	t.Helper()
	if boundary == "" {
		t.Fatalf("multipart boundary is empty")
	}
	reader := multipart.NewReader(bytes.NewReader(body.Bytes()), boundary)
	form, err := reader.ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("read multipart form: %v", err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form
}

func TestPrepareGenerateImageResultsPreservesUpstreamURL(t *testing.T) {
	oldUpload := uploadGenerateImageBase64
	uploadGenerateImageBase64 = func(mimeType, base64Data, compression, requestHost string) (string, error) {
		t.Fatalf("upload should not be called for upstream URL")
		return "", nil
	}
	defer func() { uploadGenerateImageBase64 = oldUpload }()

	images, err := prepareGenerateImageResults([]dto.GenerateImageData{
		{Url: "https://upstream.example/image.png"},
	}, "", "api.o1key.cn")
	if err != nil {
		t.Fatalf("prepareGenerateImageResults returned error: %v", err)
	}
	if len(images) != 1 || images[0].Url != "https://upstream.example/image.png" {
		t.Fatalf("images = %#v, want upstream URL preserved", images)
	}
	if images[0].B64Json != "" {
		t.Fatalf("B64Json should be empty in final URL result: %#v", images[0])
	}
}

func TestPrepareGenerateImageResultsUploadsBase64(t *testing.T) {
	oldUpload := uploadGenerateImageBase64
	var gotMimeType, gotBase64, gotCompression, gotRequestHost string
	uploadGenerateImageBase64 = func(mimeType, base64Data, compression, requestHost string) (string, error) {
		gotMimeType = mimeType
		gotBase64 = base64Data
		gotCompression = compression
		gotRequestHost = requestHost
		return "https://img.o1key.cn/uploads/oss/generated.png", nil
	}
	defer func() { uploadGenerateImageBase64 = oldUpload }()

	images, err := prepareGenerateImageResults([]dto.GenerateImageData{
		{B64Json: "AQID", MimeType: "image/png"},
	}, "origin", "api.o1key.cn")
	if err != nil {
		t.Fatalf("prepareGenerateImageResults returned error: %v", err)
	}
	if gotMimeType != "image/png" || gotBase64 != "AQID" || gotCompression != "origin" {
		t.Fatalf("upload args = (%q, %q, %q), want image/png, AQID, origin", gotMimeType, gotBase64, gotCompression)
	}
	if gotRequestHost != "api.o1key.cn" {
		t.Fatalf("requestHost = %q, want api.o1key.cn", gotRequestHost)
	}
	if len(images) != 1 || images[0].Url != "https://img.o1key.cn/uploads/oss/generated.png" {
		t.Fatalf("images = %#v, want uploaded URL", images)
	}
	if images[0].B64Json != "" {
		t.Fatalf("B64Json should be empty after upload: %#v", images[0])
	}
}
