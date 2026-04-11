package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
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

// HandleAdminContractList will list all contracts.
func HandleAdminContractList(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Gathering contract list...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Println(err)
		return
	}

	ArchiveContracts(s)

	cleanupAdminContractListSessions()
	selectedGuildName := i.GuildID
	if guild, guildErr := s.Guild(i.GuildID); guildErr == nil && guild != nil && strings.TrimSpace(guild.Name) != "" {
		selectedGuildName = strings.TrimSpace(guild.Name)
	}
	homeGuildID := guildstate.GetGuildSettingString("DEFAULT", "home_guild")
	allowGuildSelect := homeGuildID != "" && i.GuildID == homeGuildID
	session := &adminContractListSession{
		id:                fmt.Sprintf("%d", time.Now().UnixNano()),
		userID:            userID,
		selectedGuildID:   i.GuildID,
		selectedGuildName: selectedGuildName,
		allowGuildSelect:  allowGuildSelect,
		selectedIndex:     0,
		finishArmed:       false,
		statusMessage:     "",
		expiresAt:         time.Now().Add(adminContractListSessionTTL),
	}
	adminContractListSessions[session.id] = session

	content, components := renderAdminContractListPanel(session)
	if _, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content:    content,
		Components: components,
		Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsSuppressEmbeds,
	}); err != nil {
		log.Println(err)
		delete(adminContractListSessions, session.id)
	}
}

// HandleAdminContractListComponent handles all button/select interactions for admin-contract-list.
func HandleAdminContractListComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 3 {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Invalid contract list action.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	cleanupAdminContractListSessions()
	sessionID := parts[1]
	action := parts[2]
	session, ok := adminContractListSessions[sessionID]
	if !ok {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This contract list panel has expired. Please run the command again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	userID := getInteractionUserID(i)
	if session.userID != userID {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Only the command caller can use this panel.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err != nil {
		log.Println(err)
	}
	if perms&discordgo.PermissionAdministrator == 0 {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are not authorized to use this command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
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
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Contract list closed.",
				Flags:      discordgo.MessageFlagsSuppressEmbeds,
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	case "guild-select":
		if !session.allowGuildSelect {
			break
		}
		values := i.MessageComponentData().Values
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
		err = finishContractByHash(s, selected.ContractHash)
		if err != nil {
			session.statusMessage = "Unable to finish contract: " + err.Error()
			session.finishArmed = false
			break
		}
		session.statusMessage = fmt.Sprintf("Finished contract **%s/%s**.", selected.ContractID, selected.CoopID)
		session.finishArmed = false
		session.selectedIndex = 0
		ArchiveContracts(s)
	default:
		session.statusMessage = "Unknown contract list action."
	}

	if navigationAction {
		session.finishArmed = false
	}

	content, components := renderAdminContractListPanel(session)
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    content,
			Flags:      discordgo.MessageFlagsSuppressEmbeds,
			Components: components,
		},
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

func renderAdminContractListPanel(session *adminContractListSession) (string, []discordgo.MessageComponent) {
	guilds := buildAdminContractListGuilds(session.selectedGuildID, session.selectedGuildName)

	if len(guilds) == 0 {
		content := "No contracts are currently tracked."
		if session.statusMessage != "" {
			content += "\n\n" + session.statusMessage
		}
		return content, []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{Label: "Close", Style: discordgo.DangerButton, CustomID: fmt.Sprintf("%s#%s#close", adminContractListHandlerPrefix, session.id)},
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

	components := adminContractListComponents(session, guilds, len(contracts) > 0)
	return truncateDiscordText(content.String(), discordMessageContentLimit), components
}

func adminContractListComponents(session *adminContractListSession, guilds []adminContractListGuild, hasContract bool) []discordgo.MessageComponent {
	options := make([]discordgo.SelectMenuOption, 0, min(len(guilds), 25))
	for idx, guild := range guilds {
		if idx >= 25 {
			break
		}
		label := guild.Name
		if label == "" {
			label = guild.ID
		}
		description := fmt.Sprintf("%d contract(s)", len(guild.Contracts))
		options = append(options, discordgo.SelectMenuOption{
			Label:       truncateDiscordText(label, 100),
			Value:       guild.ID,
			Description: truncateDiscordText(description, 100),
			Default:     guild.ID == session.selectedGuildID,
		})
	}

	finishLabel := "Finish (Arm)"
	finishStyle := discordgo.SecondaryButton
	if session.finishArmed {
		finishLabel = "Confirm Finish"
		finishStyle = discordgo.DangerButton
	}

	secondRowButtons := []discordgo.MessageComponent{discordgo.Button{
		Label:    "Close",
		Style:    discordgo.DangerButton,
		CustomID: fmt.Sprintf("%s#%s#close", adminContractListHandlerPrefix, session.id),
	}}

	components := make([]discordgo.MessageComponent, 0, 3)
	if session.allowGuildSelect {
		components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    fmt.Sprintf("%s#%s#guild-select", adminContractListHandlerPrefix, session.id),
				Placeholder: "Select guild",
				Options:     options,
				MinValues:   &[]int{1}[0],
				MaxValues:   1,
			},
		}})
	}

	components = append(components,
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: "First", Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("%s#%s#first", adminContractListHandlerPrefix, session.id), Disabled: !hasContract},
			discordgo.Button{Label: "Previous", Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("%s#%s#prev", adminContractListHandlerPrefix, session.id), Disabled: !hasContract},
			discordgo.Button{Label: finishLabel, Style: finishStyle, CustomID: fmt.Sprintf("%s#%s#finish", adminContractListHandlerPrefix, session.id), Disabled: !hasContract},
			discordgo.Button{Label: "Next", Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("%s#%s#next", adminContractListHandlerPrefix, session.id), Disabled: !hasContract},
			discordgo.Button{Label: "Last", Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("%s#%s#last", adminContractListHandlerPrefix, session.id), Disabled: !hasContract},
		}},
		discordgo.ActionsRow{Components: secondRowButtons},
	)

	return components
}

