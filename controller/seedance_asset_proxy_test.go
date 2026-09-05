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

func TestBuildUnifiedSeedanceAssetRequestDF(t *testing.T) {
	path, body, err := buildUnifiedSeedanceAssetRequest(UnifiedSeedanceAssetRequest{
		Type:      " DF ",
		URL:       "https://cdn.example.com/reference.png",
		Name:      "0.jpg",
		AssetType: "image",
	})

	require.NoError(t, err)
	assert.Equal(t, "/v1/sd-5/assets", path)
	assert.JSONEq(t, `{
		"URL": "https://cdn.example.com/reference.png",
		"Name": "0.jpg",
		"AssetType": "Image"
	}`, string(body))
}

func TestBuildUnifiedSeedanceAssetRequestDoubao(t *testing.T) {
	path, body, err := buildUnifiedSeedanceAssetRequest(UnifiedSeedanceAssetRequest{
		Type:      " doubao ",
		URL:       " http://cdn.example.com/reference.mp4 ",
		Name:      " camera-reference ",
		AssetType: "video",
		Model:     " doubao-seedance-2-0-260128-max ",
	})

	require.NoError(t, err)
	assert.Equal(t, "/v2/db-sd-max/assets", path)
	assert.JSONEq(t, `{
		"URL": "http://cdn.example.com/reference.mp4",
		"Name": "camera-reference",
		"AssetType": "Video",
		"Model": "doubao-seedance-2-0-260128-max"
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
			wantErr: "currently supported: hc, df",
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

func TestBuildUnifiedSeedanceAssetQueryHC(t *testing.T) {
	path, err := buildUnifiedSeedanceAssetQuery(" hc ", " asset 123 ")

	require.NoError(t, err)
	assert.Equal(t, "/v1/sd/assets/asset%20123", path)
}

func TestBuildUnifiedSeedanceAssetQueryDF(t *testing.T) {
	path, err := buildUnifiedSeedanceAssetQuery("df", "asset-123")

	require.NoError(t, err)
	assert.Equal(t, "/v1/sd-5/assets/asset-123", path)
}

func TestBuildUnifiedSeedanceAssetQueryDoubao(t *testing.T) {
	path, err := buildUnifiedSeedanceAssetQuery("doubao", "mva-e144dfc364a647f1")

	require.NoError(t, err)
	assert.Equal(t, "/v2/db-sd-max/assets/mva-e144dfc364a647f1", path)
}

func TestBuildUnifiedSeedanceAssetQueryRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name          string
		assetWorkflow string
		assetID       string
		wantErr       string
	}{
		{
			name:          "unsupported workflow",
			assetWorkflow: "standard",
			assetID:       "asset-123",
			wantErr:       "currently supported: hc, df",
		},
		{
			name:          "missing asset id",
			assetWorkflow: "hc",
			wantErr:       "asset_id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := buildUnifiedSeedanceAssetQuery(test.assetWorkflow, test.assetID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}
