package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

var r2Client *s3.Client

func r2S3Client() *s3.Client {
	if r2Client != nil {
		return r2Client
	}
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")

	r2Client = s3.New(s3.Options{
		BaseEndpoint: aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)),
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
	})
	return r2Client
}

// UploadBase64ImageToR2 decodes a base64 image, uploads it to R2, and returns the public URL.
func UploadBase64ImageToR2(mimeType, base64Data string) (string, error) {
	bucket := os.Getenv("R2_BUCKET")
	baseURL := strings.TrimRight(os.Getenv("R2_PUBLIC_BASE_URL"), "/")

	imgBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	exts, err := mime.ExtensionsByType(mimeType)
	ext := ".bin"
	if err == nil && len(exts) > 0 {
		ext = exts[len(exts)-1]
	}

	key := fmt.Sprintf("images/%s%s", uuid.New().String(), ext)

	_, err = r2S3Client().PutObject(context.Background(), &s3.PutObjectInput{
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
