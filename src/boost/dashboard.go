package boost

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetSlashDashboardCommand returns the /dashboard slash command definition
func GetSlashDashboardCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Manage your personal BoostBot dashboard",
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
				Name:        "show",
				Description: "Show your personal BoostBot dashboard (active contracts, timers)",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "add-bookmark",
				Description: "Add the current channel to your dashboard bookmarks",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "remove-bookmark",
				Description: "Remove the current channel from your dashboard bookmarks",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
		},
	}
}

// HandleDashboardCommand handles the /dashboard command
func HandleDashboardCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := getInteractionUserID(i)

	subcommand := "show"
	if len(i.ApplicationCommandData().Options) > 0 {
		subcommand = i.ApplicationCommandData().Options[0].Name
	}

	switch subcommand {
	case "show":
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})

		content, components := drawDashboard(userID)

		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content:    content,
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
		})

	case "add-bookmark":
		channelName := "Unknown Channel"
		guildName := "Unknown Server"
		guildID := ""
		if ch, err := s.Channel(i.ChannelID); err == nil {
			channelName = ch.Name
			guildID = ch.GuildID
			if g, err := s.Guild(ch.GuildID); err == nil {
				guildName = g.Name
			}
		}
		addDashboardBookmark(userID, i.ChannelID, guildID, guildName, channelName)
		bms := getDashboardBookmarks(userID)

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Bookmark added for <#%s>. You have %d/15 bookmarks.", i.ChannelID, len(bms)),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

	case "remove-bookmark":
		bms := getDashboardBookmarks(userID)
		found := false
		for _, bm := range bms {
			if bm.ChannelID == i.ChannelID {
				found = true
				break
			}
		}

		var msg string
		if found {
			delDashboardBookmark(userID, i.ChannelID)
			bms = getDashboardBookmarks(userID)
			msg = fmt.Sprintf("Bookmark removed for <#%s>. You have %d/15 bookmarks.", i.ChannelID, len(bms))
		} else {
			msg = fmt.Sprintf("No bookmark found for <#%s>. You have %d/15 bookmarks.", i.ChannelID, len(bms))
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: msg,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}
}

