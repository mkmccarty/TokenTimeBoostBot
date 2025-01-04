package boost

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/olekukonko/tablewriter"
	"google.golang.org/protobuf/proto"
)

// GetSlashTeamworkEval will return the discord command for calculating token values of a running contract
func GetSlashTeamworkEval(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Evaluate teamwork values a contract",
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
				Name:        "egginc-ign",
				Description: "Egg Inc, in game name to evaluate.",
				Required:    false,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Select a contract-id",
				Required:     false,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "coop-id",
				Description: "Your coop-id",
				Required:    false,
			},
		},
	}
}

// HandleTeamworkEvalCommand will handle the /teamwork-eval command
func HandleTeamworkEvalCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var builder strings.Builder

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}
	var contractID string
	var coopID string
	var eggign string
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["egginc-ign"]; ok {
		eggign = opt.StringValue()
	} else {
		name := farmerstate.GetMiscSettingString(userID, "EggIncRawName")
		if name != "" {
			eggign = name
		}
	}

	if opt, ok := optionMap["contract-id"]; ok {
		contractID = strings.ToLower(opt.StringValue())
		contractID = strings.Replace(contractID, " ", "", -1)
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = strings.ToLower(opt.StringValue())
		coopID = strings.Replace(coopID, " ", "", -1)
	}
	// Unser contractID and coopID means we want the Boost Bot contract
	if contractID == "" || coopID == "" {
		contract := FindContract(i.ChannelID)
		if contract == nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: "No contract found in this channel. Please provide a contract-id and coop-id.",
				})

			return
		}
		contractID = strings.ToLower(contract.ContractID)
		coopID = strings.ToLower(contract.CoopID)
	}
	var embed *discordgo.MessageSend

	if config.EIUserID != "" {
		var str string
		str, embed = DownloadCoopStatus(userID, eggign, contractID, coopID)
		builder.WriteString(str)
	} else {
		builder.WriteString("This command is missing a configuration option necessary to function.")
	}

	if embed == nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: builder.String(),
			})
		return
	}
	_, err := s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: builder.String(),
			Embeds:  embed.Embeds,
		})
	if err != nil {
		log.Print(err)
	}

}

