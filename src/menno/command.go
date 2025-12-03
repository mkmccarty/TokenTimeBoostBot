package menno

import (
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const DefaultMinimumDrops = 1

// SlashHuntCommand returns the command for the /hunt command
func SlashHuntCommand(cmd string) *discordgo.ApplicationCommand {
	integerZeroMinValue := float64(0)
	var shipChoices []*discordgo.ApplicationCommandOptionChoice

	for i := 0; i < len(ei.ShipTypeName); i++ {
		shipChoices = append(shipChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  ei.ShipTypeName[int32(i)],
			Value: fmt.Sprintf("%d", i),
		})
	}
	// Create a duration type choice list
	var durationTypeChoices []*discordgo.ApplicationCommandOptionChoice
	// Collect, sort, and build choices in ascending key order
	for i := 0; i < len(ei.DurationTypeName); i++ {
		durationTypeChoices = append(durationTypeChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  ei.DurationTypeName[int32(i)],
			Value: fmt.Sprintf("%d", i),
		})
	}
	commandOne := []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "ship",
			Description: "Select the ship to search",
			Required:    true,
			Choices:     shipChoices,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "duration-type",
			Description: "Select the duration type",
			Required:    true,
			Choices:     durationTypeChoices,
		},
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "artifact",
			Description:  "What artifact or ingredient to hunt, searchable",
			Required:     true,
			Autocomplete: true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "stars",
			Description: "Ship star level, default max",
			MinValue:    &integerZeroMinValue,
			MaxValue:    8,
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "minimum-drops",
			Description: "Select the minimum number of drops (Sticky)",
			MinValue:    &integerZeroMinValue,
			Required:    false,
		},
	}

	commandTwo := []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "artifact",
			Description:  "What artifact or ingredient to hunt, searchable",
			Required:     true,
			Autocomplete: true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "duration-type",
			Description: "Select the duration type (Sticky)",
			Required:    false,
			Choices:     durationTypeChoices,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "minimum-drops",
			Description: "Select the minimum number of drops (Sticky)",
			MinValue:    &integerZeroMinValue,
			Required:    false,
		},
	}

	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Find artifact drop probabilities",
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
				Name:        "ship",
				Description: "Custom single ship hunt of the Menno drop data",
				Options:     commandOne,
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "item",
				Description: "Hunt Menno drop data across multiple ships",
				Options:     commandTwo,
			},
		},
	}
}

// HandleHuntAutoComplete handles the autocomplete for the /hunt command
func HandleHuntAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	searchString := ""

	if opt, ok := optionMap["ship-artifact"]; ok {
		if opt.Focused {
			searchString = opt.StringValue()
		}
	}
	if opt, ok := optionMap["item-artifact"]; ok {
		if opt.Focused {
			searchString = opt.StringValue()
		}
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	if searchString == "" {
		// No search string, start with a list of popular artifacts
		for id, name := range ei.ArtifactTypeNameVirtue {
			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  name,
				Value: fmt.Sprintf("%d", id),
			}
			choices = append(choices, &choice)
			if len(choices) >= 10 {
				break
			}
		}

		sort.Slice(choices, func(i, j int) bool {
			return choices[i].Name < choices[j].Name
		})

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Content: "Artifact",
				Choices: choices,
			}})
		if err != nil {
			fmt.Printf("HandleHuntAutoComplete InteractionRespond error: %v\n", err)
		}
		return
	}

	for id, name := range ei.ArtifactTypeName {
		if strings.Contains(strings.ToLower(name), strings.ToLower(searchString)) ||
			strings.Contains(strings.ToLower(fmt.Sprint(id)), strings.ToLower(searchString)) {

			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  name,
				Value: fmt.Sprintf("%d", id),
			}
			choices = append(choices, &choice)
			if len(choices) >= 10 {
				break
			}
		}
	}

	sort.Slice(choices, func(i, j int) bool {
		return choices[i].Name < choices[j].Name
	})

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Artifacts",
			Choices: choices,
		}})

}

