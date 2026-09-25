package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
)

const (
	TemporaryInputCategory     = "input"
	TemporaryInputRetention    = TemporaryImageRetention
	TemporaryInputMaxFileBytes = int64(20 * 1024 * 1024)
)

var (
	ErrTemporaryInputEmpty           = errors.New("temporary input attachment is empty")
	ErrTemporaryInputTooLarge        = errors.New("temporary input attachment is too large")
	ErrTemporaryInputUnsupportedType = errors.New("temporary input attachment type is unsupported")
	ErrTemporaryInputExpired         = errors.New("temporary input attachment expired")
	ErrTemporaryInputInvalidName     = errors.New("invalid temporary input attachment name")
)

type TemporaryInputAttachment struct {
	URL         string
	Filename    string
	ContentType string
	Size        int64
	ExpiresAt   time.Time
}

type TemporaryInputCleanupStats struct {
	Deleted int
	Bytes   int64
}

type temporaryInputFormat struct {
	Extension     string
	ContentType   string
	AcceptedMIMEs []string
}

var temporaryInputFormats = map[string]temporaryInputFormat{
	".png":  {Extension: ".png", ContentType: "image/png", AcceptedMIMEs: []string{"image/png", "image/vnd.mozilla.apng"}},
	".jpg":  {Extension: ".jpg", ContentType: "image/jpeg", AcceptedMIMEs: []string{"image/jpeg"}},
	".jpeg": {Extension: ".jpg", ContentType: "image/jpeg", AcceptedMIMEs: []string{"image/jpeg"}},
	".webp": {Extension: ".webp", ContentType: "image/webp", AcceptedMIMEs: []string{"image/webp"}},
	".gif":  {Extension: ".gif", ContentType: "image/gif", AcceptedMIMEs: []string{"image/gif"}},
	".bmp":  {Extension: ".bmp", ContentType: "image/bmp", AcceptedMIMEs: []string{"image/bmp"}},
	".tif":  {Extension: ".tiff", ContentType: "image/tiff", AcceptedMIMEs: []string{"image/tiff"}},
	".tiff": {Extension: ".tiff", ContentType: "image/tiff", AcceptedMIMEs: []string{"image/tiff"}},
	".heic": {Extension: ".heic", ContentType: "image/heic", AcceptedMIMEs: []string{"image/heic", "image/heic-sequence"}},
	".heif": {Extension: ".heif", ContentType: "image/heif", AcceptedMIMEs: []string{"image/heif", "image/heif-sequence"}},
	".avif": {Extension: ".avif", ContentType: "image/avif", AcceptedMIMEs: []string{"image/avif"}},

	".mp3":  {Extension: ".mp3", ContentType: "audio/mpeg", AcceptedMIMEs: []string{"audio/mpeg"}},
	".wav":  {Extension: ".wav", ContentType: "audio/wav", AcceptedMIMEs: []string{"audio/wav"}},
	".flac": {Extension: ".flac", ContentType: "audio/flac", AcceptedMIMEs: []string{"audio/flac"}},
	".aac":  {Extension: ".aac", ContentType: "audio/aac", AcceptedMIMEs: []string{"audio/aac"}},
	".m4a":  {Extension: ".m4a", ContentType: "audio/mp4", AcceptedMIMEs: []string{"audio/mp4", "audio/x-m4a"}},
	".oga":  {Extension: ".oga", ContentType: "audio/ogg", AcceptedMIMEs: []string{"audio/ogg", "application/ogg"}},
	".ogg":  {Extension: ".ogg", ContentType: "application/ogg", AcceptedMIMEs: []string{"application/ogg", "audio/ogg", "video/ogg"}},

	".mp4":  {Extension: ".mp4", ContentType: "video/mp4", AcceptedMIMEs: []string{"video/mp4", "audio/mp4"}},
	".mov":  {Extension: ".mov", ContentType: "video/quicktime", AcceptedMIMEs: []string{"video/quicktime"}},
	".m4v":  {Extension: ".m4v", ContentType: "video/x-m4v", AcceptedMIMEs: []string{"video/x-m4v", "video/mp4"}},
	".webm": {Extension: ".webm", ContentType: "video/webm", AcceptedMIMEs: []string{"video/webm", "audio/webm"}},
	".avi":  {Extension: ".avi", ContentType: "video/x-msvideo", AcceptedMIMEs: []string{"video/x-msvideo"}},
	".mkv":  {Extension: ".mkv", ContentType: "video/x-matroska", AcceptedMIMEs: []string{"video/x-matroska"}},
	".mpeg": {Extension: ".mpeg", ContentType: "video/mpeg", AcceptedMIMEs: []string{"video/mpeg"}},
	".mpg":  {Extension: ".mpeg", ContentType: "video/mpeg", AcceptedMIMEs: []string{"video/mpeg"}},
	".ogv":  {Extension: ".ogv", ContentType: "video/ogg", AcceptedMIMEs: []string{"video/ogg", "application/ogg"}},
	".3gp":  {Extension: ".3gp", ContentType: "video/3gpp", AcceptedMIMEs: []string{"video/3gpp"}},
	".3g2":  {Extension: ".3g2", ContentType: "video/3gpp2", AcceptedMIMEs: []string{"video/3gpp2"}},

	".pdf":  {Extension: ".pdf", ContentType: "application/pdf", AcceptedMIMEs: []string{"application/pdf"}},
	".txt":  {Extension: ".txt", ContentType: "text/plain; charset=utf-8", AcceptedMIMEs: []string{"text/plain"}},
	".md":   {Extension: ".md", ContentType: "text/markdown; charset=utf-8", AcceptedMIMEs: []string{"text/plain"}},
	".csv":  {Extension: ".csv", ContentType: "text/csv; charset=utf-8", AcceptedMIMEs: []string{"text/csv", "text/plain"}},
	".json": {Extension: ".json", ContentType: "application/json", AcceptedMIMEs: []string{"application/json"}},
	".rtf":  {Extension: ".rtf", ContentType: "application/rtf", AcceptedMIMEs: []string{"text/rtf", "application/rtf"}},
	".doc":  {Extension: ".doc", ContentType: "application/msword", AcceptedMIMEs: []string{"application/msword", "application/x-ole-storage"}},
	".xls":  {Extension: ".xls", ContentType: "application/vnd.ms-excel", AcceptedMIMEs: []string{"application/vnd.ms-excel", "application/x-ole-storage"}},
	".ppt":  {Extension: ".ppt", ContentType: "application/vnd.ms-powerpoint", AcceptedMIMEs: []string{"application/vnd.ms-powerpoint", "application/x-ole-storage"}},
	".docx": {Extension: ".docx", ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", AcceptedMIMEs: []string{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/zip"}},
	".xlsx": {Extension: ".xlsx", ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", AcceptedMIMEs: []string{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/zip"}},
	".pptx": {Extension: ".pptx", ContentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation", AcceptedMIMEs: []string{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "application/zip"}},
	".odt":  {Extension: ".odt", ContentType: "application/vnd.oasis.opendocument.text", AcceptedMIMEs: []string{"application/vnd.oasis.opendocument.text", "application/zip"}},
	".ods":  {Extension: ".ods", ContentType: "application/vnd.oasis.opendocument.spreadsheet", AcceptedMIMEs: []string{"application/vnd.oasis.opendocument.spreadsheet", "application/zip"}},
	".odp":  {Extension: ".odp", ContentType: "application/vnd.oasis.opendocument.presentation", AcceptedMIMEs: []string{"application/vnd.oasis.opendocument.presentation", "application/zip"}},
}

