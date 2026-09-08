package boost

import (
	"bytes"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime/debug"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/version"
)

const (
	adminGuildStateActionSet = "set-guild-setting"
	adminGuildStateActionGet = "get-guild-settings"
)

// SlashAdminGetContractData is the slash to get contract JSON data
func SlashAdminGetContractData(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Retrieve contract JSON data")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "contract-id",
			Description:  "Select a contract-id",
			Required:     true,
			Autocomplete: true,
		},
		dc.StringOption{
			Name:         "coop-id",
			Description:  "Your coop-id",
			Required:     true,
			Autocomplete: true,
		},
	}
	return &command
}

// SlashAdminListRoles is the slash to info about bot roles
func SlashAdminListRoles(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Display contract role usage")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "contract-id",
			Description:  "Select a contract-id",
			Required:     true,
			Autocomplete: true,
		},
	}
	return &command
}

// SlashAdminGuildStateCommand provides a generic entrypoint for guildstate admin actions.
func SlashAdminGuildStateCommand(cmd string) *dc.Command {
	guildID := guildstate.GetGuildSettingString("DEFAULT", "home_guild")
	if guildID == "" {
		guildID = "DISABLED"
	}

	command := adminGuildCommand(cmd, "Run guildstate admin command with guild override")
	command.GuildID = guildID
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "action",
			Description: "Guildstate command to run",
			Required:    true,
			Choices: []dc.Choice[string]{
				{Name: "set-guild-setting", Value: adminGuildStateActionSet},
				{Name: "get-guild-settings", Value: adminGuildStateActionGet},
			},
		},
		dc.StringOption{
			Name:         "guild-id",
			Description:  "Guild ID override (from persisted guildstate)",
			Required:     true,
			Autocomplete: true,
		},
		dc.StringOption{
			Name:        "setting",
			Description: "Setting key (used by set-guild-setting)",
		},
		dc.StringOption{
			Name:        "value",
			Description: "Optional value (used by set-guild-setting; blank clears)",
		},
	}
	return &command
}

// SlashAdminStatusMessageCommand sets the next bot status message.
func SlashAdminStatusMessageCommand(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Set the next bot status message")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "message",
			Description:  "Status message to use on the next update",
			Required:     true,
			Autocomplete: true,
		},
	}
	return &command
}

func isAdminCommandCaller(client dc.Client, e dc.InteractionEvent) bool {
	userID := e.UserID()
	perms, err := client.UserChannelPermissions(userID, e.ChannelID())
	if err != nil {
		log.Println(err)
	}
	return perms.Administrator() || userID == config.AdminUserID
}

// isAdminBotController returns true if the calling user has the guild's bot_controller role.
// Falls back to false if the setting is not configured or the member cannot be fetched.
func isAdminBotController(client dc.Client, e dc.InteractionEvent) bool {
	if e.GuildID() == "" {
		return false
	}
	roleID := guildstate.GetGuildSettingString(e.GuildID(), "bot_controller")
	if roleID == "" {
		return false
	}
	// Strip role mention formatting "<@&snowflake>" if present.
	roleID = strings.TrimPrefix(roleID, "<@&")
	roleID = strings.TrimSuffix(roleID, ">")
	roleID = strings.TrimSpace(roleID)
	member, err := client.GuildMember(e.GuildID(), e.UserID())
	if err != nil {
		log.Println(err)
		return false
	}
	return slices.Contains(member.Roles, roleID)
}

