package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

// This file is for /active-contracts and related functionality.

var threadStyleIcons = []string{"", "🟦", "🟩", "🟧", "🟥"}

// SlashAdminCurrentContracts creates the admin current-contracts command for Discord.
func SlashAdminCurrentContracts(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Display current boost contracts")
	command.Options = []dc.Option{
		dc.ChannelOption{
			Name:         "channel",
			Description:  "Override the channel to search for active contracts (defaults to current channel)",
			ChannelTypes: []dc.ChannelType{dc.ChannelText, dc.ChannelForum, dc.ChannelNews},
		},
	}
	return &command
}

// HandleAdminCurrentContracts handles the admin current-contracts command.
func HandleAdminCurrentContracts(client dc.Client, e *dc.CommandEvent) {
	_ = e.Defer(false)

	// Resolve the effective channel: explicit override > thread parent > interaction channel.
	channelID := e.ChannelID()
	if opt, ok := e.OptChannel("channel"); ok {
		channelID = opt
	} else if ch, err := client.Channel(channelID); err == nil && ch != nil {
		if ch.IsThread && ch.ParentID != "" {
			channelID = ch.ParentID
		}
	}

	components, _ := getCurrentContractsComponents(client, channelID)
	if err := e.Followup(dc.Message{Components: components}); err != nil {
		log.Println("Error sending follow-up message:", err)
	}
}

// HandleActiveContractsPage handles button interactions for the active-contracts message.
func HandleActiveContractsPage(client dc.Client, e *dc.ComponentEvent) {
	respondUsage := func(msg string) {
		_ = e.Respond(dc.Message{
			Content:   msg,
			Ephemeral: true,
		})
	}

	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		respondUsage("Invalid active-contracts action. Use the Refresh, Bump, or Close buttons on an active-contracts message.")
		return
	}

	switch parts[1] {
	case "close":
		_ = e.Update(dc.Message{Components: e.MessageComponentsWithoutActionRows()})

	case "refresh":
		if len(parts) < 3 {
			respondUsage("Invalid refresh action. Use the Refresh button from the active-contracts panel.")
			return
		}
		channelID := parts[2]
		_ = e.DeferUpdate()

		components, _ := getCurrentContractsComponents(client, channelID)
		if err := e.EditFollowup(e.MessageID(), dc.Message{Components: components}); err != nil {
			log.Println("Error refreshing active contracts:", err)
		}

	case "bump":
		if len(parts) < 3 {
			respondUsage("Invalid bump action. Use the Bump button from the active-contracts panel.")
			return
		}
		channelID := parts[2]
		_ = e.DeferUpdate()

		if err := client.DeleteMessage(e.ChannelID(), e.MessageID()); err != nil {
			log.Println("Error deleting message for bump:", err)
		}

		components, _ := getCurrentContractsComponents(client, channelID)
		if _, err := client.SendMessage(e.ChannelID(), dc.Message{Components: components}); err != nil {
			log.Println("Error sending bumped active contracts:", err)
		}

	default:
		respondUsage("Unknown active-contracts action. Use the Refresh, Bump, or Close buttons on the active-contracts panel.")
	}
}

func activeContractsButtons(channelID string) dc.ActionRow {
	return dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "Refresh",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("active-contracts#refresh#%s", channelID),
				Emoji:    &dc.Emoji{Name: "🔄"},
			},
			dc.Button{
				Label:    "Bump",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("active-contracts#bump#%s", channelID),
				Emoji:    &dc.Emoji{Name: "⤵"},
			},
			dc.Button{
				Label:    "Close",
				Style:    dc.ButtonDanger,
				CustomID: "active-contracts#close",
			},
		},
	}
}

func getCurrentContractsComponents(client dc.Client, channelID string) ([]dc.LayoutComponent, bool) {
	threads, err := client.ActiveThreads(channelID)
	if err != nil {
		log.Println("Error fetching active threads:", err)
		return []dc.LayoutComponent{
			dc.Container{Components: []dc.ContainerSubComponent{
				dc.TextDisplay{Content: "Error retrieving active contracts: " + err.Error()},
			}},
			activeContractsButtons(channelID),
		}, false
	}

	guildID := ""
	activeThreadIDs := make(map[string]bool, len(threads))
	for _, th := range threads {
		activeThreadIDs[th.ID] = true
		if guildID == "" {
			guildID = th.GuildID
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

	var contractComponents []dc.ContainerSubComponent
	shownContracts := 0
	if len(matched) == 0 {
		contractComponents = []dc.ContainerSubComponent{dc.TextDisplay{Content: "No active contracts found in this channel."}}
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
				sort.Slice(activeCoops, func(a, b int) bool {
					fullA := len(activeCoops[a].Boosters) >= activeCoops[a].CoopSize
					fullB := len(activeCoops[b].Boosters) >= activeCoops[b].CoopSize
					if fullA != fullB {
						return fullA // FULL coops first
					}
					return activeCoops[a].CoopID < activeCoops[b].CoopID
				})
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
		contractComponents = []dc.ContainerSubComponent{dc.TextDisplay{Content: "No active contracts found in this channel."}}
	}

	return []dc.LayoutComponent{
		dc.Container{
			Components:  contractComponents,
			AccentColor: 0x5865f2},
		activeContractsButtons(channelID),
	}, shownContracts > 0
}
func getContractDisplay(header *Contract, coops []*Contract, activeThreadIDs map[string]bool, charBudget int) dc.TextDisplay {
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

	return dc.TextDisplay{Content: b.String()}
}
