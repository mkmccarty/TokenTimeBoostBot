package boost

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/xid"
)

const (
	boostOrderHandlerPrefix = "bo_order"
	boostOrderSessionTTL    = 15 * time.Minute
	boostOrderPageSize      = 14
)

type boostOrderSession struct {
	xid                  string
	contractHash         string
	channelID            string
	userID               string
	commandName          string
	original             []string
	selected             []string
	undoSteps            []int
	page                 int
	expiresAt            time.Time
	changeCurrentBooster bool // Whether to reset current booster to first unboosted when saving
	selectionMode        int  // 0: Names, 1: Reverse, 2: Sort One, 3: Sort Fill
	bottomCount          int  // Track how many names were added to the bottom
}

var boostOrderSessions = make(map[string]*boostOrderSession)

// GetSlashBoostOrderCommand returns the definition of the /boost-order command.
func GetSlashBoostOrderCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name: cmd,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Description: "Interactive catalyst to reorder contract boost order",
	}
}

// HandleBoostOrderCommand handles the /boost-order command, starting an interactive session to reorder the boost order for a contract.
func HandleBoostOrderCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		respondBoostOrderCommand(s, i, "This command can only be run in a server.", nil)
		return
	}

	userID := getInteractionUserID(i)
	commandName := i.ApplicationCommandData().Name
	if commandName == "" {
		commandName = "boost-order"
	}
	contract := FindContract(i.ChannelID)
	if contract == nil {
		respondBoostOrderCommand(s, i, "Contract not found in this channel.", nil)
		return
	}

	if !creatorOfContract(s, contract, userID) {
		respondBoostOrderCommand(s, i, "Only coordinators or channel admins can change boost order.", nil)
		return
	}

	if !boostOrderHasReorderTargets(contract) {
		respondBoostOrderCommand(s, i, "There is nothing to reorder. All players have already boosted.", nil)
		return
	}

	cleanupBoostOrderSessions()
	clearBoostOrderSessionsForUserContract(userID, contract.ContractHash)

	session := &boostOrderSession{
		xid:                  xid.New().String(),
		contractHash:         contract.ContractHash,
		channelID:            i.ChannelID,
		userID:               userID,
		commandName:          commandName,
		original:             append([]string(nil), contract.Order...),
		selected:             boostOrderSeededSelection(contract),
		undoSteps:            []int{},
		page:                 0,
		expiresAt:            time.Now().Add(boostOrderSessionTTL),
		changeCurrentBooster: false, // Default to keeping current booster
		selectionMode:        0,
		bottomCount:          0,
	}
	boostOrderSessions[session.xid] = session

	content, components := renderBoostOrderInterview(contract, session, "")
	respondBoostOrderCommand(s, i, content, components)
}

