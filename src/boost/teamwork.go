package boost

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/olekukonko/tablewriter"
)

// GetSlashTeamworkEval will return the discord command for calculating token values of a running contract
func GetSlashTeamworkEval(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Evaluate teamwork values in a contract",
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
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Select a contract-id",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "coop-id",
				Description: "Your coop-id",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "egginc-ign",
				Description: "Egg Inc, in game name to evaluate.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "show-scores",
				Description: "Show Contract Scores only. Default is false. (sticky)",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "public-reply",
				Description: "Respond publicly. Default is false.",
				Required:    false,
			},
		},
	}
}

// HandleTeamworkEvalCommand will handle the /teamwork command
func HandleTeamworkEvalCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {

	publicReply := false
	flags := discordgo.MessageFlagsEphemeral

	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}
	var contractID string
	var coopID string
	var eggign string
	scoresFirst := false
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["egginc-ign"]; ok {
		eggign = strings.ToLower(opt.StringValue())
	} else {
		name := farmerstate.GetMiscSettingString(userID, "EggIncRawName")
		if name != "" {
			eggign = strings.ToLower(name)
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
	if opt, ok := optionMap["public-reply"]; ok {
		publicReply = !opt.BoolValue()
		if opt.BoolValue() {
			flags &= ^discordgo.MessageFlagsEphemeral
		}
	}
	if opt, ok := optionMap["show-scores"]; ok {
		// If show-scores is true, then we want to show the scores only
		scoresFirst = opt.BoolValue()
		farmerstate.SetMiscSettingFlag(userID, "teamwork-scores", scoresFirst)
	} else {
		scoresFirst = farmerstate.GetMiscSettingFlag(userID, "teamwork-scores")
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

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

	var str string
	str, fields := DownloadCoopStatusTeamwork(contractID, coopID)

	cache := buildTeamworkCache(str, fields)
	// Fill in our calling parameters
	cache.contractID = contractID
	cache.coopID = coopID
	cache.public = publicReply
	cache.showScores = scoresFirst
	if eggign != "" {
		for idx, name := range cache.names {
			if strings.EqualFold(name, eggign) {
				cache.page = idx
				break
			}
		}
	}

	teamworkCacheMap[cache.xid] = cache

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})

	sendTeamworkPage(s, i, true, cache.xid, false, false, false)

	// Traverse stonesCacheMap and delete expired entries
	for key, cache := range teamworkCacheMap {
		if cache.expirationTimestamp.Before(time.Now()) {
			delete(teamworkCacheMap, key)
		}
	}

}

