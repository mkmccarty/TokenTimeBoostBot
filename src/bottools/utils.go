package bottools

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// FmtDuration formats a time.Duration into a human readable string
func FmtDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	days := h / 24
	h -= days * 24

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
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
	// StringAlignCenter aligns the string to the center
	StringAlignCenter
	// StringAlignRight aligns the string to the right
	StringAlignRight
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
	}

	return leftPadding + str + rightPadding
}
