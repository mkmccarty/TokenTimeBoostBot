package boost

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	tokenEditHandlerPrefix = "token_edit"
	tokenEditSessionTTL    = 15 * time.Minute
	tokenEditPageSize      = 10
	tokenEditReceiverPage  = 25
	tokenEditMinQuantity   = 1
	tokenEditMaxQuantity   = 32
)

type tokenEditOpKind int

const (
	tokenEditOpMove tokenEditOpKind = iota
	tokenEditOpQuantity
	tokenEditOpDelete
)

type tokenEditOp struct {
	kind         tokenEditOpKind
	entryKey     string
	prevToUserID string
	prevToNick   string
	prevQuantity int
	prevDeleted  bool
}

type tokenEditEntry struct {
	key       string
	origIndex int
	token     ei.TokenUnitLog
	orig      ei.TokenUnitLog
	deleted   bool
}

func (e *tokenEditEntry) isModified() bool {
	if e.deleted {
		return true
	}
	return e.token.ToUserID != e.orig.ToUserID ||
		e.token.ToNick != e.orig.ToNick ||
		e.token.Quantity != e.orig.Quantity
}

type tokenEditSession struct {
	mu              sync.Mutex
	uuidStr         string
	contractHash    string
	channelID       string
	userID          string
	effectiveUserID string
	isCoordinator   bool
	showAll         bool
	entries         []*tokenEditEntry
	entriesByKey    map[string]*tokenEditEntry
	undoStack       []tokenEditOp
	page            int
	selectedKey     string
	receiverPage    int
	expiresAt       time.Time
}

var (
	tokenEditSessions      = make(map[string]*tokenEditSession)
	tokenEditSessionsMutex sync.Mutex
)

// GetSlashTokenEditCommand returns the slash command definition for /token-edit.
func GetSlashTokenEditCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Interactive dialog to edit tracked tokens")
	return &command
}

// HandleTokenEditInteraction handles the /token-edit slash command, starting an interactive dialog session.
func HandleTokenEditInteraction(client dc.Client, e *dc.CommandEvent) {
	if e.GuildID() == "" {
		respondTokenEditCommand(e, "This command can only be run in a server.", nil)
		return
	}

	contract := FindContract(e.ChannelID())
	if contract == nil {
		respondTokenEditCommand(e, "Contract not found in this channel.", nil)
		return
	}

	userID := e.UserID()
	effectiveUserID := resolveContractUserID(client, contract, userID)
	isCoord := creatorOfContract(client, contract, userID)
	if !UserInContract(contract, effectiveUserID) && !UserInContract(contract, userID) && !isCoord {
		respondTokenEditCommand(e, "You are not in this contract.", nil)
		return
	}

	cleanupTokenEditSessions()
	clearTokenEditSessionsForUserContract(userID, contract.ContractHash)

	var entries []*tokenEditEntry
	entriesByKey := make(map[string]*tokenEditEntry)
	myTokenCount := 0

	contract.mutex.Lock()
	for i, t := range contract.TokenLog {
		key := t.Serial
		if key == "" {
			key = fmt.Sprintf("idx:%d", i)
		}
		if _, exists := entriesByKey[key]; exists {
			key = fmt.Sprintf("%s#%d", key, i)
		}
		entry := &tokenEditEntry{
			key:       key,
			origIndex: i,
			token:     t,
			orig:      t,
		}
		entries = append(entries, entry)
		entriesByKey[key] = entry
		if t.FromUserID == effectiveUserID || t.FromUserID == userID {
			myTokenCount++
		}
	}
	contract.mutex.Unlock()

	if len(entries) == 0 {
		respondTokenEditCommand(e, "There are no tracked tokens in this contract to edit.", nil)
		return
	}
	if !isCoord && myTokenCount == 0 {
		respondTokenEditCommand(e, "You have not sent any tracked tokens in this contract.", nil)
		return
	}

	session := &tokenEditSession{
		uuidStr:         uuid.NewV7().String(),
		contractHash:    contract.ContractHash,
		channelID:       e.ChannelID(),
		userID:          userID,
		effectiveUserID: effectiveUserID,
		isCoordinator:   isCoord,
		showAll:         isCoord && myTokenCount == 0,
		entries:         entries,
		entriesByKey:    entriesByKey,
		undoStack:       []tokenEditOp{},
		page:            0,
		selectedKey:     "",
		receiverPage:    0,
		expiresAt:       time.Now().Add(tokenEditSessionTTL),
	}

	tokenEditSessionsMutex.Lock()
	tokenEditSessions[session.uuidStr] = session
	tokenEditSessionsMutex.Unlock()

	content, components := renderTokenEditDialog(contract, session, "")
	respondTokenEditCommand(e, content, components)
}

