package watch

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	WatchTypeContract     = "contract"
	WatchTypeColleggtible = "colleggtible"
)

var (
	newColleggtibles []string
	newCollMutex     sync.Mutex
)

// AddNewColleggtible registers a newly detected custom egg ID.
func AddNewColleggtible(id string) {
	newCollMutex.Lock()
	defer newCollMutex.Unlock()
	newColleggtibles = append(newColleggtibles, id)
}

// GetSlashWatchCommand returns the slash command definition for /watch.
func GetSlashWatchCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Watch for contracts or colleggtibles and get notified when they become available.",
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
				Name:        "contract",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Description: "Watch a specific contract.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "contract-id",
						Description:  "Contract ID to watch.",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{
				Name:        "colleggtible",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Description: "Watch a specific colleggtible.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "colleggtible-id",
						Description:  "Colleggtible to watch.",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{
				Name:        "missing",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Description: "Automatically watch all missing contracts and colleggtibles based on your EI backup.",
			},
			{
				Name:        "status",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Description: "View and manage your active watches.",
			},
		},
	}
}

// HandleWatchCommand handles the /watch command.
func HandleWatchCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		return
	}

	subcmd := options[0].Name
	userID := bottools.GetInteractionUserID(i)

	switch subcmd {
	case "contract":
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
		})
		contractID := ""
		for _, opt := range options[0].Options {
			if opt.Name == "contract-id" {
				contractID = opt.StringValue()
			}
		}
		if contractID == "" {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "Please provide a valid contract ID.",
			})
			return
		}

		// If they already have a watch for this contract, toggle it off (clear it)
		hasWatch := false
		watches := farmerstate.GetWatchesForUser(userID)
		for _, w := range watches {
			if w.WatchType == WatchTypeContract && w.TargetID == contractID {
				hasWatch = true
				break
			}
		}
		if hasWatch {
			farmerstate.DeleteWatch(userID, WatchTypeContract, contractID)
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("Watch for contract `%s` cleared/removed.", contractID),
			})
			return
		}

		// If contract is currently active, clear the watch instead of adding it
		isActive := false
		for _, c := range ei.EggIncContracts {
			if c.ID == contractID && !c.Predicted {
				isActive = true
				break
			}
		}
		if isActive {
			farmerstate.DeleteWatch(userID, WatchTypeContract, contractID)
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("Contract `%s` is currently active. Watch cleared/removed.", contractID),
			})
			return
		}

		farmerstate.AddWatch(userID, WatchTypeContract, contractID)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Success! Added watch for contract: `%s`.", contractID),
		})

	case "colleggtible":
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
		})
		colleggtibleID := ""
		for _, opt := range options[0].Options {
			if opt.Name == "colleggtible-id" {
				colleggtibleID = opt.StringValue()
			}
		}
		if colleggtibleID == "" {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "Please provide a valid colleggtible ID.",
			})
			return
		}

		// If they already have a watch for this colleggtible, toggle it off (clear it)
		hasWatch := false
		watches := farmerstate.GetWatchesForUser(userID)
		for _, w := range watches {
			if w.WatchType == WatchTypeColleggtible && w.TargetID == colleggtibleID {
				hasWatch = true
				break
			}
		}
		if hasWatch {
			farmerstate.DeleteWatch(userID, WatchTypeColleggtible, colleggtibleID)
			msg := fmt.Sprintf("Watch for colleggtible `%s` cleared/removed.", colleggtibleID)
			if colleggtibleID == "new" {
				msg = "Watch for any **NEW COLLEGGTIBLES** cleared/removed."
			}
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: msg,
			})
			return
		}

		// If colleggtible is currently active, clear the watch instead of adding it
		if colleggtibleID != "new" {
			isActive := false
			activeContractName := ""
			for _, c := range ei.EggIncContracts {
				if c.Predicted {
					continue
				}
				if c.EggName == colleggtibleID {
					isActive = true
					activeContractName = c.Name
					break
				}
			}
			if isActive {
				farmerstate.DeleteWatch(userID, WatchTypeColleggtible, colleggtibleID)
				_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: fmt.Sprintf("Colleggtible `%s` is currently active (offered in contract **%s**). Watch cleared/removed.", colleggtibleID, activeContractName),
				})
				return
			}
		}

		farmerstate.AddWatch(userID, WatchTypeColleggtible, colleggtibleID)
		msgContent := fmt.Sprintf("Success! Added watch for colleggtible: `%s`.", colleggtibleID)
		if colleggtibleID == "new" {
			msgContent = "Success! Added watch for any **NEW COLLEGGTIBLES**."
		}
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: msgContent,
		})

	case "missing":
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
		})
		eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		if eiID == "" {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "No EIID found. Please run the `/register` command first to link your account.",
			})
			return
		}
		backup, _ := ei.GetFirstContactFromAPI(s, eiID, userID, true)
		if backup == nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "Failed to retrieve your player data from Egg Inc API. Please try again later.",
			})
			return
		}

		completed := make(map[string]bool)
		ownedColleggtibles := make(map[string]bool)

		if backup.GetContracts() != nil {
			for _, c := range append(backup.GetContracts().GetArchive(), backup.GetContracts().GetContracts()...) {
				if c == nil {
					continue
				}
				id := c.GetContractIdentifier()
				if id == "" && c.GetContract() != nil {
					id = c.GetContract().GetIdentifier()
				}
				if id != "" {
					completed[id] = true
				}
				eggID := ""
				if c.GetContract() != nil {
					eggID = c.GetContract().GetCustomEggId()
				} else if id != "" {
					if contractInfo, ok := ei.GetEggIncContract(id); ok {
						if contractInfo.Egg == int32(ei.Egg_CUSTOM_EGG) {
							eggID = contractInfo.EggName
						}
					}
				}
				if eggID == "" {
					continue
				}
				if c.GetMaxFarmSizeReached() >= 1e7 {
					ownedColleggtibles[eggID] = true
				}
			}
		}

		var missingContracts []string
		for id, c := range ei.EggIncContractsAll {
			if c.Predicted {
				continue
			}
			if id == "first-contract" {
				continue
			}
			if !completed[id] {
				missingContracts = append(missingContracts, id)
			}
		}

		var missingColleggtibles []string
		for id := range ei.CustomEggMap {
			if !ownedColleggtibles[id] {
				missingColleggtibles = append(missingColleggtibles, id)
			}
		}

		for _, contractID := range missingContracts {
			farmerstate.AddWatch(userID, WatchTypeContract, contractID)
		}
		for _, eggID := range missingColleggtibles {
			farmerstate.AddWatch(userID, WatchTypeColleggtible, eggID)
		}

		renderStatusPage(s, i.Interaction, userID, 0, false)

	case "status":
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
		})
		renderStatusPage(s, i.Interaction, userID, 0, false)
	}
}