// HandleBoostOrderReactions handles button interactions for the boost order catalyst, allowing the user to build a new boost order and save it.
func HandleBoostOrderReactions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	cleanupBoostOrderSessions()

	reaction := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(reaction) < 3 {
		respondBoostOrderUpdate(s, i, "Boost order catalyst path was invalid. Please rerun the command.", nil)
		return
	}

	xidPart := reaction[1]
	action := reaction[2]
	userID := getInteractionUserID(i)

	session, ok := boostOrderSessions[xidPart]
	if !ok {
		respondBoostOrderUpdate(s, i, "This catalyst session expired. Please rerun the command.", nil)
		return
	}

	if session.userID != userID {
		respondBoostOrderUpdate(s, i, "Only the command caller can use this catalyst.", nil)
		return
	}

	if session.expiresAt.Before(time.Now()) {
		delete(boostOrderSessions, session.xid)
		respondBoostOrderUpdate(s, i, fmt.Sprintf("This catalyst session expired. Please rerun %s.", boostOrderCommandPath(session.commandName)), nil)
		return
	}
	session.expiresAt = time.Now().Add(boostOrderSessionTTL)

	contract := FindContractByHash(session.contractHash)
	if contract == nil {
		delete(boostOrderSessions, session.xid)
		respondBoostOrderUpdate(s, i, "Unable to find this contract anymore. Catalyst closed.", nil)
		return
	}
	if !creatorOfContract(s, contract, userID) {
		delete(boostOrderSessions, session.xid)
		respondBoostOrderUpdate(s, i, "You are no longer allowed to edit this contract.", nil)
		return
	}

	status := ""
	switch action {
	case "pick":
		if len(reaction) < 4 {
			status = "No farmer selected."
			break
		}
		targetID := reaction[3]
		if !slices.Contains(session.original, targetID) {
			status = "Selected farmer is no longer available."
			break
		}
		if slices.Contains(session.selected, targetID) {
			status = "That farmer is already selected."
			break
		}
		if session.selectionMode == 1 {
			session.selected = slices.Insert(session.selected, len(session.selected)-session.bottomCount, targetID)
			session.bottomCount++
			session.undoSteps = append(session.undoSteps, -1)
		} else {
			session.selected = slices.Insert(session.selected, len(session.selected)-session.bottomCount, targetID)
			session.undoSteps = append(session.undoSteps, 1)
		}
	case "shift":
		unselected := boostOrderUnselected(session.original, session.selected)
		pages := boostOrderPages(len(unselected))
		if pages > 1 {
			session.page = (session.page + 1) % pages
		}
	case "fill":
		remaining := boostOrderUnselected(session.original, session.selected)
		if len(remaining) == 0 {
			status = "Nothing left to fill."
			break
		}
		session.selected = slices.Insert(session.selected, len(session.selected)-session.bottomCount, remaining...)
		session.undoSteps = append(session.undoSteps, len(remaining))
		status = "Filled remaining names in existing order."
	case "mode":
		session.selectionMode = (session.selectionMode + 1) % 4
	case "sortone", "sortfill":
		if len(reaction) < 4 {
			status = "Invalid sort action."
			break
		}
		sortType := reaction[3]
		unselected := boostOrderUnselected(session.original, session.selected)
		if len(unselected) == 0 {
			status = "No farmers left to sort."
			break
		}
		sorted := boostOrderSortRemaining(contract, unselected, sortType)
		if action == "sortone" {
			session.selected = slices.Insert(session.selected, len(session.selected)-session.bottomCount, sorted[0])
			session.undoSteps = append(session.undoSteps, 1)
			status = fmt.Sprintf("Added %s via %s.", boostOrderButtonLabel(contract, sorted[0]), strings.ToUpper(sortType))
		} else {
			session.selected = slices.Insert(session.selected, len(session.selected)-session.bottomCount, sorted...)
			session.undoSteps = append(session.undoSteps, len(sorted))
			status = fmt.Sprintf("Filled remaining %d farmers via %s.", len(sorted), strings.ToUpper(sortType))
		}
	case "undo":
		removedIDs, removedCount := boostOrderUndoLastStep(session)
		if removedCount == 1 && len(removedIDs) == 1 {
			status = fmt.Sprintf("Removed %s from the new order.", boostOrderMention(contract, removedIDs[0]))
		} else if removedCount > 1 {
			status = fmt.Sprintf("Removed previous fill (%d names).", removedCount)
		} else {
			status = "Nothing to undo."
		}
	case "reset":
		session.selected = []string{}
		session.undoSteps = []int{}
		session.page = 0
		session.selectionMode = 0
		session.bottomCount = 0
		status = "Catalyst reset."
	case "setkeepcurrent":
		session.changeCurrentBooster = false
		status = "✓ Current booster position will be preserved."
	case "setresetfirst":
		session.changeCurrentBooster = true
		status = "✓ Current booster will be reset to first unboosted."
	case "save":
		// Filter selected boosters to only include those still in the contract
		var validSelected []string
		for _, userID := range session.selected {
			if contract.Boosters[userID] != nil {
				validSelected = append(validSelected, userID)
			}
		}

		// Determine which original boosters are still in the contract
		var actualOriginal []string
		for _, userID := range session.original {
			if contract.Boosters[userID] != nil {
				actualOriginal = append(actualOriginal, userID)
			}
		}

		// Check if all current boosters are selected
		if len(validSelected) != len(actualOriginal) {
			status = fmt.Sprintf("Please select all farmers first (%d/%d selected).", len(validSelected), len(actualOriginal))
			break
		}

		previousCurrentBoosterID := contract.currentBoosterID()
		applyBoostOrderSelection(contract, validSelected, session.changeCurrentBooster)
		newCurrentBoosterID := contract.currentBoosterID()
		notifiedCurrentBoosterChange := false
		if previousCurrentBoosterID != newCurrentBoosterID && newCurrentBoosterID != "" && contract.Style&ContractFlagBanker == 0 {
			sendNextNotification(s, contract, true)
			notifiedCurrentBoosterChange = true
		}
		currentBoosterText := "none"
		if newCurrentBoosterID != "" {
			currentBoosterText = boostOrderMention(contract, newCurrentBoosterID)
		}
		changeText := fmt.Sprintf("Current booster remained: %s.", currentBoosterText)
		if previousCurrentBoosterID != newCurrentBoosterID {
			changeText = fmt.Sprintf("Current booster changed to: %s.", currentBoosterText)
		}
		saveData(contract.ContractHash)
		if !notifiedCurrentBoosterChange {
			refreshBoostListMessage(s, contract, false)
		}
		delete(boostOrderSessions, session.xid)
		respondBoostOrderUpdate(s, i, fmt.Sprintf("Boost order saved and contract redrawn. %s", changeText), []discordgo.MessageComponent{})
		return
	case "exit":
		delete(boostOrderSessions, session.xid)
		respondBoostOrderUpdate(s, i, "Exited without saving changes.", []discordgo.MessageComponent{})
		return
	default:
		status = "Unknown catalyst action."
	}

	content, components := renderBoostOrderInterview(contract, session, status)
	respondBoostOrderUpdate(s, i, content, components)
}

