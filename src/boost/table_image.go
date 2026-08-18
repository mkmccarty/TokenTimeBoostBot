package boost

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type TableImageColumn struct {
	Label string
	Align bottools.StringAlign
}

type TableImageCell struct {
	Text  string
	Color string // "green", "red", "blue", or "" (default)
}

type TableImageRow struct {
	Cells []TableImageCell
}

type loadedFont struct {
	face     font.Face
	sfntFont *sfnt.Font
}

func loadContractReportFont(fontSize float64) loadedFont {
	candidates := []string{
		filepath.Join(config.BannerPath, "Always Together.otf"),
		"./banners/Always Together.otf",
		"./emoji/Always Together.otf",
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		sfFont, err := sfnt.Parse(data)
		if err != nil {
			continue
		}
		face, err := opentype.NewFace(sfFont, &opentype.FaceOptions{
			Size:    fontSize,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err == nil && face != nil {
			return loadedFont{face: face, sfntFont: sfFont}
		}
	}
	return loadedFont{face: basicfont.Face7x13, sfntFont: nil}
}

func loadEmojiFallbackFont(fontSize float64) font.Face {
	sfFont, err := opentype.Parse(goregular.TTF)
	if err == nil {
		face, err := opentype.NewFace(sfFont, &opentype.FaceOptions{
			Size:    fontSize,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err == nil && face != nil {
			return face
		}
	}
	return basicfont.Face7x13
}

func isGameFontRune(lf loadedFont, sfBuf *sfnt.Buffer, r rune) bool {
	if lf.sfntFont == nil {
		return false
	}
	if r < 32 || r > 126 {
		return false
	}
	idx, err := lf.sfntFont.GlyphIndex(sfBuf, r)
	return err == nil && idx != 0
}

func measureRuneString(lf loadedFont, fallbackFace font.Face, str string) int {
	str = ei.NormalizePlayerNameForDisplay(str)
	if lf.sfntFont == nil {
		return font.MeasureString(lf.face, str).Ceil()
	}
	var sfBuf sfnt.Buffer
	totalW := 0
	for _, r := range str {
		if isGameFontRune(lf, &sfBuf, r) {
			totalW += font.MeasureString(lf.face, string(r)).Ceil()
		} else {
			totalW += font.MeasureString(fallbackFace, string(r)).Ceil()
		}
	}
	return totalW
}

func drawRuneStringAt(img *image.RGBA, lf loadedFont, fallbackFace font.Face, str string, startX, drawY int, c color.Color) {
	str = ei.NormalizePlayerNameForDisplay(str)
	if lf.sfntFont == nil {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: lf.face,
			Dot:  fixed.Point26_6{X: fixed.I(startX), Y: fixed.I(drawY)},
		}
		d.DrawString(str)
		return
	}
	var sfBuf sfnt.Buffer
	currentX := startX
	for _, r := range str {
		rStr := string(r)
		var f font.Face
		if isGameFontRune(lf, &sfBuf, r) {
			f = lf.face
		} else {
			f = fallbackFace
		}
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: f,
			Dot:  fixed.Point26_6{X: fixed.I(currentX), Y: fixed.I(drawY)},
		}
		d.DrawString(rStr)
		currentX += font.MeasureString(f, rStr).Ceil()
	}
}

// RenderTableImage renders a generic table of columns and rows as a PNG image.
func RenderTableImage(cols []TableImageColumn, rows []TableImageRow) ([]byte, error) {
	lf := loadContractReportFont(16.0)
	fallbackFace := loadEmojiFallbackFont(14.0)

	metrics := lf.face.Metrics()
	ascent := metrics.Ascent.Ceil()
	descent := metrics.Descent.Ceil()
	cellPadX := 6
	padX := 10
	padY := 10

	// Compute max pixel width per column
	colWidths := make([]int, len(cols))
	for i, col := range cols {
		maxW := measureRuneString(lf, fallbackFace, col.Label)
		for _, row := range rows {
			if i < len(row.Cells) {
				w := measureRuneString(lf, fallbackFace, row.Cells[i].Text)
				if w > maxW {
					maxW = w
				}
			}
		}
		colWidths[i] = maxW + cellPadX*2
	}

	// Calculate column start X positions
	colX := make([]int, len(cols)+1)
	colX[0] = padX
	for i := 0; i < len(cols); i++ {
		colX[i+1] = colX[i] + colWidths[i]
	}

	bgColor := color.RGBA{R: 0x1e, G: 0x1f, B: 0x22, A: 255}          // Discord dark theme
	headerColor := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 255}      // Pure White
	dividerColor := color.RGBA{R: 0x4e, G: 0x50, B: 0x58, A: 255}     // Dark gray
	defaultTextColor := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 255} // Pure Bright White

	getColor := func(colorName string) color.RGBA {
		switch colorName {
		case "green":
			return color.RGBA{R: 0x57, G: 0xf2, B: 0x87, A: 255}
		case "red":
			return color.RGBA{R: 0xed, G: 0x42, B: 0x45, A: 255}
		case "blue":
			return color.RGBA{R: 0x58, G: 0x65, B: 0xf2, A: 255}
		default:
			return defaultTextColor
		}
	}

	rowH := max(16, ascent+descent+2)
	headerGap := 4
	dividerToRowGap := 4
	headerH := ascent + descent + headerGap

	imgW := colX[len(cols)] + padX
	imgH := padY*2 + headerH + dividerToRowGap + len(rows)*rowH

	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	drawCellTextAt := func(str string, colIdx, drawY int, c color.Color) {
		textW := measureRuneString(lf, fallbackFace, str)
		xStart := colX[colIdx]
		xEnd := colX[colIdx+1]
		w := colWidths[colIdx]

		var drawX int
		switch cols[colIdx].Align {
		case bottools.StringAlignLeft:
			drawX = xStart + cellPadX
		case bottools.StringAlignRight, bottools.StringAlignCenterRight:
			drawX = xEnd - cellPadX - textW
		case bottools.StringAlignCenter:
			drawX = xStart + (w-textW)/2
		}

		drawRuneStringAt(img, lf, fallbackFace, str, drawX, drawY, c)
	}

	// Draw Header
	headerY := padY + ascent
	for i, col := range cols {
		drawCellTextAt(col.Label, i, headerY, headerColor)
	}

	// Draw Horizontal Divider Line
	divY := padY + headerH
	for x := padX; x < imgW-padX; x++ {
		img.Set(x, divY, dividerColor)
	}

	// Draw Data Rows
	firstRowY := divY + dividerToRowGap + ascent
	for rIdx, row := range rows {
		rowY := firstRowY + rIdx*rowH
		for cIdx, cell := range row.Cells {
			if cIdx < len(cols) {
				drawCellTextAt(cell.Text, cIdx, rowY, getColor(cell.Color))
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