// HandleAdminGuildStateAutoComplete serves guild-id suggestions from persisted guildstate keys.
func HandleAdminGuildStateAutoComplete(client dc.Client, e *dc.AutocompleteEvent) {
	search := ""
	if name, value := e.FocusedOption(); name == "guild-id" {
		search = strings.TrimSpace(value)
	}

	ids, err := guildstate.GetAllGuildIDs()
	if err != nil {
		log.Println(err)
		ids = []string{}
	}

	searchLower := strings.ToLower(search)
	choices := make([]dc.Choice[string], 0, 25)
	for _, id := range ids {
		choiceName := id
		if guild, guildErr := client.Guild(id); guildErr == nil && guild != nil {
			guildName := strings.TrimSpace(guild.Name)
			if guildName != "" {
				choiceName = fmt.Sprintf("%s (%s)", guildName, id)
			}
		}

		if searchLower != "" && !strings.Contains(strings.ToLower(choiceName), searchLower) && !strings.Contains(strings.ToLower(id), searchLower) {
			continue
		}
		choices = append(choices, dc.Choice[string]{Name: choiceName, Value: id})
		if len(choices) >= 25 {
			break
		}
	}

	_ = e.RespondChoices(choices)
}

// HandleAdminGuildStateCommand routes to guildstate handlers with explicit guild override.
func HandleAdminGuildStateCommand(client dc.Client, e *dc.CommandEvent) {
	if !isAdminCommandCaller(client, e) {
		_ = e.Respond(dc.Message{
			Content:   "You are not authorized to use this command.",
			Ephemeral: true,
		})
		return
	}

	action := ""
	guildID := ""
	setting := ""
	value := ""

	if opt, ok := e.OptString("action"); ok {
		action = strings.TrimSpace(opt)
	}
	if opt, ok := e.OptString("guild-id"); ok {
		guildID = strings.TrimSpace(opt)
	}
	if opt, ok := e.OptString("setting"); ok {
		setting = strings.TrimSpace(opt)
	}
	if opt, ok := e.OptString("value"); ok {
		value = strings.TrimSpace(opt)
	}

	if guildID == "" {
		_ = e.Respond(dc.Message{Content: "guild-id is required.", Ephemeral: true})
		return
	}

	switch action {
	case adminGuildStateActionSet:
		if setting == "" {
			_ = e.Respond(dc.Message{
				Content:   "setting is required when action is set-guild-setting.",
				Ephemeral: true,
			})
			return
		}
		guildstate.SetGuildSettingForGuild(client, e, guildID, setting, value)
	case adminGuildStateActionGet:
		guildstate.GetGuildSettingsForGuild(client, e, guildID)
	default:
		_ = e.Respond(dc.Message{
			Content:   "action must be one of: set-guild-setting, get-guild-settings",
			Ephemeral: true,
		})
	}
}

// HandleAdminStatusMessageAutoComplete provides status message suggestions.
func HandleAdminStatusMessageAutoComplete(e *dc.AutocompleteEvent) {
	search := ""
	if name, value := e.FocusedOption(); name == "message" {
		search = strings.TrimSpace(value)
	}

	messages := ei.GetStatusMessageChoices(search, 25)
	choices := make([]dc.Choice[string], 0, len(messages))
	for _, message := range messages {
		if len(message) > 100 {
			continue
		}
		choices = append(choices, dc.Choice[string]{
			Name:  message,
			Value: message,
		})
	}

	_ = e.RespondChoices(choices)
}

// HandleAdminStatusMessageCommand sets a one-time status message override.
func HandleAdminStatusMessageCommand(client dc.Client, e *dc.CommandEvent) {
	if !isAdminCommandCaller(client, e) {
		_ = e.Respond(dc.Message{
			Content:   "You are not authorized to use this command.",
			Ephemeral: true,
		})
		return
	}

	message := ""
	discordID := e.UserID()
	if opt, ok := e.OptString("message"); ok {
		message = strings.TrimSpace(opt)
	}

	if err := ei.SetNextStatusMessageOverride(discordID, message); err != nil {
		_ = e.Respond(dc.Message{Content: err.Error(), Ephemeral: true})
		return
	}

	_ = e.Respond(dc.Message{
		Content:   fmt.Sprintf("Next status message set to: %q", message),
		Ephemeral: true,
	})
}

