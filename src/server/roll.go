package server

import (
	"fmt"
	"math/rand/v2"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

var (
	dieMinValue   = 1
	dieMaxValue   = 1000
	countMinValue = 1
	countMaxValue = 100

	iceCold = []string{"🥶", "❄️", "🧊", "⛄", "🌨️", "🏔️", "🌬️", "🐧", "🦭"}
	cool    = []string{"😎", "🌊", "🏄", "🌿", "🍃", "🫠", "😴", "💤", "🛋️", "🌙", "🍹", "🧋", "☕", "🤙", "🦥"}
	heated  = []string{"😤", "🤯", "⚡", "🚨", "😱", "🏃", "💨", "👊", "🤬", "😵", "‼️"}
	onFire  = []string{"🔥", "♨️", "🌋", "☀️", "🌡️", "☄️", "🌞", "🏜️", "🕯️", "🧨", "💣", "💥"}
)

func pick(pool []string) string { return pool[rand.IntN(len(pool))] }

const (
	colorPureBlue       = 0x0000FF
	colorCornflowerBlue = 0x6495ED
	colorGray           = 0x9E9E9E
	colorSalmon         = 0xFA8072
	colorPureRed        = 0xFF0000
)

// d20Flair returns the text suffix and accent color for a d20 roll.
func d20Flair(roll int) (suffix string, accent int) {
	switch {
	case roll >= 18:
		return " " + pick(iceCold), colorPureBlue
	case roll >= 15:
		return " " + pick(cool), colorCornflowerBlue
	case roll >= 12:
		return "", colorCornflowerBlue
	case roll >= 10:
		return " 😐 MID", colorGray
	case roll >= 7:
		return "", colorSalmon
	case roll >= 4:
		return " " + pick(heated), colorSalmon
	default:
		return " " + pick(onFire), colorPureRed
	}
}

// GetSlashChillCommand returns the slash command definition for the chill/roll dice command.
func GetSlashChillCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:             cmd,
		Description:      "Roll some dice.",
		Contexts:         []dc.InteractionContext{dc.ContextGuild},
		IntegrationTypes: []dc.IntegrationType{dc.IntegrationGuildInstall},
		Options: []dc.Option{
			dc.IntOption{
				Name:        "die",
				Description: "Number of sides on the die (default: 20).",
				MinValue:    &dieMinValue,
				MaxValue:    &dieMaxValue,
				Required:    false,
			},
			dc.IntOption{
				Name:        "count",
				Description: "Number of dice to roll (default: 1).",
				MinValue:    &countMinValue,
				MaxValue:    &countMaxValue,
				Required:    false,
			},
		},
	}
	return &command
}

// HandleChillCommand handles the chill/roll slash command interaction.
func HandleChillCommand(e *dc.CommandEvent) {
	sides := 20
	count := 1
	if v, ok := e.OptInt("die"); ok {
		sides = v
	}
	if v, ok := e.OptInt("count"); ok {
		count = v
	}

	total := 0
	minRoll, maxRoll := sides+1, 0
	results := make([]int, count)
	for j := range results {
		roll := rand.IntN(sides) + 1
		results[j] = roll
		total += roll
		if roll < minRoll {
			minRoll = roll
		}
		if roll > maxRoll {
			maxRoll = roll
		}
	}

	var msg string
	if count == 1 {
		msg = fmt.Sprintf("🎲 d%d: `%d`", sides, results[0])
	} else {
		msg = fmt.Sprintf("🎲 Rolling %dd%d: `%v`", count, sides, results)
		avg := float64(total) / float64(count)
		msg += fmt.Sprintf("\nTotal: **%d** | Avg: **%.1f** | Min: **%d** | Max: **%d**", total, avg, minRoll, maxRoll)
	}

	if e.CommandName() == "chill" && sides == 20 && count == 1 {
		suffix, accent := d20Flair(results[0])
		_ = e.Respond(dc.Message{
			Components: []dc.LayoutComponent{
				dc.Container{
					AccentColor: accent,
					Components: []dc.ContainerSubComponent{
						dc.TextDisplay{Content: fmt.Sprintf("**Roll: %d**%s", results[0], suffix)},
					},
				},
			},
		})
		return
	}

	_ = e.Respond(dc.Message{Content: msg})
}