func respondBoostOrderCommand(s *discordgo.Session, i *discordgo.InteractionCreate, content string, components []discordgo.MessageComponent) {
	flags := discordgo.MessageFlagsEphemeral
	if boostOrderHasV2Components(components) {
		flags |= discordgo.MessageFlagsIsComponentsV2
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    content,
			Flags:      flags,
			Components: components,
		},
	})
}

func respondBoostOrderUpdate(s *discordgo.Session, i *discordgo.InteractionCreate, content string, components []discordgo.MessageComponent) {
	flags := discordgo.MessageFlags(0)
	if i != nil && i.Message != nil {
		if i.Message.Flags&discordgo.MessageFlagsEphemeral != 0 {
			flags |= discordgo.MessageFlagsEphemeral
		}
		if i.Message.Flags&discordgo.MessageFlagsIsComponentsV2 != 0 {
			flags |= discordgo.MessageFlagsIsComponentsV2
		}
	}
	if boostOrderHasV2Components(components) {
		flags |= discordgo.MessageFlagsIsComponentsV2
	}
	if flags&discordgo.MessageFlagsIsComponentsV2 != 0 && len(components) == 0 && content != "" {
		components = []discordgo.MessageComponent{&discordgo.TextDisplay{Content: content}}
		content = ""
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    content,
			Flags:      flags,
			Components: components,
		},
	})
}

func boostOrderHasV2Components(components []discordgo.MessageComponent) bool {
	for _, component := range components {
		switch component.(type) {
		case *discordgo.TextDisplay, *discordgo.Separator:
			return true
		}
	}
	return false
}

func boostOrderCommandPath(commandName string) string {
	if commandName == "" {
		return "/boost-order"
	}
	return "/" + commandName
}

func cleanupBoostOrderSessions() {
	now := time.Now()
	for key, session := range boostOrderSessions {
		if session.expiresAt.Before(now) {
			delete(boostOrderSessions, key)
		}
	}
}

func clearBoostOrderSessionsForUserContract(userID string, contractHash string) {
	for key, session := range boostOrderSessions {
		if session.userID == userID && session.contractHash == contractHash {
			delete(boostOrderSessions, key)
		}
	}
}

