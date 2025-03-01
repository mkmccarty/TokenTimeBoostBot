package boost

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
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
		},
	}
}

// HandleEstimateTimeCommand will handle the estimate-contract-time command
func HandleEstimateTimeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	//var builder strings.Builder
	var contractID = ""
	var str = ""
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	showScores := false
	if opt, ok := optionMap["contract-id"]; ok {
		contractID = opt.StringValue()
	} else {
		// No contract ID in parameter, go find one
		runningContract := FindContract(i.ChannelID)
		if runningContract != nil {
			contractID = runningContract.ContractID
		}
	}
	if strings.HasPrefix(contractID, "-") {
		parts := strings.Split(contractID, "(")
		if len(parts) > 1 {
			contractID = strings.Split(parts[1], ")")[0]
		}
		showScores = true

	}
	c := ei.EggIncContractsAll[contractID]

	if c.ID == "" {
		str = "No contract found in this channel, use the command parameters to pick one."

	}
	if str == "" {
		str := getContractEstimateString(contractID, showScores)

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    str,
				Flags:      discordgo.MessageFlagsSuppressEmbeds,
				Components: []discordgo.MessageComponent{}},
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

func getContractEstimateString(contractID string, showScores bool) string {

	str := ""
	c := ei.EggIncContractsAll[contractID]

	if c.ID == "" {
		str = "No contract found in this channel, use the command parameters to pick one."
		return str
	}
	eggStr := FindEggEmoji(c.EggName)
	tokenStr, _, _ := ei.GetBotEmoji("token")
	runStr, _, _ := ei.GetBotEmoji("icon_chicken_run")
	seasonalStr := ""
	if c.SeasonID != "" {
		seasonYear := strings.Split(c.SeasonID, "_")[1]
		seasonIcon := strings.Split(c.SeasonID, "_")[0]
		seasonEmote := map[string]string{"winter": "❄️", "spring": "🌷", "summer": "🌞", "fall": "🍂"}
		seasonalStr = fmt.Sprintf("Seasonal: %s %s\n", seasonEmote[seasonIcon], seasonYear)
	}

	str = fmt.Sprintf("%s%s **%s** [%s](%s)\n%s%d%s - %s/%dm - %s%d/%dm",
		ei.GetBotEmojiMarkdown("contract_grade_aaa"),
		eggStr, c.Name, c.ID,
		fmt.Sprintf("https://eicoop-carpet.netlify.app/?q=%s", c.ID),
		seasonalStr,
		c.MaxCoopSize, ei.GetBotEmojiMarkdown("icon_coop"),
		tokenStr, c.MinutesPerToken,
		runStr, c.ChickenRuns, c.ChickenRunCooldownMinutes)
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
	/*
		BTA := c.EstimatedDuration.Minutes() / float64(c.MinutesPerToken)
		targetTval := 3.0
		if BTA > 42.0 {
			targetTval = 0.07 * BTA
		}
		BTALower := c.EstimatedDurationLower.Minutes() / float64(c.MinutesPerToken)
		targetTvalLower := 3.0
		if BTALower > 42.0 {
			targetTvalLower = 0.07 * BTALower
		}
	*/
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
	if c.TargetAmountq[len(c.TargetAmountq)-1] < 1.0 {
		str += fmt.Sprintf("**%v** - **%v** for a fastrun needing to ship **%.3fq** eggs\n", estStrLower, estStr, float64(c.TargetAmountq[len(c.TargetAmountq)-1]))
	} else {
		str += fmt.Sprintf("**%v** - **%v** for a fastrun needing to ship **%.dq** eggs\n", estStrLower, estStr, int(c.TargetAmountq[len(c.TargetAmountq)-1]))
	}
	if math.Round(c.TargetTval*100)/100 == math.Round(c.TargetTvalLower*100)/100 {
		str += fmt.Sprintf("Target TVal: **%.2f**\n", c.TargetTval)
	} else {
		str += fmt.Sprintf("Target TVal: **%.2f** for lower estimate\n", c.TargetTvalLower)
		str += fmt.Sprintf("Target TVal: **%.2f** for upper estimate\n", c.TargetTval)
	}

	// Calculate and display contract scores
	if showScores {
		scoreLower := getContractScoreEstimate(c, ei.Contract_GRADE_AAA, true, 1.0, 1.0, 100, 45, 20, 0, 100, 100, 5)
		score := getContractScoreEstimate(c, ei.Contract_GRADE_AAA, false, 1.0, 1.0, 60, 45, 15, 0, c.MaxCoopSize-1, 100, 5)
		scoreSink := getContractScoreEstimate(c, ei.Contract_GRADE_AAA, false, 1.0, 1.0, 60, 45, 15, 0, c.MaxCoopSize-1, 3, 100)

		str += fmt.Sprintf("Contract Score Top: **%d** (100%%/20%%/CR/TVAL)\n", scoreLower)
		str += fmt.Sprintf("Contract Score ACO Fastrun: **%d**(60%%/15%%/TVAL)\n", score)
		str += fmt.Sprintf("Contract Score Sink: **%d**(60%%/15%%)\n", scoreSink)
	}

	noteStr := ""
	if c.ContractVersion == 1 {
		noteStr = fmt.Sprintf("**This is a ELITE Version 1 contract last seen <t:%d:F>.**\n", c.StartTime.Unix())
	} else if c.ExpirationTime.Before(time.Now().UTC()) {
		noteStr = fmt.Sprintf("**This is an unavailable V2 contract last seen <t:%d:F>.**\n", c.StartTime.Unix())
	}

	return noteStr + str
}

