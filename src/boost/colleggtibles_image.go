package boost

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
)

var (
	cachedEggIcons     = make(map[string]image.Image)
	cachedEggIconsLock sync.RWMutex

	colImgBg           = color.RGBA{R: 0x1E, G: 0x1F, B: 0x22, A: 255} // Discord dark background
	colImgCardBg       = color.RGBA{R: 0x2B, G: 0x2D, B: 0x31, A: 255} // Column card background
	colImgCardBorder   = color.RGBA{R: 0x38, G: 0x3A, B: 0x40, A: 255} // Card border
	colImgHeaderBg     = color.RGBA{R: 0x23, G: 0x24, B: 0x28, A: 255} // Header card background
	colImgHeaderBorder = color.RGBA{R: 0x35, G: 0x37, B: 0x3C, A: 255} // Header card border
	colImgDivider      = color.RGBA{R: 0x38, G: 0x3A, B: 0x40, A: 255} // Hairline divider

	colImgWhite        = color.RGBA{R: 0xFD, G: 0xFD, B: 0xFE, A: 255} // Crisp bright white
	colImgHeaderSilver = color.RGBA{R: 0xEB, G: 0xED, B: 0xF0, A: 255} // Silver for headers
	colImgMuted        = color.RGBA{R: 0x94, G: 0x9B, B: 0xA4, A: 255} // Muted gray text
	colImgGold         = color.RGBA{R: 0xF1, G: 0xC4, B: 0x0F, A: 255} // Title gold / Egg Inc yellow
	colImgGreen        = color.RGBA{R: 0x57, G: 0xF2, B: 0x87, A: 255} // Maxed bright green
	colImgGreenBg      = color.RGBA{R: 0x1E, G: 0x3A, B: 0x2B, A: 255} // Badge dark green background
	colImgGreenBorder  = color.RGBA{R: 0x2D, G: 0x7D, B: 0x46, A: 255} // Badge green border
	colImgYellow       = color.RGBA{R: 0xFE, G: 0xE7, B: 0x5C, A: 255} // Tier yellow
	colImgYellowBg     = color.RGBA{R: 0x3E, G: 0x38, B: 0x1E, A: 255} // Badge dark yellow background
	colImgYellowBorder = color.RGBA{R: 0x7E, G: 0x72, B: 0x28, A: 255} // Badge yellow border
	colImgRed          = color.RGBA{R: 0xF8, G: 0x71, B: 0x71, A: 255} // Bright coral red for missing
	colImgLavender     = color.RGBA{R: 0xC4, G: 0xB5, B: 0xFD, A: 255} // Bright prediction lavender
	colImgDeliveryBlue = color.RGBA{R: 0x58, G: 0x65, B: 0xF2, A: 255} // Accent blue for Coop & Delivery
	colImgEconomyGold  = color.RGBA{R: 0xEB, G: 0xB3, B: 0x46, A: 255} // Accent gold for Farm & Economy
)

type colleggtibleFontSet struct {
	lf           loadedFont
	fallbackFace font.Face
	ascent       int
	descent      int
}

func newColleggtibleFontSet(size float64, fallbackSize float64) colleggtibleFontSet {
	lf := loadContractReportFont(size)
	fb := loadEmojiFallbackFont(fallbackSize)
	m := lf.face.Metrics()
	return colleggtibleFontSet{
		lf:           lf,
		fallbackFace: fb,
		ascent:       m.Ascent.Ceil(),
		descent:      m.Descent.Ceil(),
	}
}