func renderBoostOrderInterview(contract *Contract, session *boostOrderSession, status string) (string, []discordgo.MessageComponent) {
	unselected := boostOrderUnselected(session.original, session.selected)
	sort.SliceStable(unselected, func(i, j int) bool {
		left := boostOrderSortKey(contract, unselected[i])
		right := boostOrderSortKey(contract, unselected[j])
		if left == right {
			return unselected[i] < unselected[j]
		}
		return left < right
	})
	pages := boostOrderPages(len(unselected))
	if pages == 0 {
		session.page = 0
	} else if session.page >= pages {
		session.page = 0
	}

	visible := boostOrderVisiblePage(unselected, session.page)
	headerText, currentText, boostedText, buildingText, instructionsText, footerText := buildBoostOrderTextSections(contract, session, len(unselected), pages)
	components := []discordgo.MessageComponent{
		&discordgo.TextDisplay{Content: headerText},
		&discordgo.TextDisplay{Content: currentText},
	}
	if boostedText != "" {
		components = append(components, boostOrderSeparatorComponent())
		components = append(components, &discordgo.TextDisplay{Content: boostedText})
	}
	components = append(components,
		&discordgo.TextDisplay{Content: buildingText},
		boostOrderSeparatorComponent(),
		&discordgo.TextDisplay{Content: instructionsText},
		boostOrderSeparatorComponent(),
	)
	components = append(components, boostOrderNameButtons(contract, session, visible)...)
	components = append(components, boostOrderControlButtons(contract, session, len(unselected), pages)...)
	components = append(components, &discordgo.TextDisplay{Content: footerText})
	if status != "" {
		components = append(components, &discordgo.TextDisplay{Content: status})
	}

	return "", components
}

func boostOrderNameButtons(contract *Contract, session *boostOrderSession, visible []string) []discordgo.MessageComponent {
	if len(visible) == 0 {
		return nil
	}

	components := make([]discordgo.MessageComponent, 0, 3)

	if session.selectionMode == 0 || session.selectionMode == 1 {
		var rowButtons []discordgo.MessageComponent
		for _, userID := range visible {
			if len(rowButtons) == 5 {
				components = append(components, discordgo.ActionsRow{Components: rowButtons})
				rowButtons = make([]discordgo.MessageComponent, 0, 5)
			}
			rowButtons = append(rowButtons, discordgo.Button{
				Label:    boostOrderButtonLabel(contract, userID),
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("%s#%s#pick#%s", boostOrderHandlerPrefix, session.xid, userID),
			})
		}
		if len(rowButtons) == 5 {
			components = append(components, discordgo.ActionsRow{Components: rowButtons})
			rowButtons = make([]discordgo.MessageComponent, 0, 5)
		}
		modeLabel := "Mode: Forward"
		if session.selectionMode == 1 {
			modeLabel = "Mode: Reverse"
		}
		rowButtons = append(rowButtons, discordgo.Button{
			Label:    modeLabel,
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("%s#%s#mode", boostOrderHandlerPrefix, session.xid),
		})
		components = append(components, discordgo.ActionsRow{Components: rowButtons})
	} else {
		limit := min(len(visible), 10)
		var rowButtons []discordgo.MessageComponent
		for i := 0; i < limit; i++ {
			if len(rowButtons) == 5 {
				components = append(components, discordgo.ActionsRow{Components: rowButtons})
				rowButtons = make([]discordgo.MessageComponent, 0, 5)
			}
			rowButtons = append(rowButtons, discordgo.Button{
				Label:    boostOrderButtonLabel(contract, visible[i]),
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("%s#%s#pick#%s", boostOrderHandlerPrefix, session.xid, visible[i]),
			})
		}
		if len(rowButtons) > 0 {
			components = append(components, discordgo.ActionsRow{Components: rowButtons})
		}

		modeLabel := "Mode: Sort One"
		sortAction := "sortone"
		if session.selectionMode == 3 {
			modeLabel = "Mode: Sort Fill"
			sortAction = "sortfill"
		}

		sortRow := discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{Label: "Next TE", Style: discordgo.SuccessButton, CustomID: fmt.Sprintf("%s#%s#%s#te", boostOrderHandlerPrefix, session.xid, sortAction)},
				discordgo.Button{Label: "Next TE+", Style: discordgo.SuccessButton, CustomID: fmt.Sprintf("%s#%s#%s#teplus", boostOrderHandlerPrefix, session.xid, sortAction)},
				discordgo.Button{Label: "Next ELR", Style: discordgo.SuccessButton, CustomID: fmt.Sprintf("%s#%s#%s#elr", boostOrderHandlerPrefix, session.xid, sortAction)},
				discordgo.Button{Label: "Random", Style: discordgo.SuccessButton, CustomID: fmt.Sprintf("%s#%s#%s#random", boostOrderHandlerPrefix, session.xid, sortAction)},
				discordgo.Button{Label: modeLabel, Style: discordgo.SecondaryButton, CustomID: fmt.Sprintf("%s#%s#mode", boostOrderHandlerPrefix, session.xid)},
			},
		}
		components = append(components, sortRow)
	}

	return components
}

