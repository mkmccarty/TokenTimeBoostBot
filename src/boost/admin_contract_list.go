package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

const (
	adminContractListHandlerPrefix = "admin-contract-list"
	adminContractListSessionTTL    = 15 * time.Minute
)

const (
	discordMessageContentLimit = 2000
	discordEmbedDescLimit      = 4096
	discordEmbedFieldNameLimit = 256
	discordEmbedTotalCharLimit = 6000
)

type adminContractListSession struct {
	id                string
	userID            string
	selectedGuildID   string
	selectedGuildName string
	allowGuildSelect  bool
	selectedIndex     int
	finishArmed       bool
	statusMessage     string
	expiresAt         time.Time
}

type adminContractListGuild struct {
	ID        string
	Name      string
	Contracts []*Contract
}

var adminContractListSessions = make(map[string]*adminContractListSession)

// HandleAdminContractList opens the admin contract list panel.
//
// It still takes a raw session because ArchiveContracts is not on the facade
// yet.
func HandleAdminContractList(client dc.Client, e *dc.CommandEvent) {
	userID := e.UserID()

	// Only allow command if users is in the admin list
	perms, err := client.UserChannelPermissions(userID, e.ChannelID())
	if err != nil {
		log.Println(err)
	}
	if !perms.Administrator() {
		_ = e.Respond(dc.Message{
			Content:   "You are not authorized to use this command.",
			Ephemeral: true,
		})
		return
	}

	if err = e.Defer(true); err != nil {
		log.Println(err)
		return
	}

	ArchiveContracts(client)

	cleanupAdminContractListSessions()
	selectedGuildName := e.GuildID()
	if guild, guildErr := client.Guild(e.GuildID()); guildErr == nil && guild != nil && strings.TrimSpace(guild.Name) != "" {
		selectedGuildName = strings.TrimSpace(guild.Name)
	}
	homeGuildID := guildstate.GetGuildSettingString("DEFAULT", "home_guild")
	allowGuildSelect := homeGuildID != "" && e.GuildID() == homeGuildID
	session := &adminContractListSession{
		id:                fmt.Sprintf("%d", time.Now().UnixNano()),
		userID:            userID,
		selectedGuildID:   e.GuildID(),
		selectedGuildName: selectedGuildName,
		allowGuildSelect:  allowGuildSelect,
		selectedIndex:     0,
		finishArmed:       false,
		statusMessage:     "",
		expiresAt:         time.Now().Add(adminContractListSessionTTL),
	}
	adminContractListSessions[session.id] = session

	content, components := renderAdminContractListPanel(session, false)
	if err = e.Followup(dc.Message{
		Content:        content,
		Components:     components,
		Ephemeral:      true,
		SuppressEmbeds: true,
		ComponentsV1:   true,
	}); err != nil {
		log.Println(err)
		delete(adminContractListSessions, session.id)
	}
}

