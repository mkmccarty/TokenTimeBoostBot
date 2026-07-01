package boost

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"sort"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"

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

	if opt, ok := optionMap["coop-id"]; ok && opt.Focused {
		handleCoopIDAutoComplete(s, i, opt.StringValue())
		return
	}
	if opt, ok := optionMap["contract-coop-id-coop-id"]; ok && opt.Focused {
		handleCoopIDAutoComplete(s, i, opt.StringValue())
		return
	}

	searchString := ""
	if opt, ok := optionMap["contract-id"]; ok {
		searchString = strings.ToLower(opt.StringValue())
	}
	if opt, ok := optionMap["contract-contract-id-contract-id"]; ok {
		searchString = strings.ToLower(opt.StringValue())
	}

	isContractCommand := i.ApplicationCommandData().Name == "contract"

	contract := FindContract(i.ChannelID)
	allowPredicted := isContractCommand || (contract != nil && contract.State == ContractStateSignup && contract.PredictionSignup)

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)
	contracts := make([]ei.EggIncContract, len(ei.EggIncContracts))
	copy(contracts, ei.EggIncContracts)
	sort.SliceStable(contracts, func(i, j int) bool {
		return contracts[i].ValidFrom.After(contracts[j].ValidFrom)
	})

	for _, c := range contracts {
		isPredicted := c.Predicted

		if isPredicted && (!allowPredicted || searchString == "") {
			continue
		}

		if searchString != "" &&
			!strings.Contains(strings.ToLower(c.ID), searchString) &&
			!strings.Contains(strings.ToLower(c.Name), searchString) {
			continue
		}

		seasonalStr := ""
		if c.SeasonID != "" {
			//seasonYear := strings.Split(c.SeasonID, "_")[1]
			seasonIcon := strings.Split(c.SeasonID, "_")[0]
			seasonEmote := map[string]string{"winter": "❄️", "spring": "🌷", "summer": "☀️", "fall": "🍂"}
			seasonalStr = seasonEmote[seasonIcon]
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

	for k, opt := range optionMap {
		if strings.HasSuffix(k, "contract-id") {
			searchString = opt.StringValue()
			break
		}
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

			seasonalStr := ""
			if c.SeasonID != "" {
				seasonYear := strings.Split(c.SeasonID, "_")[1]
				seasonIcon := strings.Split(c.SeasonID, "_")[0]
				seasonEmote := map[string]string{"winter": "❄️", "spring": "🌷", "summer": "🌞", "fall": "🍂"}
				seasonalStr = fmt.Sprintf("%s%s", seasonEmote[seasonIcon], seasonYear[2:4])
			}

			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  fmt.Sprintf("%s (%s)%s %s", c.Name, c.ID, ultra, seasonalStr),
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
			strings.Contains(strings.ToLower(c.Name), strings.ToLower(searchString)) ||
			strings.Contains(strings.ToLower(c.SeasonID), strings.ToLower(searchString)) {

			seasonalStr := ""
			if c.SeasonID != "" {
				seasonYear := strings.Split(c.SeasonID, "_")[1]
				seasonIcon := strings.Split(c.SeasonID, "_")[0]
				seasonEmote := map[string]string{"winter": "❄️", "spring": "🌷", "summer": "🌞", "fall": "🍂"}
				seasonalStr = fmt.Sprintf("%s%s", seasonEmote[seasonIcon], seasonYear[2:4])
			}

			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  fmt.Sprintf("%s (%s) %s", c.Name, c.ID, seasonalStr),
				Value: c.ID,
			}
			choices = append(choices, &choice)
			if len(choices) >= 13 {
				break
			}
		}
	}

	sortContractChoices(choices)
	choices = limitContractChoices(choices, 13)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Contract ID",
			Choices: choices,
		}})

}

type eggscapeCoopIDFile struct {
	CoopCodes []string `json:"coop_codes"`
}

var eggscapeCoopCodes []string

// LoadEggscapeCoopIDs loads the eggscape coop ID list from a JSON file.
func LoadEggscapeCoopIDs(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Printf("failed to open eggscape coop ID file: %v", err)
		return
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			log.Printf("failed to close eggscape coop ID file: %v", cerr)
		}
	}()

	var loaded eggscapeCoopIDFile
	if err := json.NewDecoder(file).Decode(&loaded); err != nil {
		log.Printf("failed to decode eggscape coop ID file: %v", err)
		return
	}

	eggscapeCoopCodes = append([]string(nil), loaded.CoopCodes...)
	log.Printf("Loaded %d eggscape coop ID codes", len(loaded.CoopCodes))
}

func handleCoopIDAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate, search string) {
	if !guildstate.GetGuildSettingFlag(i.GuildID, "coopid_suggestions") {
		userID := getInteractionUserID(i)
		recent := farmerstate.GetRecentCoopIDs(userID)

		choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(recent))
		search = strings.ToLower(search)
		for _, code := range recent {
			if search == "" || strings.Contains(strings.ToLower(code), search) {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: code, Value: code})
			}
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: choices},
		})
		return
	}

	codes := append([]string(nil), eggscapeCoopCodes...)

	search = strings.ToLower(search)
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, maxAutocompleteChoices)

	if search == "" {
		rand.Shuffle(len(codes), func(a, b int) { codes[a], codes[b] = codes[b], codes[a] })
		for _, code := range codes {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: code, Value: code})
			if len(choices) >= maxAutocompleteChoices {
				break
			}
		}
	} else {
		for _, code := range codes {
			if strings.Contains(code, search) {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: code, Value: code})
				if len(choices) >= maxAutocompleteChoices {
					break
				}
			}
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Coop ID",
			Choices: choices,
		}})
}
