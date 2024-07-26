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
		fstr := "farmers"
		if int(c.MaxCoopSize) == 1 {
			fstr = "farmer"
		}
		// A speedrun or fastrun of $CONTRACT with $NUMBER farmer(s) needing to ship $GOAL eggs is estimated to take about $TIME
		if c.TargetAmountq[len(c.TargetAmountq)-1] < 1.0 {
			estStr := c.EstimatedDuration.Round(time.Second).String()
			str = fmt.Sprintf("A speedrun or fastrun of **%s** (%s) with %d %s needing to ship %.3fq eggs is estimated to take **about %v**\n", c.Name, c.ID, int(c.MaxCoopSize), fstr, c.TargetAmountq[len(c.TargetAmountq)-1], estStr)
			if c.EstimatedDuration != c.EstimatedDurationShip {
				str += fmt.Sprintf("> w/Carbon Fiber: **about %v**", c.EstimatedDurationShip.Round(time.Second).String())
			}
		} else {
			estStr := c.EstimatedDuration.Round(time.Minute).String()
			estStr = strings.TrimRight(estStr, "0s")
			estStrShip := c.EstimatedDurationShip.Round(time.Minute).String()
			estStrShip = strings.TrimRight(estStrShip, "0s")
			str = fmt.Sprintf("A speedrun or fastrun of **%s** (%s) with %d %s needing to ship %dq eggs is estimated to take **about %v**\n", c.Name, c.ID, int(c.MaxCoopSize), fstr, int(c.TargetAmountq[len(c.TargetAmountq)-1]), estStr)
			if c.EstimatedDuration != c.EstimatedDurationShip {
				str += fmt.Sprintf("> w/Carbon Fiber: **about %v**", estStrShip)
			}
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: str,
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

func getContractDurationEstimate(contractEggs float64, numFarmers float64, collegtible bool) time.Duration {
	//ASSUME: average fast player has triple leg contract artis and T4C deflector and spends 10% of contract time unboosted
	//shiny deflectors pull completion time faster than estimate, epic artifacts push slower than estimate

	// With the colleggitbles
	// Once we have access to the data mined pumpkin colleggtible this changes slightly to
	// Time = Goal / (Coop_Size * 5.8 * (1 + 0.15 * MIN(11, Coop_Size - 1)) * 1.05 ^ MAX(0, 10 - Coop_Size)) to account for the 5% shipping bonus.

	shippingMod := 0.0
	if collegtible {
		shippingMod = 1.0
	}

	estimate := contractEggs / (numFarmers * 5.8 * (1.0 + 0.15*min(numFarmers-1.0, 10.0+shippingMod)) * math.Pow(1.05, max(0.0, 10.0-numFarmers)))
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
