package boost

import (
	"bytes"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"testing"
)

func TestBuildTokenOverlayGIFPreservesAnimatedDelays(t *testing.T) {
	inputGIF := mustEncodeGIF(t, []int{5, 13})
	csvData := []byte("Frame,X,Y,Width,Visibility,Rotation,Opacity\n1,0,0,0,Hidden,0,255\n2,0,0,0,Hidden,0,255\n")

	out, err := buildTokenOverlayGIF(inputGIF, csvData)
	if err != nil {
		t.Fatalf("buildTokenOverlayGIF returned error: %v", err)
	}

	decoded, err := gif.DecodeAll(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("failed to decode output gif: %v", err)
	}
	if len(decoded.Delay) != 2 {
		t.Fatalf("expected 2 delay entries, got %d", len(decoded.Delay))
	}
	if decoded.Delay[0] != 5 || decoded.Delay[1] != 13 {
		t.Fatalf("expected delays [5 13], got [%d %d]", decoded.Delay[0], decoded.Delay[1])
	}
}

func TestBuildTokenOverlayGIFSingleFrameExpansionUsesFiveFPS(t *testing.T) {
	inputGIF := mustEncodeGIF(t, []int{9})
	csvData := []byte("Frame,X,Y,Width,Visibility,Rotation,Opacity\n1,0,0,0,Hidden,0,255\n2,0,0,0,Hidden,0,255\n3,0,0,0,Hidden,0,255\n")

	out, err := buildTokenOverlayGIF(inputGIF, csvData)
	if err != nil {
		t.Fatalf("buildTokenOverlayGIF returned error: %v", err)
	}

	decoded, err := gif.DecodeAll(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("failed to decode output gif: %v", err)
	}
	if len(decoded.Image) != 3 {
		t.Fatalf("expected 3 output frames, got %d", len(decoded.Image))
	}
	if len(decoded.Delay) != 3 {
		t.Fatalf("expected 3 delay entries, got %d", len(decoded.Delay))
	}
	for idx, d := range decoded.Delay {
		if d != 20 {
			t.Fatalf("expected delay 20 at frame %d, got %d", idx+1, d)
		}
	}
}

func mustEncodeGIF(t *testing.T, delays []int) []byte {
	t.Helper()

	if len(delays) < 1 {
		t.Fatalf("must provide at least one delay")
	}

	frames := make([]*image.Paletted, 0, len(delays))
	for idx := range delays {
		frame := image.NewPaletted(image.Rect(0, 0, 2, 2), palette.Plan9)
		frame.SetColorIndex(0, 0, uint8(idx%len(palette.Plan9)))
		frame.Palette[0] = color.RGBA{R: uint8(50 * idx), G: 100, B: 150, A: 255}
		frames = append(frames, frame)
	}

	g := &gif.GIF{
		Image: frames,
		Delay: delays,
	}

	var out bytes.Buffer
	if err := gif.EncodeAll(&out, g); err != nil {
		t.Fatalf("failed to encode input gif: %v", err)
	}
	return out.Bytes()
}
