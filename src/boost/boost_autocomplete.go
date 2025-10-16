package boost

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"github.com/bwmarrin/discordgo"
)

// HandleContractAutoComplete will handle the contract auto complete of contract-id's
func HandleContractAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)
	for _, c := range ei.EggIncContracts {
		ultra := ""
		if c.Ultra {
			ultra = " -ultra"
		}
		choice := discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("%s (%s)%s", c.Name, c.ID, ultra),
			Value: c.ID,
		}
		choices = append(choices, &choice)
	}

	sort.Slice(choices, func(i, j int) bool {
		return choices[i].Name < choices[j].Name
	})

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Contract ID",
			Choices: choices,
		}})
}

// HandleAllContractsAutoComplete will handle the contract auto complete of contract-id's
// default to new contracts but allow searching all contracts
func HandleAllContractsAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {

	optionMap := bottools.GetCommandOptionsMap(i)

	searchString := ""

	if opt, ok := optionMap["contract-id"]; ok {
		searchString = opt.StringValue()
	}
	if opt, ok := optionMap["active-contract-id"]; ok {
		searchString = opt.StringValue()
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	if searchString == "" {
		for _, c := range ei.EggIncContracts {
			ultra := ""
			if c.Ultra {
				ultra = " -ultra"
			}
			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  fmt.Sprintf("%s (%s)%s", c.Name, c.ID, ultra),
				Value: c.ID,
			}
			choices = append(choices, &choice)
		}

		sort.Slice(choices, func(i, j int) bool {
			return choices[i].Name < choices[j].Name
		})

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Content: "Contract ID",
				Choices: choices,
			}})
		return
	}

	for _, c := range ei.EggIncContractsAll {
		if strings.Contains(strings.ToLower(c.ID), strings.ToLower(searchString)) ||
			strings.Contains(strings.ToLower(c.Name), strings.ToLower(searchString)) {

			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  fmt.Sprintf("%s (%s)", c.Name, c.ID),
				Value: c.ID,
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
			Content: "Contract ID",
			Choices: choices,
		}})

}
