package boost

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

func TestTableImageUnescapesDiscordMarkdown(t *testing.T) {
	lf := loadContractReportFont(24.0)
	fallback := loadEmojiFallbackFont(21.0)

	escapedWidth := measureRuneString(lf, fallback, `Is\_a\_plant\*`)
	unescapedWidth := measureRuneString(lf, fallback, `Is_a_plant*`)
	if escapedWidth != unescapedWidth {
		t.Fatalf("measureRuneString width mismatch: escaped=%d, unescaped=%d", escapedWidth, unescapedWidth)
	}

	cols := []TableImageColumn{
		{Label: "Player", Align: bottools.StringAlignLeft},
		{Label: "Score", Align: bottools.StringAlignRight},
	}
	rows := []TableImageRow{
		{
			Cells: []TableImageCell{
				{Text: `Is\_a\_plant\*`},
				{Text: "100"},
			},
		},
	}
	imgBytes, err := RenderTableImage(cols, rows)
	if err != nil {
		t.Fatalf("RenderTableImage failed: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(imgBytes)); err != nil {
		t.Fatalf("failed to decode rendered PNG: %v", err)
	}
}
