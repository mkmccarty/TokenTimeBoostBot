package boost

import (
	"fmt"
	"log"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

const (
	modeOriginalFormula = 1
	modeStoneHuntMethod = 2
)

var (
	DefaultLeggyTE                = 100.0
	DefaultLeggyDeflectorBonus    = 0.20
	DefaultLeggyMetronome         = 1.35
	DefaultLeggyCompass           = 1.50
	DefaultLeggyGusset            = 1.25
	DefaultLeggyDeliverySlots     = 10.0
	DefaultLeggyIHR               = 7440.0
	DefaultLeggyChalice           = 1.40
	DefaultLeggyMonocle           = 1.30
	DefaultLeggyIHRSlots          = 8.0
	DefaultLeggyChickenRunPercent = 70.0
	teOverrideMinValue            = 0.0
)

// calcLeggyBoost computes the boost tokens and multiplier dynamically based on TE.
func calcLeggyBoost(te float64) (tokens float64, multiplier float64) {
	teMult := math.Pow(1.01, te)
	if teMult > 100.0 {
		tokens = 2.0
	} else if teMult > 4.0 {
		tokens = 4.0
	} else if teMult > 2.0 {
		tokens = 5.0
	} else {
		tokens = 6.0
	}
	multiplier = calcBoostMulti(tokens)
	return tokens, multiplier
}

// GetSlashEstimateTime is the definition of the slash command
func GetSlashEstimateTime(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Get an estimate of completion time of a contract.",
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
				Description:  "Contract ID",
				Required:     false,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "include-leggy",
				Description: "Include estimate for full leggy set.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "te-override",
				Description: "Override default TE (0-490) for this run.",
				Required:    false,
				MinValue:    &teOverrideMinValue,
				MaxValue:    490.0,
			},
		},
	}
}

// HandleEstimateTimeCommand will handle the estimate-contract-time command
func HandleEstimateTimeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var contractID = ""
	var str = ""
	includeLeggySet := false
	optionMap := bottools.GetCommandOptionsMap(i)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{},
	})

	if opt, ok := optionMap["contract-id"]; ok {
		contractID = opt.StringValue()
	} else {
		runningContract := FindContract(i.ChannelID)
		if runningContract != nil {
			contractID = runningContract.ContractID
		}
	}
	if opt, ok := optionMap["include-leggy"]; ok {
		includeLeggySet = opt.BoolValue()
	}

	var teOverride []float64
	if opt, ok := optionMap["te-override"]; ok {
		teOverride = append(teOverride, float64(opt.IntValue()))
	}

	c := ei.EggIncContractsAll[contractID]
	if c.ID == "" {
		str = "No contract found in this channel, use the command parameters to pick one."
	}

	if str == "" {
		estimateText := GetContractEstimateString(contractID, includeLeggySet, teOverride...)
		components := []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: estimateText},
		}

		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Flags:      discordgo.MessageFlagsSuppressEmbeds | discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		})
		return
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: str,
		Flags:   discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsSuppressEmbeds,
	})
}

