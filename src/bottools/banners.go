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
	"time"

	"github.com/google/go-github/v33/github"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

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

// getCurrentSeason returns the current season based on the month
func getCurrentSeason(t time.Time) string {
	month := t.Month()
	if month >= 3 && month <= 5 {
		return "spring"
	} else if month >= 6 && month <= 8 {
		return "summer"
	} else if month >= 9 && month <= 11 {
		return "fall"
	}
	return "winter"
}

type styleData struct {
	name  string
	id    string
	image image.Image
}

// SyncCustomBannerCallback is a function hook to sync custom banners from the database to disk.
var SyncCustomBannerCallback func(userID string, destPath string) bool

// GenerateBanner creates a banner image with a background, overlay image, and text
func GenerateBanner(ID string, eggName string, text string, creatorID string, styleOverride string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("GenerateBanner recovered from panic for %s: %v", ID, r)
		}
	}()

	// 1. Load Images
	styleArray := []string{"c", "a", "f", "l"}
	if styleOverride != "" {
		styleArray = []string{styleOverride}
	}
	// Check if any of the style images already exist
	// make sure the output path exists, and create it if it doesn't
	if _, err := os.Stat(config.BannerOutputPath); os.IsNotExist(err) {
		err = os.MkdirAll(config.BannerOutputPath, 0755)
		if err != nil {
			log.Println("Error creating output directory:", err)
			return
		}
	}

	currentSeason := getCurrentSeason(time.Now())
	allExistAndFresh := true

	hasCustomBanner := false
	if creatorID != "" {
		bgCustomPath := fmt.Sprintf("%s/banner_%s.png", config.BannerPath, creatorID)
		if SyncCustomBannerCallback != nil {
			hasCustomBanner = SyncCustomBannerCallback(creatorID, bgCustomPath)
		} else {
			if _, err := os.Stat(bgCustomPath); err == nil {
				hasCustomBanner = true
			}
		}
	}

	for _, style := range styleArray {
		if hasCustomBanner {
			customImgPath := fmt.Sprintf("%s/%s-b%s-%s.png", config.BannerOutputPath, ID, style, creatorID)
			info, err := os.Stat(customImgPath)
			if os.IsNotExist(err) {
				allExistAndFresh = false
				break
			}
			bgInfo, _ := os.Stat(fmt.Sprintf("%s/banner_%s.png", config.BannerPath, creatorID))
			if bgInfo != nil && info.ModTime().Before(bgInfo.ModTime()) {
				allExistAndFresh = false
				break
			}
		} else {
			seasonImgPath := fmt.Sprintf("%s/%s-b%s.png", config.BannerOutputPath, ID, style)

			info, err := os.Stat(seasonImgPath)
			if os.IsNotExist(err) || getCurrentSeason(info.ModTime()) != currentSeason {
				allExistAndFresh = false
				break
			}

			if style == "f" || style == "l" {
				spaceImgPath := fmt.Sprintf("%s/%s-b%s-space.png", config.BannerOutputPath, ID, style)
				if _, err := os.Stat(spaceImgPath); os.IsNotExist(err) {
					allExistAndFresh = false
					break
				}
			}
		}
	}

	// if all images already exist, return
	if allExistAndFresh {
		return
	}
	log.Printf("Creating banners for %s (Season: %s)", ID, currentSeason)
	cleanEggID := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(eggName), " ", ""), "-", "")
	cleanEggID = strings.ReplaceAll(cleanEggID, "_", "")

	overlayImagePath := fmt.Sprintf("egg_%s.png", cleanEggID)

	// Test if the overlay image exists
	if _, err := os.Stat(config.BannerPath + "/" + overlayImagePath); os.IsNotExist(err) {
		err := DownloadLatestEggImages(config.BannerPath)
		if err != nil {
			log.Println("Error downloading latest egg images:", err)
		}
	}

	chillImg, err := loadImage(config.BannerPath + "/chill.png")
	if err != nil {
		log.Println("Error loading chill image:", err)
		return
	}

	acoImg, err := loadImage(config.BannerPath + "/aco.png")
	if err != nil {
		log.Println("Error loading aco image:", err)
		return
	}

	fastrunImg, err := loadImage(config.BannerPath + "/fastrun.png")
	if err != nil {
		log.Println("Error loading fastrun image:", err)
		return
	}

	leaderboardImg, err := loadImage(config.BannerPath + "/leaderboard.png")
	if err != nil {
		log.Println("Error loading leaderboard image:", err)
		return
	}

	sData := []styleData{
		{"chill", "c", chillImg},
		{"aco", "a", acoImg},
		{"fastrun", "f", fastrunImg},
		{"leaderboard", "l", leaderboardImg},
	}

	bgSeasonPath := fmt.Sprintf("%s/banner_%s_640.png", config.BannerPath, currentSeason)
	bgSpacePath := config.BannerPath + "/banner_space_640.png"
	bgCustomPath := fmt.Sprintf("%s/banner_%s.png", config.BannerPath, creatorID)

	if _, err := os.Stat(bgSeasonPath); os.IsNotExist(err) {
		_ = DownloadLatestEggImages(config.BannerPath)
	}
	if _, err := os.Stat(bgSpacePath); os.IsNotExist(err) {
		_ = DownloadLatestEggImages(config.BannerPath)
	}

	seasonStr := ""
	if contract, ok := ei.EggIncContractsAll[ID]; ok {
		if strings.HasPrefix(contract.SeasonID, "winter") {
			seasonStr = "winter"
		} else if strings.HasPrefix(contract.SeasonID, "spring") {
			seasonStr = "spring"
		} else if strings.HasPrefix(contract.SeasonID, "summer") {
			seasonStr = "summer"
		} else if strings.HasPrefix(contract.SeasonID, "fall") {
			seasonStr = "fall"
		}
	}

	haveEggImg := true
	overlayImageOrig, err := loadImage(config.BannerPath + "/" + overlayImagePath)
	if err != nil {
		log.Println("Error loading overlay image:", err)
		haveEggImg = false
	}
	// I want to make overlayImage a 128 by 128 image
	var overlayImage *image.RGBA
	if haveEggImg {
		overlayImage = image.NewRGBA(image.Rect(0, 0, 128, 128))
		draw.NearestNeighbor.Scale(overlayImage, overlayImage.Rect, overlayImageOrig, overlayImageOrig.Bounds(), draw.Over, nil)
	}

	var seasonImgOrig image.Image
	if seasonStr != "" {
		seasonImagePath := fmt.Sprintf("%s.png", seasonStr)
		if _, err := os.Stat(config.BannerPath + "/" + seasonImagePath); os.IsNotExist(err) {
			_ = DownloadLatestEggImages(config.BannerPath)
		}
		seasonImgOrig, _ = loadImage(config.BannerPath + "/" + seasonImagePath)
	}

	fontFile := config.BannerPath + "/Always Together.otf"
	fontSize := 64.0
	dpi := 72.0

	face, err := loadFontFile(fontFile, fontSize, dpi)
	if err != nil {
		log.Printf("Error loading font: %v", err)
		return
	}
	defer func() {
		if err := face.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	type bgDef struct {
		path   string
		suffix string
	}
	var backgrounds []bgDef

	if hasCustomBanner {
		backgrounds = []bgDef{
			{path: bgCustomPath, suffix: "-" + creatorID},
		}
	} else {
		backgrounds = []bgDef{
			{path: bgSeasonPath, suffix: ""},
			{path: bgSpacePath, suffix: "-space"},
		}
	}

	for _, bgInfo := range backgrounds {
		bgImage, err := loadImage(bgInfo.path)
		if err != nil {
			log.Println("Error loading background image:", err)
			continue
		}

		bounds := bgImage.Bounds()
		compositeImage := image.NewRGBA(bounds)

		draw.Draw(compositeImage, bounds, bgImage, image.Point{}, draw.Src)

		if haveEggImg {
			overlayRect := image.Rect(0, 0, 48+overlayImage.Bounds().Dx(), 48+overlayImage.Bounds().Dy())
			draw.Draw(compositeImage, overlayRect, overlayImage, image.Point{}, draw.Over)
		}

		if seasonImgOrig != nil {
			origBounds := seasonImgOrig.Bounds()
			targetHeight := bounds.Dy() * 1 / 2
			targetWidth := origBounds.Dx()
			if origBounds.Dy() > 0 {
				targetWidth = (origBounds.Dx() * targetHeight) / origBounds.Dy()
			}
			scaledSeasonImg := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
			draw.CatmullRom.Scale(scaledSeasonImg, scaledSeasonImg.Rect, seasonImgOrig, origBounds, draw.Over, nil)

			seasonRect := image.Rect(bounds.Max.X-targetWidth-10, 4, bounds.Max.X-10, 4+targetHeight)
			draw.Draw(compositeImage, seasonRect, scaledSeasonImg, image.Point{}, draw.Over)
		}

		textColor := color.RGBA{255, 255, 255, 255}
		outlineColor := color.RGBA{0, 0, 0, 255}
		outlineWidth := 2

		textWidth := font.MeasureString(face, text).Ceil()
		maxWidth := bounds.Dx() - 138 - 20

		adjustedFace := face
		if textWidth > maxWidth {
			scaleFactor := float64(maxWidth) / float64(textWidth)
			adjustedFontSize := fontSize * scaleFactor
			adjFace, err := loadFontFile(fontFile, adjustedFontSize, dpi)
			if err == nil {
				adjustedFace = adjFace
			}
		}

		for dx := -outlineWidth; dx <= outlineWidth; dx++ {
			for dy := -outlineWidth; dy <= outlineWidth; dy++ {
				if dx != 0 || dy != 0 {
					addLabel(compositeImage, 138+dx, 68+dy, text, adjustedFace, outlineColor)
				}
			}
		}

		addLabel(compositeImage, 138, 68, text, adjustedFace, textColor)

		if adjustedFace != face {
			_ = adjustedFace.Close()
		}

		for _, style := range sData {
			if styleOverride != "" && style.id != styleOverride {
				continue
			}
			if bgInfo.suffix == "-space" && style.id != "f" && style.id != "l" {
				continue
			}
			styleImage := image.NewRGBA(compositeImage.Bounds())
			draw.Draw(styleImage, compositeImage.Bounds(), compositeImage, image.Point{}, draw.Src)
			styleRect := image.Rect(0, bounds.Max.Y-style.image.Bounds().Dy(), style.image.Bounds().Dx(), bounds.Max.Y)
			draw.Draw(styleImage, styleRect, style.image, image.Point{}, draw.Over)
			styleImagePath := fmt.Sprintf("%s/%s-b%s%s.png", config.BannerOutputPath, ID, style.id, bgInfo.suffix)
			_ = saveImage(styleImagePath, styleImage)
		}
	}
	log.Println("Images created successfully for:", ID)
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
			stringsToCheck := []string{"egg_", "banner", "Always Together", "aco", "chill", "fastrun", "leaderboard", "winter", "spring", "summer", "fall"}
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
