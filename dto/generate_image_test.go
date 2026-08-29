package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateImageInputAcceptsLegacyAndExplicitFormats(t *testing.T) {
	requestBody := []byte(`{
		"model":"nano-banana-pro",
		"prompt":"draw",
		"images":[
			"https://example.com/legacy.png",
			{"inlineData":{"mimeType":"image/png","data":"AAAA"}},
			{"fileData":{"mimeType":"image/jpeg","fileUri":"https://example.com/file.jpg"}}
		]
	}`)

	var request GenerateImageRequest
	require.NoError(t, common.Unmarshal(requestBody, &request))
	require.Len(t, request.Images, 3)
	require.NotNil(t, request.Images[0].Value)
	assert.Equal(t, "https://example.com/legacy.png", *request.Images[0].Value)
	require.NotNil(t, request.Images[1].InlineData)
	assert.Equal(t, "image/png", request.Images[1].InlineData.MimeType)
	require.NotNil(t, request.Images[2].FileData)
	assert.Equal(t, "https://example.com/file.jpg", request.Images[2].FileData.FileURI)

	encoded, err := common.Marshal(request)
	require.NoError(t, err)
	assert.JSONEq(t, string(requestBody), string(encoded))
}

func TestGenerateImageInputRejectsAmbiguousObjects(t *testing.T) {
	for _, inputJSON := range []string{
		`{}`,
		`{"inlineData":{"mimeType":"image/png","data":"AAAA"},"fileData":{"mimeType":"image/png","fileUri":"https://example.com/a.png"}}`,
		`null`,
		`123`,
	} {
		t.Run(inputJSON, func(t *testing.T) {
			var input GenerateImageInput
			require.Error(t, common.Unmarshal([]byte(inputJSON), &input))
		})
	}
}