// GetContractEstimateString returns a string with the estimated completion time of a contract
func GetContractEstimateString(contractID string, includeLeggySet bool, teOverride ...float64) string {

	str := ""
	c := ei.EggIncContractsAll[contractID]
	teVal := DefaultLeggyTE
	hasOverride := false
	if len(teOverride) > 0 {
		teVal = teOverride[0]
		hasOverride = true
	}

	if c.ID != "" && hasOverride {
		cCopy := c
		estAll := getContractDurationEstimate(cCopy, cCopy.TargetAmount[len(cCopy.TargetAmount)-1], float64(cCopy.MaxCoopSize), cCopy.LengthInSeconds,
			cCopy.ModifierSR, cCopy.ModifierELR, cCopy.ModifierHabCap, false, teVal)
		cCopy.EstimatedDuration = estAll.Upper
		cCopy.EstimatedDurationLower = estAll.Lower
		cCopy.EstimatedDurationMax = estAll.Max
		cCopy.EstimatedDurationSIAB = estAll.SIAB
		cCopy.EstimatedDurationMaxGG = estAll.MaxGG
		cCopy.EstimatedDurationSIABGG = estAll.SIABGG
		cCopy.SIABCompass = estAll.SIABCompass
		cCopy.SIABGGCompass = estAll.SIABGGCompass
		cCopy.MaxCompass = estAll.MaxCompass
		cCopy.MaxGGCompass = estAll.MaxGGCompass
		cCopy.UpperCompass = estAll.UpperCompass
		cCopy.LowerCompass = estAll.LowerCompass

		if len(cCopy.TargetAmount) != 0 {
			BTA := math.Floor(float64(cCopy.EstimatedDuration.Minutes() / float64(cCopy.MinutesPerToken)))
			cCopy.TargetTval = 3.0
			if BTA > 42.0 {
				cCopy.TargetTval = 0.07 * BTA
			}
			BTALower := math.Floor(float64(cCopy.EstimatedDurationLower.Minutes() / float64(cCopy.MinutesPerToken)))
			cCopy.TargetTvalLower = 3.0
			if BTALower > 42.0 {
				cCopy.TargetTvalLower = 0.07 * BTALower
			}
		}

		fairShare := 1.00
		if cCopy.SeasonalScoring == ei.SeasonalScoringNerfed {
			fairShare = 1.00
			if cCopy.ID == "quant-blitz" {
				fairShare = 3.85
			}
		}
		if cCopy.MaxCoopSize == 1 {
			fairShare = 1.0
		}
		cCopy.CxpMax = float64(getContractScoreEstimateWithDuration(cCopy, ei.Contract_GRADE_AAA,
			cCopy.EstimatedDurationMax,
			fairShare,
			100, 30,
			20, 0,
			cCopy.ChickenRuns,
			100, 5))

		cCopy.CxpMaxSiab = float64(getContractScoreEstimateWithDuration(cCopy, ei.Contract_GRADE_AAA,
			cCopy.EstimatedDurationSIAB,
			fairShare,
			100, int(cCopy.EstimatedDurationSIAB.Minutes()),
			20, 0,
			cCopy.ChickenRuns,
			100, 5))

		cCopy.CxpMaxGG = float64(getContractScoreEstimateWithDuration(cCopy, ei.Contract_GRADE_AAA,
			cCopy.EstimatedDurationMaxGG,
			fairShare,
			100, 30,
			20, 0,
			cCopy.ChickenRuns,
			100, 5))

		cCopy.CxpMaxSiabGG = float64(getContractScoreEstimateWithDuration(cCopy, ei.Contract_GRADE_AAA,
			cCopy.EstimatedDurationSIABGG,
			fairShare,
			100, int(cCopy.EstimatedDurationSIABGG.Minutes()),
			20, 0,
			cCopy.ChickenRuns,
			100, 5))

		cCopy.Cxp = float64(getContractScoreEstimate(cCopy, ei.Contract_GRADE_AAA,
			true, 1.0,
			fairShare,
			100, 30,
			20, 0,
			cCopy.ChickenRuns,
			100, 5))

		c = cCopy
	}

	if c.ID == "" {
		str = "No contract found  use the command parameters to pick one."
		return str
	}
	eggStr := FindEggEmoji(c.EggName)
	tokenStr, _, _ := ei.GetBotEmoji("token")
	runStr, _, _ := ei.GetBotEmoji("icon_chicken_run")
	seasonalStr := ""
	if c.SeasonID != "" {
		seasonYear := strings.Split(c.SeasonID, "_")[1]
		seasonIcon := strings.Split(c.SeasonID, "_")[0]
		seasonEmote := map[string]string{"winter": "❄️", "spring": "🌷", "summer": "☀️", "fall": "🍂"}
		seasonalStr = fmt.Sprintf("Seasonal: %s %s\n", seasonEmote[seasonIcon], seasonYear)
	}

	// SR sandbox calls
	cxpToggle := false
	if c.ContractVersion == 2 {
		if c.SeasonalScoring == ei.SeasonalScoringNerfed {
			cxpToggle = true
		}
	}
	playerCount := c.MaxCoopSize
	if playerCount < 1 {
		playerCount = 1
	}
	leggyTokens, _ := calcLeggyBoost(teVal)
	leggyTokensStr := strconv.Itoa(int(leggyTokens))
	leggyTEStr := strconv.FormatFloat(teVal, 'f', -1, 64)
	players := make([]SandboxPlayer, 0, playerCount)
	for i := 0; i < playerCount; i++ {
		players = append(players, SandboxPlayer{
			Name:         fmt.Sprintf("Player %d", i),
			Tokens:       leggyTokensStr,
			TE:           leggyTEStr,
			Mirror:       false,
			Colleggtible: true,
			Sink:         i == playerCount-1,
			Creator:      i == 0,
			Item1:        "00",
			Item2:        "00",
			Item3:        "00",
			Item4:        "00",
			Item5:        "00",
			Item6:        "00",
			Item7:        "00",
			Item8:        "00",
		})
	}
	staabData, staabError := EncodeSandboxData(cxpToggle, c.TargetAmount[len(c.TargetAmount)-1],
		strconv.Itoa(c.MinutesPerToken), c.LengthInSeconds, c.MaxCoopSize, &c, players)
	if staabError != nil {
		// Fallback to the default SR Sandbox configuration if encoding fails
		staabData = "data=" + url.QueryEscape("v-5MTEwMDAwMC0wLTEzLTctNTAwLTYwLTEtMS0yLVBsYXllci01LTUw=B6mEavjeExzag")
	}

	str = fmt.Sprintf("%s%s **%s** [%s](%s), [SR Sandbox](https://srsandbox-staabmia.netlify.app/?%s)\n%s%d%s - %s/%dm - %s%d/%dm - 📏%s",
		ei.GetBotEmojiMarkdown("contract_grade_aaa"),
		eggStr, c.Name, c.ID,
		fmt.Sprintf("https://eicoop-carpet.netlify.app/?q=%s", c.ID),
		staabData,
		seasonalStr,
		c.MaxCoopSize, ei.GetBotEmojiMarkdown("icon_coop"),
		tokenStr, c.MinutesPerToken,
		runStr, c.ChickenRuns, c.ChickenRunCooldownMinutes,
		bottools.FmtDuration(time.Duration(c.LengthInSeconds)*time.Second))
	if c.ModifierSR != 1.0 && c.ModifierSR > 0.0 {
		str += fmt.Sprintf(" / 🛻 %1.3gx", c.ModifierSR)
	}
	if c.ModifierELR != 1.0 && c.ModifierELR > 0.0 {
		str += fmt.Sprintf(" / 🥚 %1.3gx", c.ModifierELR)
	}
	if c.ModifierHabCap != 1.0 && c.ModifierHabCap > 0.0 {
		str += fmt.Sprintf(" / 🏠 %1.3gx", c.ModifierHabCap)
	}
	if c.ModifierEarnings != 1.0 && c.ModifierEarnings > 0.0 {
		str += fmt.Sprintf(" / 💰 %1.3gx", c.ModifierEarnings)
	}
	if c.ModifierIHR != 1.0 && c.ModifierIHR > 0.0 {
		str += fmt.Sprintf(" / 🐣 %1.3gx", c.ModifierIHR)
	}
	if c.ModifierAwayEarnings != 1.0 && c.ModifierAwayEarnings > 0.0 {
		str += fmt.Sprintf(" / 🏝️💰 %1.3gx", c.ModifierAwayEarnings)
	}
	if c.ModifierVehicleCost != 1.0 && c.ModifierVehicleCost > 0.0 {
		str += fmt.Sprintf(" / 🚗💲 %1.3gx", c.ModifierVehicleCost)
	}
	if c.ModifierResearchCost != 1.0 && c.ModifierResearchCost > 0.0 {
		str += fmt.Sprintf(" / 📚💲 %1.3gx", c.ModifierResearchCost)
	}
	if c.ModifierHabCost != 1.0 && c.ModifierHabCost > 0.0 {
		str += fmt.Sprintf(" / 🏠💲 %1.3gx", c.ModifierHabCost)
	}
	str += "\n"
	estStr := c.EstimatedDuration.Round(time.Minute).String()
	estStr = strings.TrimRight(estStr, "0s")
	if c.UpperCompass {
		estStr += " (no compass)"
	}
	estStrLower := c.EstimatedDurationLower.Round(time.Minute).String()
	estStrLower = strings.TrimRight(estStrLower, "0s")
	if c.LowerCompass {
		estStrLower += " (no compass)"
	}

	/*
		upperEstEmotes := fmt.Sprintf("%s%s%s%s",
			ei.GetBotEmojiMarkdown("defl_T4C"),
			ei.GetBotEmojiMarkdown("metr_T4L"),
			ei.GetBotEmojiMarkdown("comp_T4L"),
			ei.GetBotEmojiMarkdown("gusset_T4L"))
		lowerEstEmotes := fmt.Sprintf("%s%s%s%s%s%s",
			ei.GetBotEmojiMarkdown("defl_T4L"),
			ei.GetBotEmojiMarkdown("metr_T4L"),
			ei.GetBotEmojiMarkdown("comp_T4L"),
			ei.GetBotEmojiMarkdown("gusset_T4L"),
			ei.GetBotEmojiMarkdown("egg_carbonfiber"),
			ei.GetBotEmojiMarkdown("egg_pumpkin"))
	*/
	// A speedrun or fastrun of $CONTRACT with $NUMBER farmer(s) needing to ship $GOAL eggs is estimated to take about $TIME
	options := map[string]any{
		"decimals": 2,
		"trim":     true,
	}
	str += fmt.Sprintf("**%v** - **%v** for a fastrun needing to ship **%s** eggs\n",
		estStrLower,
		estStr,
		ei.FormatEIValue(c.TargetAmount[len(c.TargetAmount)-1], options))

	footerAboutCR := false

	if c.ContractVersion == 2 {
		// Two ranges of estimates
		// Speedrun w/ perfect set through to Sink with decent set
		// Fair share from 1.05 to 0.92
		scoreSink := getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
			c.EstimatedDurationLower, // Use faster duration at a 1.0 modifier
			0.92,                     // 0.92 Fair Share (Last booster sink)
			60, 45,                   // T4C SIAB for 45m
			15, 0, // T4C Deflector for full duration
			c.MaxCoopSize-1, // All Chicken Runs
			3, 100)          // Sink token use, sent at least 3 (max) and received a lot

		// Use the current season Grade AAA final CXP goal from periodicals as the numerator.
		gradeAAAFinalCxpGoal := ei.GetEggIncCurrentSeasonAAAFinalCxpGoal()
		if gradeAAAFinalCxpGoal <= 0 {
			gradeAAAFinalCxpGoal = 800_000.0
		}

		prevSeasonScore := 980_000.0
		if value := strings.TrimSpace(guildstate.GetGuildSettingString("DEFAULT", "top_season_score")); value != "" {
			if parsed, err := strconv.ParseFloat(value, 64); err == nil && parsed > 0 {
				prevSeasonScore = parsed
			}
		}
		csTarget := gradeAAAFinalCxpGoal / prevSeasonScore

		if c.SeasonalScoring != ei.SeasonalScoringNerfed { // Leggacies originally released before Sept 22, 2025
			var cs strings.Builder
			fmt.Fprintf(&cs, "CS Est: **%d** ", int64(c.Cxp))
			if c.ChickenRuns > c.MaxCoopSize-1 {
				footerAboutCR = true
				//fmt.Fprintf(&cs, "*+%.0f/%s* ", c.CxpRunDelta, ei.GetBotEmojiMarkdown("icon_chicken_run"))
			}
			cs.WriteString("(SR)")
			if c.MaxCoopSize > 1 {
				fmt.Fprintf(&cs, " - **%d** (Sink) ", scoreSink)
			}

			fmt.Fprintf(&cs, " - **%.0f** ", c.Cxp*csTarget) // 82% of max CXP for a fair share estimate
			if c.SeasonID == "" {
				cs.WriteString(("(Low Target)\n"))
			} else {
				cs.WriteString(("(Seasonal Target)\n"))
			}
			str += cs.String()
		} else { // Seasonal contracts released starting Sept 22, 2025
			str += fmt.Sprintf("CS Est: **%d** (SR) - **%.0f** (Seasonal Target)\n",
				int64(c.Cxp),
				c.Cxp*csTarget)
		}
		if includeLeggySet {
			gg, ugg, _ := ei.GetGenerousGiftEvent()
			ggicon := ""
			if gg > 1.0 {
				ggicon = " " + ei.GetBotEmojiMarkdown("std_gg")
			}
			if ugg > 1.0 {
				// farmers with ultra
				//gg = ugg + (float64(contract.UltraCount) / float64(contract.CoopSize))
				ggicon = " " + ei.GetBotEmojiMarkdown("ultra_gg")
			}

			estStrMax := c.EstimatedDurationMax.Round(time.Minute).String()
			estStrMax = strings.TrimRight(estStrMax, "0s")
			maxComment := ""
			if c.MaxCompass {
				maxComment = " (no compass, 11 stones)"
			}
			str += fmt.Sprintf("Leggy Set: **%s** CS:**%d**%s", estStrMax, int64(c.CxpMax), maxComment)

			if c.CxpMaxSiab > c.CxpMax {
				estStrMaxSiab := c.EstimatedDurationSIAB.Round(time.Minute).String()
				estStrMaxSiab = strings.TrimRight(estStrMaxSiab, "0s")
				siabComment := "no gusset, 9 stones"
				if c.SIABCompass {
					siabComment = "no compass, 10 stones"
				}
				str += fmt.Sprintf(" / %s **%s** CS:**%d** (%s)\n", ei.GetBotEmojiMarkdown("SIAB_T4L"), estStrMaxSiab, int64(c.CxpMaxSiab), siabComment)
			} else {
				str += "\n"
			}

			if ggicon != "" {
				estStrGG := c.EstimatedDurationMaxGG.Round(time.Minute).String()
				estStrGG = strings.TrimRight(estStrGG, "0s")
				maxGGComment := ""
				if c.MaxGGCompass {
					maxGGComment = " (no compass, 11 stones)"
				}
				str += fmt.Sprintf("%s **%s** CS:**%d**%s", ggicon, estStrGG, int64(c.CxpMaxGG), maxGGComment)
				if c.CxpMaxSiab > c.CxpMax {
					estStrGG := c.EstimatedDurationSIABGG.Round(time.Minute).String()
					estStrGG = strings.TrimRight(estStrGG, "0s")
					str += fmt.Sprintf(" / %s **%s** CS:**%d**", ei.GetBotEmojiMarkdown("SIAB_T4L"), estStrGG, int64(c.CxpMaxSiabGG))
				}
				str += fmt.Sprintf(" (6%s)\n", ei.GetBotEmojiMarkdown("token"))
			}

			str += fmt.Sprintf("-# Leggy set %.0f TE, %.0f IHR & %.0f delivery stone sets, 1.0 fair share, %.0f%s boost.\n",
				teVal, DefaultLeggyIHRSlots, DefaultLeggyDeliverySlots, leggyTokens, ei.GetBotEmojiMarkdown("token"))
		}
		if footerAboutCR && c.MaxCoopSize > 1 {
			str += fmt.Sprintf("-# CoopSize-1 used for CR, extras **+%.0f**/%s\n",
				c.CxpRunDelta,
				ei.GetBotEmojiMarkdown("icon_chicken_run"))
		}

		if c.SeasonalScoring != ei.SeasonalScoringNerfed && c.MaxCoopSize > 1 {
			if math.Round(c.TargetTval*100)/100 == math.Round(c.TargetTvalLower*100)/100 {
				str += fmt.Sprintf("Target TVal: **%.2f**\n", c.TargetTval)
			} else {
				str += fmt.Sprintf("Target TVal: **%.2f** for lower estimate\n", c.TargetTvalLower)
				str += fmt.Sprintf("Target TVal: **%.2f** for upper estimate\n", c.TargetTval)
			}
		}
	}

	noteStr := ""
	if c.ContractVersion == 1 {
		noteStr = fmt.Sprintf("**ELITE V1 contract** last seen <t:%d:D>.\n", c.ValidFrom.Unix())
	} else if c.ValidUntil.Before(time.Now().UTC()) {
		noteStr = fmt.Sprintf("**Unavailable V2 contract** last seen <t:%d:D>.\n", c.ValidFrom.Unix())
		noteStr += getContractPredLine(c.ID)
	}

	return noteStr + str
}

