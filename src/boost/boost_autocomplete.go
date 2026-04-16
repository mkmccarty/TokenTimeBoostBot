package boost

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"github.com/bwmarrin/discordgo"
)

const maxAutocompleteChoices = 25

func sortContractChoices(choices []*discordgo.ApplicationCommandOptionChoice) {
	sort.Slice(choices, func(i, j int) bool {
		return choices[i].Name < choices[j].Name
	})
}

func limitContractChoices(choices []*discordgo.ApplicationCommandOptionChoice, maxChoices int) []*discordgo.ApplicationCommandOptionChoice {
	if len(choices) <= maxChoices {
		return choices
	}
	return choices[:maxChoices]
}

// HandleContractAutoComplete will handle the contract auto complete of contract-id's
func HandleContractAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	searchString := ""
	if opt, ok := optionMap["contract-id"]; ok {
		searchString = strings.ToLower(opt.StringValue())
	}
	isContractCommand := i.ApplicationCommandData().Name == "contract"

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)
	contracts := make([]ei.EggIncContract, len(ei.EggIncContracts))
	copy(contracts, ei.EggIncContracts)
	sort.SliceStable(contracts, func(i, j int) bool {
		return contracts[i].ValidFrom.After(contracts[j].ValidFrom)
	})

	for _, c := range contracts {
		isPredicted := c.Predicted

		if isContractCommand {
			if searchString == "" && isPredicted {
				continue
			}
			if searchString != "" &&
				!strings.Contains(strings.ToLower(c.ID), searchString) &&
				!strings.Contains(strings.ToLower(c.Name), searchString) {
				continue
			}
		} else if isPredicted {
			continue
		}

		seasonalStr := ""
		if c.SeasonID != "" {
			//seasonYear := strings.Split(c.SeasonID, "_")[1]
			seasonIcon := strings.Split(c.SeasonID, "_")[0]
			seasonEmote := map[string]string{"winter": "❄️", "spring": "🌷", "summer": "☀️", "fall": "🍂"}
			seasonalStr = fmt.Sprintf("%s", seasonEmote[seasonIcon])
		}

		ultra := ""
		if c.Ultra && !c.Predicted {
			ultra = " -ultra"
		}
		choice := discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("%s (%s)%s %s", c.Name, c.ID, ultra, seasonalStr),
			Value: c.ID,
		}
		choices = append(choices, &choice)
	}

	//sortContractChoices(choices)
	choices = limitContractChoices(choices, maxAutocompleteChoices)

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
			if c.Predicted {
				continue
			}
			ultra := ""
			if c.Ultra && !c.Predicted {
				ultra = " -ultra"
			}
			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  fmt.Sprintf("%s (%s)%s", c.Name, c.ID, ultra),
				Value: c.ID,
			}
			choices = append(choices, &choice)
		}

		sortContractChoices(choices)
		choices = limitContractChoices(choices, maxAutocompleteChoices)

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Content: "Contract ID",
				Choices: choices,
			}})
		return
	}

	for _, c := range ei.EggIncContractsAll {
		if c.Predicted {
			continue
		}
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

	sortContractChoices(choices)
	choices = limitContractChoices(choices, 10)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Contract ID",
			Choices: choices,
		}})

}
