package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultR2PublicUploadClientsContainRequestedOrigins(t *testing.T) {
	clients, err := NewDefaultR2PublicUploadClients()
	require.NoError(t, err)
	require.NoError(t, ValidateR2PublicUploadClients(clients))

	origins := make(map[string]bool)
	for _, client := range clients {
		for _, origin := range client.Origins {
			origins[origin] = true
		}
	}
	for _, origin := range []string{
		"https://api.o1key.cn", "https://cf-api.o1key.com",
		"https://api.o1key.com", "https://api.thinksapi.com",
	} {
		assert.True(t, origins[origin], origin)
	}
}

func TestValidateR2PublicUploadClientsRejectsUnsafeOriginAndOversizedLimit(t *testing.T) {
	base := R2PublicUploadClient{
		ID: "client-a", Name: "Client A", Origins: []string{"http://example.com"},
		Secret: "secret", Enabled: true, MaxFileSizeMB: 100, RequestsPerMinute: 10,
	}
	assert.ErrorContains(t, ValidateR2PublicUploadClients([]R2PublicUploadClient{base}), "HTTPS origin")

	base.Origins = []string{"https://example.com"}
	base.MaxFileSizeMB = 101
	assert.ErrorContains(t, ValidateR2PublicUploadClients([]R2PublicUploadClient{base}), "between 1 and 100 MB")
}
