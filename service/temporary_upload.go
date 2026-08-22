package service

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
)

const (
	TemporaryInputCategory     = "input"
	TemporaryInputRetention    = TemporaryImageRetention
	TemporaryInputMaxFileBytes = int64(48 * 1024 * 1024)
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

func StoreTemporaryInputAttachment(reader io.Reader, originalFilename, requestHost string) (*TemporaryInputAttachment, error) {
	originalExtension := strings.ToLower(filepath.Ext(strings.TrimSpace(originalFilename)))
	if originalExtension != "" {
		if _, ok := temporaryInputFormats[originalExtension]; !ok {
			return nil, ErrTemporaryInputUnsupportedType
		}
	}

	inputDir := temporaryInputDir()
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		return nil, fmt.Errorf("create temporary input directory: %w", err)
	}

	stagingFile, err := os.CreateTemp(inputDir, ".input-*.part")
	if err != nil {
		return nil, fmt.Errorf("create temporary input staging file: %w", err)
	}
	stagingPath := stagingFile.Name()
	committed := false
	defer func() {
		_ = stagingFile.Close()
		if !committed {
			_ = os.Remove(stagingPath)
		}
	}()

	if err := stagingFile.Chmod(0644); err != nil {
		return nil, fmt.Errorf("set temporary input permissions: %w", err)
	}
	size, err := io.Copy(stagingFile, io.LimitReader(reader, TemporaryInputMaxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("write temporary input attachment: %w", err)
	}
	if size == 0 {
		return nil, ErrTemporaryInputEmpty
	}
	if size > TemporaryInputMaxFileBytes {
		return nil, ErrTemporaryInputTooLarge
	}
	if _, err := stagingFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind temporary input attachment: %w", err)
	}
	detected, err := mimetype.DetectReader(stagingFile)
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

	if err := stagingFile.Close(); err != nil {
		return nil, fmt.Errorf("close temporary input attachment: %w", err)
	}
	if !validateTemporaryInputArchive(stagingPath, originalExtension) {
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
	finalPath := filepath.Join(inputDir, filename)
	if err := os.Rename(stagingPath, finalPath); err != nil {
		return nil, fmt.Errorf("commit temporary input attachment: %w", err)
	}
	committed = true

	info, err := os.Stat(finalPath)
	if err != nil {
		return nil, fmt.Errorf("stat temporary input attachment: %w", err)
	}
	publicBase := temporaryInputPublicBaseURL(requestHost)
	return &TemporaryInputAttachment{
		URL:         fmt.Sprintf("%s/tmp/%s/%s", publicBase, TemporaryInputCategory, filename),
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		ExpiresAt:   info.ModTime().Add(TemporaryInputRetention),
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

func temporaryInputDir() string {
	root := strings.TrimSpace(os.Getenv("TEMP_STORAGE_DIR"))
	if root == "" {
		root = "tmp"
	}
	return filepath.Join(root, TemporaryInputCategory)
}

func temporaryInputPublicBaseURL(requestHost string) string {
	switch normalizeRequestHost(requestHost) {
	case "api.o1key.cn":
		return "https://api.o1key.cn"
	case "api.o1key.com", "cf-api.o1key.cn", "cf-api.o1key.com":
		return "https://cf-api.o1key.com"
	default:
		return normalizeHTTPBaseURL(firstNonEmptyString(
			os.Getenv("TEMP_STORAGE_PUBLIC_BASE_URL"),
			os.Getenv("LOCAL_PUBLIC_BASE_URL"),
			"https://cf-api.o1key.com",
		))
	}
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

func validateTemporaryInputArchive(filePath, extension string) bool {
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

	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return false
	}
	defer archive.Close()
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
