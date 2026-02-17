package events

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
	"github.com/xhit/go-str2duration/v2"
)

// SlashLaunchHelperCommand returns the command for the /launch-helper command
func SlashLaunchHelperCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Display timestamp table for next mission.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "mission-duration",
				Description: "Time remaining for next mission(s). Example: 8h15m",
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
			/*
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
				},*/
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "ftl",
				Description: "FTL Drive Upgrades level. Default is 60.",
				MinValue:    &integerZeroMinValue,
				MaxValue:    60,
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "ultra",
				Description: "Enable ultra event calculations. Default is false. [Sticky]",
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
	var chainExtended bool

	showDubCap := false
	doubleCapacityStr := ""
	var dubCapTime = time.Now()
	var dubCapTimeCaution = time.Now()
	var fasterMissions = 1.0

	var selectedShipPrimary int
	var selectedShipSecondary int

	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	var components []discordgo.MessageComponent

	optionMap := bottools.GetCommandOptionsMap(i)
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
			selectedShipPrimary = 1 // Default to Henterprise
			farmerstate.SetMissionShipSecondary(userID, selectedShipSecondary)
		}
	}

	var missionShips []ei.ShipData
	missionShips = append(missionShips, ei.MissionArt.Ships[len(ei.MissionArt.Ships)-selectedShipPrimary-1])
	if selectedShipSecondary != -1 && selectedShipPrimary != selectedShipSecondary {
		// append secondary mission
		switch selectedShipSecondary {
		case -2: // All Stars Club
			missionShips = append(missionShips, ei.MissionArt.Ships[len(ei.MissionArt.Ships)-1])
			missionShips = append(missionShips, ei.MissionArt.Ships[len(ei.MissionArt.Ships)-2])
			missionShips = append(missionShips, ei.MissionArt.Ships[len(ei.MissionArt.Ships)-3])
			missionShips = append(missionShips, ei.MissionArt.Ships[len(ei.MissionArt.Ships)-4])
			missionShips = append(missionShips, ei.MissionArt.Ships[len(ei.MissionArt.Ships)-5])
			missionShips = append(missionShips, ei.MissionArt.Ships[len(ei.MissionArt.Ships)-6])
			missionShips = append(missionShips, ei.MissionArt.Ships[len(ei.MissionArt.Ships)-7])
		case -3: // Starfleet Commander
			missionShips = append(missionShips, ei.MissionArt.Ships[len(ei.MissionArt.Ships)-8], ei.MissionArt.Ships[len(ei.MissionArt.Ships)-9], ei.MissionArt.Ships[len(ei.MissionArt.Ships)-10])
		default:
			missionShips = append(missionShips, ei.MissionArt.Ships[len(ei.MissionArt.Ships)-selectedShipSecondary-1])
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
	} else {
		chainExtended = farmerstate.GetLaunchHistory(userID)
	}
	if opt, ok := optionMap["mission-duration"]; ok {
		// Timespan is when the next mission arrives
		var list []string
		inputStr := opt.StringValue()
		inputList := strings.Split(inputStr, ",")
		for _, durationStr := range inputList {
			list = append(list, bottools.SanitizeStringDuration(durationStr))
		}
		arrivalTimespan = strings.Join(list, ",")
	}

	ultra := false
	if opt, ok := optionMap["ultra"]; ok {
		ultra = opt.BoolValue()
		farmerstate.SetMiscSettingFlag(userID, "ultra", ultra)
	} else {
		ultra = farmerstate.GetMiscSettingFlag(userID, "ultra")
	}
	/*
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
	*/
	fuel := getEventMultiplier("mission-fuel")
	fuelStr := ""
	if fuel != nil {
		uIcon := ei.GetBotEmojiMarkdown("std_fuel")
		if fuel.Ultra {
			uIcon = ei.GetBotEmojiMarkdown("ultra_fuel")
		}
		if !fuel.Ultra || (fuel.Ultra && ultra) {
			fuelStr = fmt.Sprintf("%s%s Ends <t:%d:R>\n", uIcon, fuel.Message, fuel.EndTime.Unix())
		} else {
			fuelStr = fmt.Sprintf("Ultra only : %s\n", fuel.Message)
		}
	}
	dupcapIcon := ""

	capacity := getEventMultiplier("mission-capacity")
	if capacity != nil {
		if !capacity.Ultra || (capacity.Ultra && ultra) {
			capIcon := ei.GetBotEmojiMarkdown("std_dubcap")
			if capacity.Ultra {
				capIcon = ei.GetBotEmojiMarkdown("ultra_dubcap")
			}
			dupcapIcon = capIcon
			showDubCap = true
			dubCapTime = capacity.EndTime
			dubCapTimeCaution = dubCapTime.Add(-5 * time.Minute)
			doubleCapacityStr = fmt.Sprintf("%s%s Ends <t:%d:R>\n", capIcon, capacity.Message, dubCapTime.Unix())
		} else {
			doubleCapacityStr = fmt.Sprintf("Ultra only : %s Not used in calculations.\n", capacity.Message)
		}
	}

	fastDuration := getEventMultiplier("mission-duration")
	durationStr := ""
	if fastDuration != nil {
		if !fastDuration.Ultra || (fastDuration.Ultra && ultra) {
			durIcon := ei.GetBotEmojiMarkdown("std_fast")
			if fastDuration.Ultra {
				durIcon = ei.GetBotEmojiMarkdown("ultra_fast")
			}
			durationStr = fmt.Sprintf("%s%s  Ends <t:%d:R>\n", durIcon, fastDuration.Message, fastDuration.EndTime.Unix())
			fasterMissions = fastDuration.Multiplier
		} else {
			durationStr = fmt.Sprintf("Ultra only : %s Not used in calculations.\n", fastDuration.Message)
		}

	}

	var events strings.Builder
	var builder strings.Builder
	//var header strings.Builder
	shipDurationName := [...]string{"SH", "ST", "EX"}
	var header strings.Builder

	// Split array, trim to 3 elements
	durationList := strings.Split(arrivalTimespan, ",")
	if len(durationList) > 3 {
		durationList = durationList[:3]
	}
	/*
		if len(missionShips) > 4 {
			// If more than 4 ships, limit to one duration
			durationList = durationList[:1]
		}
	*/
	displayDubcapInstructions := false
	displaySunInstructions := false

	if showDubCap {
		events.WriteString(doubleCapacityStr)
	}
	if fuelStr != "" {
		events.WriteString(fuelStr)
	}
	if durationStr != "" {
		events.WriteString(durationStr)
	}

	for _, missionTimespanRaw := range durationList {

		missionTimespan := strings.TrimSpace(missionTimespanRaw)
		dur, err := str2duration.ParseDuration(missionTimespan)
		if err != nil {
			// Error during parsing means skip this duration
			continue
		}

		arrivalTime := t.Add(dur)

		// loop through missionData
		// for each ship, calculate the arrival time
		// if arrival time is less than endTime, then add to the message

		fmt.Fprintf(&header, "## Mission arriving on <t:%d:f> (FTL:%d)\n", arrivalTime.Unix(), ftlLevel)

		for shipIndex, ship := range missionShips {
			var sName = " " + ship.Name
			var sArt = ei.GetBotEmojiMarkdown(ship.Art)
			if shipIndex == 0 || len(missionShips) <= 2 {
				builder.WriteString(sArt + " __" + ship.Name + "__:\n")
				sName = "" // Clear this out for single missions
				sArt = ""
			} else if shipIndex == 1 {
				switch selectedShipSecondary {
				case -2:
					builder.WriteString("__All Stars Club__:\n")
				case -3:
					builder.WriteString("__Starfleet Commander__:\n")
				}
			}

			for i, missionLen := range ship.Duration {
				dcBubble := ""
				sunBubble := ""
				d, _ := str2duration.ParseDuration(missionLen)

				minutesStr := fmt.Sprintf("%dm", int(d.Minutes()*ftlMult*fasterMissions))
				if fastDuration != nil && arrivalTime.After(fastDuration.EndTime) {
					minutesStr = fmt.Sprintf("%dm", int(d.Minutes()*ftlMult*1.0))
				}
				ftlDuration, _ := str2duration.ParseDuration(minutesStr)

				launchTime := arrivalTime.Add(ftlDuration)
				if showDubCap {
					if arrivalTime.Before(dubCapTimeCaution) {
						dcBubble = dupcapIcon + " " // More than 5 min left in event
					} else if arrivalTime.Before(dubCapTime) {
						dcBubble = dupcapIcon + "🟡 " // Within 5 minutes
					} else {
						dcBubble = ""
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

					exhenDuration, _ := str2duration.ParseDuration("4d")
					minutesStr := fmt.Sprintf("%dm", int(exhenDuration.Minutes()*ftlMult*fasterMissions))
					if fastDuration != nil && launchTime.After(fastDuration.EndTime) {
						minutesStr = fmt.Sprintf("%dm", int(exhenDuration.Minutes()*ftlMult*1.0))
					}
					exDuration, _ := str2duration.ParseDuration(minutesStr)

					chainLaunchTime := launchTime.Add(exDuration)
					if showDubCap && launchTime.Before(dubCapTimeCaution) {
						chainString = fmt.Sprintf(" +%sEX (%s) <t:%d:t>", dupcapIcon, bottools.FmtDuration(exDuration), chainLaunchTime.Unix())
					} else {
						chainString = fmt.Sprintf(" +EX (%s) <t:%d:t>", bottools.FmtDuration(exDuration), chainLaunchTime.Unix())
					}
					if fasterMissions != 1.0 {
						// Calculate an additional duration
						minutesStr := fmt.Sprintf("%dm", int(exhenDuration.Minutes()*ftlMult*fasterMissions))
						if fastDuration != nil && chainLaunchTime.After(fastDuration.EndTime) {
							minutesStr = fmt.Sprintf("%dm", int(exhenDuration.Minutes()*ftlMult*1.0))
						}
						exDuration, _ := str2duration.ParseDuration(minutesStr)

						chainLaunchTime = chainLaunchTime.Add(exDuration)
						chainString += fmt.Sprintf(" +EX (%s) <t:%d:t>", bottools.FmtDuration(exDuration), chainLaunchTime.Unix())
					}
				}

				fmt.Fprintf(&builder, "> %s%s%s%s%s (%s): <t:%d:t>%s\n", dcBubble, sunBubble, sArt, shipDurationName[i], sName, bottools.FmtDuration(ftlDuration), launchTime.Unix(), chainString)
				if shipIndex != 0 && len(missionShips) > 2 && selectedShipSecondary < -1 {
					break
				}
			}
		}
		if events.Len() > 0 {
			components = append(components, &discordgo.TextDisplay{
				Content: events.String(),
			})
		}

		components = append(components, &discordgo.TextDisplay{
			Content: header.String() + "\n" + builder.String(),
		})
		divider := true
		spacing := discordgo.SeparatorSpacingSizeLarge

		components = append(components, &discordgo.Separator{
			Divider: &divider,
			Spacing: &spacing,
		})
		header.Reset()
		builder.Reset()
	}

	var instr strings.Builder
	var prevEvents strings.Builder

	if displaySunInstructions {
		instr.WriteString("-# ⚠️ Launch before Sunday event will arrive after event begins\n")
	}
	if displayDubcapInstructions {
		//instr.WriteString(dupcapIcon + " Arrives during dubcap\n")
		instr.WriteString("-# 🟡 Arrives with less than 5 minutes of dubcap end\n")
		//instr.WriteString("🔴 After Dubcap ends\n")
	}
	if chainExtended {
		instr.WriteString("-# Chained EX launches are Henliner/Henerprise extended missions\n")
	}

	for _, e := range ei.LastMissionEvent {
		eventIconStr := ""
		switch e.EventType {
		case "mission-duration":
			eventIconStr = ei.GetBotEmojiMarkdown("std_fast")
		case "mission-capacity":
			eventIconStr = ei.GetBotEmojiMarkdown("std_dubcap")
		case "mission-fuel":
			eventIconStr = ei.GetBotEmojiMarkdown("std_fuel")
		}

		if e.Ultra {
			if !ultra {
				continue
			}
			switch e.EventType {
			case "mission-duration":
				eventIconStr = ei.GetBotEmojiMarkdown("ultra_fast")
			case "mission-capacity":
				eventIconStr = ei.GetBotEmojiMarkdown("ultra_dubcap")
			case "mission-fuel":
				eventIconStr = ei.GetBotEmojiMarkdown("ultra_fuel")
			}
		}
		hours := e.EndTime.Sub(e.StartTime).Hours()
		fmt.Fprintf(&prevEvents, "%s %s for %.2dh on <t:%d:R>\n", eventIconStr, e.Message, int(hours), e.StartTime.Unix())
		//prevEvents.WriteString(fmt.Sprintf("%s on <t:%d:d>\n", e.Message, e.StartTime.Unix()))
	}
	components = append(components, &discordgo.TextDisplay{
		Content: "## Event History\n" + prevEvents.String(),
	})
	divider := false
	spacing := discordgo.SeparatorSpacingSizeSmall

	components = append(components, &discordgo.Separator{
		Divider: &divider,
		Spacing: &spacing,
	})
	if instr.Len() > 0 {
		components = append(components, &discordgo.TextDisplay{
			Content: instr.String(),
		})
	}

	_, err := s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		})
	if err != nil {
		log.Println("Error sending followup message:", err)
		return
	}
}