// DownloadCoopStatusTeamwork will download the coop status for a given contract and coop ID
func DownloadCoopStatusTeamwork(contractID string, coopID string) (string, map[string][]*discordgo.MessageEmbedField) {
	var siabMsg strings.Builder
	siabMsgCount := 0
	var dataTimestampStr string
	nowTime := time.Now()

	eiContract := ei.EggIncContractsAll[contractID]
	if eiContract.ID == "" {
		return "Invalid contract ID.", nil
	}

	coopStatus, _, dataTimestampStr, err := ei.GetCoopStatus(contractID, coopID)
	if err != nil {
		return err.Error(), nil
	}

	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		return ei.ContractCoopStatusResponse_ResponseStatus_name[int32(coopStatus.GetResponseStatus())], nil
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

	grade := int(coopStatus.GetGrade())

	// Want estimated contribution ratio for entire contract
	contribution := make([]float64, len(coopStatus.GetContributors()))
	contractDurationInDays := int(float64(eiContract.Grade[grade].LengthInSeconds) / 86400.0)

	prefix := ""
	startTime := nowTime
	secondsRemaining := int64(coopStatus.GetSecondsRemaining())
	endTime := nowTime
	if coopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
		secondsSinceAllGoals := int64(coopStatus.GetSecondsSinceAllGoalsAchieved())
		endTime = endTime.Add(-time.Duration(secondsSinceAllGoals) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
		builder.WriteString(fmt.Sprintf("Completed %s contract %s/[**%s**](%s)\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), contractID, coopID, fmt.Sprintf("%s/%s/%s", "https://eicoop-carpet.netlify.app", contractID, coopID)))
	} else {
		prefix = "Est. "
		var totalContributions float64
		var contributionRatePerSecond float64
		// Need to figure out how much longer this contract will run
		for _, c := range coopStatus.GetContributors() {
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
		builder.WriteString(fmt.Sprintf("In Progress %s %s/[**%s**](%s) on target to complete <t:%d:R>\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), contractID, coopID, fmt.Sprintf("%s/%s/%s", "https://eicoop-carpet.netlify.app", contractID, coopID), endTime.Unix()))
	}
	builder.WriteString(fmt.Sprintf("Start Time: <t:%d:f>\n", startTime.Unix()))
	builder.WriteString(fmt.Sprintf("%sEnd Time: <t:%d:f>\n", prefix, endTime.Unix()))
	builder.WriteString(fmt.Sprintf("%sDuration: %v\n", prefix, (endTime.Sub(startTime)).Round(time.Second)))

	siabMsg.WriteString("Showing those with SIAB equipped and can swap it out before the end of the contract without losing teamwork score.\n")

	/*

		// Want to retrieve this so we can adjust duration for SIAB strategy
		type Production struct {
			ELR       float64
			SR        float64
			Delivered float64
			SIAB      bool
		}
		farmerProduction := make(map[int]Production)

		for i, c := range coopStatus.GetContributors() {
			p := c.GetProductionParams()
			PP := Production{ELR: p.GetElr(), SR: p.GetSr(), Delivered: p.GetDelivered(), SIAB: false}
			// Determine if SIAB is equipped in last segment
			bh := c.GetBuffHistory()
			if len(bh) > 0 {
				// If the last buff is SIAB, then we need to adjust the duration
				if bh[len(bh)-1].GetEarnings() != 0.0 {
					PP.SIAB = true
				}
			}
			// If the last buff is SIAB, then we need to adjust the duration
			farmerProduction[i] = PP
		}
	*/

	// Used to collect the return values for each farmer
	var farmerFields = make(map[string][]*discordgo.MessageEmbedField)

	for i, c := range coopStatus.GetContributors() {
		var field []*discordgo.MessageEmbedField
		name := strings.ToLower(c.GetUserName())

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Name",
			Value:  c.GetUserName(),
			Inline: false,
		})

		// Determine the contribution rate for the user
		futureDeliveries := c.GetContributionRate() * float64(calcSecondsRemaining)
		contributionPast := c.GetContributionAmount()
		offlineDeliveries := -c.GetFarmInfo().GetTimestamp() * c.GetContributionRate()
		if coopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
			offlineDeliveries = float64(0.0)
		}
		contribution[i] = contributionPast + offlineDeliveries + futureDeliveries
		BuffTimeValues = nil
		// Build slice of BuffTimeValues
		for _, a := range c.GetBuffHistory() {
			earnings := int(math.Round(a.GetEarnings()*100 - 100))
			eggRate := int(math.Round(a.GetEggLayingRate()*100 - 100))
			serverTimestamp := int64(a.GetServerTimestamp()) // When it was equipped
			if coopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
				serverTimestamp -= int64(coopStatus.GetSecondsSinceAllGoalsAchieved())
			} else {
				serverTimestamp += calcSecondsRemaining
			}
			serverTimestamp = int64(contractDurationSeconds) - serverTimestamp
			BuffTimeValues = append(BuffTimeValues, BuffTimeValue{name, earnings, 0.0075 * float64(earnings), eggRate, 0.0075 * float64(eggRate) * 10.0, serverTimestamp, 0, 0, 0, 0})
		}

		// From the last equipped buff, calculate the time until the end of the contract
		remainingTime := contractDurationSeconds
		for i, b := range BuffTimeValues {
			if i == len(BuffTimeValues)-1 {
				BuffTimeValues[i].durationEquiped = int64(contractDurationSeconds) - b.timeEquiped
			} else {
				BuffTimeValues[i].durationEquiped = BuffTimeValues[i+1].timeEquiped - b.timeEquiped
			}
			remainingTime -= float64(BuffTimeValues[i].durationEquiped)
		}
		var teamwork strings.Builder

		if len(BuffTimeValues) == 0 {
			teamwork.WriteString("**No buffs found for this contract.**\n")
		} else {
			table := tablewriter.NewWriter(&teamwork)
			table.SetHeader([]string{"Time", "Duration", "Defl", "SIAB", "BTV", "TeamWork"})
			table.SetBorder(false)
			table.SetAlignment(tablewriter.ALIGN_RIGHT)
			table.SetCenterSeparator("")
			table.SetColumnSeparator("")
			table.SetRowSeparator("")
			table.SetHeaderLine(false)
			table.SetTablePadding(" ") // pad with tabs
			table.SetNoWhiteSpace(true)
			table.SetBorders(tablewriter.Border{Left: false, Top: false, Right: false, Bottom: false})

			BestSIAB := 0.0
			LastSIAB := 0.0
			LastSIABCalc := 0.0
			var MostRecentDuration time.Time
			var buffTimeValue float64

			//buffTimeValue2, B2 = calculateBuffTimeValue(contractDurationSeconds, c.GetBuffHistory())

			for _, b := range BuffTimeValues {
				if b.durationEquiped < 0 {
					b.durationEquiped = 0
				}

				b.buffTimeValue = float64(b.durationEquiped)*b.earningsCalc + float64(b.durationEquiped)*b.eggRateCalc
				// Want pure buff time value score for each
				B := min(2, b.buffTimeValue/contractDurationSeconds)
				segmentTeamworkScore := getPredictedTeamwork(B, 0.0, 0.0)

				dur := time.Duration(b.durationEquiped) * time.Second
				when := time.Duration(b.timeEquiped) * time.Second
				MostRecentDuration = startTime.Add(when)

				// Track the best SIAB for the contract
				if b.earnings > int(BestSIAB) {
					BestSIAB = b.earningsCalc
				}
				LastSIAB = b.earningsCalc
				LastSIABCalc = float64(b.durationEquiped) * b.earningsCalc

				table.Append([]string{fmt.Sprintf("%v", when.Round(time.Second)), fmt.Sprintf("%v", dur.Round(time.Second)), fmt.Sprintf("%d%%", b.eggRate), fmt.Sprintf("%d%%", b.earnings), fmt.Sprintf("%6.0f", b.buffTimeValue), fmt.Sprintf("%1.6f", segmentTeamworkScore)})
				buffTimeValue += b.buffTimeValue
			}

			// Calculate the Teamwork Score for all the time segments
			uncappedBuffTimeValue := buffTimeValue / contractDurationSeconds
			B := min(uncappedBuffTimeValue, 2.0)
			TeamworkScore := getPredictedTeamwork(B, 0.0, 0.0)
			table.Append([]string{"", "", "", "", fmt.Sprintf("%6.0f", buffTimeValue), fmt.Sprintf("%1.6f", TeamworkScore)})
			table.Render()

			// If the teamwork segment
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

			// If SIAB still equipped, subtract that time from the total
			shortTeamwork := (contractDurationSeconds * 2.0) - (buffTimeValue - LastSIABCalc)
			siabSecondsNeeded := shortTeamwork / LastSIAB
			siabTimeEquipped := time.Duration(siabSecondsNeeded) * time.Second

			// Compensate for someone having a lesser SIAB equipped
			nowSIAB := BestSIAB
			if LastSIAB > 0 {
				nowSIAB = LastSIAB
			}

			if nowSIAB > 0 && coopStatus.GetSecondsSinceAllGoalsAchieved() <= 0 {
				// Your deflector % + your ship % (divided by 10) needs to average 26.7 over the course of the contract
				var maxTeamwork strings.Builder
				if nowSIAB != 0 {
					maxTeamwork.WriteString(fmt.Sprintf("Equip current SIAB for %s (<t:%d:t>) in the most recent teamwork segment to max BTV by %6.0f.\n",
						bottools.FmtDuration(siabTimeEquipped),
						MostRecentDuration.Add(siabTimeEquipped).Unix(),
						shortTeamwork))
					if MostRecentDuration.Add(siabTimeEquipped).Before(endTime) {
						fmt.Fprintf(&siabMsg, "<t:%d:t> %s\n", MostRecentDuration.Add(siabTimeEquipped).Unix(), name)
						siabMsgCount++
					}
				} else {
					if time.Now().Add(siabTimeEquipped).After(endTime) {
						// How much longer is this siabTimeEquipped than the end of the contract
						extraTime := time.Now().Add(siabTimeEquipped).Sub(endTime)

						// Calculate the shortTeamwork reducing the extra time from the siabTimeEquipped
						extraPercent := (siabTimeEquipped - extraTime).Seconds() / siabTimeEquipped.Seconds()

						maxTeamwork.WriteString(fmt.Sprintf("Equip your best SIAB through end of contract (<t:%d:t>) in new teamwork segment to improve BTV by %6.0f. ", endTime.Unix(), shortTeamwork*extraPercent))
						maxTeamwork.WriteString(fmt.Sprintf("The maximum BTV increase of %6.0f would be achieved if the contract finished at <t:%d:f>.", shortTeamwork, time.Now().Add(siabTimeEquipped).Unix()))
						if time.Now().Add(siabTimeEquipped).Before(endTime) {
							fmt.Fprintf(&siabMsg, "<t:%d:t> %s\n", endTime.Unix(), name)
							siabMsgCount++
						}
					} else {
						maxTeamwork.WriteString(fmt.Sprintf("Equip your best SIAB for %s (<t:%d:t>) in new teamwork segment to max BTV by %6.0f.\n", bottools.FmtDuration(siabTimeEquipped), time.Now().Add(siabTimeEquipped).Unix(), shortTeamwork))
						//if time.Now().Add(siabTimeEquipped).Before(endTime) {
						//	fmt.Fprintf(&siabMsg, "<t:%d:t> %s\n", time.Now().Add(siabTimeEquipped).Unix(), name)
						//}
					}
				}

				field = append(field, &discordgo.MessageEmbedField{
					Name:   "Maximize Teamwork",
					Value:  maxTeamwork.String(),
					Inline: false,
				})
			}

			// Chicken Runs
			// Create a table of Chicken Runs with maximized TVAL
			capCR := min((eiContract.MaxCoopSize*contractDurationInDays)/2, 20)

			// Maximize Token Value for this
			T := calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 100.0, 0)

			var crBuilder strings.Builder
			for maxCR := capCR; maxCR >= 0; maxCR-- {
				CR := calculateChickenRunTeamwork(eiContract.MaxCoopSize, contractDurationInDays, maxCR)
				score := calculateContractScore(grade,
					eiContract.MaxCoopSize,
					eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
					contribution[i],
					eiContract.Grade[grade].LengthInSeconds,
					contractDurationSeconds,
					B, CR, T)
				crBuilder.WriteString(fmt.Sprintf("%d:%d ", maxCR, score))
			}
			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Chicken Runs w/Tval 🎯",
				Value:  "```" + crBuilder.String() + "```",
				Inline: false,
			})

			// Create a table of Contract Scores for this user
			var csBuilder strings.Builder

			// Maximum Contract Score with current buffs and max CR & TVAL
			CR := calculateChickenRunTeamwork(eiContract.MaxCoopSize, contractDurationInDays, capCR)
			T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 100.0, 5.0)
			scoreMax := calculateContractScore(grade,
				eiContract.MaxCoopSize,
				eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
				contribution[i],
				eiContract.Grade[grade].LengthInSeconds,
				contractDurationSeconds,
				B, CR, T)
			fmt.Fprintf(&csBuilder, "Max: %d\n", scoreMax)

			// Sink Contract Score with current buffs and max CR & negative TVAL
			T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 3.0, 11.0)
			CR = calculateChickenRunTeamwork(eiContract.MaxCoopSize, contractDurationInDays, capCR)
			scoreMid := calculateContractScore(grade,
				eiContract.MaxCoopSize,
				eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
				contribution[i],
				eiContract.Grade[grade].LengthInSeconds,
				contractDurationSeconds,
				B, CR, T)
			fmt.Fprintf(&csBuilder, "Sink: %d (CR=%d)\n", scoreMid, capCR)

			// Sink Contract Score with current buffs and max CR & negative TVAL
			T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 0.0, 11.0)
			CR = calculateChickenRunTeamwork(eiContract.MaxCoopSize, contractDurationInDays, 0)
			scoreMin := calculateContractScore(grade,
				eiContract.MaxCoopSize,
				eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
				contribution[i],
				eiContract.Grade[grade].LengthInSeconds,
				contractDurationSeconds,
				B, CR, T)
			fmt.Fprintf(&csBuilder, "Min: %d (CR/TV=0)\n", scoreMin)

			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Contract Score",
				Value:  csBuilder.String(),
				Inline: false,
			})
			farmerFields[name] = field
		}
	}
	var siabMax []*discordgo.MessageEmbedField
	if siabMsgCount == 0 {
		siabMsg.WriteString("\nNo SIAB swaps needed.\n")
	} else {
		siabMsg.WriteString("\nUsing your best SiaB will result in higher CS.\n")
	}
	siabMax = append(siabMax, &discordgo.MessageEmbedField{
		Name:   "SIAB",
		Value:  siabMsg.String(),
		Inline: false,
	})
	farmerFields["siab"] = siabMax

	builder.WriteString(dataTimestampStr)

	return builder.String(), farmerFields
}
