package server

import (
	"fmt"
	"math/rand/v2"

	"github.com/bwmarrin/discordgo"
)

var (
	integerOneMinValue float64 = 1.0

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
// Accent colors follow blackbody radiation: dark red (coldest) → orange → yellow → white (hottest).
func d20Flair(roll int64) (suffix string, accent int) {
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
func GetSlashChillCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name: cmd,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Description: "Roll some dice.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "die",
				Description: "Number of sides on the die (default: 20).",
				MinValue:    &integerOneMinValue,
				MaxValue:    1000,
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "count",
				Description: "Number of dice to roll (default: 1).",
				MinValue:    &integerOneMinValue,
				MaxValue:    100,
				Required:    false,
			},
		},
	}
}

// HandleChillCommand handles the chill/roll slash command interaction.
func HandleChillCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	sides := int64(20)
	count := int64(1)

	for _, opt := range i.ApplicationCommandData().Options {
		switch opt.Name {
		case "die":
			sides = opt.IntValue()
		case "count":
			count = opt.IntValue()
		}
	}

	total := int64(0)
	minRoll, maxRoll := int64(sides+1), int64(0)
	results := make([]int64, count)
	for j := range results {
		roll := rand.Int64N(sides) + 1
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
	}
	if count > 1 {
		avg := float64(total) / float64(count)
		msg += fmt.Sprintf("\nTotal: **%d** | Avg: **%.1f** | Min: **%d** | Max: **%d**", total, avg, minRoll, maxRoll)
	}

	if i.ApplicationCommandData().Name == "chill" && sides == 20 && count == 1 {
		suffix, accent := d20Flair(results[0])
		msg = fmt.Sprintf("**Roll: %d**%s", results[0], suffix)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsIsComponentsV2,
				Components: []discordgo.MessageComponent{
					discordgo.Container{
						AccentColor: &accent,
						Components: []discordgo.MessageComponent{
							discordgo.TextDisplay{Content: msg},
						},
					},
				},
			},
		})
		return
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
		},
	})
}