func drawDashboard(userID string) (string, []discordgo.MessageComponent) {
	var components []discordgo.MessageComponent
	components = append(components, discordgo.TextDisplay{Content: "# 📊 Your BoostBot Dashboard"})

	colorContracts := 0xAAAAAA // Blurple
	colorTimers := 0x999999    // Yellow
	colorBookmarks := 0x777777 // Fuchsia
	colorCommands := 0x555555  // Green

	// Active Contracts
	var activeContracts []*Contract
	for _, c := range Contracts {
		if userInContract(c, userID) {
			if !c.EstimatedEndTime.IsZero() && time.Since(c.EstimatedEndTime) > 24*time.Hour {
				continue
			}
			activeContracts = append(activeContracts, c)
		}
	}

	sort.Slice(activeContracts, func(i, j int) bool {
		a := activeContracts[i]
		b := activeContracts[j]
		now := time.Now()

		group := func(c *Contract) int {
			if !c.EstimatedEndTime.IsZero() && now.After(c.EstimatedEndTime) {
				if now.Sub(c.EstimatedEndTime) <= 12*time.Hour {
					return 0 // Completed recently
				}
				return 3 // Completed between 12-24h ago
			}
			if c.State == ContractStateSignup {
				return 2 // Not started
			}
			return 1 // Running
		}

		gA := group(a)
		gB := group(b)
		if gA != gB {
			return gA < gB
		}

		switch gA {
		case 0, 3:
			return a.EstimatedEndTime.After(b.EstimatedEndTime)
		case 1:
			if a.EstimatedEndTime.IsZero() && b.EstimatedEndTime.IsZero() {
				return a.StartTime.Before(b.StartTime)
			}
			if a.EstimatedEndTime.IsZero() {
				return false
			}
			if b.EstimatedEndTime.IsZero() {
				return true
			}
			return a.EstimatedEndTime.Before(b.EstimatedEndTime)
		case 2:
			return a.ValidFrom.Before(b.ValidFrom)
		}
		return false
	})

	contractCount := len(activeContracts)
	var contractBuilder strings.Builder
	for _, c := range activeContracts {
		channelStr := "Unknown Channel"
		if len(c.Location) > 0 {
			guildID := c.Location[0].GuildID
			if guildID == "" {
				guildID = "@me"
			}
			channelStr = fmt.Sprintf("https://discord.com/channels/%s/%s", guildID, c.Location[0].ChannelID)
		}

		var timeStr string
		if !c.EstimatedEndTime.IsZero() {
			timeStr = fmt.Sprintf("Completion: <t:%d:f>", c.EstimatedEndTime.Unix())
		} else if c.State == ContractStateSignup {
			timeStr = "In Sign-up"
		} else {
			timeStr = "Completion: TBD"
		}

		fmt.Fprintf(&contractBuilder, "[**%s / %s**](%s)\n", c.ContractID, c.CoopID, channelStr)
		fmt.Fprintf(&contractBuilder, "-# _       _ %s\n", timeStr)
	}
	if contractCount > 0 {
		components = append(components, discordgo.Container{
			AccentColor: &colorContracts,
			Components:  []discordgo.MessageComponent{discordgo.TextDisplay{Content: "## 🚀 Active Contracts\n" + contractBuilder.String()}},
		})
	}

	// Active Timers
	timerCount := 0
	var timerBuilder strings.Builder
	timersMutex.Lock()
	now := time.Now()
	for _, t := range timers {
		if t.UserID == userID && now.Before(t.Reminder) {
			timerCount++
			displayMessage := t.Message
			if t.OriginalChannelID != "" {
				displayMessage = fmt.Sprintf("%s in <#%s>", displayMessage, t.OriginalChannelID)
			}
			fmt.Fprintf(&timerBuilder, "⏱️ **%s**\n", displayMessage)
			fmt.Fprintf(&timerBuilder, "-# _       _ Reminder: <t:%d:R>\n", t.Reminder.Unix())
		}
	}
	timersMutex.Unlock()

	if timerCount > 0 {
		components = append(components, discordgo.Container{
			AccentColor: &colorTimers,
			Components:  []discordgo.MessageComponent{discordgo.TextDisplay{Content: "## ⏱️ Active Timers\n" + timerBuilder.String()}},
		})
	}

	// Bookmarks
	bms := getDashboardBookmarks(userID)
	if len(bms) > 0 {
		var bmBuilder strings.Builder
		bmBuilder.WriteString("## 🔖 Channel Bookmarks\n")
		for _, bm := range bms {
			if bm.GuildID != "" && bm.ChannelName != "" {
				fmt.Fprintf(&bmBuilder, "[#%s](https://discord.com/channels/%s/%s)\n", bm.ChannelName, bm.GuildID, bm.ChannelID)
			} else {
				fmt.Fprintf(&bmBuilder, "<#%s>\n", bm.ChannelID)
			}
		}
		components = append(components, discordgo.Container{
			AccentColor: &colorBookmarks,
			Components:  []discordgo.MessageComponent{discordgo.TextDisplay{Content: bmBuilder.String()}},
		})
	}

	// Command Links
	var cmdBuilder strings.Builder
	cmdBuilder.WriteString("## 🔗 Useful Commands\n")
	stonesCmd := bottools.GetFormattedCommand("stones")
	if stonesCmd == "" {
		stonesCmd = "`/stones`"
	}
	csEstimateCmd := bottools.GetFormattedCommand("cs-estimate")
	if csEstimateCmd == "" {
		csEstimateCmd = "`/cs-estimate`"
	}
	timerCmd := bottools.GetFormattedCommand("timer")
	if timerCmd == "" {
		timerCmd = "`/timer`"
	}

	fmt.Fprintf(&cmdBuilder, "-# %s 🪨 • %s 📈 • %s ⏱️", stonesCmd, csEstimateCmd, timerCmd)
	components = append(components, discordgo.Container{
		AccentColor: &colorCommands,
		Components:  []discordgo.MessageComponent{discordgo.TextDisplay{Content: cmdBuilder.String()}},
	})

	components = append(components, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Refresh",
				Style:    discordgo.SecondaryButton,
				CustomID: "dashboard_btn#refresh",
				Emoji:    &discordgo.ComponentEmoji{Name: "🔄"},
			},
			discordgo.Button{
				Label:    "Add Bookmark",
				Style:    discordgo.PrimaryButton,
				CustomID: "dashboard_btn#add_bookmark",
				Emoji:    &discordgo.ComponentEmoji{Name: "🔖"},
			},
			discordgo.Button{
				Label:    "Delete Bookmark",
				Style:    discordgo.DangerButton,
				CustomID: "dashboard_btn#del_bookmark",
				Disabled: len(bms) == 0,
			},
		},
	})

	return "", components
}

