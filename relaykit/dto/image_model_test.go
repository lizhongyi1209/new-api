package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(value int) *int { return &value }

func uintPtr(value uint) *uint { return &value }

func TestGPTImageModelFamilyDetection(t *testing.T) {
	for _, tc := range []struct {
		model          string
		family         bool
		extended       bool
		markup         bool
		complexity     bool
		vectorAnimated bool
		omitFidelity   bool
	}{
		{model: "gpt-image-2", family: true, omitFidelity: true},
		{model: "GPT-IMAGE-2-C-SP", family: true, omitFidelity: true},
		{model: "gpt-image-2.5-sunburst", family: true, extended: true, markup: true},
		{model: "gpt-image-2.5-flare", family: true, extended: true, markup: true},
		{model: "gpt-image-2.6-sunburst", family: true, extended: true, markup: true, complexity: true, vectorAnimated: true},
		{model: "gpt-image-2.6-flare", family: true, extended: true, markup: true, complexity: true, vectorAnimated: true},
		{model: " gpt-image-2.6-flare ", family: true, extended: true, markup: true, complexity: true, vectorAnimated: true},
		{model: "gpt-image-20"},
		{model: "gpt-image-1.5"},
		{model: "dall-e-3"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			assert.Equal(t, tc.family, IsGPTImage2FamilyModel(tc.model), "family")
			assert.Equal(t, tc.extended, SupportsExtendedImageQuality(tc.model), "extended quality")
			assert.Equal(t, tc.markup, SupportsImageMarkup(tc.model), "markup")
			assert.Equal(t, tc.complexity, SupportsImageComplexity(tc.model), "complexity")
			assert.Equal(t, tc.vectorAnimated, SupportsImageVectorAndAnimationOutput(tc.model), "svg/gif")
			assert.Equal(t, tc.omitFidelity, RequiresOmittedInputFidelity(tc.model), "omit input_fidelity")
		})
	}
}

func TestImageGenerationOptionsValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options ImageGenerationOptions
		wantErr string
	}{
		{
			name:    "svg requires 2.6",
			options: ImageGenerationOptions{Model: "gpt-image-2.5-flare", OutputFormat: "svg", N: uintPtr(1)},
			wantErr: "only supported by GPT Image 2.6",
		},
		{
			name:    "gif requires 2.6",
			options: ImageGenerationOptions{Model: "gpt-image-2.5-flare", OutputFormat: "gif"},
			wantErr: "only supported by GPT Image 2.6",
		},
		{
			name:    "svg is generations only",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", Target: ImageTargetEdits, OutputFormat: "svg", N: uintPtr(1)},
			wantErr: "only supported on /v1/images/generations",
		},
		{
			name:    "svg needs a single frame",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", OutputFormat: "svg", N: uintPtr(2)},
			wantErr: "n must be 1 when output_format is svg",
		},
		{
			name:    "svg rejects streaming",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", OutputFormat: "svg", N: uintPtr(1), Stream: true},
			wantErr: "stream is not supported",
		},
		{
			name:    "svg rejects partial images",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", OutputFormat: "svg", N: uintPtr(1), PartialImages: intPtr(2)},
			wantErr: "partial_images is not supported",
		},
		{
			name:    "svg rejects compression",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", OutputFormat: "svg", N: uintPtr(1), OutputCompression: intPtr(50)},
			wantErr: "output_compression is not supported",
		},
		{
			name:    "gif rejects streaming",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", OutputFormat: "gif", Stream: true},
			wantErr: "stream is not supported",
		},
		{
			name:    "transparent needs png or webp",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", Background: "transparent", OutputFormat: "jpeg"},
			wantErr: "png or webp",
		},
		{
			name:    "xhigh requires 2.5 or later",
			options: ImageGenerationOptions{Model: "gpt-image-2", Quality: "xhigh"},
			wantErr: "only supported by GPT Image 2.5",
		},
		{
			name:    "complexity requires 2.6",
			options: ImageGenerationOptions{Model: "gpt-image-2.5-flare", Complexity: "high"},
			wantErr: "only supported by GPT Image 2.6",
		},
		{
			name:    "complexity rejects unknown values",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", Complexity: "extreme"},
			wantErr: "complexity must be one of",
		},
		{
			name:    "markup requires 2.5 or later",
			options: ImageGenerationOptions{Model: "gpt-image-2", Markup: true},
			wantErr: "only supported by GPT Image 2.5",
		},
		{
			name:    "markup excludes mask",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", Markup: true, Mask: true},
			wantErr: "markup cannot be combined with mask",
		},
		{
			name:    "frame count needs gif",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", OutputFormat: "png", FrameCount: intPtr(9)},
			wantErr: "only supported with output_format gif",
		},
		{
			name:    "frame count bound",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", OutputFormat: "gif", FrameCount: intPtr(65)},
			wantErr: "frame_count must be an integer between",
		},
		{
			name:    "frame duration bound",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", OutputFormat: "gif", FrameDurationMs: intPtr(10)},
			wantErr: "frame_duration_ms must be an integer between",
		},
		{
			name:    "partial images bound",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", PartialImages: intPtr(4)},
			wantErr: "partial_images must be an integer between",
		},
		{
			name:    "output compression bound",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", OutputFormat: "jpeg", OutputCompression: intPtr(101)},
			wantErr: "output_compression must be an integer between",
		},
		{
			name:    "size must be a multiple of 16",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", Size: "1000x1000"},
			wantErr: "multiples of 16",
		},
		{
			name:    "size edge bound",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", Size: "4096x2048"},
			wantErr: "must not exceed 3840",
		},
		{
			name:    "size aspect bound",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", Size: "3840x1024"},
			wantErr: "aspect ratio must be between 1:3 and 3:1",
		},
		{
			name:    "size pixel floor",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", Size: "512x512"},
			wantErr: "total pixels must be between",
		},
		{
			name:    "documented 4K size is accepted",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", Size: "3840x2160"},
		},
		{
			name:    "2.6 accepts svg, gif frames and complexity",
			options: ImageGenerationOptions{Model: "gpt-image-2.6-flare", OutputFormat: "gif", FrameCount: intPtr(9), FrameDurationMs: intPtr(200), Complexity: "high"},
		},
		{
			name:    "non gpt models keep their own size vocabulary",
			options: ImageGenerationOptions{Model: "gemini-3-pro-image", Size: "4K"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.options.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