// getContractPredLine returns a single prediction line for an unavailable contract,
// including the ultra prediction date on the same line if the contract is a PE ultra.
func getContractPredLine(contractID string) string {
	c, ok := ei.EggIncContractsAll[contractID]
	if !ok {
		return ""
	}

	_, wedTime, friTime, _ := contractTimes9amPacific(0)

	wed, friPE, friUltra := GetPredictionBrackets()

	var bracket []ei.EggIncContract
	var baseTime time.Time
	switch {
	case c.HasPE && !c.Ultra:
		bracket, baseTime = friUltra, friTime
	case c.HasPE && c.Ultra:
		bracket, baseTime = friPE, friTime
	default:
		bracket, baseTime = wed, wedTime
	}

	pos := -1
	for idx, bc := range bracket {
		if bc.ID == contractID {
			pos = idx
			break
		}
	}
	if pos < 0 {
		return ""
	}

	predDate := baseTime.AddDate(0, 0, 7*pos)
	line := fmt.Sprintf("-# 🔮 %s", bottools.WrapTimestamp(predDate.Unix(), bottools.TimestampShortDate))

	if c.HasPE && !c.Ultra {
		pePos := pos + len(friPE)
		ultraDate := friTime.AddDate(0, 0, 7*pePos)
		line += fmt.Sprintf(" · %s %s", ei.GetBotEmojiMarkdown("ultra"), bottools.WrapTimestamp(ultraDate.Unix(), bottools.TimestampShortDate))
	}

	return line + "\n"
}