// findEggIconFile locates the local egg icon PNG file for an egg ID.
func findEggIconFile(eggID string) string {
	cleanID := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(eggID), " ", ""), "-", "")
	names := []string{
		fmt.Sprintf("egg_%s.png", cleanID),
		fmt.Sprintf("egg_%s.png", strings.ToLower(eggID)),
		fmt.Sprintf("%s.png", cleanID),
	}

	dirCandidates := []string{}
	if config.BannerPath != "" {
		dirCandidates = append(dirCandidates, config.BannerPath)
	}
	dirCandidates = append(dirCandidates,
		"banners",
		"./banners",
		"../banners",
		"../../banners",
		"emoji",
		"./emoji",
		"../emoji",
		"../../emoji",
	)

	for _, dir := range dirCandidates {
		for _, n := range names {
			p := filepath.Join(dir, n)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	// If not found in any candidate directory, try downloading to config.BannerPath
	targetDir := config.BannerPath
	if targetDir == "" {
		targetDir = "banners"
	}
	_ = bottools.DownloadLatestEggImages(targetDir)

	for _, n := range names {
		p := filepath.Join(targetDir, n)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	for _, dir := range dirCandidates {
		p := filepath.Join(dir, "egg_unknown.png")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// loadAndScaleEggIcon returns a scaled egg icon image.
func loadAndScaleEggIcon(eggID string, size int) image.Image {
	key := fmt.Sprintf("%s_%d", strings.ToLower(eggID), size)
	cachedEggIconsLock.RLock()
	if img, ok := cachedEggIcons[key]; ok {
		cachedEggIconsLock.RUnlock()
		return img
	}
	cachedEggIconsLock.RUnlock()

	filePath := findEggIconFile(eggID)
	if filePath == "" {
		return nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	src, err := png.Decode(f)
	if err != nil {
		return nil
	}

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	cachedEggIconsLock.Lock()
	cachedEggIcons[key] = dst
	cachedEggIconsLock.Unlock()

	return dst
}

// drawRoundedBox draws a rounded rectangle with optional fill and border.
func drawRoundedBox(img *image.RGBA, r image.Rectangle, radius int, fill, border color.Color) {
	if radius <= 0 {
		if fill != nil {
			draw.Draw(img, r, &image.Uniform{fill}, image.Point{}, draw.Over)
		}
		if border != nil {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.Set(x, r.Min.Y, border)
				img.Set(x, r.Max.Y-1, border)
			}
			for y := r.Min.Y; y < r.Max.Y; y++ {
				img.Set(r.Min.X, y, border)
				img.Set(r.Max.X-1, y, border)
			}
		}
		return
	}

	rSq := radius * radius
	innerRSq := (radius - 1) * (radius - 1)

	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dx := 0
			if x < r.Min.X+radius {
				dx = (r.Min.X + radius) - x
			} else if x >= r.Max.X-radius {
				dx = x - (r.Max.X - radius - 1)
			}

			dy := 0
			if y < r.Min.Y+radius {
				dy = (r.Min.Y + radius) - y
			} else if y >= r.Max.Y-radius {
				dy = y - (r.Max.Y - radius - 1)
			}

			if dx > 0 && dy > 0 {
				distSq := dx*dx + dy*dy
				if distSq > rSq {
					continue // outside rounded corner
				}
				if border != nil && distSq >= innerRSq {
					img.Set(x, y, border)
					continue
				}
			} else if border != nil {
				if x == r.Min.X || x == r.Max.X-1 || y == r.Min.Y || y == r.Max.Y-1 {
					img.Set(x, y, border)
					continue
				}
			}

			if fill != nil {
				img.Set(x, y, fill)
			}
		}
	}
}

// drawHLine draws a horizontal line with the specified thickness.
func drawHLine(img *image.RGBA, x1, x2, y, thickness int, c color.Color) {
	for t := 0; t < thickness; t++ {
		lineY := y + t
		if lineY < 0 || lineY >= img.Bounds().Dy() {
			continue
		}
		startX := min(x1, x2)
		endX := max(x1, x2)
		for x := max(0, startX); x <= min(img.Bounds().Dx()-1, endX); x++ {
			img.Set(x, lineY, c)
		}
	}
}

// findEggPrediction finds a predicted contract drop date for a colleggtible egg.
func findEggPrediction(c ei.PlayerColleggtibleStatus, preds map[string]time.Time) time.Time {
	if preds == nil {
		return time.Time{}
	}
	cleanID := strings.ReplaceAll(strings.ToLower(c.EggID), "-", "")
	if t, ok := preds[cleanID]; ok {
		return t
	}
	if t, ok := preds[strings.ToLower(c.EggID)]; ok {
		return t
	}
	cleanName := strings.ReplaceAll(strings.ToLower(c.Name), " ", "")
	if t, ok := preds[cleanName]; ok {
		return t
	}
	if t, ok := preds[strings.ToLower(c.Name)]; ok {
		return t
	}
	return time.Time{}
}

// isCoopDeliveryDimension returns true if the dimension belongs to Coop & Delivery.
func isCoopDeliveryDimension(d ei.GameModifier_GameDimension) bool {
	return d == ei.GameModifier_EGG_LAYING_RATE ||
		d == ei.GameModifier_SHIPPING_CAPACITY ||
		d == ei.GameModifier_HAB_CAPACITY ||
		d == ei.GameModifier_INTERNAL_HATCHERY_RATE
}

// RenderColleggtiblesImage creates a polished, high-resolution 2-column image representation of colleggtibles.
func RenderColleggtiblesImage(overview ei.ColleggtiblesOverview, userName string, preds map[string]time.Time, showAllPredictions ...bool) ([]byte, error) {
	// 2x Retina resolution fonts for razor-sharp rendering
	titleFonts := newColleggtibleFontSet(40.0, 36.0)
	subtitleFonts := newColleggtibleFontSet(26.0, 23.0)
	badgeFonts := newColleggtibleFontSet(23.0, 20.0)
	colTitleFonts := newColleggtibleFontSet(29.0, 26.0)
	dimHeaderFonts := newColleggtibleFontSet(26.0, 23.0)
	eggNameFonts := newColleggtibleFontSet(25.0, 22.0)
	eggSubFonts := newColleggtibleFontSet(22.0, 19.0)
	footerFonts := newColleggtibleFontSet(20.0, 18.0)

	forcePredictions := len(showAllPredictions) > 0 && showAllPredictions[0]

	var deliveryDims []ei.DimensionBuffSummary
	var economyDims []ei.DimensionBuffSummary
	for _, dim := range overview.Dimensions {
		if isCoopDeliveryDimension(dim.Dimension) {
			deliveryDims = append(deliveryDims, dim)
		} else {
			economyDims = append(economyDims, dim)
		}
	}

	const (
		canvasW      = 860
		padX         = 20
		padY         = 20
		colW         = 400
		colGap       = 20
		col1X        = padX
		col2X        = padX + colW + colGap
		headerH      = 92
		eggIconSize  = 48
		dimHeaderH   = 34
		baseSpacing  = 16
		cardInternal = 16
	)

	// Clean up user display name if it's empty or only unprintable characters
	cleanUserName := strings.TrimSpace(userName)
	hasPrintable := false
	for _, r := range cleanUserName {
		if r >= 32 && r <= 126 {
			hasPrintable = true
			break
		}
	}
	if !hasPrintable {
		cleanUserName = "Farmer"
	}

	// Calculate column heights dynamically
	calcColHeight := func(dims []ei.DimensionBuffSummary, spacing int) int {
		h := cardInternal + 40 + 8 // header + spacing
		for i, dim := range dims {
			h += dimHeaderH + 8 // dim header + hairline
			for _, egg := range dim.Colleggtibles {
				predTime := findEggPrediction(egg, preds)
				hasPred := !predTime.IsZero() && (!egg.IsMaxed || forcePredictions)
				hasSecondLine := (egg.Tier >= 0 && !egg.IsMaxed) || hasPred
				if hasSecondLine {
					h += 72
				} else {
					h += 52
				}
			}
			if i < len(dims)-1 {
				h += spacing
			}
		}
		h += cardInternal
		return h
	}

	col1BaseH := calcColHeight(deliveryDims, baseSpacing)
	col2BaseH := calcColHeight(economyDims, baseSpacing)
	cardH := max(col1BaseH, col2BaseH, 640)

	hasAnyPredictions := false
	for _, dim := range append(deliveryDims, economyDims...) {
		for _, egg := range dim.Colleggtibles {
			predTime := findEggPrediction(egg, preds)
			if !predTime.IsZero() && (!egg.IsMaxed || forcePredictions) {
				hasAnyPredictions = true
				break
			}
		}
		if hasAnyPredictions {
			break
		}
	}

	footerH := 0
	if hasAnyPredictions {
		footerH = 36
	}
	canvasH := padY + headerH + 16 + cardH + footerH + padY

	img := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	draw.Draw(img, img.Bounds(), &image.Uniform{colImgBg}, image.Point{}, draw.Src)

	// ==================== Header Card ====================
	headerRect := image.Rect(padX, padY, canvasW-padX, padY+headerH)
	drawRoundedBox(img, headerRect, 12, colImgHeaderBg, colImgHeaderBorder)

	// Title
	drawRuneStringAt(img, titleFonts.lf, titleFonts.fallbackFace, "COLLEGGTIBLES", padX+24, padY+40, colImgGold)

	// Subtitle
	subText := fmt.Sprintf("%s's Buffs & Collection", cleanUserName)
	drawRuneStringAt(img, subtitleFonts.lf, subtitleFonts.fallbackFace, subText, padX+24, padY+76, colImgWhite)

	// Badge on right of header
	var (
		badgeText   string
		badgeColor  color.Color
		badgeBg     color.Color
		badgeBorder color.Color
	)

	if overview.MaxedCount == overview.TotalCount && overview.TotalCount > 0 {
		badgeText = fmt.Sprintf("%d/%d MAXED", overview.MaxedCount, overview.TotalCount)
		badgeColor = colImgGreen
		badgeBg = colImgGreenBg
		badgeBorder = colImgGreenBorder
	} else if overview.CollectedCount == overview.TotalCount {
		badgeText = fmt.Sprintf("%d/%d MAXED", overview.MaxedCount, overview.TotalCount)
		badgeColor = colImgYellow
		badgeBg = colImgYellowBg
		badgeBorder = colImgYellowBorder
	} else {
		missing := overview.TotalCount - overview.CollectedCount
		badgeText = fmt.Sprintf("%d/%d MAXED - %d MISSING", overview.MaxedCount, overview.TotalCount, missing)
		badgeColor = colImgYellow
		badgeBg = colImgYellowBg
		badgeBorder = colImgYellowBorder
	}

	badgeTextW := measureRuneString(badgeFonts.lf, badgeFonts.fallbackFace, badgeText)
	badgePadX := 16
	badgeW := badgeTextW + badgePadX*2
	badgeH := 48
	badgeX := headerRect.Max.X - 20 - badgeW
	badgeY := padY + (headerH-badgeH)/2
	drawRoundedBox(img, image.Rect(badgeX, badgeY, badgeX+badgeW, badgeY+badgeH), 10, badgeBg, badgeBorder)

	badgeBaseline := badgeY + (badgeH+badgeFonts.ascent-badgeFonts.descent)/2
	drawRuneStringAt(img, badgeFonts.lf, badgeFonts.fallbackFace, badgeText, badgeX+badgePadX, badgeBaseline, badgeColor)

	// ==================== Columns ====================
	colTopY := padY + headerH + 16
	anyPredicted := false

	renderColumn := func(startX int, colTitle string, accentColor color.Color, dims []ei.DimensionBuffSummary, baseH int) {
		cardRect := image.Rect(startX, colTopY, startX+colW, colTopY+cardH)
		drawRoundedBox(img, cardRect, 12, colImgCardBg, colImgCardBorder)

		// Column Header
		accentBarW := 6
		accentBarH := 26
		accentBarY := colTopY + cardInternal + 2
		drawRoundedBox(img, image.Rect(startX+cardInternal, accentBarY, startX+cardInternal+accentBarW, accentBarY+accentBarH), 2, accentColor, nil)

		titleX := startX + cardInternal + accentBarW + 10
		titleBaseline := colTopY + cardInternal + 24
		drawRuneStringBoldAt(img, colTitleFonts.lf, colTitleFonts.fallbackFace, colTitle, titleX, titleBaseline, colImgWhite)

		// Hairline below column header
		divY := colTopY + cardInternal + 38
		drawHLine(img, startX+cardInternal, startX+colW-cardInternal, divY, 2, colImgDivider)

		// Distribute spacing between dimensions if there's headroom (capped at 24px extra)
		dimSpacing := baseSpacing
		if len(dims) > 1 && cardH > baseH {
			extra := (cardH - baseH) / (len(dims) - 1)
			if extra > 24 {
				extra = 24
			}
			dimSpacing = baseSpacing + extra
		}

		// Content dimensions
		currentY := divY + 12
		usableStart := startX + cardInternal
		usableEnd := startX + colW - cardInternal

		for i, dim := range dims {
			// Dimension Title (left aligned)
			dimTitle := strings.ToUpper(dim.DimensionName)
			drawRuneStringBoldAt(img, dimHeaderFonts.lf, dimHeaderFonts.fallbackFace, dimTitle, usableStart, currentY+24, colImgHeaderSilver)

			// Total Buff Value (right aligned)
			var (
				buffStr   string
				buffColor color.Color
			)
			if dim.IsMaxed {
				buffStr = fmt.Sprintf("%s (Maxed)", dim.CurrentString)
				buffColor = colImgGreen
			} else {
				buffStr = fmt.Sprintf("%s / %s", dim.CurrentString, dim.MaxString)
				buffColor = colImgYellow
			}
			buffW := measureRuneString(dimHeaderFonts.lf, dimHeaderFonts.fallbackFace, buffStr)
			drawRuneStringAt(img, dimHeaderFonts.lf, dimHeaderFonts.fallbackFace, buffStr, usableEnd-buffW, currentY+24, buffColor)

			// Hairline under dimension header
			dimDivY := currentY + dimHeaderH
			drawHLine(img, usableStart, usableEnd, dimDivY, 2, colImgDivider)

			// Eggs
			eggCurrentY := dimDivY + 8
			for _, egg := range dim.Colleggtibles {
				predTime := findEggPrediction(egg, preds)
				hasPred := !predTime.IsZero() && (!egg.IsMaxed || forcePredictions)
				hasSecondLine := (egg.Tier >= 0 && !egg.IsMaxed) || hasPred

				eggRowH := 52
				if hasSecondLine {
					eggRowH = 72
				}

				// Egg icon
				iconY := eggCurrentY + (eggRowH-eggIconSize)/2
				if hasSecondLine {
					iconY = eggCurrentY + 2
				}
				icon := loadAndScaleEggIcon(egg.EggID, eggIconSize)
				if icon != nil {
					iconRect := image.Rect(usableStart+2, iconY, usableStart+2+eggIconSize, iconY+eggIconSize)
					draw.Draw(img, iconRect, icon, image.Point{}, draw.Over)
				}

				textX := usableStart + 2 + eggIconSize + 12

				if !hasSecondLine {
					// Single-line layout for:
					// 1. Maxed eggs without predictions
					// 2. Missing eggs without predictions (e.g. brand new egg before first prediction cycle)
					drawRuneStringAt(img, eggNameFonts.lf, eggNameFonts.fallbackFace, egg.Name, textX, eggCurrentY+34, colImgWhite)

					var (
						valText  string
						valColor color.Color
					)
					if egg.IsMaxed {
						valText = fmt.Sprintf("Maxed: %s", egg.CurrentValueString)
						valColor = colImgGreen
					} else {
						valText = fmt.Sprintf("Missing (%s)", egg.MaxValueString)
						valColor = colImgRed
					}
					valW := measureRuneString(eggSubFonts.lf, eggSubFonts.fallbackFace, valText)
					drawRuneStringAt(img, eggSubFonts.lf, eggSubFonts.fallbackFace, valText, usableEnd-valW, eggCurrentY+34, valColor)
				} else {
					// Two-line layout for:
					// 1. In-progress eggs (with or without prediction)
					// 2. Missing eggs with prediction
					// 3. Maxed eggs with prediction (dev force flag)
					drawRuneStringAt(img, eggNameFonts.lf, eggNameFonts.fallbackFace, egg.Name, textX, eggCurrentY+28, colImgWhite)

					var (
						statusText  string
						statusColor color.Color
					)
					if egg.IsMaxed {
						statusText = fmt.Sprintf("Maxed: %s", egg.CurrentValueString)
						statusColor = colImgGreen
					} else if egg.Tier >= 0 {
						statusText = fmt.Sprintf("Tier %d: %s", egg.Tier+1, egg.CurrentValueString)
						statusColor = colImgYellow
					} else {
						statusText = fmt.Sprintf("Missing (%s)", egg.MaxValueString)
						statusColor = colImgRed
					}
					statusW := measureRuneString(eggSubFonts.lf, eggSubFonts.fallbackFace, statusText)
					drawRuneStringAt(img, eggSubFonts.lf, eggSubFonts.fallbackFace, statusText, usableEnd-statusW, eggCurrentY+28, statusColor)

					// Line 2: Progress (left) + Predicted Date (right)
					if egg.Tier >= 0 && !egg.IsMaxed {
						sizeStr := ei.FormatEIValue(egg.FarmSize, map[string]any{"decimals": 1, "trim": true})
						progStr := fmt.Sprintf("(%s/10B)", sizeStr)
						drawRuneStringAt(img, eggSubFonts.lf, eggSubFonts.fallbackFace, progStr, textX, eggCurrentY+56, colImgMuted)
					}

					if !predTime.IsZero() {
						anyPredicted = true
						dateStr := predTime.Format("Jan 2")
						if predTime.Year() != time.Now().Year() {
							dateStr = predTime.Format("Jan 2 '06")
						}
						predStr := fmt.Sprintf("Est: %s", dateStr)
						predW := measureRuneString(eggSubFonts.lf, eggSubFonts.fallbackFace, predStr)
						drawRuneStringAt(img, eggSubFonts.lf, eggSubFonts.fallbackFace, predStr, usableEnd-predW, eggCurrentY+56, colImgLavender)
					}
				}

				eggCurrentY += eggRowH
			}

			if i < len(dims)-1 {
				currentY = eggCurrentY + dimSpacing
			} else {
				currentY = eggCurrentY
			}
		}
	}

	renderColumn(col1X, "COOP & DELIVERY", colImgDeliveryBlue, deliveryDims, col1BaseH)
	renderColumn(col2X, "FARM & ECONOMY", colImgEconomyGold, economyDims, col2BaseH)

	// ==================== Footer ====================
	if anyPredicted {
		footerY := colTopY + cardH + 4
		drawRuneStringAt(img, footerFonts.lf, footerFonts.fallbackFace, "* Est: Next predicted contract drop date", padX+4, footerY+22, colImgMuted)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