func buildAdminContractListGuilds(defaultGuildID string, defaultGuildName string) []adminContractListGuild {
	guildMap := make(map[string]*adminContractListGuild)

	for _, contract := range Contracts {
		if contract == nil {
			continue
		}
		for _, loc := range contract.Location {
			if loc == nil || loc.GuildID == "" {
				continue
			}
			entry, exists := guildMap[loc.GuildID]
			if !exists {
				name := loc.GuildName
				if name == "" {
					name = loc.GuildID
				}
				entry = &adminContractListGuild{ID: loc.GuildID, Name: name, Contracts: []*Contract{}}
				guildMap[loc.GuildID] = entry
			}
			entry.Contracts = append(entry.Contracts, contract)
			break
		}
	}

	if defaultGuildID != "" {
		if _, ok := guildMap[defaultGuildID]; !ok {
			name := strings.TrimSpace(defaultGuildName)
			if name == "" {
				name = defaultGuildID
			}
			guildMap[defaultGuildID] = &adminContractListGuild{
				ID:        defaultGuildID,
				Name:      name,
				Contracts: []*Contract{},
			}
		}
	}

	guilds := make([]adminContractListGuild, 0, len(guildMap))
	for _, guild := range guildMap {
		guild.Contracts = adminContractListContractsForGuildRaw(guild.ID)
		guilds = append(guilds, *guild)
	}

	sort.Slice(guilds, func(i, j int) bool {
		if guilds[i].Name == guilds[j].Name {
			return guilds[i].ID < guilds[j].ID
		}
		return guilds[i].Name < guilds[j].Name
	})

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

func adminContractListContractsForGuildRaw(guildID string) []*Contract {
	contracts := make([]*Contract, 0)
	for _, contract := range Contracts {
		if contract == nil {
			continue
		}
		for _, loc := range contract.Location {
			if loc != nil && loc.GuildID == guildID {
				contracts = append(contracts, contract)
				break
			}
		}
	}

	sort.Slice(contracts, func(i, j int) bool {
		left := contracts[i]
		right := contracts[j]
		if !left.StartTime.Equal(right.StartTime) {
			return left.StartTime.Before(right.StartTime)
		}
		if left.ContractID != right.ContractID {
			return left.ContractID < right.ContractID
		}
		return left.CoopID < right.CoopID
	})

	return contracts
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