// calculateSingleEstimate computes the completion duration for a contract
// using the provided artifact and player configuration.
// Returns the estimated duration in hours.
func calculateSingleEstimate(
	est estimatePlayer,
	c ei.EggIncContract,
	contractEggsTotal float64,
	numFarmers float64,
	contractLengthInSeconds int,
	modifierSR float64,
	modifierELR float64,
	modifierHabCap float64,
	deflectorsOnFarmer float64,
	debug bool,
) float64 {
	modHab := modifierHabCap
	modELR := modifierELR
	modShip := modifierSR

	slots := est.deliverySlots
	deflectorBonus := est.deflectorBonus
	colELR := est.colELR
	colShip := est.colShip
	colHab := est.colHab

	// Base rate with T4L Metronome +35% and T4L Gusset +25%
	baseELR := 3.772 * est.metronome * est.gusset
	// Base rate with T4L Compass +50%
	baseShipping := 7.148 * est.compass
	maxShipping := baseShipping * math.Pow(1.05, slots) * colShip
	contractBaseELR := baseELR * modELR * modHab
	contractShipCap := maxShipping * modShip
	deflectorMultiplier := 1.0 + deflectorBonus*deflectorsOnFarmer
	bestTotal := 0.0
	intSlots := int(slots)

	if debug {
		log.Printf("id: %v\n", est.id)
		log.Printf("slots: %v\n", slots)
		log.Printf("modELR: %v\n", modELR)
		log.Printf("modShip: %v\n", modShip)
		log.Printf("modHab: %v\n", modHab)
		log.Printf("colELR: %v\n", colELR)
		log.Printf("colShip: %v\n", colShip)
		log.Printf("colHab: %v\n", colHab)
		log.Printf("baseELR: %v\n", baseELR)
		log.Printf("baseShipping: %v\n", baseShipping)
		log.Printf("maxShipping: %v\n", maxShipping)
		log.Printf("contractBaseELR: %v\n", contractBaseELR)
		log.Printf("contractShipCap: %v\n", contractShipCap)
		log.Printf("deflectorMultiplier: %v\n", deflectorMultiplier)
	}

	if est.calcMode == modeStoneHuntMethod {
		tachStones := 0
		quantStones := 0
		bestELR := 0.0
		bestSR := 0.0
		for i := 0; i <= intSlots; i++ {
			stoneLayRate := contractBaseELR
			stoneLayRate *= deflectorMultiplier
			stoneLayRate *= math.Pow(1.05, float64(i)) * colELR * colHab

			stoneShipRate := baseShipping * math.Pow(1.05, float64((intSlots-i))) * colShip * modShip

			bestMin := min(stoneLayRate, stoneShipRate)
			if bestMin > bestTotal {
				bestTotal = bestMin
				tachStones = i
				quantStones = intSlots - i
				bestELR = stoneLayRate
				bestSR = stoneShipRate
				est.contractELR = stoneLayRate
			} else if bestTotal > 0 {
				break
			}
		}
		if debug {
			log.Printf("tachStones: %v\n", tachStones)
			log.Printf("quantStones: %v\n", quantStones)
			log.Printf("bestELR: %v\n", bestELR)
			log.Printf("bestSR: %v\n", bestSR)
			log.Printf("boundedELR: %v\n", bestTotal)
		}
	} else {
		// Original formula method from Halcyon
		tachStones := slots +
			((modShip * colShip) / (modELR * colELR * modHab * colHab)) -
			deflectorsOnFarmer*slots/(slots+(modShip*colShip)/(modELR*colELR*modHab*colHab))
		tachBounded := max(0.0, min(slots, tachStones))
		tachMultiplier := math.Pow(1.05, tachBounded)
		contractELR := contractBaseELR * deflectorMultiplier * tachMultiplier
		est.contractELR = contractELR
		bestTotal = min(contractShipCap, contractELR)
		if debug {
			log.Printf("tachStones: %v\n", tachStones)
			log.Printf("tachBounded: %v\n", tachBounded)
			log.Printf("tachMultiplier: %v\n", tachMultiplier)
			log.Printf("contractELR: %v\n", contractELR)
			log.Printf("boundedELR: %v\n", bestTotal)
		}
	}

	est.boundedELR = bestTotal
	eggsTotal := contractEggsTotal / 1e15
	timerTokens := float64(c.MinutesPerToken) / 60.0
	tokenRate := (6.0 * est.generousGifts) + timerTokens

	tokensPerHourAllPlayers := tokenRate * numFarmers
	hoursPerTokenAllPlayers := 1.0 / tokensPerHourAllPlayers
	rampUpHours := est.boostTokens * hoursPerTokenAllPlayers

	unusedRatioELR := max(1.0, est.contractELR/bestTotal)
	population := (14_175_000_000 * est.colHab) / unusedRatioELR
	populationForCR := population * (est.chickenRunPercent / 100.0)
	crPopulation := populationForCR * 0.05 * max(1.0, numFarmers-1.0)
	if numFarmers == 1 {
		crPopulation = population
	}
	adjustedPop := max(populationForCR, population-crPopulation)

	ihr := est.ihr * est.chalice * math.Pow(1.04, est.ihrSlots) * est.colIHR
	ihr *= math.Pow(1.01, est.te)
	boostTime := adjustedPop / (ihr * 12 * (est.monocle * est.boostMultiplier)) / 60

	if debug {
		log.Printf("ihr: %v\n", ihr)
		log.Printf("unusedRatioELR: %v\n", unusedRatioELR)
		log.Printf("Useful population: %v\n", population)
		log.Printf("populationForCR: %v\n", populationForCR)
		log.Printf("crPopulation: %v\n", crPopulation)
		log.Printf("adjustedPop: %v\n", adjustedPop)
		log.Print("boostTime (h): ", boostTime)
		log.Printf("rampUpHours (before boost): %v\n", rampUpHours)
	}

	// For short contracts, use a two-phase boost: 4 tokens for 2 minutes, then 8 tokens
	if float64(contractLengthInSeconds) < 45*60 {
		twoPhaseDuration := calculateTwoPhaseBoostedEstimate(est, contractEggsTotal, contractLengthInSeconds, modELR, deflectorsOnFarmer, debug)
		rampUpHours += twoPhaseDuration
		if debug {
			log.Print("Two-phase boost (4tok for 2min, then 8tok): ", time.Duration(twoPhaseDuration*float64(time.Hour)))
		}
	} else if est.calcMode == modeOriginalFormula {
		rampUpHours += (est.boostTokens / tokenRate) + (10.0 / 60.0)
	} else {
		rampUpHours += (est.boostTokens / tokenRate) + boostTime
	}

	rampUpDeliveries := 0.5 * (numFarmers * est.boundedELR) * rampUpHours
	remainingEggs := max(0, eggsTotal-rampUpDeliveries)
	steadyStateTime := remainingEggs / (numFarmers * est.boundedELR)
	estimate := min(float64(c.LengthInSeconds)/3600.0, rampUpHours+steadyStateTime)

	if debug {
		log.Printf("tokenRate: %v\n", tokenRate)
		log.Printf("rampUpHours: %v\n", rampUpHours)
		log.Printf("rampUpDeliveries: %v\n", rampUpDeliveries)
		log.Printf("remainingEggs: %v\n", remainingEggs)
		log.Printf("steadyStateTime: %v\n", steadyStateTime)
		log.Printf("estimate (hours): %v\n", estimate)
	}

	return estimate
}

