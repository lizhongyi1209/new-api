package dto

import (
	"fmt"
	"strconv"
	"strings"
)

// gptImage2Minor resolves how far behind "gpt-image-2" a model name is: 0 for
// the base model and its aliases (gpt-image-2-c-sp), 5 for the 2.5 series, 6
// for the 2.6 series. ok is false for every other model, including names such
// as gpt-image-20 that merely share the prefix.
func gptImage2Minor(model string) (minor int, ok bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if !strings.HasPrefix(model, "gpt-image-2") {
		return 0, false
	}
	rest := strings.TrimPrefix(model, "gpt-image-2")
	switch {
	case rest == "":
		return 0, true
	case strings.HasPrefix(rest, "-"):
		// Alias suffixes such as -c-sp, -c-sd, -sp and -sd.
		return 0, true
	case strings.HasPrefix(rest, "."):
	default:
		// gpt-image-20 and other look-alikes are not in this family.
		return 0, false
	}
	rest = strings.TrimPrefix(rest, ".")
	digits := 0
	for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return 0, false
	}
	minor, err := strconv.Atoi(rest[:digits])
	if err != nil {
		return 0, false
	}
	return minor, true
}

// IsGPTImage2FamilyModel reports whether model belongs to the gpt-image-2
// family: gpt-image-2 and its aliases, plus the 2.5 and 2.6 series.
func IsGPTImage2FamilyModel(model string) bool {
	_, ok := gptImage2Minor(model)
	return ok
}

// SupportsExtendedImageQuality reports whether model accepts the xhigh and max
// quality tiers, which the 2.5 series introduced.
func SupportsExtendedImageQuality(model string) bool {
	minor, ok := gptImage2Minor(model)
	return ok && minor >= 5
}

// SupportsImageMarkup reports whether model accepts markup-guided edits, which
// the 2.5 series introduced.
func SupportsImageMarkup(model string) bool {
	minor, ok := gptImage2Minor(model)
	return ok && minor >= 5
}

// SupportsImageComplexity reports whether model accepts the complexity hint,
// which the 2.6 series introduced.
func SupportsImageComplexity(model string) bool {
	minor, ok := gptImage2Minor(model)
	return ok && minor >= 6
}

// SupportsImageVectorAndAnimationOutput reports whether model can emit SVG and
// GIF results, which the 2.6 series introduced.
func SupportsImageVectorAndAnimationOutput(model string) bool {
	minor, ok := gptImage2Minor(model)
	return ok && minor >= 6
}

// RequiresOmittedInputFidelity reports whether input_fidelity must be stripped
// before forwarding. The base gpt-image-2 model always processes image input at
// high fidelity and rejects the parameter.
func RequiresOmittedInputFidelity(model string) bool {
	minor, ok := gptImage2Minor(model)
	return ok && minor == 0
}

// ImageGenerationTarget distinguishes the endpoint a request is headed for,
// because several parameters are valid on only one of them.
type ImageGenerationTarget int

const (
	ImageTargetGenerations ImageGenerationTarget = iota
	ImageTargetEdits
)

// ImageGenerationOptions carries the documented GPT Image parameters that need
// cross-field validation. Absent optional values stay nil so "not sent" and an
// explicit zero remain distinguishable. Fields that do not apply to a model are
// still validated for shape, so a typo fails here instead of upstream.
type ImageGenerationOptions struct {
	Model             string
	Target            ImageGenerationTarget
	N                 *uint
	Size              string
	Quality           string
	Background        string
	OutputFormat      string
	OutputCompression *int
	PartialImages     *int
	Stream            bool
	Markup            bool
	Mask              bool
	Complexity        string
	FrameCount        *int
	FrameDurationMs   *int
}

