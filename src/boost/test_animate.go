package boost

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"image"
	"image/color/palette"
	stdDraw "image/draw"
	"image/gif"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/go-github/v33/github"
	xdraw "golang.org/x/image/draw"
)

const (
	testAnimateGIFOption = "gif"
	testAnimateCSVOption = "csv"
	testAnimateCreateSub = "create"
	testAnimateHelpSub   = "help"
	testAnimateTokenPath = "emoji/token_overlay.png"
	maxAnimateFileBytes  = 10 * 1024 * 1024
)

type animationTrackingRow struct {
	Frame      int
	X          int
	Y          int
	Width      int
	Visibility string
	Rotation   float64
	Opacity    float64
}

// GetSlashTestAnimateCommand creates the /test-animate command.
func GetSlashTestAnimateCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Overlay the token image on an animated GIF or MP4/M4P using tracked CSV coordinates.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        testAnimateCreateSub,
				Description: "Render animation using uploaded media and CSV tracking data",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionAttachment,
						Name:        testAnimateGIFOption,
						Description: "Animated GIF or MP4/M4P to process",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionAttachment,
						Name:        testAnimateCSVOption,
						Description: "Tracking CSV with Frame,X,Y,Width,Visibility,Rotation,Opacity columns",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        testAnimateHelpSub,
				Description: "Show usage help for /test-animate",
			},
		},
	}
}

// HandleTestAnimateCommand validates user-uploaded GIF/CSV and returns an overlaid animation.
func HandleTestAnimateCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if len(data.Options) == 0 {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Please choose a subcommand: /test-animate create or /test-animate help.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	sub := data.Options[0]
	if sub.Type != discordgo.ApplicationCommandOptionSubCommand {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Invalid usage. Use /test-animate create or /test-animate help.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if sub.Name == testAnimateHelpSub {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: buildTestAnimateUsageText(),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if sub.Name != testAnimateCreateSub {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown subcommand. Use /test-animate create or /test-animate help.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(sub.Options))
	for _, opt := range sub.Options {
		optionMap[opt.Name] = opt
	}
	gifOpt, ok := optionMap[testAnimateGIFOption]
	if !ok {
		sendTestAnimateError(s, i, "Missing required GIF attachment.")
		return
	}
	csvOpt, ok := optionMap[testAnimateCSVOption]
	if !ok {
		sendTestAnimateError(s, i, "Missing required CSV attachment.")
		return
	}

	gifAttachment := getCommandAttachment(i, gifOpt)
	if gifAttachment == nil {
		sendTestAnimateError(s, i, "Unable to read the GIF attachment.")
		return
	}
	csvAttachment := getCommandAttachment(i, csvOpt)
	if csvAttachment == nil {
		sendTestAnimateError(s, i, "Unable to read the CSV attachment.")
		return
	}

	gifBytes, err := downloadAttachmentBytes(gifAttachment)
	if err != nil {
		sendTestAnimateError(s, i, fmt.Sprintf("Failed downloading GIF: %v", err))
		return
	}
	csvBytes, err := downloadAttachmentBytes(csvAttachment)
	if err != nil {
		sendTestAnimateError(s, i, fmt.Sprintf("Failed downloading CSV: %v", err))
		return
	}

	inputFormat, outExt, outContentType, err := detectAnimationFormat(gifAttachment, gifBytes)
	if err != nil {
		sendTestAnimateError(s, i, err.Error())
		return
	}

	frameDetails := getFrameDetailsText(inputFormat, gifBytes, csvBytes, outExt)
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "Downloaded input files. " + frameDetails + " Rendering has started and may take a little bit of time.",
		Flags:   discordgo.MessageFlagsEphemeral,
	})

	var outputData []byte
	if inputFormat == "gif" {
		outputData, err = buildTokenOverlayGIF(gifBytes, csvBytes)
	} else {
		outputData, err = buildTokenOverlayVideo(gifBytes, csvBytes, outExt)
	}
	if err != nil {
		sendTestAnimateError(s, i, err.Error())
		return
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "Rendering complete. Generated file:",
		Files: []*discordgo.File{
			{
				Name:        "test-animate-output" + outExt,
				ContentType: outContentType,
				Reader:      bytes.NewReader(outputData),
			},
		},
		Flags: discordgo.MessageFlagsEphemeral,
	})
}

