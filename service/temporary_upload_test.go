package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureTemporaryInputOSSTest(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	previousClient, previousPresignClient := ossClient, ossPresignClient
	ossClient, ossPresignClient = nil, nil
	t.Cleanup(func() { ossClient, ossPresignClient = previousClient, previousPresignClient })
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_SECRET", "test-secret-key")
	t.Setenv("ALIYUN_OSS_REGION", "test-region")
	t.Setenv("ALIYUN_OSS_ENDPOINT", server.URL)
	t.Setenv("ALIYUN_OSS_FORCE_PATH_STYLE", "true")
	t.Setenv("ALIYUN_OSS_BUCKET", "test-bucket")
	t.Setenv("ALIYUN_OSS_PUBLIC_BASE_URL", "https://media.example.com")
	t.Setenv("DISABLE_ALIYUN_OSS", "false")
}

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

	type storedObject struct {
		path               string
		contentType        string
		contentDisposition string
		body               []byte
	}
	uploads := make(chan storedObject, len(tests))
	configureTemporaryInputOSSTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		uploads <- storedObject{r.URL.Path, r.Header.Get("Content-Type"), r.Header.Get("Content-Disposition"), body}
		w.WriteHeader(http.StatusOK)
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storageDir := t.TempDir()
			t.Setenv("TEMP_STORAGE_DIR", storageDir)

			before := time.Now()
			attachment, err := StoreTemporaryInputAttachment(context.Background(), bytes.NewReader(test.contents), test.filename)
			require.NoError(t, err)
			assert.Equal(t, test.contentType, attachment.ContentType)
			assert.Equal(t, int64(len(test.contents)), attachment.Size)
			assert.True(t, strings.HasPrefix(attachment.URL, "https://media.example.com/tmp/input/"))
			assert.Equal(t, test.extension, filepath.Ext(attachment.Filename))
			assert.WithinDuration(t, before.Add(TemporaryInputRetention), attachment.ExpiresAt, 2*time.Second)

			upload := <-uploads
			assert.Equal(t, "/test-bucket/tmp/input/"+attachment.Filename, upload.path)
			assert.Equal(t, test.contentType, upload.contentType)
			assert.Equal(t, test.contents, upload.body)
			if test.name == "document" {
				assert.Equal(t, "attachment", upload.contentDisposition)
			}
			assert.NoDirExists(t, filepath.Join(storageDir, TemporaryInputCategory))
		})
	}
}

func TestStoreTemporaryInputAttachmentValidatesOfficeArchive(t *testing.T) {
	configureTemporaryInputOSSTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

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

	attachment, err := StoreTemporaryInputAttachment(context.Background(), bytes.NewReader(document.Bytes()), "brief.docx")
	require.NoError(t, err)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", attachment.ContentType)
	assert.True(t, strings.HasPrefix(attachment.URL, "https://media.example.com/tmp/input/"))
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
			_, err := StoreTemporaryInputAttachment(context.Background(), strings.NewReader(test.contents), test.filename)
			assert.ErrorIs(t, err, ErrTemporaryInputUnsupportedType)
		})
	}
}

func TestStoreTemporaryInputAttachmentRejectsFilesOver20MiB(t *testing.T) {
	for _, test := range []struct {
		name     string
		filename string
	}{
		{"image", "image.png"},
		{"audio", "audio.wav"},
		{"video", "video.mp4"},
		{"document", "document.pdf"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := StoreTemporaryInputAttachment(context.Background(), bytes.NewReader(make([]byte, TemporaryInputMaxFileBytes+1)), test.filename)
			assert.ErrorIs(t, err, ErrTemporaryInputTooLarge)
		})
	}
}

func TestStoreTemporaryInputAttachmentFailsWhenOSSUnavailable(t *testing.T) {
	previousClient, previousPresignClient := ossClient, ossPresignClient
	ossClient, ossPresignClient = nil, nil
	t.Cleanup(func() { ossClient, ossPresignClient = previousClient, previousPresignClient })
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_ID", "")
	t.Setenv("OSS_ACCESS_KEY_ID", "")
	t.Setenv("DISABLE_ALIYUN_OSS", "false")
	storageDir := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", storageDir)
	pngBytes, err := base64.StdEncoding.DecodeString(temporaryImageTestPNG)
	require.NoError(t, err)
	_, err = StoreTemporaryInputAttachment(context.Background(), bytes.NewReader(pngBytes), "image.png")
	require.Error(t, err)
	assert.NoDirExists(t, filepath.Join(storageDir, TemporaryInputCategory))
}

func TestStoreTemporaryInputAttachmentUsesR2WhenOSSDisabled(t *testing.T) {
	uploaded := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploaded <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	previousClient, previousPresignClient := r2Client, r2PresignClient
	r2Client = s3.NewFromConfig(aws.Config{
		Region:      "auto",
		Credentials: credentials.NewStaticCredentialsProvider("test-access-key", "test-secret-key", ""),
	}, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(server.URL)
		options.UsePathStyle = true
	})
	r2PresignClient = nil
	t.Cleanup(func() { r2Client, r2PresignClient = previousClient, previousPresignClient })
	t.Setenv("DISABLE_ALIYUN_OSS", "true")
	t.Setenv("R2_BUCKET", "r2-bucket")
	t.Setenv("R2_PUBLIC_BASE_URL", "https://r2.example.com")
	pngBytes, err := base64.StdEncoding.DecodeString(temporaryImageTestPNG)
	require.NoError(t, err)

	attachment, err := StoreTemporaryInputAttachment(context.Background(), bytes.NewReader(pngBytes), "image.png")
	require.NoError(t, err)
	assert.Equal(t, "https://r2.example.com/tmp/input/"+attachment.Filename, attachment.URL)
	assert.Equal(t, "/r2-bucket/tmp/input/"+attachment.Filename, <-uploaded)
}

func TestOpenTemporaryInputAttachmentRejectsExpiredFile(t *testing.T) {
	storageDir := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", storageDir)
	pngBytes, err := base64.StdEncoding.DecodeString(temporaryImageTestPNG)
	require.NoError(t, err)
	filename := "8045b62c-39b6-4a7d-a75e-15fdb83420c2.png"
	path := filepath.Join(storageDir, TemporaryInputCategory, filename)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, pngBytes, 0644))
	now := time.Now()
	expiredAt := now.Add(-TemporaryInputRetention)
	require.NoError(t, os.Chtimes(path, expiredAt, expiredAt))

	file, _, _, err := OpenTemporaryInputAttachment(filename, now)
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

func TestCleanupExpiredTemporaryInputObjectsDeletesOnlyExpiredPrefixObjects(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	deleted := make(chan string, 2)
	configureTemporaryInputOSSTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("prefix") != "tmp/input/" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprintf(w, `<ListBucketResult><IsTruncated>false</IsTruncated>
<Contents><Key>tmp/input/expired.png</Key><LastModified>%s</LastModified><Size>12</Size></Contents>
<Contents><Key>tmp/input/fresh.png</Key><LastModified>%s</LastModified><Size>24</Size></Contents>
</ListBucketResult>`, now.Add(-25*time.Hour).Format(time.RFC3339), now.Add(-time.Hour).Format(time.RFC3339))
		case http.MethodDelete:
			deleted <- r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	t.Setenv("R2_BUCKET", "")

	stats, err := CleanupExpiredTemporaryInputObjects(context.Background(), now)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.Deleted)
	assert.Equal(t, int64(12), stats.Bytes)
	assert.Equal(t, "/test-bucket/tmp/input/expired.png", <-deleted)
	assert.Empty(t, deleted)
}
