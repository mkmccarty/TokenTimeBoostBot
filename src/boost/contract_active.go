package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

// This file is for /active-contracts and related functionality.

var threadStyleIcons = []string{"", "🟦", "🟩", "🟧", "🟥"}

// SlashAdminCurrentContracts creates the admin current-contracts command for Discord.
func SlashAdminCurrentContracts(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Display current boost contracts",
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Override the channel to search for active contracts (defaults to current channel)",
				Required:    false,
				ChannelTypes: []discordgo.ChannelType{
					discordgo.ChannelTypeGuildText,
					discordgo.ChannelTypeGuildForum,
					discordgo.ChannelTypeGuildNews,
				},
			},
		},
	}
}

// HandleAdminCurrentContracts handles the admin current-contracts command.
func HandleAdminCurrentContracts(s *discordgo.Session, i *discordgo.InteractionCreate) {
	flags := discordgo.MessageFlagsIsComponentsV2
	bottools.AcknowledgeResponse(s, i, flags)

	// Resolve the effective channel: explicit override > thread parent > interaction channel.
	channelID := i.ChannelID
	opts := bottools.GetCommandOptionsMap(i)
	if opt, ok := opts["channel"]; ok {
		channelID = opt.ChannelValue(s).ID
	} else if ch, err := s.Channel(channelID); err == nil && ch != nil {
		if isThreadChannelType(ch.Type) && ch.ParentID != "" {
			channelID = ch.ParentID
		}
	}

	components, _ := getCurrentContractsComponents(s, channelID)
	if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      flags,
		Components: components,
	}); err != nil {
		log.Println("Error sending follow-up message:", err)
	}
}

// HandleActiveContractsPage handles button interactions for the active-contracts message.
func HandleActiveContractsPage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	flags := discordgo.MessageFlagsIsComponentsV2
	respondUsage := func(msg string) {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: msg,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 2 {
		respondUsage("Invalid active-contracts action. Use the Refresh, Bump, or Close buttons on an active-contracts message.")
		return
	}

	switch parts[1] {
	case "close":
		var kept []discordgo.MessageComponent
		for _, c := range i.Message.Components {
			if _, ok := c.(*discordgo.ActionsRow); !ok {
				kept = append(kept, c)
			}
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Flags:      flags,
				Components: kept,
			},
		})

	case "refresh":
		if len(parts) < 3 {
			respondUsage("Invalid refresh action. Use the Refresh button from the active-contracts panel.")
			return
		}
		channelID := parts[2]
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{Flags: flags},
		})

		components, _ := getCurrentContractsComponents(s, channelID)
		edit := discordgo.WebhookEdit{Components: &components}
		if _, err := s.FollowupMessageEdit(i.Interaction, i.Message.ID, &edit); err != nil {
			log.Println("Error refreshing active contracts:", err)
		}

	case "bump":
		if len(parts) < 3 {
			respondUsage("Invalid bump action. Use the Bump button from the active-contracts panel.")
			return
		}
		channelID := parts[2]
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{Flags: flags},
		})

		if err := s.ChannelMessageDelete(i.ChannelID, i.Message.ID); err != nil {
			log.Println("Error deleting message for bump:", err)
		}

		components, _ := getCurrentContractsComponents(s, channelID)
		if _, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
			Flags:      flags,
			Components: components,
		}); err != nil {
			log.Println("Error sending bumped active contracts:", err)
		}

	default:
		respondUsage("Unknown active-contracts action. Use the Refresh, Bump, or Close buttons on the active-contracts panel.")
	}
}

func activeContractsButtons(channelID string) discordgo.ActionsRow {
	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Refresh",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("active-contracts#refresh#%s", channelID),
				Emoji:    &discordgo.ComponentEmoji{Name: "🔄"},
			},
			discordgo.Button{
				Label:    "Bump",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("active-contracts#bump#%s", channelID),
				Emoji:    &discordgo.ComponentEmoji{Name: "⤵"},
			},
			discordgo.Button{
				Label:    "Close",
				Style:    discordgo.DangerButton,
				CustomID: "active-contracts#close",
			},
		},
	}
}

