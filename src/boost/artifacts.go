package boost

import (
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

func getArtifactsComponents(userID string, channelID string, contractOnly bool) (string, []discordgo.MessageComponent) {
	minValues := 0
	minV := 0

	// is this channelID a thread
	as := getUserArtifacts(userID, nil)

	var builder strings.Builder
	if !contractOnly {
		fmt.Fprintf(&builder, "Select your global coop artifacts <@%s>\nELR: %1.3f", userID, as.LayRate)
	} else {
		fmt.Fprintf(&builder, "Adjust your coop artifact overrides for this contract <@%s>\n ELR: %2.3f  SR:%2.3f", userID, as.LayRate, as.ShipRate)
	}

	// These are the global settings
	deflector := ""
	metronome := ""
	compass := ""
	gusset := ""
	coll := ""

	temp := "PERM"
	if contractOnly {
		temp = "TEMP"
		contract := FindContract(channelID)
		if contract != nil {
			if userInContract(contract, userID) {
				for a := range contract.Boosters[userID].ArtifactSet.Artifacts {
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Deflector") {
						deflector = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Metronome") {
						metronome = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Compass") {
						compass = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Gusset") {
						gusset = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
				}
			}
		} else {
			return "No contract exists in this channel", nil
		}
	} else {
		deflector = farmerstate.GetMiscSettingString(userID, "defl")
		metronome = farmerstate.GetMiscSettingString(userID, "metr")
		compass = farmerstate.GetMiscSettingString(userID, "comp")
		gusset = farmerstate.GetMiscSettingString(userID, "guss")
		coll = farmerstate.GetMiscSettingString(userID, "collegg")

		// Need to perform a conversion on what's in coll.
		// CarbonFiber,Chocolate,Easter,Firework,Pumpkin,Waterballoon,Lithium
		coll = strings.ToUpper(coll)
		coll = strings.ReplaceAll(coll, "CARBONFIBER", "CARBON FIBER")
		coll = strings.ReplaceAll(coll, "FLAMERETARDANT", "FLAME RETARDANT")
	}
	// Remove the extra closing brace

	component := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#DEFL#" + userID + "#" + temp,
					Placeholder: "Select your Deflector...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Deflector T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     deflector == "T4L",
							Emoji:       ei.GetBotComponentEmoji("defl_T4L")},
						{
							Label:       "Deflector T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     deflector == "T4E",
							Emoji:       ei.GetBotComponentEmoji("defl_T4E"),
						},
						{
							Label:       "Deflector T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     deflector == "T4R",
							Emoji:       ei.GetBotComponentEmoji("defl_T4R"),
						},
						{
							Label:       "Deflector T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     deflector == "T4C",
							Emoji:       ei.GetBotComponentEmoji("defl_T4C"),
						},
						{
							Label:       "Deflector T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     deflector == "T3R",
							Emoji:       ei.GetBotComponentEmoji("defl_T3R"),
						},
						{
							Label:       "Deflector T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     deflector == "T3C",
							Emoji:       ei.GetBotComponentEmoji("defl_T3C"),
						},
						{
							Label:       "None",
							Description: "No Deflector equipped",
							Value:       "NONE",
							Default:     deflector == "NONE" || deflector == "",
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#METR#" + userID + "#" + temp,
					Placeholder: "Select your Metronome...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Metronome T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     metronome == "T4L",
							Emoji:       ei.GetBotComponentEmoji("metr_T4L"),
						},
						{
							Label:       "Metronome T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     metronome == "T4E",
							Emoji:       ei.GetBotComponentEmoji("metr_T4E"),
						},
						{
							Label:       "Metronome T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     metronome == "T4R",
							Emoji:       ei.GetBotComponentEmoji("metr_T4R"),
						},
						{
							Label:       "Metronome T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     metronome == "T4C",
							Emoji:       ei.GetBotComponentEmoji("metr_T4C"),
						},
						{
							Label:       "Metronome T3E",
							Description: "Epic",
							Value:       "T3E",
							Default:     metronome == "T3E",
							Emoji:       ei.GetBotComponentEmoji("metr_T3E"),
						},
						{
							Label:       "Metronome T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     metronome == "T3R",
							Emoji:       ei.GetBotComponentEmoji("metr_T3R"),
						},
						{
							Label:       "Metronome T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     metronome == "T3C",
							Emoji:       ei.GetBotComponentEmoji("metr_T3C"),
						},
						{
							Label:       "None",
							Description: "No Metronome equipped",
							Value:       "NONE",
							Default:     metronome == "NONE" || metronome == "",
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#COMP#" + userID + "#" + temp,
					Placeholder: "Select your Compass...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Compass T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     compass == "T4L",
							Emoji:       ei.GetBotComponentEmoji("comp_T4L"),
						},
						{
							Label:       "Compass T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     compass == "T4E",
							Emoji:       ei.GetBotComponentEmoji("comp_T4E"),
						},
						{
							Label:       "Compass T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     compass == "T4R",
							Emoji:       ei.GetBotComponentEmoji("comp_T4R"),
						},
						{
							Label:       "Compass T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     compass == "T4C",
							Emoji:       ei.GetBotComponentEmoji("comp_T4C"),
						},
						{
							Label:       "Compass T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     compass == "T3R",
							Emoji:       ei.GetBotComponentEmoji("comp_T3R"),
						},
						{
							Label:       "Compass T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     compass == "T3C",
							Emoji:       ei.GetBotComponentEmoji("comp_T3C"),
						},
						{
							Label:       "None",
							Description: "No Compass equipped",
							Value:       "NONE",
							Default:     compass == "NONE" || compass == "",
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#GUSS#" + userID + "#" + temp,
					Placeholder: "Select your Gusset...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Gusset T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     gusset == "T4L",
							Emoji:       ei.GetBotComponentEmoji("gusset_T4L"),
						},
						{
							Label:       "Gusset T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     gusset == "T4E",
							Emoji:       ei.GetBotComponentEmoji("gusset_T4E"),
						},
						{
							Label:       "Gusset T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     gusset == "T4C",
							Emoji:       ei.GetBotComponentEmoji("gusset_T4C"),
						},
						{
							Label:       "Gusset T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     gusset == "T3R",
							Emoji:       ei.GetBotComponentEmoji("gusset_T3R"),
						},
						{
							Label:       "Gusset T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     gusset == "T3C",
							Emoji:       ei.GetBotComponentEmoji("gusset_T3C"),
						},
						{
							Label:       "Gusset T2E",
							Description: "Epic",
							Value:       "T2E",
							Default:     gusset == "T2E",
							Emoji:       ei.GetBotComponentEmoji("gusset_T2E"),
						},
						{
							Label:       "None",
							Description: "No Gusset equipped",
							Value:       "NONE",
							Default:     gusset == "NONE" || gusset == "",
						},
					},
				},
			},
		},
	}

	if !contractOnly {
		keys := make([]string, 0, len(ei.CustomEggMap))
		for k := range ei.CustomEggMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Dynamically build the list of custom eggs

		var eggOptions []discordgo.SelectMenuOption
		for _, k := range keys {
			eggOptions = append(eggOptions, discordgo.SelectMenuOption{
				Label:       ei.CustomEggMap[k].Name,
				Description: ei.CustomEggMap[k].Description,
				Value:       strings.ReplaceAll(ei.CustomEggMap[k].Name, " ", ""),
				Default:     strings.Contains(coll, ei.CustomEggMap[k].Name),
				Emoji:       ei.GetBotComponentEmoji("egg_" + ei.CustomEggMap[k].ID),
			})
		}

		component = append(component, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#COLLEGG#" + userID + "#" + temp,
					Placeholder: "Select your Colleggtibles",
					MinValues:   &minV,
					MaxValues:   len(ei.CustomEggMap),
					Options:     eggOptions,
				},
			},
		})
	}

	return builder.String(), component
}

