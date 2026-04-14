package boost

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"image"
	"image/color/palette"
	stdDraw "image/draw"
	"image/gif"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/go-github/v33/github"
	xdraw "golang.org/x/image/draw"
)

const (
	testAnimateGIFOption   = "gif"
	testAnimateCSVOption   = "csv"
	testAnimateCreateSub   = "create"
	testAnimateHelpSub     = "help"
	testAnimateTokenPath   = "emoji/token_overlay.png"
	mintPreviewPrefix      = "mint_preview"
	mintPreviewProceed     = "proceed"
	mintPreviewUpdateCSV   = "updatecsv"
	mintPreviewClose       = "close"
	maxAnimateFileBytes    = 10 * 1024 * 1024
	testAnimateCleanupAge  = 2 * time.Hour
	mintPreviewMaxAge      = 20 * time.Minute
	mintEstimateMultiplier = 1.25
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

type mintPreviewSelection struct {
	InitialFrame int
	DistantFrame int
	InitialX     int
	InitialY     int
	DistantX     int
	DistantY     int
	Distance     float64
}

type mintPreviewSession struct {
	SessionID             string
	UserID                string
	ChannelID             string
	InputFormat           string
	OutExt                string
	OutContentType        string
	MediaBytes            []byte
	CSVBytes              []byte
	Interaction           *discordgo.Interaction
	AwaitingCSV           bool
	PreviewSampleDuration time.Duration
	PreviewSampleFrames   int
	UpdatedAt             time.Time
}

var mintPreviewMu sync.Mutex
var mintPreviewSessions = make(map[string]*mintPreviewSession)

// GetSlashMintCommand creates the /mint command.
func GetSlashMintCommand(cmd string) *discordgo.ApplicationCommand {
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
				Description: "Show usage help for /mint",
			},
		},
	}
}

// HandleMintCommand validates user-uploaded GIF/CSV and returns an overlaid animation.
func HandleMintCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if len(data.Options) == 0 {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Please choose a subcommand: /mint create or /mint help.",
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
				Content: "Invalid usage. Use /mint create or /mint help.",
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
				Content: "Unknown subcommand. Use /mint create or /mint help.",
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

	session := &mintPreviewSession{
		SessionID:      fmt.Sprintf("%d", time.Now().UnixNano()),
		UserID:         getInteractionUserID(i),
		ChannelID:      i.ChannelID,
		InputFormat:    inputFormat,
		OutExt:         outExt,
		OutContentType: outContentType,
		MediaBytes:     gifBytes,
		CSVBytes:       csvBytes,
		Interaction:    i.Interaction,
		UpdatedAt:      time.Now(),
	}

	mintPreviewMu.Lock()
	cleanupExpiredMintPreviewSessionsLocked(time.Now())
	mintPreviewSessions[session.SessionID] = session
	mintPreviewMu.Unlock()

	if err := sendMintPreviewMessage(s, session); err != nil {
		sendTestAnimateError(s, i, err.Error())
		return
	}
}

func cleanupOldTestAnimateFiles(maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return 0, fmt.Errorf("failed reading temp directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	var firstErr error

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "ttbb-mint-") && !strings.HasPrefix(name, "ttbb-mint-probe-") {
			continue
		}

		fullPath := filepath.Join(os.TempDir(), name)
		info, statErr := entry.Info()
		if statErr != nil {
			if firstErr == nil {
				firstErr = statErr
			}
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}

		if removeErr := os.RemoveAll(fullPath); removeErr != nil {
			if firstErr == nil {
				firstErr = removeErr
			}
			continue
		}
		removed++
	}

	return removed, firstErr
}

