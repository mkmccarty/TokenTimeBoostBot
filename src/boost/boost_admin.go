package boost

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// HandleAdminContractFinish is called when the contract is complete
func HandleAdminContractFinish(s *discordgo.Session, i *discordgo.InteractionCreate) {
	contractHash := ""
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["contract-hash"]; ok {
		contractHash = strings.TrimSpace(opt.StringValue())
	}

	str := "Marking contract " + contractHash + " as finished."
	err := finishContractByHash(contractHash)
	if err != nil {
		str = err.Error()
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

// HandleAdminContractList will list all contracts
func HandleAdminContractList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str, err := getContractList()
	if err != nil {
		str = err.Error()
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

// getContractList returns a list of all contracts
func getContractList() (string, error) {
	str := ""
	if len(Contracts) == 0 {
		return "", errors.New("no contracts found")
	}

	i := 1
	for _, c := range Contracts {
		str += fmt.Sprintf("%d - **%s/%s**\n", i, c.ContractID, c.CoopID)
		for _, loc := range c.Location {
			str += fmt.Sprintf("> *%s*\t%s\t%s\n", loc.GuildName, loc.ChannelName, loc.ChannelMention)
		}
		str += fmt.Sprintf("> Hash: *%s*\n", c.ContractHash)
		str += fmt.Sprintf("> Started: <t:%d:R>\n", c.StartTime.Unix())
		i++
	}
	return str, nil
}

// finishContractByHash is called only when the contract is complete
func finishContractByHash(contractHash string) error {
	var contract *Contract
	for _, c := range Contracts {
		if c.ContractHash == contractHash {
			contract = c
			break
		}
	}
	if contract == nil {
		return errors.New(errorNoContract)
	}

	// Don't delete the final boost message
	farmerstate.SetOrderPercentileAll(contract.Order, len(contract.Order))

	saveEndData(contract) // Save for historical purposes
	delete(Contracts, contract.ContractHash)
	saveData(Contracts)

	return nil
}