// HandleWatchAutoComplete handles autocomplete requests for /watch parameters.
func HandleWatchAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if len(data.Options) == 0 {
		return
	}
	subcmd := data.Options[0]
	optionMap := bottools.GetCommandOptionsMap(i)

	if subcmd.Name == "contract" {
		boost.HandleAllContractsAutoComplete(s, i)
		return
	}

	if subcmd.Name == "colleggtible" {
		searchString := ""
		if opt, ok := optionMap["colleggtible-id"]; ok {
			searchString = strings.ToLower(opt.StringValue())
		}

		choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)
		if searchString == "" || strings.Contains("new colleggtible", searchString) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  "NEW COLLEGGTIBLE",
				Value: "new",
			})
		}
		type eggChoice struct {
			Name string
			ID   string
		}
		var list []eggChoice
		for id, egg := range ei.CustomEggMap {
			if egg != nil {
				if searchString == "" || strings.Contains(strings.ToLower(egg.Name), searchString) || strings.Contains(strings.ToLower(id), searchString) {
					list = append(list, eggChoice{Name: egg.Name, ID: id})
				}
			}
		}
		sort.Slice(list, func(i, j int) bool {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		})

		for _, item := range list {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  item.Name,
				Value: item.ID,
			})
		}
		if len(choices) > 25 {
			choices = choices[:25]
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: choices,
			},
		})
	}
}

