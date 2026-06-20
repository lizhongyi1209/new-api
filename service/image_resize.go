package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// CompressImageToLimit decodes an image (png/jpeg/gif/webp), and if its encoded
// size exceeds maxBytes, repeatedly downscales (and, for JPEG output, lowers
// quality) until it fits or no further reduction is possible. It returns the
// final bytes, the output mime type, and whether any re-encode happened.
//
// PNG sources stay PNG; everything else is flattened onto a white background and
// encoded as JPEG, which is what shrinks photographic content the most. When the
// input is already under maxBytes the original bytes are returned untouched.
func CompressImageToLimit(raw []byte, maxBytes int) ([]byte, string, bool, error) {
	if maxBytes <= 0 {
		return raw, "", false, fmt.Errorf("invalid size limit")
	}

	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		// image.Decode lacks webp; fall back to the x/image decoder.
		if webpImg, werr := webp.Decode(bytes.NewReader(raw)); werr == nil {
			img, format = webpImg, "webp"
		} else {
			return nil, "", false, fmt.Errorf("unsupported or corrupt image: %w", err)
		}
	}

	isPNG := format == "png"
	outMime := "image/jpeg"
	if isPNG {
		outMime = "image/png"
	}

	// Already small enough: keep the original bytes and its real mime type.
	if len(raw) <= maxBytes {
		return raw, "image/" + normalizeFormatName(format), false, nil
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	quality := 90

	// Try up to a handful of passes: shrink dimensions ~15% each round, and for
	// JPEG also step the quality down, until the encoded size fits.
	for attempt := 0; attempt < 12; attempt++ {
		encoded, err := encodeAtSize(img, isPNG, width, height, quality)
		if err != nil {
			return nil, "", false, err
		}
		if len(encoded) <= maxBytes {
			return encoded, outMime, true, nil
		}

		// Reduce for the next pass.
		width = width * 85 / 100
		height = height * 85 / 100
		if !isPNG && quality > 50 {
			quality -= 10
		}
		if width < 300 || height < 300 {
			// Don't go below Tencent's 300px minimum; return the best effort.
			return encoded, outMime, true, nil
		}
	}

	// Fallback: return whatever the last attempt produced.
	encoded, err := encodeAtSize(img, isPNG, width, height, quality)
	if err != nil {
		return nil, "", false, err
	}
	return encoded, outMime, true, nil
}

// encodeAtSize scales src to width x height and encodes it as PNG (when isPNG)
// or JPEG at the given quality.
func encodeAtSize(src image.Image, isPNG bool, width, height, quality int) ([]byte, error) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	dstRect := image.Rect(0, 0, width, height)
	var buf bytes.Buffer

	if isPNG {
		dst := image.NewNRGBA(dstRect)
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&buf, dst); err != nil {
			return nil, fmt.Errorf("png encode failed: %w", err)
		}
		return buf.Bytes(), nil
	}

	dst := image.NewRGBA(dstRect)
	xdraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, xdraw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("jpeg encode failed: %w", err)
	}
	return buf.Bytes(), nil
}

func normalizeFormatName(format string) string {
	if format == "jpeg" {
		return "jpeg"
	}
	if format == "" {
		return "png"
	}
	return format
}

// EncodeImageBase64 is a small convenience wrapper used by upload handlers.
func EncodeImageBase64(raw []byte) string {
	return base64.StdEncoding.EncodeToString(raw)
}
