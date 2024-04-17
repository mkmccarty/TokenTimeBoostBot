package launch

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dannav/hhmmss"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/xhit/go-str2duration/v2"
)

// SlashLaunchHelperCommand returns the command for the /launch-helper command
func SlashLaunchHelperCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Display timestamp table for next mission.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "mission-duration",
				Description: "Time remaining for next mission(s). Example: 8h15m or \"8h15m, 10h5m, 1d2m\"",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "primary-ship",
				Description: "Select the primary ship to display. Default is Atreggies Henliner. [Sticky]",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Atreggies Henliner",
						Value: 0,
					},
					{
						Name:  "Henerprise",
						Value: 1,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "secondary-ship",
				Description: "Select a secondary ship to display. Default is Henerprise. [Sticky]",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "None",
						Value: -1,
					},
					{
						Name:  "All Stars Club",
						Value: -2,
					},
					{
						Name:  "Starfleet Commander",
						Value: -3,
					},
					{
						Name:  "Henerprise",
						Value: 1,
					},
					{
						Name:  "Voyegger",
						Value: 2,
					},
					{
						Name:  "Defihent",
						Value: 3,
					},
					{
						Name:  "Galeggtica",
						Value: 4,
					},
					{
						Name:  "Cornish-Hen Corvette",
						Value: 5,
					},
					{
						Name:  "Quintillion Chicken",
						Value: 6,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "chain",
				Description: "Show return time for a chained Henliner extended mission. [Sticky]",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "dubcap-time",
				Description: "Time remaining for double capacity event. Examples: `43:16:22` or `43h16m22s`",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "ftl",
				Description: "FTL Drive Upgrades level. Default is 60.",
				MinValue:    &integerZeroMinValue,
				MaxValue:    60,
				Required:    false,
			},
		},
	}
}