// calculateTwoPhaseBoostedEstimate computes the completion duration for a short contract
// where a single booster starts with 4 tokens for 2 minutes, then switches to 8 tokens.
// This is optimized for short contracts with one active booster using IHR artifacts.
// Returns the estimated duration in hours.
func calculateTwoPhaseBoostedEstimate(
	est estimatePlayer,
	contractEggsTotal float64,
	contractLengthInSeconds int,
	modifierELR float64,
	deflectorsOnFarmer float64,
	debug bool,
) float64 {
	// Phase 1: 4 tokens for 2 minutes (120 seconds)
	phase1Seconds := 120.0
	phase1Est := est
	phase1Est.boostTokens = 4.0
	phase1Est.boostMultiplier = calcBoostMulti(phase1Est.boostTokens) * est.monocle

	// Phase 2: 8 tokens for remaining time
	phase2Est := est
	phase2Est.boostTokens = 8.0
	phase2Est.boostMultiplier = calcBoostMulti(phase2Est.boostTokens) * est.monocle

	// Compute IHR (per hour) for both phases using artifact set and boost multiplier
	ihrPhase1 := est.ihr * est.chalice * math.Pow(1.04, est.ihrSlots) * est.colIHR
	ihrPhase1 *= math.Pow(1.01, est.te)
	ihrPhase1 *= 12 * phase1Est.boostMultiplier

	ihrPhase2 := est.ihr * est.chalice * math.Pow(1.04, est.ihrSlots) * est.colIHR
	ihrPhase2 *= math.Pow(1.01, est.te)
	ihrPhase2 *= 12 * phase2Est.boostMultiplier

	// ELR baseline (per hour) scaled similarly to the debug approach
	deflectorMultiplier := 1.0 + est.deflectorBonus*deflectorsOnFarmer
	myELRPerHour := 252720.0 * est.colELR * modifierELR * deflectorMultiplier / 60.0

	// Population carrying capacity approximation based on bounded ELR
	unusedRatioELR := max(1.0, est.contractELR/est.boundedELR)
	habCapacity := (14_175_000_000 * est.colHab) / unusedRatioELR
	maxPop := habCapacity

	// Initial (seed) population; follow prior debug convention
	initialPop := 10_000_000.0

	// Lay baseline per hour scaled (debug used 10M factor)
	layingRatePerHourBaseline := myELRPerHour * 10_000_000.0

	// Phase 1 simulation: eggs delivered and end-state using EI simulator logic over fixed time
	phase1EggsDelivered, popAfterPhase1, layingStepAfterPhase1 := simulateEggsDeliveredForSeconds(
		initialPop,
		maxPop,
		ihrPhase1/60.0,            // growth per minute
		layingRatePerHourBaseline, // baseline laying per hour
		int(phase1Seconds),
	)

	// Remaining eggs for phase 2 (use raw contract eggs; simulator expects raw units)
	remainingEggs := max(0.0, contractEggsTotal-phase1EggsDelivered)

	// Start phase 2 from the end-of-phase-1 state
	// Convert end-of-phase-1 laying step back to per hour baseline
	layingRatePerHourPhase2Start := layingStepAfterPhase1 * 3600.0

	// Use EI simulator to get time (seconds) to deliver remaining eggs in phase 2
	phase2Seconds := ei.TimeToDeliverEggsInSeconds(
		popAfterPhase1,
		maxPop,
		ihrPhase2/60.0,
		layingRatePerHourPhase2Start,
		remainingEggs,
	)

	totalSeconds := phase1Seconds + phase2Seconds
	totalHours := totalSeconds / 3600.0
	estimate := min(float64(contractLengthInSeconds)/3600.0, totalHours)

	if debug {
		log.Printf("Phase 1 (4tok, 2min): IHR=%.0f, eggs=%.3f, endPop=%.0f\n", ihrPhase1, phase1EggsDelivered, popAfterPhase1)
		log.Printf("Phase 2 (8tok): IHR=%.0f, remaining eggs=%.3f, duration=%.3f hours\n", ihrPhase2, remainingEggs, phase2Seconds/3600.0)
		log.Printf("Two-phase total: %.3f hours\n", estimate)
	}

	return estimate
}