// SlashArtifactCommand creates the new /artifact slash command with subcommands
func SlashArtifactCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Artifact analysis and explorer.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "dialog",
				Description: "Indicate best contract artifacts you have.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "production",
				Description: "Analyze artifacts and stones for production boost.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "category",
						Description: "Select farm category to analyze",
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{
								Name:  "Home/Contract",
								Value: 0,
							},
							{
								Name:  "Virtue",
								Value: 1,
							},
						},
						Required: true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "earnings",
				Description: "Analyze artifacts and stones for earnings boost.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "category",
						Description: "Select farm category to analyze",
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{
								Name:  "Home/Contract",
								Value: 0,
							},
							{
								Name:  "Virtue",
								Value: 1,
							},
						},
						Required: true,
					},
				},
			},
		},
	}
}

// SlashArtifactsCommand creates a new slash command for setting Egg, Inc name
func SlashArtifactsCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Indicate best contract artifacts you have.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
	}
}

func getInteractionUserID(i *discordgo.InteractionCreate) string {
	if i.GuildID == "" {
		return i.User.ID
	}
	return i.Member.User.ID
}

// HandleArtifactsCommand handles the /artifacts command (legacy dialog-only command)
func HandleArtifactsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {

	userID := getInteractionUserID(i)

	contractOnly := false

	str, comp := getArtifactsComponents(userID, i.ChannelID, contractOnly)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Components: comp,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	},
	)
	if err != nil {
		log.Println("InteractionRespond: ", err)
	}

}

