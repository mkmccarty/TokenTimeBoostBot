package bottools

import (
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
	"github.com/mattn/go-runewidth"
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
	return formatString(str, width, alignment, false)
}

// FitString truncates and pads left or right
func FitString(str string, width int, alignment StringAlign) string {
	switch alignment {
	case StringAlignLeft, StringAlignRight:
		return formatString(str, width, alignment, true)
	default:
		return formatString(str, width, StringAlignLeft, true)
	}
}

// formatString formats a string to a given width with alignment and optional truncation
func formatString(str string, width int, alignment StringAlign, truncate bool) string {
	if truncate {
		str = runewidth.Truncate(str, width, "")
	}

	w := runewidth.StringWidth(str)
	if w >= width {
		return str
	}

	// Calculate the padding needed
	pad := width - w
	var left int
	switch alignment {
	case StringAlignLeft:
		left = 0
	case StringAlignCenter:
		left = pad / 2
	case StringAlignRight:
		left = pad
	case StringAlignCenterRight:
		left = pad - pad/2
	}

	return strings.Repeat(" ", left) + str +
		strings.Repeat(" ", width-left-w)
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

// TimestampNumber is a set of integer types that can be used as Unix timestamps.
type TimestampNumber interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// DiscordTimestampFormat holds the style postfixes for Discord timestamp formatting.
type DiscordTimestampFormat string

const (
	// Default        <t:1543392060>      November 28, 2018 9:01 AM            || 28 November 2018 09:01
	TimestampDefault DiscordTimestampFormat = ""
	// Short Time     <t:1543392060:t>    9:01 AM                              || 09:01
	TimestampShortTime DiscordTimestampFormat = "t"
	// Long Time      <t:1543392060:T>    9:01:00 AM                           || 09:01:00
	TimestampLongTime DiscordTimestampFormat = "T"
	// Short Date     <t:1543392060:d>    11/28/2018                           || 28/11/2018
	TimestampShortDate DiscordTimestampFormat = "d"
	// Long Date      <t:1543392060:D>    November 28, 2018                    || 28 November 2018
	TimestampLongDate DiscordTimestampFormat = "D"
	// Short Date/Time<t:1543392060:f>    November 28, 2018 9:01 AM            || 28 November 2018 09:01
	TimestampShortDateTime DiscordTimestampFormat = "f"
	// Long Date/Time <t:1543392060:F>    Wednesday, November 28, 2018 9:01 AM || Wednesday, 28 November 2018 09:01
	TimestampLongDateTime DiscordTimestampFormat = "F"
	// Relative Time  <t:1543392060:R>    3 years ago                          || 3 years ago
	TimestampRelativeTime DiscordTimestampFormat = "R"
)

// WrapTimestamp returns ts formatted as a Discord timestamp string.
//
// ts can be any integer type assumed to be a Unix timestamp in seconds.
// format is one of the DiscordTimestampFormat constants.
// If format is TimestampDefault, it returns "<t:ts>". Otherwise it returns "<t:ts:format>".
func WrapTimestamp[N TimestampNumber](ts N, format DiscordTimestampFormat) string {
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
