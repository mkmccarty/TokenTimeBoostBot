package boost

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
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
	str, fields, _ := DownloadCoopStatusTeamwork(contractID, coopID, true)
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

	sendTeamworkPage(s, i, true, cache.xid, false, false, true)

	// Traverse stonesCacheMap and delete expired entries
	for key, cache := range teamworkCacheMap {
		if cache.expirationTimestamp.Before(time.Now()) {
			delete(teamworkCacheMap, key)
		}
	}
}

// DownloadCoopStatusTeamwork will download the coop status for a given contract and coop ID
func DownloadCoopStatusTeamwork(contractID string, coopID string, setContractEstimate bool) (string, map[string][]TeamworkOutputData, ContractScore) {
	var dataTimestampStr string
	var nowTime time.Time

	eiContract := ei.EggIncContractsAll[contractID]
	if eiContract.ID == "" {
		return "Invalid contract ID.", nil, ContractScore{}
	}

	if after, hasPrefix := strings.CutPrefix(coopID, "**"); hasPrefix {
		coopID = after

		// Print working directory and directory being read
		cwd, _ := os.Getwd()
		log.Println("Current working dir:", cwd)
		log.Println("Reading directory:", filepath.Join(cwd, "ttbb-data/pb"))

		// Read directory
		files, err := os.ReadDir("ttbb-data/pb")
		if err != nil {
			log.Println("❌ Error reading directory:", err)
			return "Failed to read ttbb-data directory.", nil, ContractScore{}
		}

		// Build search pattern
		var pattern string
		if coopID == "" {
			pattern = contractID + "-"
		} else {
			pattern = contractID + "-" + coopID
		}
		log.Println("Searching for pattern:", pattern)

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
			return fmt.Sprintf("No matching files found in %s.", filepath.Join(cwd, "ttbb-data/pb")), nil, ContractScore{}
		}
		return fmt.Sprintf("Filenames:\n%s", strings.Join(fileNames, "\n")), nil, ContractScore{}
	}

	coopStatus, nowTime, dataTimestampStr, err := ei.GetCoopStatus(contractID, coopID, "")
	if err != nil {
		return err.Error(), nil, ContractScore{}
	}

	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		return ei.ContractCoopStatusResponse_ResponseStatus_name[int32(coopStatus.GetResponseStatus())], nil, ContractScore{}
	}
	if coopStatus.GetGrade() == ei.Contract_GRADE_UNSET {
		return fmt.Sprintf("No grade found for contract %s/%s", contractID, coopID), nil, ContractScore{}
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
	var calcSecondsRemaining float64
	var contractScores ContractScore

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
		fmt.Fprintf(&builder, "Completed %s contract %s/[**%s**](%s)\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), coopStatus.GetContractIdentifier(), coopStatus.GetCoopIdentifier(), fmt.Sprintf("%s/%s/%s", "https://eicoop-carpet.netlify.app", contractID, coopID))
	} else {
		prefix = "Est. "
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
		calcSecondsRemaining = (totalRequired - totalContributions) / contributionRatePerSecond
		endTime = nowTime.Add(time.Duration(calcSecondsRemaining) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
		fmt.Fprintf(&builder, "In Progress %s %s/[**%s**](%s)\nOn target to complete %s\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), coopStatus.GetContractIdentifier(), coopStatus.GetCoopIdentifier(), fmt.Sprintf("%s/%s/%s", "https://eicoop-carpet.netlify.app", contractID, coopID), bottools.WrapTimestamp(endTime.Unix(), bottools.TimestampRelativeTime))
		if setContractEstimate {
			c := FindContractByIDs(contractID, coopID)
			if c != nil {
				c.mutex.Lock()
				if contributionRatePerSecond > 0 &&
					!math.IsInf(calcSecondsRemaining, 0) &&
					!math.IsNaN(calcSecondsRemaining) &&
					calcSecondsRemaining >= 0 {
					c.EstimatedDuration = time.Duration(calcSecondsRemaining) * time.Second
					c.EstimatedDurationValid = true
					c.StartTime = startTime
					c.EstimatedEndTime = endTime
				} else {
					// Mark estimate as invalid rather than persisting a bad/overflowed duration.
					c.EstimatedDurationValid = false
				}
				c.mutex.Unlock()
			}
		}
	}

	UpdateContractTime(coopStatus.GetContractIdentifier(), coopStatus.GetCoopIdentifier(), startTime, endTime, contractDurationSeconds)

	fmt.Fprintf(&builder, "Start Time: %s at %s\n", bottools.WrapTimestamp(startTime.Unix(), bottools.TimestampLongDate), bottools.WrapTimestamp(startTime.Unix(), bottools.TimestampLongTime))
	fmt.Fprintf(&builder, "%sEnd Time: %s at %s\n", prefix, bottools.WrapTimestamp(endTime.Unix(), bottools.TimestampLongDate), bottools.WrapTimestamp(endTime.Unix(), bottools.TimestampLongTime))
	fmt.Fprintf(&builder, "%sDuration: %v\n", prefix, (endTime.Sub(startTime)).Round(time.Second))
	// Used to collect the return values for each farmer
	var farmerFields = make(map[string][]TeamworkOutputData)

	var DeliveryTimeValues []DeliveryTimeValue

	deliveryTableMap := make(map[string][]DeliveryTimeValue)

	/*
		type ContractScore struct {
			coopID                   string
			contractID               string
			cxpversion               int
			grade                    int
			coopSize                 int
			crRequirement            int
			contractLengthSeconds    int
			targetGoal               float64
			activeContractDurSeconds float64
			playerParamters          []PlayerScoreParameters
		}
	*/

	// Build contractScores for csEstimates
	contractScores = ContractScore{
		coopID:                   coopID,
		contractID:               contractID,
		cxpversion:               eiContract.SeasonalScoring,
		grade:                    grade,
		coopSize:                 eiContract.MaxCoopSize,
		crRequirement:            eiContract.ChickenRuns,
		contractLengthSeconds:    eiContract.Grade[grade].LengthInSeconds,
		targetGoal:               eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
		activeContractDurSeconds: contractDurationSeconds,
	}

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
				fmt.Fprintf(&crBuilder, "%d:%d ", maxCR, scoreCRdiff+int64(math.Ceil(added)))
			}
			crBuilder.WriteString("```")

			if diffCR > 0 {
				fmt.Fprintf(&crBuilder, "\nEach Chicken Run adds `%3.1f` to Contract Score.",
					diffCR)
				if math.Mod(crThreshold, 1.0) == 0.5 {
					// print the half val
					fmt.Fprintf(&crBuilder, "\nNote: CR Threshold is `%3.1f` The final run gives only half credit `%3.1f` (+0.5 CR threshold).", crThreshold, diffCR/2.0)
				}
			}
			str := "Chicken Runs (CR)"
			field = append(field, TeamworkOutputData{str, crBuilder.String()})

		}
		// Create a table of Contract Scores for this user
		var csBuilder strings.Builder
		// No token sharing, with CR to coop size -1
		T := calculateTokenTeamwork(contractDurationSeconds, eiContract.MinutesPerToken, 0.0, 11.0)
		CR := calculateChickenRunTeamwork(eiContract.SeasonalScoring, eiContract.MaxCoopSize, contractDurationInDays, min(eiContract.MaxCoopSize-1, eiContract.ChickenRuns))
		scoreRuns := calculateContractScore(eiContract.SeasonalScoring, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, CR, T)
		fmt.Fprintf(&csBuilder, "Max: %d (CR=%d)\n", scoreRuns, min(eiContract.MaxCoopSize-1, eiContract.ChickenRuns))

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
		fmt.Fprintf(&csBuilder, "Min: %d (CR=0)\n", scoreMin)

		scoreBase := calculateContractScore(eiContract.SeasonalScoring, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			0, 0, 0)
		fmt.Fprintf(&csBuilder, "Base: %d (Buff/CR=0)\n", scoreBase)

		field = append(field, TeamworkOutputData{"Contract Score", csBuilder.String()})

		farmerFields[name] = field

		/*
			type PlayerScoreParameters struct {
				name         string
				contribution float64
				btv          float64
			}
		*/

		// Add player specific parameters to contractScores for csEstimates
		contractScores.playerParamters = append(contractScores.playerParamters, PlayerScoreParameters{
			name:         c.GetUserName(),
			contribution: contribution[i],
			buff:         B,
		})

	}

	builder.WriteString(dataTimestampStr)

	return builder.String(), farmerFields, contractScores
}
