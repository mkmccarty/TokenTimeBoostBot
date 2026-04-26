package boost

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"github.com/bwmarrin/discordgo"
)

// GetSlashAvailabilityCommand returns the slash command for setting availability
func GetSlashAvailabilityCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Set your availability for a contract.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
	}
}

// GetAvailabilityComponents returns the components for the availability command
func GetAvailabilityComponents(s *discordgo.Session, contract *Contract, userID string) []discordgo.MessageComponent {
	isCoord := creatorOfContract(s, contract, userID)
	inContract := UserInContract(contract, userID)

	var out []discordgo.MessageComponent

	timeLabels := map[string]string{
		"00-02": "0", "02-04": "+2", "04-06": "+4",
		"06-08": "+6", "08-10": "+8", "10-12": "+10",
		"12-14": "+12", "14-16": "+14", "16-18": "+16",
		"18-20": "+18", "20-22": "+20", "22-24": "+22",
	}

	formatTimes := func(slots []string) string {
		if len(slots) == 0 {
			return "Not set"
		}
		if len(slots) == 12 {
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

	if inContract {
		b := contract.Boosters[userID]
		if contract.PredictionSignup && len(contract.PredictionInfo) > 0 {
			options := make([]discordgo.SelectMenuOption, len(contract.PredictionInfo))
			for idx, pi := range contract.PredictionInfo {
				eggName := pi.EggName
				if !strings.HasPrefix(strings.ToLower(eggName), "egg_") {
					eggName = "egg_" + eggName
				}
				isDefault := false
				if b != nil && slices.Contains(b.Availability.Contract, pi.ContractID) {
					isDefault = true
				}
				options[idx] = discordgo.SelectMenuOption{
					Label:       pi.Name,
					Value:       pi.ContractID,
					Description: pi.ContractID,
					Emoji:       ei.GetBotComponentEmoji(eggName),
					Default:     isDefault,
				}
			}
			minValues := 0
			maxValues := len(options)
			out = append(out, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "rc_#predmenu#" + contract.ContractHash,
						Placeholder: "Select contracts you want to run",
						MinValues:   &minValues,
						MaxValues:   maxValues,
						Options:     options,
					},
				},
			})
		}

		timeOptions := []discordgo.SelectMenuOption{
			{Label: "0", Value: "00-02"},
			{Label: "+2", Value: "02-04"},
			{Label: "+4", Value: "04-06"},
			{Label: "+6", Value: "06-08"},
			{Label: "+8", Value: "08-10"},
			{Label: "+10", Value: "10-12"},
			{Label: "+12", Value: "12-14"},
			{Label: "+14", Value: "14-16"},
			{Label: "+16", Value: "16-18"},
			{Label: "+18", Value: "18-20"},
			{Label: "+20", Value: "20-22"},
			{Label: "+22", Value: "22-24"},
		}
		for i := range timeOptions {
			if b != nil && slices.Contains(b.Availability.Timeslots, timeOptions[i].Value) {
				timeOptions[i].Default = true
			}
		}
		minValuesTime := 0
		maxValuesTime := len(timeOptions)
		out = append(out, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "rc_#predtime#" + contract.ContractHash,
					Placeholder: "Select preferred start offsets (hours)",
					MinValues:   &minValuesTime,
					MaxValues:   maxValuesTime,
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
							name := b.Name
							if name == "" {
								name = b.UserID
							}
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
						name := b.Name
						if name == "" {
							name = b.UserID
						}
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
						name := b.Name
						if name == "" {
							name = b.UserID
						}
						fmt.Fprintf(&report, "> **%s**: %s\n", name, formatTimes(b.Availability.Timeslots))
					}
				}
			}
			if count == 0 {
				report.WriteString("> *No availability times set yet.*\n")
			}
		}

		out = append([]discordgo.MessageComponent{
			&discordgo.TextDisplay{Content: report.String()},
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
			out = append([]discordgo.MessageComponent{
				&discordgo.TextDisplay{Content: report.String()},
			}, out...)
		}
	}

	if inContract && len(out) > 0 {
		for _, c := range out {
			if _, ok := c.(discordgo.ActionsRow); ok {
				out = append([]discordgo.MessageComponent{
					&discordgo.TextDisplay{Content: "Select your availability options below:"},
				}, out...)
				break
			}
		}
	}

	return out
}

// HandleAvailabilityCommand handles the /availability command
func HandleAvailabilityCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This command can only be run in a server.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	contract := FindContract(i.ChannelID)
	if contract == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This command requires a running contract in this channel.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	userID := getInteractionUserID(i)
	isCoord := creatorOfContract(s, contract, userID)
	inContract := UserInContract(contract, userID)

	if !inContract && !isCoord {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You must join the contract first to set your availability.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	out := GetAvailabilityComponents(s, contract, userID)
	flags := discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Components: out,
			Flags:      flags,
		},
	})
	if err != nil {
		fmt.Printf("Error responding to availability command: %v\n", err)
	}
}