func boostOrderControlButtons(contract *Contract, session *boostOrderSession, unselectedCount int, pages int) []discordgo.MessageComponent {
	controls := make([]discordgo.MessageComponent, 0, 5)
	if unselectedCount > boostOrderPageSize {
		controls = append(controls, discordgo.Button{
			Label:    "Shift",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("%s#%s#shift", boostOrderHandlerPrefix, session.xid),
		})
	} else {
		controls = append(controls, discordgo.Button{
			Label:    "Fill",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("%s#%s#fill", boostOrderHandlerPrefix, session.xid),
			Disabled: unselectedCount == 0,
		})
	}

	// Calculate how many original boosters are still in the contract
	var actualOriginalCount int
	for _, userID := range session.original {
		if contract.Boosters[userID] != nil {
			actualOriginalCount++
		}
	}

	// Add preference buttons when the order is full (order complete)
	var toggleComponents []discordgo.MessageComponent
	if unselectedCount == 0 {
		keepLabel := "Keep current booster"
		resetLabel := "Reset to first unboosted"
		if !session.changeCurrentBooster {
			keepLabel = "✓ Keep current booster"
		} else {
			resetLabel = "✓ Reset to first unboosted"
		}
		toggleComponents = append(toggleComponents, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    keepLabel,
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("%s#%s#setkeepcurrent", boostOrderHandlerPrefix, session.xid),
				},
				discordgo.Button{
					Label:    resetLabel,
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("%s#%s#setresetfirst", boostOrderHandlerPrefix, session.xid),
				},
			},
		})
	}

	controls = append(controls,
		discordgo.Button{
			Label:    "Undo",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("%s#%s#undo", boostOrderHandlerPrefix, session.xid),
			Disabled: len(session.undoSteps) == 0,
		},
		discordgo.Button{
			Label:    "Reset",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("%s#%s#reset", boostOrderHandlerPrefix, session.xid),
			Disabled: len(session.selected) == 0,
		},
		discordgo.Button{
			Label:    "Save",
			Style:    discordgo.SuccessButton,
			CustomID: fmt.Sprintf("%s#%s#save", boostOrderHandlerPrefix, session.xid),
			Disabled: len(session.selected) != actualOriginalCount,
		},
		discordgo.Button{
			Label:    "Exit",
			Style:    discordgo.DangerButton,
			CustomID: fmt.Sprintf("%s#%s#exit", boostOrderHandlerPrefix, session.xid),
		},
	)
	if pages <= 1 {
		session.page = 0
	}

	// Build final components: toggle first (if present), then control buttons
	var result []discordgo.MessageComponent
	if len(toggleComponents) > 0 {
		result = append(result, toggleComponents...)
	}
	if len(controls) > 0 {
		result = append(result, discordgo.ActionsRow{Components: controls})
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func buildBoostOrderTextSections(contract *Contract, session *boostOrderSession, unselectedCount int, pages int) (string, string, string, string, string, string) {
	if pages < 1 {
		pages = 1
	}

	currentSummary := boostOrderSummary(contract, session.original, 0)
	// Add rocket emoji to current booster in summary
	currentBoosterID := contract.currentBoosterID()
	if currentBoosterID != "" {
		currentSummary = strings.ReplaceAll(currentSummary, boostOrderMention(contract, currentBoosterID), "🚀 "+boostOrderMention(contract, currentBoosterID))
	}

	boostedSelection := boostOrderSeededSelection(contract)
	boostedSummary := boostOrderSummary(contract, boostedSelection, 0)
	buildingSelection := boostOrderExclude(session.selected, boostedSelection)

	var buildingItems []string
	insertIndex := len(buildingSelection) - session.bottomCount
	if insertIndex < 0 {
		insertIndex = 0
	}
	insertEmoji := "🔽"
	if session.selectionMode == 1 {
		insertEmoji = "🔼"
	}
	for idx, userID := range buildingSelection {
		if idx == insertIndex {
			buildingItems = append(buildingItems, insertEmoji)
		}
		buildingItems = append(buildingItems, fmt.Sprintf("%d:%s", idx+1, boostOrderMention(contract, userID)))
	}
	if insertIndex == len(buildingSelection) {
		buildingItems = append(buildingItems, insertEmoji)
	}
	selectedSummary := strings.Join(buildingItems, ", ")
	if selectedSummary == insertEmoji {
		selectedSummary = insertEmoji + " none"
	}

	buildingTarget := max(len(session.original)-len(boostedSelection), 0)

	headerText := "# Boost  Catalyst\n-# Precision sequencing for maximum velocity."
	currentText := fmt.Sprintf("**Current:** %s", currentSummary)

	// Add toggle preference info when order is complete
	if unselectedCount == 0 {
		var toggleNote string
		if session.changeCurrentBooster {
			toggleNote = " (on completion: reset to first unboosted)"
		} else {
			toggleNote = " (on completion: keep current in new position)"
		}
		currentText += toggleNote
	}

	boostedText := ""
	if boostedSummary != "" {
		boostedText = fmt.Sprintf("**Boosted:** %s", boostedSummary)
	}
	buildingText := fmt.Sprintf("**Reordered:** %s (%d/%d selected)", selectedSummary, len(buildingSelection), buildingTarget)

	instructionsText := boostOrderButtonsHint(unselectedCount)

	var footerBuilder strings.Builder
	fmt.Fprintf(&footerBuilder, "Available names: %d", unselectedCount)
	if unselectedCount > boostOrderPageSize {
		fmt.Fprintf(&footerBuilder, " (page %d/%d)", session.page+1, pages)
	}

	return headerText, currentText, boostedText, buildingText, instructionsText, footerBuilder.String()
}

func boostOrderSeparatorComponent() *discordgo.Separator {
	divider := true
	spacing := discordgo.SeparatorSpacingSizeSmall
	return &discordgo.Separator{
		Divider: &divider,
		Spacing: &spacing,
	}
}

func boostOrderButtonsHint(unselectedCount int) string {
	moveHint := "Fill adds all remaining names in current order"
	if unselectedCount > boostOrderPageSize {
		moveHint = "Shift cycles name pages"
	}
	return fmt.Sprintf("-# Buttons: select names to build order. Toggle Mode for sorting/reverse.\n-# %s.\n-# Undo reverts the last action, Reset clears, Save applies, Exit closes.", moveHint)
}

func boostOrderSummary(contract *Contract, ordered []string, limit int) string {
	if len(ordered) == 0 {
		return ""
	}
	if limit <= 0 || limit > len(ordered) {
		limit = len(ordered)
	}
	items := make([]string, 0, limit)
	for idx, userID := range ordered {
		if idx >= limit {
			break
		}
		items = append(items, fmt.Sprintf("%d:%s", idx+1, boostOrderMention(contract, userID)))
	}
	return strings.Join(items, ", ")
}

func boostOrderMention(contract *Contract, userID string) string {
	if contract != nil && contract.Boosters[userID] != nil {
		return contract.Boosters[userID].Mention
	}
	return fmt.Sprintf("<@%s>", userID)
}

func boostOrderButtonLabel(contract *Contract, userID string) string {
	label := userID
	metric := ""
	if contract != nil && contract.Boosters[userID] != nil {
		booster := contract.Boosters[userID]
		if contract.Boosters[userID].Nick != "" {
			label = contract.Boosters[userID].Nick
		} else if contract.Boosters[userID].Name != "" {
			label = contract.Boosters[userID].Name
		} else if contract.Boosters[userID].Mention != "" {
			label = contract.Boosters[userID].Mention
		}

		if contract.BoostOrder == ContractOrderELR {
			metric = fmt.Sprintf("(ELR:%0.2f)", booster.ArtifactSet.LayRate)
		} else if booster.TECount > 0 {
			metric = fmt.Sprintf("(TE:%d)", booster.TECount)
		}
	}

	maxLabelLen := 80
	maxNameLen := maxLabelLen
	if metric != "" {
		maxNameLen = max(6, maxLabelLen-len(metric)-1)
	}
	if len(label) > maxNameLen {
		label = label[:maxNameLen]
	}

	if metric != "" {
		label = label + " " + metric
	}
	return label
}

func boostOrderSortKey(contract *Contract, userID string) string {
	if contract != nil && contract.Boosters[userID] != nil {
		booster := contract.Boosters[userID]
		switch {
		case booster.Nick != "":
			return strings.ToLower(booster.Nick)
		case booster.Name != "":
			return strings.ToLower(booster.Name)
		case booster.Mention != "":
			return strings.ToLower(booster.Mention)
		}
	}
	return strings.ToLower(userID)
}

func boostOrderUnselected(original []string, selected []string) []string {
	if len(original) == 0 {
		return nil
	}
	used := make(map[string]struct{}, len(selected))
	for _, userID := range selected {
		used[userID] = struct{}{}
	}
	remaining := make([]string, 0, len(original)-len(selected))
	for _, userID := range original {
		if _, ok := used[userID]; !ok {
			remaining = append(remaining, userID)
		}
	}
	return remaining
}

func boostOrderExclude(values []string, excludes []string) []string {
	if len(values) == 0 {
		return nil
	}
	if len(excludes) == 0 {
		return append([]string(nil), values...)
	}

	excludeSet := make(map[string]struct{}, len(excludes))
	for _, value := range excludes {
		excludeSet[value] = struct{}{}
	}

	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := excludeSet[value]; !exists {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func boostOrderUndoLastStep(session *boostOrderSession) ([]string, int) {
	if session == nil || len(session.undoSteps) == 0 {
		return nil, 0
	}
	step := session.undoSteps[len(session.undoSteps)-1]
	session.undoSteps = session.undoSteps[:len(session.undoSteps)-1]

	isBottom := step < 0
	count := step
	if isBottom {
		count = -step
	}

	if count <= 0 {
		return nil, 0
	}
	if count > len(session.selected) {
		count = len(session.selected)
	}

	var removedIDs []string
	if isBottom {
		startIndex := len(session.selected) - session.bottomCount
		if startIndex < 0 {
			startIndex = 0
		}
		endIndex := startIndex + count
		if endIndex > len(session.selected) {
			endIndex = len(session.selected)
		}
		removedIDs = append([]string(nil), session.selected[startIndex:endIndex]...)
		session.selected = slices.Delete(session.selected, startIndex, endIndex)
		session.bottomCount -= count
		if session.bottomCount < 0 {
			session.bottomCount = 0
		}
	} else {
		endIndex := len(session.selected) - session.bottomCount
		if endIndex > len(session.selected) {
			endIndex = len(session.selected)
		}
		startIndex := endIndex - count
		if startIndex < 0 {
			startIndex = 0
		}
		removedIDs = append([]string(nil), session.selected[startIndex:endIndex]...)
		session.selected = slices.Delete(session.selected, startIndex, endIndex)
	}
	return removedIDs, count
}

func boostOrderVisiblePage(unselected []string, page int) []string {
	if len(unselected) == 0 {
		return nil
	}
	pages := boostOrderPages(len(unselected))
	if pages < 1 {
		return nil
	}
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = page % pages
	}
	start := page * boostOrderPageSize
	end := min(start+boostOrderPageSize, len(unselected))
	if start >= len(unselected) {
		return nil
	}
	return unselected[start:end]
}

func boostOrderPages(count int) int {
	if count <= 0 {
		return 0
	}
	return (count + boostOrderPageSize - 1) / boostOrderPageSize
}

func applyBoostOrderSelection(contract *Contract, selected []string, changeCurrentBooster bool) {
	newOrder := append([]string(nil), selected...)

	contract.mutex.Lock()
	previousCurrent := contract.currentBoosterID()
	contract.Order = newOrder
	contract.OrderRevision++

	if contract.State != ContractStateSignup {
		if changeCurrentBooster {
			// Reset to first unboosted, or keep previous if still eligible, or clear
			nextID := boostOrderFirstNotBoostedID(contract)
			if nextID != "" {
				contract.setCurrentBoosterByUserIDWithStart(nextID)
			} else if previousCurrent != "" && slices.Contains(contract.Order, previousCurrent) {
				contract.setCurrentBoosterByUserIDWithStart(previousCurrent)
			} else {
				contract.clearCurrentBooster()
			}
		} else {
			// Keep current booster in their new position (if still in order)
			if previousCurrent != "" && slices.Contains(contract.Order, previousCurrent) {
				contract.setCurrentBoosterByUserIDWithStart(previousCurrent)
			} else {
				// Current booster was removed, fall back to first unboosted
				nextID := boostOrderFirstNotBoostedID(contract)
				if nextID != "" {
					contract.setCurrentBoosterByUserIDWithStart(nextID)
				} else {
					contract.clearCurrentBooster()
				}
			}
		}
		// Enforce that only current booster has BoostStateTokenTime
		contract.enforceOnlyOneTokenTimeBooster()
	} else {
		contract.BoostOrder = ContractManualOrder
	}
	contract.mutex.Unlock()
}

func boostOrderFirstNotBoostedID(contract *Contract) string {
	for _, userID := range contract.Order {
		booster := contract.Boosters[userID]
		if booster == nil {
			continue
		}
		if booster.BoostState == BoostStateUnboosted || booster.BoostState == BoostStateTokenTime {
			return userID
		}
	}
	return ""
}

func boostOrderSeededSelection(contract *Contract) []string {
	if contract == nil || contract.State == ContractStateSignup {
		return []string{}
	}

	seeded := make([]string, 0, len(contract.Order))
	for _, userID := range contract.Order {
		booster := contract.Boosters[userID]
		if booster == nil {
			continue
		}
		if booster.BoostState == BoostStateBoosted {
			seeded = append(seeded, userID)
		}
	}
	return seeded
}

func boostOrderHasReorderTargets(contract *Contract) bool {
	if contract == nil {
		return false
	}
	seeded := boostOrderSeededSelection(contract)
	remaining := boostOrderUnselected(contract.Order, seeded)
	return len(remaining) > 0
}

func boostOrderSortRemaining(contract *Contract, unselected []string, sortType string) []string {
	sorted := append([]string(nil), unselected...)
	switch sortType {
	case "random":
		rand.Shuffle(len(sorted), func(i, j int) {
			sorted[i], sorted[j] = sorted[j], sorted[i]
		})
	case "elr":
		sort.SliceStable(sorted, func(i, j int) bool {
			elrI, elrJ := 0.0, 0.0
			if b := contract.Boosters[sorted[i]]; b != nil {
				elrI = b.ArtifactSet.LayRate
			}
			if b := contract.Boosters[sorted[j]]; b != nil {
				elrJ = b.ArtifactSet.LayRate
			}
			return elrI > elrJ
		})
	case "te", "teplus":
		type tePair struct {
			name string
			te   float64
		}
		pairs := make([]tePair, len(sorted))
		for i, name := range sorted {
			baseTE := 0.0
			if b := contract.Boosters[name]; b != nil {
				baseTE = float64(max(b.TECount, 0))
			}
			sortTE := baseTE
			if sortType == "teplus" {
				randomBonusMax := math.Max(baseTE*0.1, math.Sqrt(baseTE))
				randomOffset := (rand.Float64()*2 - 1) * randomBonusMax
				sortTE = baseTE + randomOffset
			}
			pairs[i] = tePair{name: name, te: sortTE}
		}
		sort.SliceStable(pairs, func(i, j int) bool {
			return pairs[i].te > pairs[j].te
		})
		for i, p := range pairs {
			sorted[i] = p.name
		}
	}
	return sorted
}
