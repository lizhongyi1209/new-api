package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif" // register GIF decoder
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"net"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"golang.org/x/image/webp"
)

var r2Client *s3.Client
var r2PresignClient *s3.PresignClient
var ossClient *s3.Client
var ossPresignClient *s3.PresignClient

const (
	ImageStorageProviderR2        = "r2"
	ImageStorageProviderAliyunOSS = "aliyun_oss"
)

const (
	defaultAliyunOSSStorageHosts = "api.o1key.cn"
	defaultR2StorageHosts        = "cf-api.o1key.cn,cf-api.o1key.com"
)

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

type OSSPresignResult struct {
	Method    string            `json:"method"`
	UploadURL string            `json:"upload_url"`
	Headers   map[string]string `json:"headers,omitempty"`
	PublicURL string            `json:"public_url"`
	ObjectKey string            `json:"object_key"`
	ExpiresAt int64             `json:"expires_at"`
	Provider  string            `json:"provider"`
}

func GeneratePresignedUploadURLForHost(requestHost, filename, contentType string, maxSize int64) (any, error) {
	if SelectImageStorageProvider(requestHost) == ImageStorageProviderAliyunOSS {
		return GenerateOSSPresignedUploadURL(filename, contentType, maxSize)
	}
	return GeneratePresignedUploadURL(filename, contentType, maxSize)
}

func SelectImageStorageProvider(requestHost string) string {
	host := normalizeRequestHost(requestHost)
	ossHosts := firstNonEmptyString(os.Getenv("ALIYUN_OSS_STORAGE_HOSTS"), defaultAliyunOSSStorageHosts)
	if hostMatchesCSV(host, ossHosts) {
		return ImageStorageProviderAliyunOSS
	}

	r2Hosts := firstNonEmptyString(os.Getenv("R2_STORAGE_HOSTS"), defaultR2StorageHosts)
	if hostMatchesCSV(host, r2Hosts) {
		return ImageStorageProviderR2
	}

	return ImageStorageProviderR2
}

func UploadBase64ImageToHostStorageCompressed(mimeType, base64Data, compression, requestHost string) (string, error) {
	switch SelectImageStorageProvider(requestHost) {
	case ImageStorageProviderAliyunOSS:
		return UploadBase64ImageToOSSCompressed(mimeType, base64Data, compression)
	default:
		return UploadBase64ImageToR2Compressed(mimeType, base64Data, compression)
	}
}

func normalizeRequestHost(requestHost string) string {
	host := strings.TrimSpace(strings.ToLower(requestHost))
	if host == "" {
		return ""
	}
	if parsed, err := url.Parse(host); err == nil && parsed.Host != "" {
		host = parsed.Host
	}
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	}
	return strings.Trim(host, "[]")
}

func hostMatchesCSV(host, csv string) bool {
	if host == "" {
		return false
	}
	for _, item := range strings.Split(csv, ",") {
		item = normalizeRequestHost(item)
		if item == "" {
			continue
		}
		if host == item {
			return true
		}
	}
	return false
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

func getAliyunOSSClient() (*s3.Client, *s3.PresignClient, error) {
	if ossClient != nil {
		return ossClient, ossPresignClient, nil
	}

	accessKeyID := firstNonEmptyEnv("ALIYUN_OSS_ACCESS_KEY_ID", "OSS_ACCESS_KEY_ID")
	accessKeySecret := firstNonEmptyEnv("ALIYUN_OSS_ACCESS_KEY_SECRET", "OSS_ACCESS_KEY_SECRET")
	region := firstNonEmptyEnv("ALIYUN_OSS_REGION", "OSS_REGION")
	rawEndpoint := firstNonEmptyEnv("ALIYUN_OSS_ENDPOINT", "OSS_ENDPOINT")
	if region == "" {
		region = inferAliyunOSSRegion(rawEndpoint)
	}
	endpoint := normalizeAliyunOSSEndpoint(rawEndpoint, region)
	if endpoint == "" && region != "" {
		endpoint = fmt.Sprintf("https://s3.oss-%s.aliyuncs.com", region)
	}

	if accessKeyID == "" || accessKeySecret == "" || region == "" || endpoint == "" {
		return nil, nil, fmt.Errorf("missing Aliyun OSS config: require ALIYUN_OSS_ACCESS_KEY_ID, ALIYUN_OSS_ACCESS_KEY_SECRET, ALIYUN_OSS_REGION and optional ALIYUN_OSS_ENDPOINT")
	}

	cfg := aws.Config{
		Credentials: credentials.NewStaticCredentialsProvider(accessKeyID, accessKeySecret, ""),
		Region:      region,
	}
	ossClient = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = strings.EqualFold(os.Getenv("ALIYUN_OSS_FORCE_PATH_STYLE"), "true")
	})
	ossPresignClient = s3.NewPresignClient(ossClient)
	return ossClient, ossPresignClient, nil
}

