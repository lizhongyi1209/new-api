package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildUnifiedSeedanceAssetRequestHC(t *testing.T) {
	path, body, err := buildUnifiedSeedanceAssetRequest(UnifiedSeedanceAssetRequest{
		Type:      " hc ",
		URL:       " https://cdn.example.com/avatar.png ",
		Name:      " avatar-front ",
		AssetType: "image",
	})

	require.NoError(t, err)
	assert.Equal(t, "/v1/sd/assets", path)
	assert.JSONEq(t, `{
		"URL": "https://cdn.example.com/avatar.png",
		"Name": "avatar-front",
		"AssetType": "Image"
	}`, string(body))
}

func TestBuildUnifiedSeedanceAssetRequestDefaultsToImage(t *testing.T) {
	_, body, err := buildUnifiedSeedanceAssetRequest(UnifiedSeedanceAssetRequest{
		Type: "HC",
		URL:  "https://cdn.example.com/avatar.png",
	})

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"URL": "https://cdn.example.com/avatar.png",
		"AssetType": "Image"
	}`, string(body))
}

func TestBuildUnifiedSeedanceAssetRequestNormalizesAssetTypes(t *testing.T) {
	tests := []struct {
		assetType string
		want      string
	}{
		{assetType: "Image", want: "Image"},
		{assetType: "video", want: "Video"},
		{assetType: "AUDIO", want: "Audio"},
	}

	for _, test := range tests {
		t.Run(test.assetType, func(t *testing.T) {
			_, body, err := buildUnifiedSeedanceAssetRequest(UnifiedSeedanceAssetRequest{
				Type:      "hc",
				URL:       "https://cdn.example.com/material",
				AssetType: test.assetType,
			})

			require.NoError(t, err)
			var upstream map[string]any
			require.NoError(t, common.Unmarshal(body, &upstream))
			assert.Equal(t, test.want, upstream["AssetType"])
		})
	}
}

func TestBuildUnifiedSeedanceAssetRequestRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		request UnifiedSeedanceAssetRequest
		wantErr string
	}{
		{
			name:    "unsupported workflow",
			request: UnifiedSeedanceAssetRequest{Type: "standard", URL: "https://cdn.example.com/a.png"},
			wantErr: "currently supported: hc",
		},
		{
			name:    "non https url",
			request: UnifiedSeedanceAssetRequest{Type: "hc", URL: "http://cdn.example.com/a.png"},
			wantErr: "public HTTPS URL",
		},
		{
			name:    "unsupported asset type",
			request: UnifiedSeedanceAssetRequest{Type: "hc", URL: "https://cdn.example.com/a.txt", AssetType: "text"},
			wantErr: "image, video, or audio",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := buildUnifiedSeedanceAssetRequest(test.request)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}
