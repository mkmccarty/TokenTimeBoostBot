package boost

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

var adminUsers = []string{config.AdminUserID, "238786501700222986", "393477262412087319", "430186990260977665", "184063956539670528"}

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

	// Only allow command if users is in the admin list
	if slices.Index(adminUsers, i.Member.User.ID) == -1 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "You are not authorized to use this command.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
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
		str += fmt.Sprintf("> Coordinator: <@%s>  <%s/%s/%s>\n", c.CreatorID[0], "https://eicoop-carpet.netlify.app", c.ContractID, c.CoopID)
		for _, loc := range c.Location {
			str += fmt.Sprintf("> *%s*\t%s\t%s\n", loc.GuildName, loc.ChannelName, loc.ChannelMention)
		}
		str += fmt.Sprintf("> Started: <t:%d:R>\n", c.StartTime.Unix())
		str += fmt.Sprintf("> Contract State: *%s*\n", contractStateNames[c.State])
		if c.Speedrun {
			str += fmt.Sprintf("> Speedrun State: *%s*\n", speedrunStateNames[c.SRData.SpeedrunState])
		}
		str += fmt.Sprintf("> Hash: *%s*\n", c.ContractHash)
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
