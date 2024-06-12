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
		Description: "Evaluate teamwork values a contract",
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
		estStr := c.EstimatedDuration.Round(time.Minute).String()
		estStr = strings.TrimRight(estStr, "0s")
		str = fmt.Sprintf("**%s** (%s) with %d %s needing to ship %dq eggs is estimated to take **about %v**", c.Name, c.ID, int(c.MaxCoopSize), fstr, int(c.TargetAmountq[len(c.TargetAmountq)-1]), estStr)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: str,
				//Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
	} else {
		// Error messages only go back to the caller
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    str,
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
	}
}

func getContractDurationEstimate(contractEggs float64, numFarmers float64) time.Duration {
	//ASSUME: average fast player has triple leg contract artis and T4C deflector and spends 10% of contract time unboosted
	//shiny deflectors pull completion time faster than estimate, epic artifacts push slower than estimate

	estimate := contractEggs / (numFarmers * 5.8 * (1.0 + 0.15*min(numFarmers-1.0, 10.0)) * math.Pow(1.05, max(0.0, 10.0-numFarmers)))
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
