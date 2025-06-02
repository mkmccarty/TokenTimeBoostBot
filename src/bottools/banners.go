package bottools

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func loadFontFile(name string, size, dpi float64) (font.Face, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read font file: %w", err)
	}

	col, err := sfnt.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font data: %w", err)
	}

	face, err := opentype.NewFace(col, &opentype.FaceOptions{
		Size:    size,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create font face: %w", err)
	}

	return face, nil
}

// GenerateBanner creates a banner image with a background, overlay image, and text
func GenerateBanner(ID string, eggName string, text string) {
	// 1. Load Images
	outputImagePath := fmt.Sprintf("%s/b-%s.png", config.BannerOutputPath, ID)
	// if the image already exists, return
	if _, err := os.Stat(outputImagePath); err == nil {
		//fmt.Println("Image already exists:", outputImagePath)
		return
	} else if !os.IsNotExist(err) {
		fmt.Println("Error checking output image:", err)
		return
	}
	log.Printf("Creating banner in %s", outputImagePath)
	bgImagePath := config.BannerPath + "/banner.png"
	cleanEggID := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(eggName), " ", ""), "-", "")
	cleanEggID = strings.ReplaceAll(cleanEggID, "_", "")

	overlayImagePath := fmt.Sprintf("egg_%s.png", cleanEggID)

	bgImage, err := loadImage(bgImagePath)
	if err != nil {
		fmt.Println("Error loading background image:", err)
		return
	}

	overlayImage, err := loadImage(config.BannerPath + "/" + overlayImagePath)
	if err != nil {
		fmt.Println("Error loading overlay image:", err)
		return
	}

	// 2. Create Canvas (same size as background)
	bounds := bgImage.Bounds()
	compositeImage := image.NewRGBA(bounds)

	// 3. Draw Background
	draw.Draw(compositeImage, bounds, bgImage, image.Point{}, draw.Src)

	// 4. Draw Overlay (example position, adjust as needed)
	overlayRect := image.Rect(0, 0, 48+overlayImage.Bounds().Dx(), 48+overlayImage.Bounds().Dy()) // Example position
	draw.Draw(compositeImage, overlayRect, overlayImage, image.Point{}, draw.Over)                // Use draw.Over for overlay

	// 5. Load Font

	fontFile := config.BannerPath + "/Always Together.otf"
	fontSize := 64.0
	dpi := 72.0

	face, err := loadFontFile(fontFile, fontSize, dpi)
	if err != nil {
		log.Fatalf("Error loading font: %v", err)
	}
	defer func() {
		if err := face.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	// 6. Draw Text
	addLabel(compositeImage, 140, 70, text, face, color.RGBA{0, 0, 0, 255})
	addLabel(compositeImage, 140, 66, text, face, color.RGBA{0, 0, 0, 255})
	addLabel(compositeImage, 136, 66, text, face, color.RGBA{0, 0, 0, 255})
	addLabel(compositeImage, 136, 70, text, face, color.RGBA{0, 0, 0, 255})

	addLabel(compositeImage, 138, 68, text, face, color.RGBA{255, 255, 255, 255})

	// 7. Encode and Save
	err = saveImage(outputImagePath, compositeImage)

	if err != nil {
		fmt.Println("Error saving output image:", err)
		return
	}
	fmt.Println("Image created successfully:", outputImagePath)
}

// Helper function to load an image from a file
func loadImage(filePath string) (image.Image, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := f.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	img, _, err := image.Decode(f)
	return img, err
}

// Helper function to save an image to a file
func saveImage(filePath string, img image.Image) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	ext := filepath.Ext(filePath)
	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Encode(file, img, nil)
	case ".png":
		return png.Encode(file, img)
	default:
		return fmt.Errorf("unsupported image format: %s", ext)
	}
}

// Helper function to add text to an image
func addLabel(img *image.RGBA, x, y int, label string, face font.Face, textColor color.Color) {
	point := fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(textColor),
		Face: face,
		Dot:  point,
	}
	d.DrawString(label)
}
