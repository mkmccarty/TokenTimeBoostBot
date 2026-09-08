package boost

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// GetSlashAvailabilityCommand returns the slash command for setting availability
func GetSlashAvailabilityCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Set your availability for a contract.")
	return &command
}

// GetAvailabilityComponents returns the components for the availability command
func GetAvailabilityComponents(client dc.Client, contract *Contract, userID string) []dc.LayoutComponent {
	isCoord := creatorOfContract(client, contract, userID)
	inContract := UserInContract(contract, userID)

	var out []dc.LayoutComponent

	timeLabels := map[string]string{
		"00-01": "+0", "01-02": "+1", "02-03": "+2", "03-04": "+3",
		"04-05": "+4", "05-06": "+5", "06-07": "+6", "07-08": "+7", "08-09": "+8",
		"20-21": "-4", "21-22": "-3", "22-23": "-2", "23-24": "-1",
	}

	formatTimes := func(slots []string) string {
		if len(slots) == 0 {
			return "Not set"
		}
		if len(slots) == len(timeLabels) {
			return "Any"
		}
		sorted := make([]string, len(slots))
		copy(sorted, slots)
		sort.Strings(sorted)
		var short []string
		for _, s := range sorted {
			if l, ok := timeLabels[s]; ok {
				short = append(short, l)
			} else {
				short = append(short, s)
			}
		}
		return strings.Join(short, ", ")
	}

	// Discord defaults an unset MinValues to 1; these menus are deselectable,
	// so the zero has to be sent explicitly.
	minValues := 0

	if inContract {
		b := contract.Boosters[userID]
		if contract.PredictionSignup && len(contract.PredictionInfo) > 0 {
			options := make([]dc.SelectOption, len(contract.PredictionInfo))
			for idx, pi := range contract.PredictionInfo {
				isDefault := false
				if b != nil && slices.Contains(b.Availability.Contract, pi.ContractID) {
					isDefault = true
				}
				componentEmoji := ei.FindEggComponentEmoji(pi.EggName)
				options[idx] = dc.SelectOption{
					Label:       pi.Name,
					Value:       pi.ContractID,
					Description: pi.ContractID,
					Emoji:       componentEmoji,
					Default:     isDefault,
				}
			}
			out = append(out, dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.SelectMenu{
						CustomID:    "rc_#predmenu#" + contract.ContractHash,
						Placeholder: "Select contracts you want to run",
						MinValues:   &minValues,
						MaxValues:   len(options),
						Options:     options,
					},
				},
			})
		}

		timeOptions := []dc.SelectOption{
			{Label: "+0", Value: "00-01"},
			{Label: "+1", Value: "01-02"},
			{Label: "+2", Value: "02-03"},
			{Label: "+3", Value: "03-04"},
			{Label: "+4", Value: "04-05"},
			{Label: "+5", Value: "05-06"},
			{Label: "+6", Value: "06-07"},
			{Label: "+7", Value: "07-08"},
			{Label: "+8", Value: "08-09"},
			{Label: "-4", Value: "20-21"},
			{Label: "-3", Value: "21-22"},
			{Label: "-2", Value: "22-23"},
			{Label: "-1", Value: "23-24"},
		}
		for i := range timeOptions {
			if b != nil && slices.Contains(b.Availability.Timeslots, timeOptions[i].Value) {
				timeOptions[i].Default = true
			}
		}
		out = append(out, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    "rc_#predtime#" + contract.ContractHash,
					Placeholder: "Select preferred start offsets (hours)",
					MinValues:   &minValues,
					MaxValues:   len(timeOptions),
					Options:     timeOptions,
				},
			},
		})
	}

	if isCoord {
		var report strings.Builder
		report.WriteString("## Availability Report\n")

		if contract.PredictionSignup && len(contract.PredictionInfo) > 0 {
			for _, pi := range contract.PredictionInfo {
				fmt.Fprintf(&report, "### %s %s\n", ei.FindEggEmoji(pi.EggName), pi.Name)
				count := 0
				for _, orderUserID := range contract.Order {
					if b := contract.Boosters[orderUserID]; b != nil {
						if slices.Contains(b.Availability.Contract, pi.ContractID) {
							count++
							name := b.Nick
							fmt.Fprintf(&report, "> **%s**: %s\n", name, formatTimes(b.Availability.Timeslots))
						}
					}
				}
				if count == 0 {
					report.WriteString("> *No farmers selected this contract.*\n")
				}
			}

			unassignedCount := 0
			var unassignedReport strings.Builder
			for _, orderUserID := range contract.Order {
				if b := contract.Boosters[orderUserID]; b != nil {
					if len(b.Availability.Contract) == 0 && len(b.Availability.Timeslots) > 0 {
						unassignedCount++
						name := b.Nick
						fmt.Fprintf(&unassignedReport, "> **%s**: %s\n", name, formatTimes(b.Availability.Timeslots))
					}
				}
			}
			if unassignedCount > 0 {
				report.WriteString("### No Specific Contract Selected\n")
				report.WriteString(unassignedReport.String())
			}
		} else {
			count := 0
			for _, orderUserID := range contract.Order {
				if b := contract.Boosters[orderUserID]; b != nil {
					if len(b.Availability.Timeslots) > 0 {
						count++
						name := b.Nick
						fmt.Fprintf(&report, "> **%s**: %s\n", name, formatTimes(b.Availability.Timeslots))
					}
				}
			}
			if count == 0 {
				report.WriteString("> *No availability times set yet.*\n")
			}
		}

		out = append([]dc.LayoutComponent{
			dc.TextDisplay{Content: report.String()},
		}, out...)
	} else if inContract {
		b := contract.Boosters[userID]
		if b != nil {
			var report strings.Builder
			report.WriteString("## Your Current Availability\n")
			if contract.PredictionSignup && len(contract.PredictionInfo) > 0 {
				var selectedContracts []string
				for _, pi := range contract.PredictionInfo {
					if slices.Contains(b.Availability.Contract, pi.ContractID) {
						selectedContracts = append(selectedContracts, fmt.Sprintf("%s %s", ei.FindEggEmoji(pi.EggName), pi.Name))
					}
				}
				if len(selectedContracts) > 0 {
					fmt.Fprintf(&report, "> **Contracts**: %s\n", strings.Join(selectedContracts, ", "))
				} else {
					report.WriteString("> **Contracts**: None selected\n")
				}
			}
			fmt.Fprintf(&report, "> **Times**: %s\n", formatTimes(b.Availability.Timeslots))
			out = append([]dc.LayoutComponent{
				dc.TextDisplay{Content: report.String()},
			}, out...)
		}
	}

	if inContract && len(out) > 0 {
		for _, c := range out {
			if _, ok := c.(dc.ActionRow); ok {
				out = append([]dc.LayoutComponent{
					dc.TextDisplay{Content: "Select your availability options below:"},
				}, out...)
				break
			}
		}
	}

	return out
}

// HandleAvailabilityCommand handles the /availability command through the dc
// facade.
//
// It still takes a raw session because creatorOfContract is not on the facade
// yet.
func HandleAvailabilityCommand(client dc.Client, e *dc.CommandEvent) {
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}

	contract := FindContract(e.ChannelID())
	if contract == nil {
		_ = e.Respond(dc.Message{
			Content:   "This command requires a running contract in this channel.",
			Ephemeral: true,
		})
		return
	}

	userID := e.UserID()
	isCoord := creatorOfContract(client, contract, userID)
	inContract := UserInContract(contract, userID)

	if !inContract && !isCoord {
		_ = e.Respond(dc.Message{
			Content:   "You must join the contract first to set your availability.",
			Ephemeral: true,
		})
		return
	}

	err := e.Respond(dc.Message{
		Components: GetAvailabilityComponents(client, contract, userID),
		Ephemeral:  true,
	})
	if err != nil {
		fmt.Printf("Error responding to availability command: %v\n", err)
	}
}
