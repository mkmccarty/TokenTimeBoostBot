package menno

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// SlashHuntCommand returns the command for the /hunt command
func SlashHuntCommand(cmd string) *discordgo.ApplicationCommand {
	integerZeroMinValue := float64(0)
	var shipChoices []*discordgo.ApplicationCommandOptionChoice

	// Populate shipChoices here if needed
	for shipID, shipName := range ei.ShipTypeName {
		shipChoices = append(shipChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  shipName,
			Value: shipID,
		})
	}
	// Create a duration type choice list
	var durationTypeChoices []*discordgo.ApplicationCommandOptionChoice
	for durationTypeID, durationTypeName := range ei.DurationTypeName {
		durationTypeChoices = append(durationTypeChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  durationTypeName,
			Value: durationTypeID,
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
		for id, name := range ei.ArtifactTypeNameVirtue {
			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  name,
				Value: id,
			}
			choices = append(choices, &choice)
		}

		sort.Slice(choices, func(i, j int) bool {
			return choices[i].Name < choices[j].Name
		})

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Content: "Artifact",
				Choices: choices,
			}})
		return
	}

	for id, name := range ei.ArtifactTypeName {
		if strings.Contains(strings.ToLower(name), strings.ToLower(searchString)) ||
			strings.Contains(strings.ToLower(fmt.Sprint(id)), strings.ToLower(searchString)) {

			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  name,
				Value: id,
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