// HandleLaunchHelper handles the /launch-helper command
func HandleLaunchHelper(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var ftlLevel = 60
	var ftlMult = 0.4
	var t = time.Now()
	var arrivalTimespan = ""
	var chainExtended = false

	showDubCap := false
	doubleCapacityStr := ""
	var dubCapTime = time.Now()
	var dubCapTimeCaution = time.Now()

	var selectedShipPrimary = 0   // Default to AH
	var selectedShipSecondary = 1 // Default to H

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["primary-ship"]; ok {
		selectedShipPrimary = int(opt.IntValue())
		farmerstate.SetMissionShipPrimary(i.Member.User.ID, selectedShipPrimary)
	} else {
		selectedShipPrimary = farmerstate.GetMissionShipPrimary(i.Member.User.ID)
	}

	if opt, ok := optionMap["secondary-ship"]; ok {
		selectedShipSecondary = int(opt.IntValue())
		farmerstate.SetMissionShipSecondary(i.Member.User.ID, selectedShipSecondary)
	} else {
		selectedShipSecondary = farmerstate.GetMissionShipSecondary(i.Member.User.ID)
		if selectedShipSecondary == 0 {
			// This value should never be 0, so set to the default of 1
			selectedShipPrimary = 1
			farmerstate.SetMissionShipSecondary(i.Member.User.ID, selectedShipSecondary)
		}
	}

	var missionShips []shipData
	missionShips = append(missionShips, mis.Ships[selectedShipPrimary])
	if selectedShipSecondary != -1 && selectedShipPrimary != selectedShipSecondary {
		// append secondary mission
		switch selectedShipSecondary {
		case -2: // All Stars Club
			missionShips = append(missionShips, mis.Ships[1])
			missionShips = append(missionShips, mis.Ships[2])
			missionShips = append(missionShips, mis.Ships[3])
			missionShips = append(missionShips, mis.Ships[4])
			missionShips = append(missionShips, mis.Ships[5])
			missionShips = append(missionShips, mis.Ships[6])
		case -3: // Starfleet Commander
			missionShips = append(missionShips, mis.Ships[1], mis.Ships[2], mis.Ships[3])
		default:
			missionShips = append(missionShips, mis.Ships[selectedShipSecondary])
		}
	}

	if opt, ok := optionMap["ftl"]; ok {
		ftlLevel = int(opt.IntValue())
		ftlMult = float64(100-ftlLevel) / 100.0
	}
	if opt, ok := optionMap["chain"]; ok {
		chainExtended = opt.BoolValue()
		farmerstate.SetLaunchHistory(i.Member.User.ID, chainExtended)
	} else {
		chainExtended = farmerstate.GetLaunchHistory(i.Member.User.ID)
	}
	if opt, ok := optionMap["mission-duration"]; ok {
		// Timespan is when the next mission arrives
		arrivalTimespan = opt.StringValue()
		arrivalTimespan = strings.Replace(arrivalTimespan, "min", "m", -1)
		arrivalTimespan = strings.Replace(arrivalTimespan, "hr", "h", -1)
		arrivalTimespan = strings.Replace(arrivalTimespan, "sec", "s", -1)
	}
	if opt, ok := optionMap["dubcap-time"]; ok {
		// Timespan is when the next mission arrives
		// Time could be HH:MM:SS or 1h2m3s
		dcTimespan := opt.StringValue()
		// Does String contain a colon? then it's in HH:MM:SS format
		durDubCap, err := hhmmss.Parse(dcTimespan)
		if err != nil {
			dcTimespan = strings.Replace(dcTimespan, "day", "d", -1)
			dcTimespan = strings.Replace(dcTimespan, "hr", "h", -1)
			dcTimespan = strings.Replace(dcTimespan, "min", "m", -1)
			dcTimespan = strings.Replace(dcTimespan, "sec", "s", -1)
			durDubCap, _ = str2duration.ParseDuration(dcTimespan)
		}

		dubCapTime = t.Add(durDubCap)
		dubCapTimeCaution = dubCapTime.Add(-5 * time.Minute)

		showDubCap = true
		doubleCapacityStr = fmt.Sprintf("Double Capacity Event ends at <t:%d:f>\n", dubCapTime.Unix())
	}
	var builder strings.Builder
	shipDurationName := [...]string{"SH", "ST", "EX"}

	// Split array, trim to 3 elements
	durationList := strings.Split(arrivalTimespan, ",")
	if len(durationList) > 3 {
		durationList = durationList[:3]
	}

	ed, _ := str2duration.ParseDuration("4d")
	minutesStr := fmt.Sprintf("%dm", int(ed.Minutes()*ftlMult))
	exDuration, _ := str2duration.ParseDuration(minutesStr)

	for i, missionTimespanRaw := range durationList {
		missionTimespan := strings.TrimSpace(missionTimespanRaw)
		dur, err := str2duration.ParseDuration(missionTimespan)
		if err != nil {
			// Error during parsing means skip this duration
			continue
		}

		if i != 0 {
			builder.WriteString("\n")
		}

		arrivalTime := t.Add(dur)

		// loop through missionData
		// for each ship, calculate the arrival time
		// if arrival time is less than endTime, then add to the message
		builder.WriteString(fmt.Sprintf("**Launch options for mission arriving on <t:%d:f> (FTL:%d)**\n", arrivalTime.Unix(), ftlLevel))
		if showDubCap {
			builder.WriteString(doubleCapacityStr)
		}

		for shipIndex, ship := range missionShips {
			var sName = " " + ship.Name
			if shipIndex == 0 || len(missionShips) <= 2 {
				builder.WriteString("__" + ship.Name + "__:\n")
				sName = "" // Clear this out for single missions
			} else if shipIndex == 1 {
				if selectedShipSecondary == -2 {
					builder.WriteString("__All Stars Club__:\n")
				} else if selectedShipSecondary == -3 {
					builder.WriteString("__Starfleet Commander__:\n")
				}
			}

			for i, missionLen := range ship.Duration {
				dcBubble := ""
				d, _ := str2duration.ParseDuration(missionLen)

				minutesStr := fmt.Sprintf("%dm", int(d.Minutes()*ftlMult))
				ftlDuration, _ := str2duration.ParseDuration(minutesStr)

				launchTime := arrivalTime.Add(ftlDuration)
				if showDubCap {
					if launchTime.Before(dubCapTimeCaution) {
						dcBubble = "🟢 " // More than 5 min left in event
					} else if launchTime.Before(dubCapTime) {
						dcBubble = "🟡 " // Within 5 minutes
					} else {
						dcBubble = "🔴 "
					}
				}
				var chainString = ""
				if chainExtended {
					chainLaunchTime := launchTime.Add(exDuration)
					chainString = fmt.Sprintf(" +next EX return <t:%d:t>", chainLaunchTime.Unix())
				}

				builder.WriteString(fmt.Sprintf("> %s%s%s (%s): <t:%d:t>%s\n", dcBubble, shipDurationName[i], sName, fmtDuration(ftlDuration), launchTime.Unix(), chainString))
				if shipIndex != 0 && len(missionShips) > 2 && selectedShipSecondary < -1 {
					break
				}
			}
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    builder.String(),
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

}
