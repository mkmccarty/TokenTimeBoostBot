package menno

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

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
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "ship",
				Description: "Select the ship to search",
				Required:    true,
				Choices:     shipChoices,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "stars",
				Description: "Ship star level",
				MinValue:    &integerZeroMinValue,
				MaxValue:    8,
				Required:    true,
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
				Description:  "What artifact or ingredient to hunt",
				Required:     false,
				Autocomplete: true,
			},
		},
	}
}

// HandleHuntAutoComplete handles the autocomplete for the /hunt command
func HandleHuntAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	searchString := ""

	if opt, ok := optionMap["artifact"]; ok {
		searchString = opt.StringValue()
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

	shipID := int(optionMap["ship"].IntValue())
	shipStars := int(optionMap["stars"].IntValue())
	durationTypeID := int(optionMap["duration-type"].IntValue())
	artifactID := 10000 // No Target
	if opt, ok := optionMap["artifact"]; ok {
		artifactID, _ = strconv.Atoi(opt.StringValue())
	}

	response := PrintDropData(ei.MissionInfo_Spaceship(shipID), ei.MissionInfo_DurationType(durationTypeID), shipStars, ei.ArtifactSpec_Name(artifactID))

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsIsComponentsV2,
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: response,
				},
			}},
	})
}
