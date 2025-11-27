package bottools

import (
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

// FmtDuration formats a time.Duration into a human readable string
func FmtDuration(d time.Duration) string {
	d = d.Round(time.Minute)

	days := d / (24 * time.Hour)
	hours := (d % (24 * time.Hour)) / time.Hour
	mins := (d % time.Hour) / time.Minute

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	return strings.Join(parts, "")
}

// FmtDurationSingleUnit formats a time.Duration into a single unit string and its corresponding unit integer
// Parameters:
//   - d (time.Duration)
//
// Returns:
//   - (string): the duration value as a string.
//   - (int): the duration unit as an integer (0: days, 1: hours, 2: minutes, 3: seconds).
func FmtDurationSingleUnit(d time.Duration) (string, int) {
	switch {
	case d%(24*time.Hour) == 0:
		return fmt.Sprintf("%d", d/(24*time.Hour)), 0
	case d%(time.Hour) == 0:
		return fmt.Sprintf("%d", d/time.Hour), 1
	case d%(time.Minute) == 0:
		return fmt.Sprintf("%d", d/time.Minute), 2
	default:
		return fmt.Sprintf("%d", d/time.Second), 3
	}
}

// SanitizeStringDuration takes an hms string and returns a sanitized version of it
func SanitizeStringDuration(s string) string {
	// Ignore everything after a comma
	if strings.Contains(s, ",") {
		s = strings.Split(s, ",")[0]
	}

	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "day", "d")
	s = strings.ReplaceAll(s, "min", "m")
	s = strings.ReplaceAll(s, "hr", "h")
	s = strings.ReplaceAll(s, "sec", "s")
	s = strings.ReplaceAll(s, " ", "")
	var days, hours, minutes, seconds string

	re := regexp.MustCompile(`(\d+)d`)
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		days = matches[1] + "d"
	}

	re = regexp.MustCompile(`(\d+)h`)
	matches = re.FindStringSubmatch(s)
	if len(matches) > 1 {
		hours = matches[1] + "h"
	}

	re = regexp.MustCompile(`(\d+)m`)
	matches = re.FindStringSubmatch(s)
	if len(matches) > 1 {
		minutes = matches[1] + "m"
	}

	re = regexp.MustCompile(`(\d+)s`)
	matches = re.FindStringSubmatch(s)
	if len(matches) > 1 {
		seconds = matches[1] + "s"
	}

	return days + hours + minutes + seconds
}

// StringAlign is an enum for string alignment
type StringAlign int

const (
	// StringAlignLeft aligns the string to the left
	StringAlignLeft StringAlign = iota
	// StringAlignCenter aligns the string to the center (left biased)
	StringAlignCenter
	// StringAlignRight aligns the string to the right
	StringAlignRight
	// StringAlignCenterRight aligns the string to the center (right biased)
	StringAlignCenterRight
)

// AlignString aligns a string to the left, center, or right within a given width
func AlignString(str string, width int, alignment StringAlign) string {
	// Calculate the padding needed

	padding := width - utf8.RuneCountInString(str)
	if padding <= 0 {
		return str
	}

	var leftPadding, rightPadding string
	switch alignment {
	case StringAlignLeft:
		leftPadding = ""
		rightPadding = strings.Repeat(" ", padding)
	case StringAlignCenter:
		leftPadding = strings.Repeat(" ", padding/2)
		rightPadding = strings.Repeat(" ", padding-padding/2)
	case StringAlignRight:
		leftPadding = strings.Repeat(" ", padding)
		rightPadding = ""
	case StringAlignCenterRight:
		leftPadding = strings.Repeat(" ", padding-padding/2)
		rightPadding = strings.Repeat(" ", padding/2)
	}

	return leftPadding + str + rightPadding
}

// GetCommandOptionsMap returns a map of command options
// subcommand options are stored as "subcommand-option"
func GetCommandOptionsMap(i *discordgo.InteractionCreate) map[string]*discordgo.ApplicationCommandInteractionDataOption {

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
		if opt.Type == discordgo.ApplicationCommandOptionSubCommand {
			for _, subOpt := range opt.Options {
				optionMap[opt.Name+"-"+subOpt.Name] = subOpt
			}
		}
	}
	return optionMap
}

// ===== Ansi Formatting =====

const (
	ansiReset = "\x1b[0;0m"
	ansiRed   = "\x1b[31m"
	ansiGreen = "\x1b[32m"
	ansiBlue  = "\x1b[34m"
)

// ANSI escape codes.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// WrapANSI wraps s with an ANSI color by name ("red", "green", "blue").
// Unknown color returns s unchanged.
func WrapANSI(s, color string) string {
	switch strings.ToLower(color) {
	case "red":
		return ansiRed + s + ansiReset
	case "green":
		return ansiGreen + s + ansiReset
	case "blue":
		return ansiBlue + s + ansiReset
	default:
		return s
	}
}

// VisibleLenANSI returns the rune length of s excluding ANSI escape codes.
func VisibleLenANSI(s string) int {
	return utf8.RuneCountInString(ansiRe.ReplaceAllString(s, ""))
}

// CellANSI pads s to width with spaces and applies color to the content only.
// If right: the content is right-aligned.
func CellANSI(s, color string, width int, right bool) string {
	colored := WrapANSI(s, color)
	pad := max(width-VisibleLenANSI(s), 0)
	space := strings.Repeat(" ", pad)
	if right {
		return space + colored
	}
	return colored + space
}

// ==== Timestamp Formatting ====

type DiscordTimestampFormat string

const (
	TimestampDefault       DiscordTimestampFormat = ""  // Default        <t:1543392060>      November 28, 2018 9:01 AM            || 28 November 2018 09:01
	TimestampShortTime     DiscordTimestampFormat = "t" // Short Time     <t:1543392060:t>    9:01 AM                              || 09:01
	TimestampLongTime      DiscordTimestampFormat = "T" // Long Time      <t:1543392060:T>    9:01:00 AM                           || 09:01:00
	TimestampShortDate     DiscordTimestampFormat = "d" // Short Date     <t:1543392060:d>    11/28/2018                           || 28/11/2018
	TimestampLongDate      DiscordTimestampFormat = "D" // Long Date      <t:1543392060:D>    November 28, 2018                    || 28 November 2018
	TimestampShortDateTime DiscordTimestampFormat = "f" // Short Date/Time<t:1543392060:f>    November 28, 2018 9:01 AM            || 28 November 2018 09:01
	TimestampLongDateTime  DiscordTimestampFormat = "F" // Long Date/Time <t:1543392060:F>    Wednesday, November 28, 2018 9:01 AM || Wednesday, 28 November 2018 09:01
	TimestampRelativeTime  DiscordTimestampFormat = "R" // Relative Time  <t:1543392060:R>    3 years ago                          || 3 years ago
)

// WrapTimestamp builds a Discord timestamp like <t:1234567890:F>.
// Parameters:
//   - ts (int64): The Unix timestamp in seconds.
//   - format (DiscordTimestampFormat): The desired format.
//
// Returns:
//   - (string): The formatted Discord timestamp.
func WrapTimestamp(ts int64, format DiscordTimestampFormat) string {
	if format == TimestampDefault {
		return fmt.Sprintf("<t:%d>", ts)
	}
	return fmt.Sprintf("<t:%d:%s>", ts, format)
}

// RefreshMap creates and returns a shallow copy of the given map
func RefreshMap[K comparable, V any](m map[K]V) map[K]V {
	newMap := make(map[K]V, len(m))
	maps.Copy(newMap, m)
	return newMap
}
