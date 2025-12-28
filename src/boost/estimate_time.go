package boost

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

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
		},
	}
}

// HandleEstimateTimeCommand will handle the estimate-contract-time command
func HandleEstimateTimeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	//var builder strings.Builder
	var contractID = ""
	var str = ""
	includeLeggySet := false
	optionMap := bottools.GetCommandOptionsMap(i)

	if opt, ok := optionMap["contract-id"]; ok {
		contractID = opt.StringValue()
	} else {
		// No contract ID in parameter, go find one
		runningContract := FindContract(i.ChannelID)
		if runningContract != nil {
			contractID = runningContract.ContractID
		}
	}
	if opt, ok := optionMap["include-leggy"]; ok {
		includeLeggySet = opt.BoolValue()
	}
	c := ei.EggIncContractsAll[contractID]

	if c.ID == "" {
		str = "No contract found in this channel, use the command parameters to pick one."

	}
	if str == "" {
		str := getContractEstimateString(contractID, includeLeggySet)

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsSuppressEmbeds | discordgo.MessageFlagsIsComponentsV2,
				Components: []discordgo.MessageComponent{
					discordgo.TextDisplay{
						Content: str,
					},
				}},
		})
	} else {
		// Error messages only go back to the caller
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    str,
				Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsSuppressEmbeds,
				Components: []discordgo.MessageComponent{}},
		})
	}
}

func getContractEstimateString(contractID string, includeLeggySet bool) string {

	str := ""
	c := ei.EggIncContractsAll[contractID]

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
	staabData, staabError := EncodeData(cxpToggle, c.TargetAmount[len(c.TargetAmount)-1],
		strconv.Itoa(c.MinutesPerToken), c.LengthInSeconds, c.MaxCoopSize, &c)
	if staabError != nil {
		// Fallback to the default SR Sandbox configuration if encoding fails
		staabData = "v-5MTEwMDAwMC0wLTEzLTctNTAwLTYwLTEtMS0yLVBsYXllciUyMDAtNi0xMA=B6mEavjeExzag"
	}

	str = fmt.Sprintf("%s%s **%s** [%s](%s), [SR Sandbox](%s)\n%s%d%s - %s/%dm - %s%d/%dm - 📏%s",
		ei.GetBotEmojiMarkdown("contract_grade_aaa"),
		eggStr, c.Name, c.ID,
		fmt.Sprintf("https://eicoop-carpet.netlify.app/?q=%s", c.ID),
		fmt.Sprintf("https://srsandbox-staabmia.netlify.app/?data=%s", staabData),
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
	estStrLower := c.EstimatedDurationLower.Round(time.Minute).String()
	estStrLower = strings.TrimRight(estStrLower, "0s")

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
		scoreSink := getContractScoreEstimate(c, ei.Contract_GRADE_AAA,
			true, 1.0, // Use faster duration at a 1.0 modifier
			0.92,   // 0.92 Fair Share (Last booster sink)
			60, 45, // T4C SIAB for 45m
			15, 0, // T4C Deflector for full duration
			c.MaxCoopSize-1, // All Chicken Runs
			3, 100)          // Sink token use, sent at least 3 (max) and received a lot

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
			fmt.Fprintf(&cs, " - **%.0f** ", c.Cxp*0.70)
			if c.SeasonID == "" {
				cs.WriteString(("(Low Target)\n"))
			} else {
				cs.WriteString(("(Seasonal Target)\n"))
			}
			str += cs.String()
		} else { // Seasonal contracts released starting Sept 22, 2025
			str += fmt.Sprintf("CS Est: **%d** (SR) - **%.0f** (Seasonal Target)\n",
				int64(c.Cxp),
				c.Cxp*0.70)
		}
		if includeLeggySet {
			estStrMax := c.EstimatedDurationMax.Round(time.Minute).String()
			estStrMax = strings.TrimRight(estStrMax, "0s")
			str += fmt.Sprintf("Leggy Set: **%s** CS:**%d**\n", estStrMax, int64(c.CxpMax))
			str += fmt.Sprintf("-# Leggy set 50 TE, 8 IHR & 10 delivery stones sets, 5%s boost.\n", ei.GetBotEmojiMarkdown("token"))
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
		noteStr = fmt.Sprintf("**This is a ELITE Version 1 contract last seen <t:%d:F>.**\n", c.ValidFrom.Unix())
	} else if c.ValidUntil.Before(time.Now().UTC()) {
		noteStr = fmt.Sprintf("**This is an unavailable V2 contract last seen <t:%d:F>.**\n", c.ValidFrom.Unix())
	}

	return noteStr + str
}

