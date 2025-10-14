package bottools

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v33/github"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"

	"golang.org/x/image/draw"
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

type styleData struct {
	name  string
	id    string
	image image.Image
}

// GenerateBanner creates a banner image with a background, overlay image, and text
func GenerateBanner(ID string, eggName string, text string) {
	// 1. Load Images
	styleArray := []string{"", "c", "a", "f", "l"}
	// Check if any of the style images already exist
	// make sure the output path exists, and create it if it doesn't
	if _, err := os.Stat(config.BannerOutputPath); os.IsNotExist(err) {
		err = os.MkdirAll(config.BannerOutputPath, 0755)
		if err != nil {
			fmt.Println("Error creating output directory:", err)
			return
		}
	}

	outputImagePath := fmt.Sprintf("%s/b-%s.png", config.BannerOutputPath, ID)
	allExist := true
	for _, style := range styleArray {
		testImagePath := fmt.Sprintf("%s/b%s-%s.png", config.BannerOutputPath, style, ID)
		if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
			allExist = false
			break
		} else if err != nil {
			fmt.Println("Error checking output image:", err)
			return
		}
	}

	// if all images already exist, return
	if allExist {
		//fmt.Println("All images already exist")
		return
	}
	log.Printf("Creating banner in %s", outputImagePath)
	bgImagePath := config.BannerPath + "/banner.png"
	cleanEggID := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(eggName), " ", ""), "-", "")
	cleanEggID = strings.ReplaceAll(cleanEggID, "_", "")

	overlayImagePath := fmt.Sprintf("egg_%s.png", cleanEggID)

	// Test if the overlay image exists
	if _, err := os.Stat(config.BannerPath + "/" + overlayImagePath); os.IsNotExist(err) {
		err := DownloadLatestEggImages(config.BannerPath)
		if err != nil {
			fmt.Println("Error downloading latest egg images:", err)
		}
	}

	chillImg, err := loadImage(config.BannerPath + "/chill.png")
	if err != nil {
		fmt.Println("Error loading chill image:", err)
		return
	}

	acoImg, err := loadImage(config.BannerPath + "/aco.png")
	if err != nil {
		fmt.Println("Error loading aco image:", err)
		return
	}

	fastrunImg, err := loadImage(config.BannerPath + "/fastrun.png")
	if err != nil {
		fmt.Println("Error loading fastrun image:", err)
		return
	}

	leaderboardImg, err := loadImage(config.BannerPath + "/leaderboard.png")
	if err != nil {
		fmt.Println("Error loading leaderboard image:", err)
		return
	}

	sData := []styleData{
		{"chill", "c", chillImg},
		{"aco", "a", acoImg},
		{"fastrun", "f", fastrunImg},
		{"leaderboard", "l", leaderboardImg},
	}

	bgImage, err := loadImage(bgImagePath)
	if err != nil {
		fmt.Println("Error loading background image:", err)
		return
	}

	haveEggImg := true
	overlayImageOrig, err := loadImage(config.BannerPath + "/" + overlayImagePath)
	if err != nil {
		fmt.Println("Error loading overlay image:", err)
		haveEggImg = false
	}
	// I want to make overlayImage a 128 by 128 image
	overlayImage := image.NewRGBA(image.Rect(0, 0, 128, 128))
	draw.NearestNeighbor.Scale(overlayImage, overlayImage.Rect, overlayImageOrig, overlayImageOrig.Bounds(), draw.Over, nil)

	// 2. Create Canvas (same size as background)
	bounds := bgImage.Bounds()
	compositeImage := image.NewRGBA(bounds)

	// 3. Draw Background
	draw.Draw(compositeImage, bounds, bgImage, image.Point{}, draw.Src)

	// 4. Draw Overlay (example position, adjust as needed)
	if haveEggImg {
		overlayRect := image.Rect(0, 0, 48+overlayImage.Bounds().Dx(), 48+overlayImage.Bounds().Dy()) // Example position
		draw.Draw(compositeImage, overlayRect, overlayImage, image.Point{}, draw.Over)                // Use draw.Over for overlay
	}

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
	// Create text outline effect
	textColor := color.RGBA{255, 255, 255, 255} // White text
	outlineColor := color.RGBA{0, 0, 0, 255}    // Black outline
	outlineWidth := 2

	// Calculate text width and adjust font size if necessary
	textWidth := font.MeasureString(face, text).Ceil()
	maxWidth := bounds.Dx() - 138 - 20 // Available width minus x position and some padding

	adjustedFace := face

	if textWidth > maxWidth {
		// Calculate scale factor and reduce font size
		scaleFactor := float64(maxWidth) / float64(textWidth)
		adjustedFontSize := fontSize * scaleFactor

		var err error
		adjustedFace, err = loadFontFile(fontFile, adjustedFontSize, dpi)
		if err != nil {
			log.Printf("Error loading adjusted font: %v", err)
			adjustedFace = face // Fallback to original
		} else {
			defer func() {
				if err := adjustedFace.Close(); err != nil {
					log.Printf("Failed to close adjusted font face: %v", err)
				}
			}()
		}
	}

	// Draw outline by rendering text at multiple offset positions
	for dx := -outlineWidth; dx <= outlineWidth; dx++ {
		for dy := -outlineWidth; dy <= outlineWidth; dy++ {
			if dx != 0 || dy != 0 { // Skip center position
				addLabel(compositeImage, 138+dx, 68+dy, text, adjustedFace, outlineColor)
			}
		}
	}

	// Draw the main text on top
	addLabel(compositeImage, 138, 68, text, adjustedFace, textColor)

	// For each style create an image in the lower left corner and save it
	for _, style := range sData {
		// Create a copy of the composite image for this style
		styleImage := image.NewRGBA(compositeImage.Bounds())
		draw.Draw(styleImage, compositeImage.Bounds(), compositeImage, image.Point{}, draw.Src)
		styleRect := image.Rect(0, bounds.Max.Y-style.image.Bounds().Dy(), style.image.Bounds().Dx(), bounds.Max.Y)
		draw.Draw(styleImage, styleRect, style.image, image.Point{}, draw.Over)
		styleImagePath := fmt.Sprintf("%s/b%s-%s.png", config.BannerOutputPath, style.id, ID)
		err = saveImage(styleImagePath, styleImage)
		if err != nil {
			fmt.Println("Error saving output image:", err)
			return
		}
	}

	// 7. Encode and Save the image without the style overlay
	err = saveImage(outputImagePath, compositeImage)

	if err != nil {
		fmt.Println("Error saving output image:", err)
		return
	}
	fmt.Println("Images created successfully:", outputImagePath)
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

