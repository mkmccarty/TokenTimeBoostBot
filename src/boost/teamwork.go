package boost

import (
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// DeliveryTimeValue is a struct to hold the values for a delivery time
type DeliveryTimeValue struct {
	name                      string
	sr                        float64
	elr                       float64
	originalDelivery          float64
	contributions             float64
	contributionRateInSeconds float64
	timeEquipped              time.Time
	duration                  time.Duration
	cumulativeContrib         float64
}

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
			/*
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "show-scores",
					Description: "Show Contract Scores only. Default is false. (sticky)",
					Required:    false,
				},
			*/
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
		contractID = opt.StringValue()
		contractID = strings.ReplaceAll(contractID, " ", "")
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = strings.ToLower(opt.StringValue())
		coopID = strings.ReplaceAll(coopID, " ", "")
		// Only Development Staff can use a coop-id that starts with '?'
		if !slices.Contains(config.DevelopmentStaff, userID) && strings.HasPrefix(coopID, "?") {
			coopID = strings.TrimPrefix(coopID, "?")
		}
	}
	if opt, ok := optionMap["public-reply"]; ok {
		publicReply = !opt.BoolValue()
		if opt.BoolValue() {
			flags &= ^discordgo.MessageFlagsEphemeral
		}
	}
	/*
		if opt, ok := optionMap["show-scores"]; ok {
			// If show-scores is true, then we want to show the scores only
			scoresFirst = opt.BoolValue()
			farmerstate.SetMiscSettingFlag(userID, "teamwork-scores", scoresFirst)
		} else {
			scoresFirst = farmerstate.GetMiscSettingFlag(userID, "teamwork-scores")
		}
	*/

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
		contractID = contract.ContractID
		coopID = strings.ToLower(contract.CoopID)
	}

	var str string
	str, fields, _ := DownloadCoopStatusTeamwork(contractID, coopID, 0)
	if fields == nil || strings.HasSuffix(str, "no such file or directory") || strings.HasPrefix(str, "No grade found") {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: str,
		})
		return
	}

	/*
		if scoresFirst {
			var footer strings.Builder
			footer.WriteString("-# MAX : Max Chicken Runs & ∆T-Val\n")
			footer.WriteString("-# TVAL: Coop Size-1 Chicken Runs & ∆T-Val\n")
			footer.WriteString("-# SINK: Max Chicken Runs & Token Sink\n")
			footer.WriteString("-# RUNS: Coop Size-1 Chicken Runs, No token sharing\n")
			footer.WriteString("-# BASE: No Chicken Runs & No token sharing\n")
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Flags: discordgo.MessageFlagsIsComponentsV2,
				Components: []discordgo.MessageComponent{
					discordgo.TextDisplay{
						Content: str,
					},
					discordgo.TextDisplay{
						Content: "## Projected Contract Scores",
					},
					discordgo.TextDisplay{
						Content: scores,
					},
					discordgo.TextDisplay{
						Content: footer.String(),
					},
				},
			})
			return
		}
	*/
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
func DownloadCoopStatusTeamwork(contractID string, coopID string, offsetEndTime time.Duration) (string, map[string][]*discordgo.MessageEmbedField, string) {
	var siabMsg strings.Builder
	siabSwapMap := make(map[int64]string)
	var dataTimestampStr string
	var nowTime time.Time
	//totalSiabSwapSeconds := time.Duration(0)

	eiContract := ei.EggIncContractsAll[contractID]
	if eiContract.ID == "" {
		return "Invalid contract ID.", nil, ""
	}

	// extraInfo is true if coopID starts with '?'
	extraInfo := false
	if strings.HasPrefix(coopID, "?") {
		coopID = strings.TrimPrefix(coopID, "?")
		extraInfo = true
	}

	if strings.HasPrefix(coopID, "**") {
		coopID = strings.TrimPrefix(coopID, "**")
		// Want to get a list of the filenames within the ttbb-data directory
		files, err := os.ReadDir("ttbb-data/pb")
		if err != nil {
			return "Failed to read ttbb-data directory.", nil, ""
		}
		var fileNames []string
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			fileName := file.Name()
			// Check if filename contains the coopID pattern
			if strings.Contains(fileName, contractID+"-"+coopID+"-") {
				fileNames = append(fileNames, fileName)
			}
		}
		// Return the list of matching filenames
		return fmt.Sprintf("Filenames:\n%s", strings.Join(fileNames, "\n")), nil, ""
	}

	coopStatus, nowTime, dataTimestampStr, err := ei.GetCoopStatus(contractID, coopID)
	if err != nil {
		return err.Error(), nil, ""
	}

	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		return ei.ContractCoopStatusResponse_ResponseStatus_name[int32(coopStatus.GetResponseStatus())], nil, ""
	}
	if coopStatus.GetGrade() == ei.Contract_GRADE_UNSET {
		return fmt.Sprintf("No grade found for contract %s/%s", contractID, coopID), nil, ""
	}

	type BuffTimeValue struct {
		name            string
		earnings        int
		earningsCalc    float64
		eggRate         int
		eggRateCalc     float64
		timeEquiped     float64
		durationEquiped float64
		buffTimeValue   float64
		tb              int64
		totalValue      float64
	}

	var BuffTimeValues []BuffTimeValue
	var contractDurationSeconds float64
	var elapsedSeconds float64
	var calcSecondsRemaining float64
	var siabEndtimes []int64

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
	contractDurationInDays := int(math.Ceil(float64(eiContract.Grade[grade].LengthInSeconds) / 86400.0))

	var totalContributions float64
	var contributionRatePerSecond float64
	// Need to figure out how much longer this contract will run
	for _, c := range coopStatus.GetContributors() {
		totalContributions += c.GetContributionAmount()
		totalContributions += -(c.GetContributionRate() * c.GetFarmInfo().GetTimestamp()) // offline eggs
		contributionRatePerSecond += c.GetContributionRate()
	}

	totalRequired := eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1]

	prefix := ""
	startTime := nowTime
	secondsRemaining := coopStatus.GetSecondsRemaining()
	endTime := nowTime

	if coopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
		calcSecondsRemaining = -coopStatus.GetSecondsSinceAllGoalsAchieved()
		endTime = endTime.Add(time.Duration(calcSecondsRemaining) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
		builder.WriteString(fmt.Sprintf("Completed %s contract %s/[**%s**](%s)\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), coopStatus.GetContractIdentifier(), coopStatus.GetCoopIdentifier(), fmt.Sprintf("%s/%s/%s", "https://eicoop-carpet.netlify.app", contractID, coopID)))
	} else {
		prefix = "Est. "
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
		calcSecondsRemaining = (totalRequired-totalContributions)/contributionRatePerSecond - offsetEndTime.Seconds()
		endTime = nowTime.Add(time.Duration(calcSecondsRemaining) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
		elapsedSeconds = nowTime.Sub(startTime).Seconds()
		builder.WriteString(fmt.Sprintf("In Progress %s %s/[**%s**](%s) on target to complete <t:%d:R>\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), coopStatus.GetContractIdentifier(), coopStatus.GetCoopIdentifier(), fmt.Sprintf("%s/%s/%s", "https://eicoop-carpet.netlify.app", contractID, coopID), endTime.Unix()))
	}

	UpdateContractTime(coopStatus.GetContractIdentifier(), coopStatus.GetCoopIdentifier(), startTime, contractDurationSeconds)

	builder.WriteString(fmt.Sprintf("Start Time: <t:%d:f>\n", startTime.Unix()))
	builder.WriteString(fmt.Sprintf("%sEnd Time: <t:%d:f>\n", prefix, endTime.Unix()))
	builder.WriteString(fmt.Sprintf("%sDuration: %v\n", prefix, (endTime.Sub(startTime)).Round(time.Second)))
	if offsetEndTime > 0 {
		builder.WriteString(fmt.Sprintf("End Time includes %s for SIAB swaps\n", bottools.FmtDuration(offsetEndTime)))
	}

	siabMsg.WriteString("Showing those with SIAB equipped and can swap it out before the end of the contract without losing teamwork score.\n")

	// Used to collect the return values for each farmer
	var farmerFields = make(map[string][]*discordgo.MessageEmbedField)

	type contractScores struct {
		name string
		max  int64
		sink int64
		tval int64
		runs int64
		min  int64
		base int64
	}
	var contractScoreArr []contractScores
	var scoresTable strings.Builder
	fmt.Fprintf(&scoresTable, "`%12s %6s %6s %6s %6s %6s %6s`\n",
		bottools.AlignString("NAME", 12, bottools.StringAlignCenter),
		bottools.AlignString("MAX", 6, bottools.StringAlignCenter),
		bottools.AlignString("TVAL", 6, bottools.StringAlignCenter),
		bottools.AlignString("SINK", 6, bottools.StringAlignCenter),
		bottools.AlignString("RUNS", 6, bottools.StringAlignCenter),
		bottools.AlignString("MIN", 6, bottools.StringAlignCenter),
		bottools.AlignString("BASE", 6, bottools.StringAlignCenter),
	)

	var DeliveryTimeValues []DeliveryTimeValue

	deliveryTableMap := make(map[string][]DeliveryTimeValue)

	for _, c := range coopStatus.GetContributors() {
		pp := c.GetProductionParams()
		DeliveryTimeValues = nil

		// 	totalContributions += c.GetContributionAmount()
		//	totalContributions += -(c.GetContributionRate() * c.GetFarmInfo().GetTimestamp()) // offline eggs
		durationPast := time.Since(startTime) + time.Duration(c.GetFarmInfo().GetTimestamp())*time.Second
		DeliveryTimeValues = append(DeliveryTimeValues, DeliveryTimeValue{
			"Past",
			pp.GetSr() * 3600,
			pp.GetElr() * 3600,
			0,
			c.GetContributionAmount(),
			(c.GetContributionAmount() / durationPast.Seconds()),
			startTime,
			durationPast,
			c.GetContributionAmount(),
		})
		if calcSecondsRemaining > 0 {
			DeliveryTimeValues = append(DeliveryTimeValues, DeliveryTimeValue{
				"Offline",
				pp.GetSr() * 3600,
				pp.GetElr() * 3600,
				0,
				-(c.GetContributionRate() * c.GetFarmInfo().GetTimestamp()),
				c.GetContributionRate(),
				nowTime.Add(time.Duration(c.GetFarmInfo().GetTimestamp()) * time.Second),
				time.Duration(-c.GetFarmInfo().GetTimestamp()) * time.Second,
				DeliveryTimeValues[0].contributions + -(c.GetContributionRate() * c.GetFarmInfo().GetTimestamp()),
			})
			DeliveryTimeValues = append(DeliveryTimeValues, DeliveryTimeValue{
				"Future",
				pp.GetSr() * 3600,
				pp.GetElr() * 3600,
				0,
				c.GetContributionRate() * float64(calcSecondsRemaining),
				c.GetContributionRate(),
				nowTime,
				time.Duration(calcSecondsRemaining) * time.Second,
				DeliveryTimeValues[1].contributions + c.GetContributionRate()*float64(calcSecondsRemaining),
			})
		}
		deliveryTableMap[strings.ToLower(c.GetUserName())] = DeliveryTimeValues
	}

	// Used to determine the entire coop swap time
	type ProductionScheduleParams struct {
		targetEggAmount float64
		initialElr      float64
		deltaElr        float64
		alpha           float64
		elapsedTimeSec  float64
		eggsShipped     float64
		startTime       time.Time
		timezone        string
		futureSwapTime  time.Time
	}
	var productionScheduleParamsArray []ProductionScheduleParams

	for i, c := range coopStatus.GetContributors() {

		var field []*discordgo.MessageEmbedField
		name := strings.ToLower(c.GetUserName())

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Name",
			Value:  c.GetUserName(),
			Inline: false,
		})

		// Determine the contribution rate for the user
		futureDeliveries := c.GetContributionRate() * math.Max(0, float64(calcSecondsRemaining))
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
			// Equiptime is relative to the estimated end of the contract
			equipTimestamp := contractDurationSeconds - (a.GetServerTimestamp() + calcSecondsRemaining)
			BuffTimeValues = append(BuffTimeValues, BuffTimeValue{name, earnings, 0.0075 * float64(earnings), eggRate, 0.0075 * float64(eggRate) * 10.0, equipTimestamp, 0, 0, 0, 0})
		}

		// From the last equipped buff, calculate the time until the end of the contract
		remainingTime := contractDurationSeconds
		for i, b := range BuffTimeValues {
			if i == len(BuffTimeValues)-1 {
				BuffTimeValues[i].durationEquiped = contractDurationSeconds - b.timeEquiped
			} else {
				BuffTimeValues[i].durationEquiped = BuffTimeValues[i+1].timeEquiped - b.timeEquiped
			}
			remainingTime -= float64(BuffTimeValues[i].durationEquiped)
		}
		var teamwork strings.Builder

		B := 0.0
		if len(BuffTimeValues) == 0 {
			teamwork.WriteString("**No buffs found for this contract.**\n")
		} else {
			teamworkFmtHdr := "%10s %10s %3s %4s %6s %-8s\n"
			teamworkFm := "%10s %10s %3s %4s %6s %8s\n"
			fmt.Fprintf(&teamwork, teamworkFmtHdr,
				bottools.AlignString("TIME", 10, bottools.StringAlignCenter),
				bottools.AlignString("DURATION", 10, bottools.StringAlignCenter),
				bottools.AlignString("DEF", 3, bottools.StringAlignCenter),
				bottools.AlignString("SIAB", 4, bottools.StringAlignCenter),
				bottools.AlignString("BTV", 6, bottools.StringAlignRight),
				bottools.AlignString("TEAMWORK", 8, bottools.StringAlignRight),
			)

			BestSIAB := 0.0
			LastSIAB := 0.0
			LastSIABCalc := 0.0
			var MostRecentDuration time.Time
			var buffTimeValue float64

			for _, b := range BuffTimeValues {
				if b.durationEquiped < 0 {
					b.durationEquiped = 0
				}

				segmentBuffTimeValue := calculateBuffTimeValue(int(eiContract.CxpVersion), b.durationEquiped, b.eggRate, b.earnings)
				b.buffTimeValue = segmentBuffTimeValue
				// We'll calculate this for the segment but it seems suspect
				B := calculateTeamworkB(segmentBuffTimeValue, b.durationEquiped)
				segmentTeamworkScore := getPredictedTeamwork(eiContract.CxpVersion, B, 0.0, 0.0)

				dur := time.Duration(b.durationEquiped) * time.Second
				when := time.Duration(b.timeEquiped) * time.Second
				MostRecentDuration = startTime.Add(when)

				// Track the best SIAB for the contract
				if b.earnings > int(BestSIAB) {
					BestSIAB = b.earningsCalc
				}
				LastSIAB = b.earningsCalc
				LastSIABCalc = float64(b.durationEquiped) * b.earningsCalc

				fmt.Fprintf(&teamwork, teamworkFm,
					bottools.AlignString(fmt.Sprintf("%v", when.Round(time.Second)), 10, bottools.StringAlignCenter),
					bottools.AlignString(fmt.Sprintf("%v", dur.Round(time.Second)), 10, bottools.StringAlignCenter),
					bottools.AlignString(fmt.Sprintf("%d%%", b.eggRate), 3, bottools.StringAlignRight),
					bottools.AlignString(fmt.Sprintf("%d%%", b.earnings), 4, bottools.StringAlignRight),
					bottools.AlignString(fmt.Sprintf("%6.0f", b.buffTimeValue), 6, bottools.StringAlignRight),
					bottools.AlignString(fmt.Sprintf("%1.6f", segmentTeamworkScore), 8, bottools.StringAlignRight),
				)
				buffTimeValue += b.buffTimeValue
			}

			// Calculate the Teamwork Score for all the time segments
			B = calculateTeamworkB(buffTimeValue, contractDurationSeconds)
			fmt.Fprintf(&teamwork, teamworkFm,
				"", "", "", "",
				bottools.AlignString(fmt.Sprintf("%6.0f", buffTimeValue), 6, bottools.StringAlignRight),
				bottools.AlignString(fmt.Sprintf("%1.6f", B), 8, bottools.StringAlignRight))

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

			// Compensate for someone having a lesser SIAB equipped
			lastOrBestSIAB := BestSIAB
			if LastSIAB > 0 {
				lastOrBestSIAB = LastSIAB
			}

			// If SIAB still equipped, subtract that time from the total
			shortTeamwork := (contractDurationSeconds * 2.0) - (buffTimeValue - LastSIABCalc)

			// Using the current, or best SIAB if none equipped, calculate the time needed to max BTV
			siabSecondsNeeded := shortTeamwork / lastOrBestSIAB
			siabTimeEquipped := time.Duration(siabSecondsNeeded) * time.Second

			if lastOrBestSIAB > 0 && coopStatus.GetSecondsSinceAllGoalsAchieved() <= 0 {
				// Your deflector % + your ship % (divided by 10) needs to average 26.7 over the course of the contract
				var maxTeamwork strings.Builder
				// If SIAB is equipped beyond it's teamwork need, then make it it a swap for now
				if lastOrBestSIAB != 0 && LastSIAB != 0 {
					if shortTeamwork < 0 {
						siabTimeEquipped = time.Duration(0) * time.Second
						shortTeamwork = 0
					}

					maxTeamwork.WriteString(fmt.Sprintf("Equip current SIAB for %s (<t:%d:t>) in the most recent teamwork segment to max BTV by %6.0f.\n",
						bottools.FmtDuration(siabTimeEquipped),
						MostRecentDuration.Add(siabTimeEquipped).Unix(),
						shortTeamwork))

					// For testing I want to make siabTimeEquipped to be about an hour from now
					//siabTimeEquipped = time.Duration(3600/2) * time.Second
					// Add timestamp and name to the map for SIAB swaps
					if MostRecentDuration.Add(siabTimeEquipped).Before(endTime) {
						siabSwapMap[MostRecentDuration.Add(siabTimeEquipped).Unix()] = fmt.Sprintf("<t:%d:t> %s\n", MostRecentDuration.Add(siabTimeEquipped).Unix(), name)

						future := deliveryTableMap[name][len(deliveryTableMap[name])-1]
						siab := future
						//oldCumulative := future.cumulativeContrib
						siab.name = "SIAB"
						siab.timeEquipped = nowTime
						siab.duration = siabTimeEquipped - nowTime.Sub(MostRecentDuration)
						siab.contributions = c.GetContributionRate() * siab.duration.Seconds()
						siab.cumulativeContrib = future.cumulativeContrib - future.contributions + siab.contributions

						future.name = "Post-SIAB"
						future.timeEquipped = nowTime.Add(siab.duration)
						future.duration = endTime.Sub(future.timeEquipped)
						// Need to determine the adjusted contribution rate
						artifactPercentLevels := []float64{1.02, 1.04, 1.05}
						siabStones := 0
						stoneSlots := 0
						elrMult := 1.0
						srMult := 1.0
						// I want an array of artifact names ei.ArtifactSpec_Name
						var artifactIDs []int32
						for _, artifact := range c.GetFarmInfo().GetEquippedArtifacts() {
							spec := artifact.GetSpec()
							artifactName := spec.GetName()
							artifactIDs = append(artifactIDs, int32(artifactName))
							if artifactName == ei.ArtifactSpec_SHIP_IN_A_BOTTLE {
								siabStones, _ = ei.GetStones(spec.GetName(), spec.GetLevel(), spec.GetRarity())
							}
							for _, stone := range artifact.GetStones() {
								stoneSlots++
								if stone.GetName() == ei.ArtifactSpec_TACHYON_STONE {
									value := artifactPercentLevels[stone.GetLevel()]
									elrMult *= value
								}
								if stone.GetName() == ei.ArtifactSpec_QUANTUM_STONE {
									value := artifactPercentLevels[stone.GetLevel()]
									srMult *= value
								}
							}
						}
						// Assuming being replaced with a 3 slot artifact
						p := c.GetProductionParams()
						farmCapacity := p.GetFarmCapacity() * eiContract.Grade[grade].ModifierHabCap
						future.originalDelivery = min(future.sr, future.elr*farmCapacity)
						future.sr /= float64(srMult)
						future.elr /= float64(elrMult)

						//adjustedContributionRate, newRate, rateIncrease := determinePostSiabRate(future, stoneSlots+(3-siabStones), farmCapacity, maxFarm)
						_, newRate, rateIncrease, swapArtifactName := determinePostSiabRateOrig(future, stoneSlots, farmCapacity, artifactIDs)
						future.contributionRateInSeconds = newRate / 3600.0 // Assuming newRate is per hour
						future.contributions = (future.contributionRateInSeconds * future.duration.Seconds())
						future.cumulativeContrib = siab.cumulativeContrib + future.contributions

						// Calculate the saved number of seconds
						//maxTeamwork.WriteString(fmt.Sprintf("Increased contribution rate of %2.3g%% swapping %d slot SIAB with a 3 slot artifact and speeding the contract by %v\n", (adjustedContributionRate-1)*100, siabStones, diffSeconds))
						newSlots := 3
						if swapArtifactName == "Compass" {
							newSlots = 2
						}
						maxTeamwork.WriteString(fmt.Sprintf("Increased contribution rate of %2.3g%% swapping %d slot SIAB with a %d slot %s.\n",
							(future.contributionRateInSeconds/siab.contributionRateInSeconds-1)*100, siabStones, newSlots, swapArtifactName))
						siabSwapMap[MostRecentDuration.Add(siabTimeEquipped).Unix()] = fmt.Sprintf("<t:%d:t> %s\n", MostRecentDuration.Add(siabTimeEquipped).Unix(), name)

						if shortTeamwork == 0 {
							deliveryTableMap[name] = append(deliveryTableMap[name][:2], future)
						} else {
							deliveryTableMap[name] = append(deliveryTableMap[name][:2], siab, future)
						}

						targetEggAmount := totalRequired / 1e15
						initialElr := (contributionRatePerSecond * 3600) / 1e15
						deltaElr := rateIncrease / 1e15
						alpha := future.timeEquipped.Sub(startTime).Seconds() / contractDurationSeconds
						elapsedTimeSec := elapsedSeconds // in seconds
						eggsShipped := totalContributions / 1e15
						maxTeamwork.WriteString(fmt.Sprintf("\nTarget Egg Amount: %g\nInitial ELR: %g\nElapsed Time Sec: %g\nEggs Shipped: %g\n",
							targetEggAmount, initialElr, elapsedTimeSec, eggsShipped))
						if extraInfo {
							maxTeamwork.WriteString(fmt.Sprintf("\nDelta ELR: %g\nAlpha: %g\n",
								deltaElr, alpha))
						}

						switchTime, switchTimestamp, finishTimeWithSwitch, finishTimestampWithSwitch, finishTimeWithoutSwitch, finishTimestampWithoutSwitch, err := ProductionSchedule(
							targetEggAmount,
							initialElr,
							deltaElr,
							alpha,
							elapsedTimeSec,
							eggsShipped,
							startTime,
							"America/Los_Angeles",
						)

						params := ProductionScheduleParams{
							targetEggAmount: targetEggAmount,
							initialElr:      initialElr,
							deltaElr:        deltaElr,
							alpha:           alpha,
							elapsedTimeSec:  elapsedTimeSec,
							eggsShipped:     eggsShipped,
							startTime:       startTime,
							timezone:        "America/Los_Angeles",
							futureSwapTime:  future.timeEquipped,
						}

						productionScheduleParamsArray = append(productionScheduleParamsArray, params)

						if err == nil {
							maxTeamwork.WriteString(fmt.Sprintf("\n%s: <t:%d:f>\n", "Switch time ", switchTimestamp))
							maxTeamwork.WriteString(fmt.Sprintf("%s: <t:%d:f>\n", "Finish time with 1 player switch", finishTimestampWithSwitch))
							if extraInfo {
								siabEndtimes = append(siabEndtimes, finishTimestampWithSwitch)

								labels := []string{"Switch time", "Finish time with switch", "Finish time without switch"}
								results := []struct {
									dt time.Time
									ts int64
								}{
									{switchTime, switchTimestamp},
									{finishTimeWithSwitch, finishTimestampWithSwitch},
									{finishTimeWithoutSwitch, finishTimestampWithoutSwitch},
								}

								for i, lbl := range labels {
									maxTeamwork.WriteString(fmt.Sprintf("%s: <t:%d:f>\n", lbl, results[i].ts))
								}
							}
							maxTeamwork.WriteString("\nCompletion formulas from @James.WST")
						}

					}
				} else {
					if nowTime.Add(siabTimeEquipped).After(endTime) {
						// How much longer is this siabTimeEquipped than the end of the contract
						extraTime := nowTime.Add(siabTimeEquipped).Sub(endTime)

						// Calculate the shortTeamwork reducing the extra time from the siabTimeEquipped
						extraPercent := (siabTimeEquipped - extraTime).Seconds() / siabTimeEquipped.Seconds()

						maxTeamwork.WriteString(fmt.Sprintf("Equip your best SIAB through end of contract (<t:%d:t>) in new teamwork segment to improve BTV by %6.0f. ", endTime.Unix(), shortTeamwork*extraPercent))
						maxTeamwork.WriteString(fmt.Sprintf("The maximum BTV increase of %6.0f would be achieved if the contract finished at <t:%d:f>.", shortTeamwork, nowTime.Add(siabTimeEquipped).Unix()))

						if nowTime.Add(siabTimeEquipped).Before(endTime) {
							siabSwapMap[MostRecentDuration.Add(siabTimeEquipped).Unix()] = fmt.Sprintf("<t:%d:t> %s\n", endTime.Unix(), name)
						}
					} else {
						maxTeamwork.WriteString(fmt.Sprintf("Equip your best SIAB for %s (<t:%d:t>) in new teamwork segment to max BTV by %6.0f.\n", bottools.FmtDuration(siabTimeEquipped), nowTime.Add(siabTimeEquipped).Unix(), shortTeamwork))
						//if time.Now().Add(siabTimeEquipped).Before(endTime) {
						//	fmt.Fprintf(&siabMsg, "<t:%d:t> %s\n", nowTime.Add(siabTimeEquipped).Unix(), name)
						//}
					}
				}

				field = append(field, &discordgo.MessageEmbedField{
					Name:   "Maximize Teamwork",
					Value:  maxTeamwork.String(),
					Inline: false,
				})
			}

			var deliv strings.Builder
			deliveryFmtHdr := "%9s %10s %10s %7s %8s\n"
			deliveryFmt := "%9s %10s %10s %7s %8s\n"
			fmt.Fprintf(&deliv, deliveryFmtHdr,
				bottools.AlignString("TYPE", 9, bottools.StringAlignCenter),
				bottools.AlignString("TIME", 10, bottools.StringAlignCenter),
				bottools.AlignString("DURATION", 10, bottools.StringAlignCenter),
				bottools.AlignString("RATE/HR", 7, bottools.StringAlignCenter),
				bottools.AlignString("CONTRIB", 8, bottools.StringAlignCenter),
			)
			for _, d := range deliveryTableMap[name] {
				fmt.Fprintf(&deliv, deliveryFmt,
					bottools.AlignString(d.name, 9, bottools.StringAlignCenter),
					bottools.AlignString(d.timeEquipped.Sub(startTime).Round(time.Second).String(), 10, bottools.StringAlignCenter),
					bottools.AlignString(fmt.Sprintf("%v", d.duration.Round(time.Second)), 10, bottools.StringAlignCenter),
					bottools.AlignString(fmt.Sprintf("%2.3fq", (d.contributionRateInSeconds*3600)/1e15), 7, bottools.StringAlignCenter),
					bottools.AlignString(fmt.Sprintf("%2.3fq", d.contributions/1e15), 8, bottools.StringAlignCenter),
				)
			}

			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Deliveries",
				Value:  "```" + deliv.String() + "```",
				Inline: false,
			})

			// Chicken Runs
			// Create a Base score with no teamwork multipliers
			scoreBase := calculateContractScore(eiContract.CxpVersion, grade,
				eiContract.MaxCoopSize,
				eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
				contribution[i],
				eiContract.Grade[grade].LengthInSeconds,
				contractDurationSeconds,
				0, 0, 0)
			diffCR := (float64(scoreBase) * 0.06) / float64(eiContract.ChickenRuns)
			if eiContract.CxpVersion == 1 {
				diffCR = (float64(scoreBase) * 0.05) / float64(eiContract.ChickenRuns)
			}

			// Calculate a score with only the Buffs included
			var crBuilder strings.Builder
			minScore := calculateContractScore(eiContract.CxpVersion, grade,
				eiContract.MaxCoopSize,
				eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
				contribution[i],
				eiContract.Grade[grade].LengthInSeconds,
				contractDurationSeconds,
				B, 0, 0)

			for maxCR := eiContract.ChickenRuns; maxCR >= 0; maxCR-- {
				// Each CR is worth 6% of base score divided by maxCR
				crBuilder.WriteString(fmt.Sprintf("%d:%d ", maxCR, minScore+int64(math.Ceil(float64(maxCR)*diffCR))))
			}
			if diffCR > 0 {
				tvalString := " and TVAL=0"
				if eiContract.CxpVersion == 1 {
					tvalString = ""
				}
				crBuilder.WriteString(fmt.Sprintf("\nEach Chicken Run adds %6.3f to Contract Score with current buffs%s", diffCR, tvalString))
			}
			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Chicken Runs w/No Token Sharing",
				Value:  "```" + crBuilder.String() + "```",
				Inline: false,
			})

		}
		// Create a table of Contract Scores for this user
		var csBuilder strings.Builder

		// Maximum Contract Score with current buffs and max CR & TVAL
		CR := calculateChickenRunTeamwork(eiContract.CxpVersion, eiContract.MaxCoopSize, contractDurationInDays, eiContract.ChickenRuns)
		T := calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 100.0, 5.0)
		scoreMax := calculateContractScore(eiContract.CxpVersion, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "Max: %d\n", scoreMax)

		// TVAL Met, with CR to coop size -1
		T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 100.0, 5.0)
		CR = calculateChickenRunTeamwork(eiContract.CxpVersion, eiContract.MaxCoopSize, contractDurationInDays, eiContract.MaxCoopSize-1)
		scoreTval := calculateContractScore(eiContract.CxpVersion, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "TVal: %d (CR=%d)\n", scoreTval, min(eiContract.MaxCoopSize-1, eiContract.ChickenRuns))

		// Sink Contract Score with current buffs and max CR & negative TVAL
		T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 3.0, 11.0)
		CR = calculateChickenRunTeamwork(eiContract.CxpVersion, eiContract.MaxCoopSize, contractDurationInDays, eiContract.ChickenRuns)
		scoreMid := calculateContractScore(eiContract.CxpVersion, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "Sink: %d (CR=%d)\n", scoreMid, eiContract.ChickenRuns)

		// No token sharing, with CR to coop size -1
		T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 0.0, 11.0)
		CR = calculateChickenRunTeamwork(eiContract.CxpVersion, eiContract.MaxCoopSize, contractDurationInDays, min(eiContract.MaxCoopSize-1, eiContract.ChickenRuns))
		scoreRuns := calculateContractScore(eiContract.CxpVersion, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "Runs: %d (TV=0, CR=%d)\n", scoreRuns, min(eiContract.MaxCoopSize-1, eiContract.ChickenRuns))

		// Minimum Contract Score with current buffs and 0 CR & 0 TVAL
		T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 0.0, 11.0)
		CR = calculateChickenRunTeamwork(eiContract.CxpVersion, eiContract.MaxCoopSize, contractDurationInDays, 0)
		scoreMin := calculateContractScore(eiContract.CxpVersion, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "Min: %d (CR/TV=0)\n", scoreMin)

		scoreBase := calculateContractScore(eiContract.CxpVersion, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			0, 0, 0)
		fmt.Fprintf(&csBuilder, "Base: %d (B/CR/TV=0)\n", scoreBase)

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Contract Score",
			Value:  csBuilder.String(),
			Inline: false,
		})

		farmerFields[name] = field
		trimmedName := c.GetUserName()
		if len(trimmedName) > 12 {
			trimmedName = trimmedName[:12]
		}
		contractScoreArr = append(contractScoreArr, contractScores{
			trimmedName,
			scoreMax,
			scoreMid,
			scoreTval,
			scoreRuns,
			scoreMin,
			scoreBase,
		})

	}

	// Determine entire coop swap time for SIAB Swap
	// Sort this array by futureSwapTime
	sort.Slice(productionScheduleParamsArray, func(i, j int) bool {
		return productionScheduleParamsArray[i].futureSwapTime.Before(productionScheduleParamsArray[j].futureSwapTime)
	})

	// Interate the array and call ProductionSchedule for each
	deltaELRSum := 0.0
	alpha := 1.0
	for i, params := range productionScheduleParamsArray {
		deltaELRSum += params.deltaElr
		alpha = min(alpha, params.alpha)
		// If this isn't the last params item, then go to the next iteration
		if i < len(productionScheduleParamsArray)-1 {
			continue
		}

		switchTime, switchTimestamp, finishTimeWithSwitch, finishTimestampWithSwitch, finishTimeWithoutSwitch, finishTimestampWithoutSwitch, err := ProductionSchedule(
			params.targetEggAmount,
			params.initialElr,
			deltaELRSum,
			alpha,
			params.elapsedTimeSec,
			params.eggsShipped,
			params.startTime,
			params.timezone,
		)
		if err != nil {
			fmt.Printf("Error in ProductionSchedule: %v\n", err)
			continue
		}
		if extraInfo {
			builder.WriteString(fmt.Sprintf("Switch Time: %s <t:%d:f>\nFinish Time with Switch: %s <t:%d:f>\nFinish Time without Switch: %s <t:%d:f>\n",
				switchTime, switchTimestamp,
				finishTimeWithSwitch, finishTimestampWithSwitch,
				finishTimeWithoutSwitch, finishTimestampWithoutSwitch))
		}
	}

	if extraInfo {
		fmt.Printf("Total Delta ELR Sum: %f\n", deltaELRSum)
		builder.WriteString(fmt.Sprintf("Total Delta ELR Sum: %f\n", deltaELRSum))
		fmt.Printf("Min Alpha: %f\n", alpha)
		builder.WriteString(fmt.Sprintf("Min Alpha: %f\n", alpha))
	}

	// Create a table of Contract Scores for this user

	// Want to sort contractScoreArr by max score
	sort.SliceStable(contractScoreArr, func(i, j int) bool {
		if contractScoreArr[i].max == contractScoreArr[j].max {
			// Compare names, ignoring leading spaces
			nameI := strings.TrimLeft(contractScoreArr[i].name, " ")
			nameJ := strings.TrimLeft(contractScoreArr[j].name, " ")
			return nameI < nameJ
		}
		return contractScoreArr[i].max > contractScoreArr[j].max
	})
	for _, cs := range contractScoreArr {
		fmt.Fprintf(&scoresTable, "`%12s %6d %6d %6d %6d %6d %6d`\n",
			bottools.AlignString(cs.name, 12, bottools.StringAlignLeft),
			cs.max, cs.tval, cs.sink, cs.runs, cs.min, cs.base)
	}

	var siabMax []*discordgo.MessageEmbedField
	if len(siabSwapMap) == 0 {
		siabMsg.WriteString("\nNo SIAB swaps needed.\n")
	} else {
		var sortedKeys []int64
		for k := range siabSwapMap {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Slice(sortedKeys, func(i, j int) bool { return sortedKeys[i] < sortedKeys[j] })

		for _, k := range sortedKeys {
			siabMsg.WriteString(siabSwapMap[k])
		}
		siabMsg.WriteString("\nUsing your best SiaB will result in higher CS.\n")
		if extraInfo && len(siabEndtimes) > 0 {
			// Want to print a message with an average of the siab endtimes
			var totalSiabEndTimes time.Duration
			for _, t := range siabEndtimes {
				totalSiabEndTimes += time.Duration(t) * time.Second
			}

			/*
				avgSiabSwapEndTime := time.Unix(int64(totalSiabEndTimes.Seconds()/float64(len(siabEndtimes))), 0)
				timeSaved := endTime.Sub(avgSiabSwapEndTime)
				swapText := "swap"
				if len(siabEndtimes) > 1 {
					swapText = "swaps"
				}
				siabMsg.WriteString(fmt.Sprintf("\nWith %d timely %s, the contract can finish %s earlier at <t:%d:t>.\n",
					len(siabEndtimes),
					swapText,
					bottools.FmtDuration(timeSaved),
					avgSiabSwapEndTime.Unix()))
			*/

		}
	}

	siabMax = append(siabMax, &discordgo.MessageEmbedField{
		Name:   "SIAB",
		Value:  siabMsg.String(),
		Inline: false,
	})
	farmerFields["siab"] = siabMax

	builder.WriteString(dataTimestampStr)

	//if totalSiabSwapSeconds > 0 && offsetEndTime == 0 && coopStatus.GetSecondsSinceAllGoalsAchieved() == 0 {
	//	return DownloadCoopStatusTeamwork(contractID, coopID, totalSiabSwapSeconds)
	//}

	return builder.String(), farmerFields, scoresTable.String()
}