// DashboardBookmark represents a bookmark for a specific channel in the dashboard
type DashboardBookmark struct {
	ChannelID   string    `json:"channel_id"`
	GuildID     string    `json:"guild_id,omitempty"`
	GuildName   string    `json:"guild_name,omitempty"`
	ChannelName string    `json:"channel_name,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

func getDashboardBookmarks(userID string) []DashboardBookmark {
	str := farmerstate.GetMiscSettingString(userID, "dashboard_bookmarks")
	var bms []DashboardBookmark
	if str != "" {
		_ = json.Unmarshal([]byte(str), &bms)
	}
	sort.Slice(bms, func(i, j int) bool {
		return bms[i].Timestamp.Before(bms[j].Timestamp)
	})
	return bms
}

func saveDashboardBookmarks(userID string, bms []DashboardBookmark) {
	b, _ := json.Marshal(bms)
	farmerstate.SetMiscSettingString(userID, "dashboard_bookmarks", string(b))
}

func addDashboardBookmark(userID string, channelID string, guildID string, guildName string, channelName string) {
	bms := getDashboardBookmarks(userID)
	found := false
	for i := range bms {
		if bms[i].ChannelID == channelID {
			bms[i].Timestamp = time.Now()
			bms[i].GuildID = guildID
			bms[i].GuildName = guildName
			bms[i].ChannelName = channelName
			found = true
			break
		}
	}
	if !found {
		bms = append(bms, DashboardBookmark{
			ChannelID:   channelID,
			GuildID:     guildID,
			GuildName:   guildName,
			ChannelName: channelName,
			Timestamp:   time.Now(),
		})
	}
	sort.Slice(bms, func(i, j int) bool {
		return bms[i].Timestamp.Before(bms[j].Timestamp)
	})
	if len(bms) > 15 {
		bms = bms[len(bms)-15:]
	}
	saveDashboardBookmarks(userID, bms)
}

func delDashboardBookmark(userID string, channelID string) {
	bms := getDashboardBookmarks(userID)
	var newBms []DashboardBookmark
	for _, bm := range bms {
		if bm.ChannelID != channelID {
			newBms = append(newBms, bm)
		}
	}
	saveDashboardBookmarks(userID, newBms)
}

// HandleDashboardInteraction handles interactions on the dashboard like refreshing and bookmarks
func HandleDashboardInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 2 {
		return
	}
	action := parts[1]
	userID := getInteractionUserID(i)

	flags := discordgo.MessageFlags(0)
	if i.Message != nil && i.Message.Flags&discordgo.MessageFlagsEphemeral != 0 {
		flags |= discordgo.MessageFlagsEphemeral
	}
	flags |= discordgo.MessageFlagsIsComponentsV2

	switch action {
	case "add_bookmark":
		channelName := "Unknown Channel"
		guildName := "Unknown Server"
		guildID := ""
		if ch, err := s.Channel(i.ChannelID); err == nil {
			channelName = ch.Name
			guildID = ch.GuildID
			if g, err := s.Guild(ch.GuildID); err == nil {
				guildName = g.Name
			}
		}
		addDashboardBookmark(userID, i.ChannelID, guildID, guildName, channelName)
		content, components := drawDashboard(userID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    content,
				Components: components,
				Flags:      flags,
			},
		})

	case "del_bookmark":
		bms := getDashboardBookmarks(userID)
		if len(bms) == 0 {
			return
		}

		var bmBuilder strings.Builder
		bmBuilder.WriteString("## 🗑️ Delete a Bookmark\n")

		options := make([]discordgo.SelectMenuOption, 0, len(bms))
		for idx, bm := range bms {
			if bm.GuildID != "" && bm.ChannelName != "" {
				fmt.Fprintf(&bmBuilder, "%d. [#%s](https://discord.com/channels/@me/%s/%s)\n", idx+1, bm.ChannelName, bm.GuildID, bm.ChannelID)
			} else {
				fmt.Fprintf(&bmBuilder, "%d. <#%s>\n", idx+1, bm.ChannelID)
			}
			options = append(options, discordgo.SelectMenuOption{
				Label: fmt.Sprintf("%d", idx+1),
				Value: bm.ChannelID,
			})
		}

		minValues := 1
		accentColor := 0xed4245 // Danger red
		components := []discordgo.MessageComponent{
			discordgo.Container{
				AccentColor: &accentColor,
				Components: []discordgo.MessageComponent{
					discordgo.TextDisplay{Content: bmBuilder.String()},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "dashboard_btn#del_select",
						Placeholder: "Select bookmark number to delete",
						Options:     options,
						MinValues:   &minValues,
						MaxValues:   1,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Cancel",
						Style:    discordgo.SecondaryButton,
						CustomID: "dashboard_btn#refresh",
					},
				},
			},
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Components: components,
				Flags:      flags,
			},
		})

	case "del_select":
		vals := i.MessageComponentData().Values
		if len(vals) > 0 {
			delDashboardBookmark(userID, vals[0])
		}
		content, components := drawDashboard(userID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    content,
				Components: components,
				Flags:      flags,
			},
		})

	case "refresh":
		content, components := drawDashboard(userID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    content,
				Components: components,
				Flags:      flags,
			},
		})
	}
}
