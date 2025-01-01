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

// HandleEstimateTimeCommand will handle the /teamwork-eval command
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

	if opt, ok := optionMap["contract-id"]; ok {
		contractID = opt.StringValue()
	} else {
		// No contract ID in parameter, go find one
		runningContract := FindContract(i.ChannelID)
		if runningContract != nil {
			contractID = runningContract.ContractID
		}
	}
	c := ei.EggIncContractsAll[contractID]

	if c.ID == "" {
		str = "No contract found in this channel, use the command parameters to pick one."

	}

	if str == "" {
		eggStr := FindEggEmoji(c.EggName)
		tokenStr, _, _ := ei.GetBotEmoji("token")
		runStr, _, _ := ei.GetBotEmoji("icon_chicken_run")

		str = fmt.Sprintf("%s **%s** (%s)\n%d🧑‍🌾 - %s/%dm - %s%d/%dm",
			eggStr, c.Name, c.ID,
			c.MaxCoopSize,
			tokenStr, c.MinutesPerToken,
			runStr, c.ChickenRuns, c.ChickenRunCooldownMinutes)
		if c.ModifierSR != 1.0 && c.ModifierSR > 0.0 {
			str += fmt.Sprintf(" / 🛻 %2.1fx", c.ModifierSR)
		}
		str += "\n"

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

		estStr := c.EstimatedDuration.Round(time.Minute).String()
		estStr = strings.TrimRight(estStr, "0s")
		estStrLower := c.EstimatedDurationLower.Round(time.Minute).String()
		estStrLower = strings.TrimRight(estStrLower, "0s")

		/*
			upperEstEmotes := fmt.Sprintf("%s%s%s%s",
				ei.GetBotEmojiMarkdown("afx_tachyon_deflector_4"),
				ei.GetBotEmojiMarkdown("MT4La"),
				ei.GetBotEmojiMarkdown("CT4La"),
				ei.GetBotEmojiMarkdown("GT4La"))
			lowerEstEmotes := fmt.Sprintf("%s%s%s%s%s%s",
				ei.GetBotEmojiMarkdown("DT4La"),
				ei.GetBotEmojiMarkdown("MT4La"),
				ei.GetBotEmojiMarkdown("CT4La"),
				ei.GetBotEmojiMarkdown("GT4La"),
				ei.GetBotEmojiMarkdown("egg_carbonfiber"),
				ei.GetBotEmojiMarkdown("egg_pumpkin"))
		*/
		// A speedrun or fastrun of $CONTRACT with $NUMBER farmer(s) needing to ship $GOAL eggs is estimated to take about $TIME
		if c.TargetAmountq[len(c.TargetAmountq)-1] < 1.0 {
			str += fmt.Sprintf("**%v** - **%v** for a fastrun needing to ship **%.3fq** eggs\n", estStrLower, estStr, float64(c.TargetAmountq[len(c.TargetAmountq)-1]))
		} else {
			str += fmt.Sprintf("**%v** - **%v** for a fastrun needing to ship **%.dq** eggs\n", estStrLower, estStr, int(c.TargetAmountq[len(c.TargetAmountq)-1]))
		}
		if math.Round(targetTval*100)/100 == math.Round(targetTvalLower*100)/100 {
			str += fmt.Sprintf("Target TVal: **%.2f**\n", targetTval)
		} else {
			str += fmt.Sprintf("Target TVal: **%.2f** for lower estimate\n", targetTvalLower)
			str += fmt.Sprintf("Target TVal: **%.2f** for upper estimate\n", targetTval)
		}

		noteStr := ""
		if c.ContractVersion == 1 {
			noteStr = fmt.Sprintf("**This is a ELITE Version 1 contract last seen <t:%d:F>.**\n", c.StartTime.Unix())
		} else if c.ExpirationTime.Before(time.Now().UTC()) {
			noteStr = fmt.Sprintf("**This is an unavailable V2 contract last seen <t:%d:F>.**\n", c.StartTime.Unix())
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: noteStr + str,
				//Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
	} else {
		// Error messages only go back to the caller
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    str,
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
	}
}

func getContractDurationEstimate(contractEggsInSmallQ float64, numFarmers float64, contractLengthInSeconds int) (time.Duration, time.Duration) {
	//baseLaying := 3.772
	//baseShipping := 7.148
	// 6440.0 is the base Internal Hatchery Rate for a solo player with no boosts
	//maxIHR := 6440.0
	// 10% if the time is building the farm, so only credit for 90% of the time
	//averagingMult := 1.0
	// When the averingMult i 1.0 then we'll use the extraMinutes to add to the estimate
	// The 5.8 is Solo production without stones times 0.9
	//	soloIHRRate := maxIHR * averagingMult / 1000.0
	//if modifierSR != 1.0 && modifierSR > 0.0 {
	//soloIHRRate *= modifierSR
	//}

	contractDuration := time.Duration(contractLengthInSeconds) * time.Second

	/* Future ideas

	- Colleggtibles increasing ELR or hab will Increase 6.365 base rate,
	decrease the cap on useful deflectors in the deflector multiplier,
	and decrease the number of "free deflectors" accounted for in the tachyon stone multiplier.
	This may affect the slope of the linear approximation of stone swaps.

	- Colleggtibles increasing shipping will increase the maximum shipping cap,
	increase the cap on useful deflectors in the deflector multiplier,
	and increase the number of "free deflectors" accounted for in the tachyon stone multiplier.
	This may affect the slope of the linear approximation of stone swaps.

	- If it becomes desirable to bound the estimate using higher-grade deflectors
	this will affect the power of each deflector in the deflector multiplier and the
	number of available stone slots in the tachyon stone multiplier as well as increase
	the allowed shipping cap to account for the additional quantum stone.
	*/

	/*
		T4C and 3 T4Ls with no colleggtibles (upper estimate bound):
		   time(hours) = 0.75 + Goal / (Coop_Size * MIN(15.841, 6.365 * (1 + 0.15 * MIN(10, Coop_Size-1)) * 1.05 ^ MAX(0, MIN(8, 9-(Coop_Size-1)*8/9))))
		T4R and 3 T4Ls with all colleggtibles (lower estimate bound):
		   time(hours) = 0.75 + Goal / (Coop_Size * MIN(18.338, 6.365 * (1 + 0.17 * MIN(12, Coop_Size-1)) * 1.05 ^ MAX(0, MIN(9, 10-(Coop_Size-1)*9/10))))
	*/

	// T4C and 3 T4Ls with no colleggtibles (upper estimate bound):
	estimateUpper := 0.75 + contractEggsInSmallQ/(numFarmers*min(15.841, 6.365*(1.0+0.15*min(10, numFarmers-1.0))*math.Pow(1.05, max(0.0, min(8.0, 9.0-(numFarmers-1.0)*8.0/9.0)))))
	estimateDurationUpper := time.Duration(estimateUpper*float64(time.Hour)) + time.Duration(45*float64(time.Minute))

	// T4R and 3 T4Ls with all colleggtibles (lower estimate bound):
	estimateLower := 0.75 + contractEggsInSmallQ/(numFarmers*min(18.338, 6.365*(1.0+0.17*min(12, numFarmers-1.0))*math.Pow(1.05, max(0.0, min(9.0, 10.0-(numFarmers-1.0)*9.0/10.0)))))
	estimateDurationLower := time.Duration(estimateLower*float64(time.Hour)) + time.Duration(45*float64(time.Minute))

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