func buildTestAnimateUsageText() string {
	limitMiB := maxAnimateFileBytes / (1024 * 1024)
	return strings.Join([]string{
		"Usage tips for /mint:",
		"- Use /mint create to generate an output file.",
		"- Use /mint help anytime to display this guide.",
		"- Input animation file: animated GIF, MP4, or M4P.",
		"- Output format matches input format (GIF->GIF, MP4/M4P->video).",
		fmt.Sprintf("- Current attachment size limit is %d MiB per file (subject to change).", limitMiB),
		"- CSV header must be exactly: Frame,X,Y,Width,Visibility,Rotation,Opacity",
		"- Frame: 1-based frame index or inclusive range (for example: 12 or 12-20).",
		"- When a range is used, that row is duplicated and applied to every frame in the range.",
		"- Coordinate system: (0,0) is the upper-left corner of each frame.",
		"- X, Y: center position in pixels where the token is placed.",
		"- Width: token size in pixels (square dimensions).",
		"- Visibility: use Visible/Hidden (Hidden skips drawing for that row).",
		"- Rotation: degrees as seen by the viewer (positive = clockwise, negative = counterclockwise); examples: 0 = no rotation, 90 = quarter-turn right, 180 = upside down, -45 = slight left tilt.",
		"- Opacity: alpha 0-255 (0 transparent, 255 fully visible).",
		"- Multiple rows for the same frame are merged in CSV order.",
		"- Frames missing from the CSV receive no overlay.",
		"",
		"## Tips for creating tracking CSVs:",
		"- RAIYC currently uses Acorn for macOS (by Flying Meat) to capture X, Y, and token width for each frame. If you know good web or cross-platform alternatives, please share them so this guide can be updated.",
		"",
		"## Example Input GIF and CSV:",
		"https://media4.giphy.com/media/v1.Y2lkPTc5MGI3NjExNW05Z3g3NXh5emJhMm5xdTg5b3E2cjVhcmVxZzIyeDNmNjdvNjh6dyZlcD12MV9pbnRlcm5hbF9naWZfYnlfaWQmY3Q9Zw/26ufbnExH8CwvAN3O/giphy.gif",
		"```csv",
		"Frame,X,Y,Width,Visibility,Rotation,Opacity",
		"1,145,126,57,Visible,0,255",
		"2-3,145,129,58,Visible,0,255",
		"4,145,139,62,Visible,0,255",
		"5,145,141,63,Visible,0,255",
		"6,145,152,65,Visible,0,255",
		"7-8,145,155,68,Visible,0,255",
		"9,145,165,72,Visible,0,255",
		"10,145,168,73,Visible,0,255",
		"11,145,177,77,Visible,0,255",
		"12,145,180,80,Visible,0,255",
		"13,145,180,81,Visible,0,255",
		"14,145,190,83,Visible,0,255",
		"```",
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

	tmpDir, err := os.MkdirTemp("", "ttbb-mint-probe-*")
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

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

// HandleMintPreviewComponent handles button interactions for mint preview flows.
func HandleMintPreviewComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) != 3 {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Invalid mint preview action.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	sessionID := parts[1]
	action := parts[2]
	userID := getInteractionUserID(i)

	mintPreviewMu.Lock()
	cleanupExpiredMintPreviewSessionsLocked(time.Now())
	session, ok := mintPreviewSessions[sessionID]
	if ok {
		session.UpdatedAt = time.Now()
	}
	mintPreviewMu.Unlock()

	if !ok {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This mint preview has expired. Please run /mint create again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	if session.UserID != userID {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Only the original requester can use these preview controls.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	switch action {
	case mintPreviewProceed:
		estimateText := describeMintRenderEstimate(session)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    estimateText,
				Components: []discordgo.MessageComponent{},
				Flags:      discordgo.MessageFlagsEphemeral,
			},
		})

		outputData, err := renderMintOutput(session)
		if err != nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: err.Error(),
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			return
		}

		removedCount, cleanupErr := cleanupOldTestAnimateFiles(testAnimateCleanupAge)
		cleanupNote := fmt.Sprintf("Reminder: non-token temporary files older than %s are cleaned up after each run.", testAnimateCleanupAge.Round(time.Hour))
		if cleanupErr != nil {
			cleanupNote += " (Cleanup hit some errors; it will retry on future runs.)"
		} else if removedCount > 0 {
			cleanupNote += fmt.Sprintf(" Removed %d stale file(s)/folder(s) this run.", removedCount)
		}

		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Rendering complete. Generated file:\n" + cleanupNote,
			Files: []*discordgo.File{
				{
					Name:        "mint-output" + session.OutExt,
					ContentType: session.OutContentType,
					Reader:      bytes.NewReader(outputData),
				},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		})

		mintPreviewMu.Lock()
		delete(mintPreviewSessions, session.SessionID)
		mintPreviewMu.Unlock()

	case mintPreviewUpdateCSV:
		mintPreviewMu.Lock()
		session.AwaitingCSV = true
		session.UpdatedAt = time.Now()
		mintPreviewMu.Unlock()

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})

		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Upload a new CSV file in this channel (same user) and I will regenerate the preview using the original animation.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})

		return

	case mintPreviewClose:
		mintPreviewMu.Lock()
		delete(mintPreviewSessions, session.SessionID)
		mintPreviewMu.Unlock()

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Mint preview closed.",
				Components: []discordgo.MessageComponent{},
				Flags:      discordgo.MessageFlagsEphemeral,
			},
		})

	default:
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown mint preview action.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}
}

