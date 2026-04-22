package boost

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetSlashUpdateCommand returns the /update slash command with main subcommand groups for farmer and contract
func GetSlashUpdateCommand(cmd string) *discordgo.ApplicationCommand {
	intMin0 := float64(0)
	intMax12 := float64(12)
	intMax490 := float64(490)

	return &discordgo.ApplicationCommand{
		Name: cmd,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Description: "Update farmer statistics",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Name:        "farmer",
				Description: "Update farmer statistics",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "boost-tokens",
						Description: "Update boost tokens (0-12)",
						Options: []*discordgo.ApplicationCommandOption{
							{
								Type:        discordgo.ApplicationCommandOptionString,
								Name:        "farmername",
								Description: "Farmer name to update",
								Required:    true,
							},
							{
								Type:        discordgo.ApplicationCommandOptionInteger,
								Name:        "value",
								Description: "Number of boost tokens (0-12)",
								Required:    true,
								MinValue:    &intMin0,
								MaxValue:    intMax12,
							},
						},
					},
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "te",
						Description: "Update TE value (0-490)",
						Options: []*discordgo.ApplicationCommandOption{
							{
								Type:        discordgo.ApplicationCommandOptionString,
								Name:        "farmername",
								Description: "Farmer name to update",
								Required:    true,
							},
							{
								Type:        discordgo.ApplicationCommandOptionInteger,
								Name:        "value",
								Description: "TE value (0-490)",
								Required:    true,
								MinValue:    &intMin0,
								MaxValue:    intMax490,
							},
						},
					},
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "artifacts",
						Description: "Update farmer artifact settings (defl, metr, comp, guss)",
						Options: []*discordgo.ApplicationCommandOption{
							{
								Type:        discordgo.ApplicationCommandOptionString,
								Name:        "farmername",
								Description: "Farmer name to update",
								Required:    true,
							},
							{
								Type:        discordgo.ApplicationCommandOptionString,
								Name:        "deflector",
								Description: "Deflector tier",
								Required:    false,
								Choices: []*discordgo.ApplicationCommandOptionChoice{
									{Name: "Deflector T4L (Legendary)", Value: "T4L"},
									{Name: "Deflector T4E (Epic)", Value: "T4E"},
									{Name: "Deflector T4R (Rare)", Value: "T4R"},
									{Name: "Deflector T4C (Common)", Value: "T4C"},
									{Name: "Deflector T3R (Rare)", Value: "T3R"},
									{Name: "Deflector T3C (Common)", Value: "T3C"},
									{Name: "None", Value: "NONE"},
								},
							},
							{
								Type:        discordgo.ApplicationCommandOptionString,
								Name:        "metronome",
								Description: "Metronome tier",
								Required:    false,
								Choices: []*discordgo.ApplicationCommandOptionChoice{
									{Name: "Metronome T4L (Legendary)", Value: "T4L"},
									{Name: "Metronome T4E (Epic)", Value: "T4E"},
									{Name: "Metronome T4R (Rare)", Value: "T4R"},
									{Name: "Metronome T4C (Common)", Value: "T4C"},
									{Name: "Metronome T3E (Epic)", Value: "T3E"},
									{Name: "Metronome T3R (Rare)", Value: "T3R"},
									{Name: "Metronome T3C (Common)", Value: "T3C"},
									{Name: "None", Value: "NONE"},
								},
							},
							{
								Type:        discordgo.ApplicationCommandOptionString,
								Name:        "compass",
								Description: "Compass tier",
								Required:    false,
								Choices: []*discordgo.ApplicationCommandOptionChoice{
									{Name: "Compass T4L (Legendary)", Value: "T4L"},
									{Name: "Compass T4E (Epic)", Value: "T4E"},
									{Name: "Compass T4R (Rare)", Value: "T4R"},
									{Name: "Compass T4C (Common)", Value: "T4C"},
									{Name: "Compass T3R (Rare)", Value: "T3R"},
									{Name: "Compass T3C (Common)", Value: "T3C"},
									{Name: "None", Value: "NONE"},
								},
							},
							{
								Type:        discordgo.ApplicationCommandOptionString,
								Name:        "gusset",
								Description: "Gusset tier",
								Required:    false,
								Choices: []*discordgo.ApplicationCommandOptionChoice{
									{Name: "Gusset T4L (Legendary)", Value: "T4L"},
									{Name: "Gusset T4E (Epic)", Value: "T4E"},
									{Name: "Gusset T4C (Common)", Value: "T4C"},
									{Name: "Gusset T3R (Rare)", Value: "T3R"},
									{Name: "Gusset T3C (Common)", Value: "T3C"},
									{Name: "Gusset T2E (Epic)", Value: "T2E"},
									{Name: "None", Value: "NONE"},
								},
							},
						},
					},
				},
			},
		},
	}
}

