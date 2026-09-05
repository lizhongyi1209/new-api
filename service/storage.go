package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"  // register GIF decoder
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

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

const maxVideoOutputBytes int64 = 512 * 1024 * 1024

const generatedImageCacheControl = "public, max-age=31536000, immutable"

type objectUploader interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

const (
	ImageStorageProviderR2        = "r2"
	ImageStorageProviderAliyunOSS = "aliyun_oss"
	ImageStorageProviderLocal     = "local"
)

const (
	defaultAliyunOSSStorageHosts = ""
	defaultR2StorageHosts        = "cf-api.o1key.cn,cf-api.o1key.com"
	defaultLocalStorageHosts     = "api.o1key.cn"
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
	UploadURL string            `json:"upload_url"`
	PublicURL string            `json:"public_url"`
	ExpiresAt int64             `json:"expires_at"` // Unix timestamp
	Headers   map[string]string `json:"headers,omitempty"`
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
	provider := SelectImageStorageProvider(requestHost)
	switch provider {
	case ImageStorageProviderAliyunOSS:
		return GenerateOSSPresignedUploadURL(filename, contentType, maxSize)
	case ImageStorageProviderLocal:
		return GenerateLocalPresignedUploadURL(filename, contentType, maxSize)
	default:
		return GeneratePresignedUploadURL(filename, contentType, maxSize)
	}
}

// IsAliyunOSSBlocked reports whether Aliyun OSS uploads are administratively
// disabled. When true, every storage path that would otherwise target OSS is
// transparently redirected to R2. Toggle via the DISABLE_ALIYUN_OSS env var.
func IsAliyunOSSBlocked() bool {
	return common.GetEnvOrDefaultBool("DISABLE_ALIYUN_OSS", false)
}

func SelectImageStorageProvider(requestHost string) string {
	host := normalizeRequestHost(requestHost)

	// Check local storage first (api.o1key.cn)
	localHosts := firstNonEmptyString(os.Getenv("LOCAL_STORAGE_HOSTS"), defaultLocalStorageHosts)
	if hostMatchesCSV(host, localHosts) {
		return ImageStorageProviderLocal
	}

	ossHosts := firstNonEmptyString(os.Getenv("ALIYUN_OSS_STORAGE_HOSTS"), defaultAliyunOSSStorageHosts)
	if ossHosts != "" && hostMatchesCSV(host, ossHosts) {
		// Kill-switch: redirect would-be OSS uploads to R2, but leave local
		// and R2 host routing untouched. Toggle via DISABLE_ALIYUN_OSS.
		if IsAliyunOSSBlocked() {
			return ImageStorageProviderR2
		}
		return ImageStorageProviderAliyunOSS
	}

	r2Hosts := firstNonEmptyString(os.Getenv("R2_STORAGE_HOSTS"), defaultR2StorageHosts)
	if hostMatchesCSV(host, r2Hosts) {
		return ImageStorageProviderR2
	}

	return ImageStorageProviderLocal
}

func UploadBase64ImageToHostStorage(mimeType, base64Data, requestHost string) (string, error) {
	switch SelectImageStorageProvider(requestHost) {
	case ImageStorageProviderAliyunOSS:
		return UploadBase64ImageToOSS(mimeType, base64Data)
	case ImageStorageProviderLocal:
		return UploadBase64ImageToLocal(mimeType, base64Data)
	default:
		return UploadBase64ImageToR2(mimeType, base64Data)
	}
}

func UploadBase64ImageWithOutputStrategy(mimeType, base64Data, strategy, requestHost string) (string, error) {
	switch strategy {
	case dto.ImageOutputStrategyOSS:
		return UploadBase64ImageToOSS(mimeType, base64Data)
	case dto.ImageOutputStrategyR2:
		return UploadBase64ImageToR2(mimeType, base64Data)
	case dto.ImageOutputStrategyLocalTemp:
		return UploadBase64ImageToTemporaryOutput(mimeType, base64Data, "")
	case dto.ImageOutputStrategyLocalTempCF:
		return UploadBase64ImageToTemporaryOutput(mimeType, base64Data, "https://cf-api.o1key.com")
	case dto.ImageOutputStrategyLocalTempESA:
		return UploadBase64ImageToTemporaryOutput(mimeType, base64Data, "https://api.o1key.cn")
	default:
		return UploadBase64ImageToHostStorage(mimeType, base64Data, requestHost)
	}
}