// HandleAdminContractListComponent drives the admin contract list panel.
//
// It still takes a raw session because finishContractByHash and
// ArchiveContracts are not on the facade yet.
func HandleAdminContractListComponent(client dc.Client, e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 3 {
		_ = e.Respond(dc.Message{Content: "Invalid contract list action.", Ephemeral: true})
		return
	}

	cleanupAdminContractListSessions()
	sessionID := parts[1]
	action := parts[2]
	session, ok := adminContractListSessions[sessionID]
	if !ok {
		_ = e.Respond(dc.Message{
			Content:   "This contract list panel has expired. Please run the command again.",
			Ephemeral: true,
		})
		return
	}

	userID := e.UserID()
	if session.userID != userID {
		_ = e.Respond(dc.Message{Content: "Only the command caller can use this panel.", Ephemeral: true})
		return
	}

	perms, err := client.UserChannelPermissions(userID, e.ChannelID())
	if err != nil {
		log.Println(err)
	}
	if !perms.Administrator() {
		_ = e.Respond(dc.Message{Content: "You are not authorized to use this command.", Ephemeral: true})
		return
	}

	session.expiresAt = time.Now().Add(adminContractListSessionTTL)
	session.statusMessage = ""

	guilds := buildAdminContractListGuilds(session.selectedGuildID, session.selectedGuildName)
	currentContracts := adminContractListContractsForGuild(guilds, session.selectedGuildID)

	navigationAction := false
	switch action {
	case "close":
		delete(adminContractListSessions, session.id)
		_ = e.Update(dc.Message{
			Content:        "Contract list closed.",
			SuppressEmbeds: true,
		})
		return
	case "guild-select":
		if !session.allowGuildSelect {
			break
		}
		values := e.Values()
		if len(values) > 0 {
			session.selectedGuildID = values[0]
			session.selectedIndex = 0
			navigationAction = true
		}
	case "first":
		session.selectedIndex = 0
		navigationAction = true
	case "prev":
		if len(currentContracts) > 0 {
			session.selectedIndex--
			if session.selectedIndex < 0 {
				session.selectedIndex = len(currentContracts) - 1
			}
		}
		navigationAction = true
	case "next":
		if len(currentContracts) > 0 {
			session.selectedIndex++
			if session.selectedIndex >= len(currentContracts) {
				session.selectedIndex = 0
			}
		}
		navigationAction = true
	case "last":
		if len(currentContracts) > 0 {
			session.selectedIndex = len(currentContracts) - 1
		} else {
			session.selectedIndex = 0
		}
		navigationAction = true
	case "finish":
		if len(currentContracts) == 0 {
			session.statusMessage = "No contract is selected for this guild."
			break
		}
		if session.selectedIndex < 0 || session.selectedIndex >= len(currentContracts) {
			session.selectedIndex = 0
		}
		selected := currentContracts[session.selectedIndex]
		if selected == nil {
			session.statusMessage = "Selected contract is no longer available."
			break
		}
		if !session.finishArmed {
			session.finishArmed = true
			session.statusMessage = "Finish armed. Press the same button again to finish this contract."
			break
		}

		session.finishArmed = false
		session.statusMessage = "Deleting selected contract..."
		loadingContent, loadingComponents := renderAdminContractListPanel(session, true)
		if err = e.Update(dc.Message{
			Content:        loadingContent,
			Components:     loadingComponents,
			SuppressEmbeds: true,
			ComponentsV1:   true,
		}); err != nil {
			log.Println(err)
			return
		}

		err = finishContractByHash(client, selected.ContractHash)
		if err != nil {
			session.statusMessage = "Unable to finish contract: " + err.Error()
		} else {
			session.statusMessage = fmt.Sprintf("Finished contract **%s/%s**.", selected.ContractID, selected.CoopID)
			session.selectedIndex = 0
			ArchiveContracts(client)
		}

		updatedContent, updatedComponents := renderAdminContractListPanel(session, false)
		if err = e.EditFollowup(e.MessageID(), dc.Message{
			Content:      updatedContent,
			Components:   updatedComponents,
			ComponentsV1: true,
		}); err != nil {
			log.Println(err)
		}
		return
	default:
		session.statusMessage = "Unknown contract list action."
	}

	if navigationAction {
		session.finishArmed = false
	}

	content, components := renderAdminContractListPanel(session, false)

	_ = e.Update(dc.Message{
		Content:        content,
		Components:     components,
		SuppressEmbeds: true,
		ComponentsV1:   true,
	})
}

func cleanupAdminContractListSessions() {
	now := time.Now()
	for id, session := range adminContractListSessions {
		if session.expiresAt.Before(now) {
			delete(adminContractListSessions, id)
		}
	}
}

