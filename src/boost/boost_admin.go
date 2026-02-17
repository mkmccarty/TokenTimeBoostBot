package boost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
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

// SlashAdminListRoles is the slash to info about bot roles
func SlashAdminListRoles(cmd string) *discordgo.ApplicationCommand {
	//var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Display contract role usage",
		//DefaultMemberPermissions: &adminPermission,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Select a contract-id",
				Required:     true,
				Autocomplete: true,
			},
		},
	}
}

// HandleAdminListRoles is the handler for the list roles command
func HandleAdminListRoles(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var contractID string
	var builder strings.Builder
	optionMap := bottools.GetCommandOptionsMap(i)
	if opt, ok := optionMap["contract-id"]; ok {
		contractID = strings.TrimSpace(opt.StringValue())
	}

	var components []discordgo.MessageComponent
	//

	guildRoles, err := s.GuildRoles(i.GuildID)
	if err != nil {
		builder.WriteString("Error retrieving guild roles: " + err.Error())
	} else {

		for _, c := range ei.EggIncContracts {
			if c.ID == contractID {
				sortedContractRoles := make([]string, 0)
				usingFallbackRoles := false
				if len(c.TeamNames) == 0 {
					if names := fetchContractTeamNames(c.Description, 30); len(names) > 0 {
						c.TeamNames = names
						ei.EggIncContractsAll[c.ID] = c
					} else {
						// Use fallback roles when API key is undefined
						c.TeamNames = randomThingNames
						ei.EggIncContractsAll[c.ID] = c
						usingFallbackRoles = true
					}
				}

				// If using fallback roles, only include roles that are actually in use
				if usingFallbackRoles {
					// Collect roles that are actually in the guild
					for _, role := range c.TeamNames {
						for _, guildRole := range guildRoles {
							if guildRole.Name == role {
								sortedContractRoles = append(sortedContractRoles, role)
								break
							}
						}
					}
				} else {
					sortedContractRoles = append(sortedContractRoles, c.TeamNames...)
				}
				slices.Sort(sortedContractRoles)
				for _, role := range sortedContractRoles {
					// if this role is in the guild roles, display it
					name := role
					for _, guildRole := range guildRoles {
						if guildRole.Name == role {
							name = guildRole.Mention()

							// Lets find the running contract with this role
							for _, c := range Contracts {
								for _, loc := range c.Location {
									if loc.RoleMention == guildRole.Mention() {
										name += fmt.Sprintf(" in %s", loc.ChannelMention)
									}
								}
							}
							break
						}
					}
					fmt.Fprintf(&builder, "%s\n", name)
				}
			}
		}
	}

	components = append(components, &discordgo.TextDisplay{
		Content: builder.String(),
	})

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		},
	})
}

// HandleAdminContractFinish is called when the contract is complete
func HandleAdminContractFinish(s *discordgo.Session, i *discordgo.InteractionCreate) {
	contractHash := ""
	optionMap := bottools.GetCommandOptionsMap(i)

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
	err = finishContractByHash(s, contractHash)
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
	str, embed, err := getContractList(i.GuildID)
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

var lastContractListTime time.Time
var lastContractListIndex int

// getContractList returns a list of all contracts within the specified guild
func getContractList(guildID string) (string, *discordgo.MessageSend, error) {
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

	if time.Since(lastContractListTime) > 1*time.Minute {
		lastContractListIndex = 0
		lastContractListTime = time.Now()
	}

	// Calculate window start and end
	windowSize := 10
	startIdx := lastContractListIndex
	endIdx := startIdx + windowSize

	// Reset if we've reached the end
	if startIdx >= len(Contracts) {
		lastContractListIndex = 0
		startIdx = 0
		endIdx = windowSize
	}

	if endIdx > len(Contracts) {
		endIdx = len(Contracts)
	}

	contractList := make([]*Contract, 0)
	idx := 0
	for _, c := range Contracts {
		if idx >= startIdx && idx < endIdx {
			contractList = append(contractList, c)
		}
		idx++
	}

	// Update index for next call
	lastContractListIndex = endIdx

	i := 1 + startIdx
	for _, c := range contractList {
		if guildID != "766330702689992720" {
			if guildID != "" && c.Location[0].GuildID != guildID {
				continue
			}
		}
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

	if len(field) == 0 {
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
func finishContractByHash(s *discordgo.Session, contractHash string) error {
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

	// Get rid of any roles
	for _, loc := range contract.Location {
		err := s.GuildRoleDelete(loc.GuildID, loc.GuildContractRole.ID)
		if err != nil {
			log.Println(err)
		}
	}

	// Don't delete the final boost message
	if len(contract.BoostedOrder) != len(contract.Order) {
		contract.BoostedOrder = contract.Order
	}

	contract.State = ContractStateArchive
	//_ = saveEndData(contract) // Save for historical purposes
	saveData(contract.ContractHash)
	delete(Contracts, contract.ContractHash)

	return nil
}

// HandleCoopAutoComplete will handle the contract auto complete of contract-id's
func HandleCoopAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)

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
	optionMap := bottools.GetCommandOptionsMap(i)
	var contractID string
	var coopID string

	if opt, ok := optionMap["contract-id"]; ok {
		contractID = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = strings.TrimSpace(opt.StringValue())
	}

	// Find a contract by contract ID and coop ID
	contract := FindContractByIDs(contractID, coopID)

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
