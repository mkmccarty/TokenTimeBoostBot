package bottools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

var discordMarkdownEscaper = strings.NewReplacer(
	"\\", "\\\\",
	"*", "\\*",
	"_", "\\_",
	"~", "\\~",
	"`", "\\`",
	"|", "\\|",
	">", "\\>",
	"#", "\\#",
	"[", "\\[",
	"]", "\\]",
	"(", "\\(",
	")", "\\)",
)

// EscapeDiscordMarkdown escapes special characters in a string to prevent Discord from interpreting them as Markdown formatting.
func EscapeDiscordMarkdown(s string) string {
	if s == "" {
		return ""
	}
	return discordMarkdownEscaper.Replace(s)
}

const (
	// MaxAnimateFileBytes defines the maximum allowed file size for attachments to be processed by the bot.
	MaxAnimateFileBytes = 10 * 1024 * 1024
	// MintPreviewMaxAge defines how long a mint preview is considered valid before it should be refreshed.
	MintPreviewMaxAge = 20 * time.Minute
)

// FindCategoryID walks up the parent chain until it finds the category.
// Returns the category ID, or "" if err.
func FindCategoryID(client dc.Client, channelID string) (string, error) {
	ch, err := client.Channel(channelID)
	if err != nil {
		return "", err
	}
	// Is a category.
	if ch.IsCategory {
		return ch.ID, nil
	}
	// No parent found.
	if ch.ParentID == "" {
		return "", nil
	}

	// Recurse up the parent chain.
	return FindCategoryID(client, ch.ParentID)
}

// IsValidDiscordID checks if a string is a valid Discord snowflake ID by validating
// the embedded timestamp is within Discord's operational range
func IsValidDiscordID(id string) bool {
	// Discord IDs are 17-20 digit numbers
	if len(id) < 17 || len(id) > 20 {
		return false
	}

	// Parse as int64
	snowflake, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return false
	}

	// Extract timestamp: top 42 bits, milliseconds since Discord epoch (Jan 1, 2015)
	const discordEpoch int64 = 1420070400000
	timestamp := (snowflake >> 22) + discordEpoch

	// Validate timestamp is after Discord's launch and not too far in future
	now := time.Now().UnixMilli()
	tenYearsFromNow := now + (10 * 365 * 24 * 60 * 60 * 1000)

	return timestamp >= discordEpoch && timestamp <= tenYearsFromNow
}

// DownloadAttachmentBytesDC downloads the attachment content and returns it
// as a byte slice, given the dc facade's neutral Attachment form.
func DownloadAttachmentBytesDC(att *dc.Attachment) ([]byte, error) {
	if att.Size > MaxAnimateFileBytes {
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

	limited := io.LimitReader(resp.Body, MaxAnimateFileBytes+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > MaxAnimateFileBytes {
		return nil, fmt.Errorf("attachment %q exceeds %d bytes", att.Filename, MaxAnimateFileBytes)
	}
	return buf, nil
}