// UploadImageBytesWithOutputStrategy uploads already-decoded image bytes without
// routing OSS-bound remote images through an unnecessary Base64 encode/decode cycle.
func UploadImageBytesWithOutputStrategy(ctx context.Context, mimeType string, imageBytes []byte, strategy, requestHost string) (string, error) {
	provider := ""
	if strategy == "" {
		provider = SelectImageStorageProvider(requestHost)
	}
	if strategy == dto.ImageOutputStrategyOSS || provider == ImageStorageProviderAliyunOSS {
		return UploadImageBytesToOSSContext(ctx, imageBytes)
	}
	return UploadBase64ImageWithOutputStrategy(
		mimeType,
		base64.StdEncoding.EncodeToString(imageBytes),
		strategy,
		requestHost,
	)
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
	return generateR2PresignedUploadURL("uploads", filename, contentType, maxSize)
}

func GeneratePublicR2PresignedUploadURL(clientID, filename, contentType string, size int64) (*PresignResult, error) {
	prefix := "downstream-uploads/" + clientID
	return generateR2PresignedUploadURL(prefix, filename, contentType, size)
}

func generateR2PresignedUploadURL(prefix, filename, contentType string, maxSize int64) (*PresignResult, error) {
	_, presignClient := getR2Client()

	bucket := os.Getenv("R2_BUCKET")
	publicBase := os.Getenv("R2_PUBLIC_BASE_URL")

	id := uuid.New().String()
	objectKey := fmt.Sprintf("%s/%s_%s", strings.Trim(prefix, "/"), id, filename)

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

	headers := map[string]string{
		"Content-Type": contentType,
	}
	if maxSize > 0 {
		headers["Content-Length"] = strconv.FormatInt(maxSize, 10)
	}

	return &PresignResult{
		UploadURL: req.URL,
		PublicURL: fmt.Sprintf("%s/%s", publicBase, objectKey),
		ExpiresAt: time.Now().Add(expiresIn).Unix(),
		Headers:   headers,
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
	// Kill-switch: redirect explicit OSS presign requests to R2, wrapping the
	// R2 result in the OSS response shape so existing clients keep working.
	if IsAliyunOSSBlocked() {
		r2Result, err := GeneratePresignedUploadURL(filename, contentType, maxSize)
		if err != nil {
			return nil, err
		}
		// Derive object_key from public_url so the response matches the OSS shape
		// (R2's PresignResult does not carry it explicitly).
		objectKey := ""
		r2Base := normalizeHTTPBaseURL(os.Getenv("R2_PUBLIC_BASE_URL"))
		if r2Base != "" {
			objectKey = strings.TrimPrefix(r2Result.PublicURL, r2Base+"/")
		}
		return &OSSPresignResult{
			Method:    "PUT",
			UploadURL: r2Result.UploadURL,
			Headers:   r2Result.Headers,
			PublicURL: r2Result.PublicURL,
			ObjectKey: objectKey,
			ExpiresAt: r2Result.ExpiresAt,
			Provider:  ImageStorageProviderR2,
		}, nil
	}

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

// UploadBase64ImageToOSS uploads a generated base64 image to Aliyun OSS without re-encoding it.
func UploadBase64ImageToOSS(mimeType, base64Data string) (string, error) {
	return UploadBase64ImageToOSSContext(context.Background(), mimeType, base64Data)
}

// UploadBase64ImageToOSSContext is the context-aware variant used by request-level tracing.
func UploadBase64ImageToOSSContext(ctx context.Context, _ string, base64Data string) (string, error) {
	uploadBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("decode image base64: %w", err)
	}
	return UploadImageBytesToOSSContext(ctx, uploadBytes)
}

// UploadImageBytesToOSSContext uploads validated image bytes to Aliyun OSS.
// The detected byte format determines the object extension and Content-Type.
func UploadImageBytesToOSSContext(ctx context.Context, uploadBytes []byte) (string, error) {
	client, _, err := getAliyunOSSClient()
	if err != nil {
		return "", err
	}

	bucket := firstNonEmptyEnv("ALIYUN_OSS_BUCKET", "OSS_BUCKET")
	publicBase := normalizeHTTPBaseURL(firstNonEmptyEnv("ALIYUN_OSS_PUBLIC_BASE_URL", "OSS_PUBLIC_BASE_URL"))
	if bucket == "" || publicBase == "" {
		return "", fmt.Errorf("missing Aliyun OSS config: require ALIYUN_OSS_BUCKET and ALIYUN_OSS_PUBLIC_BASE_URL")
	}

	ext, contentType, err := detectImageUploadFormat(uploadBytes)
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("output/%s.%s", uuid.New().String(), ext)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(uploadBytes),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String(generatedImageCacheControl),
	})
	if err != nil {
		return "", fmt.Errorf("aliyun oss upload failed: %w", err)
	}

	return fmt.Sprintf("%s/%s", publicBase, key), nil
}

