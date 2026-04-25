package dashboard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
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
				Description: "Remove a channel from your dashboard bookmarks",
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

		components := drawDashboard(s, userID, false)

		msg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
		})
		if err == nil && msg != nil {
			trackDashboard(userID, i.Interaction, msg.ID)
		}

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

		if len(bms) > 15 {
			components := getDeleteDialogComponents(userID, "channel")
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Components: components,
					Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
				},
			})
			return
		}

		UpdateDashboardsForUser(s, userID, "")

		msg := fmt.Sprintf("Bookmark added for <#%s>. You have %d/15 channel bookmarks.", i.ChannelID, len(bms))

		activeDashboardsMutex.Lock()
		instances := activeDashboards[userID]
		if len(instances) > 0 {
			last := instances[len(instances)-1]
			gID := last.Interaction.GuildID
			if gID == "" {
				gID = "@me"
			}
			msg += fmt.Sprintf("\n\n[Return to Dashboard](https://discord.com/channels/%s/%s/%s)", gID, last.Interaction.ChannelID, last.MessageID)

			t, err := discordgo.SnowflakeTimestamp(last.MessageID)
			if err == nil {
				msg += " " + bottools.WrapTimestamp(t.Unix(), bottools.TimestampRelativeTime)
			}
		}
		activeDashboardsMutex.Unlock()

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Components: []discordgo.MessageComponent{discordgo.TextDisplay{Content: msg}},
				Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
			},
		})

	case "remove-bookmark":
		components := getDeleteDialogComponents(userID, "")

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Components: components,
				Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
			},
		})
	}
}

type cachedExtContract struct {
	ContractID       string
	CoopID           string
	StartTime        time.Time
	EstimatedEndTime time.Time
	State            int
}

type userExtContracts struct {
	Contracts []cachedExtContract
	Expires   time.Time
	FullLoad  bool
}

var (
	extContractCache      = make(map[string]userExtContracts)
	extContractCacheMutex sync.Mutex
)

type dashboardInstance struct {
	Interaction *discordgo.Interaction
	MessageID   string
}

var (
	activeDashboards      = make(map[string][]dashboardInstance)
	activeDashboardsMutex sync.Mutex
)

func trackDashboard(userID string, interaction *discordgo.Interaction, messageID string) {
	activeDashboardsMutex.Lock()
	defer activeDashboardsMutex.Unlock()
	instances := activeDashboards[userID]
	instances = append(instances, dashboardInstance{
		Interaction: interaction,
		MessageID:   messageID,
	})
	if len(instances) > 2 {
		instances = instances[len(instances)-2:]
	}
	activeDashboards[userID] = instances
}

// UpdateDashboardsForUser updates any currently tracked dashboard messages for the user.
func UpdateDashboardsForUser(s *discordgo.Session, userID string, currentMessageID string) {
	activeDashboardsMutex.Lock()
	instances := activeDashboards[userID]
	activeDashboardsMutex.Unlock()

	if len(instances) == 0 {
		return
	}

	components := drawDashboard(s, userID, false)

	for _, instance := range instances {
		if instance.MessageID == currentMessageID {
			continue
		}
		_, _ = s.FollowupMessageEdit(instance.Interaction, instance.MessageID, &discordgo.WebhookEdit{
			Components: &components,
		})
	}
}

func init() {
	bottools.UpdateDashboardDisplays = func(s *discordgo.Session, userID string) {
		UpdateDashboardsForUser(s, userID, "")
	}

	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		for range ticker.C {
			extContractCacheMutex.Lock()
			now := time.Now()
			for k, v := range extContractCache {
				if now.After(v.Expires) {
					delete(extContractCache, k)
				}
			}
			extContractCacheMutex.Unlock()
		}
	}()
}