// simulateEggsDeliveredForSeconds mirrors the EI simulator step logic to compute eggs delivered
// over a fixed number of seconds, returning eggs delivered, final population, and final laying
// rate per step (seconds). This allows chaining phases with changing boost setups.
func simulateEggsDeliveredForSeconds(initialPop, maxPop, growthRatePerMinute, layingRatePerHour float64, seconds int) (eggsDelivered float64, finalPop float64, finalLayingRatePerStep float64) {
	if initialPop <= 0 || maxPop <= 0 || growthRatePerMinute <= 0 || layingRatePerHour <= 0 || seconds <= 0 {
		return 0, initialPop, layingRatePerHour / 3600.0
	}

	timeStepSeconds := 1.0
	layingRatePerStep := (layingRatePerHour / 3600.0) * timeStepSeconds
	growthRatePerStep := (growthRatePerMinute / 60.0) * timeStepSeconds

	currentPop := initialPop
	totalEggs := 0.0

	for i := 0; i < seconds; i++ {
		// Eggs delivered in this step
		totalEggs += layingRatePerStep

		// Population growth and rate adjustment
		if currentPop <= maxPop {
			oldPop := currentPop
			currentPop += growthRatePerStep
			if currentPop > maxPop {
				currentPop = maxPop
			}
			popIncrease := currentPop - oldPop
			layingRatePerStep *= (1 + popIncrease/oldPop)
		}
	}

	return totalEggs, currentPop, layingRatePerStep
}

type estimatePlayer struct {
	id                string
	deflectorBonus    float64
	boostTokens       float64
	boostMultiplier   float64
	colELR            float64
	colShip           float64
	colHab            float64
	colIHR            float64
	calcMode          int
	boundedELR        float64
	contractELR       float64
	metronome         float64 // Delivery artifacts
	gusset            float64
	compass           float64
	deliverySlots     float64
	ihr               float64 // IHR artifacts
	te                float64
	chalice           float64
	monocle           float64
	ihrSlots          float64
	chickenRunPercent float64
	generousGifts     float64
}

// ContractDurationEstimate groups the various duration estimates
// produced by getContractDurationEstimate.
type ContractDurationEstimate struct {
	Upper         time.Duration
	Lower         time.Duration
	Max           time.Duration
	SIAB          time.Duration
	MaxGG         time.Duration
	SIABGG        time.Duration
	SIABCompass   bool
	SIABGGCompass bool
	MaxCompass    bool
	MaxGGCompass  bool
	UpperCompass  bool
	LowerCompass  bool
}

