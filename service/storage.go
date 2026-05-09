package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	_ "image/gif"  // register GIF decoder
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"golang.org/x/image/webp"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

var r2Client *s3.Client
var r2PresignClient *s3.PresignClient

func getR2Client() (*s3.Client, *s3.PresignClient) {
	if r2Client != nil {
		return r2Client, r2PresignClient
	}
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")

	cfg := aws.Config{
		Credentials: credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		Region:      "auto",
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	r2Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	})
	r2PresignClient = s3.NewPresignClient(r2Client)
	return r2Client, r2PresignClient
}

type PresignResult struct {
	UploadURL string `json:"upload_url"`
	PublicURL string `json:"public_url"`
	ExpiresAt int64  `json:"expires_at"` // Unix timestamp
}

func GeneratePresignedUploadURL(filename, contentType string, maxSize int64) (*PresignResult, error) {
	_, presignClient := getR2Client()

	bucket := os.Getenv("R2_BUCKET")
	publicBase := os.Getenv("R2_PUBLIC_BASE_URL")

	id := uuid.New().String()
	objectKey := fmt.Sprintf("uploads/%s_%s", id, filename)

	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}

	// Add size limit if specified
	if maxSize > 0 {
		putInput.ContentLength = aws.Int64(maxSize)
	}

	expiresIn := 15 * time.Minute
	req, err := presignClient.PresignPutObject(context.Background(), putInput, func(o *s3.PresignOptions) {
		o.Expires = expiresIn
	})
	if err != nil {
		return nil, err
	}

	return &PresignResult{
		UploadURL: req.URL,
		PublicURL: fmt.Sprintf("%s/%s", publicBase, objectKey),
		ExpiresAt: time.Now().Add(expiresIn).Unix(),
	}, nil
}

// UploadBase64ImageToR2 decodes a base64 image, uploads it to R2, and returns the public URL.
func UploadBase64ImageToR2(mimeType, base64Data string) (string, error) {
	client, _ := getR2Client()
	bucket := os.Getenv("R2_BUCKET")
	baseURL := strings.TrimRight(os.Getenv("R2_PUBLIC_BASE_URL"), "/")

	imgBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	key := fmt.Sprintf("images/%s.png", uuid.New().String())

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(imgBytes),
		ContentType: aws.String(mimeType),
	})
	if err != nil {
		return "", fmt.Errorf("r2 upload failed: %w", err)
	}

	return fmt.Sprintf("%s/%s", baseURL, key), nil
}

// convertToWebP converts image bytes to WebP using the cwebp CLI tool.
// quality is 0-100, 85 is recommended for visually lossless results.
func convertToWebP(imgBytes []byte, quality int) ([]byte, error) {
	inFile, err := os.CreateTemp("", "r2upload_*.png")
	if err != nil {
		return nil, fmt.Errorf("create temp input: %w", err)
	}
	defer os.Remove(inFile.Name())

	outFile, err := os.CreateTemp("", "r2upload_*.webp")
	if err != nil {
		return nil, fmt.Errorf("create temp output: %w", err)
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	if _, err := inFile.Write(imgBytes); err != nil {
		inFile.Close()
		return nil, fmt.Errorf("write temp input: %w", err)
	}
	inFile.Close()

	cmd := exec.Command("cwebp", "-q", strconv.Itoa(quality), inFile.Name(), "-o", outPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cwebp: %w: %s", err, stderr.String())
	}

	webpBytes, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read temp output: %w", err)
	}
	return webpBytes, nil
}

const ImageCompressionWebP = "webp"
const ImageCompressionJPG = "jpg"
const ImageCompressionOrigin = "origin"

// convertToJPEG decodes image bytes in any supported format (PNG, JPEG, GIF, WebP)
// and re-encodes as JPEG with the given quality (1-100). Returns original bytes
// if decoding fails, to avoid data loss.
func convertToJPEG(imgBytes []byte, quality int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		// Try WebP decoder for webp source images
		img, err = webp.Decode(bytes.NewReader(imgBytes))
		if err != nil {
			return imgBytes, nil // Return original bytes if unable to decode
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return imgBytes, nil // Return original bytes if encode fails
	}
	// Only use JPEG result if it's smaller or comparable to original
	result := buf.Bytes()
	if len(result) > len(imgBytes)*3 {
		return imgBytes, nil // Don't replace if JPEG is >3x larger (avoid size blowup)
	}
	return result, nil
}

// UploadBase64ImageToR2Compressed uploads a base64 image to R2 with optional compression.
// compression modes:
//
//	"webp"  - convert to JPEG (quality 95, same as "jpg" mode)
//	"jpg"   - convert to JPEG (quality 95, fallback to original if decode fails)
//	"origin" - keep original format, detect extension
//	default  - store as-is with PNG extension
func UploadBase64ImageToR2Compressed(mimeType, base64Data, compression string) (string, error) {
	client, _ := getR2Client()
	bucket := os.Getenv("R2_BUCKET")
	baseURL := strings.TrimRight(os.Getenv("R2_PUBLIC_BASE_URL"), "/")

	imgBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	var uploadBytes []byte
	var ext, contentType string

	switch compression {
	case ImageCompressionWebP:
		// Changed: webp mode now uses JPEG quality 95 instead of WebP conversion
		jpgBytes, err := convertToJPEG(imgBytes, 95)
		if err != nil {
			return "", fmt.Errorf("jpeg conversion failed: %w", err)
		}
		uploadBytes = jpgBytes
		ext = "jpg"
		contentType = "image/jpeg"
	case ImageCompressionJPG:
		jpgBytes, err := convertToJPEG(imgBytes, 95)
		if err != nil {
			return "", fmt.Errorf("jpeg conversion failed: %w", err)
		}
		uploadBytes = jpgBytes
		ext = "jpg"
		contentType = "image/jpeg"
	case ImageCompressionOrigin:
		// Detect actual format from binary header
		_, format, _ := image.DecodeConfig(bytes.NewReader(imgBytes))
		if format == "" {
			if _, err := webp.DecodeConfig(bytes.NewReader(imgBytes)); err == nil {
				format = "webp"
			}
		}
		if format == "" {
			format = "png" // fallback
		}
		uploadBytes = imgBytes
		ext = format
		contentType = mimeType
	default:
		uploadBytes = imgBytes
		ext = "png"
		contentType = mimeType
	}

	key := fmt.Sprintf("images/%s.%s", uuid.New().String(), ext)

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(uploadBytes),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("r2 upload failed: %w", err)
	}

	return fmt.Sprintf("%s/%s", baseURL, key), nil
}
