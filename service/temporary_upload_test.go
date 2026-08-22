package service

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreTemporaryInputAttachmentSupportsMainstreamAttachments(t *testing.T) {
	pngBytes, err := base64.StdEncoding.DecodeString(temporaryImageTestPNG)
	require.NoError(t, err)
	wavBytes := []byte{
		'R', 'I', 'F', 'F', 36, 0, 0, 0, 'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ', 16, 0, 0, 0, 1, 0, 1, 0,
		0x40, 0x1f, 0, 0, 0x80, 0x3e, 0, 0, 2, 0, 16, 0,
		'd', 'a', 't', 'a', 0, 0, 0, 0,
	}
	mp4Bytes := []byte{
		0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm',
		0, 0, 2, 0, 'i', 's', 'o', 'm', 'i', 's', 'o', '2',
	}

	tests := []struct {
		name        string
		filename    string
		contents    []byte
		contentType string
		extension   string
	}{
		{name: "image", filename: "reference.png", contents: pngBytes, contentType: "image/png", extension: ".png"},
		{name: "audio", filename: "sample.wav", contents: wavBytes, contentType: "audio/wav", extension: ".wav"},
		{name: "video", filename: "clip.mp4", contents: mp4Bytes, contentType: "video/mp4", extension: ".mp4"},
		{name: "document", filename: "brief.pdf", contents: []byte("%PDF-1.7\n% temporary upload\n"), contentType: "application/pdf", extension: ".pdf"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storageDir := t.TempDir()
			t.Setenv("TEMP_STORAGE_DIR", storageDir)

			before := time.Now()
			attachment, err := StoreTemporaryInputAttachment(bytes.NewReader(test.contents), test.filename, "cf-api.o1key.com")
			require.NoError(t, err)
			assert.Equal(t, test.contentType, attachment.ContentType)
			assert.Equal(t, int64(len(test.contents)), attachment.Size)
			assert.True(t, strings.HasPrefix(attachment.URL, "https://cf-api.o1key.com/tmp/input/"))
			assert.Equal(t, test.extension, filepath.Ext(attachment.Filename))
			assert.WithinDuration(t, before.Add(TemporaryInputRetention), attachment.ExpiresAt, 2*time.Second)

			stored, err := os.ReadFile(filepath.Join(storageDir, TemporaryInputCategory, attachment.Filename))
			require.NoError(t, err)
			assert.Equal(t, test.contents, stored)
		})
	}
}

func TestStoreTemporaryInputAttachmentValidatesOfficeArchive(t *testing.T) {
	storageDir := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", storageDir)

	var document bytes.Buffer
	archive := zip.NewWriter(&document)
	contentTypes, err := archive.Create("[Content_Types].xml")
	require.NoError(t, err)
	_, err = contentTypes.Write([]byte("<Types/>"))
	require.NoError(t, err)
	body, err := archive.Create("word/document.xml")
	require.NoError(t, err)
	_, err = body.Write([]byte("<document/>"))
	require.NoError(t, err)
	require.NoError(t, archive.Close())

	attachment, err := StoreTemporaryInputAttachment(bytes.NewReader(document.Bytes()), "brief.docx", "api.o1key.cn")
	require.NoError(t, err)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", attachment.ContentType)
	assert.True(t, strings.HasPrefix(attachment.URL, "https://api.o1key.cn/tmp/input/"))
}

func TestStoreTemporaryInputAttachmentRejectsDisguisedAndActiveContent(t *testing.T) {
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())

	tests := []struct {
		name     string
		filename string
		contents string
	}{
		{name: "renamed executable", filename: "payload.pdf", contents: "MZ-not-a-pdf"},
		{name: "HTML", filename: "page.html", contents: "<!doctype html><script>alert(1)</script>"},
		{name: "SVG", filename: "image.svg", contents: "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := StoreTemporaryInputAttachment(strings.NewReader(test.contents), test.filename, "cf-api.o1key.com")
			assert.ErrorIs(t, err, ErrTemporaryInputUnsupportedType)
		})
	}
}

func TestStoreTemporaryInputAttachmentDoesNotReflectUnknownHost(t *testing.T) {
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "https://fallback.example.com")
	pngBytes, err := base64.StdEncoding.DecodeString(temporaryImageTestPNG)
	require.NoError(t, err)

	attachment, err := StoreTemporaryInputAttachment(bytes.NewReader(pngBytes), "image.png", "attacker.example")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(attachment.URL, "https://fallback.example.com/tmp/input/"))
	assert.NotContains(t, attachment.URL, "attacker.example")
}

func TestOpenTemporaryInputAttachmentRejectsExpiredFile(t *testing.T) {
	storageDir := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", storageDir)
	pngBytes, err := base64.StdEncoding.DecodeString(temporaryImageTestPNG)
	require.NoError(t, err)
	attachment, err := StoreTemporaryInputAttachment(bytes.NewReader(pngBytes), "image.png", "cf-api.o1key.com")
	require.NoError(t, err)

	path := filepath.Join(storageDir, TemporaryInputCategory, attachment.Filename)
	now := time.Now()
	expiredAt := now.Add(-TemporaryInputRetention)
	require.NoError(t, os.Chtimes(path, expiredAt, expiredAt))

	file, _, _, err := OpenTemporaryInputAttachment(attachment.Filename, now)
	if file != nil {
		_ = file.Close()
	}
	assert.ErrorIs(t, err, ErrTemporaryInputExpired)
	_, statErr := os.Stat(path)
	assert.True(t, errors.Is(statErr, os.ErrNotExist))
}

func TestCleanupExpiredTemporaryInputAttachmentsPreservesFreshFiles(t *testing.T) {
	storageDir := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", storageDir)
	inputDir := filepath.Join(storageDir, TemporaryInputCategory)
	require.NoError(t, os.MkdirAll(inputDir, 0755))

	now := time.Now()
	freshPath := filepath.Join(inputDir, "8045b62c-39b6-4a7d-a75e-15fdb83420c2.pdf")
	expiredPath := filepath.Join(inputDir, "20e21317-4a0a-4379-b438-3141d4d33af0.pdf")
	require.NoError(t, os.WriteFile(freshPath, []byte("fresh"), 0644))
	require.NoError(t, os.WriteFile(expiredPath, []byte("expired"), 0644))
	expiredAt := now.Add(-TemporaryInputRetention - time.Minute)
	require.NoError(t, os.Chtimes(expiredPath, expiredAt, expiredAt))

	stats, err := CleanupExpiredTemporaryInputAttachments(now)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.Deleted)
	assert.Equal(t, int64(len("expired")), stats.Bytes)
	require.FileExists(t, freshPath)
	assert.NoFileExists(t, expiredPath)
}