type estimatePlayer struct {
	slots           float64
	deflectorBonus  float64
	boostTokens     float64
	boostMultiplier float64
	colELR          float64
	colShip         float64
	colHab          float64
	colIHR          float64
	calcMode        int
	ihr             float64
	te              float64
	boundedELR      float64
	contractELR     float64
}

// getContractDurationEstimate returns three estimated durations (upper, lower, and max) of a contract based on great and well equipped artifact sets
func getContractDurationEstimate(contractEggsTotal float64, numFarmers float64, contractLengthInSeconds int, modifierSR float64, modifierELR float64, modifierHabCap float64, debug bool) (time.Duration, time.Duration, time.Duration) {

	contractDuration := time.Duration(contractLengthInSeconds) * time.Second

	modHab := modifierHabCap
	modELR := modifierELR
	modShip := modifierSR

	collectibleELR, colllectibleShip, colleggtibleHab, colleggtiblesIHR := ei.GetColleggtibleValues()

	deflectorsOnFarmer := numFarmers - 1.0

	const modeOriginalFormula = 1
	const modeStoneHuntMethod = 2

	estimates := []estimatePlayer{
		{
			slots:           8.0,
			deflectorBonus:  0.15,
			boostTokens:     8.0,
			boostMultiplier: calcBoostMulti(8.0),
			colELR:          1.0,
			colShip:         1.0,
			colHab:          1.0,
			colIHR:          1.0,
			calcMode:        modeStoneHuntMethod,
			ihr:             7440.0 * 1.3 * 1.2 * math.Pow(1.05, 6), // improved set
			te:              0,
		},
		{
			slots:           9.0,
			deflectorBonus:  0.17,
			boostTokens:     6.0,
			boostMultiplier: calcBoostMulti(6.0),
			colELR:          collectibleELR,
			colShip:         colllectibleShip,
			colHab:          colleggtibleHab,
			colIHR:          1.0,
			calcMode:        modeStoneHuntMethod,
			ihr:             7440.0 * 1.4 * 1.3 * math.Pow(1.05, 8), // leggacy set, Deflector w/o IHR stones
			te:              0,
		},
		{
			// This is for a full leggacy set with TE boosts of 5 tokens
			slots:           10.0,
			deflectorBonus:  0.20,
			boostTokens:     5.0,
			boostMultiplier: calcBoostMulti(5.0),
			colELR:          collectibleELR,
			colShip:         colllectibleShip,
			colHab:          colleggtibleHab,
			colIHR:          colleggtiblesIHR,
			calcMode:        modeStoneHuntMethod,
			ihr:             7440.0 * 1.4 * 1.3 * math.Pow(1.05, 8), // leggacy set, Deflector w/o IHR stones
			te:              50,
		},
	}

	var estimateDurationUpper time.Duration
	var estimateDurationLower time.Duration
	var estimateDurationMax time.Duration

	for _, est := range estimates {
		slots := est.slots
		deflectorBonus := est.deflectorBonus
		colELR := est.colELR
		colShip := est.colShip
		colHab := est.colHab

		// Base rate with T4L Metronome +35% and T4L Gusset +25%
		baseELR := 3.772 * 1.35 * 1.25
		// Base rate with T4L Compass +50%
		baseShipping := 7.148 * 1.5
		maxShipping := baseShipping * math.Pow(1.05, slots) * colShip
		contractBaseELR := baseELR * modELR * modHab
		contractShipCap := maxShipping * modShip
		deflectorMultiplier := 1.0 + deflectorBonus*deflectorsOnFarmer
		bestTotal := 0.0
		intSlots := int(slots)
		bestELR := 0.0
		bestSR := 0.0

		if debug {
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
			for i := 0; i <= intSlots; i++ {
				stoneLayRate := contractBaseELR
				stoneLayRate *= deflectorMultiplier
				stoneLayRate *= math.Pow(1.05, float64(i)) * colELR * colHab

				stoneShipRate := baseShipping * math.Pow(1.05, float64((intSlots-i))) * colShip

				bestMin := min(stoneLayRate, stoneShipRate)
				if bestMin > bestTotal {
					bestTotal = bestMin
					tachStones = i
					quantStones = intSlots - i
					bestELR = stoneLayRate
					bestSR = stoneShipRate
					est.contractELR = stoneLayRate

				} else if bestTotal > 0 {
					// We've passed the peak, no point continuing
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

		// 1. Define the ramp-up time before we have any boosting started
		tokensInHourAllPlayers := 5.0 / 6.0 * numFarmers * 60.0 // tokens per hour all players
		hoursPerTokenAllPlayers := 60 / tokensInHourAllPlayers

		// Need to seed this with initial wait time for first boost tokens
		rampUpHours := est.boostTokens * hoursPerTokenAllPlayers

		// In this bit of code I want to consider only the amount of population we
		// really need to hit the boundedELR. We'll consider CR and excess ELR
		unusedRatioELR := max(1.0, est.contractELR/bestTotal)
		population := (14_175_000_000 * est.colHab) / unusedRatioELR
		populationForCR := population * 0.70
		// At 70% of used population, with chicken giving 5% of population
		crPopulation := populationForCR * 0.05 * (numFarmers - 1.0)
		adjustedPop := max(populationForCR, population-crPopulation)

		ihr := est.ihr * est.colIHR * math.Pow(1.01, est.te)
		boostTime := adjustedPop / (ihr * 12 * est.boostMultiplier) / 60

		if debug {
			log.Printf("ihr: %v\n", ihr)
			log.Printf("unusedRatioELR: %v\n", unusedRatioELR)
			log.Printf("Useful population: %v\n", population)
			log.Printf("populationForCR: %v\n", populationForCR)
			log.Printf("crPopulation: %v\n", crPopulation)
			log.Printf("adjustedPop: %v\n", adjustedPop)
			log.Print("boostTime (h): ", boostTime)
		}
		if float64(contractLengthInSeconds) < 45*60 {
			// For small contracts, add less time padding for boosts
			// as possibly only 1 boost will be needed
			rampUpHours += 0.30
			// 4 tokens to boost at a rate of 6 tokens per hour, 10 minutes to boost
			//estimate += (((numFarmers * 4) / (numFarmers * 6) * 60) + 10) / 60
		} else if est.calcMode == modeOriginalFormula {
			//rampUpHours += 0.50
			rampUpHours += (est.boostTokens / 6.0) + (10.0 / 60.0)
		} else {
			// 5 tokens to boost at a rate of 6 tokens per hour
			// Boost time is 13.5 minutes to boost
			rampUpHours += (est.boostTokens / 6.0) + boostTime
		}

		// 2. Calculate deliveries made DURING the ramp-up period
		// Formula: Area of a triangle (1/2 * base * height)
		rampUpDeliveries := 0.5 * (numFarmers * est.boundedELR) * rampUpHours

		// 3. Subtract ramp-up deliveries from the total eggs to find what's left
		// for the "steady state" period
		remainingEggs := eggsTotal - rampUpDeliveries

		// 4. Calculate time spent at full speed
		steadyStateTime := remainingEggs / (numFarmers * est.boundedELR)

		// 5. Total Estimate = Ramp-up time + Steady-state time
		// Note: Boost overhead is already included in rampUpHours for modeStoneHuntMethod
		estimate := rampUpHours + steadyStateTime

		//estimate *= 0.90

		if debug {
			log.Printf("rampUpHours: %v\n", rampUpHours)
			log.Printf("rampUpDeliveries: %v\n", rampUpDeliveries)
			log.Printf("remainingEggs: %v\n", remainingEggs)
			log.Printf("steadyStateTime: %v\n", steadyStateTime)
			log.Printf("estimate (hours): %v\n", estimate)
		}
		//estimate := eggsTotal / (numFarmers * boundedELR)

		switch est.slots {
		case 8.0:
			estimateDurationUpper = time.Duration(estimate * float64(time.Hour))
		case 9.0:
			estimateDurationLower = time.Duration(estimate * float64(time.Hour))
		default:
			estimateDurationMax = time.Duration(estimate * float64(time.Hour))
		}

		if debug {
			switch est.slots {
			case 8.0:
				log.Printf("estimateDurationUpper: %v\n", estimateDurationUpper)
				log.Print("--------------------\n")
			case 9.0:
				log.Printf("estimateDurationLower: %v\n", estimateDurationLower)
			default:
				log.Printf("estimateDurationMax: %v\n", estimateDurationMax)
			}
		}
	}

	if estimateDurationUpper > contractDuration {
		return contractDuration, contractDuration, estimateDurationMax
	}

	return estimateDurationUpper, estimateDurationLower, estimateDurationMax
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
