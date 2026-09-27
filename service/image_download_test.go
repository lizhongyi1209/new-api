package service

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetImageBytesFromURLRetriesHTTP522And525(t *testing.T) {
	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	originalWorkerURL := system_setting.WorkerUrl
	originalHTTPClient := httpClient
	t.Cleanup(func() {
		*fetchSetting = originalFetchSetting
		system_setting.WorkerUrl = originalWorkerURL
		httpClient = originalHTTPClient
	})
	fetchSetting.EnableSSRFProtection = false
	system_setting.WorkerUrl = ""

	tests := []struct {
		name         string
		statuses     []int
		wantAttempts int32
		wantError    string
	}{
		{name: "recovers from 522", statuses: []int{522, 522, http.StatusOK}, wantAttempts: 3},
		{name: "stops after three 522 retries", statuses: []int{522, 522, 522, 522}, wantAttempts: 4, wantError: "HTTP 522"},
		{name: "recovers on third retry", statuses: []int{525, 525, 525, http.StatusOK}, wantAttempts: 4},
		{name: "stops after three retries", statuses: []int{525, 525, 525, 525}, wantAttempts: 4, wantError: "HTTP 525"},
		{name: "does not retry other statuses", statuses: []int{http.StatusBadGateway}, wantAttempts: 1, wantError: "HTTP 502"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempt := int(attempts.Add(1))
				status := test.statuses[min(attempt-1, len(test.statuses)-1)]
				w.Header().Set("Content-Type", "image/png")
				w.WriteHeader(status)
				if status == http.StatusOK {
					_, _ = w.Write([]byte("image-bytes"))
				}
			}))
			t.Cleanup(server.Close)
			httpClient = server.Client()

			mimeType, imageBytes, err := GetImageBytesFromUrlWithLimit(server.URL+"/reference.png", 1)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				assert.Empty(t, imageBytes)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "image/png", mimeType)
				assert.Equal(t, []byte("image-bytes"), imageBytes)
			}
			assert.Equal(t, test.wantAttempts, attempts.Load())
		})
	}
}

func TestGetImageBytesFromR2TemporaryInputUsesSDK(t *testing.T) {
	const filename = "8045b62c-39b6-4a7d-a75e-15fdb83420c2.png"
	var calls atomic.Int32
	configureTemporaryInputR2Test(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/r2-bucket/tmp/input/"+filename, r.URL.Path)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("image-bytes"))
	})

	mimeType, imageBytes, err := GetImageBytesFromUrlWithLimit("https://r2.example.com/tmp/input/"+filename, 1)
	require.NoError(t, err)
	assert.Equal(t, "image/png", mimeType)
	assert.Equal(t, []byte("image-bytes"), imageBytes)
	assert.Equal(t, int32(1), calls.Load())
}

func TestR2TemporaryInputImageKeyRejectsOtherObjectsAndAmbiguousURLs(t *testing.T) {
	t.Setenv("R2_BUCKET", "r2-bucket")
	t.Setenv("R2_PUBLIC_BASE_URL", "https://r2.example.com")
	const filename = "8045b62c-39b6-4a7d-a75e-15fdb83420c2.png"
	for _, rawURL := range []string{
		"https://r2.example.com.evil.test/tmp/input/" + filename,
		"https://r2.example.com/tmp/output/" + filename,
		"https://r2.example.com/tmp/input/not-a-uuid.png",
		"https://r2.example.com/tmp/input/8045b62c-39b6-4a7d-a75e-15fdb83420c2.pdf",
		"https://r2.example.com/tmp/input/" + filename + "?signature=secret",
		"https://r2.example.com/tmp%2Finput/" + filename,
	} {
		key, ok := r2TemporaryInputImageKey(rawURL)
		assert.False(t, ok, rawURL)
		assert.Empty(t, key)
	}
}