// renderStatusPage renders the watches status page.
func renderStatusPage(s *discordgo.Session, interaction *discordgo.Interaction, userID string, page int, showClearConfirm bool) {
	watches := farmerstate.GetWatchesForUser(userID)
	if len(watches) == 0 {
		components := []discordgo.MessageComponent{}
		// We want to update or send followup
		content := "You currently have no active watches."
		if interaction.Type == discordgo.InteractionMessageComponent {
			_, _ = s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content:    &content,
				Components: &components,
			})
		} else {
			_, _ = s.FollowupMessageCreate(interaction, true, &discordgo.WebhookParams{
				Content:    content,
				Components: components,
			})
		}
		return
	}

	sortStyle := farmerstate.GetMiscSettingString(userID, "watch_sort")
	if sortStyle == "" {
		sortStyle = "predicted"
	}

	contractPreds, eggPreds := boost.GetPredictedTimes()
	getPredTime := func(w farmerstate.Watch) time.Time {
		if w.WatchType == WatchTypeContract {
			return contractPreds[w.TargetID]
		}
		return eggPreds[w.TargetID]
	}

	sort.Slice(watches, func(i, j int) bool {
		// "new" watches should always appear first
		if watches[i].TargetID == "new" && watches[j].TargetID != "new" {
			return true
		}
		if watches[j].TargetID == "new" && watches[i].TargetID != "new" {
			return false
		}

		if sortStyle == "predicted" {
			tI := getPredTime(watches[i])
			tJ := getPredTime(watches[j])
			if !tI.IsZero() && !tJ.IsZero() {
				if !tI.Equal(tJ) {
					return tI.Before(tJ)
				}
			} else if !tI.IsZero() {
				return true
			} else if !tJ.IsZero() {
				return false
			}
		}
		// Fallback to alphabetical sorting of the target name or ID
		getName := func(w farmerstate.Watch) string {
			if w.WatchType == WatchTypeColleggtible {
				if w.TargetID == "new" {
					return "New Colleggtibles"
				}
				if egg, ok := ei.CustomEggMap[w.TargetID]; ok && egg != nil {
					return egg.Name
				}
			} else {
				if c, ok := ei.EggIncContractsAll[w.TargetID]; ok {
					return c.Name
				}
			}
			return w.TargetID
		}
		nameI := strings.ToLower(getName(watches[i]))
		nameJ := strings.ToLower(getName(watches[j]))
		return nameI < nameJ
	})

	pageSize := 10
	totalPages := int(math.Ceil(float64(len(watches)) / float64(pageSize)))
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * pageSize
	end := start + pageSize
	if end > len(watches) {
		end = len(watches)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "### Your Active Watches (Page %d/%d):\n", page+1, totalPages)
	for idx, w := range watches[start:end] {
		targetName := w.TargetID
		if w.WatchType == WatchTypeColleggtible {
			if w.TargetID == "new" {
				targetName = "New Colleggtibles"
			} else if egg, ok := ei.CustomEggMap[w.TargetID]; ok && egg != nil {
				targetName = egg.Name
			}
		} else {
			if c, ok := ei.EggIncContractsAll[w.TargetID]; ok {
				targetName = c.Name
			}
		}

		pTime := getPredTime(w)
		timeStr := ""
		if !pTime.IsZero() {
			timeStr = fmt.Sprintf(" - 🔮 <t:%d:d> (<t:%d:R>)", pTime.Unix(), pTime.Unix())
		} else {
			timeStr = " - 🔮 Unknown"
		}

		typeStr := "📜"
		if w.WatchType == WatchTypeColleggtible {
			if w.TargetID == "new" {
				typeStr = "🆕"
			} else {
				typeStr = ei.FindEggEmoji(w.TargetID)
			}
		} else {
			if c, ok := ei.EggIncContractsAll[w.TargetID]; ok {
				typeStr = ei.FindEggEmoji(c.EggName)
			}
		}

		fmt.Fprintf(&sb, "%d. %s **%s** `%s` %s\n", start+idx+1, typeStr, targetName, w.TargetID, timeStr)
	}

	// Create buttons
	var row1 []discordgo.MessageComponent
	if totalPages > 2 {
		row1 = append(row1, discordgo.Button{
			Label:    "⏮ First",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("watch-page-first#%s#0", userID),
			Disabled: page == 0,
		})
	}
	row1 = append(row1, discordgo.Button{
		Label:    "◀ Previous",
		Style:    discordgo.SecondaryButton,
		CustomID: fmt.Sprintf("watch-page-prev#%s#%d", userID, page-1),
		Disabled: page == 0,
	})
	row1 = append(row1, discordgo.Button{
		Label:    "Next ▶",
		Style:    discordgo.SecondaryButton,
		CustomID: fmt.Sprintf("watch-page-next#%s#%d", userID, page+1),
		Disabled: page == totalPages-1,
	})
	if totalPages > 2 {
		row1 = append(row1, discordgo.Button{
			Label:    "Last ⏭",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("watch-page-last#%s#%d", userID, totalPages-1),
			Disabled: page == totalPages-1,
		})
	}

	var row2 []discordgo.MessageComponent
	sortLabel := "Sort: Predicted"
	if sortStyle == "alpha" {
		sortLabel = "Sort: Alphabetical"
	}
	row2 = append(row2, discordgo.Button{
		Label:    sortLabel,
		Style:    discordgo.PrimaryButton,
		CustomID: fmt.Sprintf("watch-toggle-sort#%s#%d", userID, page),
	})

	if showClearConfirm {
		row2 = append(row2, discordgo.Button{
			Label:    "Are you sure?",
			Style:    discordgo.DangerButton,
			CustomID: fmt.Sprintf("watch-clear#%s", userID),
		})
		row2 = append(row2, discordgo.Button{
			Label:    "Cancel",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("watch-page-cancel#%s#%d", userID, page),
		})
	} else {
		row2 = append(row2, discordgo.Button{
			Label:    "Clear All Watches (safe)",
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("watch-clear-confirm#%s#%d", userID, page),
		})
	}

	content := sb.String()
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: row1,
		},
		discordgo.ActionsRow{
			Components: row2,
		},
	}

	if config.IsDevBot() {
		row3 := []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Test Contract",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("watch-test-contract#%s", userID),
			},
			discordgo.Button{
				Label:    "Test Colleggtible",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("watch-test-colleggtible#%s", userID),
			},
		}
		components = append(components, discordgo.ActionsRow{
			Components: row3,
		})
	}

	if interaction.Type == discordgo.InteractionMessageComponent {
		_, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content:    &content,
			Components: &components,
		})
		if err != nil {
			log.Printf("watch: InteractionResponseEdit error: %v", err)
		}
	} else {
		_, err := s.FollowupMessageCreate(interaction, true, &discordgo.WebhookParams{
			Content:    content,
			Components: components,
		})
		if err != nil {
			log.Printf("watch: FollowupMessageCreate error: %v", err)
		}
	}
}

