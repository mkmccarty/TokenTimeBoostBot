package mint

import (
	"bytes"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"os"
	"path/filepath"
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

func TestEnsureTokenImageFileResolvesRepoAssetFromParentDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	assetDir := filepath.Join(repoRoot, "emoji")
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		t.Fatalf("failed to create fixture asset dir: %v", err)
	}
	assetPath := filepath.Join(assetDir, "token_overlay.png")
	if err := os.WriteFile(assetPath, []byte("fixture"), 0600); err != nil {
		t.Fatalf("failed to write fixture asset: %v", err)
	}

	packageDir := filepath.Join(repoRoot, "src", "mint")
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		t.Fatalf("failed to create fixture package dir: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()
	if err := os.Chdir(packageDir); err != nil {
		t.Fatalf("failed to enter fixture package dir: %v", err)
	}

	resolvedPath, err := ensureTokenImageFile()
	if err != nil {
		t.Fatalf("ensureTokenImageFile returned error: %v", err)
	}

	resolvedInfo, err := os.Stat(resolvedPath)
	if err != nil {
		t.Fatalf("failed to stat resolved asset: %v", err)
	}
	assetInfo, err := os.Stat(assetPath)
	if err != nil {
		t.Fatalf("failed to stat fixture asset: %v", err)
	}
	if !os.SameFile(resolvedInfo, assetInfo) {
		t.Fatalf("expected resolved asset to match fixture asset, got %q and %q", resolvedPath, assetPath)
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