func StoreTemporaryInputAttachment(ctx context.Context, reader io.Reader, originalFilename string) (*TemporaryInputAttachment, error) {
	originalExtension := strings.ToLower(filepath.Ext(strings.TrimSpace(originalFilename)))
	if originalExtension != "" {
		if _, ok := temporaryInputFormats[originalExtension]; !ok {
			return nil, ErrTemporaryInputUnsupportedType
		}
	}

	var body bytes.Buffer
	size, err := io.Copy(&body, io.LimitReader(reader, TemporaryInputMaxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read temporary input attachment: %w", err)
	}
	if size == 0 {
		return nil, ErrTemporaryInputEmpty
	}
	if size > TemporaryInputMaxFileBytes {
		return nil, ErrTemporaryInputTooLarge
	}
	detected, err := mimetype.DetectReader(bytes.NewReader(body.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("detect temporary input attachment type: %w", err)
	}

	if originalExtension == "" {
		originalExtension = strings.ToLower(detected.Extension())
	}
	format, ok := temporaryInputFormats[originalExtension]
	if !ok {
		return nil, ErrTemporaryInputUnsupportedType
	}
	matchesDetectedType := false
	for _, acceptedMIME := range format.AcceptedMIMEs {
		if detected.Is(acceptedMIME) {
			matchesDetectedType = true
			break
		}
	}
	if !matchesDetectedType {
		return nil, ErrTemporaryInputUnsupportedType
	}

	if !validateTemporaryInputArchive(body.Bytes(), originalExtension) {
		return nil, ErrTemporaryInputUnsupportedType
	}

	contentType := format.ContentType
	finalExtension := format.Extension
	if originalExtension == ".mp4" && detected.Is("audio/mp4") {
		contentType = "audio/mp4"
		finalExtension = ".m4a"
	}
	if originalExtension == ".ogg" {
		switch {
		case detected.Is("audio/ogg"):
			contentType = "audio/ogg"
			finalExtension = ".oga"
		case detected.Is("video/ogg"):
			contentType = "video/ogg"
			finalExtension = ".ogv"
		default:
			contentType = "application/ogg"
		}
	}

	filename := uuid.New().String() + finalExtension
	var client *s3.Client
	var bucket, publicBase string
	if IsAliyunOSSBlocked() {
		client, _ = getR2Client()
		bucket = strings.TrimSpace(os.Getenv("R2_BUCKET"))
		publicBase = normalizeHTTPBaseURL(os.Getenv("R2_PUBLIC_BASE_URL"))
	} else {
		client, _, err = getAliyunOSSClient()
		if err != nil {
			return nil, err
		}
		bucket = strings.TrimSpace(firstNonEmptyEnv("ALIYUN_OSS_BUCKET", "OSS_BUCKET"))
		publicBase = normalizeHTTPBaseURL(firstNonEmptyEnv("ALIYUN_OSS_PUBLIC_BASE_URL", "OSS_PUBLIC_BASE_URL"))
	}
	if bucket == "" || publicBase == "" {
		return nil, fmt.Errorf("missing object storage bucket or public base URL for temporary input attachment")
	}
	parsedBase, err := url.Parse(publicBase)
	if err != nil || parsedBase.Scheme != "https" || parsedBase.Host == "" || parsedBase.User != nil || parsedBase.RawQuery != "" || parsedBase.Fragment != "" {
		return nil, fmt.Errorf("temporary input public base URL must be an HTTPS origin")
	}
	objectKey := "tmp/input/" + filename
	putInput := &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(objectKey),
		Body:          bytes.NewReader(body.Bytes()),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
		CacheControl:  aws.String("public, max-age=3600, must-revalidate"),
	}
	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "audio/") && !strings.HasPrefix(contentType, "video/") {
		putInput.ContentDisposition = aws.String("attachment")
	}
	_, err = client.PutObject(ctx, putInput)
	if err != nil {
		return nil, fmt.Errorf("upload temporary input attachment to object storage: %w", err)
	}
	expiresAt := time.Now().Add(TemporaryInputRetention)
	return &TemporaryInputAttachment{
		URL:         publicBase + "/" + objectKey,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		ExpiresAt:   expiresAt,
	}, nil
}

