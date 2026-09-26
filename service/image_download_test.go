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

func TestGetImageBytesFromURLRetriesOnlyHTTP525(t *testing.T) {
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