// HandleTokenEditReactions handles button and select menu interactions for the /token-edit dialog.
func HandleTokenEditReactions(client dc.Client, e *dc.ComponentEvent) {
	cleanupTokenEditSessions()

	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 3 {
		respondTokenEditUpdate(e, "Token edit action was invalid. Please rerun `/token-edit`.", nil)
		return
	}

	uuidPart := parts[1]
	action := parts[2]
	userID := e.UserID()

	tokenEditSessionsMutex.Lock()
	session, ok := tokenEditSessions[uuidPart]
	tokenEditSessionsMutex.Unlock()
	if !ok {
		respondTokenEditUpdate(e, "This token edit session expired. Please rerun `/token-edit`.", nil)
		return
	}

	effectiveUserID := farmerstate.GetEffectiveUserID(userID, e.MessageID(), e.ChannelID())
	if session.userID != userID && session.userID != effectiveUserID && farmerstate.GetEffectiveUserID(session.userID, e.MessageID(), e.ChannelID()) != userID {
		respondTokenEditUpdate(e, "Only the command caller can use this dialog.", nil)
		return
	}

	session.mu.Lock()

	if session.expiresAt.Before(time.Now()) {
		session.mu.Unlock()
		deleteTokenEditSession(session.uuidStr)
		respondTokenEditUpdate(e, "This token edit session expired. Please rerun `/token-edit`.", nil)
		return
	}
	session.expiresAt = time.Now().Add(tokenEditSessionTTL)

	contract := FindContractByHash(session.contractHash)
	if contract == nil {
		session.mu.Unlock()
		deleteTokenEditSession(session.uuidStr)
		respondTokenEditUpdate(e, "Unable to find this contract anymore. Dialog closed.", nil)
		return
	}

	if !session.isCoordinator && !UserInContract(contract, session.effectiveUserID) && !UserInContract(contract, userID) {
		session.mu.Unlock()
		deleteTokenEditSession(session.uuidStr)
		respondTokenEditUpdate(e, "You are no longer allowed to edit tokens in this contract.", nil)
		return
	}

	tokenEmoji := tokenEditEmoji(contract)
	status := ""
	switch action {
	case "select":
		values := e.Values()
		if len(values) == 0 {
			status = "No token selected."
			break
		}
		if _, exists := session.entriesByKey[values[0]]; !exists {
			status = "Selected token is no longer available."
			break
		}
		session.selectedKey = values[0]
		session.receiverPage = 0

	case "back":
		session.selectedKey = ""
		session.receiverPage = 0

	case "page":
		if len(parts) >= 4 {
			switch parts[3] {
			case "prev":
				if session.page > 0 {
					session.page--
				}
			case "next":
				session.page++
			}
		}

	case "rpage":
		if len(parts) >= 4 {
			switch parts[3] {
			case "prev":
				if session.receiverPage > 0 {
					session.receiverPage--
				}
			case "next":
				session.receiverPage++
			}
		}

	case "scope":
		if session.isCoordinator {
			session.showAll = !session.showAll
			session.page = 0
			session.selectedKey = ""
			session.receiverPage = 0
			if session.showAll {
				status = "Showing all contract tokens."
			} else {
				status = "Showing only your sent tokens."
			}
		}

	case "move":
		entry := session.entriesByKey[session.selectedKey]
		if entry == nil {
			session.selectedKey = ""
			status = "Selected token not found."
			break
		}
		if entry.deleted {
			status = "Restore the token before moving it."
			break
		}
		values := e.Values()
		if len(values) == 0 || values[0] == "" {
			status = "No receiver selected."
			break
		}
		targetID := values[0]
		var targetUserID, targetNick string
		var found bool
		contract.mutex.Lock()
		if b, exists := contract.Boosters[targetID]; exists && b != nil {
			targetUserID = b.UserID
			targetNick = b.Nick
			found = true
		}
		contract.mutex.Unlock()
		if !found {
			status = "Selected receiver is no longer in this contract."
			break
		}
		if entry.token.ToUserID == targetUserID {
			status = fmt.Sprintf("Token is already assigned to %s.", targetNick)
			break
		}
		session.undoStack = append(session.undoStack, tokenEditOp{
			kind:         tokenEditOpMove,
			entryKey:     entry.key,
			prevToUserID: entry.token.ToUserID,
			prevToNick:   entry.token.ToNick,
		})
		entry.token.ToUserID = targetUserID
		entry.token.ToNick = targetNick
		status = fmt.Sprintf("Staged %s move to %s.", tokenEmoji, targetNick)

	case "qty":
		entry := session.entriesByKey[session.selectedKey]
		if entry == nil {
			session.selectedKey = ""
			status = "Selected token not found."
			break
		}
		if entry.deleted {
			status = "Restore the token before changing quantity."
			break
		}
		if len(parts) < 4 {
			status = "Invalid quantity adjustment."
			break
		}
		delta, err := strconv.Atoi(parts[3])
		if err != nil {
			status = "Invalid quantity adjustment."
			break
		}
		newQty := entry.token.Quantity + delta
		if newQty < tokenEditMinQuantity {
			newQty = tokenEditMinQuantity
		}
		if newQty > tokenEditMaxQuantity {
			newQty = tokenEditMaxQuantity
		}
		if newQty == entry.token.Quantity {
			status = fmt.Sprintf("Quantity is already at %d.", newQty)
			break
		}
		session.undoStack = append(session.undoStack, tokenEditOp{
			kind:         tokenEditOpQuantity,
			entryKey:     entry.key,
			prevQuantity: entry.token.Quantity,
		})
		entry.token.Quantity = newQty
		status = fmt.Sprintf("Staged %s quantity update to %d.", tokenEmoji, newQty)

	case "delete":
		entry := session.entriesByKey[session.selectedKey]
		if entry == nil {
			session.selectedKey = ""
			status = "Selected token not found."
			break
		}
		session.undoStack = append(session.undoStack, tokenEditOp{
			kind:        tokenEditOpDelete,
			entryKey:    entry.key,
			prevDeleted: entry.deleted,
		})
		entry.deleted = !entry.deleted
		if entry.deleted {
			status = fmt.Sprintf("Staged %s deletion. Click Save & Close to apply.", tokenEmoji)
		} else {
			status = fmt.Sprintf("Restored %s.", tokenEmoji)
		}

	case "undo":
		status = tokenEditUndoLast(session, tokenEmoji)

	case "save":
		modifiedCount, deletedCount := applyTokenEditSession(contract, session)
		session.mu.Unlock()
		deleteTokenEditSession(session.uuidStr)
		if modifiedCount == 0 && deletedCount == 0 {
			respondTokenEditUpdate(e, "No changes to save.", nil)
			return
		}
		respondTokenEditUpdate(e, formatTokenEditSaveSummary(tokenEmoji, modifiedCount, deletedCount), nil)
		saveData(contract.ContractHash)
		refreshBoostListMessage(client, contract, false)
		return

	case "cancel":
		session.mu.Unlock()
		deleteTokenEditSession(session.uuidStr)
		respondTokenEditUpdate(e, "Exited without saving changes.", nil)
		return

	default:
		status = "Unknown token edit action."
	}

	content, components := renderTokenEditDialog(contract, session, status)
	session.mu.Unlock()
	respondTokenEditUpdate(e, content, components)
}