// HandleAdminListRoles is the handler for the list roles command.
func HandleAdminListRoles(client dc.Client, e *dc.CommandEvent) {
	var contractID string
	var builder strings.Builder
	if opt, ok := e.OptString("contract-id"); ok {
		contractID = strings.TrimSpace(opt)
	}

	var components []dc.LayoutComponent
	//

	guildRoles, err := client.GuildRoles(e.GuildID())
	if err != nil {
		builder.WriteString("Error retrieving guild roles: ")
		builder.WriteString(err.Error())
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

	components = append(components, dc.TextDisplay{
		Content: builder.String(),
	})

	_ = e.Respond(dc.Message{
		Ephemeral:  true,
		Components: components,
	})
}

// finishContractByHash is called only when the contract is complete
func finishContractByHash(client dc.Client, contractHash string) error {
	var contract *Contract
	ContractsMutex.RLock()
	for _, c := range Contracts {
		if c.ContractHash == contractHash {
			contract = c
			break
		}
	}
	ContractsMutex.RUnlock()
	if contract == nil {
		return errors.New(errorNoContract)
	}

	// Get rid of any roles
	for _, loc := range contract.Location {
		err := client.DeleteGuildRole(loc.GuildID, loc.GuildContractRole.ID)
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
	ContractsMutex.Lock()
	delete(Contracts, contract.ContractHash)
	ContractsMutex.Unlock()

	return nil
}

// HandleCoopAutoComplete will handle the contract auto complete of contract-id's
func HandleCoopAutoComplete(e *dc.AutocompleteEvent) {
	// The contract-id field has its own suggestions; this handler only serves
	// the coop-id field, narrowed to the contract already chosen.
	if focused, _ := e.FocusedOption(); focused == "contract-id" {
		HandleContractAutoComplete(e)
		return
	}

	contractID := ""
	coopID := ""
	if opt, ok := e.OptString("contract-id"); ok {
		contractID = opt
	}
	if opt, ok := e.OptString("coop-id"); ok {
		coopID = opt
	}

	choices := make([]dc.Choice[string], 0)

	ContractsMutex.RLock()
	for _, c := range Contracts {
		if c.ContractID == contractID {
			// if coopID is empty, or contains the search string
			if coopID == "" || strings.Contains(c.CoopID, coopID) {
				choices = append(choices, dc.Choice[string]{
					Name:  c.CoopID,
					Value: c.CoopID,
				})
			}
		}
	}
	ContractsMutex.RUnlock()

	sort.Slice(choices, func(i, j int) bool {
		return choices[i].Name < choices[j].Name
	})

	_ = e.RespondChoices(choices)
}

// HandleAdminGetContractData gets JSON data about a contract given the contract and coop id.
func HandleAdminGetContractData(e *dc.CommandEvent) {
	var contractID string
	var coopID string

	if opt, ok := e.OptString("contract-id"); ok {
		contractID = strings.TrimSpace(opt)
	}
	if opt, ok := e.OptString("coop-id"); ok {
		coopID = strings.TrimSpace(opt)
	}

	// Find a contract by contract ID and coop ID
	contract := FindContractByIDs(e.ChannelID(), contractID, coopID)

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
		builder.WriteString("Error formatting JSON data. ")
		builder.WriteString(err.Error())
	} else {
		err = json.Indent(buf, jsonData, "", "  ")
		if err != nil {
			builder.WriteString("Error formatting JSON data. ")
			builder.WriteString(err.Error())
		} else {
			// Create io.Reader from JSON string
			reader = bytes.NewReader(buf.Bytes())
		}
	}

	_ = e.Respond(dc.Message{
		Content:   fmt.Sprintf("Here is the JSON data for contract %s/%s", contractID, coopID),
		Ephemeral: true,
		Files: []dc.File{
			{
				Name:        filename,
				ContentType: "application/json",
				Reader:      reader,
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
func AdminContractReport(client dc.Client, e *dc.ComponentEvent, contract *Contract, targetChannelID string) {
	if contract == nil {
		_ = e.Respond(dc.Message{
			Content:   "No contract found.",
			Ephemeral: true,
		})
		return
	}

	// Respond immediately to buy some time for processing and to avoid interaction timeout
	_ = e.Defer(true)

	type reportBoosterSnapshot struct {
		UserID   string
		Nick     string
		Mention  string
		Register time.Time
	}

	contractHash := ""
	contractID := ""
	coopID := ""
	coordinatorID := ""
	var startTime time.Time
	var actualStartTime time.Time
	loc := LocationData{
		GuildName:         "Unknown",
		GuildID:           "Unknown",
		ChannelID:         "Unknown",
		GuildContractRole: GuildRole{Name: "Unknown"},
	}
	var boosterSnapshots []reportBoosterSnapshot

	contract.mutex.Lock()
	contractHash = contract.ContractHash
	contractID = contract.ContractID
	coopID = contract.CoopID
	startTime = contract.StartTime
	actualStartTime = contract.ActualStartTime

	if len(contract.CreatorID) > 0 {
		coordinatorID = contract.CreatorID[0]
	}

	if len(contract.Location) > 0 && contract.Location[0] != nil {
		loc = *contract.Location[0]
	}

	boosterSnapshots = make([]reportBoosterSnapshot, 0, len(contract.Boosters))
	for userID, booster := range contract.Boosters {
		if booster == nil {
			continue
		}
		boosterSnapshots = append(boosterSnapshots, reportBoosterSnapshot{
			UserID:   userID,
			Nick:     booster.Nick,
			Mention:  booster.Mention,
			Register: booster.Register,
		})
	}
	contract.mutex.Unlock()

	// Get contract data from the list of all contract
	eiContract, ok := ei.EggIncContractsAll[contractID]
	if !ok { // ContractID not set
		eiContract.MaxCoopSize = len(boosterSnapshots)
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

	// Carpet URL for summary view
	carpetURL := fmt.Sprintf("https://eicoop-carpet.netlify.app/%s/%s", contractID, coopID)

	// Set contract Start time
	if !actualStartTime.IsZero() {
		startTime = actualStartTime
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
		userID   string
		nick     string
		mention  string
		register time.Time
	}

	entries := make([]boosterEntry, 0, len(boosterSnapshots))
	for _, booster := range boosterSnapshots {
		entries = append(entries, boosterEntry{
			userID:   booster.UserID,
			nick:     booster.Nick,
			mention:  booster.Mention,
			register: booster.Register,
		})
	}
	// Sort by join date
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].register.Before(entries[j].register)
	})

	reportMembers := make([]adminContractReportMember, 0, len(entries))
	summaryMemberLines := make([]string, 0, len(entries))
	for _, entry := range entries {
		// Use nickname if available, fallback to userID
		nick := entry.nick
		if nick == "" {
			nick = entry.userID
		}
		// Joined timestamp for the current booster
		joinedUnix := int64(0)
		if !entry.register.IsZero() {
			joinedUnix = entry.register.Unix()
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
				entry.mention,
				member.UserID,
				bottools.WrapTimestamp(member.JoinedUnix, bottools.TimestampShortTime),
			),
		)
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
		ContractHash:   contractHash,
		ContractID:     contractID,
		CoopID:         coopID,
		RunType:        runType,
		GGType:         ggType,
		ContractSize:   int64(eiContract.MaxCoopSize),
		StartTimestamp: startTime.Unix(),
		RoleName:       loc.GuildContractRole.Name,
		Members:        reportMembers,
	}

	// Write the summary section of the report
	var summary strings.Builder
	coordinatorSummary := "Unknown"
	if reportJSON.CoordinatorID != "" {
		coordinatorSummary = fmt.Sprintf("<@%s> (%s)", reportJSON.CoordinatorID, reportJSON.CoordinatorID)
	}

	fmt.Fprintf(&summary, `### Admin Logs
Coordinator ID: %s
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
		coordinatorSummary,
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

	var memberChunks []string
	if len(summaryMemberLines) == 0 {
		memberChunks = append(memberChunks, "*No boosters found for this contract.*\n")
	} else {
		var currentChunk strings.Builder
		for _, line := range summaryMemberLines {
			if currentChunk.Len()+len(line)+1 > 1800 {
				memberChunks = append(memberChunks, currentChunk.String())
				currentChunk.Reset()
			}
			if currentChunk.Len() > 0 {
				currentChunk.WriteString("\n")
			}
			currentChunk.WriteString(line)
		}
		if currentChunk.Len() > 0 {
			memberChunks = append(memberChunks, currentChunk.String())
		}
	}

	components := []dc.LayoutComponent{
		dc.TextDisplay{
			Content: summary.String(),
		},
	}
	if len(memberChunks) > 0 {
		components = append(components, dc.TextDisplay{
			Content: fmt.Sprintf("## %s\n%s", reportJSON.RoleName, memberChunks[0]),
		})
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
		// The interaction was deferred above, so this has to be a followup.
		// A second initial response is rejected as already acknowledged.
		_ = e.Followup(dc.Message{
			Content:   "Error formatting contract JSON: " + err.Error(),
			Ephemeral: true,
		})
		return
	}

	err = e.Followup(dc.Message{
		Ephemeral:  true,
		Components: components,
	})
	if err != nil {
		log.Println("Error sending admin contract summary:", err)
		return
	}

	for _, chunk := range memberChunks[1:] {
		err = e.Followup(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{
					Content: chunk,
				},
			},
		})
		if err != nil {
			log.Println("Error sending admin contract member chunk:", err)
		}
	}

	_, err = client.SendMessage(targetChannelID, dc.Message{
		Content: summary.String(),
		Files: []dc.File{
			{
				Name:        filename,
				ContentType: "application/json",
				Reader:      bytes.NewReader(jsonData),
			},
		},
		SuppressEmbeds: true,
	})
	if apiErr, ok := dc.AsAPIError(err); ok {
		log.Printf("Failed to send JSON file to channel %s: HTTP %d, Discord message: %s\n",
			targetChannelID, apiErr.StatusCode, apiErr.Message)
		return
	}
}

// SlashAdminMembers returns the admin-members slash command definition with set/remove subcommands.
func SlashAdminMembers(cmd string) *dc.Command {
	farmerOptions := []dc.Option{
		dc.StringOption{
			Name:        "farmers",
			Description: "List of user mentions or IDs",
		},
		dc.StringOption{
			Name:        "names",
			Description: "Comma-separated list of plain user Names",
		},
	}
	command := adminGuildCommand(cmd, "Manage farmers as members of this server.")
	command.Options = []dc.Option{
		dc.SubCommand{
			Name:        "set",
			Description: "Add one or more farmers as members of this server.",
			Options:     farmerOptions,
		},
		dc.SubCommand{
			Name:        "remove",
			Description: "Remove one or more farmers from this server's membership.",
			Options:     farmerOptions,
		},
		dc.SubCommand{
			Name:        "list",
			Description: "List all farmers registered as members of this server.",
		},
	}
	return &command
}

var adminMembersRe = regexp.MustCompile(`\d+`)

// adminMembersListContent builds the member list string for the current guild.
func adminMembersListContent(client dc.Client, guildID, guildName string) string {
	members := farmerstate.GetGuildMembers(guildID)
	var b strings.Builder
	if len(members) == 0 {
		b.WriteString("No farmers registered for this server.")
		return b.String()
	}
	fmt.Fprintf(&b, "**Members of guild `%s`** (%d)\n", guildName, len(members))
	for _, userID := range members {
		ign := farmerstate.GetMiscSettingString(userID, "ei_ign")
		_, inGuild := client.GuildMember(guildID, userID)
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
func adminMembersSetContent(client dc.Client, e *dc.CommandEvent, guildID, guildName string) string {
	var affected, skipped, invalid []string

	if opt, ok := e.OptString("set-farmers"); ok {
		for _, userID := range adminMembersRe.FindAllString(opt, -1) {
			if _, err := client.GuildMember(guildID, userID); err != nil {
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
	if opt, ok := e.OptString("set-names"); ok {
		for name := range strings.SplitSeq(opt, ",") {
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
func adminMembersRemoveContent(e *dc.CommandEvent, guildID, guildName string) string {
	var affected, invalid []string

	if opt, ok := e.OptString("remove-farmers"); ok {
		for _, userID := range adminMembersRe.FindAllString(opt, -1) {
			if !farmerstate.FarmerExists(userID) {
				invalid = append(invalid, fmt.Sprintf("<@%s>", userID))
				continue
			}
			farmerstate.RemoveGuildMembership(userID, guildID)
			affected = append(affected, fmt.Sprintf("<@%s>", userID))
		}
	}
	if opt, ok := e.OptString("remove-names"); ok {
		for name := range strings.SplitSeq(opt, ",") {
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
func HandleAdminMembers(client dc.Client, e *dc.CommandEvent) {
	_ = e.Defer(true)

	guildID := e.GuildID()
	g, _ := client.Guild(guildID)
	guildName := g.Name
	subcmd, _ := e.Subcommand()

	var content string
	var allowedMentions *dc.AllowedMentions
	switch subcmd {
	case "list":
		content = adminMembersListContent(client, guildID, guildName)
		allowedMentions = &dc.AllowedMentions{}
	case "set":
		content = adminMembersSetContent(client, e, guildID, guildName)
	case "remove":
		content = adminMembersRemoveContent(e, guildID, guildName)
	}

	_ = e.Followup(dc.Message{
		Ephemeral:       true,
		AllowedMentions: allowedMentions,
		Components: []dc.LayoutComponent{
			dc.TextDisplay{Content: content},
		},
	})
}

// SlashAdminExitCommand provides an admin-only command to gracefully exit the bot.
func SlashAdminExitCommand(cmd string) *dc.Command {

	guildID := guildstate.GetGuildSettingString("DEFAULT", "home_guild")
	if guildID == "" {
		guildID = "DISABLED"
	}

	command := guildOnlyCommand(cmd, "Gracefully exit the bot so that it can be restarted by its controlling daemon")
	command.GuildID = guildID
	return &command
}

// HandleAdminExitCommand handles the admin-exit command to gracefully shutdown the bot.
func HandleAdminExitCommand(client dc.Client, e *dc.CommandEvent) {
	defer func() {
		if recovered := recover(); recovered != nil {
			handleAdminExitPanic(e, "HandleAdminExitCommand", recovered)
		}
	}()

	if !isAdminCommandCaller(client, e) && !isAdminBotController(client, e) {
		_ = e.Respond(dc.Message{
			Content:   "You are not authorized to use this command.",
			Ephemeral: true,
		})
		return
	}

	_ = e.Defer(true)

	if err := e.Followup(dc.Message{
		Ephemeral:  true,
		Components: buildAdminExitResponse(0),
	}); err != nil {
		log.Println("Error sending admin-exit follow-up message:", err)
	}
}

func handleAdminExitPanic(e dc.InteractionEvent, handlerName string, recovered any) {
	log.Printf("panic in %s: %v\n%s", handlerName, recovered, string(debug.Stack()))

	if e == nil {
		return
	}

	message := "Internal error while handling admin-exit. Bot restart was not requested. Please try again."
	if err := e.Respond(dc.Message{Content: message, Ephemeral: true}); err != nil {
		// The interaction was already answered, so a followup is the only way
		// left to say anything.
		_ = e.Followup(dc.Message{Content: message, Ephemeral: true})
	}
}

func getActiveContractsForAdminExit(now time.Time) []*Contract {
	activeContracts := make([]*Contract, 0)

	//ContractsMutex.RLock()
	for _, c := range Contracts {
		if c == nil {
			continue
		}
		if c.State == ContractStateCompleted || c.State == ContractStateArchive || c.State == ContractStateSignup {
			continue
		}
		if !c.LastInteractionTime.IsZero() && now.Sub(c.LastInteractionTime) > 3*time.Hour {
			continue
		}
		activeContracts = append(activeContracts, c)
	}
	//ContractsMutex.RUnlock()

	// Sort active contracts by last interaction time descending (most recently active first)
	sort.Slice(activeContracts, func(idx, jdx int) bool {
		return activeContracts[idx].LastInteractionTime.After(activeContracts[jdx].LastInteractionTime)
	})

	return activeContracts
}

func buildAdminExitResponse(page int) []dc.LayoutComponent {
	activeContracts := getActiveContractsForAdminExit(time.Now())

	runningVer, runningRev, runningTime, diskRev, diskTime := getVersionAndRevisionInfo()

	var b strings.Builder
	b.WriteString("## ⚠️ Bot Exit & Restart Confirmation\n")
	fmt.Fprintf(&b, "**Running Version:** `%s` (Commit: `%s`, Built: `%s`)\n", runningVer, shortenRevision(runningRev), runningTime)
	if runningRev != diskRev && diskRev != "Unknown" {
		fmt.Fprintf(&b, "**Disk Version (Pending):** Commit `%s` (Built: `%s`)\n", shortenRevision(diskRev), diskTime)
		b.WriteString("⚠️ *A new binary version is detected on disk. Confirming restart will load this version.*\n\n")
	} else {
		b.WriteString("**Disk Version:** Matches running version.\n\n")
	}

	const pageSize = 5
	totalContracts := len(activeContracts)
	totalPages := (totalContracts + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	if totalContracts == 0 {
		b.WriteString("There are currently no active contracts.\n\n")
	} else {
		fmt.Fprintf(&b, "Here is a list of active contracts (Page %d of %d):\n\n", page+1, totalPages)
		startIndex := page * pageSize
		endIndex := startIndex + pageSize
		if endIndex > totalContracts {
			endIndex = totalContracts
		}

		for _, c := range activeContracts[startIndex:endIndex] {
			stateStr := "Unknown"
			switch c.State {
			case ContractStateSignup:
				stateStr = "Signup"
			case ContractStateFastrun:
				stateStr = "🚀 Boosting (Fastrun)"
			case ContractStateWaiting:
				stateStr = "Waiting"
			case ContractStateBanker:
				stateStr = "💰 Boosting (Banker)"
			}

			lastInteraction := "Never"
			if !c.LastInteractionTime.IsZero() {
				lastInteraction = fmt.Sprintf("%s (%s)",
					bottools.WrapTimestamp(c.LastInteractionTime.Unix(), bottools.TimestampShortDateTime),
					bottools.WrapTimestamp(c.LastInteractionTime.Unix(), bottools.TimestampRelativeTime))
			}

			fmt.Fprintf(&b, "- **%s** (`%s/%s`)\n  ↳ State: `%s`\n  ↳ Last Interaction: %s\n",
				c.Name, c.ContractID, c.CoopID, stateStr, lastInteraction)
		}
		b.WriteString("\n")
	}
	b.WriteString("**Are you sure you want to gracefully exit the bot for a restart?**")

	confirmBtn := dc.Button{
		Label:    "Confirm Restart",
		Style:    dc.ButtonDanger,
		CustomID: "admin_exit#confirm",
		Emoji:    &dc.Emoji{Name: "🔄"},
	}
	cancelBtn := dc.Button{
		Label:    "Cancel",
		Style:    dc.ButtonSecondary,
		CustomID: "admin_exit#cancel",
		Emoji:    &dc.Emoji{Name: "❌"},
	}

	var row1Components []dc.InteractiveComponent
	if totalPages > 1 {
		prevBtn := dc.Button{
			Label:    "Previous Page",
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("admin_exit#page#%d", page-1),
			Emoji:    &dc.Emoji{Name: "◀️"},
			Disabled: page == 0,
		}
		nextBtn := dc.Button{
			Label:    "Next Page",
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("admin_exit#page#%d", page+1),
			Emoji:    &dc.Emoji{Name: "▶️"},
			Disabled: page == totalPages-1,
		}
		row1Components = append(row1Components, prevBtn, nextBtn)
	}

	row1Components = append(row1Components, confirmBtn, cancelBtn)

	components := []dc.LayoutComponent{
		dc.TextDisplay{Content: b.String()},
		dc.ActionRow{
			Components: row1Components,
		},
	}

	return components
}

// HandleAdminExitButton handles the admin-exit button clicks (confirm, cancel, and page navigation).
func HandleAdminExitButton(client dc.Client, e *dc.ComponentEvent) {
	defer func() {
		if recovered := recover(); recovered != nil {
			handleAdminExitPanic(e, "HandleAdminExitButton", recovered)
		}
	}()

	if !isAdminCommandCaller(client, e) && !isAdminBotController(client, e) {
		_ = e.Respond(dc.Message{
			Content:   "You are not authorized to use this button.",
			Ephemeral: true,
		})
		return
	}

	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		_ = e.Respond(dc.Message{Content: "Invalid action.", Ephemeral: true})
		return
	}

	action := parts[1]

	respondAndClose := func(content string) {
		err := e.Update(dc.Message{
			Ephemeral: true,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{Content: content},
			},
		})
		if err != nil {
			log.Println("Error updating exit button dialog:", err)
		}
	}

	switch action {
	case "confirm":
		respondAndClose("Bot is exiting gracefully for a restart...")
		// Run graceful shutdown in a goroutine so that the response is fully sent and processed
		go func() {
			log.Println("Exit confirmed by administrator")
			SaveAllData()
			time.Sleep(5 * time.Second)
			if serr := syscall.Kill(os.Getpid(), syscall.SIGTERM); serr != nil {
				log.Printf("Exit command error: could not signal shutdown (forcing exit): %v", serr)
				os.Exit(1)
			}
		}()

	case "cancel":
		respondAndClose("Restart cancelled. The bot remains active.")

	case "page":
		page := 0
		if len(parts) >= 3 {
			if p, err := strconv.Atoi(parts[2]); err == nil {
				page = p
			}
		}
		_ = e.Update(dc.Message{
			Ephemeral:  true,
			Components: buildAdminExitResponse(page),
		})
	}
}

var (
	startupExeModTime time.Time
	startupExeSize    int64
)

func init() {
	if exePath, err := os.Executable(); err == nil {
		if info, err := os.Stat(exePath); err == nil {
			startupExeModTime = info.ModTime()
			startupExeSize = info.Size()
		}
	} else if info, err := os.Stat(os.Args[0]); err == nil {
		startupExeModTime = info.ModTime()
		startupExeSize = info.Size()
	}
}

func getVersionAndRevisionInfo() (runningVer, runningRev, runningTime, diskRev, diskTime string) {
	runningVer = version.Version
	if runningVer == "" {
		runningVer = "Unknown"
	}

	runningRev = "Unknown"
	runningTime = "Unknown"
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				runningRev = s.Value
			case "vcs.time":
				runningTime = s.Value
			}
		}
	}

	diskRev = "Unknown"
	diskTime = "Unknown"
	if exePath, err := os.Executable(); err == nil {
		if info, err := os.Stat(exePath); err == nil {
			if !startupExeModTime.IsZero() && (info.ModTime().After(startupExeModTime) || info.Size() != startupExeSize) {
				diskRev = "New Version Detected (Disk Changed)"
				diskTime = info.ModTime().Format("2006-01-02 15:04:05")
				return
			}
		}
		if bi, err := buildinfo.ReadFile(exePath); err == nil && bi != nil {
			for _, s := range bi.Settings {
				switch s.Key {
				case "vcs.revision":
					diskRev = s.Value
				case "vcs.time":
					diskTime = s.Value
				}
			}
		}
	} else {
		if info, err := os.Stat(os.Args[0]); err == nil {
			if !startupExeModTime.IsZero() && (info.ModTime().After(startupExeModTime) || info.Size() != startupExeSize) {
				diskRev = "New Version Detected (Disk Changed)"
				diskTime = info.ModTime().Format("2006-01-02 15:04:05")
				return
			}
		}
		if bi, err := buildinfo.ReadFile(os.Args[0]); err == nil && bi != nil {
			for _, s := range bi.Settings {
				switch s.Key {
				case "vcs.revision":
					diskRev = s.Value
				case "vcs.time":
					diskTime = s.Value
				}
			}
		}
	}

	return
}

func shortenRevision(rev string) string {
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}