// HandlePage handles watch status pagination clicks.
func HandlePage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 3 {
		return
	}
	userID := parts[1]
	page, err := strconv.Atoi(parts[2])
	if err != nil {
		log.Printf("watch: HandlePage Atoi error: %v", err)
		return
	}

	// Ensure clicking user is the owner
	clickerID := bottools.GetInteractionUserID(i)
	if clickerID != userID {
		log.Printf("watch: HandlePage clicker ID mismatch: clicker=%s, owner=%s", clickerID, userID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only interact with your own watch status pages.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	renderStatusPage(s, i.Interaction, userID, page, false)
}

// HandleToggleSort handles sorting toggle clicks.
func HandleToggleSort(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 3 {
		return
	}
	userID := parts[1]

	clickerID := bottools.GetInteractionUserID(i)
	if clickerID != userID {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only interact with your own watch status pages.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	sortStyle := farmerstate.GetMiscSettingString(userID, "watch_sort")
	if sortStyle == "alpha" {
		farmerstate.SetMiscSettingString(userID, "watch_sort", "predicted")
	} else {
		farmerstate.SetMiscSettingString(userID, "watch_sort", "alpha")
	}

	renderStatusPage(s, i.Interaction, userID, 0, false)
}

// HandleClearConfirm handles transitioning clear all watches to confirmation state.
func HandleClearConfirm(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 3 {
		return
	}
	userID := parts[1]
	page, err := strconv.Atoi(parts[2])
	if err != nil {
		page = 0
	}

	clickerID := bottools.GetInteractionUserID(i)
	if clickerID != userID {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only clear your own watches.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	renderStatusPage(s, i.Interaction, userID, page, true)
}

// HandleClear handles clearing all watches for a user.
func HandleClear(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 2 {
		return
	}
	userID := parts[1]

	clickerID := bottools.GetInteractionUserID(i)
	if clickerID != userID {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only clear your own watches.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	farmerstate.DeleteUserWatches(userID)
	renderStatusPage(s, i.Interaction, userID, 0, false)
}

// HandleDismiss handles dismissing a watch DM message.
func HandleDismiss(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	_ = s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
}

// HandleKeep handles keeping a watch DM message (removes the buttons).
func HandleKeep(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    i.Message.Content,
			Components: []discordgo.MessageComponent{},
		},
	})
}

// CheckWatches is called periodically to check watches and send DM notifications if matches are found.
func CheckWatches(s *discordgo.Session) {
	watches := farmerstate.GetAllWatches()
	if len(watches) == 0 {
		return
	}

	newCollMutex.Lock()
	newEggs := make(map[string]bool)
	for _, id := range newColleggtibles {
		newEggs[id] = true
	}
	newColleggtibles = nil // Reset list
	newCollMutex.Unlock()

	// Group matches by user to prevent spamming if multiple matches occur
	type notification struct {
		userID     string
		watchType  string
		targetID   string
		contractID string
	}

	var matches []notification

	for _, w := range watches {
		if w.WatchType == WatchTypeContract {
			// Check if this contract is active
			active := false
			for _, c := range ei.EggIncContracts {
				if c.ID == w.TargetID && !c.Predicted {
					active = true
					break
				}
			}
			if active {
				matches = append(matches, notification{
					userID:     w.UserID,
					watchType:  w.WatchType,
					targetID:   w.TargetID,
					contractID: w.TargetID,
				})
			}
		} else if w.WatchType == WatchTypeColleggtible {
			if w.TargetID == "new" {
				// Check if any active contract offers a new custom egg
				for _, c := range ei.EggIncContracts {
					if c.Predicted {
						continue
					}
					if newEggs[c.EggName] {
						matches = append(matches, notification{
							userID:     w.UserID,
							watchType:  w.WatchType,
							targetID:   w.TargetID,
							contractID: c.ID,
						})
					}
				}
			} else {
				// Check if any active contract offers this custom egg
				for _, c := range ei.EggIncContracts {
					if c.Predicted {
						continue
					}
					if c.EggName == w.TargetID {
						matches = append(matches, notification{
							userID:     w.UserID,
							watchType:  w.WatchType,
							targetID:   w.TargetID,
							contractID: c.ID,
						})
						break
					}
				}
			}
		}
	}

	for _, m := range matches {
		// Get output layout with legendary set info
		estimateText := boost.GetContractEstimateString(m.contractID, true)

		// Create DM channel
		channel, err := s.UserChannelCreate(m.userID)
		if err != nil {
			log.Printf("watch: failed to create DM channel for user %s: %v", m.userID, err)
			continue
		}

		// Send DM with Dismiss and Keep buttons
		_, err = s.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
			Content: estimateText,
			Flags:   discordgo.MessageFlagsSuppressEmbeds,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Dismiss",
							Style:    discordgo.DangerButton,
							CustomID: "watch-dismiss",
						},
						discordgo.Button{
							Label:    "Keep",
							Style:    discordgo.SuccessButton,
							CustomID: "watch-keep",
						},
					},
				},
			},
		})
		if err != nil {
			log.Printf("watch: failed to send DM message to user %s: %v", m.userID, err)
			// Do not clear the watch if DM fails so we can retry next time, or should we?
			// Let's keep it so they can receive it when their DMs open, or delete it to avoid stuck watches.
			// The requirement says: "and then the watch for that item is cleared." So let's clear it.
		}

		// Clear the watch from DB (unless it is a persistent "new" colleggtible watch)
		if m.targetID != "new" {
			farmerstate.DeleteWatch(m.userID, m.watchType, m.targetID)
		}
	}
}

