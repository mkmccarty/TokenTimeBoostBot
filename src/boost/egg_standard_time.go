package boost

import (
	"fmt"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashEggStandardTimeCommand returns the slash command definition for /egg-standard-time.
func GetSlashEggStandardTimeCommand(cmd string) *dc.Command {
	command := anywhereCommand(cmd, "Show the current Egg Standard Time and eggtime offset.")
	command.Options = []dc.Option{
		dc.BoolOption{
			Name:        "private-reply",
			Description: "Respond privately. Default is false.",
			Required:    false,
		},
	}
	return &command
}

// FormatEggTimeOffset returns a formatted string representation of the offset duration from Egg Standard Time.
func FormatEggTimeOffset(diff time.Duration) string {
	hours := diff.Hours()
	sign := "+"
	if hours < 0 {
		sign = "-"
		hours = -hours
	}

	dec := fmt.Sprintf("%.2f", hours)
	dec = strings.TrimRight(dec, "0")
	dec = strings.TrimRight(dec, ".")
	if dec == "" || dec == "0" {
		dec = "0"
		sign = "+"
	}

	d := diff
	if d < 0 {
		d = -d
	}
	totalMinutes := int(d.Round(time.Minute).Minutes())
	h := totalMinutes / 60
	m := totalMinutes % 60

	var durStr string
	if h > 0 && m > 0 {
		durStr = fmt.Sprintf("%dh %dm", h, m)
	} else if h > 0 {
		durStr = fmt.Sprintf("%dh", h)
	} else {
		durStr = fmt.Sprintf("%dm", m)
	}

	return fmt.Sprintf("%s%s (%s%s)", sign, dec, sign, durStr)
}

// BuildEggStandardTimeString returns the message content for the /egg-standard-time command.
func BuildEggStandardTimeString(now time.Time) string {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.FixedZone("PST", -8*60*60)
	}
	nowInLoc := now.In(loc)
	est := GetEggStandardTime(nowInLoc)
	offsetStr := FormatEggTimeOffset(nowInLoc.Sub(est))

	return fmt.Sprintf(
		"🥚 **Egg Standard Time:** %s (%s)\n🕒 **Kev's Time:** %s\n⏱️ **Current Time:** %s\n⏱️ **Current Offset Time:** `%s`",
		est.Format("3:04 PM MST"),
		bottools.WrapTimestamp(est.Unix(), bottools.TimestampShortTime),
		nowInLoc.Format("3:04 PM MST"),
		bottools.WrapTimestamp(now.Unix(), bottools.TimestampRelativeTime),
		offsetStr,
	)
}

// HandleEggStandardTimeCommand handles the /egg-standard-time slash command.
func HandleEggStandardTimeCommand(e *dc.CommandEvent) {
	privateReply := false
	if opt, ok := e.OptBool("private-reply"); ok {
		privateReply = opt
	}

	content := BuildEggStandardTimeString(time.Now())

	_ = e.Respond(dc.Message{
		Content:   content,
		Ephemeral: privateReply,
	})
}