// DownloadCoopStatus will download the coop status for a given contract and coop ID
func DownloadCoopStatus(userID string, einame string, contractID string, coopID string) (string, *discordgo.MessageSend) {
	eggIncID := config.EIUserID
	reqURL := "https://www.auxbrain.com/ei/coop_status"
	enc := base64.StdEncoding

	var protoData string
	var dataTimestampStr string

	eiContract := ei.EggIncContractsAll[contractID]
	if eiContract.ID == "" {
		return "Invalid contract ID.", nil
	}

	cacheID := contractID + ":" + coopID
	cachedData := eiDatas[cacheID]
	var nowTime time.Time

	var field []*discordgo.MessageEmbedField

	// Check if the file exists
	if cachedData != nil && time.Now().Before(cachedData.expirationTimestamp) {
		protoData = cachedData.protoData
		dataTimestampStr = fmt.Sprintf("Using cached data, within %d second cooldown.", 60)
		nowTime = cachedData.timestamp
		// Use protoData as needed
	} else {
		coopID := strings.ToLower(coopID)
		contractID := strings.ToLower(contractID)

		coopStatusRequest := ei.ContractCoopStatusRequest{
			ContractIdentifier: &contractID,
			CoopIdentifier:     &coopID,
			UserId:             &eggIncID,
		}
		reqBin, err := proto.Marshal(&coopStatusRequest)
		if err != nil {
			return err.Error(), nil
		}
		reqDataEncoded := enc.EncodeToString(reqBin)

		response, err := http.PostForm(reqURL, url.Values{"data": {reqDataEncoded}})

		if err != nil {
			log.Print(err)
			return err.Error(), nil
		}

		defer response.Body.Close()

		// Read the response body
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Print(err)
			return err.Error(), nil
		}
		dataTimestampStr = ""
		protoData = string(body)
		data := eiData{ID: cacheID, timestamp: time.Now(), expirationTimestamp: time.Now().Add(1 * time.Minute), contractID: contractID, coopID: coopID, protoData: protoData}
		eiDatas[cacheID] = &data
		nowTime = time.Now()
	}

	decodedAuthBuf := &ei.AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(protoData)
	err := proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		return err.Error(), nil
	}

	decodeCoopStatus := &ei.ContractCoopStatusResponse{}
	err = proto.Unmarshal(decodedAuthBuf.Message, decodeCoopStatus)
	if err != nil {
		log.Print(err)
		return err.Error(), nil
	}

	type BuffTimeValue struct {
		name            string
		earnings        int
		earningsCalc    float64
		eggRate         int
		eggRateCalc     float64
		timeEquiped     int64
		durationEquiped int64
		buffTimeValue   float64
		tb              int64
		totalValue      float64
	}

	var BuffTimeValues []BuffTimeValue
	var contractDurationSeconds float64
	var calcSecondsRemaining int64

	//prevServerTimestamp = int64(decodeCoopStatus.GetSecondsRemaining()) + BuffTimeValues[0].timeEquiped
	// If the coop completed, the secondsSinceAllGoalsAchieved (towards the end) is present.
	// If coop isn't complete, you have to back calculate from secondsRemaining,
	// and estimate completion time based off rates

	/*
		Start time can be found via:
		Date.now() + secondsRemaining - contract.gradeSpecs[(grade)].lengthSeconds
		End time can be found via:
		Date.now() - secondsSinceAllGoalsAchieved
		Then use day.js to generate timespan and then create time string
	*/
	var builder strings.Builder
	var teamwork strings.Builder

	grade := int(decodeCoopStatus.GetGrade())

	startTime := nowTime
	secondsRemaining := int64(decodeCoopStatus.GetSecondsRemaining())
	endTime := nowTime
	if decodeCoopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
		secondsSinceAllGoals := int64(decodeCoopStatus.GetSecondsSinceAllGoalsAchieved())
		endTime = endTime.Add(-time.Duration(secondsSinceAllGoals) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
		builder.WriteString(fmt.Sprintf("Completed %s contract **%s/%s**\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), contractID, coopID))
		builder.WriteString(fmt.Sprintf("Start Time: <t:%d:f>\n", startTime.Unix()))
		builder.WriteString(fmt.Sprintf("End Time: <t:%d:f>\n", endTime.Unix()))
		builder.WriteString(fmt.Sprintf("Duration: %v\n", (endTime.Sub(startTime)).Round(time.Second)))

	} else {
		var totalContributions float64
		var contributionRatePerSecond float64
		// Need to figure out the
		for _, c := range decodeCoopStatus.GetContributors() {
			totalContributions += c.GetContributionAmount()
			totalContributions += -(c.GetContributionRate() * c.GetFarmInfo().GetTimestamp()) // offline eggs
			contributionRatePerSecond += c.GetContributionRate()
		}
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
		totalReq := eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1]
		calcSecondsRemaining = int64((totalReq - totalContributions) / contributionRatePerSecond)
		endTime = nowTime.Add(time.Duration(calcSecondsRemaining) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
		builder.WriteString(fmt.Sprintf("In Progress %s **%s/%s** on target to complete <t:%d:R>\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), contractID, coopID, endTime.Unix()))
		builder.WriteString(fmt.Sprintf("Start Time: <t:%d:f>\n", startTime.Unix()))
		builder.WriteString(fmt.Sprintf("Est. End Time: <t:%d:f>\n", endTime.Unix()))
		builder.WriteString(fmt.Sprintf("Est. Duration: %v\n", (endTime.Sub(startTime)).Round(time.Second)))
	}
	if einame != "" {
		builder.WriteString(fmt.Sprintf("Evaluating data for **%s**\n", einame))
	}

	// Take care of other parameter calculations here

	// Chicken Runs
	/*
		contractDurationInDays := int(contractDurationSeconds / 86400.0)
		fCR := max(12.0/float64(contract.CoopSize*contractDurationInDays), 0.3)
		CR := min(fCR, 6.0)

		// Token Values
		BTA := contractDurationSeconds / float64(contract.MinutesPerToken)
		T := 0.0
		if BTA <= 42.0 {
			T = (2.0 / 3.0 * (3.0)) + ((8.0 / 3.0) * min(tval, 3.0))
		} else {
			T = (200.0/(7.0*BTA))*(0.07*BTA) + (800.0 / (7.0 * BTA) * min(tval, 0.07*BTA))
		}
	*/

	found := false
	var teamworkNames []string

	for _, c := range decodeCoopStatus.GetContributors() {

		name := c.GetUserName()

		if einame != name {
			teamworkNames = append(teamworkNames, name)
			continue
		}
		found = true

		for _, a := range c.GetBuffHistory() {
			earnings := int(math.Round(a.GetEarnings()*100 - 100))
			eggRate := int(math.Round(a.GetEggLayingRate()*100 - 100))
			serverTimestamp := int64(a.GetServerTimestamp()) // When it was equipped
			if decodeCoopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
				serverTimestamp -= int64(decodeCoopStatus.GetSecondsSinceAllGoalsAchieved())
			} else {
				serverTimestamp += calcSecondsRemaining
			}
			serverTimestamp = int64(contractDurationSeconds) - serverTimestamp
			BuffTimeValues = append(BuffTimeValues, BuffTimeValue{name, earnings, 0.0075 * float64(earnings), eggRate, 0.0075 * float64(eggRate) * 10.0, serverTimestamp, 0, 0, 0, 0})
		}
	}

	remainingTime := contractDurationSeconds
	for i, b := range BuffTimeValues {
		if i == len(BuffTimeValues)-1 {
			BuffTimeValues[i].durationEquiped = int64(contractDurationSeconds) - b.timeEquiped
		} else {
			BuffTimeValues[i].durationEquiped = BuffTimeValues[i+1].timeEquiped - b.timeEquiped
		}
		remainingTime -= float64(BuffTimeValues[i].durationEquiped)
	}

	if len(BuffTimeValues) == 0 {
		teamwork.WriteString("**No buffs found for this contract.**\n")
	} else {

		table := tablewriter.NewWriter(&teamwork)
		//table.SetHeader([]string{"Time", "Duration", "Defl", "SIAB", "BTV-Defl", "BTV-SIAB ", "Buff Val", "TeamWork"})
		table.SetHeader([]string{"Time", "Duration", "Defl", "SIAB", "BTV", "TeamWork"})
		table.SetBorder(false)
		//table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_RIGHT)
		//if len(BuffTimeValues) > 10 {
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetTablePadding(" ") // pad with tabs
		table.SetNoWhiteSpace(true)
		table.SetBorders(tablewriter.Border{Left: false, Top: false, Right: false, Bottom: false})
		//}
		//table.SetTablePadding(" ") // pad with tabs
		//table.SetNoWhiteSpace(true)

		BestSIAB := 0.0
		LastSIABCalc := 0.0
		var MostRecentDuration time.Time
		var buffTimeValue float64
		for _, b := range BuffTimeValues {
			if b.durationEquiped < 0 {
				b.durationEquiped = 0
			}

			b.buffTimeValue = float64(b.durationEquiped)*b.earningsCalc + float64(b.durationEquiped)*b.eggRateCalc
			B := min(b.buffTimeValue/contractDurationSeconds, 2)
			CR := min(0.0, 6.0)
			T := 0.0
			teamworkScore := ((5.0 * B) + CR + T) / 19.0

			dur := time.Duration(b.durationEquiped) * time.Second
			when := time.Duration(b.timeEquiped) * time.Second
			MostRecentDuration = startTime.Add(when)

			if b.earnings > int(BestSIAB) {
				BestSIAB = b.earningsCalc
			}
			LastSIABCalc = float64(b.durationEquiped) * b.earningsCalc

			//table.Append([]string{fmt.Sprintf("%v", when.Round(time.Second)), fmt.Sprintf("%v", dur.Round(time.Second)), fmt.Sprintf("%d%%", b.eggRate), fmt.Sprintf("%d%%", b.earnings), fmt.Sprintf("%8.2f", float64(b.durationEquiped)*b.eggRateCalc), fmt.Sprintf("%8.2f", float64(b.durationEquiped)*b.earningsCalc), fmt.Sprintf("%8.2f", b.buffTimeValue), fmt.Sprintf("%1.6f", teamworkScore)})
			table.Append([]string{fmt.Sprintf("%v", when.Round(time.Second)), fmt.Sprintf("%v", dur.Round(time.Second)), fmt.Sprintf("%d%%", b.eggRate), fmt.Sprintf("%d%%", b.earnings), fmt.Sprintf("%6.0f", b.buffTimeValue), fmt.Sprintf("%1.6f", teamworkScore)})
			//table.Append([]string{fmt.Sprintf("%v", when.Round(time.Second)), fmt.Sprintf("%v", dur.Round(time.Second)), fmt.Sprintf("%d%%", b.eggRate), fmt.Sprintf("%d%%", b.earnings), fmt.Sprintf("%8.2f", float64(b.durationEquiped)*b.eggRateCalc), fmt.Sprintf("%8.2f", float64(b.durationEquiped)*b.earningsCalc), fmt.Sprintf("%8.2f", b.buffTimeValue), fmt.Sprintf("%1.6f", teamworkScore)})
			buffTimeValue += b.buffTimeValue
		}

		//completionTime :=

		uncappedBuffTimeValue := buffTimeValue / contractDurationSeconds
		B := min(uncappedBuffTimeValue, 2.0)
		CR := min(0.0, 6.0)
		T := 0.0

		// If SIAB still equipped, subtract that time from the total
		shortTeamwork := (contractDurationSeconds * 2.0) - (buffTimeValue - LastSIABCalc)
		siabSecondsNeeded := shortTeamwork / BestSIAB
		// SIABTimeEquipped * 0.75 * SIABPercentage/100
		// make siabSecondsNeeded a time.Duration
		siabTimeEquipped := time.Duration(siabSecondsNeeded) * time.Second

		TeamworkScore := ((5.0 * B) + (CR * 0) + (T * 0)) / 19.0
		table.Append([]string{"", "", "", "", fmt.Sprintf("%6.0f", buffTimeValue), fmt.Sprintf("%1.6f", TeamworkScore)})
		log.Printf("Teamwork Score: %f\n", TeamworkScore)

		table.Render()
		// If the length of teamwork is > 1500 then split it into two fields. Find a linefeed nearest to 1500
		teamworkStr := teamwork.String()
		if len(teamworkStr) > 1020 {
			i := 0
			for len(teamworkStr) > 0 {
				i++
				chunkSize := 1022
				if len(teamworkStr) < chunkSize {
					chunkSize = len(teamworkStr)
				} else {
					splitIndex := strings.LastIndex(teamworkStr[:chunkSize], "\n")
					if splitIndex != -1 {
						chunkSize = splitIndex
					}
				}
				field = append(field, &discordgo.MessageEmbedField{
					Name:   fmt.Sprintf("Teamwork-%d", i),
					Value:  "```" + teamworkStr[:chunkSize] + "```",
					Inline: false,
				})
				teamworkStr = teamworkStr[chunkSize:]
			}
		} else {
			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Teamwork",
				Value:  "```" + teamworkStr + "```",
				Inline: false,
			})
		}
		if BestSIAB > 0 && decodeCoopStatus.GetSecondsSinceAllGoalsAchieved() <= 0 {
			var maxTeamwork strings.Builder
			if LastSIABCalc != 0 {
				maxTeamwork.WriteString(fmt.Sprintf("Equip SIAB for %s (<t:%d:t>) in the most recent teamwork segment to max BTV by %6.0f.\n", fmtDuration(siabTimeEquipped), MostRecentDuration.Add(siabTimeEquipped).Unix(), shortTeamwork))
			} else {
				if time.Now().Add(siabTimeEquipped).After(endTime) {
					// How much longer is this siabTimeEquipped than the end of the contract
					extraTime := time.Now().Add(siabTimeEquipped).Sub(endTime)

					// Calculate the shortTeamwork reducing the extra time from the siabTimeEquipped
					extraPercent := (siabTimeEquipped - extraTime).Seconds() / siabTimeEquipped.Seconds()

					maxTeamwork.WriteString(fmt.Sprintf("Equip SIAB through end of contract (<t:%d:t>) in new teamwork segment to improve BTV by %6.0f. ", endTime.Unix(), shortTeamwork*extraPercent))
					maxTeamwork.WriteString(fmt.Sprintf("The maximum BTV increase of %6.0f would be achieved if the contract finished at <t:%d:f>.", shortTeamwork, time.Now().Add(siabTimeEquipped).Unix()))
				} else {
					maxTeamwork.WriteString(fmt.Sprintf("Equip SIAB for %s (<t:%d:t>) in new teamwork segment to max BTV by %6.0f.\n", fmtDuration(siabTimeEquipped), time.Now().Add(siabTimeEquipped).Unix(), shortTeamwork))
				}
			}
			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Maximize Teamwork",
				Value:  maxTeamwork.String(),
				Inline: false,
			})
		}

		if len(BuffTimeValues) <= 10 {
			var chickenRun strings.Builder
			// Calculate the ChickenRun score
			tableCR := tablewriter.NewWriter(&chickenRun)
			tableCR.ClearRows()

			tableCR.SetHeader([]string{"1 Run", "N*Runs", fmt.Sprintf("%d Runs", eiContract.ChickenRuns)})
			tableCR.SetBorder(false)
			tableCR.SetAlignment(tablewriter.ALIGN_RIGHT)

			fCR := max(12.0/float64(eiContract.MaxCoopSize*eiContract.ContractDurationInDays), 0.3)
			//for i := range eiContract.ChickenRuns {
			CR = min(fCR*float64(1), 6.0)
			CRTeamworkScore := ((5.0 * B * 0) + CR + (T * 0)) / 19.0
			CRMax := min(fCR*float64(eiContract.ChickenRuns), 6.0)
			CRMaxTeamworkScore := ((5.0 * B * 0) + CRMax + (T * 0)) / 19.0
			tableCR.Append([]string{
				fmt.Sprintf("%1.6f", CRTeamworkScore),
				fmt.Sprintf("N * %1.6f", CRTeamworkScore),
				fmt.Sprintf("%1.6f", CRMaxTeamworkScore)})
			//}
			chickenRun.WriteString("```")
			tableCR.Render()
			chickenRun.WriteString("```")
			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Chicken Run",
				Value:  chickenRun.String(),
				Inline: false,
			})

		}
		/*
			if builder.Len() < 1500 {

				tableT := tablewriter.NewWriter(&builder)
				tableT.ClearRows()

				BTA := (contractDurationSeconds / 60) / float64(eiContract.MinutesPerToken)

				tableT.SetHeader([]string{"△ TVAL", "sent 0 tval", "2.5 tval", "3 tval"})
				tableT.SetBorder(false)
				tableT.SetAlignment(tablewriter.ALIGN_RIGHT)

				for tval := -3.0; tval <= 3.0; tval += 1.0 {
					var items []string
					items = append(items, fmt.Sprintf("%1.1f", tval))
					for tSent := 0.0; tSent <= 3.0; tSent += 1.5 {
						if BTA <= 42.0 {
							T = (2.0 / 3.0 * min(tSent, 3.0)) + ((8.0 / 3.0) * min(tval, 3.0))
						} else {
							T = (200.0/(7.0*BTA))*min(tSent, 0.07*BTA) + (800.0 / (7.0 * BTA) * min(tval, 0.07*BTA))
						}
						TTeamworkScore := ((5.0 * B * 0) + (CR * 0) + T) / 19.0
						if tSent < tval {
							items = append(items, "")
						} else {
							items = append(items, fmt.Sprintf("%1.4f", TTeamworkScore))
						}
						//		tableT.Append([]string{fmt.Sprintf("%1.1f", tval), fmt.Sprintf("%1.6f", tSent), fmt.Sprintf("%1.6f", TTeamworkScore)})
					}
					tableT.Append(items)
				}
				//builder.Reset()
				builder.WriteString("```")
				tableT.Render()
				builder.WriteString("```")
			}
		*/

		log.Printf("\n%s", builder.String())

		log.Print("Buff Time Value: ", buffTimeValue)
	}

	if !found {
		var notes strings.Builder

		// Write to builder a message about using /seteiname to associate your discorn name with your Eggs IGN
		// create string of teamworkNames
		teamworkNamesStr := strings.Join(teamworkNames, ", ")
		notes.WriteString("\n")
		notes.WriteString("Your discord name must be different from your EggInc IGN.\n")
		notes.WriteString("Use **/seteggincname** to make this association.\n\n")
		notes.WriteString(fmt.Sprintf("Farmers in this contract are:\n> %s", teamworkNamesStr))
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Name not Found",
			Value:  notes.String(),
			Inline: false,
		})
	}

	embed := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Type:        discordgo.EmbedTypeRich,
			Title:       "Teamwork Evaluation",
			Description: "",
			Color:       0xffaa00,
			Fields:      field,
			Timestamp:   nowTime.Format(time.RFC3339),
			Footer: &discordgo.MessageEmbedFooter{
				Text: dataTimestampStr,
			},
		},
		},
	}

	return builder.String(), embed
}

func fmtDuration(d time.Duration) string {
	str := ""
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d = h / 24
	h -= d * 24

	if d > 0 {
		str = fmt.Sprintf("%dd%dh%dm", d, h, m)
	} else {
		str = fmt.Sprintf("%dh%dm", h, m)
	}
	return strings.Replace(str, "0h0m", "", -1)
}