// HandleArtifactReactions handles all the button reactions for a contract settings
func HandleArtifactReactions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// cs_#Name # cs_#ID # HASH
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")
	cmd := strings.ToLower(reaction[1])
	userID := reaction[len(reaction)-2]
	//override := reaction[len(reaction)-1]

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	data := i.MessageComponentData()

	setValue := len(data.Values) != 0

	//if override == "PERM" {
	switch cmd {
	case "defl", "metr", "comp", "guss":
		if setValue {
			farmerstate.SetMiscSettingString(userID, cmd, data.Values[0])
		} else {
			farmerstate.SetMiscSettingString(userID, cmd, "") // Clear the value
		}
	case "collegg":
		farmerstate.SetMiscSettingString(userID, cmd, strings.Join(data.Values, ","))
	}

	// Redraw the artifact list
	str, comp := getArtifactsComponents(userID, i.ChannelID, false)

	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    &str,
		Components: &comp,
	})
	if err != nil {
		log.Println("InteractionResponseEdit: ", err)
	}

	//} else {
	contract := FindContract(i.ChannelID)
	if contract != nil {
		if userInContract(contract, userID) {
			// User in this contract
			currentSet := contract.Boosters[userID].ArtifactSet

			var prefix string
			switch cmd {
			case "defl":
				prefix = "D-"
			case "metr":
				prefix = "M-"
			case "comp":
				prefix = "C-"
			case "guss":
				prefix = "G-"
			}
			var newArtifact *ei.Artifact
			if len(data.Values) == 0 {
				newArtifact = ei.ArtifactMap[prefix+"NONE"]
			} else {
				newArtifact = ei.ArtifactMap[prefix+data.Values[0]]
			}

			// Check if artifact was found in map
			if newArtifact != nil {
				// Check if the artifact already exists in the current set
				exists := false
				for i, artifact := range currentSet.Artifacts {
					if artifact.Type == newArtifact.Type {
						exists = true
						if setValue {
							currentSet.Artifacts[i] = *newArtifact
						} else {
							// Removing this artifact
							currentSet.Artifacts = append(currentSet.Artifacts[:i], currentSet.Artifacts[i+1:]...)
						}
						break
					}
				}
				// If the artifact doesn't exist, add it to the current set
				if !exists {
					currentSet.Artifacts = append(currentSet.Artifacts, *newArtifact)
				}

				contract.Boosters[userID].ArtifactSet = getUserArtifacts(userID, &currentSet)

				refreshBoostListMessage(s, contract, false)
			}

		}
	}
	//}
	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			//Content: "",
			//Flags: discordgo.MessageFlagsEphemeral,
		})

}

