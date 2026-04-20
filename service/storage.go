package service

import (
	"context"
	"fmt"
	"os"
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