// HandleHuntCommand handles the /hunt command
func HandleHuntCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	var response string
	artifactID := 10000 // No Target
	minimumDrops := 1000
	userID := bottools.GetInteractionUserID(i)

	// Quick reply to buy us some time
	flags := discordgo.MessageFlagsIsComponentsV2

	if _, ok := optionMap["ship"]; ok {
		shipID := int(optionMap["ship-ship"].IntValue())
		shipStars := 8
		durationTypeID := int(optionMap["ship-duration-type"].IntValue())
		if opt, ok := optionMap["ship-artifact"]; ok {
			artifactID, _ = strconv.Atoi(opt.StringValue())
		}
		if opt, ok := optionMap["ship-stars"]; ok {
			shipStars = int(opt.IntValue())
		}
		if opt, ok := optionMap["ship-minimum-drops"]; ok {
			minimumDrops = int(opt.IntValue())
			farmerstate.SetMiscSettingString(userID, "huntMinimumDrops", fmt.Sprintf("%d", minimumDrops))
		} else {
			savedMinDrops := farmerstate.GetMiscSettingString(userID, "huntMinimumDrops")
			if savedMinDrops != "" {
				parsedMinDrops, err := strconv.Atoi(savedMinDrops)
				// GOOD: Check bounds before assigning to minimumDrops
				if err == nil && parsedMinDrops >= 0 && parsedMinDrops <= math.MaxInt32 {
					minimumDrops = parsedMinDrops
				} else {
					minimumDrops = DefaultMinimumDrops
				}
			}
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Processing request...",
				Flags:   flags,
			},
		})

		response = PrintDropData(ei.MissionInfo_Spaceship(shipID), ei.MissionInfo_DurationType(durationTypeID), shipStars, ei.ArtifactSpec_Name(artifactID), int32(minimumDrops))
	}

	if _, ok := optionMap["item"]; ok {
		// This command requires the user to be registered
		eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		if eiID != "" {
			durationTypeID := 0
			if opt, ok := optionMap["item-artifact"]; ok {
				artifactID, _ = strconv.Atoi(opt.StringValue())
			}
			if opt, ok := optionMap["item-duration-type"]; ok {
				durationTypeID = int(opt.IntValue())
				farmerstate.SetMiscSettingString(userID, "huntItemDuration", fmt.Sprintf("%d", durationTypeID))
			} else {
				durationTypeStr := farmerstate.GetMiscSettingString(userID, "huntItemDuration")
				if durationTypeStr != "" {
					durationTypeID, _ = strconv.Atoi(durationTypeStr)
				}
			}
			if opt, ok := optionMap["item-minimum-drops"]; ok {
				minimumDrops = int(opt.IntValue())
				farmerstate.SetMiscSettingString(userID, "huntMinimumDrops", fmt.Sprintf("%d", minimumDrops))
			} else {
				savedMinDrops := farmerstate.GetMiscSettingString(userID, "huntMinimumDrops")
				if savedMinDrops != "" {
					minimumDrops, _ = strconv.Atoi(savedMinDrops)
				}
			}

			eggIncID := ""
			encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
			if err == nil {
				decodedData, err := base64.StdEncoding.DecodeString(eiID)
				if err == nil {
					decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
					if err == nil {
						eggIncID = string(decryptedData)
					}
				}
			}
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Processing request...",
					Flags:   flags,
				},
			})

			backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, userID, true)

			response = PrintUserDropData(backup, ei.MissionInfo_DurationType(durationTypeID), ei.ArtifactSpec_Name(artifactID), int32(minimumDrops))
		} else {
			flags += discordgo.MessageFlagsEphemeral
			response = fmt.Sprintf("You must register your EI ID with the bot to use this command. Use the %s command.", bottools.GetFormattedCommand("register"))

			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: flags,
					Components: []discordgo.MessageComponent{
						discordgo.TextDisplay{
							Content: response,
						},
					},
				},
			})
			return
		}
	}

	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags: flags,
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{
				Content: response,
			},
		}},
	)
	if err != nil {
		fmt.Printf("HandleHuntCommand error: %v\n", err)
	}

}
