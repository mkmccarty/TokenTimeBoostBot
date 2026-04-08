package boost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

const (
	adminGuildStateActionSet = "set-guild-setting"
	adminGuildStateActionGet = "get-guild-settings"
)

// SlashAdminGetContractData is the slash to get contract JSON data
func SlashAdminGetContractData(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Retrieve contract JSON data",
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},

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
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Display contract role usage",
		DefaultMemberPermissions: &adminPermission,
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
				Description:  "Select a contract-id",
				Required:     true,
				Autocomplete: true,
			},
		},
	}
}

// SlashAdminGuildStateCommand provides a generic entrypoint for guildstate admin actions.
func SlashAdminGuildStateCommand(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)

	guildID := guildstate.GetGuildSettingString("DEFAULT", "home_guild")
	if guildID == "" {
		guildID = "DISABLED"
	}

	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Run guildstate admin command with guild override",
		GuildID:                  guildID,
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "action",
				Description: "Guildstate command to run",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "set-guild-setting", Value: adminGuildStateActionSet},
					{Name: "get-guild-settings", Value: adminGuildStateActionGet},
				},
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "guild-id",
				Description:  "Guild ID override (from persisted guildstate)",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "setting",
				Description: "Setting key (used by set-guild-setting)",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "value",
				Description: "Optional value (used by set-guild-setting; blank clears)",
				Required:    false,
			},
		},
	}
}

// SlashAdminStatusMessageCommand sets the next bot status message.
func SlashAdminStatusMessageCommand(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Set the next bot status message",
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "message",
				Description:  "Status message to use on the next update",
				Required:     true,
				Autocomplete: true,
			},
		},
	}
}

func isAdminCommandCaller(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	userID := getInteractionUserID(i)
	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err != nil {
		log.Println(err)
	}
	return perms&discordgo.PermissionAdministrator != 0 || userID == config.AdminUserID
}

// HandleAdminGuildStateAutoComplete serves guild-id suggestions from persisted guildstate keys.
func HandleAdminGuildStateAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	search := ""
	if opt, ok := optionMap["guild-id"]; ok {
		search = strings.TrimSpace(opt.StringValue())
	}

	ids, err := guildstate.GetAllGuildIDs()
	if err != nil {
		log.Println(err)
		ids = []string{}
	}

	searchLower := strings.ToLower(search)
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, 25)
	for _, id := range ids {
		choiceName := id
		if guild, guildErr := s.Guild(id); guildErr == nil && guild != nil {
			guildName := strings.TrimSpace(guild.Name)
			if guildName != "" {
				choiceName = fmt.Sprintf("%s (%s)", guildName, id)
			}
		}

		if searchLower != "" && !strings.Contains(strings.ToLower(choiceName), searchLower) && !strings.Contains(strings.ToLower(id), searchLower) {
			continue
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: choiceName, Value: id})
		if len(choices) >= 25 {
			break
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Guild IDs",
			Choices: choices,
		},
	})
}

// HandleAdminGuildStateCommand routes to guildstate handlers with explicit guild override.
func HandleAdminGuildStateCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !isAdminCommandCaller(s, i) {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "You are not authorized to use this command.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	optionMap := bottools.GetCommandOptionsMap(i)
	action := ""
	guildID := ""
	setting := ""
	value := ""

	if opt, ok := optionMap["action"]; ok {
		action = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["guild-id"]; ok {
		guildID = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["setting"]; ok {
		setting = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["value"]; ok {
		value = strings.TrimSpace(opt.StringValue())
	}

	if guildID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "guild-id is required.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	switch action {
	case adminGuildStateActionSet:
		if setting == "" {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    "setting is required when action is set-guild-setting.",
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{},
				},
			})
			return
		}
		guildstate.SetGuildSettingForGuild(s, i, guildID, setting, value)
	case adminGuildStateActionGet:
		guildstate.GetGuildSettingsForGuild(s, i, guildID)
	default:
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "action must be one of: set-guild-setting, get-guild-settings",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{},
			},
		})
	}
}