func renderAdminContractListPanel(session *adminContractListSession, deleting bool) (string, []dc.LayoutComponent) {
	guilds := buildAdminContractListGuilds(session.selectedGuildID, session.selectedGuildName)

	if len(guilds) == 0 {
		content := "No contracts are currently tracked."
		if session.statusMessage != "" {
			content += "\n\n" + session.statusMessage
		}
		return content, []dc.LayoutComponent{
			dc.ActionRow{Components: []dc.InteractiveComponent{
				dc.Button{Label: "Close", Style: dc.ButtonDanger, CustomID: fmt.Sprintf("%s#%s#close", adminContractListHandlerPrefix, session.id)},
			}},
		}
	}

	if adminContractListGuildIndex(guilds, session.selectedGuildID) == -1 {
		session.selectedGuildID = guilds[0].ID
		session.selectedIndex = 0
		session.finishArmed = false
	}

	selectedGuildIdx := adminContractListGuildIndex(guilds, session.selectedGuildID)
	if selectedGuildIdx < 0 {
		selectedGuildIdx = 0
	}
	selectedGuild := guilds[selectedGuildIdx]

	contracts := selectedGuild.Contracts
	if session.selectedIndex < 0 {
		session.selectedIndex = 0
	}
	if len(contracts) == 0 {
		session.selectedIndex = 0
	}
	if len(contracts) > 0 && session.selectedIndex >= len(contracts) {
		session.selectedIndex = len(contracts) - 1
	}

	var content strings.Builder
	fmt.Fprintf(&content, "**Guild:** %s (`%s`)\n", selectedGuild.Name, selectedGuild.ID)

	if len(contracts) == 0 {
		content.WriteString("No contracts running for this guild.")
	} else {
		selected := contracts[session.selectedIndex]
		fmt.Fprintf(&content, "Showing oldest-first contract %d of %d\n\n", session.selectedIndex+1, len(contracts))

		coordinatorID := "unknown"
		if len(selected.CreatorID) > 0 && selected.CreatorID[0] != "" {
			coordinatorID = selected.CreatorID[0]
		}
		stateName := adminContractListStateName(selected.State)

		fieldName := truncateDiscordText(
			fmt.Sprintf("%d - **%s/%s**", session.selectedIndex+1, selected.ContractID, selected.CoopID),
			discordEmbedFieldNameLimit,
		)
		fmt.Fprintf(&content, "%s\n", fieldName)
		fmt.Fprintf(&content, "> Coordinator: <@%s>  [%s](%s/%s/%s)\n", coordinatorID, selected.CoopID, "https://eicoop-carpet.netlify.app", selected.ContractID, selected.CoopID)
		for _, loc := range selected.Location {
			if loc == nil {
				continue
			}
			fmt.Fprintf(&content, "> *%s*\t%s\n", loc.GuildName, loc.ChannelMention)
		}
		fmt.Fprintf(&content, "> Started: <t:%d:R>\n", selected.StartTime.Unix())
		fmt.Fprintf(&content, "> Contract State: *%s*\n", stateName)
		fmt.Fprintf(&content, "> Hash: *%s*", selected.ContractHash)
	}

	if session.statusMessage != "" {
		content.WriteString("\n\n")
		content.WriteString(session.statusMessage)
	}

	components := adminContractListComponents(session, guilds, len(contracts) > 0, deleting)
	return truncateDiscordText(content.String(), discordMessageContentLimit), components
}

func adminContractListComponents(session *adminContractListSession, guilds []adminContractListGuild, hasContract bool, deleting bool) []dc.LayoutComponent {
	options := make([]dc.SelectOption, 0, min(len(guilds), 25))
	for idx, guild := range guilds {
		if idx >= 25 {
			break
		}
		label := guild.Name
		if label == "" {
			label = guild.ID
		}
		description := fmt.Sprintf("%d contract(s)", len(guild.Contracts))
		options = append(options, dc.SelectOption{
			Label:       truncateDiscordText(label, 100),
			Value:       guild.ID,
			Description: truncateDiscordText(description, 100),
			Default:     guild.ID == session.selectedGuildID,
		})
	}

	finishLabel := "Finish (Arm)"
	finishStyle := dc.ButtonSecondary
	if session.finishArmed {
		finishLabel = "Confirm Finish"
		finishStyle = dc.ButtonDanger
	}
	if deleting {
		finishLabel = "Deleting..."
		finishStyle = dc.ButtonSecondary
	}

	navDisabled := !hasContract || deleting

	secondRowButtons := []dc.InteractiveComponent{dc.Button{
		Label:    "Close",
		Style:    dc.ButtonDanger,
		CustomID: fmt.Sprintf("%s#%s#close", adminContractListHandlerPrefix, session.id),
		Disabled: deleting,
	}}

	components := make([]dc.LayoutComponent, 0, 3)
	if session.allowGuildSelect {
		minValues := 1
		components = append(components, dc.ActionRow{Components: []dc.InteractiveComponent{
			dc.SelectMenu{
				CustomID:    fmt.Sprintf("%s#%s#guild-select", adminContractListHandlerPrefix, session.id),
				Placeholder: "Select guild",
				Options:     options,
				MinValues:   &minValues,
				MaxValues:   1,
			},
		}})
	}

	components = append(components,
		dc.ActionRow{Components: []dc.InteractiveComponent{
			dc.Button{Label: "First", Style: dc.ButtonSecondary, CustomID: fmt.Sprintf("%s#%s#first", adminContractListHandlerPrefix, session.id), Disabled: navDisabled},
			dc.Button{Label: "Previous", Style: dc.ButtonSecondary, CustomID: fmt.Sprintf("%s#%s#prev", adminContractListHandlerPrefix, session.id), Disabled: navDisabled},
			dc.Button{Label: finishLabel, Style: finishStyle, CustomID: fmt.Sprintf("%s#%s#finish", adminContractListHandlerPrefix, session.id), Disabled: navDisabled},
			dc.Button{Label: "Next", Style: dc.ButtonSecondary, CustomID: fmt.Sprintf("%s#%s#next", adminContractListHandlerPrefix, session.id), Disabled: navDisabled},
			dc.Button{Label: "Last", Style: dc.ButtonSecondary, CustomID: fmt.Sprintf("%s#%s#last", adminContractListHandlerPrefix, session.id), Disabled: navDisabled},
		}},
		dc.ActionRow{Components: secondRowButtons},
	)

	return components
}