// Validate enforces the documented GPT Image parameter contract before any
// quota is reserved. Values are checked against their own bounds first, then
// against the model and the endpoint that will receive them.
//
// The contract describes gpt-image models only. Other providers reuse the same
// parameter names with their own vocabularies (for example output_format on
// Replicate), so nothing is enforced for them here.
func (o ImageGenerationOptions) Validate() error {
	if o.N != nil && *o.N > MaxImageN {
		return fmt.Errorf("n must be an integer between 1 and %d", MaxImageN)
	}
	if !isGPTImageModel(o.Model) {
		return nil
	}

	format := strings.ToLower(strings.TrimSpace(o.OutputFormat))
	switch format {
	case "", "png", "jpeg", "webp", "svg", "gif":
	default:
		return fmt.Errorf("output_format must be one of png, jpeg, webp, svg, gif")
	}
	if format == "svg" || format == "gif" {
		if !SupportsImageVectorAndAnimationOutput(o.Model) {
			return fmt.Errorf("output_format %s is only supported by GPT Image 2.6 and later", format)
		}
	}
	// SVG output is generations-only. GIF is valid on both generations and
	// edits; reference stills for an animation belong on the edit endpoint.
	// frame_count and frame_duration_ms stay GIF-only and apply on either target.
	if format == "svg" && o.Target == ImageTargetEdits {
		return fmt.Errorf("output_format svg is only supported on /v1/images/generations")
	}

	background := strings.ToLower(strings.TrimSpace(o.Background))
	switch background {
	case "", "auto", "transparent", "opaque":
	default:
		return fmt.Errorf("background must be one of auto, transparent, opaque")
	}
	if background == "transparent" && format != "" && format != "png" && format != "webp" {
		return fmt.Errorf("output_format must be png or webp when background is transparent")
	}

	quality := strings.ToLower(strings.TrimSpace(o.Quality))
	switch quality {
	case "xhigh", "max":
		if !SupportsExtendedImageQuality(o.Model) {
			return fmt.Errorf("quality %s is only supported by GPT Image 2.5 and later", quality)
		}
	}

	if o.OutputCompression != nil {
		if *o.OutputCompression < 0 || *o.OutputCompression > MaxImageOutputCompression {
			return fmt.Errorf("output_compression must be an integer between 0 and %d", MaxImageOutputCompression)
		}
		if format == "svg" || format == "gif" {
			return fmt.Errorf("output_compression is not supported by output_format %s", format)
		}
	}

	if o.PartialImages != nil {
		if *o.PartialImages < MinImagePartialImages || *o.PartialImages > MaxImagePartialImages {
			return fmt.Errorf("partial_images must be an integer between %d and %d", MinImagePartialImages, MaxImagePartialImages)
		}
		if format == "svg" || format == "gif" {
			return fmt.Errorf("partial_images is not supported by output_format %s", format)
		}
	}
	if o.Stream && (format == "svg" || format == "gif") {
		return fmt.Errorf("stream is not supported by output_format %s", format)
	}

	// frame_count and frame_duration_ms describe GIF frames only.
	if o.FrameCount != nil || o.FrameDurationMs != nil {
		if format != "gif" {
			return fmt.Errorf("frame_count and frame_duration_ms are only supported with output_format gif")
		}
		if o.FrameCount != nil && (*o.FrameCount < MinImageFrameCount || *o.FrameCount > MaxImageFrameCount) {
			return fmt.Errorf("frame_count must be an integer between %d and %d", MinImageFrameCount, MaxImageFrameCount)
		}
		if o.FrameDurationMs != nil && (*o.FrameDurationMs < MinImageFrameDurationMs || *o.FrameDurationMs > MaxImageFrameDurationMs) {
			return fmt.Errorf("frame_duration_ms must be an integer between %d and %d", MinImageFrameDurationMs, MaxImageFrameDurationMs)
		}
	}

	// A single SVG frame is a vector document, not a raster, so n must be 1.
	if format == "svg" && o.N != nil && *o.N != 1 {
		return fmt.Errorf("n must be 1 when output_format is svg")
	}

	complexity := strings.ToLower(strings.TrimSpace(o.Complexity))
	if complexity != "" {
		switch complexity {
		case "low", "medium", "high":
		default:
			return fmt.Errorf("complexity must be one of low, medium, high")
		}
		if !SupportsImageComplexity(o.Model) {
			return fmt.Errorf("complexity is only supported by GPT Image 2.6 and later")
		}
	}

	if o.Markup {
		if !SupportsImageMarkup(o.Model) {
			return fmt.Errorf("markup is only supported by GPT Image 2.5 and later")
		}
		if o.Mask {
			return fmt.Errorf("markup cannot be combined with mask")
		}
	}

	return ValidateImageSize(o.Model, o.Size)
}

// isGPTImageModel reports whether model belongs to the gpt-image family, which
// is the family the documented parameter contract describes.
func isGPTImageModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-image")
}

// ValidateImageSize enforces the documented GPT Image resolution bounds. Sizes
// that are not WxH (auto, and the 1K/2K/4K tiers other providers accept) are
// left to the provider-specific validators.
func ValidateImageSize(model, size string) error {
	if !isGPTImageModel(model) {
		return nil
	}
	size = strings.ToLower(strings.TrimSpace(size))
	if size == "" || size == "auto" {
		return nil
	}
	parts := strings.SplitN(size, "x", 2)
	if len(parts) != 2 {
		return nil
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return fmt.Errorf("size must be WIDTHxHEIGHT with positive integers, e.g. 1024x1024")
	}
	if width%16 != 0 || height%16 != 0 {
		return fmt.Errorf("size width and height must be multiples of 16")
	}
	if width > imageMaxEdgePixels || height > imageMaxEdgePixels {
		return fmt.Errorf("size width and height must not exceed %d pixels", imageMaxEdgePixels)
	}
	// The aspect ratio must stay between 1:3 and 3:1.
	if width > height*3 || height > width*3 {
		return fmt.Errorf("size aspect ratio must be between 1:3 and 3:1")
	}
	totalPixels := width * height
	if totalPixels < imageMinTotalPixels || totalPixels > imageMaxTotalPixels {
		return fmt.Errorf("size total pixels must be between %d and %d", imageMinTotalPixels, imageMaxTotalPixels)
	}
	return nil
}