// HandleMintCSVUploadMessage consumes a replacement CSV attachment for an active preview update request.
func HandleMintCSVUploadMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil || m.Author == nil || m.Author.Bot || len(m.Attachments) == 0 {
		return
	}

	now := time.Now()
	var selected *mintPreviewSession
	var sameUserDifferentChannel *mintPreviewSession

	mintPreviewMu.Lock()
	cleanupExpiredMintPreviewSessionsLocked(now)
	for _, session := range mintPreviewSessions {
		if !session.AwaitingCSV {
			continue
		}
		if session.UserID != m.Author.ID {
			continue
		}
		if session.ChannelID == m.ChannelID {
			if selected == nil || session.UpdatedAt.After(selected.UpdatedAt) {
				selected = session
			}
			continue
		}
		if sameUserDifferentChannel == nil || session.UpdatedAt.After(sameUserDifferentChannel.UpdatedAt) {
			sameUserDifferentChannel = session
		}
	}
	mintPreviewMu.Unlock()

	if selected == nil {
		if sameUserDifferentChannel != nil {
			msg := fmt.Sprintf("I am waiting for your replacement CSV in channel ID %s. Please upload it there, or click 'Update the CSV file' again in the newest preview.", sameUserDifferentChannel.ChannelID)
			if _, err := s.ChannelMessageSend(m.ChannelID, msg); err != nil {
				log.Printf("mint csv update notice send failed: %v", err)
			}
		}
		return
	}

	att := pickCSVAttachment(m.Attachments)
	if att == nil {
		if _, err := s.ChannelMessageSend(m.ChannelID, "Please upload a CSV file attachment."); err != nil {
			log.Printf("mint csv update prompt send failed: %v", err)
		}
		return
	}

	csvBytes, err := downloadAttachmentBytes(att)
	if err != nil {
		if _, sendErr := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Failed downloading CSV: %v", err)); sendErr != nil {
			log.Printf("mint csv download error send failed: %v", sendErr)
		}
		return
	}

	mintPreviewMu.Lock()
	selected.CSVBytes = csvBytes
	selected.AwaitingCSV = false
	selected.UpdatedAt = time.Now()
	mintPreviewMu.Unlock()

	if err := sendMintPreviewMessage(s, selected); err != nil {
		if _, sendErr := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Could not render updated preview: %v", err)); sendErr != nil {
			log.Printf("mint csv preview error send failed: %v", sendErr)
		}
		return
	}

	if _, err := s.ChannelMessageSend(m.ChannelID, "Updated CSV received. A new preview was sent."); err != nil {
		log.Printf("mint csv success send failed: %v", err)
	}
}