func buildAdminContractListGuilds(defaultGuildID string, defaultGuildName string) []adminContractListGuild {
	guilds := []adminContractListGuild{}

	ContractsMutex.RLock()
	for _, contract := range Contracts {
		if contract != nil && contract.State != ContractStateArchive {
			for _, loc := range contract.Location {
				if loc == nil || loc.GuildID == "" {
					continue
				}
				var entry *adminContractListGuild
				for i := range guilds {
					if guilds[i].ID == loc.GuildID {
						entry = &guilds[i]
						break
					}
				}
				if entry == nil {
					name := loc.GuildName
					if name == "" {
						name = loc.GuildID
					}
					guilds = append(guilds, adminContractListGuild{ID: loc.GuildID, Name: name, Contracts: []*Contract{}})
					entry = &guilds[len(guilds)-1]
				}
				entry.Contracts = append(entry.Contracts, contract)
			}
		}
	}
	ContractsMutex.RUnlock()

	if defaultGuildID != "" {
		found := false
		for _, g := range guilds {
			if g.ID == defaultGuildID {
				found = true
				break
			}
		}
		if !found {
			name := strings.TrimSpace(defaultGuildName)
			if name == "" {
				name = defaultGuildID
			}
			guilds = append(guilds, adminContractListGuild{
				ID:        defaultGuildID,
				Name:      name,
				Contracts: []*Contract{},
			})
		}
	}

	sort.Slice(guilds, func(i, j int) bool {
		if guilds[i].Name == guilds[j].Name {
			return guilds[i].ID < guilds[j].ID
		}
		return guilds[i].Name < guilds[j].Name
	})

	for i := range guilds {
		sort.Slice(guilds[i].Contracts, func(a, b int) bool {
			c1 := guilds[i].Contracts[a]
			c2 := guilds[i].Contracts[b]
			if !c1.StartTime.Equal(c2.StartTime) {
				return c1.StartTime.Before(c2.StartTime)
			}
			if c1.ContractID != c2.ContractID {
				return c1.ContractID < c2.ContractID
			}
			return c1.CoopID < c2.CoopID
		})
	}

	return guilds
}

func adminContractListContractsForGuild(guilds []adminContractListGuild, guildID string) []*Contract {
	for _, guild := range guilds {
		if guild.ID == guildID {
			return guild.Contracts
		}
	}
	return []*Contract{}
}

func adminContractListGuildIndex(guilds []adminContractListGuild, guildID string) int {
	for idx, guild := range guilds {
		if guild.ID == guildID {
			return idx
		}
	}
	return -1
}

func adminContractListStateName(state int) string {
	if state >= 0 && state < len(contractStateNames) {
		return contractStateNames[state]
	}
	return "Unknown"
}

func truncateDiscordText(input string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(input) <= maxRunes {
		return input
	}
	runes := []rune(input)
	const ellipsis = "..."
	if maxRunes <= len(ellipsis) {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-len(ellipsis)]) + ellipsis
}