// HandleArtifactCommand handles the /artifact command with subcommands
func HandleArtifactCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	subcommand := i.ApplicationCommandData().Options[0].Name

	switch subcommand {
	case "dialog":
		handleArtifactDialog(s, i)
	case "production":
		handleArtifactProduction(s, i, optionMap)
	case "earnings":
		handleArtifactEarnings(s, i, optionMap)
	}
}

// handleArtifactDialog handles the /artifact dialog subcommand (existing functionality)
func handleArtifactDialog(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)

	contractOnly := false

	str, comp := getArtifactsComponents(userID, i.ChannelID, contractOnly)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Components: comp,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	},
	)
	if err != nil {
		log.Println("InteractionRespond: ", err)
	}
}

// handleArtifactProduction handles the /artifact production subcommand
func handleArtifactProduction(s *discordgo.Session, i *discordgo.InteractionCreate, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	userID := bottools.GetInteractionUserID(i)

	// Get category from option
	var category int64 = 0
	if opt, ok := optionMap["category"]; ok {
		category = opt.IntValue()
	}

	// Quick reply
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Analyzing artifacts for production...",
		},
	})

	// Get the Egg Inc ID from the stored settings
	eggIncID := ""
	encryptedID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	if encryptedID != "" {
		encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
		if err == nil {
			decodedData, err := base64.StdEncoding.DecodeString(encryptedID)
			if err == nil {
				decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
				if err == nil {
					eggIncID = string(decryptedData)
				}
			}
		}
	}
	if eggIncID == "" || len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Error: Your Egg Inc ID is not configured. Please use `/virtue` command first to set up your ID.",
		})
		return
	}

	backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, userID, true)
	if backup == nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Failed to retrieve player data from Egg Inc API",
		})
		return
	}

	// Parse and analyze artifacts
	result := analyzeArtifactsForProduction(backup, category == 1)

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content:    result,
		Components: []discordgo.MessageComponent{},
	})
}

// handleArtifactEarnings handles the /artifact earnings subcommand
func handleArtifactEarnings(s *discordgo.Session, i *discordgo.InteractionCreate, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	userID := bottools.GetInteractionUserID(i)

	// Get category from option
	var category int64 = 0
	if opt, ok := optionMap["category"]; ok {
		category = opt.IntValue()
	}

	// Quick reply
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Analyzing artifacts for earnings...",
		},
	})

	// Get the Egg Inc ID from the stored settings
	eggIncID := ""
	encryptedID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	if encryptedID != "" {
		encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
		if err == nil {
			decodedData, err := base64.StdEncoding.DecodeString(encryptedID)
			if err == nil {
				decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
				if err == nil {
					eggIncID = string(decryptedData)
				}
			}
		}
	}
	if eggIncID == "" || len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Error: Your Egg Inc ID is not configured. Please use `/virtue` command first to set up your ID.",
		})
		return
	}

	backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, userID, true)
	if backup == nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Failed to retrieve player data from Egg Inc API",
		})
		return
	}

	// Parse and analyze artifacts
	result := analyzeArtifactsForEarnings(backup, category == 1)

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content:    result,
		Components: []discordgo.MessageComponent{},
	})
}