func sendMintPreviewMessage(s *discordgo.Session, session *mintPreviewSession) error {
	start := time.Now()
	initialGIF, distantGIF, detailsText, err := buildMintPreviewAssets(session)
	if err != nil {
		return err
	}
	previewDuration := time.Since(start)

	mintPreviewMu.Lock()
	session.PreviewSampleDuration = previewDuration
	session.PreviewSampleFrames = 2
	session.UpdatedAt = time.Now()
	mintPreviewMu.Unlock()

	frameDetails := getFrameDetailsText(session.InputFormat, session.MediaBytes, session.CSVBytes, session.OutExt)
	buttons := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Okay to proceed",
				Style:    discordgo.SuccessButton,
				CustomID: fmt.Sprintf("%s#%s#%s", mintPreviewPrefix, session.SessionID, mintPreviewProceed),
			},
			discordgo.Button{
				Label:    "Close",
				Style:    discordgo.DangerButton,
				CustomID: fmt.Sprintf("%s#%s#%s", mintPreviewPrefix, session.SessionID, mintPreviewClose),
			},
		}},
	}

	_, err = s.FollowupMessageCreate(session.Interaction, true, &discordgo.WebhookParams{
		Content: "Preview before full render:\n" + frameDetails + "\n" + detailsText,
		Files: []*discordgo.File{
			{
				Name:        "mint-preview-initial.gif",
				ContentType: "image/gif",
				Reader:      bytes.NewReader(initialGIF),
			},
			{
				Name:        "mint-preview-distant.gif",
				ContentType: "image/gif",
				Reader:      bytes.NewReader(distantGIF),
			},
		},
		Components: buttons,
		Flags:      discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		return fmt.Errorf("failed sending mint preview: %w", err)
	}

	return nil
}

func describeMintRenderEstimate(session *mintPreviewSession) string {
	estimate := estimateMintRenderDuration(session)
	if estimate <= 0 {
		return "Preview approved\nRendering full output now..."
	}

	pretty := estimate.Round(time.Second)
	if pretty < time.Second {
		pretty = time.Second
	}
	return fmt.Sprintf("Preview approved\nEstimated time: about %s.", pretty)
}

func estimateMintRenderDuration(session *mintPreviewSession) time.Duration {
	sampleDuration := session.PreviewSampleDuration
	sampleFrames := session.PreviewSampleFrames
	if sampleDuration <= 0 || sampleFrames < 1 {
		return 0
	}

	totalFrames := detectMintTotalFramesForEstimate(session)
	if totalFrames < 1 {
		totalFrames = sampleFrames
	}

	est := (sampleDuration * time.Duration(totalFrames)) / time.Duration(sampleFrames)
	est = time.Duration(float64(est) * mintEstimateMultiplier)
	if est < sampleDuration {
		est = sampleDuration
	}
	return est
}

func detectMintTotalFramesForEstimate(session *mintPreviewSession) int {
	if session.InputFormat == "gif" {
		if decoded, err := gif.DecodeAll(bytes.NewReader(session.MediaBytes)); err == nil {
			return len(decoded.Image)
		}
		return 0
	}

	if frameCount, err := probeVideoFrameCount(session.MediaBytes, session.OutExt); err == nil && frameCount > 0 {
		return frameCount
	}

	_, maxFrame := csvFrameSummary(session.CSVBytes)
	return maxFrame
}

func buildMintPreviewAssets(session *mintPreviewSession) ([]byte, []byte, string, error) {
	rows, err := parseTrackingCSV(session.CSVBytes, 0)
	if err != nil {
		return nil, nil, "", err
	}
	if len(rows) == 0 {
		return nil, nil, "", errors.New("CSV has no tracking rows")
	}

	selection, err := selectMintPreviewFrames(rows)
	if err != nil {
		return nil, nil, "", err
	}

	initialFrameImg, err := renderPreviewFrame(session, rows, selection.InitialFrame)
	if err != nil {
		return nil, nil, "", err
	}
	distantFrameImg, err := renderPreviewFrame(session, rows, selection.DistantFrame)
	if err != nil {
		return nil, nil, "", err
	}

	initialGIF, err := encodeSingleFrameGIF(initialFrameImg)
	if err != nil {
		return nil, nil, "", err
	}
	distantGIF, err := encodeSingleFrameGIF(distantFrameImg)
	if err != nil {
		return nil, nil, "", err
	}

	details := fmt.Sprintf(
		"Initial frame: %d (x=%d, y=%d). Most distant frame: %d (x=%d, y=%d). Distance: %.2fpx.",
		selection.InitialFrame,
		selection.InitialX,
		selection.InitialY,
		selection.DistantFrame,
		selection.DistantX,
		selection.DistantY,
		selection.Distance,
	)

	return initialGIF, distantGIF, details, nil
}

