package bottools

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"

	"golang.org/x/image/draw"
)

const (
	BannerWidth  = 640
	BannerHeight = 85
)

// NormalizeBannerImage resizes an image to fill the banner dimensions and center-crops when needed.
func NormalizeBannerImage(img image.Image) ([]byte, string, error) {
	bounds := img.Bounds()
	sourceWidth := bounds.Dx()
	sourceHeight := bounds.Dy()
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return nil, "", fmt.Errorf("invalid image dimensions: %dx%d", sourceWidth, sourceHeight)
	}

	result := image.NewRGBA(image.Rect(0, 0, BannerWidth, BannerHeight))
	feedback := ""

	if sourceWidth == BannerWidth && sourceHeight == BannerHeight {
		draw.Draw(result, result.Bounds(), img, bounds.Min, draw.Src)
	} else {
		scale := math.Max(float64(BannerWidth)/float64(sourceWidth), float64(BannerHeight)/float64(sourceHeight))
		scaledWidth := int(math.Ceil(float64(sourceWidth) * scale))
		scaledHeight := int(math.Ceil(float64(sourceHeight) * scale))
		if scaledWidth < BannerWidth {
			scaledWidth = BannerWidth
		}
		if scaledHeight < BannerHeight {
			scaledHeight = BannerHeight
		}

		scaled := image.NewRGBA(image.Rect(0, 0, scaledWidth, scaledHeight))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, bounds, draw.Over, nil)

		offsetX := (scaledWidth - BannerWidth) / 2
		offsetY := (scaledHeight - BannerHeight) / 2
		draw.Draw(result, result.Bounds(), scaled, image.Point{X: offsetX, Y: offsetY}, draw.Src)

		if scaledWidth == BannerWidth && scaledHeight == BannerHeight {
			feedback = fmt.Sprintf("Image was resized from %dx%d to %dx%d.", sourceWidth, sourceHeight, BannerWidth, BannerHeight)
		} else {
			feedback = fmt.Sprintf("Image was resized from %dx%d to %dx%d, then center-cropped to %dx%d.", sourceWidth, sourceHeight, scaledWidth, scaledHeight, BannerWidth, BannerHeight)
		}
	}

	var pngBuffer bytes.Buffer
	if err := png.Encode(&pngBuffer, result); err != nil {
		return nil, "", err
	}

	return pngBuffer.Bytes(), feedback, nil
}