// HandleAdminStatusMessageAutoComplete provides status message suggestions.
func HandleAdminStatusMessageAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	search := ""
	if opt, ok := optionMap["message"]; ok {
		search = strings.TrimSpace(opt.StringValue())
	}

	messages := ei.GetStatusMessageChoices(search, 25)
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(messages))
	for _, message := range messages {
		if len(message) > 100 {
			continue
		}
		choiceName := message
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  choiceName,
			Value: message,
		})
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Content: "Status message suggestions",
			Choices: choices,
		},
	})
}

// HandleAdminStatusMessageCommand sets a one-time status message override.
func HandleAdminStatusMessageCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !isAdminCommandCaller(s, i) {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "You are not authorized to use this command.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	optionMap := bottools.GetCommandOptionsMap(i)
	message := ""
	discordID := getInteractionUserID(i)
	if opt, ok := optionMap["message"]; ok {
		message = strings.TrimSpace(opt.StringValue())
	}

	if err := ei.SetNextStatusMessageOverride(discordID, message); err != nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    err.Error(),
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	responseMessage := fmt.Sprintf("Next status message set to: %q", message)
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    responseMessage,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{},
		},
	})
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
		var str strings.Builder
		fmt.Fprintf(&str, "> Coordinator: <@%s>  [%s](%s/%s/%s)\n", c.CreatorID[0], c.CoopID, "https://eicoop-carpet.netlify.app", c.ContractID, c.CoopID)
		for _, loc := range c.Location {
			fmt.Fprintf(&str, "> *%s*\t%s\n", loc.GuildName, loc.ChannelMention)
		}
		fmt.Fprintf(&str, "> Started: <t:%d:R>\n", c.StartTime.Unix())
		fmt.Fprintf(&str, "> Contract State: *%s*\n", contractStateNames[c.State])
		fmt.Fprintf(&str, "> Hash: *%s*\n", c.ContractHash)
		field = append(field, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%d - **%s/%s**\n", i, c.ContractID, c.CoopID),
			Value:  str.String(),
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

// adminContractReportJSON holds full admin log contract report for admins
type adminContractReportJSON struct {
	CoordinatorID  string                      `json:"coordinator_id,omitempty"`
	GuildName      string                      `json:"guild_name"`
	GuildID        string                      `json:"guild_id"`
	ChannelID      string                      `json:"channel_id"`
	ChannelURL     string                      `json:"channel_url"`
	ContractHash   string                      `json:"contract_hash"`
	ContractID     string                      `json:"contract_id"`
	CoopID         string                      `json:"coop_id"`
	RunType        string                      `json:"run_type"`
	GGType         string                      `json:"gg_type"`
	ContractSize   int64                       `json:"contract_size"`
	StartTimestamp int64                       `json:"start_timestamp"`
	RoleName       string                      `json:"role_name"`
	Members        []adminContractReportMember `json:"members"`
}

// adminContractReportMember represents a single booster in the contract report.
type adminContractReportMember struct {
	UserID     string `json:"user_id"`
	Nick       string `json:"nick"`
	JoinedUnix int64  `json:"joined_unix,omitempty"`
}

// AdminContractReport sends a contract summary plus a JSON attachment containing
func AdminContractReport(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract, targetChannelID string) {
	if contract == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "No contract found.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Respond immediately to buy some time for processing and to avoid interaction timeout
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing Request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	// Get contract data from the list of all contract
	eiContract, ok := ei.EggIncContractsAll[contract.ContractID]
	if !ok { // ContractID not set
		eiContract.MaxCoopSize = len(contract.Boosters)
	}

	// Determine run type based on contract valid from date
	runType := "Unknown"
	validFrom := eiContract.ValidFrom
	if !validFrom.IsZero() {
		switch validFrom.Weekday() {
		case time.Monday:
			runType = "Seasonal"
		case time.Wednesday:
			runType = "Wednesday Leggacy"
		case time.Friday:
			if eiContract.Ultra {
				runType = "Ultra PE Leggacy"
			} else {
				runType = "Non-ultra PE Leggacy"
			}
		}
	}

	coordinatorID := contract.CreatorID[0]

	// Carpet URL for summary view
	carpetURL := fmt.Sprintf("https://eicoop-carpet.netlify.app/%s/%s", contract.ContractID, contract.CoopID)

	// Set contract Start time
	startTime := contract.StartTime
	if !contract.ActualStartTime.IsZero() {
		startTime = contract.ActualStartTime
	}

	// Determine whether if GG was active at the start of the contract
	ggType := "Non-GG"
	if !startTime.IsZero() {
		ggEvent := ei.FindGiftEvent(startTime)
		if ggEvent.EventType != "" {
			if ggEvent.Ultra {
				ggType = "Ultra-GG"
			} else {
				ggType = "GG"
			}
		}
	}

	// Build the list of members sorted by join date
	type boosterEntry struct {
		userID  string
		booster *Booster
	}

	entries := make([]boosterEntry, 0, len(contract.Boosters))
	for userID, booster := range contract.Boosters {
		if booster != nil {
			entries = append(entries, boosterEntry{userID, booster})
		}
	}
	// Sort by join date
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].booster.Register.Before(entries[j].booster.Register)
	})

	reportMembers := make([]adminContractReportMember, 0, len(entries))
	summaryMemberLines := make([]string, 0, len(entries))
	for _, entry := range entries {
		// Use nickname if available, fallback to userID
		nick := entry.booster.Nick
		if nick == "" {
			nick = entry.userID
		}
		// Joined timestamp for the current booster
		joinedUnix := int64(0)
		if !entry.booster.Register.IsZero() {
			joinedUnix = entry.booster.Register.Unix()
		}

		// Build JSON object
		member := adminContractReportMember{
			UserID:     entry.userID,
			Nick:       nick,
			JoinedUnix: joinedUnix,
		}

		reportMembers = append(reportMembers, member)
		summaryMemberLines = append(summaryMemberLines,
			fmt.Sprintf("%s %s (`%s`) joined: %s",
				member.Nick,
				entry.booster.Mention,
				member.UserID,
				bottools.WrapTimestamp(member.JoinedUnix, bottools.TimestampShortTime),
			),
		)
	}

	// Guild and channel info for the contract
	loc := LocationData{
		GuildName:         "Unknown",
		GuildID:           "Unknown",
		ChannelID:         "Unknown",
		GuildContractRole: discordgo.Role{Name: "Unknown"},
	}
	if len(contract.Location) > 0 && contract.Location[0] != nil {
		loc = *contract.Location[0]
	}

	// Generate a link to contract thread
	channelURL := ""
	if loc.GuildID != "" && loc.ChannelID != "" {
		channelURL = fmt.Sprintf("https://discord.com/channels/%s/%s", loc.GuildID, loc.ChannelID)
	}

	reportJSON := adminContractReportJSON{
		CoordinatorID:  coordinatorID,
		GuildName:      loc.GuildName,
		GuildID:        loc.GuildID,
		ChannelID:      loc.ChannelID,
		ChannelURL:     channelURL,
		ContractHash:   contract.ContractHash,
		ContractID:     contract.ContractID,
		CoopID:         contract.CoopID,
		RunType:        runType,
		GGType:         ggType,
		ContractSize:   int64(eiContract.MaxCoopSize),
		StartTimestamp: startTime.Unix(),
		RoleName:       loc.GuildContractRole.Name,
		Members:        reportMembers,
	}

	// Write the summary section of the report
	var summary strings.Builder

	fmt.Fprintf(&summary, `### Admin Logs
Coordinator ID: <@%s> (%s)
Guild Name: *%s*
Channel URL: %s
Contract Hash: *%s*
Contract ID: *%s*
Coop ID: [**⧉**](%s)*%s* 
Run Type: *%s*
GG Type: *%s*
Contract Size: *%d*
Start Time: %s
Role Name: *%s*
`,
		reportJSON.CoordinatorID, reportJSON.CoordinatorID,
		reportJSON.GuildName,
		reportJSON.ChannelURL,
		reportJSON.ContractHash,
		reportJSON.ContractID,
		carpetURL, reportJSON.CoopID,
		reportJSON.RunType,
		reportJSON.GGType,
		reportJSON.ContractSize,
		bottools.WrapTimestamp(reportJSON.StartTimestamp, bottools.TimestampShortDateTime),
		reportJSON.RoleName,
	)

	memberContent := strings.Join(summaryMemberLines, "\n")
	if memberContent == "" {
		memberContent = "No boosters found for this contract.*\n"
	}

	components := []discordgo.MessageComponent{
		&discordgo.TextDisplay{
			Content: summary.String(),
		},
		&discordgo.TextDisplay{
			Content: fmt.Sprintf("## %s\n%s", reportJSON.RoleName, memberContent),
		},
	}

	// Shouldn't ever happen but just to be safe sanitize file names
	sanitizedID := strings.ToLower(fmt.Sprintf("%s-%s", reportJSON.ContractID, reportJSON.CoopID))
	sanitizedID = strings.NewReplacer(
		" ", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		";", "-",
		"\t", "-",
		"\n", "-",
		"\r", "-",
	).Replace(sanitizedID)

	filename := "contract-report-" + sanitizedID + ".json"

	jsonData, err := json.MarshalIndent(reportJSON, "", "  ")
	if err != nil {
		log.Println("Error marshaling contract report JSON:", err)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error formatting contract JSON: " + err.Error(),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
		Components: components,
	})
	if err != nil {
		log.Println("Error sending admin contract summary:", err)
		return
	}

	_, err = s.ChannelMessageSendComplex(targetChannelID, &discordgo.MessageSend{
		Content: summary.String(),
		Files: []*discordgo.File{
			{
				Name:        filename,
				ContentType: "application/json",
				Reader:      bytes.NewReader(jsonData),
			},
		},
		Flags: discordgo.MessageFlagsSuppressEmbeds,
	})
	if restErr, ok := err.(*discordgo.RESTError); ok && restErr.Message != nil {
		log.Printf("Failed to send JSON file to channel %s: HTTP %d, Discord message: %s\n",
			targetChannelID, restErr.Response.StatusCode, restErr.Message.Message)
		return
	}
}

