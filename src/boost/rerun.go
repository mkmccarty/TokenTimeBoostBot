package boost

import (
	"encoding/base64"
	"slices"
	"strconv"
	"strings"

	"log"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashRerunEvalCommand returns the command for the /launch-helper command
func GetSlashRerunEvalCommand(cmd string) *dc.Command {
	percentMin, percentMax := 0, 50
	refreshOption := dc.BoolOption{
		Name:        "refresh",
		Description: "If you want to force a refresh due a recent change to your contracts.",
	}
	mobileOption := dc.BoolOption{
		Name:        "mobile-friendly",
		Description: "Format output for mobile devices (sticky)",
	}
	// This command is built from subcommands, and Discord does not allow a
	// command to carry top-level options alongside them, so the reset flag is
	// declared on each subcommand. The handler reads it by its bare name,
	// which resolves whichever subcommand was invoked.
	resetOption := dc.BoolOption{
		Name:        "reset",
		Description: "Reset stored EI number",
	}

	command := anywhereCommand(cmd, "Evaluate a contract's history and provide replay guidance.")
	command.Options = []dc.Option{
		dc.SubCommand{
			Name:        "active",
			Description: "Evaluate Active Contract Details",
			Options: []dc.Option{
				dc.StringOption{
					Name:         "contract-id",
					Description:  "Contract ID",
					Required:     true,
					Autocomplete: true,
				},
				refreshOption,
				mobileOption,
				resetOption,
			},
		},
		dc.SubCommand{
			Name:        "chart",
			Description: "Summary chart of active contracts evaluations",
			Options:     []dc.Option{refreshOption, mobileOption, resetOption},
		},
		dc.SubCommand{
			Name:        "predictions",
			Description: "Summary chart of predicted contracts evaluations",
			Options:     []dc.Option{refreshOption, mobileOption, resetOption},
		},
		dc.SubCommand{
			Name:        "season",
			Description: "Summary chart of evaluations for a specific season",
			Options: []dc.Option{
				dc.StringOption{
					Name:         "season",
					Description:  "Season to display. Defaults to the current season.",
					Autocomplete: true,
				},
				refreshOption,
				mobileOption,
				resetOption,
			},
		},
		dc.SubCommand{
			Name:        "threshold",
			Description: "Summarize contracts below a certain % of speedrun score",
			Options: []dc.Option{
				dc.IntOption{
					Name:        "percent",
					Description: "Below % of speedrun score",
					Required:    true,
					MinValue:    &percentMin,
					MaxValue:    &percentMax,
				},
				mobileOption,
				resetOption,
			},
		},
	}
	return &command
}

// HandleRerunEvalAutoComplete dispatches autocomplete for /rerun-eval subcommands.
// The "active" subcommand gets contract-id autocomplete; the "season" subcommand
// gets season autocomplete powered by leaderboardSeasons().
func HandleRerunEvalAutoComplete(e *dc.AutocompleteEvent) {
	if sub, ok := e.Subcommand(); ok && sub == "season" {
		HandleRerunEvalSeasonAutoComplete(e)
		return
	}
	HandleAllContractsAutoComplete(e)
}

// HandleRerunEvalSeasonAutoComplete suggests seasons for the /rerun-eval season option.
func HandleRerunEvalSeasonAutoComplete(e *dc.AutocompleteEvent) {
	search := ""
	if name, value := e.FocusedOption(); name == "season" {
		search = strings.ToLower(strings.TrimSpace(value))
	}

	choices := make([]dc.Choice[string], 0, leaderboardMaxAutocompleteChoices)
	for _, season := range leaderboardSeasons() {
		if season.value == leaderboardAllTimeScope {
			continue // season subcommand is always for a specific season
		}
		if search != "" {
			name := strings.ToLower(season.name)
			value := strings.ToLower(season.value)
			if !strings.Contains(name, search) && !strings.Contains(value, search) {
				continue
			}
		}
		choices = append(choices, dc.Choice[string]{
			Name:  season.name,
			Value: season.value,
		})
		if len(choices) >= leaderboardMaxAutocompleteChoices {
			break
		}
	}

	_ = e.RespondChoices(choices)
}

// HandleReplayEval handles the /replay-eval command.
func HandleReplayEval(e *dc.CommandEvent) {
	// Check if user has permission to use CoopStatus API
	if !CheckCoopStatusPermission(e, ei.CoopStatusFixEnabled != nil && ei.CoopStatusFixEnabled()) {
		return
	}

	userID := e.UserID()

	if opt, ok := e.OptBool("reset"); ok && opt {
		farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
	}
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	RerunEval(e, e.Options(), eiID, true)
}

// RerunEval evaluates the contract history and provides replay guidance
func RerunEval(e dc.InteractionEvent, options dc.OptionValues, eiID string, okayToSave bool) {
	// Get the Egg Inc ID from the stored settings
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
	if eggIncID == "" || len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		// Only the command path reaches this: the modal path already checked
		// that an ID came back, and Discord will not answer a modal with a
		// modal.
		if cmd, ok := e.(*dc.CommandEvent); ok {
			RequestEggIncIDModal(cmd, "replay", options)
		}
		return
	}

	percent := -1
	page := 1
	contractID := ""
	forceRefresh := false
	contractIDList := []string{}
	seasonScope := ""

	if opt, ok := options.Uint("threshold-percent"); ok {
		percent = int(opt)
	}
	if opt, ok := options.String("active-contract-id"); ok {
		contractID = opt
		contractIDList = append(contractIDList, contractID)
	}
	contractDayMap := make(map[string]string)
	if sub, ok := options.Subcommand(); ok && sub == "predictions" {
		fridayNonUltra, fridayUltra, wednesdayNonUltra := predictJeli(3)
		// for each of these 3 I want to collect the contract IDs
		for _, c := range fridayNonUltra {
			if slices.Contains(contractIDList, c.ID) {
				continue
			}
			contractIDList = append(contractIDList, c.ID)
			contractDayMap[c.ID] = "F"
		}
		for _, c := range fridayUltra {
			if slices.Contains(contractIDList, c.ID) {
				continue
			}
			contractIDList = append(contractIDList, c.ID)
			contractDayMap[c.ID] = "U"
		}
		for _, c := range wednesdayNonUltra {
			if slices.Contains(contractIDList, c.ID) {
				continue
			}
			contractIDList = append(contractIDList, c.ID)
			contractDayMap[c.ID] = "W"
		}
		percent = -200
	} else if sub, ok := options.Subcommand(); ok && sub == "season" {
		// Default to the current season if the user did not pick one.
		if opt, ok := options.String("season-season"); ok && opt != "" {
			seasonScope = opt
		}
		if seasonScope == "" {
			if name, year, ok := leaderboardMostRecentSeason(); ok {
				seasonScope = leaderboardSeasonID(name, year)
			}
		}
		ei.EggIncContractsMutex.RLock()
		for _, c := range ei.EggIncContractsAll {
			if c.Predicted {
				continue
			}
			if !strings.EqualFold(c.SeasonID, seasonScope) {
				continue
			}
			if !slices.Contains(contractIDList, c.ID) {
				contractIDList = append(contractIDList, c.ID)
			}
		}
		ei.EggIncContractsMutex.RUnlock()
		percent = -100 // season chart: show all contracts, no time or percent filter
	}
	if opt, ok := options.Bool("chart-refresh"); ok {
		forceRefresh = opt
	}
	if opt, ok := options.Bool("predictions-refresh"); ok {
		forceRefresh = opt
	}
	if opt, ok := options.Bool("active-refresh"); ok {
		forceRefresh = opt
	}
	if opt, ok := options.Bool("season-refresh"); ok {
		forceRefresh = opt
	}

	// Quick reply to buy us some time
	_ = e.Defer(false)

	userID := e.UserID()

	mobileFriendly := farmerstate.GetMiscSettingString(userID, "rerunMobileFriendly") == "true"
	if opt, ok := options.Bool("chart-mobile-friendly"); ok {
		mobileFriendly = opt
		farmerstate.SetMiscSettingString(userID, "rerunMobileFriendly", strconv.FormatBool(mobileFriendly))
	} else if opt, ok := options.Bool("predictions-mobile-friendly"); ok {
		mobileFriendly = opt
		farmerstate.SetMiscSettingString(userID, "rerunMobileFriendly", strconv.FormatBool(mobileFriendly))
	} else if opt, ok := options.Bool("threshold-mobile-friendly"); ok {
		mobileFriendly = opt
		farmerstate.SetMiscSettingString(userID, "rerunMobileFriendly", strconv.FormatBool(mobileFriendly))
	} else if opt, ok := options.Bool("season-mobile-friendly"); ok {
		mobileFriendly = opt
		farmerstate.SetMiscSettingString(userID, "rerunMobileFriendly", strconv.FormatBool(mobileFriendly))
	} else if val := farmerstate.GetMiscSettingString(userID, "rerunMobileFriendly"); val != "" {
		mobileFriendly, _ = strconv.ParseBool(val)
	}

	// Do I know the user's IGN?
	farmerName := farmerstate.GetMiscSettingString(userID, "ei_ign")
	if farmerName == "" {
		backup, _ := ei.GetFirstContactFromAPI(eggIncID, userID, okayToSave)
		if backup != nil {
			farmerName = backup.GetUserName()
			farmerstate.SetMiscSettingString(userID, "ei_ign", farmerName)
		}
	}
	archive, _ := ei.GetContractArchiveFromAPI(eggIncID, userID, forceRefresh, okayToSave)

	var components []dc.LayoutComponent
	if len(contractIDList) == 1 {
		components = printActiveContractDetails(userID, archive, contractIDList[0])
	} else {
		components = printContractChart(userID, archive, percent, page, contractIDList, contractDayMap, mobileFriendly, seasonScope)
	}

	if err = e.Followup(dc.Message{Components: components}); err != nil {
		log.Println("Error sending follow-up message:", err)
	}

}