func buildTestAnimateUsageText() string {
	limitMiB := maxAnimateFileBytes / (1024 * 1024)
	return strings.Join([]string{
		"Usage tips for /test-animate:",
		"- Use /test-animate create to generate an output file.",
		"- Use /test-animate help anytime to display this guide.",
		"- Input animation file: animated GIF, MP4, or M4P.",
		"- Output format matches input format (GIF->GIF, MP4/M4P->video).",
		fmt.Sprintf("- Current attachment size limit is %d MiB per file (subject to change).", limitMiB),
		"- CSV header must be exactly: Frame,X,Y,Width,Visibility,Rotation,Opacity",
		"- Coordinate system: (0,0) is the upper-left corner of each frame.",
		"- Frame: 1-based frame index to apply this overlay row.",
		"- X, Y: center position in pixels where the token is placed.",
		"- Width: token size in pixels (square dimensions).",
		"- Visibility: use Visible/Hidden (Hidden skips drawing for that row).",
		"- Rotation: degrees of rotation applied to the token image.",
		"- Opacity: alpha 0-255 (0 transparent, 255 fully visible).",
		"- Multiple rows for the same frame are merged in CSV order.",
		"- Frames missing from the CSV receive no overlay.",
	}, "\n")
}

func getFrameDetailsText(inputFormat string, mediaBytes []byte, csvBytes []byte, outExt string) string {
	if inputFormat == "gif" {
		decoded, err := gif.DecodeAll(bytes.NewReader(mediaBytes))
		if err == nil {
			return fmt.Sprintf("Detected %d GIF frames.", len(decoded.Image))
		}
		return "Detected GIF input."
	}

	if frameCount, err := probeVideoFrameCount(mediaBytes, outExt); err == nil && frameCount > 0 {
		return fmt.Sprintf("Detected %d video frames.", frameCount)
	}

	if overlayRows, maxFrame := csvFrameSummary(csvBytes); overlayRows > 0 {
		return fmt.Sprintf("Video frame count unavailable; CSV has %d overlay rows targeting up to frame %d.", overlayRows, maxFrame)
	}

	return "Video frame count unavailable."
}

func csvFrameSummary(csvBytes []byte) (overlayRows int, maxFrame int) {
	rows, err := parseTrackingCSV(csvBytes, 0)
	if err != nil {
		return 0, 0
	}
	for _, row := range rows {
		overlayRows++
		if row.Frame > maxFrame {
			maxFrame = row.Frame
		}
	}
	return overlayRows, maxFrame
}

func probeVideoFrameCount(videoBytes []byte, outExt string) (int, error) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return 0, err
	}

	tmpDir, err := os.MkdirTemp("", "ttbb-test-animate-probe-*")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "probe-input"+outExt)
	if err := os.WriteFile(inputPath, videoBytes, 0600); err != nil {
		return 0, err
	}

	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-count_frames",
		"-show_entries", "stream=nb_read_frames",
		"-of", "default=nokey=1:noprint_wrappers=1",
		inputPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" || trimmed == "N/A" {
		return 0, fmt.Errorf("frame count unavailable")
	}
	count, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func detectAnimationFormat(att *discordgo.MessageAttachment, data []byte) (format string, outExt string, contentType string, err error) {
	lowerName := strings.ToLower(att.Filename)
	ext := strings.ToLower(filepath.Ext(lowerName))

	if isGIFBytes(data) || strings.Contains(strings.ToLower(att.ContentType), "gif") {
		return "gif", ".gif", "image/gif", nil
	}

	if isISOBaseMediaBytes(data) {
		switch ext {
		case ".m4p":
			return "video", ".m4p", "video/mp4", nil
		case ".mp4":
			return "video", ".mp4", "video/mp4", nil
		default:
			if strings.Contains(strings.ToLower(att.ContentType), "mp4") {
				return "video", ".mp4", "video/mp4", nil
			}
		}
	}

	return "", "", "", fmt.Errorf("unsupported input format: upload an animated GIF or MP4/M4P video")
}

