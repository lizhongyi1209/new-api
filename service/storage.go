package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
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
}

func GeneratePresignedUploadURL(filename, contentType string) (*PresignResult, error) {
	_, presignClient := getR2Client()

	bucket := os.Getenv("R2_BUCKET")
	publicBase := os.Getenv("R2_PUBLIC_BASE_URL")

	id := uuid.New().String()
	objectKey := fmt.Sprintf("uploads/%s_%s", id, filename)

	req, err := presignClient.PresignPutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}, func(o *s3.PresignOptions) {
		o.Expires = 5 * time.Minute
	})
	if err != nil {
		return nil, err
	}

	return &PresignResult{
		UploadURL: req.URL,
		PublicURL: fmt.Sprintf("%s/%s", publicBase, objectKey),
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

// convertPNGToWebP converts PNG bytes to WebP using the cwebp CLI tool.
// quality is 0-100, 85 is recommended for visually lossless results.
func convertPNGToWebP(pngBytes []byte, quality int) ([]byte, error) {
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

	if _, err := inFile.Write(pngBytes); err != nil {
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

// UploadBase64ImageToR2Compressed is like UploadBase64ImageToR2 but converts to WebP when
// compression is ImageCompressionWebP. Other values or empty string upload as PNG.
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

	if compression == ImageCompressionWebP {
		webpBytes, err := convertPNGToWebP(imgBytes, 85)
		if err != nil {
			return "", fmt.Errorf("webp conversion failed: %w", err)
		}
		uploadBytes = webpBytes
		ext = "webp"
		contentType = "image/webp"
	} else {
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