// SlashAdminMembers returns the admin-members slash command definition with set/remove subcommands.
func SlashAdminMembers(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	farmerOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "farmers",
			Description: "List of user mentions or IDs",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "names",
			Description: "Comma-separated list of plain user Names",
			Required:    false,
		},
	}
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Manage farmers as members of this server.",
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "set",
				Description: "Add one or more farmers as members of this server.",
				Options:     farmerOptions,
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "remove",
				Description: "Remove one or more farmers from this server's membership.",
				Options:     farmerOptions,
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "list",
				Description: "List all farmers registered as members of this server.",
			},
		},
	}
}

var adminMembersRe = regexp.MustCompile(`\d+`)

// adminMembersListContent builds the member list string for the current guild.
func adminMembersListContent(s *discordgo.Session, guildID, guildName string) string {
	members := farmerstate.GetGuildMembers(guildID)
	var b strings.Builder
	if len(members) == 0 {
		b.WriteString("No farmers registered for this server.")
		return b.String()
	}
	fmt.Fprintf(&b, "**Members of guild `%s`** (%d)\n", guildName, len(members))
	for _, userID := range members {
		ign := farmerstate.GetMiscSettingString(userID, "ei_ign")
		_, inGuild := s.GuildMember(guildID, userID)
		mention := ""
		if inGuild == nil {
			mention = fmt.Sprintf(" <@%s>", userID)
		}
		if ign != "" {
			fmt.Fprintf(&b, "`%s`%s `%s`\n", userID, mention, ign)
		} else {
			fmt.Fprintf(&b, "`%s`%s\n", userID, mention)
		}
	}
	return b.String()
}

