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
				Name:        "fast-missions",
				Description: "Missions return 2x, 3x or 4x faster. Default is 1x.",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "1x / None",
						Value: 1,
					},
					{
						Name:  "2x",
						Value: 2,
					},
					{
						Name:  "3x",
						Value: 3,
					},
					{
						Name:  "4x",
						Value: 4,
					},
				},
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
	var fasterMissions = 1.0

	var selectedShipPrimary = 0   // Default to AH
	var selectedShipSecondary = 1 // Default to H

	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	var field []*discordgo.MessageEmbedField

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["primary-ship"]; ok {
		selectedShipPrimary = int(opt.IntValue())
		farmerstate.SetMissionShipPrimary(userID, selectedShipPrimary)
	} else {
		selectedShipPrimary = farmerstate.GetMissionShipPrimary(userID)
	}

	if opt, ok := optionMap["secondary-ship"]; ok {
		selectedShipSecondary = int(opt.IntValue())
		farmerstate.SetMissionShipSecondary(userID, selectedShipSecondary)
	} else {
		selectedShipSecondary = farmerstate.GetMissionShipSecondary(userID)
		if selectedShipSecondary == 0 {
			// This value should never be 0, so set to the default of 1
			selectedShipPrimary = 1
			farmerstate.SetMissionShipSecondary(userID, selectedShipSecondary)
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

	pacificLoc, _ := time.LoadLocation("America/Los_Angeles")
	// Based on current time, when will it be UTC Sunday at 3pm, standard time
	var sundayEventStart = time.Now()
	// change eventStart to be the next Sunday at 3pm == 9AM PST
	for sundayEventStart.Weekday() != time.Sunday {
		dayDiff := 7 - int(sundayEventStart.Weekday())
		sundayEventStart = sundayEventStart.AddDate(0, 0, dayDiff)
	}
	sundayEventStart = time.Date(sundayEventStart.Year(), sundayEventStart.Month(), sundayEventStart.Day(), 17, 0, 0, 0, time.UTC)
	utc := sundayEventStart.UTC()
	if utc.In(pacificLoc).IsDST() {
		sundayEventStart = sundayEventStart.Add(-1 * time.Hour)
	}
	//log.Print("Sunday Event Start: ", sundayEventStart.Unix())

	if opt, ok := optionMap["ftl"]; ok {
		ftlLevel = int(opt.IntValue())
		ftlMult = float64(100-ftlLevel) / 100.0
	}
	if opt, ok := optionMap["chain"]; ok {
		chainExtended = opt.BoolValue()
		farmerstate.SetLaunchHistory(userID, chainExtended)
		chainExtended = farmerstate.GetLaunchHistory(userID)
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
	if opt, ok := optionMap["fast-missions"]; ok {
		switch opt.IntValue() {
		case 2:
			fasterMissions = 0.5
		case 3:
			fasterMissions = 0.333
		case 4:
			fasterMissions = 0.25
		default:
			fasterMissions = 1.0
		}

	}
	var builder strings.Builder
	//var header strings.Builder
	shipDurationName := [...]string{"SH", "ST", "EX"}

	// Split array, trim to 3 elements
	durationList := strings.Split(arrivalTimespan, ",")
	if len(durationList) > 3 {
		durationList = durationList[:3]
	}

	ed, _ := str2duration.ParseDuration("4d")
	minutesStr := fmt.Sprintf("%dm", int(ed.Minutes()*ftlMult*fasterMissions))
	exDuration, _ := str2duration.ParseDuration(minutesStr)
	displayDubcapInstructions := false
	displaySunInstructions := false

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
		builder.WriteString(fmt.Sprintf("**Mission arriving on <t:%d:f> (FTL:%d)**\n", arrivalTime.Unix(), ftlLevel))
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
				sunBubble := ""
				d, _ := str2duration.ParseDuration(missionLen)

				minutesStr := fmt.Sprintf("%dm", int(d.Minutes()*ftlMult*fasterMissions))
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
				if launchTime.After(sundayEventStart) && launchTime.Before(sundayEventStart.Add(ftlDuration)) {
					sunBubble = "⚠️ "
				}

				if dcBubble != "" {
					displayDubcapInstructions = true
				}
				if sunBubble != "" {
					displaySunInstructions = true
				}

				var chainString = ""
				if chainExtended {
					chainLaunchTime := launchTime.Add(exDuration)
					chainString = fmt.Sprintf(" +next EX return <t:%d:t>", chainLaunchTime.Unix())
					if fasterMissions != 1.0 {
						chainLaunchTime := launchTime.Add(exDuration * 2)
						chainString += fmt.Sprintf(" +return <t:%d:t>", chainLaunchTime.Unix())
					}
				}

				builder.WriteString(fmt.Sprintf("> %s%s%s%s (%s): <t:%d:t>%s\n", dcBubble, sunBubble, shipDurationName[i], sName, fmtDuration(ftlDuration), launchTime.Unix(), chainString))
				if shipIndex != 0 && len(missionShips) > 2 && selectedShipSecondary < -1 {
					break
				}
			}
		}
		/*
			field = append(field, &discordgo.MessageEmbedField{
				Name:   header.String(),
				Value:  builder.String(),
				Inline: false,
			})*/
		//header.Reset()
		//builder.Reset()
	}
	var instr strings.Builder
	if displaySunInstructions {
		instr.WriteString("⚠️ Arrives after Sunday event\n")
	}
	if displayDubcapInstructions {
		instr.WriteString("🟢 Arrives within dubcap.\n")
		instr.WriteString("🟡 Arrives with less than 5 minutes of dubcap\n")
		instr.WriteString("🔴 After Dubcap\n")
	}

	s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: builder.String(),
			Embeds: []*discordgo.MessageEmbed{{
				Type: discordgo.EmbedTypeRich,
				//Title: "Mission Arrival Times",
				//Description: "",
				Color:  0xffaa00,
				Fields: field,
				Footer: &discordgo.MessageEmbedFooter{
					Text: instr.String(),
				},
			}},
		})

}
