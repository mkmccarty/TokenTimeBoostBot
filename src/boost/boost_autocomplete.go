package boost

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"sort"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

const maxAutocompleteChoices = 25

func sortContractChoices(choices []dc.Choice[string]) {
	sort.Slice(choices, func(i, j int) bool {
		return choices[i].Name < choices[j].Name
	})
}

func limitContractChoices(choices []dc.Choice[string], maxChoices int) []dc.Choice[string] {
	if len(choices) <= maxChoices {
		return choices
	}
	return choices[:maxChoices]
}

// HandleContractAutoComplete will handle the contract auto complete of contract-id's
func HandleContractAutoComplete(e *dc.AutocompleteEvent) {
	// The focused option carries the leaf name, so a nested "coop-id" and a
	// top-level one arrive under the same name.
	switch focused, value := e.FocusedOption(); focused {
	case "boost-order":
		handleBoostOrderAutoComplete(e, value)
		return
	case "coop-id":
		handleCoopIDAutoComplete(e, value)
		return
	}

	searchString := ""
	if opt, ok := e.OptString("contract-id"); ok {
		searchString = strings.ToLower(opt)
	}
	if opt, ok := e.OptString("contract-contract-id-contract-id"); ok {
		searchString = strings.ToLower(opt)
	}

	isContractCommand := e.CommandName() == "contract"

	contract := FindContract(e.ChannelID())
	allowPredicted := isContractCommand || (contract != nil && contract.State == ContractStateSignup && contract.PredictionSignup)

	choices := make([]dc.Choice[string], 0)
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
		choices = append(choices, dc.Choice[string]{
			Name:  fmt.Sprintf("%s (%s)%s %s", c.Name, c.ID, ultra, seasonalStr),
			Value: c.ID,
		})
	}

	//sortContractChoices(choices)
	choices = limitContractChoices(choices, maxAutocompleteChoices)

	_ = e.RespondChoices(choices)
}

// HandleAllContractsAutoComplete will handle the contract auto complete of contract-id's
// default to new contracts but allow searching all contracts
func HandleAllContractsAutoComplete(e *dc.AutocompleteEvent) {
	searchString := ""
	if name, value := e.FocusedOption(); strings.HasSuffix(name, "contract-id") {
		searchString = value
	}

	choices := make([]dc.Choice[string], 0)

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

			choices = append(choices, dc.Choice[string]{
				Name:  fmt.Sprintf("%s (%s)%s %s", c.Name, c.ID, ultra, seasonalStr),
				Value: c.ID,
			})
		}

		sortContractChoices(choices)
		choices = limitContractChoices(choices, maxAutocompleteChoices)

		_ = e.RespondChoices(choices)
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

			choices = append(choices, dc.Choice[string]{
				Name:  fmt.Sprintf("%s (%s) %s", c.Name, c.ID, seasonalStr),
				Value: c.ID,
			})
			if len(choices) >= 13 {
				break
			}
		}
	}

	sortContractChoices(choices)
	choices = limitContractChoices(choices, 13)

	_ = e.RespondChoices(choices)
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

func handleCoopIDAutoComplete(e *dc.AutocompleteEvent, search string) {
	if !guildstate.GetGuildSettingFlag(e.GuildID(), "coopid_suggestions") {
		recent := farmerstate.GetRecentCoopIDs(e.UserID())

		choices := make([]dc.Choice[string], 0, len(recent))
		search = strings.ToLower(search)
		for _, code := range recent {
			if search == "" || strings.Contains(strings.ToLower(code), search) {
				choices = append(choices, dc.Choice[string]{Name: code, Value: code})
			}
		}

		_ = e.RespondChoices(choices)
		return
	}

	codes := append([]string(nil), eggscapeCoopCodes...)

	search = strings.ToLower(search)
	choices := make([]dc.Choice[string], 0, maxAutocompleteChoices)

	if search == "" {
		rand.Shuffle(len(codes), func(a, b int) { codes[a], codes[b] = codes[b], codes[a] })
		for _, code := range codes {
			choices = append(choices, dc.Choice[string]{Name: code, Value: code})
			if len(choices) >= maxAutocompleteChoices {
				break
			}
		}
	} else {
		for _, code := range codes {
			if strings.Contains(code, search) {
				choices = append(choices, dc.Choice[string]{Name: code, Value: code})
				if len(choices) >= maxAutocompleteChoices {
					break
				}
			}
		}
	}

	_ = e.RespondChoices(choices)
}