func getCurrentContractsComponents(s *discordgo.Session, channelID string) ([]discordgo.MessageComponent, bool) {
	tl, err := s.ThreadsActive(channelID)
	if err != nil {
		log.Println("Error fetching active threads:", err)
		return []discordgo.MessageComponent{
			discordgo.Container{Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: "Error retrieving active contracts: " + err.Error()},
			}},
			activeContractsButtons(channelID),
		}, false
	}

	guildID := ""
	activeThreadIDs := make(map[string]bool, len(tl.Threads))
	for _, th := range tl.Threads {
		if th != nil {
			activeThreadIDs[th.ID] = true
			if guildID == "" {
				guildID = th.GuildID
			}
		}
	}

	var matched []*Contract
	for _, c := range Contracts {
		if c == nil {
			continue
		}
		for _, loc := range c.Location {
			if loc != nil && activeThreadIDs[loc.ChannelID] {
				matched = append(matched, c)
				break
			}
		}
	}

	var contractComponents []discordgo.MessageComponent
	shownContracts := 0
	if len(matched) == 0 {
		contractComponents = []discordgo.MessageComponent{discordgo.TextDisplay{Content: "No active contracts found in this channel."}}
	} else {
		sort.Slice(matched, func(i, j int) bool {
			if matched[i].ContractID != matched[j].ContractID {
				return matched[i].ContractID < matched[j].ContractID
			}
			return matched[i].CoopID < matched[j].CoopID
		})

		// Group by ContractID and display each contract with its active coops.
		totalChars := 0
		for i := 0; i < len(matched); {
			j := i + 1
			for j < len(matched) && matched[j].ContractID == matched[i].ContractID {
				j++
			}
			group := matched[i:j]
			var activeCoops []*Contract
			for _, c := range group {
				if c.State != ContractStateCompleted || guildstate.GetGuildSettingFlag(guildID, "active-contracts-show-completed") {
					activeCoops = append(activeCoops, c)
				}
			}
			// Get the contract display component for this contract and its active coops, if any.
			if len(activeCoops) > 0 {
				display := getContractDisplay(group[0], activeCoops, activeThreadIDs, discordEmbedTotalCharLimit-totalChars)
				if totalChars+len(display.Content) > discordEmbedTotalCharLimit {
					break
				}
				totalChars += len(display.Content)
				contractComponents = append(contractComponents, display)
				shownContracts++
			}
			i = j
		}
	}

	if len(contractComponents) == 0 {
		contractComponents = []discordgo.MessageComponent{discordgo.TextDisplay{Content: "No active contracts found in this channel."}}
	}

	accentColor := 0x5865f2
	return []discordgo.MessageComponent{
		discordgo.Container{
			Components:  contractComponents,
			AccentColor: &accentColor},
		activeContractsButtons(channelID),
	}, shownContracts > 0
}
func getContractDisplay(header *Contract, coops []*Contract, activeThreadIDs map[string]bool, charBudget int) discordgo.TextDisplay {
	iconCoop := ei.GetBotEmojiMarkdown("icon_coop")

	var b strings.Builder
	coopSizeStr := ""
	if header.EggName != "" {
		coopSizeStr = fmt.Sprintf(" %s `%d`", iconCoop, header.CoopSize)
	}
	fmt.Fprintf(&b, "## %s **%s**%s\n", header.EggEmoji, header.Name, coopSizeStr)

	for _, c := range coops {
		if b.Len() >= min(charBudget, discordEmbedDescLimit) {
			break
		}
		threadURL := ""
		for _, loc := range c.Location {
			if loc != nil && activeThreadIDs[loc.ChannelID] {
				threadURL = fmt.Sprintf("https://discord.com/channels/%s/%s", loc.GuildID, loc.ChannelID)
				break
			}
		}
		colorEmoji := ""
		if c.PlayStyle > 0 && c.PlayStyle < len(threadStyleIcons) {
			colorEmoji = threadStyleIcons[c.PlayStyle]
		}
		count := fmt.Sprintf("%d/%d", len(c.Boosters), c.CoopSize)
		if len(c.Boosters) >= c.CoopSize {
			count = "FULL"
		}
		fmt.Fprintf(&b, "%s%s `%s` [**⧉ %s**](%s) \n",
			strings.Repeat("_ _ ", 5),
			colorEmoji, count, c.CoopID, threadURL)
		if !c.PlannedStartTime.IsZero() {
			fmt.Fprintf(&b, "-# %s↳Start: %s\n",
				strings.Repeat("_ _ ", 7),
				bottools.WrapTimestamp(c.PlannedStartTime.Unix(), bottools.TimestampLongDateTime))
		}
	}

	return discordgo.TextDisplay{Content: b.String()}
}

func isThreadChannelType(t discordgo.ChannelType) bool {
	switch t {
	case discordgo.ChannelTypeGuildPublicThread,
		discordgo.ChannelTypeGuildPrivateThread,
		discordgo.ChannelTypeGuildNewsThread:
		return true
	default:
		return false
	}
}