func tokenEditEmoji(contract *Contract) string {
	if contract != nil && contract.TokenStr != "" {
		return contract.TokenStr
	}
	if emoji := ei.GetBotEmojiMarkdown("token"); emoji != "" {
		return emoji
	}
	return "token"
}

func tokenEditUndoLast(session *tokenEditSession, tokenEmoji string) string {
	if len(session.undoStack) == 0 {
		return "Nothing to undo."
	}
	op := session.undoStack[len(session.undoStack)-1]
	session.undoStack = session.undoStack[:len(session.undoStack)-1]
	entry := session.entriesByKey[op.entryKey]
	if entry == nil {
		return "Undid last change."
	}
	switch op.kind {
	case tokenEditOpMove:
		entry.token.ToUserID = op.prevToUserID
		entry.token.ToNick = op.prevToNick
		return fmt.Sprintf("Undid %s move (restored receiver to %s).", tokenEmoji, op.prevToNick)
	case tokenEditOpQuantity:
		entry.token.Quantity = op.prevQuantity
		return fmt.Sprintf("Undid %s quantity change (restored to %d).", tokenEmoji, op.prevQuantity)
	case tokenEditOpDelete:
		entry.deleted = op.prevDeleted
		if entry.deleted {
			return fmt.Sprintf("Undid %s restore (marked for deletion).", tokenEmoji)
		}
		return fmt.Sprintf("Undid %s deletion.", tokenEmoji)
	}
	return "Undid last change."
}

