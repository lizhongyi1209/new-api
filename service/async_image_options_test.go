package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func asyncStringPtr(value string) *string { return &value }

func asyncIntPtr(value int) *int { return &value }

func asyncUintPtr(value uint) *uint { return &value }

// TestValidateAsyncImageRequestAppliesGPTImageContract covers the legacy
// /async/v1/images/generations endpoint, which reaches the same upstream edit
// and generation paths as the sync route and therefore shares its rules.
func TestValidateAsyncImageRequestAppliesGPTImageContract(t *testing.T) {
	// Reference validation also runs an SSRF check, so the fixture must be a
	// resolvable, fetchable URL rather than a made-up hostname.
	const reference = "https://o1key-client.oss-cn-guangzhou.aliyuncs.com/output/00001645-776a-4c27-8ca8-292f7beb58d4.jpg"

	for _, tc := range []struct {
		name    string
		request *dto.AsyncImageRequest
		wantErr string
	}{
		{
			name:    "2.6 generation with complexity",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.6-flare", Prompt: "a vase", Complexity: asyncStringPtr("high")},
		},
		{
			name:    "2.6 gif output with frame timing",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.6-flare", Prompt: "a lantern", OutputFormat: asyncStringPtr("gif"), FrameCount: asyncIntPtr(9), FrameDurationMs: asyncIntPtr(200)},
		},
		{
			name:    "2.5 accepts markup on an edit",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.5-sunburst", Prompt: "recolor", Images: []string{reference}, Markup: &dto.ImageReference{ImageURL: asyncStringPtr(reference)}},
		},
		{
			name:    "complexity is rejected before 2.6",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.5-flare", Prompt: "a vase", Complexity: asyncStringPtr("high")},
			wantErr: "only supported by GPT Image 2.6",
		},
		{
			name:    "gif is rejected before 2.6",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.5-flare", Prompt: "a lantern", OutputFormat: asyncStringPtr("gif")},
			wantErr: "only supported by GPT Image 2.6",
		},
		{
			name:    "markup is rejected before 2.5",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2", Prompt: "recolor", Images: []string{reference}, Markup: &dto.ImageReference{ImageURL: asyncStringPtr(reference)}},
			wantErr: "only supported by GPT Image 2.5",
		},
		{
			name: "markup excludes mask",
			request: &dto.AsyncImageRequest{
				Model: "gpt-image-2.6-flare", Prompt: "recolor", Images: []string{reference},
				Markup: &dto.ImageReference{ImageURL: asyncStringPtr(reference)},
				Mask:   &dto.ImageReference{ImageURL: asyncStringPtr(reference)},
			},
			wantErr: "markup cannot be combined with mask",
		},
		{
			name:    "frame timing needs gif",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.6-flare", Prompt: "a lantern", OutputFormat: asyncStringPtr("png"), FrameCount: asyncIntPtr(9)},
			wantErr: "only supported with output_format gif",
		},
		{
			name:    "opaque background is accepted",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.6-flare", Prompt: "a vase", Background: asyncStringPtr("opaque")},
		},
		{
			name:    "transparent background still needs png or webp",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.6-flare", Prompt: "a vase", Background: asyncStringPtr("transparent"), OutputFormat: asyncStringPtr("jpeg")},
			wantErr: "png or webp",
		},
		{
			name:    "n is bounded before billing",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.6-flare", Prompt: "a vase", N: asyncUintPtr(dto.MaxImageN + 1)},
			wantErr: "n must be an integer between 1 and",
		},
		{
			name:    "custom size honours the documented bounds",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.6-flare", Prompt: "a vase", Size: "1000x1000"},
			wantErr: "multiples of 16",
		},
		{
			name:    "mask must carry a single reference",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.6-flare", Prompt: "recolor", Images: []string{reference}, Mask: &dto.ImageReference{}},
			wantErr: "mask must provide exactly one of file_id or image_url",
		},
		{
			name:    "markup must carry a single reference",
			request: &dto.AsyncImageRequest{Model: "gpt-image-2.6-flare", Prompt: "recolor", Images: []string{reference}, Markup: &dto.ImageReference{}},
			wantErr: "markup must provide exactly one of file_id or image_url",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAsyncImageRequest(tc.request)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestAsyncImageRequestRoutesMarkupThroughEdits guards the routing rule: markup
// annotates an input image, so it can never be served by the generations call.
func TestAsyncImageRequestRoutesMarkupThroughEdits(t *testing.T) {
	markupRaw, err := common.Marshal(dto.ImageReference{ImageURL: asyncStringPtr("https://upload.example.com/annotation.png")})
	require.NoError(t, err)

	request := &dto.ImageRequest{Model: "gpt-image-2.6-flare", Markup: markupRaw}
	assert.True(t, asyncImageRequestUsesEdits(request))

	generationsOnly := &dto.ImageRequest{Model: "gpt-image-2.6-flare"}
	assert.False(t, asyncImageRequestUsesEdits(generationsOnly))
}
