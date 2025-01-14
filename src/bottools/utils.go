package bottools

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// FmtDuration formats a time.Duration into a human readable string
func FmtDuration(d time.Duration) string {
	str := ""
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d = h / 24
	h -= d * 24

	if d > 0 {
		str = fmt.Sprintf("%dd%dh%dm", d, h, m)
	} else {
		str = fmt.Sprintf("%dh%dm", h, m)
	}
	return strings.Replace(str, "0h0m", "", -1)
}

// SanitizeStringDuration takes an hms string and returns a sanitized version of it
func SanitizeStringDuration(s string) string {
	// Ignore everything after a comma
	if strings.Contains(s, ",") {
		s = strings.Split(s, ",")[0]
	}

	s = strings.ToLower(s)
	s = strings.Replace(s, "day", "d", -1)
	s = strings.Replace(s, "min", "m", -1)
	s = strings.Replace(s, "hr", "h", -1)
	s = strings.Replace(s, "sec", "s", -1)
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
