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

	"github.com/google/go-github/v71/github"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func LoadFontFile(name string, size, dpi float64) (font.Face, error) {
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

type seasonBoundary struct {
	spring time.Time
	summer time.Time
	fall   time.Time
	winter time.Time
}

var celestialSeasonBoundaries = map[int]seasonBoundary{
	2020: {
		spring: time.Date(2020, time.March, 20, 3, 50, 0, 0, time.UTC),
		summer: time.Date(2020, time.June, 20, 21, 43, 0, 0, time.UTC),
		fall:   time.Date(2020, time.September, 22, 13, 30, 0, 0, time.UTC),
		winter: time.Date(2020, time.December, 21, 10, 2, 0, 0, time.UTC),
	},
	2021: {
		spring: time.Date(2021, time.March, 20, 9, 37, 0, 0, time.UTC),
		summer: time.Date(2021, time.June, 21, 3, 32, 0, 0, time.UTC),
		fall:   time.Date(2021, time.September, 22, 19, 20, 0, 0, time.UTC),
		winter: time.Date(2021, time.December, 21, 15, 59, 0, 0, time.UTC),
	},
	2022: {
		spring: time.Date(2022, time.March, 20, 15, 33, 0, 0, time.UTC),
		summer: time.Date(2022, time.June, 21, 9, 13, 0, 0, time.UTC),
		fall:   time.Date(2022, time.September, 23, 1, 4, 0, 0, time.UTC),
		winter: time.Date(2022, time.December, 21, 21, 48, 0, 0, time.UTC),
	},
	2023: {
		spring: time.Date(2023, time.March, 20, 21, 24, 0, 0, time.UTC),
		summer: time.Date(2023, time.June, 21, 14, 57, 0, 0, time.UTC),
		fall:   time.Date(2023, time.September, 23, 6, 50, 0, 0, time.UTC),
		winter: time.Date(2023, time.December, 22, 3, 27, 0, 0, time.UTC),
	},
	2024: {
		spring: time.Date(2024, time.March, 20, 3, 6, 0, 0, time.UTC),
		summer: time.Date(2024, time.June, 20, 20, 50, 0, 0, time.UTC),
		fall:   time.Date(2024, time.September, 22, 12, 43, 0, 0, time.UTC),
		winter: time.Date(2024, time.December, 21, 9, 20, 0, 0, time.UTC),
	},
	2025: {
		spring: time.Date(2025, time.March, 20, 9, 1, 0, 0, time.UTC),
		summer: time.Date(2025, time.June, 21, 2, 42, 0, 0, time.UTC),
		fall:   time.Date(2025, time.September, 22, 18, 19, 0, 0, time.UTC),
		winter: time.Date(2025, time.December, 21, 15, 3, 0, 0, time.UTC),
	},
	2026: {
		spring: time.Date(2026, time.March, 20, 14, 45, 0, 0, time.UTC),
		summer: time.Date(2026, time.June, 21, 8, 24, 0, 0, time.UTC),
		fall:   time.Date(2026, time.September, 23, 0, 5, 0, 0, time.UTC),
		winter: time.Date(2026, time.December, 21, 20, 50, 0, 0, time.UTC),
	},
	2027: {
		spring: time.Date(2027, time.March, 20, 20, 24, 0, 0, time.UTC),
		summer: time.Date(2027, time.June, 21, 14, 10, 0, 0, time.UTC),
		fall:   time.Date(2027, time.September, 23, 6, 1, 0, 0, time.UTC),
		winter: time.Date(2027, time.December, 22, 2, 42, 0, 0, time.UTC),
	},
	2028: {
		spring: time.Date(2028, time.March, 20, 2, 17, 0, 0, time.UTC),
		summer: time.Date(2028, time.June, 20, 20, 1, 0, 0, time.UTC),
		fall:   time.Date(2028, time.September, 22, 11, 45, 0, 0, time.UTC),
		winter: time.Date(2028, time.December, 21, 8, 20, 0, 0, time.UTC),
	},
	2029: {
		spring: time.Date(2029, time.March, 20, 8, 1, 0, 0, time.UTC),
		summer: time.Date(2029, time.June, 21, 1, 48, 0, 0, time.UTC),
		fall:   time.Date(2029, time.September, 22, 17, 37, 0, 0, time.UTC),
		winter: time.Date(2029, time.December, 21, 14, 14, 0, 0, time.UTC),
	},
	2030: {
		spring: time.Date(2030, time.March, 20, 13, 51, 0, 0, time.UTC),
		summer: time.Date(2030, time.June, 21, 7, 31, 0, 0, time.UTC),
		fall:   time.Date(2030, time.September, 22, 23, 27, 0, 0, time.UTC),
		winter: time.Date(2030, time.December, 21, 20, 9, 0, 0, time.UTC),
	},
	2031: {
		spring: time.Date(2031, time.March, 20, 19, 41, 0, 0, time.UTC),
		summer: time.Date(2031, time.June, 21, 13, 17, 0, 0, time.UTC),
		fall:   time.Date(2031, time.September, 23, 5, 15, 0, 0, time.UTC),
		winter: time.Date(2031, time.December, 22, 1, 56, 0, 0, time.UTC),
	},
	2032: {
		spring: time.Date(2032, time.March, 20, 1, 22, 0, 0, time.UTC),
		summer: time.Date(2032, time.June, 20, 19, 8, 0, 0, time.UTC),
		fall:   time.Date(2032, time.September, 22, 11, 10, 0, 0, time.UTC),
		winter: time.Date(2032, time.December, 21, 7, 56, 0, 0, time.UTC),
	},
	2033: {
		spring: time.Date(2033, time.March, 20, 7, 23, 0, 0, time.UTC),
		summer: time.Date(2033, time.June, 21, 1, 1, 0, 0, time.UTC),
		fall:   time.Date(2033, time.September, 22, 16, 51, 0, 0, time.UTC),
		winter: time.Date(2033, time.December, 21, 13, 45, 0, 0, time.UTC),
	},
	2034: {
		spring: time.Date(2034, time.March, 20, 13, 17, 0, 0, time.UTC),
		summer: time.Date(2034, time.June, 21, 6, 44, 0, 0, time.UTC),
		fall:   time.Date(2034, time.September, 22, 22, 39, 0, 0, time.UTC),
		winter: time.Date(2034, time.December, 21, 19, 34, 0, 0, time.UTC),
	},
	2035: {
		spring: time.Date(2035, time.March, 20, 19, 3, 0, 0, time.UTC),
		summer: time.Date(2035, time.June, 21, 12, 32, 0, 0, time.UTC),
		fall:   time.Date(2035, time.September, 23, 4, 38, 0, 0, time.UTC),
		winter: time.Date(2035, time.December, 22, 1, 31, 0, 0, time.UTC),
	},
	2036: {
		spring: time.Date(2036, time.March, 20, 1, 2, 0, 0, time.UTC),
		summer: time.Date(2036, time.June, 20, 18, 31, 0, 0, time.UTC),
		fall:   time.Date(2036, time.September, 22, 10, 23, 0, 0, time.UTC),
		winter: time.Date(2036, time.December, 21, 7, 12, 0, 0, time.UTC),
	},
	2037: {
		spring: time.Date(2037, time.March, 20, 6, 49, 0, 0, time.UTC),
		summer: time.Date(2037, time.June, 21, 0, 22, 0, 0, time.UTC),
		fall:   time.Date(2037, time.September, 22, 16, 12, 0, 0, time.UTC),
		winter: time.Date(2037, time.December, 21, 13, 7, 0, 0, time.UTC),
	},
	2038: {
		spring: time.Date(2038, time.March, 20, 12, 40, 0, 0, time.UTC),
		summer: time.Date(2038, time.June, 21, 6, 9, 0, 0, time.UTC),
		fall:   time.Date(2038, time.September, 22, 22, 2, 0, 0, time.UTC),
		winter: time.Date(2038, time.December, 21, 19, 2, 0, 0, time.UTC),
	},
	2039: {
		spring: time.Date(2039, time.March, 20, 18, 32, 0, 0, time.UTC),
		summer: time.Date(2039, time.June, 21, 11, 57, 0, 0, time.UTC),
		fall:   time.Date(2039, time.September, 23, 3, 49, 0, 0, time.UTC),
		winter: time.Date(2039, time.December, 22, 0, 40, 0, 0, time.UTC),
	},
	2040: {
		spring: time.Date(2040, time.March, 20, 0, 11, 0, 0, time.UTC),
		summer: time.Date(2040, time.June, 20, 17, 46, 0, 0, time.UTC),
		fall:   time.Date(2040, time.September, 22, 9, 44, 0, 0, time.UTC),
		winter: time.Date(2040, time.December, 21, 6, 33, 0, 0, time.UTC),
	},
}

// getCelestialSeason returns the current season using exact astronomical equinox and solstice times (in UTC).
func getCelestialSeason(t time.Time) string {
	utc := t.UTC()
	year := utc.Year()

	bounds, ok := celestialSeasonBoundaries[year]
	if !ok {
		// Fallback for years outside the lookup table
		bounds = seasonBoundary{
			spring: time.Date(year, time.March, 20, 0, 0, 0, 0, time.UTC),
			summer: time.Date(year, time.June, 21, 0, 0, 0, 0, time.UTC),
			fall:   time.Date(year, time.September, 22, 0, 0, 0, 0, time.UTC),
			winter: time.Date(year, time.December, 21, 0, 0, 0, 0, time.UTC),
		}
	}

	switch {
	case !utc.Before(bounds.spring) && utc.Before(bounds.summer):
		return "spring"
	case !utc.Before(bounds.summer) && utc.Before(bounds.fall):
		return "summer"
	case !utc.Before(bounds.fall) && utc.Before(bounds.winter):
		return "fall"
	default:
		return "winter"
	}
}

type styleData struct {
	name  string
	id    string
	image image.Image
}

type bgDef struct {
	path   string
	suffix string
}

// SyncCustomBannerCallback is a function hook to sync custom banners from the database to disk.
var SyncCustomBannerCallback func(userID string, guildID string, destPath string) bool

// RefreshGuildContractsForBannerCallback is a function hook to refresh/redraw contracts
// in a guild after its default banner is updated.
var RefreshGuildContractsForBannerCallback func(guildID string)

// GenerateBanner creates a banner image with a background, overlay image, and text
func GenerateBanner(ID string, eggName string, text string, creatorID string, guildID string, styleOverride string) {
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

	currentSeason := getCelestialSeason(time.Now())
	allExistAndFresh := true

	hasCustomBanner := false
	hasDefaultBanner := false
	bgCustomPath := ""
	bgDefaultPath := ""
	if creatorID != "" {
		bgCustomPath = fmt.Sprintf("%s/banner_%s_%s.png", config.BannerPath, creatorID, guildID)
		if SyncCustomBannerCallback != nil {
			hasCustomBanner = SyncCustomBannerCallback(creatorID, guildID, bgCustomPath)
		} else if _, err := os.Stat(bgCustomPath); err == nil {
			hasCustomBanner = true
		}
	}
	if guildID != "" {
		bgDefaultPath = fmt.Sprintf("%s/banner_%s.png", config.BannerPath, guildID)
		if SyncCustomBannerCallback != nil {
			hasDefaultBanner = SyncCustomBannerCallback(guildID, "DEFAULT", bgDefaultPath)
		} else if _, err := os.Stat(bgDefaultPath); err == nil {
			hasDefaultBanner = true
		}
	}

	for _, style := range styleArray {
		if hasCustomBanner {
			customImgPath := fmt.Sprintf("%s/%s-b%s-%s_%s.png", config.BannerOutputPath, ID, style, creatorID, guildID)
			info, err := os.Stat(customImgPath)
			if os.IsNotExist(err) {
				allExistAndFresh = false
				break
			}
			bgInfo, _ := os.Stat(fmt.Sprintf("%s/banner_%s_%s.png", config.BannerPath, creatorID, guildID))
			if bgInfo != nil && info.ModTime().Before(bgInfo.ModTime()) {
				allExistAndFresh = false
				break
			}
			continue
		}
		if hasDefaultBanner {
			defaultImgPath := fmt.Sprintf("%s/%s-b%s-%s.png", config.BannerOutputPath, ID, style, guildID)
			info, err := os.Stat(defaultImgPath)
			if err != nil {
				allExistAndFresh = false
				break
			}
			bgInfo, _ := os.Stat(fmt.Sprintf("%s/banner_%s.png", config.BannerPath, guildID))
			if bgInfo != nil && info.ModTime().Before(bgInfo.ModTime()) {
				allExistAndFresh = false
				break
			}
			continue
		}

		seasonImgPath := fmt.Sprintf("%s/%s-b%s.png", config.BannerOutputPath, ID, style)
		info, err := os.Stat(seasonImgPath)
		if os.IsNotExist(err) || getCelestialSeason(info.ModTime()) != currentSeason {
			allExistAndFresh = false
			break
		}
		if !allExistAndFresh {
			break
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

	if _, err := os.Stat(bgSeasonPath); os.IsNotExist(err) {
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

	face, err := LoadFontFile(fontFile, fontSize, dpi)
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

	var backgrounds []bgDef

	if hasCustomBanner {
		backgrounds = []bgDef{
			{path: bgCustomPath, suffix: fmt.Sprintf("-%s_%s", creatorID, guildID)},
		}
	} else if hasDefaultBanner {
		backgrounds = []bgDef{
			{path: bgDefaultPath, suffix: fmt.Sprintf("-%s", guildID)},
		}
	} else {
		backgrounds = []bgDef{
			{path: bgSeasonPath, suffix: ""},
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
			adjFace, err := LoadFontFile(fontFile, adjustedFontSize, dpi)
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
// Once downloaded, files are considered permanent — a sentinel file gates re-scanning for 7 days
// to avoid hitting the GitHub API rate limit on dev restarts.
func DownloadLatestEggImages(localDownloadDir string) error {
	const successScanCooldown = 7 * 24 * time.Hour
	const failureScanCooldown = 1 * time.Hour

	// Ensure the local download directory exists.
	if err := os.MkdirAll(localDownloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create local directory %s: %w", localDownloadDir, err)
	}

	// If we scanned recently, skip the API call entirely. These files rarely change.
	sentinel := filepath.Join(localDownloadDir, ".last_scan")
	if info, err := os.Stat(sentinel); err == nil && time.Since(info.ModTime()) < successScanCooldown {
		return nil
	}

	failureSentinel := filepath.Join(localDownloadDir, ".last_scan_failed")
	if info, err := os.Stat(failureSentinel); err == nil && time.Since(info.ModTime()) < failureScanCooldown {
		return nil
	}

	owner := "mkmccarty"
	repo := "TokenTimeBoostBot"
	repoPath := "emoji"
	client := github.NewClient(nil)
	ctx := context.Background()

	// Get the contents of the specified repository directory.
	_, directoryContents, _, err := client.Repositories.GetContents(ctx, owner, repo, repoPath, &github.RepositoryContentGetOptions{
		Ref: "main",
	})
	if err != nil {
		return fmt.Errorf("error getting repository contents: %w", err)
	}

	failedDownloads := 0

	for _, content := range directoryContents {
		// Only process files.
		if content.GetType() == "file" {
			err := func() error {
				downloadURL := content.GetDownloadURL()
				if downloadURL == "" {
					return nil // Skip if there's no download URL.
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
					return nil
				}

				// If the file already exists, don't need to download it
				localFilePath := filepath.Join(localDownloadDir, content.GetName())
				if _, err := os.Stat(localFilePath); err == nil {
					//log.Printf("File %s already exists, skipping download.\n", localFilePath)
					return nil
				}

				log.Printf("Downloading %s...\n", content.GetName())

				// Make an HTTP request to download the file.
				resp, err := http.Get(downloadURL)
				if err != nil {
					return fmt.Errorf("error downloading file %s: %w", content.GetName(), err)
				}
				defer func() {
					if err := resp.Body.Close(); err != nil {
						log.Printf("Failed to close: %v", err)
					}
				}()

				if resp.StatusCode < 200 || resp.StatusCode > 299 {
					return fmt.Errorf("error downloading file %s: unexpected status %s", content.GetName(), resp.Status)
				}

				outFile, err := os.Create(localFilePath)
				if err != nil {
					return fmt.Errorf("error creating local file %s: %w", localFilePath, err)
				}
				defer func() {
					if err := outFile.Close(); err != nil {
						log.Printf("Failed to close: %v", err)
					}
				}()

				// Copy the downloaded content to the local file.
				if _, err := io.Copy(outFile, resp.Body); err != nil {
					return fmt.Errorf("error writing to file %s: %w", localFilePath, err)
				}
				log.Printf("Successfully downloaded %s.\n", content.GetName())
				return nil
			}()
			if err != nil {
				log.Print(err)
				failedDownloads++
			}
		}
	}

	if failedDownloads > 0 {
		_ = os.WriteFile(failureSentinel, []byte(time.Now().Format(time.RFC3339)), 0644)
		return fmt.Errorf("download scan completed with %d failed file(s)", failedDownloads)
	}
	_ = os.Remove(failureSentinel)

	// Update the sentinel so we don't re-scan until next week.
	_ = os.WriteFile(sentinel, []byte(time.Now().Format(time.RFC3339)), 0644)
	return nil
}