func isGIFBytes(data []byte) bool {
	if len(data) < 6 {
		return false
	}
	header := string(data[:6])
	return header == "GIF87a" || header == "GIF89a"
}

func isISOBaseMediaBytes(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	return string(data[4:8]) == "ftyp"
}

func sendTestAnimateError(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: msg,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

func getCommandAttachment(i *discordgo.InteractionCreate, opt *discordgo.ApplicationCommandInteractionDataOption) *discordgo.MessageAttachment {
	if opt == nil || opt.Type != discordgo.ApplicationCommandOptionAttachment {
		return nil
	}
	attachmentID, ok := opt.Value.(string)
	if !ok || attachmentID == "" {
		return nil
	}
	resolved := i.ApplicationCommandData().Resolved
	if resolved == nil {
		return nil
	}
	return resolved.Attachments[attachmentID]
}

func downloadAttachmentBytes(att *discordgo.MessageAttachment) ([]byte, error) {
	if att.Size > maxAnimateFileBytes {
		return nil, fmt.Errorf("attachment %q is too large (%d bytes)", att.Filename, att.Size)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, att.URL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("download failed with status %s", resp.Status)
	}

	limited := io.LimitReader(resp.Body, maxAnimateFileBytes+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > maxAnimateFileBytes {
		return nil, fmt.Errorf("attachment %q exceeds %d bytes", att.Filename, maxAnimateFileBytes)
	}
	return buf, nil
}

func buildTokenOverlayGIF(gifBytes []byte, csvBytes []byte) ([]byte, error) {
	sourceGIF, err := gif.DecodeAll(bytes.NewReader(gifBytes))
	if err != nil {
		return nil, fmt.Errorf("invalid GIF: %w", err)
	}
	if len(sourceGIF.Image) < 2 {
		return nil, fmt.Errorf("input file must be an animated GIF with at least 2 frames")
	}

	rows, err := parseTrackingCSV(csvBytes, len(sourceGIF.Image))
	if err != nil {
		return nil, err
	}
	rowsByFrame := make(map[int][]animationTrackingRow, len(sourceGIF.Image))
	for _, row := range rows {
		rowsByFrame[row.Frame] = append(rowsByFrame[row.Frame], row)
	}

	tokenImg, err := loadTokenImage()
	if err != nil {
		return nil, err
	}
	frameRect := gifCanvasBounds(sourceGIF)
	composited := image.NewNRGBA(frameRect)
	var previousCanvas *image.NRGBA
	var previousFrameBounds image.Rectangle
	var previousDisposal byte

	result := &gif.GIF{
		Image:           make([]*image.Paletted, 0, len(sourceGIF.Image)),
		Delay:           append([]int(nil), sourceGIF.Delay...),
		LoopCount:       sourceGIF.LoopCount,
		Config:          image.Config{Width: frameRect.Dx(), Height: frameRect.Dy()},
		BackgroundIndex: sourceGIF.BackgroundIndex,
	}

	for idx, srcFrame := range sourceGIF.Image {
		if idx > 0 {
			switch previousDisposal {
			case gif.DisposalBackground:
				stdDraw.Draw(composited, previousFrameBounds, image.Transparent, image.Point{}, stdDraw.Src)
			case gif.DisposalPrevious:
				if previousCanvas != nil {
					copy(composited.Pix, previousCanvas.Pix)
				}
			}
		}

		currentDisposal := byte(0)
		if idx < len(sourceGIF.Disposal) {
			currentDisposal = sourceGIF.Disposal[idx]
		}
		if currentDisposal == gif.DisposalPrevious {
			previousCanvas = cloneNRGBA(composited)
		} else {
			previousCanvas = nil
		}

		stdDraw.Draw(composited, srcFrame.Bounds(), srcFrame, srcFrame.Bounds().Min, stdDraw.Over)
		canvas := cloneNRGBA(composited)

		for _, row := range rowsByFrame[idx+1] {
			if !strings.EqualFold(row.Visibility, "visible") {
				continue
			}
			overlay := rotateNRGBA(tokenImg, row.Rotation)
			overlay = resizeNRGBA(overlay, row.Width, row.Width)
			applyOpacity(overlay, row.Opacity/255.0)

			pasteX := row.X - (row.Width / 2)
			pasteY := row.Y - (row.Width / 2)
			rect := image.Rect(pasteX, pasteY, pasteX+overlay.Bounds().Dx(), pasteY+overlay.Bounds().Dy())
			stdDraw.Draw(canvas, rect, overlay, image.Point{}, stdDraw.Over)
		}

		paletted := image.NewPaletted(frameRect, palette.Plan9)
		stdDraw.FloydSteinberg.Draw(paletted, paletted.Rect, canvas, image.Point{})
		result.Image = append(result.Image, paletted)

		previousFrameBounds = srcFrame.Bounds()
		previousDisposal = currentDisposal
	}

	var out bytes.Buffer
	if err := gif.EncodeAll(&out, result); err != nil {
		return nil, fmt.Errorf("failed to encode output GIF: %w", err)
	}
	return out.Bytes(), nil
}

func gifCanvasBounds(g *gif.GIF) image.Rectangle {
	width := g.Config.Width
	height := g.Config.Height
	if width > 0 && height > 0 {
		return image.Rect(0, 0, width, height)
	}

	maxX := 0
	maxY := 0
	for _, frame := range g.Image {
		if frame.Bounds().Max.X > maxX {
			maxX = frame.Bounds().Max.X
		}
		if frame.Bounds().Max.Y > maxY {
			maxY = frame.Bounds().Max.Y
		}
	}
	if maxX < 1 {
		maxX = 1
	}
	if maxY < 1 {
		maxY = 1
	}
	return image.Rect(0, 0, maxX, maxY)
}

func buildTokenOverlayVideo(videoBytes []byte, csvBytes []byte, outExt string) ([]byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("video input requires ffmpeg to be installed on the bot host")
	}

	rows, err := parseTrackingCSV(csvBytes, 0)
	if err != nil {
		return nil, err
	}

	visibleRows := make([]animationTrackingRow, 0, len(rows))
	for _, row := range rows {
		if strings.EqualFold(row.Visibility, "visible") {
			visibleRows = append(visibleRows, row)
		}
	}
	if len(visibleRows) == 0 {
		return videoBytes, nil
	}

	tokenPath, err := ensureTokenImageFile()
	if err != nil {
		return nil, fmt.Errorf("failed preparing token image: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "ttbb-test-animate-*")
	if err != nil {
		return nil, fmt.Errorf("failed creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input"+outExt)
	if err := os.WriteFile(inputPath, videoBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed writing temp input video: %w", err)
	}

	outputPath := filepath.Join(tmpDir, "output"+outExt)

	parts := make([]string, 0, len(visibleRows)*2)
	currentVideoLabel := "[0:v]"
	for idx, row := range visibleRows {
		tokenLabel := fmt.Sprintf("tok%d", idx)
		outLabel := fmt.Sprintf("v%d", idx)
		alpha := row.Opacity / 255.0
		if alpha < 0 {
			alpha = 0
		}
		if alpha > 1 {
			alpha = 1
		}

		parts = append(parts,
			fmt.Sprintf("[1:v]scale=%d:%d,rotate=%0.8f*PI/180:ow=rotw(iw):oh=roth(ih):c=none,format=rgba,colorchannelmixer=aa=%0.6f[%s]",
				row.Width, row.Width, row.Rotation, alpha, tokenLabel),
		)
		parts = append(parts,
			fmt.Sprintf("%s[%s]overlay=x=%d:y=%d:enable='eq(n\\,%d)'[%s]",
				currentVideoLabel, tokenLabel, row.X-(row.Width/2), row.Y-(row.Width/2), row.Frame-1, outLabel),
		)
		currentVideoLabel = "[" + outLabel + "]"
	}
	filterComplex := strings.Join(parts, ";")

	args := []string{
		"-y",
		"-i", inputPath,
		"-i", tokenPath,
		"-filter_complex", filterComplex,
		"-map", currentVideoLabel,
		"-map", "0:a?",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-movflags", "+faststart",
		outputPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg video processing failed: %v (%s)", err, strings.TrimSpace(string(output)))
	}

	result, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading processed output video: %w", err)
	}
	return result, nil
}

func loadTokenImage() (*image.NRGBA, error) {
	filePath, err := ensureTokenImageFile()
	if err != nil {
		return nil, fmt.Errorf("failed loading token image %q: %w", testAnimateTokenPath, err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed loading token image %q: %w", filePath, err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("invalid token image %q: %w", filePath, err)
	}
	return toNRGBA(img), nil
}

func ensureTokenImageFile() (string, error) {
	filePath := filepath.Clean(testAnimateTokenPath)
	if _, statErr := os.Stat(filePath); os.IsNotExist(statErr) {
		if err := downloadTokenOverlayFromRepo(filePath); err != nil {
			return "", err
		}
	}
	return filePath, nil
}

func downloadTokenOverlayFromRepo(localFilePath string) error {
	if err := os.MkdirAll(filepath.Dir(localFilePath), 0755); err != nil {
		return fmt.Errorf("failed creating local directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := github.NewClient(nil)
	content, _, _, err := client.Repositories.GetContents(ctx, "mkmccarty", "TokenTimeBoostBot", testAnimateTokenPath, &github.RepositoryContentGetOptions{Ref: "main"})
	if err != nil {
		return fmt.Errorf("failed to resolve token file in repository: %w", err)
	}
	downloadURL := content.GetDownloadURL()
	if downloadURL == "" {
		return fmt.Errorf("token image download URL not found in repository")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("token image download failed with status %s", resp.Status)
	}

	outFile, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed creating token image file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("failed writing token image file: %w", err)
	}
	return nil
}

func parseTrackingCSV(raw []byte, expectedFrames int) ([]animationTrackingRow, error) {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	reader := csv.NewReader(bytes.NewReader(raw))
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV header: %w", err)
	}

	expected := []string{"Frame", "X", "Y", "Width", "Visibility", "Rotation", "Opacity"}
	if len(header) != len(expected) {
		return nil, fmt.Errorf("CSV header must be exactly: %s", strings.Join(expected, ","))
	}
	for idx := range expected {
		if strings.TrimSpace(header[idx]) != expected[idx] {
			return nil, fmt.Errorf("CSV header must be exactly: %s", strings.Join(expected, ","))
		}
	}

	rows := make([]animationTrackingRow, 0, expectedFrames)
	lineNumber := 1
	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		lineNumber++
		if readErr != nil {
			return nil, fmt.Errorf("error reading CSV on line %d: %w", lineNumber, readErr)
		}
		if len(record) != len(expected) {
			return nil, fmt.Errorf("line %d must have %d columns", lineNumber, len(expected))
		}

		frame, err := strconv.Atoi(strings.TrimSpace(record[0]))
		if err != nil {
			return nil, fmt.Errorf("line %d has invalid Frame value", lineNumber)
		}
		if frame < 1 {
			return nil, fmt.Errorf("line %d has frame %d out of range; expected >= 1", lineNumber, frame)
		}
		if expectedFrames > 0 && frame > expectedFrames {
			return nil, fmt.Errorf("line %d has frame %d out of range; expected 1-%d", lineNumber, frame, expectedFrames)
		}
		x, err := strconv.Atoi(strings.TrimSpace(record[1]))
		if err != nil {
			return nil, fmt.Errorf("line %d has invalid X value", lineNumber)
		}
		y, err := strconv.Atoi(strings.TrimSpace(record[2]))
		if err != nil {
			return nil, fmt.Errorf("line %d has invalid Y value", lineNumber)
		}
		width, err := strconv.Atoi(strings.TrimSpace(record[3]))
		if err != nil || width <= 0 {
			return nil, fmt.Errorf("line %d has invalid Width value", lineNumber)
		}
		visibility := strings.TrimSpace(record[4])
		if !strings.EqualFold(visibility, "visible") && !strings.EqualFold(visibility, "hidden") {
			return nil, fmt.Errorf("line %d has invalid Visibility value; use Visible or Hidden", lineNumber)
		}
		rotation, err := strconv.ParseFloat(strings.TrimSpace(record[5]), 64)
		if err != nil {
			return nil, fmt.Errorf("line %d has invalid Rotation value", lineNumber)
		}
		opacity, err := strconv.ParseFloat(strings.TrimSpace(record[6]), 64)
		if err != nil || opacity < 0 || opacity > 255 {
			return nil, fmt.Errorf("line %d has invalid Opacity value; expected 0-255", lineNumber)
		}

		rows = append(rows, animationTrackingRow{
			Frame:      frame,
			X:          x,
			Y:          y,
			Width:      width,
			Visibility: visibility,
			Rotation:   rotation,
			Opacity:    opacity,
		})
	}

	return rows, nil
}

func toNRGBA(src image.Image) *image.NRGBA {
	bounds := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	stdDraw.Draw(dst, dst.Bounds(), src, bounds.Min, stdDraw.Src)
	return dst
}

func cloneNRGBA(src *image.NRGBA) *image.NRGBA {
	dst := image.NewNRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

func resizeNRGBA(src *image.NRGBA, width, height int) *image.NRGBA {
	if width <= 0 || height <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), stdDraw.Over, nil)
	return dst
}

func rotateNRGBA(src *image.NRGBA, degrees float64) *image.NRGBA {
	angle := math.Mod(degrees, 360)
	if angle < 0 {
		angle += 360
	}
	if angle == 0 {
		return src
	}

	radians := angle * math.Pi / 180.0
	cosA := math.Cos(radians)
	sinA := math.Sin(radians)

	sw := float64(src.Bounds().Dx())
	sh := float64(src.Bounds().Dy())
	dw := int(math.Ceil(math.Abs(sw*cosA) + math.Abs(sh*sinA)))
	dh := int(math.Ceil(math.Abs(sw*sinA) + math.Abs(sh*cosA)))
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}

	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	srcCx := sw / 2.0
	srcCy := sh / 2.0
	dstCx := float64(dw) / 2.0
	dstCy := float64(dh) / 2.0

	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			dx := float64(x) - dstCx
			dy := float64(y) - dstCy

			sx := cosA*dx + sinA*dy + srcCx
			sy := -sinA*dx + cosA*dy + srcCy

			ix := int(math.Round(sx))
			iy := int(math.Round(sy))
			if ix >= 0 && iy >= 0 && ix < int(sw) && iy < int(sh) {
				srcOffset := src.PixOffset(ix, iy)
				dstOffset := dst.PixOffset(x, y)
				copy(dst.Pix[dstOffset:dstOffset+4], src.Pix[srcOffset:srcOffset+4])
			}
		}
	}
	return dst
}

func applyOpacity(img *image.NRGBA, alphaScale float64) {
	if alphaScale <= 0 {
		for idx := 3; idx < len(img.Pix); idx += 4 {
			img.Pix[idx] = 0
		}
		return
	}
	if alphaScale > 1 {
		alphaScale = 1
	}
	for idx := 3; idx < len(img.Pix); idx += 4 {
		img.Pix[idx] = uint8(math.Round(float64(img.Pix[idx]) * alphaScale))
	}
}
