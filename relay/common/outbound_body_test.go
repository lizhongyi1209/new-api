package common

import (
	"io"
	"testing"

	projectcommon "github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertIndependentReplayReaders(t *testing.T, payload []byte, body io.Reader, getBody func() (io.ReadCloser, error)) {
	t.Helper()
	half := len(payload) / 2

	primaryHead := make([]byte, half)
	_, err := io.ReadFull(body, primaryHead)
	require.NoError(t, err)
	assert.Equal(t, payload[:half], primaryHead)

	first, err := getBody()
	require.NoError(t, err)
	second, err := getBody()
	require.NoError(t, err)

	firstHead := make([]byte, half)
	_, err = io.ReadFull(first, firstHead)
	require.NoError(t, err)
	assert.Equal(t, payload[:half], firstHead)

	secondBody, err := io.ReadAll(second)
	require.NoError(t, err)
	require.NoError(t, second.Close())
	assert.Equal(t, payload, secondBody)

	firstRest, err := io.ReadAll(first)
	require.NoError(t, err)
	require.NoError(t, first.Close())
	assert.Equal(t, payload[half:], firstRest)

	primaryRest, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, payload[half:], primaryRest)
}

func TestNewOutboundJSONBodyReplayReadersAreIndependent(t *testing.T) {
	payload := []byte(`{"model":"test-model","input":"abcdefghijklmnopqrstuvwxyz"}`)
	body, size, getBody, closer, err := NewOutboundJSONBody(payload)
	require.NoError(t, err)
	assert.EqualValues(t, len(payload), size)
	assertIndependentReplayReaders(t, payload, body, getBody)

	require.NoError(t, closer.Close())
	_, err = getBody()
	require.ErrorIs(t, err, projectcommon.ErrStorageClosed)
}

func TestNewOutboundJSONBodyDiskReplayReadersAreIndependent(t *testing.T) {
	previous := projectcommon.GetDiskCacheConfig()
	projectcommon.SetDiskCacheConfig(projectcommon.DiskCacheConfig{
		Enabled: true, ThresholdMB: 0, MaxSizeMB: 64, Path: t.TempDir(),
	})
	t.Cleanup(func() { projectcommon.SetDiskCacheConfig(previous) })

	payload := []byte(`{"model":"test-model","input":"abcdefghijklmnopqrstuvwxyz"}`)
	body, _, getBody, closer, err := NewOutboundJSONBody(payload)
	require.NoError(t, err)
	storage, ok := closer.(projectcommon.BodyStorage)
	require.True(t, ok)
	assert.True(t, storage.IsDisk())
	assertIndependentReplayReaders(t, payload, body, getBody)

	require.NoError(t, closer.Close())
	_, err = getBody()
	require.ErrorIs(t, err, projectcommon.ErrStorageClosed)
}