func OpenTemporaryInputAttachment(filename string, now time.Time) (*os.File, os.FileInfo, string, error) {
	if !isTemporaryInputFilename(filename) {
		return nil, nil, "", ErrTemporaryInputInvalidName
	}

	filePath := filepath.Join(temporaryInputDir(), filename)
	pathInfo, err := os.Lstat(filePath)
	if err != nil {
		return nil, nil, "", err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, nil, "", ErrTemporaryInputInvalidName
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, "", err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, "", err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, "", ErrTemporaryInputInvalidName
	}
	if !now.Before(info.ModTime().Add(TemporaryInputRetention)) {
		_ = file.Close()
		_ = os.Remove(filePath)
		return nil, nil, "", ErrTemporaryInputExpired
	}
	format := temporaryInputFormats[strings.ToLower(filepath.Ext(filename))]
	return file, info, format.ContentType, nil
}

func CleanupExpiredTemporaryInputAttachments(now time.Time) (TemporaryInputCleanupStats, error) {
	var stats TemporaryInputCleanupStats
	inputDir := temporaryInputDir()
	entries, err := os.ReadDir(inputDir)
	if errors.Is(err, os.ErrNotExist) {
		return stats, nil
	}
	if err != nil {
		return stats, fmt.Errorf("read temporary input directory: %w", err)
	}

	cutoff := now.Add(-TemporaryInputRetention)
	stagingCutoff := now.Add(-2 * temporaryImageCleanupInterval)
	var cleanupErrors []error
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("stat %s: %w", entry.Name(), err))
			continue
		}
		isStaging := strings.HasPrefix(entry.Name(), ".input-") && strings.HasSuffix(entry.Name(), ".part")
		if (!isStaging && info.ModTime().After(cutoff)) || (isStaging && info.ModTime().After(stagingCutoff)) {
			continue
		}
		if err := os.Remove(filepath.Join(inputDir, entry.Name())); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove %s: %w", entry.Name(), err))
			continue
		}
		stats.Deleted++
		stats.Bytes += info.Size()
	}
	return stats, errors.Join(cleanupErrors...)
}