// getContractDurationEstimate returns three estimated durations (upper, lower, and max) of a contract based on great and well equipped artifact sets
func getContractDurationEstimate(c ei.EggIncContract, contractEggsTotal float64, numFarmers float64, contractLengthInSeconds int, modifierSR float64, modifierELR float64, modifierHabCap float64, debug bool, teOverride ...float64) ContractDurationEstimate {

	contractDuration := time.Duration(contractLengthInSeconds) * time.Second
	modHab := modifierHabCap
	modELR := modifierELR
	modShip := modifierSR

	collectibleELR, colllectibleShip, colleggtibleHab, colleggtiblesIHR := ei.GetColleggtibleValues()
	teVal := DefaultLeggyTE
	if len(teOverride) > 0 {
		teVal = teOverride[0]
	}
	leggyTokens, leggyMult := calcLeggyBoost(teVal)

	deflectorsOnFarmer := numFarmers - 1.0

	estimates := []estimatePlayer{
		{
			id:                "basic_set",
			deflectorBonus:    0.15,
			boostTokens:       6.0,
			boostMultiplier:   calcBoostMulti(6.0),
			colELR:            1.0,
			colShip:           1.0,
			colHab:            1.0,
			colIHR:            1.0,
			calcMode:          modeStoneHuntMethod,
			metronome:         1.32, // T3E
			compass:           1.45, // average
			gusset:            1.24, // T4L
			deliverySlots:     8.0,  // general average of stones
			ihr:               7440.0,
			te:                0,
			chalice:           1.3, // T4C
			monocle:           1.2, // T4C
			ihrSlots:          7.0, // any(3),chalice(0),monocle(0),any(3)
			chickenRunPercent: 70.0,
			generousGifts:     1.0,
		},
		{
			id:                "solid_set",
			deflectorBonus:    0.17,
			boostTokens:       6.0,
			boostMultiplier:   calcBoostMulti(6.0),
			colELR:            collectibleELR,
			colShip:           colllectibleShip,
			colHab:            colleggtibleHab,
			colIHR:            colleggtiblesIHR,
			calcMode:          modeStoneHuntMethod,
			metronome:         1.35, // T4L
			compass:           1.5,  // T4L
			gusset:            1.25, // T4L
			deliverySlots:     9.0,  // defl(1), metr(3), comp(2), gusset(3)
			ihr:               7440.0,
			te:                0,
			chalice:           1.4, // T4L
			monocle:           1.3, // T4L
			ihrSlots:          8.0, // solid set, Deflector w/o IHR stones
			chickenRunPercent: 70.0,
			generousGifts:     1.0,
		},
		{
			id:                "leggy_set",
			deflectorBonus:    DefaultLeggyDeflectorBonus,
			boostTokens:       leggyTokens,
			boostMultiplier:   leggyMult,
			colELR:            collectibleELR,
			colShip:           colllectibleShip,
			colHab:            colleggtibleHab,
			colIHR:            colleggtiblesIHR,
			calcMode:          modeStoneHuntMethod,
			metronome:         DefaultLeggyMetronome,
			compass:           DefaultLeggyCompass,
			gusset:            DefaultLeggyGusset,
			deliverySlots:     DefaultLeggyDeliverySlots,
			ihr:               DefaultLeggyIHR,
			te:                teVal,
			chalice:           DefaultLeggyChalice,
			monocle:           DefaultLeggyMonocle,
			ihrSlots:          DefaultLeggyIHRSlots,
			chickenRunPercent: DefaultLeggyChickenRunPercent,
			generousGifts:     1.0,
		},
		{
			// Full leggacy set with TE boosts of dynamic tokens, using SIAB instead of gusset (9 delivery slots)
			id:                "leggy_siab",
			deflectorBonus:    DefaultLeggyDeflectorBonus,
			boostTokens:       leggyTokens,
			boostMultiplier:   leggyMult,
			colELR:            collectibleELR,
			colShip:           colllectibleShip,
			colHab:            colleggtibleHab,
			colIHR:            colleggtiblesIHR,
			calcMode:          modeStoneHuntMethod,
			metronome:         DefaultLeggyMetronome,
			compass:           DefaultLeggyCompass,
			gusset:            1.0, // N/A - using SIAB instead of gusset
			deliverySlots:     DefaultLeggyDeliverySlots - 1.0,
			ihr:               DefaultLeggyIHR,
			te:                teVal,
			chalice:           DefaultLeggyChalice,
			monocle:           DefaultLeggyMonocle,
			ihrSlots:          DefaultLeggyIHRSlots,
			chickenRunPercent: DefaultLeggyChickenRunPercent,
			generousGifts:     1.0,
		},
		{
			id:                "leggy_set_gg",
			deflectorBonus:    DefaultLeggyDeflectorBonus,
			boostTokens:       6.0,
			boostMultiplier:   calcBoostMulti(6.0),
			colELR:            collectibleELR,
			colShip:           colllectibleShip,
			colHab:            colleggtibleHab,
			colIHR:            colleggtiblesIHR,
			calcMode:          modeStoneHuntMethod,
			metronome:         DefaultLeggyMetronome,
			compass:           DefaultLeggyCompass,
			gusset:            DefaultLeggyGusset,
			deliverySlots:     DefaultLeggyDeliverySlots,
			ihr:               DefaultLeggyIHR,
			te:                teVal,
			chalice:           DefaultLeggyChalice,
			monocle:           DefaultLeggyMonocle,
			ihrSlots:          DefaultLeggyIHRSlots,
			chickenRunPercent: DefaultLeggyChickenRunPercent,
			generousGifts:     2.0,
		},
		{
			id:                "leggy_siab_gg",
			deflectorBonus:    DefaultLeggyDeflectorBonus,
			boostTokens:       6.0,
			boostMultiplier:   calcBoostMulti(6.0),
			colELR:            collectibleELR,
			colShip:           colllectibleShip,
			colHab:            colleggtibleHab,
			colIHR:            colleggtiblesIHR,
			calcMode:          modeStoneHuntMethod,
			metronome:         DefaultLeggyMetronome,
			compass:           DefaultLeggyCompass,
			gusset:            1.0, // N/A - using SIAB instead of gusset
			deliverySlots:     DefaultLeggyDeliverySlots - 1.0,
			ihr:               DefaultLeggyIHR,
			te:                teVal,
			chalice:           DefaultLeggyChalice,
			monocle:           DefaultLeggyMonocle,
			ihrSlots:          DefaultLeggyIHRSlots,
			chickenRunPercent: DefaultLeggyChickenRunPercent,
			generousGifts:     2.0,
		},
	}

	var estimateDurationUpper time.Duration
	var estimateDurationLower time.Duration
	var estimateDurationMax time.Duration
	var estimateDurationSIAB time.Duration
	var estimateDurationMaxGG time.Duration
	var estimateDurationSIABGG time.Duration

	siabCompassSwap := false
	siabGGCompassSwap := false
	maxCompassSwap := false
	maxGGCompassSwap := false
	upperCompassSwap := false
	lowerCompassSwap := false

	for _, est := range estimates {
		estimate := calculateSingleEstimate(est, c, contractEggsTotal, numFarmers,
			contractLengthInSeconds, modShip, modELR, modHab, deflectorsOnFarmer, debug)

		switch est.id {
		case "basic_set":
			estimateDurationUpper = time.Duration(estimate * float64(time.Hour))
			if modELR < 1.0 {
				// Try Config: Swap Compass for 3-slot any
				estAlt := est
				estAlt.compass = 1.0
				estAlt.deliverySlots = 9.0 // 8.0 + 1.0
				estAltVal := calculateSingleEstimate(estAlt, c, contractEggsTotal, numFarmers,
					contractLengthInSeconds, modShip, modELR, modHab, deflectorsOnFarmer, debug)
				estAltDur := time.Duration(estAltVal * float64(time.Hour))
				if estAltDur < estimateDurationUpper {
					estimateDurationUpper = estAltDur
					upperCompassSwap = true
				}
			}
			if debug {
				log.Printf("estimateDurationUpper: %v (compass swap: %v)\n", estimateDurationUpper, upperCompassSwap)
				log.Print("--------------------\n")
			}
		case "solid_set":
			estimateDurationLower = time.Duration(estimate * float64(time.Hour))
			if modELR < 1.0 {
				// Try Config: Swap Compass for 3-slot any
				estAlt := est
				estAlt.compass = 1.0
				estAlt.deliverySlots = 10.0 // 9.0 + 1.0
				estAltVal := calculateSingleEstimate(estAlt, c, contractEggsTotal, numFarmers,
					contractLengthInSeconds, modShip, modELR, modHab, deflectorsOnFarmer, debug)
				estAltDur := time.Duration(estAltVal * float64(time.Hour))
				if estAltDur < estimateDurationLower {
					estimateDurationLower = estAltDur
					lowerCompassSwap = true
				}
			}
			if debug {
				log.Printf("estimateDurationLower: %v (compass swap: %v)\n", estimateDurationLower, lowerCompassSwap)
			}
		case "leggy_set":
			estimateDurationMax = time.Duration(estimate * float64(time.Hour))
			if modELR < 1.0 {
				// Try Config: Swap Compass for 3-slot any
				estAlt := est
				estAlt.compass = 1.0
				estAlt.deliverySlots = 11.0
				estAltVal := calculateSingleEstimate(estAlt, c, contractEggsTotal, numFarmers,
					contractLengthInSeconds, modShip, modELR, modHab, deflectorsOnFarmer, debug)
				estAltDur := time.Duration(estAltVal * float64(time.Hour))
				if estAltDur < estimateDurationMax {
					estimateDurationMax = estAltDur
					maxCompassSwap = true
				}
			}
			if debug {
				log.Printf("estimateDurationMax: %v (compass swap: %v)\n", estimateDurationMax, maxCompassSwap)
			}
		case "leggy_siab":
			estimateDurationSIAB = time.Duration(estimate * float64(time.Hour))
			if modELR < 1.0 {
				// Try Config B: Swap Compass for SIAB
				estB := est
				estB.compass = 1.0
				estB.gusset = 1.25
				estB.deliverySlots = 10.0
				estBVal := calculateSingleEstimate(estB, c, contractEggsTotal, numFarmers,
					contractLengthInSeconds, modShip, modELR, modHab, deflectorsOnFarmer, debug)
				estBDur := time.Duration(estBVal * float64(time.Hour))

				// Try Config C: Swap Compass for 3-slot any
				estC := est
				estC.compass = 1.0
				estC.gusset = 1.0
				estC.deliverySlots = 10.0
				estCVal := calculateSingleEstimate(estC, c, contractEggsTotal, numFarmers,
					contractLengthInSeconds, modShip, modELR, modHab, deflectorsOnFarmer, debug)
				estCDur := time.Duration(estCVal * float64(time.Hour))

				if estBDur < estimateDurationSIAB || estCDur < estimateDurationSIAB {
					siabCompassSwap = true
					if estBDur < estCDur {
						estimateDurationSIAB = estBDur
					} else {
						estimateDurationSIAB = estCDur
					}
				}
			}
			if debug {
				log.Printf("estimateDurationSIAB: %v (compass swap: %v)\n", estimateDurationSIAB, siabCompassSwap)
			}
		case "leggy_set_gg":
			estimateDurationMaxGG = time.Duration(estimate * float64(time.Hour))
			if modELR < 1.0 {
				// Try Config: Swap Compass for 3-slot any
				estAlt := est
				estAlt.compass = 1.0
				estAlt.deliverySlots = 11.0
				estAltVal := calculateSingleEstimate(estAlt, c, contractEggsTotal, numFarmers,
					contractLengthInSeconds, modShip, modELR, modHab, deflectorsOnFarmer, debug)
				estAltDur := time.Duration(estAltVal * float64(time.Hour))
				if estAltDur < estimateDurationMaxGG {
					estimateDurationMaxGG = estAltDur
					maxGGCompassSwap = true
				}
			}
			if debug {
				log.Printf("estimateDurationMaxGG: %v (compass swap: %v)\n", estimateDurationMaxGG, maxGGCompassSwap)
			}
		case "leggy_siab_gg":
			estimateDurationSIABGG = time.Duration(estimate * float64(time.Hour))
			if modELR < 1.0 {
				// Try Config B: Swap Compass for SIAB
				estB := est
				estB.compass = 1.0
				estB.gusset = 1.25
				estB.deliverySlots = 10.0
				estBVal := calculateSingleEstimate(estB, c, contractEggsTotal, numFarmers,
					contractLengthInSeconds, modShip, modELR, modHab, deflectorsOnFarmer, debug)
				estBDur := time.Duration(estBVal * float64(time.Hour))

				// Try Config C: Swap Compass for 3-slot any
				estC := est
				estC.compass = 1.0
				estC.gusset = 1.0
				estC.deliverySlots = 10.0
				estCVal := calculateSingleEstimate(estC, c, contractEggsTotal, numFarmers,
					contractLengthInSeconds, modShip, modELR, modHab, deflectorsOnFarmer, debug)
				estCDur := time.Duration(estCVal * float64(time.Hour))

				if estBDur < estimateDurationSIABGG || estCDur < estimateDurationSIABGG {
					siabGGCompassSwap = true
					if estBDur < estCDur {
						estimateDurationSIABGG = estBDur
					} else {
						estimateDurationSIABGG = estCDur
					}
				}
			}
			if debug {
				log.Printf("estimateDurationSIABGG: %v (compass swap: %v)\n", estimateDurationSIABGG, siabGGCompassSwap)
			}
		}
	}

	if estimateDurationUpper > contractDuration {
		return ContractDurationEstimate{
			Upper:         contractDuration,
			Lower:         contractDuration,
			Max:           estimateDurationMax,
			SIAB:          estimateDurationSIAB,
			MaxGG:         estimateDurationMaxGG,
			SIABGG:        estimateDurationSIABGG,
			SIABCompass:   siabCompassSwap,
			SIABGGCompass: siabGGCompassSwap,
			MaxCompass:    maxCompassSwap,
			MaxGGCompass:  maxGGCompassSwap,
			UpperCompass:  upperCompassSwap,
			LowerCompass:  lowerCompassSwap,
		}
	}

	return ContractDurationEstimate{
		Upper:         estimateDurationUpper,
		Lower:         estimateDurationLower,
		Max:           estimateDurationMax,
		SIAB:          estimateDurationSIAB,
		MaxGG:         estimateDurationMaxGG,
		SIABGG:        estimateDurationSIABGG,
		SIABCompass:   siabCompassSwap,
		SIABGGCompass: siabGGCompassSwap,
		MaxCompass:    maxCompassSwap,
		MaxGGCompass:  maxGGCompassSwap,
		UpperCompass:  upperCompassSwap,
		LowerCompass:  lowerCompassSwap,
	}
}