func determinePostSiabRateOrig(future DeliveryTimeValue, stoneSlots int, farmCapacity float64, artifactIDs []int32) (float64, float64, float64, string) {
	futureELR := future.elr
	futureELR *= farmCapacity
	futureSR := future.sr
	swapArtifactName := "artifact"
	// find which artifact is missing from ei.ArtifactSpec_Gusset, ei.ArtifactSpec_Ship_In_A_Bottle, and ei.ArtifactSpec_Compass ei.ArtifactSpec_Metronome
	// Make an array of the ArtifactSpecs below
	var allArtifacts = []int32{
		int32(ei.ArtifactSpec_ORNATE_GUSSET),
		int32(ei.ArtifactSpec_SHIP_IN_A_BOTTLE),
		int32(ei.ArtifactSpec_INTERSTELLAR_COMPASS),
		int32(ei.ArtifactSpec_QUANTUM_METRONOME),
		int32(ei.ArtifactSpec_TACHYON_DEFLECTOR),
	}
	// Using the incoming artifactIDs, determine which artifacts are missing
	artifactSet := make(map[int32]struct{})
	for _, id := range artifactIDs {
		artifactSet[id] = struct{}{}
	}
	stoneSlots++ // Typically one extra stone slot for swapping SIAB
	// Create a slice to hold the missing artifacts
	for _, artifactID := range allArtifacts {
		if _, exists := artifactSet[artifactID]; !exists {
			switch ei.ArtifactSpec_Name(artifactID) {
			case ei.ArtifactSpec_ORNATE_GUSSET:
				futureELR *= 1.25 // 25% increase for Gusset
				swapArtifactName = "Gusset"
			case ei.ArtifactSpec_INTERSTELLAR_COMPASS:
				futureSR *= 1.50 // 50% increase for Compass
				swapArtifactName = "Compass"
				stoneSlots--
			case ei.ArtifactSpec_QUANTUM_METRONOME:
				futureELR *= 1.35 // 35% increase for Metronome
				swapArtifactName = "Metronome"
			}
		}
	}

	delivery := min(future.sr, future.elr*farmCapacity)
	maxDelivery := min(futureSR, futureELR)

	for i := 0; i <= stoneSlots; i++ {
		tach := math.Pow(1.05, float64((stoneSlots - i)))
		quant := math.Pow(1.05, float64(i))
		elr := futureELR * tach
		sr := futureSR * quant
		calcDelivery := min(sr, elr)
		if config.IsDevBot() {
			fmt.Printf("T/Q: %d/%d ELR: %f  SR: %f  DLV:%f,  maxDeliv: %f\n", stoneSlots-i, i, elr/1e15, sr/1e15, calcDelivery/1e15, maxDelivery/1e15)
		}
		if calcDelivery > maxDelivery {
			maxDelivery = calcDelivery
		}
	}
	return maxDelivery / delivery, maxDelivery, maxDelivery - future.originalDelivery, swapArtifactName
}