// applyTokenEditSession commits staged edits from session into contract.TokenLog
// by matching on Serial (or original index for legacy entries without a serial),
// preserving any new tokens logged while the dialog was open.
func applyTokenEditSession(contract *Contract, session *tokenEditSession) (modifiedCount int, deletedCount int) {
	bySerial := make(map[string]*tokenEditEntry)
	byOrigIndex := make(map[int]*tokenEditEntry)
	for _, entry := range session.entries {
		if !entry.isModified() {
			continue
		}
		if entry.orig.Serial != "" {
			bySerial[entry.orig.Serial] = entry
		} else {
			byOrigIndex[entry.origIndex] = entry
		}
	}
	if len(bySerial) == 0 && len(byOrigIndex) == 0 {
		return 0, 0
	}

	contract.mutex.Lock()
	defer contract.mutex.Unlock()

	updated := make([]ei.TokenUnitLog, 0, len(contract.TokenLog))
	for i, t := range contract.TokenLog {
		var entry *tokenEditEntry
		if t.Serial != "" {
			entry = bySerial[t.Serial]
		} else {
			entry = byOrigIndex[i]
		}
		if entry == nil {
			updated = append(updated, t)
			continue
		}
		if entry.deleted {
			deletedCount++
			continue
		}
		t.ToUserID = entry.token.ToUserID
		t.ToNick = entry.token.ToNick
		t.Quantity = entry.token.Quantity
		t.Value = bottools.GetTokenValue(t.Time.Sub(contract.StartTime).Seconds(), contract.EstimatedDuration.Seconds()) * float64(t.Quantity)
		updated = append(updated, t)
		modifiedCount++
	}
	contract.TokenLog = updated
	calculateTokenValueCoopLog(contract, contract.EstimatedDuration)
	return modifiedCount, deletedCount
}

func formatTokenEditSaveSummary(tokenEmoji string, modifiedCount, deletedCount int) string {
	var parts []string
	if modifiedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d %s updated", modifiedCount, tokenEmoji))
	}
	if deletedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d %s deleted", deletedCount, tokenEmoji))
	}
	if len(parts) == 0 {
		return "No changes saved."
	}
	return fmt.Sprintf("Saved changes (%s) and redrawn boost list.", strings.Join(parts, ", "))
}

