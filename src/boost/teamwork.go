package boost

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
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
				Name:        "public-reply",
				Description: "Respond publicly. Default is false.",
				Required:    false,
			},
		},
	}
}

// HandleTeamworkEvalCommand will handle the /teamwork-eval command
func HandleTeamworkEvalCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var builder strings.Builder

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
	if opt, ok := optionMap["public-reply"]; ok {
		if opt.BoolValue() {
			flags &= ^discordgo.MessageFlagsEphemeral
			builder.WriteString("Public Reply Enabled\n")
		}
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

	var dataTimestampStr string
	var field []*discordgo.MessageEmbedField
	nowTime := time.Now()

	eiContract := ei.EggIncContractsAll[contractID]
	if eiContract.ID == "" {
		return "Invalid contract ID.", nil
	}

	coopStatus, timestamp, dataTimestampStr, err := ei.GetCoopStatus(contractID, coopID)
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
	var teamwork strings.Builder

	grade := int(coopStatus.GetGrade())

	// Want estimated contribution ratio for entire contract
	contribution := make([]float64, len(coopStatus.GetContributors()))
	contractDurationInDays := int(float64(eiContract.Grade[grade].LengthInSeconds) / 86400.0)

	startTime := nowTime
	secondsRemaining := int64(coopStatus.GetSecondsRemaining())
	endTime := nowTime
	if coopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
		secondsSinceAllGoals := int64(coopStatus.GetSecondsSinceAllGoalsAchieved())
		endTime = endTime.Add(-time.Duration(secondsSinceAllGoals) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
		builder.WriteString(fmt.Sprintf("Completed %s contract **%s/%s**\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), contractID, coopID))
		builder.WriteString(fmt.Sprintf("Start Time: <t:%d:f>\n", startTime.Unix()))
		builder.WriteString(fmt.Sprintf("End Time: <t:%d:f>\n", endTime.Unix()))
		builder.WriteString(fmt.Sprintf("Duration: %v\n", (endTime.Sub(startTime)).Round(time.Second)))

	} else {
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
		builder.WriteString(fmt.Sprintf("In Progress %s **%s/%s** on target to complete <t:%d:R>\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), contractID, coopID, endTime.Unix()))
		builder.WriteString(fmt.Sprintf("Start Time: <t:%d:f>\n", startTime.Unix()))
		builder.WriteString(fmt.Sprintf("Est. End Time: <t:%d:f>\n", endTime.Unix()))
		builder.WriteString(fmt.Sprintf("Est. Duration: %v\n", (endTime.Sub(startTime)).Round(time.Second)))
	}
	if einame != "" {
		builder.WriteString(fmt.Sprintf("Evaluating data for **%s**\n", einame))
	}

	// Take care of other parameter calculations here
	sumOfAll := 0.0
	for idx, c := range coopStatus.GetContributors() {
		if coopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
			offlineDeliveries := float64(0.0)
			futureDeliveries := c.GetContributionRate() * float64(calcSecondsRemaining)
			contributionPast := c.GetContributionAmount()
			contribution[idx] = contributionPast + offlineDeliveries + futureDeliveries
			sumOfAll += contribution[idx]
		} else {
			// Contract not finished, so we need to estimate the contribution ratio
			// based on the current rate and the estimated completion time
			offlineDeliveries := -c.GetFarmInfo().GetTimestamp() * c.GetContributionRate()
			futureDeliveries := c.GetContributionRate() * float64(calcSecondsRemaining)
			contributionPast := c.GetContributionAmount()
			contribution[idx] = contributionPast + offlineDeliveries + futureDeliveries
			//contributionRatios[idx] = contribution[idx] / float64(eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1])
			sumOfAll += contribution[idx]
		}

	}

	found := false
	var teamworkNames []string

	userIdx := -1

	for i, c := range coopStatus.GetContributors() {

		name := c.GetUserName()

		if einame != name {
			teamworkNames = append(teamworkNames, name)
			continue
		}
		userIdx = i
		found = true

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
			B := 5 * min(2, b.buffTimeValue/contractDurationSeconds)
			CR := min(0.0, 6.0)
			T := 0.0
			teamworkScore := (B + CR + T) / 19.0

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
		// for Pure BuffArtifacts teamwork score we need no CR and TVal
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
		if BestSIAB > 0 && coopStatus.GetSecondsSinceAllGoalsAchieved() <= 0 {
			var maxTeamwork strings.Builder
			if LastSIABCalc != 0 {
				maxTeamwork.WriteString(fmt.Sprintf("Equip SIAB for %s (<t:%d:t>) in the most recent teamwork segment to max BTV by %6.0f.\n", bottools.FmtDuration(siabTimeEquipped), MostRecentDuration.Add(siabTimeEquipped).Unix(), shortTeamwork))
			} else {
				if time.Now().Add(siabTimeEquipped).After(endTime) {
					// How much longer is this siabTimeEquipped than the end of the contract
					extraTime := time.Now().Add(siabTimeEquipped).Sub(endTime)

					// Calculate the shortTeamwork reducing the extra time from the siabTimeEquipped
					extraPercent := (siabTimeEquipped - extraTime).Seconds() / siabTimeEquipped.Seconds()

					maxTeamwork.WriteString(fmt.Sprintf("Equip SIAB through end of contract (<t:%d:t>) in new teamwork segment to improve BTV by %6.0f. ", endTime.Unix(), shortTeamwork*extraPercent))
					maxTeamwork.WriteString(fmt.Sprintf("The maximum BTV increase of %6.0f would be achieved if the contract finished at <t:%d:f>.", shortTeamwork, time.Now().Add(siabTimeEquipped).Unix()))
				} else {
					maxTeamwork.WriteString(fmt.Sprintf("Equip SIAB for %s (<t:%d:t>) in new teamwork segment to max BTV by %6.0f.\n", bottools.FmtDuration(siabTimeEquipped), time.Now().Add(siabTimeEquipped).Unix(), shortTeamwork))
				}
			}
			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Maximize Teamwork",
				Value:  maxTeamwork.String(),
				Inline: false,
			})
		}

		if len(BuffTimeValues) <= 10 {

			// Final calculations on the score
			// Chicken Runs
			// Maximize Token Value for this
			T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 100.0, 0.0)
			capCR := min((eiContract.MaxCoopSize*contractDurationInDays)/2, 20)
			var crBuilder strings.Builder
			for maxCR := capCR; maxCR >= 0; maxCR-- {
				CR = calculateChickenRunTeamwork(eiContract.MaxCoopSize, contractDurationInDays, maxCR)
				score := calculateContractScore(grade,
					eiContract.MaxCoopSize,
					eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
					contribution[userIdx],
					eiContract.Grade[grade].LengthInSeconds,
					contractDurationSeconds,
					B, CR, T)
				crBuilder.WriteString(fmt.Sprintf("%d:%d ", maxCR, score))
			}
			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Contract Scores with Chicken Runs",
				Value:  "```" + crBuilder.String() + "```",
				Inline: false,
			})

		}

		tokenValueSent := 10.0
		tokenValueRecv := 0.0
		T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, tokenValueSent, tokenValueRecv)

		CR = calculateChickenRunTeamwork(eiContract.MaxCoopSize, eiContract.MaxCoopSize*contractDurationInDays, 20)
		scoreMax := calculateContractScore(grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[userIdx],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)

		// Now for the Token Sink
		capCR := min((eiContract.MaxCoopSize*contractDurationInDays)/2, 20)
		CR = calculateChickenRunTeamwork(eiContract.MaxCoopSize, contractDurationInDays, capCR)
		T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 3.0, 100.0)

		scoreMin := calculateContractScore(grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[userIdx],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Contract Score",
			Value:  fmt.Sprintf(">>> Maximum: %d\nMinimum (CR=%d) %d", scoreMax, capCR, scoreMin),
			Inline: false,
		})
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
			Timestamp:   timestamp.Format(time.RFC3339),
		},
		},
	}
	builder.WriteString(dataTimestampStr)

	return builder.String(), embed
}