func selectMintPreviewFrames(rows []animationTrackingRow) (mintPreviewSelection, error) {
	ordered := make([]animationTrackingRow, 0, len(rows))
	for _, row := range rows {
		if strings.EqualFold(row.Visibility, "visible") {
			ordered = append(ordered, row)
		}
	}
	if len(ordered) == 0 {
		ordered = append(ordered, rows...)
	}
	if len(ordered) == 0 {
		return mintPreviewSelection{}, errors.New("CSV has no rows")
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Frame < ordered[j].Frame
	})

	initial := ordered[0]
	farthest := ordered[0]
	maxDist := -1.0
	for _, row := range ordered {
		dx := float64(row.X - initial.X)
		dy := float64(row.Y - initial.Y)
		d := math.Sqrt((dx * dx) + (dy * dy))
		if d > maxDist {
			maxDist = d
			farthest = row
		}
	}

	if maxDist < 0 {
		maxDist = 0
	}

	return mintPreviewSelection{
		InitialFrame: initial.Frame,
		DistantFrame: farthest.Frame,
		InitialX:     initial.X,
		InitialY:     initial.Y,
		DistantX:     farthest.X,
		DistantY:     farthest.Y,
		Distance:     maxDist,
	}, nil
}

func renderPreviewFrame(session *mintPreviewSession, rows []animationTrackingRow, targetFrame int) (*image.NRGBA, error) {
	if targetFrame < 1 {
		return nil, fmt.Errorf("invalid frame %d", targetFrame)
	}

	rowsByFrame := make(map[int][]animationTrackingRow)
	for _, row := range rows {
		rowsByFrame[row.Frame] = append(rowsByFrame[row.Frame], row)
	}

	tokenImg, err := loadTokenImage()
	if err != nil {
		return nil, err
	}

	var canvas *image.NRGBA
	if session.InputFormat == "gif" {
		decoded, err := gif.DecodeAll(bytes.NewReader(session.MediaBytes))
		if err != nil {
			return nil, fmt.Errorf("invalid GIF: %w", err)
		}
		if targetFrame > len(decoded.Image) {
			return nil, fmt.Errorf("frame %d out of range; expected 1-%d", targetFrame, len(decoded.Image))
		}
		canvas, err = gifCanvasAtFrame(decoded, targetFrame)
		if err != nil {
			return nil, err
		}
	} else {
		canvas, err = extractVideoPreviewFrame(session.MediaBytes, session.OutExt, targetFrame)
		if err != nil {
			return nil, err
		}
	}

	for _, row := range rowsByFrame[targetFrame] {
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

	return canvas, nil
}

func gifCanvasAtFrame(g *gif.GIF, targetFrame int) (*image.NRGBA, error) {
	frameRect := gifCanvasBounds(g)
	composited := image.NewNRGBA(frameRect)
	var previousCanvas *image.NRGBA
	var previousFrameBounds image.Rectangle
	var previousDisposal byte

	for idx, srcFrame := range g.Image {
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
		if idx < len(g.Disposal) {
			currentDisposal = g.Disposal[idx]
		}
		if currentDisposal == gif.DisposalPrevious {
			previousCanvas = cloneNRGBA(composited)
		} else {
			previousCanvas = nil
		}

		stdDraw.Draw(composited, srcFrame.Bounds(), srcFrame, srcFrame.Bounds().Min, stdDraw.Over)
		if idx+1 == targetFrame {
			return cloneNRGBA(composited), nil
		}

		previousFrameBounds = srcFrame.Bounds()
		previousDisposal = currentDisposal
	}

	return nil, fmt.Errorf("frame %d out of range", targetFrame)
}

func extractVideoPreviewFrame(videoBytes []byte, outExt string, targetFrame int) (*image.NRGBA, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("video input requires ffmpeg to be installed on the bot host")
	}

	tmpDir, err := os.MkdirTemp("", "ttbb-mint-preview-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	inputPath := filepath.Join(tmpDir, "input"+outExt)
	if err := os.WriteFile(inputPath, videoBytes, 0600); err != nil {
		return nil, err
	}

	filter := fmt.Sprintf("select='eq(n\\,%d)'", targetFrame-1)
	cmd := exec.Command(
		"ffmpeg",
		"-v", "error",
		"-i", inputPath,
		"-vf", filter,
		"-vframes", "1",
		"-f", "image2pipe",
		"-vcodec", "png",
		"pipe:1",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg preview frame extraction failed: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("unable to extract frame %d from input video", targetFrame)
	}

	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, err
	}
	return toNRGBA(img), nil
}

