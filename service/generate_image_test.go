package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func testStringPtr(value string) *string {
	return &value
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