// DownloadLatestEggImages downloads the latest image files from a specific GitHub repository directory.
func DownloadLatestEggImages(localDownloadDir string) error {
	owner := "mkmccarty"
	repo := "TokenTimeBoostBot"
	repoPath := "emoji"
	client := github.NewClient(nil)
	ctx := context.Background()

	// Ensure the local download directory exists.
	if err := os.MkdirAll(localDownloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create local directory %s: %w", localDownloadDir, err)
	}

	// Get the contents of the specified repository directory.
	_, directoryContents, _, err := client.Repositories.GetContents(ctx, owner, repo, repoPath, &github.RepositoryContentGetOptions{
		Ref: "main", // Always use the main branch for the latest content
	})
	if err != nil {
		return fmt.Errorf("error getting repository contents: %w", err)
	}

	for _, content := range directoryContents {
		// Only process files.
		if content.GetType() == "file" {
			downloadURL := content.GetDownloadURL()
			if downloadURL == "" {
				continue // Skip if there's no download URL.
			}
			// Only want banner related assets
			stringsToCheck := []string{"egg_", "banner", "Always Together", "aco", "chill", "fastrun", "leaderboard"}
			found := false
			for _, str := range stringsToCheck {
				if strings.Contains(content.GetName(), str) {
					found = true
					break
				}
			}
			if !found {
				continue
			}

			// If the file already exists, don't need to download it
			localFilePath := filepath.Join(localDownloadDir, content.GetName())
			if _, err := os.Stat(localFilePath); err == nil {
				//log.Printf("File %s already exists, skipping download.\n", localFilePath)
				continue
			}

			log.Printf("Downloading %s...\n", content.GetName())

			// Make an HTTP request to download the file.
			resp, err := http.Get(downloadURL)
			if err != nil {
				log.Printf("Error downloading file %s: %v\n", content.GetName(), err)
				continue
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					// Handle the error appropriately, e.g., logging or taking corrective actions
					log.Printf("Failed to close: %v", err)
				}
			}()

			outFile, err := os.Create(localFilePath)
			if err != nil {
				log.Printf("Error creating local file %s: %v\n", localFilePath, err)
				continue
			}
			defer func() {
				if err := outFile.Close(); err != nil {
					// Handle the error appropriately, e.g., logging or taking corrective actions
					log.Printf("Failed to close: %v", err)
				}
			}()

			// Copy the downloaded content to the local file.
			if _, err := io.Copy(outFile, resp.Body); err != nil {
				log.Printf("Error writing to file %s: %v\n", localFilePath, err)
				continue
			}
			log.Printf("Successfully downloaded %s.\n", content.GetName())
		}
	}
	return nil
}