// adminMembersSetContent adds farmers to the guild and returns a result string.
func adminMembersSetContent(s *discordgo.Session, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption, guildID, guildName string) string {
	var affected, skipped, invalid []string

	if opt, ok := optionMap["set-farmers"]; ok {
		for _, userID := range adminMembersRe.FindAllString(opt.StringValue(), -1) {
			if _, err := s.GuildMember(guildID, userID); err != nil {
				invalid = append(invalid, fmt.Sprintf("<@%s>", userID))
				continue
			}
			if farmerstate.AddGuildMembership(userID, guildID) {
				affected = append(affected, fmt.Sprintf("<@%s>", userID))
			} else {
				skipped = append(skipped, fmt.Sprintf("<@%s>", userID))
			}
		}
	}
	if opt, ok := optionMap["set-names"]; ok {
		for name := range strings.SplitSeq(opt.StringValue(), ",") {
			userID := strings.TrimSpace(name)
			if userID == "" {
				continue
			}
			if !farmerstate.FarmerExists(userID) {
				invalid = append(invalid, fmt.Sprintf("`%s` (not found)", userID))
				continue
			}
			if farmerstate.AddGuildMembership(userID, guildID) {
				affected = append(affected, fmt.Sprintf("`%s`", userID))
			} else {
				skipped = append(skipped, fmt.Sprintf("`%s`", userID))
			}
		}
	}

	var b strings.Builder
	if len(affected) > 0 {
		fmt.Fprintf(&b, "Added to guild `%s`: %s", guildName, strings.Join(affected, ", "))
	}
	if len(skipped) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "Already a member: %s", strings.Join(skipped, ", "))
	}
	if len(invalid) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "Not found in this server: %s", strings.Join(invalid, ", "))
	}
	if b.Len() == 0 {
		b.WriteString("No valid farmers provided.")
	}
	return b.String()
}

