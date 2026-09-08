package dashboard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetSlashDashboardCommand returns the /dashboard slash command definition
func GetSlashDashboardCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "Manage your personal BoostBot dashboard",
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
		Options: []dc.Option{
			dc.SubCommand{
				Name:        "show",
				Description: "Show your personal BoostBot dashboard (active contracts, timers)",
			},
			dc.SubCommand{
				Name:        "add-bookmark",
				Description: "Add the current channel to your dashboard bookmarks",
			},
			dc.SubCommand{
				Name:        "remove-bookmark",
				Description: "Remove a channel from your dashboard bookmarks",
				Options: []dc.Option{
					dc.BoolOption{
						Name:        "all-channels",
						Description: "Clear all non-contract (channel) bookmarks",
						Required:    false,
					},
				},
			},
		},
	}
	return &command
}

// HandleDashboard handles the /dashboard command through the dc facade.
//
// It still takes a raw session because drawDashboard hands one to
// ei.GetFirstContactFromAPI, which is not on the facade yet. Every Discord
// call it makes itself goes through client or e.
func HandleDashboard(client dc.Client, e *dc.CommandEvent) {
	userID := e.UserID()

	subcommand, ok := e.Subcommand()
	if !ok {
		subcommand = "show"
	}

	switch subcommand {
	case "show":
		_ = e.Defer(true)

		components := drawDashboard(client, userID, false)

		msg, err := e.FollowupMessage(dc.Message{
			Components: components,
			Ephemeral:  true,
		})
		if err == nil && msg != nil {
			trackDashboard(userID, e, msg.ID)
		}

	case "add-bookmark":
		channelName := "Unknown Channel"
		guildName := "Unknown Server"
		guildID := ""
		if ch, err := client.Channel(e.ChannelID()); err == nil {
			channelName = ch.Name
			guildID = ch.GuildID
			if g, err := client.Guild(ch.GuildID); err == nil {
				guildName = g.Name
			}
		}
		addDashboardBookmark(userID, e.ChannelID(), guildID, guildName, channelName)
		bms := getDashboardBookmarks(userID)

		if len(bms) > 15 {
			components := getDeleteDialogComponents(userID, "channel", true)
			_ = e.Respond(dc.Message{
				Components: components,
				Ephemeral:  true,
			})
			return
		}

		UpdateDashboardsForUser(client, userID, "")

		msg := fmt.Sprintf("Bookmark added for <#%s>. You have %d/15 channel bookmarks.", e.ChannelID(), len(bms))

		activeDashboardsMutex.Lock()
		instances := activeDashboards[userID]
		if len(instances) > 0 {
			last := instances[len(instances)-1]
			gID := last.Event.GuildID()
			if gID == "" {
				gID = "@me"
			}
			msg += fmt.Sprintf("\n\n[Return to Dashboard](https://discord.com/channels/%s/%s/%s)", gID, last.Event.ChannelID(), last.MessageID)

			t, err := dc.SnowflakeTimestamp(last.MessageID)
			if err == nil {
				msg += " " + bottools.WrapTimestamp(t.Unix(), bottools.TimestampRelativeTime)
			}
		}
		activeDashboardsMutex.Unlock()

		_ = e.Respond(dc.Message{
			Components: []dc.LayoutComponent{dc.TextDisplay{Content: msg}},
			Ephemeral:  true,
		})

	case "remove-bookmark":
		clearAll, _ := e.OptBool("all-channels")

		if clearAll {
			saveDashboardBookmarks(userID, []boost.Bookmark{})
			UpdateDashboardsForUser(client, userID, "")

			_ = e.Respond(dc.Message{
				Components: []dc.LayoutComponent{dc.TextDisplay{Content: "All non-contract channel bookmarks have been cleared."}},
				Ephemeral:  true,
			})
			return
		}

		components := getDeleteDialogComponents(userID, "", true)

		_ = e.Respond(dc.Message{
			Components: components,
			Ephemeral:  true,
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
	Event     dc.InteractionEvent
	MessageID string
}

var (
	activeDashboards      = make(map[string][]dashboardInstance)
	activeDashboardsMutex sync.Mutex
)

func trackDashboard(userID string, e dc.InteractionEvent, messageID string) {
	activeDashboardsMutex.Lock()
	defer activeDashboardsMutex.Unlock()
	instances := activeDashboards[userID]
	instances = append(instances, dashboardInstance{
		Event:     e,
		MessageID: messageID,
	})
	if len(instances) > 2 {
		instances = instances[len(instances)-2:]
	}
	activeDashboards[userID] = instances
}

// UpdateDashboardsForUser updates any currently tracked dashboard messages for the user.
func UpdateDashboardsForUser(client dc.Client, userID string, currentMessageID string) {
	activeDashboardsMutex.Lock()
	instances := activeDashboards[userID]
	activeDashboardsMutex.Unlock()

	if len(instances) == 0 {
		return
	}

	components := drawDashboard(client, userID, false)

	for _, instance := range instances {
		if instance.MessageID == currentMessageID {
			continue
		}
		_ = instance.Event.EditFollowup(instance.MessageID, dc.Message{Components: components})
	}
}

func init() {
	bottools.UpdateDashboardDisplays = func(client dc.Client, userID string) {
		UpdateDashboardsForUser(client, userID, "")
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

func drawDashboard(client dc.Client, userID string, showExternal bool) []dc.LayoutComponent {
	var components []dc.LayoutComponent
	components = append(components, dc.TextDisplay{Content: "# 📊 Your BoostBot Dashboard"})

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

	var cached userExtContracts
	var cacheFound bool

	extContractCacheMutex.Lock()
	cached, cacheFound = extContractCache[userID]
	extContractCacheMutex.Unlock()

	if !cacheFound {
		str := farmerstate.GetMiscSettingString(userID, "ext_contract_cache")
		if str != "" {
			if err := json.Unmarshal([]byte(str), &cached); err == nil {
				cacheFound = true
				extContractCacheMutex.Lock()
				extContractCache[userID] = cached
				extContractCacheMutex.Unlock()
			}
		}
	}

	if cacheFound {
		if time.Now().Before(cached.Expires) {
			useCache = true
			extContractsToDisplay = cached.Contracts
			isFullLoad = cached.FullLoad
		} else {
			extContractCacheMutex.Lock()
			delete(extContractCache, userID)
			extContractCacheMutex.Unlock()
			farmerstate.SetMiscSettingString(userID, "ext_contract_cache", "")
		}
	}

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
			backup, _ := ei.GetFirstContactFromAPI(eeid, userID, true)
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
								lcContractID := lc.GetContractIdentifier()
								if lcContractID == "" && lc.GetContract() != nil {
									lcContractID = lc.GetContract().GetIdentifier()
								} else if lcContractID == "" && lc.GetEvaluation() != nil {
									lcContractID = lc.GetEvaluation().GetContractIdentifier()
								}
								if lcContractID == contractID {
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
							if coopID == "" {
								coopID = "N/A"
							}
							extContractsToDisplay = append(extContractsToDisplay, cachedExtContract{
								ContractID: contractID,
								CoopID:     coopID,
								State:      99,
							})
						}
					}
				}
			}
		} else {
			for _, bm := range extBookmarks {
				extContractsToDisplay = append(extContractsToDisplay, cachedExtContract{
					ContractID: bm.ContractID,
					CoopID:     bm.CoopID,
					State:      99,
				})
			}
		}
		cacheData := userExtContracts{
			Contracts: extContractsToDisplay,
			Expires:   time.Now().Add(4 * 24 * time.Hour),
			FullLoad:  isFullLoad,
		}
		extContractCacheMutex.Lock()
		extContractCache[userID] = cacheData
		extContractCacheMutex.Unlock()

		if b, err := json.Marshal(cacheData); err == nil {
			farmerstate.SetMiscSettingString(userID, "ext_contract_cache", string(b))
		}
	}

	// Fetch fresh status for external contracts using their cached IDs
	if eeid != "" {
		for i := range extContractsToDisplay {
			c := &extContractsToDisplay[i]
			if c.CoopID != "" && c.CoopID != "N/A" {
				startTime, durationSeconds, err := ei.GetCoopStatusStartTimeAndDuration(c.ContractID, c.CoopID, eeid)
				if err == nil {
					c.StartTime = startTime
					c.EstimatedEndTime = startTime.Add(time.Duration(durationSeconds) * time.Second)
				}
			}
		}
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
	var bookmarkButtons []dc.InteractiveComponent

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

				bookmarkButtons = append(bookmarkButtons, dc.Button{
					Label:    label,
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("dashboard_btn#add_ext_bm#%s#%s", c.ContractID, c.CoopID),
					Emoji:    &dc.Emoji{Name: "🔖"},
					Disabled: disabled,
				})
			}
		}
	} else {
		contractBuilder.WriteString("No active contracts.\n")
	}

	components = append(components, dc.Container{
		AccentColor: colorContracts,
		Components:  []dc.ContainerSubComponent{dc.TextDisplay{Content: "## 🚀 Active Contracts\n" + contractBuilder.String()}},
	})

	// Limit buttons per action row to 5 (Discord's maximum)
	for i := 0; i < len(bookmarkButtons); i += 5 {
		end := i + 5
		if end > len(bookmarkButtons) {
			end = len(bookmarkButtons)
		}
		components = append(components, dc.ActionRow{
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
		components = append(components, dc.Container{
			AccentColor: colorTimers,
			Components:  []dc.ContainerSubComponent{dc.TextDisplay{Content: "## ⏱️ Active Timers\n" + timerBuilder.String()}},
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

	components = append(components, dc.Container{
		AccentColor: colorBookmarks,
		Components:  []dc.ContainerSubComponent{dc.TextDisplay{Content: "## 🔖 Channel Bookmarks\n" + bmBuilder.String()}},
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
	components = append(components, dc.Container{
		AccentColor: colorCommands,
		Components:  []dc.ContainerSubComponent{dc.TextDisplay{Content: cmdBuilder.String()}},
	})

	var bottomButtons []dc.InteractiveComponent
	bottomButtons = append(bottomButtons, dc.Button{
		Label:    "Refresh Contracts",
		Style:    dc.ButtonSecondary,
		CustomID: "dashboard_btn#refresh",
		Emoji:    &dc.Emoji{Name: "🔄"},
	})
	if eeid != "" {
		extBtnLabel := "Discover External Contracts"
		if showExternal {
			extBtnLabel = "Reload External Contracts"
		}
		bottomButtons = append(bottomButtons, dc.Button{
			Label:    extBtnLabel,
			Style:    dc.ButtonSecondary,
			CustomID: "dashboard_btn#load_external",
			Emoji:    &dc.Emoji{Name: "☁️"},
		})
	}

	components = append(components, dc.ActionRow{
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

func addExternalContractBookmark(client dc.Client, userID, contractID, coopID, channelID, guildID string) {
	bms := getExternalContractBookmarks(userID)

	channelName := "Unknown Channel"
	guildName := "Unknown Server"
	if ch, err := client.Channel(channelID); err == nil {
		channelName = ch.Name
		if g, err := client.Guild(ch.GuildID); err == nil {
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

func getDeleteDialogComponents(userID string, replaceType string, isStandalone bool) []dc.LayoutComponent {
	bms := getDashboardBookmarks(userID)
	extBms := getExternalContractBookmarks(userID)
	if len(bms) == 0 && len(extBms) == 0 {
		return []dc.LayoutComponent{dc.TextDisplay{Content: "You have no bookmarks to delete."}}
	}

	var bmBuilder strings.Builder
	if replaceType != "" {
		bmBuilder.WriteString("## 🔖 Replace a Bookmark\n")
		bmBuilder.WriteString("You have reached the maximum number of bookmarks (15) for this type. Please select one to replace:\n")
	} else {
		bmBuilder.WriteString("## 🗑️ Delete a Bookmark\n")
	}

	var selectMenus []dc.LayoutComponent
	idx := 1

	standaloneStr := "dash"
	if isStandalone {
		standaloneStr = "standalone"
	}

	if replaceType == "" || replaceType == "channel" {
		chanOptions := make([]dc.SelectOption, 0, len(bms))
		for _, bm := range bms {
			if bm.GuildID != "" && bm.ChannelName != "" {
				fmt.Fprintf(&bmBuilder, "%d. Name: %s / Channel: #%s\n", idx, bm.ChannelName, bm.ChannelID)
			} else {
				fmt.Fprintf(&bmBuilder, "%d. Channel: <#%s>\n", idx, bm.ChannelID)
			}
			chanOptions = append(chanOptions, dc.SelectOption{
				Label: fmt.Sprintf("%d", idx),
				Value: fmt.Sprintf("chan#%s", bm.ChannelID),
			})
			idx++
		}
		if len(chanOptions) > 0 {
			minValues := 1
			selectMenus = append(selectMenus, dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.SelectMenu{
						CustomID:    fmt.Sprintf("dashboard_btn#del_select_chan#%s", standaloneStr),
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
		extOptions := make([]dc.SelectOption, 0, len(extBms))
		for _, bm := range extBms {
			contractName := ei.EggIncContractsAll[bm.ContractID].Name
			if contractName == "" {
				contractName = bm.ContractID
			}
			fmt.Fprintf(&bmBuilder, "%d. Contract: %s / %s in <#%s>\n", idx, contractName, bm.CoopID, bm.ChannelID)
			extOptions = append(extOptions, dc.SelectOption{
				Label: fmt.Sprintf("%d", idx),
				Value: fmt.Sprintf("cont#%s#%s", bm.ContractID, bm.CoopID),
			})
			idx++
		}
		if len(extOptions) > 0 {
			minValues := 1
			selectMenus = append(selectMenus, dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.SelectMenu{
						CustomID:    fmt.Sprintf("dashboard_btn#del_select_ext#%s", standaloneStr),
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

	components := []dc.LayoutComponent{
		dc.Container{
			AccentColor: accentColor,
			Components: []dc.ContainerSubComponent{
				dc.TextDisplay{Content: bmBuilder.String()},
			},
		},
	}

	components = append(components, selectMenus...)

	cancelID := "dashboard_btn#refresh"
	if isStandalone {
		cancelID = "dashboard_btn#cancel_standalone"
	}

	components = append(components, dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "Cancel",
				Style:    dc.ButtonSecondary,
				CustomID: cancelID,
			},
		},
	})

	return components
}

// HandleDashboardComponent handles dashboard component interactions through the
// dc facade.
//
// It still takes a raw session because drawDashboard hands one to
// ei.GetFirstContactFromAPI, which is not on the facade yet.
func HandleDashboardComponent(client dc.Client, e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	action := parts[1]
	userID := e.UserID()

	ephemeral := e.MessageIsEphemeral()

	switch action {
	case "add_ext_bm":
		if len(parts) < 4 {
			return
		}
		_ = e.DeferUpdate()

		contractID := parts[2]
		coopID := parts[3]
		addExternalContractBookmark(client, userID, contractID, coopID, e.ChannelID(), e.GuildID())

		extBms := getExternalContractBookmarks(userID)
		if len(extBms) > 15 {
			components := getDeleteDialogComponents(userID, "external", false)
			_ = e.EditResponse(dc.Message{Components: components, Ephemeral: ephemeral})
			return
		}

		components := drawDashboard(client, userID, true)
		_ = e.EditResponse(dc.Message{Components: components, Ephemeral: ephemeral})
		UpdateDashboardsForUser(client, userID, e.MessageID())

	case "load_external":
		_ = e.DeferUpdate()

		extContractCacheMutex.Lock()
		delete(extContractCache, userID)
		extContractCacheMutex.Unlock()
		farmerstate.SetMiscSettingString(userID, "ext_contract_cache", "")

		components := drawDashboard(client, userID, true)
		_ = e.EditResponse(dc.Message{Components: components, Ephemeral: ephemeral})
		UpdateDashboardsForUser(client, userID, e.MessageID())

	case "add_bookmark":
		channelName := "Unknown Channel"
		guildName := "Unknown Server"
		guildID := ""
		if ch, err := client.Channel(e.ChannelID()); err == nil {
			channelName = ch.Name
			guildID = ch.GuildID
			if g, err := client.Guild(ch.GuildID); err == nil {
				guildName = g.Name
			}
		}
		addDashboardBookmark(userID, e.ChannelID(), guildID, guildName, channelName)

		bms := getDashboardBookmarks(userID)
		if len(bms) > 15 {
			components := getDeleteDialogComponents(userID, "channel", false)
			_ = e.Update(dc.Message{Components: components, Ephemeral: ephemeral})
			return
		}

		components := drawDashboard(client, userID, false)
		_ = e.Update(dc.Message{Components: components, Ephemeral: ephemeral})
		UpdateDashboardsForUser(client, userID, e.MessageID())

	case "del_bookmark":
		components := getDeleteDialogComponents(userID, "", false)
		_ = e.Update(dc.Message{Components: components, Ephemeral: ephemeral})

	case "del_select", "del_select_chan", "del_select_ext":
		vals := e.Values()
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

		var components []dc.LayoutComponent
		isStandalone := len(parts) > 2 && parts[2] == "standalone"
		isEphemeral := e.MessageIsEphemeral()
		if isStandalone || isEphemeral {
			msg := "Bookmark removed/replaced."
			bms := getDashboardBookmarks(userID)
			extBms := getExternalContractBookmarks(userID)
			if strings.Contains(action, "chan") {
				if len(bms) >= 15 {
					msg = fmt.Sprintf("Bookmark replaced. You have %d/15 channel bookmarks.", len(bms))
				} else {
					msg = fmt.Sprintf("Bookmark removed. You have %d/15 channel bookmarks.", len(bms))
				}
			} else if strings.Contains(action, "ext") {
				if len(extBms) >= 15 {
					msg = fmt.Sprintf("Bookmark replaced. You have %d/15 external bookmarks.", len(extBms))
				} else {
					msg = fmt.Sprintf("Bookmark removed. You have %d/15 external bookmarks.", len(extBms))
				}
			}

			var recentDashboard bool
			activeDashboardsMutex.Lock()
			instances := activeDashboards[userID]
			if len(instances) > 0 {
				last := instances[len(instances)-1]
				gID := last.Event.GuildID()
				if gID == "" {
					gID = "@me"
				}
				msg += fmt.Sprintf("\n\n[Return to Dashboard](https://discord.com/channels/%s/%s/%s)", gID, last.Event.ChannelID(), last.MessageID)
				t, err := dc.SnowflakeTimestamp(last.MessageID)
				if err == nil {
					msg += " " + bottools.WrapTimestamp(t.Unix(), bottools.TimestampRelativeTime)
					if time.Since(t) <= 10*time.Minute {
						recentDashboard = true
					}
				}
			}
			activeDashboardsMutex.Unlock()

			components = append(components, dc.TextDisplay{Content: msg})

			if recentDashboard {
				components = append(components, dc.ActionRow{
					Components: []dc.InteractiveComponent{
						dc.Button{
							Label:    "Move Dashboard Here",
							Style:    dc.ButtonPrimary,
							CustomID: "dashboard_btn#move_here",
							Emoji:    &dc.Emoji{Name: "📊"},
						},
					},
				})
			}

		} else {
			components = drawDashboard(client, userID, false)
		}

		_ = e.Update(dc.Message{Components: components, Ephemeral: ephemeral})
		UpdateDashboardsForUser(client, userID, e.MessageID())

	case "cancel_standalone":
		bms := getDashboardBookmarks(userID)
		if len(bms) > 15 {
			saveDashboardBookmarks(userID, bms[:len(bms)-1])
		}
		extBms := getExternalContractBookmarks(userID)
		if len(extBms) > 15 {
			saveExternalContractBookmarks(userID, extBms[:len(extBms)-1])
		}

		msg := "Action cancelled."
		var components []dc.LayoutComponent
		var recentDashboard bool
		activeDashboardsMutex.Lock()
		instances := activeDashboards[userID]
		if len(instances) > 0 {
			last := instances[len(instances)-1]
			gID := last.Event.GuildID()
			if gID == "" {
				gID = "@me"
			}
			msg += fmt.Sprintf("\n\n[Return to Dashboard](https://discord.com/channels/%s/%s/%s)", gID, last.Event.ChannelID(), last.MessageID)
			t, err := dc.SnowflakeTimestamp(last.MessageID)
			if err == nil {
				msg += " " + bottools.WrapTimestamp(t.Unix(), bottools.TimestampRelativeTime)
				if time.Since(t) <= 10*time.Minute {
					recentDashboard = true
				}
			}
		}
		activeDashboardsMutex.Unlock()

		components = append(components, dc.TextDisplay{Content: msg})

		if recentDashboard {
			components = append(components, dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.Button{
						Label:    "Move Dashboard Here",
						Style:    dc.ButtonPrimary,
						CustomID: "dashboard_btn#move_here",
						Emoji:    &dc.Emoji{Name: "📊"},
					},
				},
			})
		}

		_ = e.Update(dc.Message{Components: components, Ephemeral: ephemeral})

	case "move_here":
		components := drawDashboard(client, userID, false)
		_ = e.Update(dc.Message{Components: components, Ephemeral: ephemeral})
		if e.MessageID() != "" {
			trackDashboard(userID, e, e.MessageID())
			UpdateDashboardsForUser(client, userID, e.MessageID())
		}

	case "refresh":
		components := drawDashboard(client, userID, false)
		_ = e.Update(dc.Message{Components: components, Ephemeral: ephemeral})
		UpdateDashboardsForUser(client, userID, e.MessageID())
	}
}
