package boost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// SlashAdminGetContractData is the slash to get contract JSON data
func SlashAdminGetContractData(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Retrieve contract JSON data",
		DefaultMemberPermissions: &adminPermission,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Select a contract-id",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "coop-id",
				Description:  "Your coop-id",
				Required:     true,
				Autocomplete: true,
			},
		},
	}
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

	userID := getInteractionUserID(i)

	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err != nil {
		log.Println(err)
	}
	if perms&discordgo.PermissionAdministrator == 0 {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "You are not authorized to use this command.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}

	str := "Marking contract " + contractHash + " as finished."
	err = finishContractByHash(contractHash)
	if err != nil {
		str = err.Error()
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

	ArchiveContracts(s)

	userID := getInteractionUserID(i)

	// Only allow command if users is in the admin list
	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err != nil {
		log.Println(err)
	}
	if perms&discordgo.PermissionAdministrator == 0 {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "You are not authorized to use this command.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
		str := fmt.Sprintf("> Coordinator: <@%s>  [%s](%s/%s/%s)\n", c.CreatorID[0], c.CoopID, "https://eicoop-carpet.netlify.app", c.ContractID, c.CoopID)
		for _, loc := range c.Location {
			str += fmt.Sprintf("> *%s*\t%s\n", loc.GuildName, loc.ChannelMention)
		}
		str += fmt.Sprintf("> Started: <t:%d:R>\n", c.StartTime.Unix())
		str += fmt.Sprintf("> Contract State: *%s*\n", contractStateNames[c.State])
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
	if len(contract.BoostedOrder) != len(contract.Order) {
		contract.BoostedOrder = contract.Order
	}
	farmerstate.SetOrderPercentileAll(contract.BoostedOrder, len(contract.Order))

	_ = saveEndData(contract) // Save for historical purposes
	delete(Contracts, contract.ContractHash)
	saveData(Contracts)

	return nil
}

// HandleCoopAutoComplete will handle the contract auto complete of contract-id's
func HandleCoopAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	contractID := ""
	coopID := ""
	if opt, ok := optionMap["contract-id"]; ok {
		if opt.Focused {
			HandleContractAutoComplete(s, i)
			return
		}
		contractID = opt.StringValue()
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = opt.StringValue()
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	for _, c := range Contracts {
		if c.ContractID == contractID {
			// if coopID is empty, or contains the search string
			if coopID == "" || strings.Contains(c.CoopID, coopID) {
				choice := discordgo.ApplicationCommandOptionChoice{
					Name:  c.CoopID,
					Value: c.CoopID,
				}
				choices = append(choices, &choice)
			}
		}
	}

	sort.Slice(choices, func(i, j int) bool {
		return choices[i].Name < choices[j].Name
	})

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Coop ID",
			Choices: choices,
		}})
}

// HandleAdminGetContractData get JSON data about a contract given the contract and coop id
func HandleAdminGetContractData(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	var contractID string
	var coopID string

	if opt, ok := optionMap["contract-id"]; ok {
		contractID = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = strings.TrimSpace(opt.StringValue())
	}

	// Find a contract by contract ID and coop ID
	contract := findContractByIDs(contractID, coopID)

	// Create combined contract and coopid with only alphanumberic characters
	// This is used to create a unique filename
	sanitizedID := strings.ToLower(strings.Join(strings.Fields(fmt.Sprintf("%s-%s", contractID, coopID)), "-"))
	// Remove spaces and slashes from name
	sanitizedID = strings.ReplaceAll(sanitizedID, " ", "-")
	sanitizedID = strings.ReplaceAll(sanitizedID, "/", "-")

	var reader *bytes.Reader
	var builder strings.Builder

	filename := "boostbot-data-" + sanitizedID + ".json"
	// Check to see if this is a valid filename
	buf := &bytes.Buffer{}
	jsonData, err := json.Marshal(contract)
	if err != nil {
		log.Println(err.Error())
		builder.WriteString("Error formatting JSON data. " + err.Error())
	} else {
		err = json.Indent(buf, jsonData, "", "  ")
		if err != nil {
			builder.WriteString("Error formatting JSON data. " + err.Error())
		} else {
			// Create io.Reader from JSON string
			reader = bytes.NewReader(buf.Bytes())
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Here is the JSON data for contract %s/%s", contractID, coopID),
			Flags:   discordgo.MessageFlagsEphemeral,
			Files: []*discordgo.File{
				{
					Name:        filename,
					ContentType: "application/json",
					Reader:      reader,
				},
			},
		},
	})
}
