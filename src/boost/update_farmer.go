package boost

import (
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetSlashUpdateCommand returns the /update slash command with main subcommand groups for farmer and contract
func GetSlashUpdateCommand(cmd string) *dc.Command {
	zero := 0
	maxTokens, maxTE, maxIHR := 12, 490, 1000000000000

	farmerName := dc.StringOption{
		Name:        "farmername",
		Description: "Farmer name to update",
		Required:    true,
	}
	artifactTiers := func(name, description string, choices []dc.Choice[string]) dc.StringOption {
		return dc.StringOption{Name: name, Description: description, Choices: choices}
	}

	command := guildOnlyCommand(cmd, "Update farmer statistics")
	command.Options = []dc.Option{
		dc.SubCommandGroup{
			Name:        "farmer",
			Description: "Update farmer statistics",
			Options: []dc.SubCommand{
				{
					Name:        "boost-tokens",
					Description: "Update boost tokens (0-12)",
					Options: []dc.Option{
						farmerName,
						dc.IntOption{
							Name:        "value",
							Description: "Number of boost tokens (0-12)",
							Required:    true,
							MinValue:    &zero,
							MaxValue:    &maxTokens,
						},
					},
				},
				{
					Name:        "te",
					Description: "Update TE value (0-490)",
					Options: []dc.Option{
						farmerName,
						dc.IntOption{
							Name:        "value",
							Description: "TE value (0-490)",
							Required:    true,
							MinValue:    &zero,
							MaxValue:    &maxTE,
						},
					},
				},
				{
					Name:        "ihr",
					Description: "Update IHR value",
					Options: []dc.Option{
						farmerName,
						dc.IntOption{
							Name:        "value",
							Description: "IHR value",
							Required:    true,
							MinValue:    &zero,
							MaxValue:    &maxIHR,
						},
					},
				},
				{
					Name:        "artifacts",
					Description: "Update farmer artifact settings (defl, metr, comp, guss)",
					Options: []dc.Option{
						farmerName,
						artifactTiers("deflector", "Deflector tier", []dc.Choice[string]{
							{Name: "Deflector T4L (Legendary)", Value: "T4L"},
							{Name: "Deflector T4E (Epic)", Value: "T4E"},
							{Name: "Deflector T4R (Rare)", Value: "T4R"},
							{Name: "Deflector T4C (Common)", Value: "T4C"},
							{Name: "Deflector T3R (Rare)", Value: "T3R"},
							{Name: "Deflector T3C (Common)", Value: "T3C"},
							{Name: "None", Value: "NONE"},
						}),
						artifactTiers("metronome", "Metronome tier", []dc.Choice[string]{
							{Name: "Metronome T4L (Legendary)", Value: "T4L"},
							{Name: "Metronome T4E (Epic)", Value: "T4E"},
							{Name: "Metronome T4R (Rare)", Value: "T4R"},
							{Name: "Metronome T4C (Common)", Value: "T4C"},
							{Name: "Metronome T3E (Epic)", Value: "T3E"},
							{Name: "Metronome T3R (Rare)", Value: "T3R"},
							{Name: "Metronome T3C (Common)", Value: "T3C"},
							{Name: "None", Value: "NONE"},
						}),
						artifactTiers("compass", "Compass tier", []dc.Choice[string]{
							{Name: "Compass T4L (Legendary)", Value: "T4L"},
							{Name: "Compass T4E (Epic)", Value: "T4E"},
							{Name: "Compass T4R (Rare)", Value: "T4R"},
							{Name: "Compass T4C (Common)", Value: "T4C"},
							{Name: "Compass T3R (Rare)", Value: "T3R"},
							{Name: "Compass T3C (Common)", Value: "T3C"},
							{Name: "None", Value: "NONE"},
						}),
						artifactTiers("gusset", "Gusset tier", []dc.Choice[string]{
							{Name: "Gusset T4L (Legendary)", Value: "T4L"},
							{Name: "Gusset T4E (Epic)", Value: "T4E"},
							{Name: "Gusset T4C (Common)", Value: "T4C"},
							{Name: "Gusset T3R (Rare)", Value: "T3R"},
							{Name: "Gusset T3C (Common)", Value: "T3C"},
							{Name: "Gusset T2E (Epic)", Value: "T2E"},
							{Name: "None", Value: "NONE"},
						}),
					},
				},
			},
		},
	}
	return &command
}

// HandleUpdateCommand handles the /update slash command through the dc facade.
//
// It still takes a raw session because the boost list redraw is not on the
// facade yet.
func HandleUpdateCommand(client dc.Client, e *dc.CommandEvent) {
	_ = e.Defer(true)

	subcommandGroup := ""
	subcommand := ""
	farmername := ""
	value := int64(0)

	// Get the subcommand group and subcommand from nested options
	if path := e.SubcommandPath(); len(path) > 0 {
		subcommandGroup = path[0]
		if len(path) > 1 {
			subcommand = path[1]
		}
	}

	// Extract values based on subcommand group
	switch subcommandGroup {
	case "farmer":
		if opt, ok := e.OptString("farmer-boost-tokens-farmername"); ok {
			farmername = strings.TrimSpace(opt)
		}
		if opt, ok := e.OptInt("farmer-boost-tokens-value"); ok {
			value = int64(opt)
		}
		if opt, ok := e.OptString("farmer-te-farmername"); ok {
			farmername = strings.TrimSpace(opt)
		}
		if opt, ok := e.OptInt("farmer-te-value"); ok {
			value = int64(opt)
		}
		if opt, ok := e.OptString("farmer-ihr-farmername"); ok {
			farmername = strings.TrimSpace(opt)
		}
		if opt, ok := e.OptInt("farmer-ihr-value"); ok {
			value = int64(opt)
		}
		if opt, ok := e.OptString("farmer-artifacts-farmername"); ok {
			farmername = strings.TrimSpace(opt)
		}
	}

	userID := farmername
	resultMsg := ""

	// Try to find the user by farmername or discord mention
	if farmername != "" {
		if mentionID, isMention := parseMentionUserID(farmername); isMention {
			userID = mentionID
		}
	}

	// Handle the specific subcommand
	switch subcommandGroup {
	case "farmer":
		switch subcommand {
		case "boost-tokens":
			farmerstate.SetTokens(userID, int(value))
			resultMsg = fmt.Sprintf("✅ Updated %s's boost tokens to %d", farmername, value)

		case "te":
			farmerstate.SetMiscSettingString(userID, "TE", fmt.Sprintf("%d", value))
			resultMsg = fmt.Sprintf("✅ Updated %s's TE to %d", farmername, value)

		case "ihr":
			farmerstate.SetMiscSettingString(userID, "IHR", fmt.Sprintf("%d", value))
			resultMsg = fmt.Sprintf("✅ Updated %s's IHR to %d", farmername, value)

		case "artifacts":
			var changed []string
			artifactKeys := map[string]string{
				"farmer-artifacts-deflector": "defl",
				"farmer-artifacts-metronome": "metr",
				"farmer-artifacts-compass":   "comp",
				"farmer-artifacts-gusset":    "guss",
			}
			for optKey, settingKey := range artifactKeys {
				if opt, ok := e.OptString(optKey); ok {
					farmerstate.SetMiscSettingString(userID, settingKey, opt)
					changed = append(changed, fmt.Sprintf("%s: %s", settingKey, opt))
				}
			}
			if len(changed) > 0 {
				resultMsg = fmt.Sprintf("✅ Updated %s's artifacts: %s", farmername, strings.Join(changed, ", "))
			} else {
				resultMsg = "No artifact values provided"
			}

		default:
			resultMsg = "Unknown farmer subcommand"
		}

		// If this farmer is in any contracts, update their contract.Boosters data with the changed value
		updateFarmerInContracts(client, userID, subcommand, value)

	default:
		resultMsg = "Unknown subcommand group"
	}

	_ = e.Followup(dc.Message{
		Content:   resultMsg,
		Ephemeral: true,
	})
}

// updateFarmerInContracts updates a farmer's data in all contracts they're part of
func updateFarmerInContracts(client dc.Client, userID string, subcommand string, value int64) {
	mutex.Lock()
	contractsCopy := make([]*Contract, 0, len(Contracts))
	for _, c := range Contracts {
		contractsCopy = append(contractsCopy, c)
	}
	mutex.Unlock()

	for _, contract := range contractsCopy {
		contract.mutex.Lock()
		if booster, exists := contract.Boosters[userID]; exists {
			// Update the specific field based on which subcommand was used
			switch subcommand {
			case "boost-tokens":
				booster.TokensWanted = int(value)
			case "te":
				booster.TECount = int(value)
				rate, logStr := CalculateIHRRateFromDB(userID)
				booster.IHRRate = rate
				booster.IHRCalcLog = logStr
			case "ihr":
				booster.IHRRate = float64(value)
				booster.IHRCalcLog = fmt.Sprintf("IHR Calculation (Manual for %s): Final=%0.2f", userID, float64(value))
			case "artifacts":
				rate, logStr := CalculateIHRRateFromDB(userID)
				booster.IHRRate = rate
				booster.IHRCalcLog = logStr
			}
			contract.mutex.Unlock()

			// Redraw the boost list message to reflect the updated data
			for _, loc := range contract.Location {
				_ = loc
				refreshBoostListMessage(client, contract, false)
				break // Only need to refresh once
			}
		} else {
			contract.mutex.Unlock()
		}
	}
}
