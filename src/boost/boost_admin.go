package boost

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// AdminUsers is a list of users who are allowed to use admin commands
var AdminUsers = []string{
	"238786501700222986", // RAIYC
	"393477262412087319", // Tbone
	"430186990260977665", // aggie
	"184063956539670528", // Halcyon
	"662685289885466672", // DipDip
	"429540887383506954", // Hypnojustice
	"899945319582822400", // LousyCurve
}

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

	userID := ""
	if i.GuildID == "" {
		userID = i.User.ID
	} else {
		userID = i.Member.User.ID
	}

	// Only allow command if users is in the admin list
	if slices.Index(AdminUsers, userID) == -1 {
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
	str, embed, err := getContractList()
	if err != nil {
		str = err.Error()
	}

	userID := ""
	if i.GuildID == "" {
		userID = i.User.ID
	} else {
		userID = i.Member.User.ID
	}

	// Only allow command if users is in the admin list
	if slices.Index(AdminUsers, userID) == -1 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "You are not authorized to use this command.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Embeds:     embed.Embeds,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

func getRandomColor() int {
	return rand.IntN(16777216) // 16777216 is the maximum value for a 24-bit color
}

// getContractList returns a list of all contracts
func getContractList() (string, *discordgo.MessageSend, error) {
	var field []*discordgo.MessageEmbedField

	str := ""
	if len(Contracts) == 0 {
		embed := &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{{
				Type:        discordgo.EmbedTypeRich,
				Title:       "Contract List",
				Description: "No contracts available",
				Color:       getRandomColor(),
				Fields:      field,
			}},
		}

		return "", embed, nil
	}

	i := 1
	for _, c := range Contracts {
		str := fmt.Sprintf("> Coordinator: <@%s>  link:[%s](%s/%s/%s)\n", c.CreatorID[0], c.CoopID, "https://eicoop-carpet.netlify.app", c.ContractID, c.CoopID)
		for _, loc := range c.Location {
			str += fmt.Sprintf("> *%s*\t%s\t%s\n", loc.GuildName, loc.ChannelName, loc.ChannelMention)
		}
		str += fmt.Sprintf("> Started: <t:%d:R>\n", c.StartTime.Unix())
		str += fmt.Sprintf("> Contract State: *%s*\n", contractStateNames[c.State])
		if c.Speedrun {
			str += fmt.Sprintf("> Speedrun State: *%s*\n", speedrunStateNames[c.SRData.SpeedrunState])
		}
		str += fmt.Sprintf("> Hash: *%s*\n", c.ContractHash)
		field = append(field, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%d - **%s/%s**\n", i, c.ContractID, c.CoopID),
			Value:  str,
			Inline: false,
		})
		i++
	}

	embed := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Type:        discordgo.EmbedTypeRich,
			Title:       "Contract List",
			Description: fmt.Sprintf("%d contracts running", len(Contracts)),
			Color:       getRandomColor(),
			Fields:      field,
		}},
	}

	return str, embed, nil
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