func calculateChickenRunTeamwork(coopSize int, durationInDays int, runs int) float64 {
	fCR := max(12.0/(float64(coopSize)*float64(durationInDays)), 0.3)
	CR := min(fCR*float64(runs), 6.0)
	return CR
}

func calculateTokenTeamwork(contractDurationSeconds float64, minutesPerToken int, tokenValueSent float64, tokenValueReceived float64) float64 {
	BTA := contractDurationSeconds / (float64(minutesPerToken) * 60)
	T := 0.0

	if BTA <= 42.0 {
		T = (2.0 / 3.0 * min(tokenValueSent, 3.0)) + ((8.0 / 3.0) * min(max(tokenValueSent-tokenValueReceived, 0.0), 3.0))
	} else {
		T = (200.0/(7.0*BTA))*min(tokenValueSent, 0.07*BTA) + (800.0 / (7.0 * BTA) * min(max(tokenValueSent-tokenValueReceived, 0.0), 0.07*BTA))
	}

	//T := 2.0 * (min(V, tokenValueSent) + 4*min(V, max(0.0, tokenValueSent-tokenValueReceived))) / V
	return T
}

func calculateContractScore(grade int, coopSize int, targetGoal float64, contribution float64, contractLength int, contractDurationSeconds float64, B float64, CR float64, T float64) int64 {
	basePoints := 1.0
	durationPoints := 1.0 / 259200.0
	score := basePoints + durationPoints*float64(contractLength)

	gradeMultiplier := ei.GradeMultiplier[ei.Contract_PlayerGrade_name[int32(grade)]]
	score *= gradeMultiplier

	completionFactor := 1.0
	score *= completionFactor

	ratio := (contribution * float64(coopSize)) / targetGoal
	contributionFactor := 0.0
	if ratio <= 2.5 {
		contributionFactor = 1 + 3*math.Pow(ratio, 0.15)
	} else {
		contributionFactor = 0.02221*min(ratio, 12.5) + 4.386486
	}
	score *= contributionFactor

	completionTimeBonus := 1.0 + 4.0*math.Pow((1.0-float64(contractDurationSeconds)/float64(contractLength)), 3)
	score *= completionTimeBonus

	teamworkScore := (5.0*B + CR + T) / 19.0
	teamworkBonus := 1.0 + 0.19*teamworkScore
	score *= teamworkBonus
	score *= float64(187.5)

	return int64(math.Ceil(score))
}