// getBoostOrderAutoCompleteChoices generates autocomplete choices for the boost-order option in /contract
func getBoostOrderAutoCompleteChoices(userID string, searchString string) []dc.Choice[string] {
	searchString = strings.ToLower(strings.TrimSpace(searchString))
	choices := make([]dc.Choice[string], 0)

	userTemplates := GetUserCustomOrders(userID)
	personalPreset := farmerstate.GetMiscSettingString(userID, "custom_boost_order")
	globalTemplates := GetGlobalCustomOrders()

	isUserOrCustomSearch := searchString != "" && (strings.HasPrefix("user", searchString) || strings.Contains(searchString, "user") ||
		strings.HasPrefix("custom", searchString) || strings.Contains(searchString, "custom"))

	seenValues := make(map[string]bool)
	addChoice := func(name, value string) {
		if seenValues[value] || len(choices) >= maxAutocompleteChoices {
			return
		}
		seenValues[value] = true
		if len(name) > 100 {
			name = name[:100]
		}
		choices = append(choices, dc.Choice[string]{
			Name:  name,
			Value: value,
		})
	}

	type customChoiceItem struct {
		name  string
		value string
	}
	var userChoices []customChoiceItem

	for _, tmpl := range userTemplates {
		userChoices = append(userChoices, customChoiceItem{
			name:  fmt.Sprintf("User Custom: %s", tmpl.Name),
			value: fmt.Sprintf("custom_u:%s", tmpl.Name),
		})
	}

	if personalPreset != "" {
		lines := strings.Split(personalPreset, "\n")
		presetName := SuggestCustomOrderName(lines)
		if presetName == "" {
			presetName = "Personal Preset"
		}
		alreadyInTemplates := false
		for _, tmpl := range userTemplates {
			if strings.EqualFold(tmpl.Name, presetName) {
				alreadyInTemplates = true
				break
			}
		}
		if !alreadyInTemplates {
			userChoices = append(userChoices, customChoiceItem{
				name:  fmt.Sprintf("User Custom: %s (Preset)", presetName),
				value: "custom_p",
			})
		}
	}

	var globalChoices []customChoiceItem
	for _, tmpl := range globalTemplates {
		globalChoices = append(globalChoices, customChoiceItem{
			name:  fmt.Sprintf("Global Custom: %s", tmpl.Name),
			value: fmt.Sprintf("custom_g:%s", tmpl.Name),
		})
	}

	if isUserOrCustomSearch {
		// User specifically typing user or custom: show user defined orders first
		for _, uc := range userChoices {
			if searchString == "" || strings.Contains(strings.ToLower(uc.name), searchString) {
				addChoice(uc.name, uc.value)
			}
		}
		for _, gc := range globalChoices {
			if searchString == "" || strings.Contains(strings.ToLower(gc.name), searchString) {
				addChoice(gc.name, gc.value)
			}
		}
		if searchString == "" || strings.Contains("custom ordering", searchString) {
			addChoice("Custom Ordering", fmt.Sprintf("%d", ContractOrderCustom))
		}
	} else {
		// General search or empty: show standard orders first, then custom orders
		for orderVal, name := range contractOrderNames {
			if orderVal == ContractOrderFair {
				continue
			}
			var formattedName string
			switch orderVal {
			case ContractOrderSignup:
				formattedName = "Sign-up Ordering"
			case ContractOrderTimeBased:
				formattedName = "Time Based Ordering"
			case ContractOrderRandom:
				formattedName = "Random Ordering"
			case ContractOrderTEFuzzy:
				formattedName = "Fuzzy TE Ordering"
			case ContractOrderIHRFuzzy:
				formattedName = "Fuzzy IHR Ordering"
			default:
				formattedName = name + " Ordering"
			}

			if searchString == "" || strings.Contains(strings.ToLower(formattedName), searchString) {
				addChoice(formattedName, fmt.Sprintf("%d", orderVal))
			}
		}

		for _, uc := range userChoices {
			if searchString == "" || strings.Contains(strings.ToLower(uc.name), searchString) {
				addChoice(uc.name, uc.value)
			}
		}
		for _, gc := range globalChoices {
			if searchString == "" || strings.Contains(strings.ToLower(gc.name), searchString) {
				addChoice(gc.name, gc.value)
			}
		}
	}

	return choices
}

// handleBoostOrderAutoComplete handles autocomplete for the boost-order option in /contract
func handleBoostOrderAutoComplete(e *dc.AutocompleteEvent, searchString string) {
	choices := getBoostOrderAutoCompleteChoices(e.UserID(), searchString)
	if err := e.RespondChoices(choices); err != nil {
		log.Println("Error responding to boost order autocomplete:", err)
	}
}
