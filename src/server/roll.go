package server

import (
	"fmt"
	"math/rand/v2"

	"github.com/bwmarrin/discordgo"
)

var integerOneMinValue float64 = 1.0

func d20Emoji(roll int64) string {
	cold := []string{"🥶", "❄️", "🧊", "⛄", "🌨️"}
	fire := []string{"🔥", "♨️", "🌋", "☀️", "🌡️"}
	switch {
	case roll >= 18:
		return " " + cold[rand.IntN(len(cold))]
	case roll >= 10 && roll <= 11:
		return " 😐 MID"
	case roll <= 5:
		return " " + fire[rand.IntN(len(fire))]
	}
	return ""
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

	msg := fmt.Sprintf("🎲 Rolling %dd%d: `%v`", count, sides, results)
	if i.ApplicationCommandData().Name == "chill" && sides == 20 && count == 1 {
		msg += d20Emoji(results[0])
	}
	if count > 1 {
		avg := float64(total) / float64(count)
		msg += fmt.Sprintf("\nTotal: **%d** | Avg: **%.1f** | Min: **%d** | Max: **%d**", total, avg, minRoll, maxRoll)
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
		},
	})
}
