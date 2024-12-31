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
		BTA2 := c.EstimatedDurationShip.Minutes() / float64(c.MinutesPerToken)
		targetTval := 3.0
		if BTA > 42.0 {
			targetTval = 0.07 * BTA
		}

		// A speedrun or fastrun of $CONTRACT with $NUMBER farmer(s) needing to ship $GOAL eggs is estimated to take about $TIME
		if c.TargetAmountq[len(c.TargetAmountq)-1] < 1.0 {
			estStr := c.EstimatedDuration.Round(time.Second).String()
			str += fmt.Sprintf("**%v** for a speedrun or fastrun needing to ship %.3fq eggs\n", estStr, c.TargetAmountq[len(c.TargetAmountq)-1])
			//			str += fmt.Sprintf("A speedrun or fastrun of **%s** (%s) needing to ship %.3fq eggs is estimated to take **about %v**\n", c.Name, c.ID, c.TargetAmountq[len(c.TargetAmountq)-1], estStr)
			if c.EstimatedDuration != c.EstimatedDurationShip {
				str += fmt.Sprintf("> w/Carbon Fiber: **about %v**\n", c.EstimatedDurationShip.Round(time.Second).String())
				if BTA2 > 42.0 {
					targetTval = 0.07 * BTA2
				}
			}

			str += fmt.Sprintf("\nTarget TVal: **%.2f**", targetTval)

		} else {
			estStr := c.EstimatedDuration.Round(time.Minute).String()
			estStr = strings.TrimRight(estStr, "0s")
			estStrShip := c.EstimatedDurationShip.Round(time.Minute).String()
			estStrShip = strings.TrimRight(estStrShip, "0s")
			//str += fmt.Sprintf("A speedrun or fastrun of **%s** (%s) needing to ship %dq eggs is estimated to take **about %v**\n", c.Name, c.ID, int(c.TargetAmountq[len(c.TargetAmountq)-1]), estStr)
			str += fmt.Sprintf("**%v** for a speedrun or fastrun needing to ship %.dq eggs\n", estStr, int(c.TargetAmountq[len(c.TargetAmountq)-1]))
			if c.EstimatedDuration != c.EstimatedDurationShip {
				str += fmt.Sprintf("> w/Carbon Fiber: **about %v**\n", estStrShip)
			}

			str += fmt.Sprintf("Target TVal: **%.2f**\n", targetTval)
		}

		/*
			Goal: 950q
			:clock: Duration: 3d
			:icon_coop: Coop size: 15
			:b_icon_token: Token timer: 15m
			:r_icon_relativity_optimization: Laying rate per player: 879T/hr
			:chickenrun: Chicken Runs target: 20
		*/

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

func getContractDurationEstimate(contractEggs float64, numFarmers float64, collegtible bool, modifierSR float64) time.Duration {
	//ASSUME: average fast player has triple leg contract artis and T4C deflector and spends 10% of contract time unboosted
	//shiny deflectors pull completion time faster than estimate, epic artifacts push slower than estimate

	// With the colleggitbles
	// Once we have access to the data mined pumpkin colleggtible this changes slightly to
	// Time = Goal / (Coop_Size * 5.8 * (1 + 0.15 * MIN(11, Coop_Size - 1)) * 1.05 ^ MAX(0, 10 - Coop_Size)) to account for the 5% shipping bonus.

	shippingMod := 0.0
	if collegtible {
		shippingMod = 1.0
	}

	//baseLaying := 3.772
	//baseShipping := 7.148

	// The 5.8 is Solo production without stones times 0.9
	soloRate := 5.8
	if modifierSR != 1.0 && modifierSR > 0.0 {
		soloRate *= modifierSR
	}

	estimate := contractEggs / (numFarmers * soloRate * (1.0 + 0.15*min(numFarmers-1.0, 10.0+shippingMod)) * math.Pow(1.05, max(0.0, 10.0-numFarmers)))
	estimateDuration := time.Duration(estimate * float64(time.Hour))

	//log.Printf("%s: %dq for %d farmers = %v", ID, int(contractEggs), int(numFarmers), estimateDuration.Round(time.Second))
	return estimateDuration
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
