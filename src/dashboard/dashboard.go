package dashboard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
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
	userID := bottools.GetInteractionUserID(i)

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

		content, components := drawDashboard(s, userID)

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

func drawDashboard(s *discordgo.Session, userID string) (string, []discordgo.MessageComponent) {
	var components []discordgo.MessageComponent
	components = append(components, discordgo.TextDisplay{Content: "# 📊 Your BoostBot Dashboard"})

	colorContracts := 0xAAAAAA // Blurple
	colorTimers := 0x999999    // Yellow
	colorBookmarks := 0x777777 // Fuchsia
	colorCommands := 0x555555  // Green

	// Active Contracts
	var activeContracts []*boost.Contract
	for _, c := range boost.Contracts {
		if boost.UserInContract(c, userID) {
			if !c.EstimatedEndTime.IsZero() && time.Since(c.EstimatedEndTime) > 24*time.Hour {
				continue
			}
			activeContracts = append(activeContracts, c)
		}
	}

	// External Contracts
	eeid := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	extBookmarks := getExternalContractBookmarks(userID)
	extBookmarkMap := make(map[string]boost.ExternalContractBookmark)
	for _, bm := range extBookmarks {
		key := fmt.Sprintf("%s:%s", bm.ContractID, strings.ToLower(bm.CoopID))
		extBookmarkMap[key] = bm
	}
	seenBookmarks := make(map[string]bool)

	if eeid != "" {
		backup, _ := ei.GetFirstContactFromAPI(s, eeid, userID, true)
		if backup != nil {
			for _, farm := range backup.GetFarms() {
				if farm.GetFarmType() == ei.FarmType_CONTRACT {
					contractID := farm.GetContractId()
					if contractID == "" || contractID == "first-contract" {
						continue
					}

					coopID := ""
					if backup.GetContracts() != nil {
						for _, lc := range backup.GetContracts().GetCurrentCoopStatuses() {
							if lc.GetContractIdentifier() != "" && lc.GetContractIdentifier() == contractID {
								if lc.GetClearedForExit() {
									continue
								}
								coopID = lc.GetCoopIdentifier()
								break
							}
						}
					}

					if contractID != "" && coopID != "" {
						bookmarkKey := fmt.Sprintf("%s:%s", contractID, strings.ToLower(coopID))
						found := false
						for _, c := range activeContracts {
							if c.ContractID == contractID {
								found = true
								break
							}
						}

						if !found {
							startTime, durationSeconds, err := ei.GetCoopStatusStartTimeAndDuration(contractID, coopID, eeid)
							if err == nil {
								estEndTime := startTime.Add(time.Duration(durationSeconds) * time.Second)
								if estEndTime.IsZero() || time.Since(estEndTime) <= 24*time.Hour {
									dummy := &boost.Contract{
										ContractID:       contractID,
										CoopID:           coopID,
										StartTime:        startTime,
										EstimatedEndTime: estEndTime,
										State:            99, // Indicate active but not in signup
									}
									if bm, ok := extBookmarkMap[bookmarkKey]; ok {
										dummy.Location = []*boost.LocationData{
											{
												GuildID:   bm.GuildID,
												ChannelID: bm.ChannelID,
											},
										}
										seenBookmarks[bookmarkKey] = true
									}
									activeContracts = append(activeContracts, dummy)
								}
							}
						}
					}
				}
			}
		}
	}

	var newExtBookmarks []boost.ExternalContractBookmark
	for _, bm := range extBookmarks {
		key := fmt.Sprintf("%s:%s", bm.ContractID, strings.ToLower(bm.CoopID))
		if seenBookmarks[key] {
			newExtBookmarks = append(newExtBookmarks, bm)
		}
	}
	if len(newExtBookmarks) != len(extBookmarks) {
		saveExternalContractBookmarks(userID, newExtBookmarks)
	}

	sort.Slice(activeContracts, func(i, j int) bool {
		a := activeContracts[i]
		b := activeContracts[j]
		now := time.Now()

		group := func(c *boost.Contract) int {
			if !c.EstimatedEndTime.IsZero() && now.After(c.EstimatedEndTime) {
				if now.Sub(c.EstimatedEndTime) <= 12*time.Hour {
					return 0 // Completed recently
				}
				return 3 // Completed between 12-24h ago
			}
			if c.State == boost.ContractStateSignup {
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
	if contractCount > 0 {
		var contractBuilder strings.Builder
		var bookmarkButtons []discordgo.MessageComponent

		for _, c := range activeContracts {
			channelStr := "Unknown Channel"
			if len(c.Location) > 0 {
				guildID := c.Location[0].GuildID
				if guildID == "" {
					guildID = "@me"
				}
				channelStr = fmt.Sprintf("https://discord.com/channels/%s/%s", guildID, c.Location[0].ChannelID)
			} else if c.State == 99 {
				channelStr = "External Contract"
			}

			var timeStr string
			if !c.EstimatedEndTime.IsZero() {
				timeStr = fmt.Sprintf("Completion: <t:%d:f>", c.EstimatedEndTime.Unix())
			} else if c.State == boost.ContractStateSignup {
				timeStr = "In Sign-up"
			} else {
				timeStr = "Completion: TBD"
			}

			fmt.Fprintf(&contractBuilder, "**%s / %s**\n%s\n", c.ContractID, c.CoopID, channelStr)
			fmt.Fprintf(&contractBuilder, "-# _       _ %s\n", timeStr)

			if c.State == 99 && len(c.Location) == 0 { // It's an un-bookmarked external contract
				label := "Bookmark " + c.ContractID
				if len(label) > 80 {
					label = label[:80]
				}
				bookmarkButtons = append(bookmarkButtons, discordgo.Button{
					Label:    label,
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("dashboard_btn#add_ext_bm#%s#%s", c.ContractID, c.CoopID),
					Emoji:    &discordgo.ComponentEmoji{Name: "🔖"},
				})
			}
		}
		components = append(components, discordgo.Container{
			AccentColor: &colorContracts,
			Components:  []discordgo.MessageComponent{discordgo.TextDisplay{Content: "## 🚀 Active Contracts\n" + contractBuilder.String()}},
		})

		// Limit buttons per action row to 5 (Discord's maximum)
		for i := 0; i < len(bookmarkButtons); i += 5 {
			end := i + 5
			if end > len(bookmarkButtons) {
				end = len(bookmarkButtons)
			}
			components = append(components, discordgo.ActionsRow{
				Components: bookmarkButtons[i:end],
			})
		}
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
	extBms := getExternalContractBookmarks(userID)
	if len(bms) > 0 || len(extBms) > 0 {
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

	var bottomButtons []discordgo.MessageComponent
	bottomButtons = append(bottomButtons, discordgo.Button{
		Label:    "Refresh",
		Style:    discordgo.SecondaryButton,
		CustomID: "dashboard_btn#refresh",
		Emoji:    &discordgo.ComponentEmoji{Name: "🔄"},
	})
	bottomButtons = append(bottomButtons, discordgo.Button{
		Label:    "Bookmark Channel",
		Style:    discordgo.PrimaryButton,
		CustomID: "dashboard_btn#add_bookmark",
		Emoji:    &discordgo.ComponentEmoji{Name: "🔖"},
	})
	if len(bms) > 0 || len(extBms) > 0 {
		bottomButtons = append(bottomButtons, discordgo.Button{
			Label:    "Delete Bookmark",
			Style:    discordgo.DangerButton,
			CustomID: "dashboard_btn#del_bookmark",
		})
	}

	components = append(components, discordgo.ActionsRow{
		Components: bottomButtons,
	})

	return "", components
}

func getExternalContractBookmarks(userID string) []boost.ExternalContractBookmark {
	str := farmerstate.GetMiscSettingString(userID, "ext_contract_bookmarks")
	var bms []boost.ExternalContractBookmark
	if str != "" {
		_ = json.Unmarshal([]byte(str), &bms)
	}
	sort.Slice(bms, func(i, j int) bool {
		return bms[i].Timestamp.Before(bms[j].Timestamp)
	})
	return bms
}

func saveExternalContractBookmarks(userID string, bms []boost.ExternalContractBookmark) {
	b, _ := json.Marshal(bms)
	farmerstate.SetMiscSettingString(userID, "ext_contract_bookmarks", string(b))
}

func addExternalContractBookmark(s *discordgo.Session, userID, contractID, coopID, channelID, guildID string) {
	bms := getExternalContractBookmarks(userID)

	channelName := "Unknown Channel"
	guildName := "Unknown Server"
	if ch, err := s.Channel(channelID); err == nil {
		channelName = ch.Name
		if g, err := s.Guild(ch.GuildID); err == nil {
			guildName = g.Name
		}
	}

	bms = append(bms, boost.ExternalContractBookmark{
		ContractID: contractID, CoopID: coopID, ChannelID: channelID, GuildID: guildID,
		ChannelName: channelName, GuildName: guildName, Timestamp: time.Now(),
	})
	if len(bms) > 15 {
		bms = bms[len(bms)-15:]
	}
	saveExternalContractBookmarks(userID, bms)
}

func getDashboardBookmarks(userID string) []boost.Bookmark {
	str := farmerstate.GetMiscSettingString(userID, "dashboard_bookmarks")
	var bms []boost.Bookmark
	if str != "" {
		_ = json.Unmarshal([]byte(str), &bms)
	}
	sort.Slice(bms, func(i, j int) bool {
		return bms[i].Timestamp.Before(bms[j].Timestamp)
	})
	return bms
}

func saveDashboardBookmarks(userID string, bms []boost.Bookmark) {
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
		bms = append(bms, boost.Bookmark{
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

func delExternalContractBookmark(userID, contractID, coopID string) {
	bms := getExternalContractBookmarks(userID)
	var newBms []boost.ExternalContractBookmark
	for _, bm := range bms {
		if !(bm.ContractID == contractID && strings.EqualFold(bm.CoopID, coopID)) {
			newBms = append(newBms, bm)
		}
	}
	saveExternalContractBookmarks(userID, newBms)
}

func delDashboardBookmark(userID string, channelID string) {
	bms := getDashboardBookmarks(userID)
	var newBms []boost.Bookmark
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
	userID := bottools.GetInteractionUserID(i)

	flags := discordgo.MessageFlags(0)
	if i.Message != nil && i.Message.Flags&discordgo.MessageFlagsEphemeral != 0 {
		flags |= discordgo.MessageFlagsEphemeral
	}
	flags |= discordgo.MessageFlagsIsComponentsV2

	switch action {
	case "add_ext_bm":
		if len(parts) < 4 {
			return
		}
		contractID := parts[2]
		coopID := parts[3]
		addExternalContractBookmark(s, userID, contractID, coopID, i.ChannelID, i.GuildID)
		content, components := drawDashboard(s, userID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    content,
				Components: components,
				Flags:      flags,
			},
		})

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
		content, components := drawDashboard(s, userID)
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
		extBms := getExternalContractBookmarks(userID)
		if len(bms) == 0 && len(extBms) == 0 {
			return
		}

		var bmBuilder strings.Builder
		bmBuilder.WriteString("## 🗑️ Delete a Bookmark\n")

		options := make([]discordgo.SelectMenuOption, 0, len(bms)+len(extBms))
		idx := 1
		for _, bm := range bms {
			if bm.GuildID != "" && bm.ChannelName != "" {
				fmt.Fprintf(&bmBuilder, "%d. Name: %s / Channel: #%s\n", idx, bm.ChannelName, bm.ChannelID)
			} else {
				fmt.Fprintf(&bmBuilder, "%d. Channel: <#%s>\n", idx, bm.ChannelID)
			}
			options = append(options, discordgo.SelectMenuOption{
				Label: fmt.Sprintf("%d", idx),
				Value: fmt.Sprintf("chan#%s", bm.ChannelID),
			})
			idx++
		}

		for _, bm := range extBms {
			fmt.Fprintf(&bmBuilder, "%d. Contract: %s / %s in <#%s>\n", idx, bm.ContractID, bm.CoopID, bm.ChannelID)
			options = append(options, discordgo.SelectMenuOption{
				Label: fmt.Sprintf("%d", idx),
				Value: fmt.Sprintf("cont#%s#%s", bm.ContractID, bm.CoopID),
			})
			idx++
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
			valParts := strings.Split(vals[0], "#")
			if len(valParts) > 1 {
				bmType := valParts[0]
				if bmType == "chan" && len(valParts) == 2 {
					delDashboardBookmark(userID, valParts[1])
				} else if bmType == "cont" && len(valParts) == 3 {
					delExternalContractBookmark(userID, valParts[1], valParts[2])
				}
			}
		}
		content, components := drawDashboard(s, userID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    content,
				Components: components,
				Flags:      flags,
			},
		})

	case "refresh":
		content, components := drawDashboard(s, userID)
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