// CleanupExpiredTemporaryInputObjects removes uploaded inputs from the dedicated
// object prefix. Public object URLs can remain cached after deletion, so the
// response expiry is the scheduled cleanup time rather than a hard access gate.
func CleanupExpiredTemporaryInputObjects(ctx context.Context, now time.Time) (TemporaryInputCleanupStats, error) {
	var stats TemporaryInputCleanupStats
	var cleanupErrors []error
	type objectStore struct {
		name   string
		client *s3.Client
		bucket string
	}
	stores := make([]objectStore, 0, 2)

	if bucket := strings.TrimSpace(firstNonEmptyEnv("ALIYUN_OSS_BUCKET", "OSS_BUCKET")); bucket != "" {
		client, _, err := getAliyunOSSClient()
		if err != nil {
			cleanupErrors = append(cleanupErrors, err)
		} else {
			stores = append(stores, objectStore{"Aliyun OSS", client, bucket})
		}
	}
	if bucket := strings.TrimSpace(os.Getenv("R2_BUCKET")); bucket != "" && os.Getenv("R2_SECRET_ACCESS_KEY") != "" {
		client, _ := getR2Client()
		stores = append(stores, objectStore{"R2", client, bucket})
	}

	cutoff := now.Add(-TemporaryInputRetention)
	for _, store := range stores {
		pages := s3.NewListObjectsV2Paginator(store.client, &s3.ListObjectsV2Input{
			Bucket: aws.String(store.bucket),
			Prefix: aws.String("tmp/input/"),
		})
		for pages.HasMorePages() {
			page, err := pages.NextPage(ctx)
			if err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("list %s temporary input objects: %w", store.name, err))
				break
			}
			for _, object := range page.Contents {
				if object.Key == nil || object.LastModified == nil || object.LastModified.After(cutoff) {
					continue
				}
				_, err = store.client.DeleteObject(ctx, &s3.DeleteObjectInput{
					Bucket: aws.String(store.bucket),
					Key:    object.Key,
				})
				if err != nil {
					cleanupErrors = append(cleanupErrors, fmt.Errorf("delete %s temporary input object: %w", store.name, err))
					continue
				}
				stats.Deleted++
				stats.Bytes += aws.ToInt64(object.Size)
			}
		}
	}
	return stats, errors.Join(cleanupErrors...)
}

func temporaryInputDir() string {
	root := strings.TrimSpace(os.Getenv("TEMP_STORAGE_DIR"))
	if root == "" {
		root = "tmp"
	}
	return filepath.Join(root, TemporaryInputCategory)
}

func isTemporaryInputFilename(filename string) bool {
	if filename == "" || filepath.Base(filename) != filename {
		return false
	}
	extension := strings.ToLower(filepath.Ext(filename))
	if _, ok := temporaryInputFormats[extension]; !ok {
		return false
	}
	_, err := uuid.Parse(strings.TrimSuffix(filename, filepath.Ext(filename)))
	return err == nil
}

func validateTemporaryInputArchive(data []byte, extension string) bool {
	var requiredPath string
	var requiredMIME string
	switch extension {
	case ".docx":
		requiredPath = "word/document.xml"
	case ".xlsx":
		requiredPath = "xl/workbook.xml"
	case ".pptx":
		requiredPath = "ppt/presentation.xml"
	case ".odt":
		requiredMIME = "application/vnd.oasis.opendocument.text"
	case ".ods":
		requiredMIME = "application/vnd.oasis.opendocument.spreadsheet"
	case ".odp":
		requiredMIME = "application/vnd.oasis.opendocument.presentation"
	default:
		return true
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, entry := range archive.File {
		if requiredPath != "" && entry.Name == requiredPath {
			return true
		}
		if requiredMIME == "" || entry.Name != "mimetype" || entry.UncompressedSize64 > 256 {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return false
		}
		content, readErr := io.ReadAll(io.LimitReader(reader, 257))
		closeErr := reader.Close()
		return readErr == nil && closeErr == nil && string(content) == requiredMIME
	}
	return false
}