// GenerateLocalPresignedUploadURL generates a direct upload URL for local storage.
// For local storage, we return a direct POST endpoint that accepts multipart/form-data.
func GenerateLocalPresignedUploadURL(filename, contentType string, maxSize int64) (*OSSPresignResult, error) {
	localPublicBase := normalizeHTTPBaseURL(firstNonEmptyString(os.Getenv("LOCAL_PUBLIC_BASE_URL"), "https://api.o1key.cn"))

	id := uuid.New().String()
	objectKey := fmt.Sprintf("uploads/%s_%s", id, sanitizeUploadFilename(filename))

	uploadURL := fmt.Sprintf("%s/v1/storage/local/upload?object_key=%s", localPublicBase, objectKey)
	publicURL := fmt.Sprintf("%s/upload/%s", localPublicBase, objectKey)

	expiresIn := 15 * time.Minute
	headers := make(map[string]string)
	if contentType != "" {
		headers["Content-Type"] = contentType
	}

	return &OSSPresignResult{
		Method:    "POST",
		UploadURL: uploadURL,
		Headers:   headers,
		PublicURL: publicURL,
		ObjectKey: objectKey,
		ExpiresAt: time.Now().Add(expiresIn).Unix(),
		Provider:  ImageStorageProviderLocal,
	}, nil
}

// UploadBase64ImageToLocal uploads a base64 image to local storage without re-encoding it.
func UploadBase64ImageToLocal(mimeType, base64Data string) (string, error) {
	return UploadBase64ImageToLocalWithCategory(mimeType, base64Data, UploadDirGeneral)
}

// UploadBase64ImageToLocalWithCategory uploads a base64 image to local storage with category.
// category should be one of: UploadDirGeneral, UploadDirElements, UploadDirTemp
func UploadBase64ImageToLocalWithCategory(mimeType, base64Data, category string) (string, error) {
	localPublicBase := normalizeHTTPBaseURL(firstNonEmptyString(os.Getenv("LOCAL_PUBLIC_BASE_URL"), "https://api.o1key.cn"))
	uploadDir := firstNonEmptyString(os.Getenv("LOCAL_UPLOAD_DIR"), "uploads")

	uploadBytes, ext, contentType, err := prepareImageUpload(mimeType, base64Data)
	if err != nil {
		return "", err
	}

	objectKey := fmt.Sprintf("%s/%s.%s", category, uuid.New().String(), ext)
	filePath := fmt.Sprintf("%s/%s", uploadDir, objectKey)

	// Ensure directory exists
	if err := os.MkdirAll(fmt.Sprintf("%s/%s", uploadDir, category), 0755); err != nil {
		return "", fmt.Errorf("create upload directory failed: %w", err)
	}

	// Write file
	if err := os.WriteFile(filePath, uploadBytes, 0644); err != nil {
		return "", fmt.Errorf("local storage write failed: %w", err)
	}

	_ = contentType // contentType is determined but not stored as metadata for local files
	return fmt.Sprintf("%s/upload/%s", localPublicBase, objectKey), nil
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

func prepareImageUpload(_ string, base64Data string) ([]byte, string, string, error) {
	imgBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, "", "", fmt.Errorf("base64 decode failed: %w", err)
	}

	ext, contentType, err := detectImageUploadFormat(imgBytes)
	if err != nil {
		return nil, "", "", err
	}
	return imgBytes, ext, contentType, nil
}