func encodeSingleFrameGIF(img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	paletted := image.NewPaletted(image.Rect(0, 0, bounds.Dx(), bounds.Dy()), palette.Plan9)
	stdDraw.FloydSteinberg.Draw(paletted, paletted.Rect, img, bounds.Min)

	g := &gif.GIF{
		Image: []*image.Paletted{paletted},
		Delay: []int{20},
	}

	var out bytes.Buffer
	if err := gif.EncodeAll(&out, g); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func renderMintOutput(session *mintPreviewSession) ([]byte, error) {
	if session.InputFormat == "gif" {
		return buildTokenOverlayGIF(session.MediaBytes, session.CSVBytes)
	}
	return buildTokenOverlayVideo(session.MediaBytes, session.CSVBytes, session.OutExt)
}

func cleanupExpiredMintPreviewSessionsLocked(now time.Time) {
	for id, session := range mintPreviewSessions {
		if now.Sub(session.UpdatedAt) > mintPreviewMaxAge {
			delete(mintPreviewSessions, id)
		}
	}
}

func pickCSVAttachment(attachments []*discordgo.MessageAttachment) *discordgo.MessageAttachment {
	for _, att := range attachments {
		name := strings.ToLower(strings.TrimSpace(att.Filename))
		ct := strings.ToLower(strings.TrimSpace(att.ContentType))
		if strings.HasSuffix(name, ".csv") || strings.Contains(ct, "csv") {
			return att
		}
	}
	if len(attachments) > 0 {
		return attachments[0]
	}
	return nil
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
	defer func() {
		_ = resp.Body.Close()
	}()

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
	if len(sourceGIF.Image) < 1 {
		return nil, fmt.Errorf("input GIF must have at least 1 frame")
	}

	// Parse CSV without frame count limit first to discover max frame
	rows, err := parseTrackingCSV(csvBytes, 0)
	if err != nil {
		return nil, err
	}

	// Find the maximum frame number in CSV
	maxCSVFrame := 0
	for _, row := range rows {
		if row.Frame > maxCSVFrame {
			maxCSVFrame = row.Frame
		}
	}

	// If single-frame GIF but CSV has many frames, extend the GIF
	targetFrameCount := len(sourceGIF.Image)
	if len(sourceGIF.Image) == 1 && maxCSVFrame > 1 {
		targetFrameCount = maxCSVFrame
	}

	// Re-parse CSV with the actual target frame count for validation
	if len(sourceGIF.Image) > 1 {
		// For multi-frame GIFs, use actual frame count
		rows, err = parseTrackingCSV(csvBytes, len(sourceGIF.Image))
		if err != nil {
			return nil, err
		}
	}

	rowsByFrame := make(map[int][]animationTrackingRow, targetFrameCount)
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

	// Create delays array: use 5 FPS (20 centiseconds per frame)
	delays := make([]int, 0, targetFrameCount)
	for i := 0; i < targetFrameCount; i++ {
		delays = append(delays, 20)
	}

	result := &gif.GIF{
		Image:           make([]*image.Paletted, 0, targetFrameCount),
		Delay:           delays,
		LoopCount:       0, // 0 means infinite loop
		Config:          image.Config{Width: frameRect.Dx(), Height: frameRect.Dy()},
		BackgroundIndex: sourceGIF.BackgroundIndex,
	}

	for frameIdx := 0; frameIdx < targetFrameCount; frameIdx++ {
		// For single-frame GIFs extended to multiple frames, reuse the single frame
		var sourceFrameIdx int
		if len(sourceGIF.Image) == 1 {
			sourceFrameIdx = 0
		} else {
			sourceFrameIdx = frameIdx % len(sourceGIF.Image)
		}

		srcFrame := sourceGIF.Image[sourceFrameIdx]
		if frameIdx > 0 {
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
		if sourceFrameIdx < len(sourceGIF.Disposal) {
			currentDisposal = sourceGIF.Disposal[sourceFrameIdx]
		}
		if currentDisposal == gif.DisposalPrevious {
			previousCanvas = cloneNRGBA(composited)
		} else {
			previousCanvas = nil
		}

		stdDraw.Draw(composited, srcFrame.Bounds(), srcFrame, srcFrame.Bounds().Min, stdDraw.Over)
		canvas := cloneNRGBA(composited)

		for _, row := range rowsByFrame[frameIdx+1] {
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

	tmpDir, err := os.MkdirTemp("", "ttbb-mint-*")
	if err != nil {
		return nil, fmt.Errorf("failed creating temp directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

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
	defer func() {
		_ = file.Close()
	}()

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
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("token image download failed with status %s", resp.Status)
	}

	outFile, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed creating token image file: %w", err)
	}
	defer func() {
		_ = outFile.Close()
	}()

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

		frameSpec := strings.TrimSpace(record[0])
		startFrame := 0
		endFrame := 0
		if strings.Count(frameSpec, "-") == 1 {
			parts := strings.SplitN(frameSpec, "-", 2)
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			if left == "" || right == "" {
				return nil, fmt.Errorf("line %d has invalid Frame value", lineNumber)
			}

			startFrame, err = strconv.Atoi(left)
			if err != nil {
				return nil, fmt.Errorf("line %d has invalid Frame value", lineNumber)
			}
			endFrame, err = strconv.Atoi(right)
			if err != nil {
				return nil, fmt.Errorf("line %d has invalid Frame value", lineNumber)
			}
			if startFrame > endFrame {
				return nil, fmt.Errorf("line %d has invalid Frame range; expected start <= end", lineNumber)
			}
		} else if strings.Count(frameSpec, "-") > 1 {
			return nil, fmt.Errorf("line %d has invalid Frame value", lineNumber)
		} else {
			startFrame, err = strconv.Atoi(frameSpec)
			if err != nil {
				return nil, fmt.Errorf("line %d has invalid Frame value", lineNumber)
			}
			endFrame = startFrame
		}

		if startFrame < 1 {
			return nil, fmt.Errorf("line %d has frame %d out of range; expected >= 1", lineNumber, startFrame)
		}
		if expectedFrames > 0 && endFrame > expectedFrames {
			return nil, fmt.Errorf("line %d has frame %d out of range; expected 1-%d", lineNumber, endFrame, expectedFrames)
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
		if err != nil {
			return nil, fmt.Errorf("line %d has invalid Width value", lineNumber)
		}
		visibility := strings.TrimSpace(record[4])
		if !strings.EqualFold(visibility, "visible") && !strings.EqualFold(visibility, "hidden") {
			return nil, fmt.Errorf("line %d has invalid Visibility value; use Visible or Hidden", lineNumber)
		}
		if width < 0 || (width == 0 && !strings.EqualFold(visibility, "hidden")) {
			return nil, fmt.Errorf("line %d has invalid Width value", lineNumber)
		}
		rotation, err := strconv.ParseFloat(strings.TrimSpace(record[5]), 64)
		if err != nil {
			return nil, fmt.Errorf("line %d has invalid Rotation value", lineNumber)
		}
		opacity, err := strconv.ParseFloat(strings.TrimSpace(record[6]), 64)
		if err != nil || opacity < 0 || opacity > 255 {
			return nil, fmt.Errorf("line %d has invalid Opacity value; expected 0-255", lineNumber)
		}

		for frame := startFrame; frame <= endFrame; frame++ {
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