func drawDashboard(s *discordgo.Session, userID string, showExternal bool) []discordgo.MessageComponent {
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

	var extContractsToDisplay []cachedExtContract
	var useCache bool
	var isFullLoad bool

	extContractCacheMutex.Lock()
	if cached, ok := extContractCache[userID]; ok {
		if time.Now().Before(cached.Expires) {
			useCache = true
			extContractsToDisplay = cached.Contracts
			isFullLoad = cached.FullLoad
		} else {
			delete(extContractCache, userID)
		}
	}
	extContractCacheMutex.Unlock()

	if showExternal && eeid != "" {
		if !isFullLoad {
			useCache = false // Force refresh if explicitly requested a full load and cache is partial
			extContractsToDisplay = nil
		}
	} else if useCache && isFullLoad {
		showExternal = true // Treat as shown to process bookmarks and hide button
	}

	if !useCache && eeid != "" {
		isFullLoad = showExternal

		if isFullLoad {
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
							for _, lc := range backup.GetContracts().GetContracts() {
								if lc.GetContract() != nil && lc.GetContract().GetIdentifier() == contractID {
									coopID = lc.GetCoopIdentifier()
									break
								}
							}
							if coopID == "" {
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
						}

						if contractID != "" {
							var startTime, estEndTime time.Time
							if coopID != "" {
								st, durationSeconds, err := ei.GetCoopStatusStartTimeAndDuration(contractID, coopID, eeid)
								if err == nil {
									startTime = st
									estEndTime = startTime.Add(time.Duration(durationSeconds) * time.Second)
								}
							} else {
								coopID = "N/A"
							}
							extContractsToDisplay = append(extContractsToDisplay, cachedExtContract{
								ContractID:       contractID,
								CoopID:           coopID,
								StartTime:        startTime,
								EstimatedEndTime: estEndTime,
								State:            99,
							})
						}
					}
				}
			}
		} else {
			for _, bm := range extBookmarks {
				startTime, durationSeconds, err := ei.GetCoopStatusStartTimeAndDuration(bm.ContractID, bm.CoopID, eeid)
				var estEndTime time.Time
				if err == nil {
					estEndTime = startTime.Add(time.Duration(durationSeconds) * time.Second)
				}
				extContractsToDisplay = append(extContractsToDisplay, cachedExtContract{
					ContractID:       bm.ContractID,
					CoopID:           bm.CoopID,
					StartTime:        startTime,
					EstimatedEndTime: estEndTime,
					State:            99,
				})
			}
		}
		extContractCacheMutex.Lock()
		extContractCache[userID] = userExtContracts{
			Contracts: extContractsToDisplay,
			Expires:   time.Now().Add(2 * time.Hour),
			FullLoad:  isFullLoad,
		}
		extContractCacheMutex.Unlock()
	}

	for _, c := range extContractsToDisplay {
		found := false
		for _, ac := range activeContracts {
			if ac.ContractID == c.ContractID {
				found = true
				break
			}
		}

		bookmarkKey := fmt.Sprintf("%s:%s", c.ContractID, strings.ToLower(c.CoopID))

		if !found {
			if c.EstimatedEndTime.IsZero() || time.Since(c.EstimatedEndTime) <= 24*time.Hour {
				dummy := &boost.Contract{
					ContractID:       c.ContractID,
					CoopID:           c.CoopID,
					StartTime:        c.StartTime,
					EstimatedEndTime: c.EstimatedEndTime,
					State:            c.State,
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
		} else {
			if _, ok := extBookmarkMap[bookmarkKey]; ok {
				seenBookmarks[bookmarkKey] = true
			}
		}
	}

	if showExternal {
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

	var contractBuilder strings.Builder
	var bookmarkButtons []discordgo.MessageComponent

	contractCount := len(activeContracts)
	if contractCount > 0 {

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

			contractName := c.Name
			if contractName == "" {
				eiContract := ei.EggIncContractsAll[c.ContractID]
				if eiContract.ID != "" && eiContract.Name != "" {
					contractName = eiContract.Name
				} else {
					contractName = c.ContractID
				}
			}

			fmt.Fprintf(&contractBuilder, "**%s / %s**\n%s\n", contractName, c.CoopID, channelStr)
			fmt.Fprintf(&contractBuilder, "-# _       _ %s\n", timeStr)

			if c.State == 99 && len(c.Location) == 0 { // It's an un-bookmarked external contract
				label := "Bookmark " + contractName
				if len(label) > 80 {
					label = label[:80]
				}

				disabled := false
				if c.CoopID == "N/A" || c.CoopID == "" {
					disabled = true
				}

				bookmarkButtons = append(bookmarkButtons, discordgo.Button{
					Label:    label,
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("dashboard_btn#add_ext_bm#%s#%s", c.ContractID, c.CoopID),
					Emoji:    &discordgo.ComponentEmoji{Name: "🔖"},
					Disabled: disabled,
				})
			}
		}
	} else {
		contractBuilder.WriteString("No active contracts.\n")
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
	var bmBuilder strings.Builder

	if len(bms) > 0 {
		for _, bm := range bms {
			if bm.GuildID != "" && bm.ChannelName != "" {
				fmt.Fprintf(&bmBuilder, "[#%s](https://discord.com/channels/%s/%s)\n", bm.ChannelName, bm.GuildID, bm.ChannelID)
			} else {
				fmt.Fprintf(&bmBuilder, "<#%s>\n", bm.ChannelID)
			}
		}
	} else {
		bmBuilder.WriteString("No channel bookmarks.\n")
	}

	addBmCmd := bottools.GetFormattedCommand("dashboard add-bookmark")
	if addBmCmd == "" {
		addBmCmd = "`/dashboard add-bookmark`"
	}
	rmBmCmd := bottools.GetFormattedCommand("dashboard remove-bookmark")
	if rmBmCmd == "" {
		rmBmCmd = "`/dashboard remove-bookmark`"
	}
	fmt.Fprintf(&bmBuilder, "\n-# %s %s\n", addBmCmd, rmBmCmd)

	components = append(components, discordgo.Container{
		AccentColor: &colorBookmarks,
		Components:  []discordgo.MessageComponent{discordgo.TextDisplay{Content: "## 🔖 Channel Bookmarks\n" + bmBuilder.String()}},
	})

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
	if eeid != "" {
		extBtnLabel := "Load External Contracts"
		if showExternal {
			extBtnLabel = "Refresh External Contracts"
		}
		bottomButtons = append(bottomButtons, discordgo.Button{
			Label:    extBtnLabel,
			Style:    discordgo.SecondaryButton,
			CustomID: "dashboard_btn#load_external",
			Emoji:    &discordgo.ComponentEmoji{Name: "☁️"},
		})
	}

	components = append(components, discordgo.ActionsRow{
		Components: bottomButtons,
	})

	return components
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
	if len(bms) > 25 {
		bms = bms[len(bms)-25:]
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
	if len(bms) > 25 {
		bms = bms[len(bms)-25:]
	}
	saveDashboardBookmarks(userID, bms)
}

func delExternalContractBookmark(userID, contractID, coopID string) {
	bms := getExternalContractBookmarks(userID)
	var newBms []boost.ExternalContractBookmark
	for _, bm := range bms {
		if bm.ContractID != contractID || !strings.EqualFold(bm.CoopID, coopID) {
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

func getDeleteDialogComponents(userID string, replaceType string) []discordgo.MessageComponent {
	bms := getDashboardBookmarks(userID)
	extBms := getExternalContractBookmarks(userID)
	if len(bms) == 0 && len(extBms) == 0 {
		return []discordgo.MessageComponent{discordgo.TextDisplay{Content: "You have no bookmarks to delete."}}
	}

	var bmBuilder strings.Builder
	if replaceType != "" {
		bmBuilder.WriteString("## 🔖 Replace a Bookmark\n")
		bmBuilder.WriteString("You have reached the maximum number of bookmarks (15) for this type. Please select one to replace:\n")
	} else {
		bmBuilder.WriteString("## 🗑️ Delete a Bookmark\n")
	}

	var selectMenus []discordgo.MessageComponent
	idx := 1

	if replaceType == "" || replaceType == "channel" {
		chanOptions := make([]discordgo.SelectMenuOption, 0, len(bms))
		for _, bm := range bms {
			if bm.GuildID != "" && bm.ChannelName != "" {
				fmt.Fprintf(&bmBuilder, "%d. Name: %s / Channel: #%s\n", idx, bm.ChannelName, bm.ChannelID)
			} else {
				fmt.Fprintf(&bmBuilder, "%d. Channel: <#%s>\n", idx, bm.ChannelID)
			}
			chanOptions = append(chanOptions, discordgo.SelectMenuOption{
				Label: fmt.Sprintf("%d", idx),
				Value: fmt.Sprintf("chan#%s", bm.ChannelID),
			})
			idx++
		}
		if len(chanOptions) > 0 {
			minValues := 1
			selectMenus = append(selectMenus, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "dashboard_btn#del_select_chan",
						Placeholder: "Select channel bookmark to delete",
						Options:     chanOptions,
						MinValues:   &minValues,
						MaxValues:   1,
					},
				},
			})
		}
	}

	if replaceType == "" || replaceType == "external" {
		extOptions := make([]discordgo.SelectMenuOption, 0, len(extBms))
		for _, bm := range extBms {
			contractName := ei.EggIncContractsAll[bm.ContractID].Name
			if contractName == "" {
				contractName = bm.ContractID
			}
			fmt.Fprintf(&bmBuilder, "%d. Contract: %s / %s in <#%s>\n", idx, contractName, bm.CoopID, bm.ChannelID)
			extOptions = append(extOptions, discordgo.SelectMenuOption{
				Label: fmt.Sprintf("%d", idx),
				Value: fmt.Sprintf("cont#%s#%s", bm.ContractID, bm.CoopID),
			})
			idx++
		}
		if len(extOptions) > 0 {
			minValues := 1
			selectMenus = append(selectMenus, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "dashboard_btn#del_select_ext",
						Placeholder: "Select contract bookmark to delete",
						Options:     extOptions,
						MinValues:   &minValues,
						MaxValues:   1,
					},
				},
			})
		}
	}

	accentColor := 0xed4245 // Danger red
	if replaceType != "" {
		accentColor = 0xfee75c // Yellow for warning/replace
	}

	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &accentColor,
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: bmBuilder.String()},
			},
		},
	}

	components = append(components, selectMenus...)

	components = append(components, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Cancel",
				Style:    discordgo.SecondaryButton,
				CustomID: "dashboard_btn#refresh",
			},
		},
	})

	return components
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
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})

		contractID := parts[2]
		coopID := parts[3]
		addExternalContractBookmark(s, userID, contractID, coopID, i.ChannelID, i.GuildID)

		extBms := getExternalContractBookmarks(userID)
		if len(extBms) > 15 {
			components := getDeleteDialogComponents(userID, "external")
			_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Components: &components,
			})
			return
		}

		components := drawDashboard(s, userID, true)
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Components: &components,
		})
		UpdateDashboardsForUser(s, userID, i.Message.ID)

	case "load_external":
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})

		extContractCacheMutex.Lock()
		delete(extContractCache, userID)
		extContractCacheMutex.Unlock()

		components := drawDashboard(s, userID, true)
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Components: &components,
		})
		UpdateDashboardsForUser(s, userID, i.Message.ID)

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

		bms := getDashboardBookmarks(userID)
		if len(bms) > 15 {
			components := getDeleteDialogComponents(userID, "channel")
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Components: components,
					Flags:      flags,
				},
			})
			return
		}

		components := drawDashboard(s, userID, false)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Components: components,
				Flags:      flags,
			},
		})
		UpdateDashboardsForUser(s, userID, i.Message.ID)

	case "del_bookmark":
		components := getDeleteDialogComponents(userID, "")
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Components: components,
				Flags:      flags,
			},
		})

	case "del_select", "del_select_chan", "del_select_ext":
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
		components := drawDashboard(s, userID, false)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Components: components,
				Flags:      flags,
			},
		})
		UpdateDashboardsForUser(s, userID, i.Message.ID)

	case "refresh":
		components := drawDashboard(s, userID, false)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Components: components,
				Flags:      flags,
			},
		})
		UpdateDashboardsForUser(s, userID, i.Message.ID)
	}
}