func tokenEditFilteredEntries(session *tokenEditSession) []*tokenEditEntry {
	filtered := make([]*tokenEditEntry, 0, len(session.entries))
	// Newest first so recent tokens appear on page 1.
	for i := len(session.entries) - 1; i >= 0; i-- {
		entry := session.entries[i]
		if session.showAll || entry.token.FromUserID == session.effectiveUserID || entry.token.FromUserID == session.userID {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func tokenEditPendingCount(session *tokenEditSession) int {
	count := 0
	for _, entry := range session.entries {
		if entry.isModified() {
			count++
		}
	}
	return count
}

func renderTokenEditDialog(contract *Contract, session *tokenEditSession, status string) (string, []dc.LayoutComponent) {
	if session.selectedKey != "" {
		if entry := session.entriesByKey[session.selectedKey]; entry != nil {
			return renderTokenEditDetailView(contract, session, entry, status)
		}
		session.selectedKey = ""
	}
	return renderTokenEditListView(contract, session, status)
}

func renderTokenEditListView(contract *Contract, session *tokenEditSession, status string) (string, []dc.LayoutComponent) {
	filtered := tokenEditFilteredEntries(session)
	pages := (len(filtered) + tokenEditPageSize - 1) / tokenEditPageSize
	if pages == 0 {
		session.page = 0
	} else if session.page >= pages {
		session.page = pages - 1
	}

	start := session.page * tokenEditPageSize
	end := min(start+tokenEditPageSize, len(filtered))
	var visible []*tokenEditEntry
	if start < len(filtered) {
		visible = filtered[start:end]
	}

	scopeLabel := "My Tokens"
	if session.showAll {
		scopeLabel = "All Tokens"
	}
	pendingCount := tokenEditPendingCount(session)

	var header strings.Builder
	fmt.Fprintf(&header, "### Token Edit — `%s/%s`\n", contract.ContractID, contract.CoopID)
	fmt.Fprintf(&header, "Scope: **%s** (%d total)", scopeLabel, len(filtered))
	if pages > 1 {
		fmt.Fprintf(&header, " • Page %d/%d", session.page+1, pages)
	}
	if pendingCount > 0 {
		fmt.Fprintf(&header, " • **%d pending change(s)**", pendingCount)
	}

	var listBody strings.Builder
	if len(visible) == 0 {
		listBody.WriteString("No tokens match the current filter.")
	} else {
		for idx, entry := range visible {
			num := start + idx + 1
			val := bottools.GetTokenValue(entry.token.Time.Sub(contract.StartTime).Seconds(), contract.EstimatedDuration.Seconds()) * float64(entry.token.Quantity)
			age := formatTokenEditAge(time.Since(entry.token.Time))
			line := fmt.Sprintf("`%s` %s → %s • x%d (%.3f)", age, entry.token.FromNick, entry.token.ToNick, entry.token.Quantity, val)
			if entry.deleted {
				fmt.Fprintf(&listBody, "%d. ~~%s~~ *(deleted)*\n", num, line)
			} else if entry.isModified() {
				fmt.Fprintf(&listBody, "%d. %s *(edited)*\n", num, line)
			} else {
				fmt.Fprintf(&listBody, "%d. %s\n", num, line)
			}
		}
	}

	components := []dc.LayoutComponent{
		dc.TextDisplay{Content: header.String()},
		dc.Separator{Divider: true, Spacing: dc.SeparatorSpacingSmall},
		dc.TextDisplay{Content: strings.TrimRight(listBody.String(), "\n")},
	}

	if len(visible) > 0 {
		options := make([]dc.SelectOption, 0, len(visible))
		for idx, entry := range visible {
			num := start + idx + 1
			age := formatTokenEditAge(time.Since(entry.token.Time))
			label := fmt.Sprintf("#%d %s -> %s (x%d)", num, entry.token.FromNick, entry.token.ToNick, entry.token.Quantity)
			desc := age
			if entry.deleted {
				desc = age + " • Marked for deletion"
			} else if entry.isModified() {
				desc = age + " • Edited"
			}
			options = append(options, dc.SelectOption{
				Label:       truncateTokenEditText(label, 100),
				Value:       entry.key,
				Description: truncateTokenEditText(desc, 100),
			})
		}
		components = append(components, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("%s#%s#select", tokenEditHandlerPrefix, session.uuidStr),
					Placeholder: "Select a token to edit...",
					Options:     options,
				},
			},
		})
	}

	if pages > 1 || session.isCoordinator {
		navButtons := make([]dc.InteractiveComponent, 0, 3)
		if pages > 1 {
			navButtons = append(navButtons,
				dc.Button{
					Label:    "Prev",
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("%s#%s#page#prev", tokenEditHandlerPrefix, session.uuidStr),
					Disabled: session.page == 0,
				},
				dc.Button{
					Label:    "Next",
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("%s#%s#page#next", tokenEditHandlerPrefix, session.uuidStr),
					Disabled: session.page >= pages-1,
				},
			)
		}
		if session.isCoordinator {
			toggleLabel := "Show All Tokens"
			if session.showAll {
				toggleLabel = "Show My Tokens"
			}
			navButtons = append(navButtons, dc.Button{
				Label:    toggleLabel,
				Style:    dc.ButtonPrimary,
				CustomID: fmt.Sprintf("%s#%s#scope", tokenEditHandlerPrefix, session.uuidStr),
			})
		}
		components = append(components, dc.ActionRow{Components: navButtons})
	}

	components = append(components, dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "Undo",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("%s#%s#undo", tokenEditHandlerPrefix, session.uuidStr),
				Disabled: len(session.undoStack) == 0,
			},
			dc.Button{
				Label:    "Save & Close",
				Style:    dc.ButtonSuccess,
				CustomID: fmt.Sprintf("%s#%s#save", tokenEditHandlerPrefix, session.uuidStr),
				Disabled: pendingCount == 0,
			},
			dc.Button{
				Label:    "Cancel",
				Style:    dc.ButtonDanger,
				CustomID: fmt.Sprintf("%s#%s#cancel", tokenEditHandlerPrefix, session.uuidStr),
			},
		},
	})

	if status != "" {
		components = append(components, dc.TextDisplay{Content: status})
	}

	return "", components
}