func GenerateOSSPresignedUploadURL(filename, contentType string, maxSize int64) (*OSSPresignResult, error) {
	_, presignClient, err := getAliyunOSSClient()
	if err != nil {
		return nil, err
	}

	bucket := firstNonEmptyEnv("ALIYUN_OSS_BUCKET", "OSS_BUCKET")
	publicBase := normalizeHTTPBaseURL(firstNonEmptyEnv("ALIYUN_OSS_PUBLIC_BASE_URL", "OSS_PUBLIC_BASE_URL"))
	if bucket == "" || publicBase == "" {
		return nil, fmt.Errorf("missing Aliyun OSS config: require ALIYUN_OSS_BUCKET and ALIYUN_OSS_PUBLIC_BASE_URL")
	}

	id := uuid.New().String()
	objectKey := fmt.Sprintf("uploads/oss/%s_%s", id, sanitizeUploadFilename(filename))

	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}
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

	headers := make(map[string]string)
	for k, values := range req.SignedHeader {
		if len(values) > 0 {
			headers[k] = values[0]
		}
	}
	if _, ok := headers["Content-Type"]; !ok && contentType != "" {
		headers["Content-Type"] = contentType
	}

	method := req.Method
	if method == "" {
		method = "PUT"
	}

	return &OSSPresignResult{
		Method:    method,
		UploadURL: req.URL,
		Headers:   headers,
		PublicURL: fmt.Sprintf("%s/%s", publicBase, objectKey),
		ObjectKey: objectKey,
		ExpiresAt: time.Now().Add(expiresIn).Unix(),
		Provider:  "aliyun_oss",
	}, nil
}

// UploadBase64ImageToOSSCompressed uploads a generated base64 image to Aliyun OSS and returns its public URL.
func UploadBase64ImageToOSSCompressed(mimeType, base64Data, compression string) (string, error) {
	client, _, err := getAliyunOSSClient()
	if err != nil {
		return "", err
	}

	bucket := firstNonEmptyEnv("ALIYUN_OSS_BUCKET", "OSS_BUCKET")
	publicBase := normalizeHTTPBaseURL(firstNonEmptyEnv("ALIYUN_OSS_PUBLIC_BASE_URL", "OSS_PUBLIC_BASE_URL"))
	if bucket == "" || publicBase == "" {
		return "", fmt.Errorf("missing Aliyun OSS config: require ALIYUN_OSS_BUCKET and ALIYUN_OSS_PUBLIC_BASE_URL")
	}

	uploadBytes, ext, contentType, err := prepareCompressedImageUpload(mimeType, base64Data, compression)
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("uploads/oss/%s.%s", uuid.New().String(), ext)
	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(uploadBytes),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("aliyun oss upload failed: %w", err)
	}

	return fmt.Sprintf("%s/%s", publicBase, key), nil
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func normalizeAliyunOSSEndpoint(rawEndpoint, region string) string {
	endpoint := normalizeHTTPBaseURL(rawEndpoint)
	if endpoint == "" {
		return ""
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return endpoint
	}

	host := parsed.Host
	if strings.HasPrefix(host, "oss-") && strings.HasSuffix(host, ".aliyuncs.com") {
		parsed.Host = "s3." + host
		return strings.TrimRight(parsed.String(), "/")
	}
	if region != "" && host == fmt.Sprintf("oss-%s.aliyuncs.com", region) {
		parsed.Host = fmt.Sprintf("s3.oss-%s.aliyuncs.com", region)
		return strings.TrimRight(parsed.String(), "/")
	}
	return strings.TrimRight(parsed.String(), "/")
}

func inferAliyunOSSRegion(rawEndpoint string) string {
	host := strings.TrimSpace(rawEndpoint)
	if host == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(host), "http://") &&
		!strings.HasPrefix(strings.ToLower(host), "https://") {
		host = "https://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil || parsed.Host == "" {
		return ""
	}
	host = parsed.Host
	host = strings.TrimPrefix(host, "s3.")
	if !strings.HasPrefix(host, "oss-") || !strings.HasSuffix(host, ".aliyuncs.com") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(host, "oss-"), ".aliyuncs.com")
}

func normalizeHTTPBaseURL(value string) string {
	value = strings.TrimSpace(strings.TrimRight(value, "/"))
	if value == "" {
		return ""
	}
	lowerValue := strings.ToLower(value)
	if !strings.HasPrefix(lowerValue, "http://") && !strings.HasPrefix(lowerValue, "https://") {
		value = "https://" + value
	}
	return strings.TrimRight(value, "/")
}

func sanitizeUploadFilename(filename string) string {
	name := path.Base(strings.ReplaceAll(filename, "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "upload"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, name)
}

func prepareCompressedImageUpload(mimeType, base64Data, compression string) ([]byte, string, string, error) {
	imgBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, "", "", fmt.Errorf("base64 decode failed: %w", err)
	}

	switch compression {
	case ImageCompressionWebP, ImageCompressionJPG:
		jpgBytes, err := convertToJPEG(imgBytes, 95)
		if err != nil {
			return nil, "", "", fmt.Errorf("jpeg conversion failed: %w", err)
		}
		return jpgBytes, "jpg", "image/jpeg", nil
	case ImageCompressionOrigin, "":
		ext, contentType := detectImageUploadFormat(imgBytes, mimeType)
		return imgBytes, ext, contentType, nil
	default:
		return imgBytes, "png", firstNonEmptyString(mimeType, "image/png"), nil
	}
}

func detectImageUploadFormat(imgBytes []byte, mimeType string) (string, string) {
	_, format, _ := image.DecodeConfig(bytes.NewReader(imgBytes))
	if format == "" {
		if _, err := webp.DecodeConfig(bytes.NewReader(imgBytes)); err == nil {
			format = "webp"
		}
	}
	if format == "" {
		switch strings.ToLower(strings.TrimSpace(mimeType)) {
		case "image/jpeg", "image/jpg":
			format = "jpg"
		case "image/webp":
			format = "webp"
		case "image/gif":
			format = "gif"
		default:
			format = "png"
		}
	}
	if format == "jpeg" {
		format = "jpg"
	}

	contentType := strings.ToLower(strings.TrimSpace(mimeType))
	if contentType == "" {
		switch format {
		case "jpg":
			contentType = "image/jpeg"
		case "webp":
			contentType = "image/webp"
		case "gif":
			contentType = "image/gif"
		default:
			contentType = "image/png"
		}
	}
	return format, contentType
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