// adminMembersRemoveContent removes farmers from the guild and returns a result string.
func adminMembersRemoveContent(optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption, guildID, guildName string) string {
	var affected, invalid []string

	if opt, ok := optionMap["remove-farmers"]; ok {
		for _, userID := range adminMembersRe.FindAllString(opt.StringValue(), -1) {
			if !farmerstate.FarmerExists(userID) {
				invalid = append(invalid, fmt.Sprintf("<@%s>", userID))
				continue
			}
			farmerstate.RemoveGuildMembership(userID, guildID)
			affected = append(affected, fmt.Sprintf("<@%s>", userID))
		}
	}
	if opt, ok := optionMap["remove-names"]; ok {
		for name := range strings.SplitSeq(opt.StringValue(), ",") {
			userID := strings.TrimSpace(name)
			if userID == "" {
				continue
			}
			if !farmerstate.FarmerExists(userID) {
				invalid = append(invalid, fmt.Sprintf("`%s` (not found)", userID))
				continue
			}
			farmerstate.RemoveGuildMembership(userID, guildID)
			affected = append(affected, fmt.Sprintf("`%s`", userID))
		}
	}

	var b strings.Builder
	if len(affected) > 0 {
		fmt.Fprintf(&b, "Removed from guild `%s`: %s", guildName, strings.Join(affected, ", "))
	}
	if len(invalid) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "Not found: %s", strings.Join(invalid, ", "))
	}
	if b.Len() == 0 {
		b.WriteString("No valid farmers provided.")
	}
	return b.String()
}

// HandleAdminMembers handles the set, remove, and list subcommands for admin-members.
func HandleAdminMembers(s *discordgo.Session, i *discordgo.InteractionCreate) {
	flags := discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2
	bottools.AcknowledgeResponse(s, i, flags)

	guildID := i.GuildID
	g, _ := s.Guild(guildID)
	guildName := g.Name
	subcmd := i.ApplicationCommandData().Options[0].Name
	optionMap := bottools.GetCommandOptionsMap(i)

	var content string
	var allowedMentions *discordgo.MessageAllowedMentions
	switch subcmd {
	case "list":
		content = adminMembersListContent(s, guildID, guildName)
		allowedMentions = &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}
	case "set":
		content = adminMembersSetContent(s, optionMap, guildID, guildName)
	case "remove":
		content = adminMembersRemoveContent(optionMap, guildID, guildName)
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:           flags,
		AllowedMentions: allowedMentions,
		Components: []discordgo.MessageComponent{
			&discordgo.TextDisplay{Content: content},
		},
	})
}