// calcBoostMulti converts a number of active boost tokens into an overall
// production multiplier used when estimating contract completion time.
//
// tokens represents the count of boost tokens applied to the farm. It is
// truncated to an integer and mapped to a pre-defined multiplier schedule
// that approximates the combined effect of different boost levels.
//
// The returned value is the aggregate multiplier that should be applied to
// the baseline production/earning rate when calculating how quickly a
// contract can be completed under the current boost configuration.
func calcBoostMulti(tokens float64) float64 {
	var mult float64
	tokenInt := int(tokens)

	switch tokenInt {
	case 1:
		mult = (4 * 10) * 2
	case 2:
		mult = 100 + 4*10
	case 3:
		mult = (100 + 3*10) * 2
	case 4:
		mult = 1000 + 4*10
	case 5:
		mult = (1000 + 3*10) * 2
	case 6:
		mult = (1000 + 2*10) * (2 + 2)
	case 7:
		mult = (1000 + 10) * (2 + 2 + 2)
	case 8:
		mult = (1000 + 3*10) * 10
	case 9:
		mult = (1000 + 2*10) * (10 + 2)
	case 10:
		mult = (1000 + 10) * (10 + 2 + 2)
	case 11:
		mult = 1000 * (10 + 2 + 2 + 2)
	case 12:
		mult = (1000 + 3*10) * 50
	default:
		mult = 50
	}

	return mult
}
