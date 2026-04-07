package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
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

	if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      flags,
		Components: getCurrentContractsComponents(s, channelID),
	}); err != nil {
		log.Println("Error sending follow-up message:", err)
	}
}

// HandleActiveContractsPage handles button interactions for the active-contracts message.
func HandleActiveContractsPage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	flags := discordgo.MessageFlagsIsComponentsV2

	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 2 {
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
			return
		}
		channelID := parts[2]
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{Flags: flags},
		})

		components := getCurrentContractsComponents(s, channelID)
		edit := discordgo.WebhookEdit{Components: &components}
		if _, err := s.FollowupMessageEdit(i.Interaction, i.Message.ID, &edit); err != nil {
			log.Println("Error refreshing active contracts:", err)
		}

	case "bump":
		if len(parts) < 3 {
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

		if _, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
			Flags:      flags,
			Components: getCurrentContractsComponents(s, channelID),
		}); err != nil {
			log.Println("Error sending bumped active contracts:", err)
		}
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

func getCurrentContractsComponents(s *discordgo.Session, channelID string) []discordgo.MessageComponent {
	tl, err := s.ThreadsActive(channelID)
	if err != nil {
		log.Println("Error fetching active threads:", err)
		return []discordgo.MessageComponent{
			discordgo.Container{Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: "Error retrieving active contracts: " + err.Error()},
			}},
			activeContractsButtons(channelID),
		}
	}

	activeThreadIDs := make(map[string]bool, len(tl.Threads))
	for _, th := range tl.Threads {
		if th != nil {
			activeThreadIDs[th.ID] = true
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
	if len(matched) == 0 {
		contractComponents = []discordgo.MessageComponent{discordgo.TextDisplay{Content: "No active contracts found in this channel."}}
	} else {
		sort.Slice(matched, func(i, j int) bool {
			if matched[i].ContractID != matched[j].ContractID {
				return matched[i].ContractID < matched[j].ContractID
			}
			return matched[i].CoopID < matched[j].CoopID
		})

		for i := 0; i < len(matched); {
			j := i + 1
			for j < len(matched) && matched[j].ContractID == matched[i].ContractID {
				j++
			}
			contractComponents = append(contractComponents, getContractDisplay(matched[i:j], activeThreadIDs))
			i = j
		}
	}

	return []discordgo.MessageComponent{
		discordgo.Container{Components: contractComponents},
		activeContractsButtons(channelID),
	}
}

func getContractDisplay(coops []*Contract, activeThreadIDs map[string]bool) discordgo.TextDisplay {
	iconCoop := ei.GetBotEmojiMarkdown("icon_coop")

	var b strings.Builder
	header := coops[0]
	fmt.Fprintf(&b, "%s **%s** %s `%d`\n", header.EggEmoji, header.Name, iconCoop, header.CoopSize)

	completed := 0
	for _, c := range coops {
		if b.Len() > 3500 {
			break
		}
		if c.State == ContractStateCompleted {
			completed++
			continue
		}
		channelID := ""
		for _, loc := range c.Location {
			if loc != nil && activeThreadIDs[loc.ChannelID] {
				channelID = loc.ChannelID
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
		fmt.Fprintf(&b, "- [%s] **`%s%s`** <#%s>\n", count, colorEmoji, c.CoopID, channelID)
	}

	if completed > 0 {
		fmt.Fprintf(&b, "-# Completed contracts: %d\n", completed)
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