// analyzeArtifactsForProduction analyzes artifacts and stones for production boost
func analyzeArtifactsForProduction(backup *ei.Backup, isVirtue bool) string {
	var result strings.Builder

	if isVirtue {
		// Analyze virtue farm artifacts
		virtue := backup.GetVirtue()
		if virtue == nil {
			result.WriteString("No virtue data found in backup")
			return result.String()
		}

		virtueArtifactDB := backup.ArtifactsDb.GetVirtueAfxDb()
		virtueArtifacts := virtueArtifactDB.GetInventoryItems()
		activeAfx := virtueArtifactDB.GetActiveArtifacts()

		result.WriteString("# Production Analysis - Virtue Farm\n\n")
		result.WriteString("**Active Artifacts:**\n")

		virtueTileSet := activeAfx.GetSlots()
		for _, slot := range virtueTileSet {
			if slot.GetOccupied() {
				itemID := slot.GetItemId()
				for _, artifact := range virtueArtifacts {
					if artifact.GetItemId() == itemID {
						spec := artifact.GetArtifact().GetSpec()
						name := ei.ShortArtifactName[int32(spec.GetName())]
						level := ei.ArtifactLevels[spec.GetLevel()]
						rarity := ei.ArtifactRarity[spec.GetRarity()]
						result.WriteString(fmt.Sprintf("- %s %s%s\n", name, level, rarity))
						break
					}
				}
			}
		}

		// List all artifact inventory
		result.WriteString("\n**Full Inventory:**\n")
		for _, artifact := range virtueArtifacts {
			spec := artifact.GetArtifact().GetSpec()
			name := ei.ShortArtifactName[int32(spec.GetName())]
			level := ei.ArtifactLevels[spec.GetLevel()]
			rarity := ei.ArtifactRarity[spec.GetRarity()]
			count := artifact.GetQuantity()
			result.WriteString(fmt.Sprintf("- %s %s%s (qty: %f)\n", name, level, rarity, count))
		}

	} else {
		// For home/contract farms, we'll show a simplified view
		result.WriteString("# Production Analysis - Home/Contract Farm\n\n")
		result.WriteString("Home/Contract farm artifact analysis is not yet available.\n")
		result.WriteString("Please use the `/artifact dialog` command to configure your artifacts.\n")
	}

	return result.String()
}

// analyzeArtifactsForEarnings analyzes artifacts and stones for earnings boost
func analyzeArtifactsForEarnings(backup *ei.Backup, isVirtue bool) string {
	var result strings.Builder
	var inventoryItems []*ei.ArtifactInventoryItem

	if isVirtue {
		inventoryItems = backup.ArtifactsDb.GetInventoryItems()
	} else {
		virtueArtifactDB := backup.ArtifactsDb.GetVirtueAfxDb()
		inventoryItems = virtueArtifactDB.GetInventoryItems()

	}
	if len(inventoryItems) == 0 {
		result.WriteString("No artifacts found in inventory.")
		return result.String()
	}

	result.WriteString("# Earnings Analysis - Virtue Farm\n\n")
	result.WriteString("**Active Artifacts (Earnings Boost):**\n")

	artifactSetInUse := []*ei.CompleteArtifact{}
	inUseArtifactIDs := []uint64{}

	// Build map for quick lookup
	itemsByID := make(map[uint64]*ei.CompleteArtifact, len(inventoryItems))
	for _, it := range inventoryItems {
		itemsByID[it.GetItemId()] = it.GetArtifact()
	}

	for _, id := range inUseArtifactIDs {
		if a := itemsByID[id]; a != nil {
			artifactSetInUse = append(artifactSetInUse, a)
			spec := a.GetSpec()
			name := ei.ShortArtifactName[int32(spec.GetName())]
			level := ei.ArtifactLevels[spec.GetLevel()]
			rarity := ei.ArtifactRarity[spec.GetRarity()]
			result.WriteString(fmt.Sprintf("- %s %s%s\n", name, level, rarity))
		}
	}

	// Get earnings buffs from artifacts
	artifactBuffs := ei.GetArtifactBuffs(artifactSetInUse)
	result.WriteString(fmt.Sprintf("\n**Earnings Rate Multiplier**: %.3f\n", artifactBuffs.Earnings))

	result.WriteString("\n**Full Inventory:**\n")
	for _, artifact := range inventoryItems {
		spec := artifact.GetArtifact().GetSpec()
		name := ei.ShortArtifactName[int32(spec.GetName())]
		level := ei.ArtifactLevels[spec.GetLevel()]
		rarity := ei.ArtifactRarity[spec.GetRarity()]
		count := artifact.GetQuantity()
		result.WriteString(fmt.Sprintf("- %s %s%s (qty: %f)\n", name, level, rarity, count))
	}

	return result.String()
}
