package boost

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
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

// TeamworkOutputData is a struct to hold the output data for teamwork fields
type TeamworkOutputData struct {
	Title   string
	Content string
	//Component []discordgo.MessageComponent
}

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
	optionMap := bottools.GetCommandOptionsMap(i)

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

	// Unset contractID and coopID means we want the Boost Bot contract
	if contractID == "" || coopID == "" {
		contract := FindContract(i.ChannelID)
		if contract == nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Flags: flags | discordgo.MessageFlagsIsComponentsV2,
					Components: []discordgo.MessageComponent{
						discordgo.TextDisplay{
							Content: "No contract found in this channel. Please provide a contract-id and coop-id.",
						},
					},
				})

			return
		}
		contractID = contract.ContractID
		coopID = strings.ToLower(contract.CoopID)
	}

	var str string
	str, fields, _ := DownloadCoopStatusTeamwork(contractID, coopID, 0)
	if fields == nil || strings.HasSuffix(str, "no such file or directory") || strings.HasPrefix(str, "No grade found") {
		// Trim output to 3500 characters if needed
		trimmedStr := str
		trimNotice := ""
		const maxLen = 3500
		if len(str) > maxLen {
			trimmedStr = str[:maxLen]
			trimNotice = "\n\n*Output trimmed to 3500 characters.*"
		}
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Flags: discordgo.MessageFlagsIsComponentsV2,
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: trimmedStr + trimNotice,
				},
			},
		})
		return
	}

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

	sendTeamworkPage(s, i, true, cache.xid, false, false, false, true)

	// Traverse stonesCacheMap and delete expired entries
	for key, cache := range teamworkCacheMap {
		if cache.expirationTimestamp.Before(time.Now()) {
			delete(teamworkCacheMap, key)
		}
	}
}

func computeRateIncrease(
	c *ei.ContractCoopStatusResponse_ContributionInfo,
	future DeliveryTimeValue,
	grade int,
	eiContract ei.EggIncContract,
	siabTimeEquipped time.Duration,
	nowTime, MostRecentDuration, endTime time.Time,
) (DeliveryTimeValue, DeliveryTimeValue, float64, string, int) {

	artifactPercentLevels := []float64{1.02, 1.04, 1.05}
	siabStones := 0
	stoneSlots := 0
	elrMult := 1.0
	srMult := 1.0
	var artifactIDs []int32

	// Clone and modify for SIAB
	siab := future
	siab.name = "SIAB"
	siab.timeEquipped = nowTime
	siab.duration = siabTimeEquipped - nowTime.Sub(MostRecentDuration)
	siab.contributions = c.GetContributionRate() * siab.duration.Seconds()
	siab.cumulativeContrib = future.cumulativeContrib - future.contributions + siab.contributions

	// Modify future for post-SIAB
	future.name = "Post-SIAB"
	future.timeEquipped = nowTime.Add(siab.duration)
	future.duration = endTime.Sub(future.timeEquipped)

	// Artifact and stone processing
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
				elrMult *= artifactPercentLevels[stone.GetLevel()]
			}
			if stone.GetName() == ei.ArtifactSpec_QUANTUM_STONE {
				srMult *= artifactPercentLevels[stone.GetLevel()]
			}
		}
	}

	p := c.GetProductionParams()
	farmCapacity := p.GetFarmCapacity() * eiContract.Grade[grade].ModifierHabCap

	future.originalDelivery = min(future.sr, future.elr*farmCapacity)
	future.sr /= srMult
	future.elr /= elrMult

	_, newRate, rateIncrease, swapArtifactName :=
		determinePostSiabRateOrig(future, stoneSlots, farmCapacity, artifactIDs)

	future.contributionRateInSeconds = newRate / 3600.0
	future.contributions = future.contributionRateInSeconds * future.duration.Seconds()
	future.cumulativeContrib = siab.cumulativeContrib + future.contributions

	return future, siab, rateIncrease, swapArtifactName, siabStones
}