// HandleUpdateCommand handles the /update slash command
func HandleUpdateCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	optionMap := bottools.GetCommandOptionsMap(i)
	subcommandGroup := ""
	subcommand := ""
	farmername := ""
	value := int64(0)

	// Get the subcommand group and subcommand from nested options
	if len(i.ApplicationCommandData().Options) > 0 {
		subcommandGroup = i.ApplicationCommandData().Options[0].Name
		if len(i.ApplicationCommandData().Options[0].Options) > 0 {
			subcommand = i.ApplicationCommandData().Options[0].Options[0].Name
		}
	}

	// Extract values based on subcommand group
	switch subcommandGroup {
	case "farmer":
		if opt, ok := optionMap["farmer-boost-tokens-farmername"]; ok {
			farmername = strings.TrimSpace(opt.StringValue())
		}
		if opt, ok := optionMap["farmer-boost-tokens-value"]; ok {
			value = opt.IntValue()
		}
		if opt, ok := optionMap["farmer-te-farmername"]; ok {
			farmername = strings.TrimSpace(opt.StringValue())
		}
		if opt, ok := optionMap["farmer-te-value"]; ok {
			value = opt.IntValue()
		}
		if opt, ok := optionMap["farmer-artifacts-farmername"]; ok {
			farmername = strings.TrimSpace(opt.StringValue())
		}
	}

	userID := farmername
	resultMsg := ""

	// Try to find the user by farmername or discord mention
	if farmername != "" {
		if mentionID, isMention := parseMentionUserID(farmername); isMention {
			userID = mentionID
		}
	}

	// Handle the specific subcommand
	switch subcommandGroup {
	case "farmer":
		switch subcommand {
		case "boost-tokens":
			farmerstate.SetTokens(userID, int(value))
			resultMsg = fmt.Sprintf("✅ Updated %s's boost tokens to %d", farmername, value)

		case "te":
			farmerstate.SetMiscSettingString(userID, "TE", fmt.Sprintf("%d", value))
			resultMsg = fmt.Sprintf("✅ Updated %s's TE to %d", farmername, value)

		case "artifacts":
			var changed []string
			artifactKeys := map[string]string{
				"farmer-artifacts-deflector": "defl",
				"farmer-artifacts-metronome": "metr",
				"farmer-artifacts-compass":   "comp",
				"farmer-artifacts-gusset":    "guss",
			}
			for optKey, settingKey := range artifactKeys {
				if opt, ok := optionMap[optKey]; ok {
					farmerstate.SetMiscSettingString(userID, settingKey, opt.StringValue())
					changed = append(changed, fmt.Sprintf("%s: %s", settingKey, opt.StringValue()))
				}
			}
			if len(changed) > 0 {
				resultMsg = fmt.Sprintf("✅ Updated %s's artifacts: %s", farmername, strings.Join(changed, ", "))
			} else {
				resultMsg = "No artifact values provided"
			}

		default:
			resultMsg = "Unknown farmer subcommand"
		}

		// If this farmer is in any contracts, update their contract.Boosters data with the changed value
		updateFarmerInContracts(s, userID, subcommand, value)

	default:
		resultMsg = "Unknown subcommand group"
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: resultMsg,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

// updateFarmerInContracts updates a farmer's data in all contracts they're part of
func updateFarmerInContracts(s *discordgo.Session, userID string, subcommand string, value int64) {
	for _, contract := range Contracts {
		if booster, exists := contract.Boosters[userID]; exists {
			// Update the specific field based on which subcommand was used
			switch subcommand {
			case "boost-tokens":
				booster.TokensWanted = int(value)
			case "te":
				booster.TECount = int(value)
			}

			// Redraw the boost list message to reflect the updated data
			for _, loc := range contract.Location {
				_ = loc
				refreshBoostListMessage(s, contract, false)
				break // Only need to refresh once
			}
		}
	}
}