// HandleTestContract triggers a mock DM notification for a contract.
func HandleTestContract(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 2 {
		return
	}
	userID := parts[1]

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Testing contract watch DM notification...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	contractID := "first-contract"
	for _, c := range ei.EggIncContracts {
		if !c.Predicted {
			contractID = c.ID
			break
		}
	}

	estimateText := boost.GetContractEstimateString(contractID, true)

	channel, err := s.UserChannelCreate(userID)
	if err == nil {
		_, _ = s.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
			Content: estimateText,
			Flags:   discordgo.MessageFlagsSuppressEmbeds,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Dismiss",
							Style:    discordgo.DangerButton,
							CustomID: "watch-dismiss",
						},
						discordgo.Button{
							Label:    "Keep",
							Style:    discordgo.SuccessButton,
							CustomID: "watch-keep",
						},
					},
				},
			},
		})
	}
}

// HandleTestColleggtible triggers a mock DM notification for a colleggtible.
func HandleTestColleggtible(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 2 {
		return
	}
	userID := parts[1]

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Testing colleggtible watch DM notification...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	contractID := "first-contract"
	for _, c := range ei.EggIncContracts {
		if c.Predicted {
			continue
		}
		if _, ok := ei.CustomEggMap[c.EggName]; ok {
			contractID = c.ID
			break
		}
	}

	estimateText := boost.GetContractEstimateString(contractID, true)

	channel, err := s.UserChannelCreate(userID)
	if err == nil {
		_, _ = s.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
			Content: estimateText,
			Flags:   discordgo.MessageFlagsSuppressEmbeds,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Dismiss",
							Style:    discordgo.DangerButton,
							CustomID: "watch-dismiss",
						},
						discordgo.Button{
							Label:    "Keep",
							Style:    discordgo.SuccessButton,
							CustomID: "watch-keep",
						},
					},
				},
			},
		})
	}
}