func renderTokenEditDetailView(contract *Contract, session *tokenEditSession, entry *tokenEditEntry, status string) (string, []dc.LayoutComponent) {
	val := bottools.GetTokenValue(entry.token.Time.Sub(contract.StartTime).Seconds(), contract.EstimatedDuration.Seconds()) * float64(entry.token.Quantity)
	age := formatTokenEditAge(time.Since(entry.token.Time))
	pendingCount := tokenEditPendingCount(session)

	stateStr := "Active"
	if entry.deleted {
		stateStr = "**Marked for deletion**"
	} else if entry.isModified() {
		stateStr = "**Edited** (unsaved)"
	}

	var details strings.Builder
	fmt.Fprintf(&details, "### Edit Token — `%s/%s`\n", contract.ContractID, contract.CoopID)
	fmt.Fprintf(&details, "**From:** %s\n", entry.token.FromNick)
	fmt.Fprintf(&details, "**To:** %s\n", entry.token.ToNick)
	fmt.Fprintf(&details, "**Quantity:** %d\n", entry.token.Quantity)
	fmt.Fprintf(&details, "**Sent:** %s (`TVal: %.3f`)\n", age, val)
	fmt.Fprintf(&details, "**Status:** %s", stateStr)

	components := []dc.LayoutComponent{
		dc.TextDisplay{Content: details.String()},
		dc.Separator{Divider: true, Spacing: dc.SeparatorSpacingSmall},
	}

	// Build sorted booster receiver list for Move select menu.
	type receiverChoice struct {
		userID string
		nick   string
	}
	contract.mutex.Lock()
	receivers := make([]receiverChoice, 0, len(contract.Boosters))
	for id, b := range contract.Boosters {
		if b == nil {
			continue
		}
		receivers = append(receivers, receiverChoice{userID: id, nick: b.Nick})
	}
	contract.mutex.Unlock()

	sort.Slice(receivers, func(i, j int) bool {
		li := strings.ToLower(receivers[i].nick)
		lj := strings.ToLower(receivers[j].nick)
		if li == lj {
			return receivers[i].userID < receivers[j].userID
		}
		return li < lj
	})

	rPages := (len(receivers) + tokenEditReceiverPage - 1) / tokenEditReceiverPage
	if rPages == 0 {
		session.receiverPage = 0
	} else if session.receiverPage >= rPages {
		session.receiverPage = rPages - 1
	}

	if len(receivers) > 0 {
		tokenEmoji := tokenEditEmoji(contract)
		rStart := session.receiverPage * tokenEditReceiverPage
		rEnd := min(rStart+tokenEditReceiverPage, len(receivers))
		pageReceivers := receivers[rStart:rEnd]
		options := make([]dc.SelectOption, 0, len(pageReceivers))
		for _, r := range pageReceivers {
			isCurrent := r.userID == entry.token.ToUserID
			desc := "Set as new receiver"
			if isCurrent {
				desc = "Current receiver"
			}
			options = append(options, dc.SelectOption{
				Label:       truncateTokenEditText(r.nick, 100),
				Value:       r.userID,
				Description: desc,
				Default:     isCurrent,
			})
		}
		receiverHeader := fmt.Sprintf("**Change %s Receiver:**", tokenEmoji)
		placeholder := "Select new token receiver..."
		if rPages > 1 {
			receiverHeader = fmt.Sprintf("**Change %s Receiver (Page %d/%d):**", tokenEmoji, session.receiverPage+1, rPages)
			placeholder = fmt.Sprintf("Select new token receiver (Page %d/%d)...", session.receiverPage+1, rPages)
		}
		components = append(components,
			dc.TextDisplay{Content: receiverHeader},
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.SelectMenu{
						CustomID:    fmt.Sprintf("%s#%s#move", tokenEditHandlerPrefix, session.uuidStr),
						Placeholder: placeholder,
						Disabled:    entry.deleted,
						Options:     options,
					},
				},
			},
		)
	}

	deleteLabel := "Delete"
	deleteStyle := dc.ButtonDanger
	if entry.deleted {
		deleteLabel = "Restore"
		deleteStyle = dc.ButtonSuccess
	}

	components = append(components, dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "-1",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("%s#%s#qty#-1", tokenEditHandlerPrefix, session.uuidStr),
				Disabled: entry.deleted || entry.token.Quantity <= tokenEditMinQuantity,
			},
			dc.Button{
				Label:    "+1",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("%s#%s#qty#1", tokenEditHandlerPrefix, session.uuidStr),
				Disabled: entry.deleted || entry.token.Quantity >= tokenEditMaxQuantity,
			},
			dc.Button{
				Label:    deleteLabel,
				Style:    deleteStyle,
				CustomID: fmt.Sprintf("%s#%s#delete", tokenEditHandlerPrefix, session.uuidStr),
			},
		},
	})

	if rPages > 1 {
		components = append(components, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{
					Label:    "Receivers Prev",
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("%s#%s#rpage#prev", tokenEditHandlerPrefix, session.uuidStr),
					Disabled: session.receiverPage == 0,
				},
				dc.Button{
					Label:    "Receivers Next",
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("%s#%s#rpage#next", tokenEditHandlerPrefix, session.uuidStr),
					Disabled: session.receiverPage >= rPages-1,
				},
			},
		})
	}

	components = append(components, dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "Back to List",
				Style:    dc.ButtonPrimary,
				CustomID: fmt.Sprintf("%s#%s#back", tokenEditHandlerPrefix, session.uuidStr),
			},
			dc.Button{
				Label:    "Undo",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("%s#%s#undo", tokenEditHandlerPrefix, session.uuidStr),
				Disabled: len(session.undoStack) == 0,
			},
			dc.Button{
				Label:    "Save & Close",
				Style:    dc.ButtonSuccess,
				CustomID: fmt.Sprintf("%s#%s#save", tokenEditHandlerPrefix, session.uuidStr),
				Disabled: pendingCount == 0,
			},
			dc.Button{
				Label:    "Cancel",
				Style:    dc.ButtonDanger,
				CustomID: fmt.Sprintf("%s#%s#cancel", tokenEditHandlerPrefix, session.uuidStr),
			},
		},
	})

	if status != "" {
		components = append(components, dc.TextDisplay{Content: status})
	}

	return "", components
}

func formatTokenEditAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	secs := int(d.Seconds())
	if secs < 60 {
		return fmt.Sprintf("%ds ago", secs)
	}
	mins := secs / 60
	if mins < 60 {
		return fmt.Sprintf("%dm ago", mins)
	}
	hours := mins / 60
	remMins := mins % 60
	if remMins == 0 {
		return fmt.Sprintf("%dh ago", hours)
	}
	return fmt.Sprintf("%dh%dm ago", hours, remMins)
}

func truncateTokenEditText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func respondTokenEditCommand(e *dc.CommandEvent, content string, components []dc.LayoutComponent) {
	if err := e.Respond(dc.Message{
		Content:    content,
		Ephemeral:  true,
		Components: components,
	}); err != nil {
		log.Printf("token-edit respond error: %v", err)
	}
}

func respondTokenEditUpdate(e *dc.ComponentEvent, content string, components []dc.LayoutComponent) {
	if len(components) == 0 && content != "" {
		components = []dc.LayoutComponent{dc.TextDisplay{Content: content}}
		content = ""
	}
	if err := e.Update(dc.Message{
		Content:    content,
		Ephemeral:  e.MessageIsEphemeral(),
		Components: components,
	}); err != nil {
		log.Printf("token-edit update error: %v", err)
	}
}

func cleanupTokenEditSessions() {
	now := time.Now()
	tokenEditSessionsMutex.Lock()
	defer tokenEditSessionsMutex.Unlock()
	for key, session := range tokenEditSessions {
		if session.expiresAt.Before(now) {
			delete(tokenEditSessions, key)
		}
	}
}

func clearTokenEditSessionsForUserContract(userID string, contractHash string) {
	tokenEditSessionsMutex.Lock()
	defer tokenEditSessionsMutex.Unlock()
	for key, session := range tokenEditSessions {
		if session.userID == userID && session.contractHash == contractHash {
			delete(tokenEditSessions, key)
		}
	}
}

func deleteTokenEditSession(uuidStr string) {
	tokenEditSessionsMutex.Lock()
	defer tokenEditSessionsMutex.Unlock()
	delete(tokenEditSessions, uuidStr)
}
