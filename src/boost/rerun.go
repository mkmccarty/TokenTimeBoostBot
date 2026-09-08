package boost

import (
	"encoding/base64"
	"slices"
	"strconv"

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
		components = printContractChart(userID, archive, percent, page, contractIDList, contractDayMap, mobileFriendly)
	}

	if err = e.Followup(dc.Message{Components: components}); err != nil {
		log.Println("Error sending follow-up message:", err)
	}

}
