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

// handleBoostOrderAutoComplete handles autocomplete for the boost-order option in /contract
func handleBoostOrderAutoComplete(e *dc.AutocompleteEvent, searchString string) {
	searchString = strings.ToLower(searchString)
	choices := make([]dc.Choice[string], 0)

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
			choices = append(choices, dc.Choice[string]{
				Name:  formattedName,
				Value: fmt.Sprintf("%d", orderVal),
			})
		}
	}

	if err := e.RespondChoices(choices); err != nil {
		log.Println("Error responding to boost order autocomplete:", err)
	}
}
