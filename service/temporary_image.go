package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/image/webp"
)

const (
	TemporaryImageOutputCategory  = "output"
	TemporaryImageRetention       = 24 * time.Hour
	temporaryImageCleanupInterval = 15 * time.Minute
	maxTemporaryOutputImageBytes  = 50 * 1024 * 1024
)

var (
	ErrTemporaryImageExpired     = errors.New("temporary image expired")
	ErrTemporaryImageInvalidName = errors.New("invalid temporary image name")
)

type TemporaryImageCleanupStats struct {
	Deleted int
	Bytes   int64
}

func UploadBase64ImageToTemporaryOutput(mimeType, base64Data, publicBaseURL string) (string, error) {
	if base64Data == "" || base64.StdEncoding.DecodedLen(len(base64Data)) > maxTemporaryOutputImageBytes {
		return "", fmt.Errorf("temporary output image size must be between 1 and %d bytes", maxTemporaryOutputImageBytes)
	}
	uploadBytes, ext, _, err := prepareImageUpload(mimeType, base64Data)
	if err != nil {
		return "", err
	}
	if len(uploadBytes) == 0 || len(uploadBytes) > maxTemporaryOutputImageBytes {
		return "", fmt.Errorf("temporary output image size must be between 1 and %d bytes", maxTemporaryOutputImageBytes)
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(uploadBytes)); err != nil {
		if _, webpErr := webp.DecodeConfig(bytes.NewReader(uploadBytes)); webpErr != nil {
			return "", fmt.Errorf("temporary output is not a supported image: %w", err)
		}
	}

	outputDir := temporaryImageOutputDir()
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("create temporary output directory: %w", err)
	}

	filename := uuid.New().String() + "." + ext
	finalPath := filepath.Join(outputDir, filename)
	stagingFile, err := os.CreateTemp(outputDir, ".output-*.part")
	if err != nil {
		return "", fmt.Errorf("create temporary output staging file: %w", err)
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
		return "", fmt.Errorf("set temporary output permissions: %w", err)
	}
	if _, err := stagingFile.Write(uploadBytes); err != nil {
		return "", fmt.Errorf("write temporary output image: %w", err)
	}
	if err := stagingFile.Close(); err != nil {
		return "", fmt.Errorf("close temporary output image: %w", err)
	}
	if err := os.Rename(stagingPath, finalPath); err != nil {
		return "", fmt.Errorf("commit temporary output image: %w", err)
	}
	committed = true

	publicBase := normalizeHTTPBaseURL(firstNonEmptyString(
		publicBaseURL,
		os.Getenv("TEMP_STORAGE_PUBLIC_BASE_URL"),
		os.Getenv("LOCAL_PUBLIC_BASE_URL"),
		"https://cf-api.o1key.com",
	))
	return fmt.Sprintf("%s/tmp/%s/%s", publicBase, TemporaryImageOutputCategory, filename), nil
}

func OpenTemporaryOutputImage(filename string, now time.Time) (*os.File, os.FileInfo, error) {
	if !isTemporaryImageFilename(filename) {
		return nil, nil, ErrTemporaryImageInvalidName
	}

	filePath := filepath.Join(temporaryImageOutputDir(), filename)
	pathInfo, err := os.Lstat(filePath)
	if err != nil {
		return nil, nil, err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, nil, ErrTemporaryImageInvalidName
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, ErrTemporaryImageInvalidName
	}
	if !now.Before(info.ModTime().Add(TemporaryImageRetention)) {
		_ = file.Close()
		_ = os.Remove(filePath)
		return nil, nil, ErrTemporaryImageExpired
	}
	return file, info, nil
}

func CleanupExpiredTemporaryOutputImages(now time.Time) (TemporaryImageCleanupStats, error) {
	var stats TemporaryImageCleanupStats
	outputDir := temporaryImageOutputDir()
	entries, err := os.ReadDir(outputDir)
	if errors.Is(err, os.ErrNotExist) {
		return stats, nil
	}
	if err != nil {
		return stats, fmt.Errorf("read temporary output directory: %w", err)
	}

	cutoff := now.Add(-TemporaryImageRetention)
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
		isStaging := strings.HasPrefix(entry.Name(), ".output-") && strings.HasSuffix(entry.Name(), ".part")
		if (!isStaging && info.ModTime().After(cutoff)) || (isStaging && info.ModTime().After(stagingCutoff)) {
			continue
		}
		if err := os.Remove(filepath.Join(outputDir, entry.Name())); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove %s: %w", entry.Name(), err))
			continue
		}
		stats.Deleted++
		stats.Bytes += info.Size()
	}
	return stats, errors.Join(cleanupErrors...)
}

func temporaryImageOutputDir() string {
	root := strings.TrimSpace(os.Getenv("TEMP_STORAGE_DIR"))
	if root == "" {
		root = "tmp"
	}
	return filepath.Join(root, TemporaryImageOutputCategory)
}

func isTemporaryImageFilename(filename string) bool {
	if filename == "" || filepath.Base(filename) != filename {
		return false
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".png" && ext != ".jpg" && ext != ".webp" && ext != ".gif" {
		return false
	}
	_, err := uuid.Parse(strings.TrimSuffix(filename, filepath.Ext(filename)))
	return err == nil
}