// DownloadCoopStatusTeamwork will download the coop status for a given contract and coop ID
func DownloadCoopStatusTeamwork(contractID string, coopID string, offsetEndTime time.Duration) (string, map[string][]TeamworkOutputData, string) {
	var siabMsg strings.Builder
	var dataTimestampStr string
	var nowTime time.Time

	type siabEntry struct {
		Name       string
		DeltaELR   float64
		SwitchTime int64
	}
	var siabEntries []siabEntry

	eiContract := ei.EggIncContractsAll[contractID]
	if eiContract.ID == "" {
		return "Invalid contract ID.", nil, ""
	}

	// extraInfo is true if coopID starts with '?'
	extraInfo := false
	if after, hasPrefix := strings.CutPrefix(coopID, "?"); hasPrefix {
		coopID = after
		extraInfo = true
	}

	if after, hasPrefix := strings.CutPrefix(coopID, "**"); hasPrefix {
		coopID = after

		// Print working directory and directory being read
		cwd, _ := os.Getwd()
		fmt.Println("Current working dir:", cwd)
		fmt.Println("Reading directory:", filepath.Join(cwd, "ttbb-data/pb"))

		// Read directory
		files, err := os.ReadDir("ttbb-data/pb")
		if err != nil {
			fmt.Println("❌ Error reading directory:", err)
			return "Failed to read ttbb-data directory.", nil, ""
		}

		// Build search pattern
		var pattern string
		if coopID == "" {
			pattern = contractID + "-"
		} else {
			pattern = contractID + "-" + coopID
		}
		fmt.Println("Searching for pattern:", pattern)

		var fileNames []string
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			fileName := file.Name()
			// Check if filename contains the pattern
			if strings.Contains(fileName, pattern) {
				fileNames = append(fileNames, fileName)
			}
		}
		// Return the list of matching filenames
		if len(fileNames) == 0 {
			return fmt.Sprintf("No matching files found in %s.", filepath.Join(cwd, "ttbb-data/pb")), nil, ""
		}
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

	siabMsg.WriteString("SiaB Swap Times\n")
	siabMsg.WriteString("Showing those with SiaB equipped and can swap it out before the end of the contract without losing teamwork score.\n")

	// Used to collect the return values for each farmer
	var farmerFields = make(map[string][]TeamworkOutputData)

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
	if eiContract.SeasonalScoring == ei.SeasonalScoringNerfed {
		fmt.Fprintf(&scoresTable, "`%12s %6s %6s %6s`\n",
			bottools.AlignString("NAME", 12, bottools.StringAlignCenter),
			bottools.AlignString("MAX", 6, bottools.StringAlignCenter),
			bottools.AlignString("MIN", 6, bottools.StringAlignCenter),
			bottools.AlignString("BASE", 6, bottools.StringAlignCenter),
		)
	} else {
		fmt.Fprintf(&scoresTable, "`%12s %6s %6s %6s %6s %6s %6s`\n",
			bottools.AlignString("NAME", 12, bottools.StringAlignCenter),
			bottools.AlignString("MAX", 6, bottools.StringAlignCenter),
			bottools.AlignString("TVAL", 6, bottools.StringAlignCenter),
			bottools.AlignString("SINK", 6, bottools.StringAlignCenter),
			bottools.AlignString("RUNS", 6, bottools.StringAlignCenter),
			bottools.AlignString("MIN", 6, bottools.StringAlignCenter),
			bottools.AlignString("BASE", 6, bottools.StringAlignCenter),
		)
	}
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

	var productionScheduleParamsArray []ProductionScheduleParams

	for i, c := range coopStatus.GetContributors() {

		var field []TeamworkOutputData
		name := strings.ToLower(c.GetUserName())

		field = append(field, TeamworkOutputData{"Name", c.GetUserName()})
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

				segmentBuffTimeValue := calculateBuffTimeValue(int(eiContract.SeasonalScoring), b.durationEquiped, b.eggRate, b.earnings)
				b.buffTimeValue = segmentBuffTimeValue
				// We'll calculate this for the segment but it seems suspect
				B := calculateTeamworkB(segmentBuffTimeValue, b.durationEquiped)
				segmentTeamworkScore := getPredictedTeamwork(eiContract.SeasonalScoring, B, 0.0, 0.0)

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

					field = append(field, TeamworkOutputData{fmt.Sprintf("Teamwork-%d", i), "```" + teamworkStr[:chunkSize] + "```"})
					teamworkStr = teamworkStr[chunkSize:]
				}
			} else {
				field = append(field, TeamworkOutputData{"Teamwork", "```" + teamworkStr + "```"})
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
				//  if the player is using a SiaB make switch time predictions
				if lastOrBestSIAB != 0 && LastSIAB != 0 {

					if shortTeamwork < 0 {
						siabTimeEquipped = time.Duration(0) * time.Second
						shortTeamwork = 0
					}

					// For testing I want to make siabTimeEquipped to be about an hour from now
					//siabTimeEquipped = time.Duration(3600/2) * time.Second
					// Add timestamp and name to the map for SIAB swaps
					//if MostRecentDuration.Add(siabTimeEquipped).Before(endTime) {

					future := deliveryTableMap[name][len(deliveryTableMap[name])-1]
					future, siab, rateIncrease, swapArtifactName, siabStones :=
						computeRateIncrease(
							c,                  // *ei.ContractCoopStatusResponse_ContributionInfo
							future,             // DeliveryTimeValue
							grade,              // int
							eiContract,         // ei.EggIncContract
							siabTimeEquipped,   // time.Duration
							nowTime,            // time.Time
							MostRecentDuration, // time.Time
							endTime,            // time.Time
						)

					// Calculate the saved number of seconds
					//maxTeamwork.WriteString(fmt.Sprintf("Increased contribution rate of %2.3g%% swapping %d slot SIAB with a 3 slot artifact and speeding the contract by %v\n", (adjustedContributionRate-1)*100, siabStones, diffSeconds))
					newSlots := 3
					if swapArtifactName == "Compass" {
						newSlots = 2
					}
					maxTeamwork.WriteString(fmt.Sprintf("Increased contribution rate of %2.3g%% swapping %d slot SiaB with a %d slot %s.\n",
						(future.contributionRateInSeconds/siab.contributionRateInSeconds-1)*100, siabStones, newSlots, swapArtifactName))

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

					_, switchTimestamp, _, finishTimestampWithSwitch, _, finishTimestampWithoutSwitch, err := ProductionSchedule(
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
						name:            name,
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

					// Print 1p SiaB switch times
					if err == nil {
						if extraInfo {
							maxTeamwork.WriteString(fmt.Sprintf(
								"\nTarget Egg Amount: %g\nEggs Shipped: %f\nInitial ELR: %f\nDelta ELR: %f\nElapsed Time Sec: %g\nAlpha: %f\n",
								targetEggAmount, eggsShipped, initialElr, deltaElr, elapsedTimeSec, alpha,
							))
						}

						maxTeamwork.WriteString(fmt.Sprintf(
							"Teamwork BTV maxes at <t:%[1]d:t>, SiaB can be unequipped after this time.\n\n"+
								"**Switch time:** <t:%[1]d:f>\n"+
								"**Adjusted finish time with switch:** <t:%[2]d:f>\n",
							switchTimestamp,
							finishTimestampWithSwitch,
						))

						if extraInfo {
							maxTeamwork.WriteString(fmt.Sprintf(
								"**Finish time without switch:** <t:%d:f>\n",
								finishTimestampWithoutSwitch,
							))
						}

						siabEntries = append(siabEntries, siabEntry{
							Name:       name,
							DeltaELR:   deltaElr,
							SwitchTime: switchTimestamp,
						})

					}

				} else {
					if nowTime.Add(siabTimeEquipped).After(endTime) {
						// How much longer is this siabTimeEquipped than the end of the contract
						extraTime := nowTime.Add(siabTimeEquipped).Sub(endTime)

						// Calculate the shortTeamwork reducing the extra time from the siabTimeEquipped
						extraPercent := (siabTimeEquipped - extraTime).Seconds() / siabTimeEquipped.Seconds()

						maxTeamwork.WriteString(fmt.Sprintf("Equip your best SIAB through end of contract (<t:%d:t>) in new teamwork segment to improve BTV by %6.0f. ", endTime.Unix(), shortTeamwork*extraPercent))
						maxTeamwork.WriteString(fmt.Sprintf("The maximum BTV increase of %6.0f would be achieved if the contract finished at <t:%d:f>.", shortTeamwork, nowTime.Add(siabTimeEquipped).Unix()))

						// We might not be able to reach here anymore
						if nowTime.Add(siabTimeEquipped).Before(endTime) {
							siabEntries = append(siabEntries, siabEntry{
								Name:       name,
								DeltaELR:   0.00000000,
								SwitchTime: endTime.Unix(),
							})
						}
					} else {
						maxTeamwork.WriteString(fmt.Sprintf("Equip your best SIAB for %s (<t:%d:t>) in new teamwork segment to max BTV by %6.0f.\n", bottools.FmtDuration(siabTimeEquipped), nowTime.Add(siabTimeEquipped).Unix(), shortTeamwork))
					}
				}
				field = append(field, TeamworkOutputData{"Maximize Teamwork", maxTeamwork.String()})
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
			field = append(field, TeamworkOutputData{"Deliveries", "```" + deliv.String() + "```"})

			// Chicken Runs
			// Create a Base score with no teamwork multipliers
			scoreBase := calculateContractScore(eiContract.SeasonalScoring, grade,
				eiContract.MaxCoopSize,
				eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
				contribution[i],
				eiContract.Grade[grade].LengthInSeconds,
				contractDurationSeconds,
				0, 0, 0,
			)
			// Calculate the Chicken Run threshold LengthinDays * MaxCoopSize / 2
			crThreshold := math.Min(20., float64(eiContract.MaxCoopSize)*(float64(eiContract.LengthInSeconds)/86400.0)/2.0)
			if eiContract.SeasonalScoring == ei.SeasonalScoringNerfed {
				crThreshold = float64(eiContract.MaxCoopSize - 1)
			}

			var diffCR float64 // difference in CS per Chicken Run
			var scoreCRdiff int64
			switch eiContract.SeasonalScoring {
			case ei.SeasonalScoringNerfed:
				diffCR = (float64(scoreBase) * 0.19) * ((5.0 / 19.0) * (1.0 / crThreshold))
				scoreCRdiff = calculateContractScore(eiContract.SeasonalScoring, grade,
					eiContract.MaxCoopSize,
					eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
					contribution[i],
					eiContract.Grade[grade].LengthInSeconds,
					contractDurationSeconds,
					B, 0, 0) // only buffs, no CR or Tokens
			default:
				diffCR = (float64(scoreBase) * 0.19) * ((6.0 / 19.0) * (1.0 / crThreshold))
				T := calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 100.0, 5.0)
				scoreCRdiff = calculateContractScore(eiContract.SeasonalScoring, grade,
					eiContract.MaxCoopSize,
					eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
					contribution[i],
					eiContract.Grade[grade].LengthInSeconds,
					contractDurationSeconds,
					B, 0, T) // MAX TVAL & Tokens sent
			}

			// Calculate a score with only the Buffs included
			var crBuilder strings.Builder
			crBuilder.WriteString("```")

			for maxCR := eiContract.ChickenRuns; maxCR >= 0; maxCR-- {
				added := float64(maxCR) * diffCR
				if float64(maxCR) > crThreshold {
					added = float64(scoreBase) * 0.19 * (6.0 / 19.0)
				}
				crBuilder.WriteString(fmt.Sprintf("%d:%d ", maxCR, scoreCRdiff+int64(math.Ceil(added))))
			}
			crBuilder.WriteString("```")

			if diffCR > 0 {

				crBuilder.WriteString(fmt.Sprintf(
					"\nEach Chicken Run adds `%3.1f` to Contract Score.",
					diffCR,
				))
				if math.Mod(crThreshold, 1.0) == 0.5 {
					// print the half val
					crBuilder.WriteString(fmt.Sprintf("\nNote: CR Threshold is `%3.1f` The final run gives only half credit `%3.1f` (+0.5 CR threshold).", crThreshold, diffCR/2.0))
				}
			}
			str := "Chicken Runs (CR)"
			field = append(field, TeamworkOutputData{str, crBuilder.String()})

		}
		// Create a table of Contract Scores for this user
		var csBuilder strings.Builder

		// Maximum Contract Score with current buffs and max CR & TVAL
		CR := calculateChickenRunTeamwork(eiContract.SeasonalScoring, eiContract.MaxCoopSize, contractDurationInDays, eiContract.ChickenRuns)
		T := calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 100.0, 5.0)
		scoreMax := calculateContractScore(eiContract.SeasonalScoring, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "Max: %d\n", scoreMax)

		// TVAL Met, with CR to coop size -1
		T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 100.0, 5.0)
		CR = calculateChickenRunTeamwork(eiContract.SeasonalScoring, eiContract.MaxCoopSize, contractDurationInDays, eiContract.MaxCoopSize-1)
		scoreTval := calculateContractScore(eiContract.SeasonalScoring, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "TVal: %d (CR=%d)\n", scoreTval, min(eiContract.MaxCoopSize-1, eiContract.ChickenRuns))

		// Sink Contract Score with current buffs and max CR & negative TVAL
		T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 3.0, 11.0)
		CR = calculateChickenRunTeamwork(eiContract.SeasonalScoring, eiContract.MaxCoopSize, contractDurationInDays, eiContract.ChickenRuns)
		scoreMid := calculateContractScore(eiContract.SeasonalScoring, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "Sink: %d (CR=%d)\n", scoreMid, eiContract.ChickenRuns)

		// No token sharing, with CR to coop size -1
		T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 0.0, 11.0)
		CR = calculateChickenRunTeamwork(eiContract.SeasonalScoring, eiContract.MaxCoopSize, contractDurationInDays, min(eiContract.MaxCoopSize-1, eiContract.ChickenRuns))
		scoreRuns := calculateContractScore(eiContract.SeasonalScoring, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "Runs: %d (TV=0, CR=%d)\n", scoreRuns, min(eiContract.MaxCoopSize-1, eiContract.ChickenRuns))

		// Minimum Contract Score with current buffs and 0 CR & 0 TVAL
		T = calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 0.0, 11.0)
		CR = calculateChickenRunTeamwork(eiContract.SeasonalScoring, eiContract.MaxCoopSize, contractDurationInDays, 0)
		scoreMin := calculateContractScore(eiContract.SeasonalScoring, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "Min: %d (CR/TV=0)\n", scoreMin)

		scoreBase := calculateContractScore(eiContract.SeasonalScoring, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			0, 0, 0)
		fmt.Fprintf(&csBuilder, "Base: %d (B/CR/TV=0)\n", scoreBase)

		field = append(field, TeamworkOutputData{"Contract Score", csBuilder.String()})

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
	// Sort by futureSwapTime
	sort.Slice(productionScheduleParamsArray, func(i, j int) bool {
		return productionScheduleParamsArray[i].futureSwapTime.Before(productionScheduleParamsArray[j].futureSwapTime)
	})

	deltaELRSum := 0.0
	alpha := 1.0
	// Iterate the array and call ProductionSchedule for each
	for i, params := range productionScheduleParamsArray {
		// Sum deltaELR and take min alpha
		deltaELRSum += params.deltaElr
		alpha = min(alpha, params.alpha)

		// Skip all but the last entry for output
		if i < len(productionScheduleParamsArray)-1 {
			continue
		}

		switchTime, switchTimestamp, finishTimeWithSwitch, finishTimestampWithSwitch,
			finishTimeWithoutSwitch, finishTimestampWithoutSwitch, err := ProductionSchedule(
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
			builder.WriteString(fmt.Sprintf("Switch Time: %d <t:%s:f>\nFinish Time with Switch: %d <t:%s:f>\nFinish Time without Switch: %d <t:%s:f>\n",
				switchTimestamp, switchTime,
				finishTimestampWithSwitch, finishTimeWithSwitch,
				finishTimestampWithoutSwitch, finishTimeWithoutSwitch))
		}
	}

	if extraInfo {
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
		if eiContract.SeasonalScoring == ei.SeasonalScoringNerfed {
			fmt.Fprintf(&scoresTable, "`%12s %6d %6d %6d`\n",
				bottools.AlignString(cs.name, 12, bottools.StringAlignLeft),
				cs.max, cs.min, cs.base)

		} else {
			fmt.Fprintf(&scoresTable, "`%12s %6d %6d %6d %6d %6d %6d`\n",
				bottools.AlignString(cs.name, 12, bottools.StringAlignLeft),
				cs.max, cs.tval, cs.sink, cs.runs, cs.min, cs.base)

		}
	}

	var siabMax []TeamworkOutputData
	if len(siabEntries) == 0 {
		siabMsg.WriteString("\nNo SiaB swaps needed.\n")
	} else {
		// Sort by time, then by name (for duplicate timestamps)
		sort.Slice(siabEntries, func(i, j int) bool {
			if siabEntries[i].SwitchTime == siabEntries[j].SwitchTime {
				return siabEntries[i].Name < siabEntries[j].Name
			}
			return siabEntries[i].SwitchTime < siabEntries[j].SwitchTime
		})

		// Header
		siabMsg.WriteString(fmt.Sprintf("`%-15s` `%-10s` `%s`\n",
			"PLAYER",
			bottools.AlignString("ΔELR", 10, bottools.StringAlignCenter),
			"TIME"))

		// Print each row
		cantSwitch := false
		for _, e := range siabEntries {
			name := e.Name
			if len(name) > 15 {
				name = name[:15]
			}

			ts := e.SwitchTime
			if time.Unix(ts, 0).After(endTime) {
				if !cantSwitch {
					cantSwitch = true
					siabMsg.WriteString("**Other SiaB users**\n")
				}
				ts = endTime.Unix()
			}

			siabMsg.WriteString(fmt.Sprintf(
				"`%-15s` `%10.6f` <t:%d:t>\n",
				name,
				e.DeltaELR,
				ts,
			))
		}

		siabMsg.WriteString("\n*Using your best SiaB will result in higher CS.*\n")
	}

	siabMax = append(siabMax, TeamworkOutputData{"SIAB", siabMsg.String()})

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
		//if config.IsDevBot() {
		//	fmt.Printf("T/Q: %d/%d ELR: %f  SR: %f  DLV:%f,  maxDeliv: %f\n", stoneSlots-i, i, elr/1e15, sr/1e15, calcDelivery/1e15, maxDelivery/1e15)
		//}
		if calcDelivery > maxDelivery {
			maxDelivery = calcDelivery
		}
	}
	return maxDelivery / delivery, maxDelivery, maxDelivery - future.originalDelivery, swapArtifactName
}