// getContractDurationEstimate returns two estimated durations of a contract based for great and well equiped artifact sets
func getContractDurationEstimate(contractEggsInSmallQ float64, numFarmers float64, contractLengthInSeconds int, modifierSR float64, modifierELR float64, modifierHabCap float64) (time.Duration, time.Duration) {

	contractDuration := time.Duration(contractLengthInSeconds) * time.Second

	modELR := modifierELR * modifierHabCap
	modShip := modifierSR

	collectibleELR, colllectibleShip, colleggtibleHab := ei.GetColleggtibleValues()
	collectibleELR *= colleggtibleHab

	deflectorsOnFarmer := numFarmers - 1.0

	estimates := []struct {
		slots          float64
		deflectorBonus float64
		colELR         float64
		colShip        float64
	}{
		{
			slots:          8.0,
			deflectorBonus: 0.15,
			colELR:         1.0,
			colShip:        1.0,
		},
		{
			slots:          9.0,
			deflectorBonus: 0.17,
			colELR:         collectibleELR,
			colShip:        colllectibleShip,
		},
	}

	var estimateDurationUpper time.Duration
	var estimateDurationLower time.Duration

	for _, est := range estimates {
		slots := est.slots
		deflectorBonus := est.deflectorBonus
		colELR := est.colELR
		colShip := est.colShip

		baseELR := 3.772 * 1.35 * 1.25
		baseShipping := 7.148 * 1.5 * colShip
		maxShipping := baseShipping * math.Pow(1.05, slots)
		contractBaseELR := baseELR * modELR
		contractShipCap := maxShipping * modShip
		deflectorMultiplier := 1.0 + deflectorBonus*deflectorsOnFarmer
		tachStones := slots + ((modShip * colShip) / (modELR * colELR)) - deflectorsOnFarmer*slots/(slots+(modShip*colShip)/(modELR*colELR))
		tachBounded := max(0.0, min(slots, tachStones))
		tachMultiplier := math.Pow(1.05, tachBounded)
		contractELR := contractBaseELR * deflectorMultiplier * tachMultiplier
		boundedELR := min(contractShipCap, contractELR)
		estimate := 0.75 + contractEggsInSmallQ/(numFarmers*boundedELR)

		if est.slots == 8.0 {
			estimateDurationUpper = time.Duration(estimate * float64(time.Hour))
		} else {
			estimateDurationLower = time.Duration(estimate * float64(time.Hour))
		}

		/*
			if modShip == 1.25 {
				fmt.Printf("slots: %v\n", slots)
				fmt.Printf("modELR: %v\n", modELR)
				fmt.Printf("modShip: %v\n", modShip)
				fmt.Printf("colELR: %v\n", colELR)
				fmt.Printf("colShip: %v\n", colShip)
				fmt.Printf("baseELR: %v\n", baseELR)
				fmt.Printf("baseShipping: %v\n", baseShipping)
				fmt.Printf("maxShipping: %v\n", maxShipping)

				fmt.Printf("contractBaseELR: %v\n", contractBaseELR)
				fmt.Printf("contractShipCap: %v\n", contractShipCap)
				fmt.Printf("deflectorMultiplier: %v\n", deflectorMultiplier)
				fmt.Printf("tachStones: %v\n", tachStones)
				fmt.Printf("tachBounded: %v\n", tachBounded)
				fmt.Printf("tachMultiplier: %v\n", tachMultiplier)
				fmt.Printf("contractELR: %v\n", contractELR)
				fmt.Printf("boundedELR: %v\n", boundedELR)
				if est.slots == 8.0 {
					fmt.Printf("estimateUpper: %v\n", estimateDurationUpper)
					fmt.Print("--------------------\n")
				} else {
					fmt.Printf("estimateUpper: %v\n", estimateDurationLower)
				}
			}
		*/
	}

	if estimateDurationUpper > contractDuration {
		return contractDuration, contractDuration
	}

	return estimateDurationUpper, estimateDurationLower
}

/*

// WriteEstimatedDurationsToCSV writes the estimatedDuration values to a CSV file
func WriteEstimatedDurationsToCSV(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	err = writer.Write([]string{"Contract ID", "Eggs", "Farmers", "Estimated Duration"})
	if err != nil {
		return err
	}

	// Write data
	for _, contract := range EggIncContractsAll {
		if len(contract.qTargetAmount) > 0 {
			err = writer.Write([]string{contract.ID, fmt.Sprintf("%d", int(contract.qTargetAmount[len(contract.qTargetAmount)-1])), fmt.Sprintf("%d", contract.MaxCoopSize), contract.estimatedDuration.Round(time.Second).String()})
			if err != nil {
				return err
			}

		}
	}

	return nil
}

*/