func detectImageUploadFormat(imgBytes []byte) (string, string, error) {
	if len(imgBytes) == 0 {
		return "", "", fmt.Errorf("image data is empty")
	}

	_, format, imageErr := image.DecodeConfig(bytes.NewReader(imgBytes))
	if format == "" {
		if _, err := webp.DecodeConfig(bytes.NewReader(imgBytes)); err == nil {
			format = "webp"
		}
	}
	if format == "" {
		return "", "", fmt.Errorf("unsupported or invalid image data: %w", imageErr)
	}
	if format == "jpeg" {
		format = "jpg"
	}

	contentType := ""
	switch format {
	case "png":
		contentType = "image/png"
	case "jpg":
		contentType = "image/jpeg"
	case "webp":
		contentType = "image/webp"
	case "gif":
		contentType = "image/gif"
	default:
		return "", "", fmt.Errorf("unsupported image format: %s", format)
	}
	return format, contentType, nil
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

	uploadBytes, ext, contentType, err := prepareImageUpload(mimeType, base64Data)
	if err != nil {
		return "", err
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

func cacheVideoOutputToR2(ctx context.Context, sourceURL string, httpClient *http.Client, headers http.Header) (string, error) {
	client, _ := getR2Client()
	bucket := strings.TrimSpace(os.Getenv("R2_BUCKET"))
	publicBase := normalizeHTTPBaseURL(os.Getenv("R2_PUBLIC_BASE_URL"))
	if bucket == "" || publicBase == "" {
		return "", fmt.Errorf("missing R2_BUCKET or R2_PUBLIC_BASE_URL")
	}
	return cacheVideoOutput(ctx, sourceURL, httpClient, headers, client, bucket, publicBase, "R2")
}

func cacheVideoOutputToOSS(ctx context.Context, source string, httpClient *http.Client, headers http.Header) (string, error) {
	client, _, err := getAliyunOSSClient()
	if err != nil {
		return "", err
	}
	bucket := strings.TrimSpace(firstNonEmptyEnv("ALIYUN_OSS_BUCKET", "OSS_BUCKET"))
	publicBase := normalizeHTTPBaseURL(firstNonEmptyEnv("ALIYUN_OSS_PUBLIC_BASE_URL", "OSS_PUBLIC_BASE_URL"))
	if bucket == "" || publicBase == "" {
		return "", fmt.Errorf("missing Aliyun OSS config: require ALIYUN_OSS_BUCKET and ALIYUN_OSS_PUBLIC_BASE_URL")
	}
	return cacheVideoOutput(ctx, source, httpClient, headers, client, bucket, publicBase, "Aliyun OSS")
}

func cacheVideoOutput(ctx context.Context, source string, httpClient *http.Client, headers http.Header, uploader objectUploader, bucket, publicBase, provider string) (string, error) {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(source, publicBase+"/output/") {
		return source, nil
	}

	var sourceBody io.ReadCloser
	contentType := ""
	contentLength := int64(-1)
	sourcePath := ""

	if strings.HasPrefix(source, "data:") {
		comma := strings.IndexByte(source, ',')
		if comma < 0 {
			return "", fmt.Errorf("invalid video data URL")
		}
		metadata := source[len("data:"):comma]
		parts := strings.Split(metadata, ";")
		if len(parts) < 2 || !strings.EqualFold(parts[len(parts)-1], "base64") {
			return "", fmt.Errorf("video data URL must use base64 encoding")
		}
		contentType = parts[0]
		encoded := source[comma+1:]
		if int64(base64.StdEncoding.DecodedLen(len(encoded))) > maxVideoOutputBytes {
			return "", fmt.Errorf("video output exceeds %d bytes", maxVideoOutputBytes)
		}
		sourceBody = io.NopCloser(base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded)))
	} else {
		parsedURL, err := url.Parse(source)
		if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
			return "", fmt.Errorf("invalid remote video URL")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return "", fmt.Errorf("create remote video request: %w", err)
		}
		for name, values := range headers {
			for _, value := range values {
				request.Header.Add(name, value)
			}
		}
		response, err := httpClient.Do(request)
		if err != nil {
			return "", fmt.Errorf("download remote video: %w", err)
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			response.Body.Close()
			return "", fmt.Errorf("download remote video returned status %d", response.StatusCode)
		}
		sourceBody = response.Body
		contentType = response.Header.Get("Content-Type")
		contentLength = response.ContentLength
		sourcePath = parsedURL.Path
	}
	defer sourceBody.Close()

	if contentLength > maxVideoOutputBytes {
		return "", fmt.Errorf("video output exceeds %d bytes", maxVideoOutputBytes)
	}

	mediaType, _, _ := mime.ParseMediaType(contentType)
	ext := ""
	switch strings.ToLower(mediaType) {
	case "video/mp4":
		ext = "mp4"
		contentType = "video/mp4"
	case "video/webm":
		ext = "webm"
		contentType = "video/webm"
	case "video/quicktime":
		ext = "mov"
		contentType = "video/quicktime"
	case "video/mpeg":
		ext = "mpeg"
		contentType = "video/mpeg"
	case "video/x-msvideo":
		ext = "avi"
		contentType = "video/x-msvideo"
	case "video/x-matroska":
		ext = "mkv"
		contentType = "video/x-matroska"
	}
	if ext == "" {
		switch strings.ToLower(path.Ext(sourcePath)) {
		case ".mp4":
			ext = "mp4"
			contentType = "video/mp4"
		case ".webm":
			ext = "webm"
			contentType = "video/webm"
		case ".mov":
			ext = "mov"
			contentType = "video/quicktime"
		case ".mpeg", ".mpg":
			ext = "mpeg"
			contentType = "video/mpeg"
		case ".avi":
			ext = "avi"
			contentType = "video/x-msvideo"
		case ".mkv":
			ext = "mkv"
			contentType = "video/x-matroska"
		}
	}
	if ext == "" && (mediaType == "" || strings.EqualFold(mediaType, "application/octet-stream")) {
		ext = "mp4"
		contentType = "video/mp4"
	}
	if ext == "" {
		return "", fmt.Errorf("unsupported video content type: %s", contentType)
	}

	tempFile, err := os.CreateTemp("", "video-output-*")
	if err != nil {
		return "", fmt.Errorf("create video output temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	defer tempFile.Close()

	written, err := io.Copy(tempFile, io.LimitReader(sourceBody, maxVideoOutputBytes+1))
	if err != nil {
		return "", fmt.Errorf("read video output: %w", err)
	}
	if written > maxVideoOutputBytes {
		return "", fmt.Errorf("video output exceeds %d bytes", maxVideoOutputBytes)
	}
	if written == 0 {
		return "", fmt.Errorf("video output is empty")
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind video output: %w", err)
	}

	objectKey := fmt.Sprintf("output/%s.%s", uuid.New().String(), ext)
	_, err = uploader.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(objectKey),
		Body:          tempFile,
		ContentLength: aws.Int64(written),
		ContentType:   aws.String(contentType),
		CacheControl:  aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return "", fmt.Errorf("upload video output to %s: %w", provider, err)
	}
	return publicBase + "/" + objectKey, nil
}

// Upload directory categories for different purposes
const (
	UploadDirGeneral  = "uploads"  // General uploads, can be cleaned
	UploadDirElements = "elements" // Kling/video elements, permanent
	UploadDirTemp     = "temp"     // Temporary files, aggressive cleanup
)
