package leaderboard

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

const discordMessageCharLimit = 1900
const leaderboardUpdateConfirmationTTL = 1 * time.Minute

func targetMemberSet(target string) map[string]struct{} {
	if target == "" {
		return nil
	}
	m := make(map[string]struct{})
	for _, k := range ExpandConfigKey(target) {
		m[k] = struct{}{}
	}
	return m
}

func intersectsTarget(memberKeys []string, targetSet map[string]struct{}) bool {
	if len(targetSet) == 0 {
		return true
	}
	for _, k := range memberKeys {
		if _, ok := targetSet[k]; ok {
			return true
		}
	}
	return false
}

func shouldProcessMember(lbType string, targetSet map[string]struct{}) bool {
	if len(targetSet) == 0 {
		return true
	}
	_, ok := targetSet[lbType]
	return ok
}

// PostLeaderboards triggers the posting task for all configured guilds (or a specific guild if guildID is provided).
func PostLeaderboards(s *discordgo.Session, snapDate string, guildID string, target string, action string, onProgress func(string)) {
	var configs []LBConfig
	var err error
	if guildID != "" {
		configs, err = GetGuildLBConfigs(guildID)
	} else {
		configs, err = GetAllLBConfigs()
	}
	if err != nil {
		log.Printf("leaderboard: PostLeaderboards: failed to load configs: %v", err)
		return
	}

	targetSet := targetMemberSet(target)

	var filteredConfigs []LBConfig
	for _, cfg := range configs {
		if !intersectsTarget(ExpandConfigKey(cfg.LBType), targetSet) {
			continue
		}
		filteredConfigs = append(filteredConfigs, cfg)
	}
	configs = filteredConfigs

	guildConfigs := make(map[string][]LBConfig)
	var guildOrder []string
	for _, cfg := range configs {
		if len(guildConfigs[cfg.GuildID]) == 0 {
			guildOrder = append(guildOrder, cfg.GuildID)
		}
		guildConfigs[cfg.GuildID] = append(guildConfigs[cfg.GuildID], cfg)
	}

	guildIndex := 0
	for _, gid := range guildOrder {
		guildIndex++
		gc := guildConfigs[gid]

		if onProgress != nil {
			onProgress(fmt.Sprintf("📬 Posting leaderboards to guild %d/%d (%s)...", guildIndex, len(guildOrder), gid))
		}

		channelsUpdated := make(map[string]bool)
		for _, cfg := range gc {
			postOneLeaderboard(s, cfg, snapDate, targetSet, action, onProgress)
			channelsUpdated[cfg.ChannelID] = true
			time.Sleep(2 * time.Second) // Gap between configs to leave room for other bot activities
		}

		for channelID := range channelsUpdated {
			postChannelUpdateConfirmation(s, channelID)
		}
	}
	if onProgress != nil {
		onProgress("🏁 Weekly leaderboard update complete!")
	}
}

// postOneLeaderboard handles the expanded posting of a single config (which might be a group).
func postOneLeaderboard(s *discordgo.Session, cfg LBConfig, snapDate string, targetSet map[string]struct{}, action string, onProgress func(string)) {
	memberKeys := ExpandConfigKey(cfg.LBType)
	var newMsgIDs []string
	msgIDOffset := 0
	forceNewPosts := false

	switch action {
	case "bump":
		forceNewPosts = true
	case "new":
		forceNewPosts = true
		if len(targetSet) == 0 {
			cfg.MessageIDs = nil
		}
	}

	for _, lbType := range memberKeys {
		if !shouldProcessMember(lbType, targetSet) {
			existingID := ""
			if msgIDOffset < len(cfg.MessageIDs) {
				existingID = cfg.MessageIDs[msgIDOffset]
			}
			newMsgIDs = append(newMsgIDs, existingID)
			msgIDOffset++
			continue
		}

		def, ok := LBDefByKey(lbType)
		if !ok {
			log.Printf("leaderboard: unknown lb_type %q in group/config for guild %s", lbType, cfg.GuildID)
			if err := DeleteGuildLBConfig(cfg.GuildID, lbType); err != nil {
				log.Printf("leaderboard: failed to delete unknown lb_type %q for guild %s: %v", lbType, cfg.GuildID, err)
			} else {
				log.Printf("leaderboard: deleted unknown lb_type %q for guild %s", lbType, cfg.GuildID)
			}
			continue
		}

		if onProgress != nil && len(memberKeys) > 1 {
			onProgress(fmt.Sprintf("📬 Guild %s: Updating %s...", cfg.GuildID, def.DisplayName))
		}

		postSingleMetric(s, cfg, lbType, snapDate, &newMsgIDs, &msgIDOffset, &forceNewPosts)
		time.Sleep(1 * time.Second) // Conservative delay to allow room for concurrent bot activities
	}

	// Clean up any leftover/orphaned messages from the channel that were not reused.
	// For "new" action we intentionally keep old messages and only post fresh ones.
	if action != "new" {
		newMsgIDsMap := make(map[string]bool)
		for _, id := range newMsgIDs {
			if id != "" {
				newMsgIDsMap[id] = true
			}
		}
		for _, oldID := range cfg.MessageIDs {
			if oldID != "" && !newMsgIDsMap[oldID] {
				log.Printf("leaderboard: deleting orphaned message %s in channel %s", oldID, cfg.ChannelID)
				_ = s.ChannelMessageDelete(cfg.ChannelID, oldID)
			}
		}
	}

	UpdateGuildLBConfigMessageIDs(cfg.GuildID, cfg.LBType, newMsgIDs)
}

func postChannelUpdateConfirmation(s *discordgo.Session, channelID string) {
	content := "✅ Leaderboards updated."
	msg, err := s.ChannelMessageSend(channelID, content)
	if err != nil {
		log.Printf("leaderboard: failed to post channel update confirmation in %s: %v", channelID, err)
		return
	}

	go func(chID, msgID string) {
		time.Sleep(leaderboardUpdateConfirmationTTL)
		if err := s.ChannelMessageDelete(chID, msgID); err != nil {
			log.Printf("leaderboard: failed to auto-delete channel update confirmation %s in %s: %v", msgID, chID, err)
		}
	}(channelID, msg.ID)
}

func getGuildRows(lbType string, snapDate string, guildID string) ([]LBEntry, map[string]float64) {
	guildRows := GetLeaderboardRows(lbType, snapDate, guildID)
	if lbType == LBCXPWeeklyDelta {
		guildRows = buildWeeklyCSRows(snapDate, guildID)
	}
	prevSnapDate := GetPreviousSnapDate(lbType, snapDate)
	prevMap := make(map[string]float64)
	if prevSnapDate != "" {
		prevRows := GetLeaderboardRows(lbType, prevSnapDate, guildID)
		for _, r := range prevRows {
			prevMap[r.Player] = r.Value
		}
	}

	isShipLB := strings.HasPrefix(lbType, "ship_") || strings.HasPrefix(lbType, "std_ship_")
	var outRows []LBEntry
	for _, row := range guildRows {
		if !PlayerIsOptedIn(guildID, row.Player, lbType) {
			continue
		}

		if isShipLB && row.Value == 0 {
			continue
		}

		outRows = append(outRows, row)
	}
	return outRows, prevMap
}

func buildWeeklyCSRows(snapDate string, guildID string) []LBEntry {
	currentRows := GetLeaderboardRows(LBContractExp, snapDate, guildID)
	lookbackMap, hasLookback := findLookbackValueMap(LBContractExp, snapDate, guildID)

	out := make([]LBEntry, 0, len(currentRows))
	for _, r := range currentRows {
		delta := 0.0
		details := ""
		if hasLookback {
			if prev, ok := lookbackMap[r.Player]; ok {
				delta = r.Value - prev
			} else {
				details = "na"
			}
		} else {
			details = "na"
		}

		out = append(out, LBEntry{
			LBType:   LBCXPWeeklyDelta,
			Player:   r.Player,
			GameName: r.GameName,
			SnapDate: r.SnapDate,
			Value:    delta,
			Details:  details,
		})
	}

	// Weekly CS is a derived metric, so sort here by highest delta first.
	sort.Slice(out, func(i, j int) bool {
		iNA := out[i].Details == "na"
		jNA := out[j].Details == "na"
		if iNA != jNA {
			return !iNA
		}
		if out[i].Value == out[j].Value {
			return out[i].GameName < out[j].GameName
		}
		return out[i].Value > out[j].Value
	})

	return out
}

func findLookbackValueMap(lbType string, snapDate string, guildID string) (map[string]float64, bool) {
	out := make(map[string]float64)
	snapTime, err := time.Parse("2006-01-02", snapDate)
	if err != nil {
		return out, false
	}

	for daysBack := 7; daysBack >= 1; daysBack-- {
		candidateDate := snapTime.AddDate(0, 0, -daysBack).Format("2006-01-02")
		rows := GetLeaderboardRows(lbType, candidateDate, guildID)
		if len(rows) == 0 {
			continue
		}
		for _, r := range rows {
			out[r.Player] = r.Value
		}
		return out, true
	}

	return out, false
}

func postSingleMetric(s *discordgo.Session, cfg LBConfig, lbType, snapDate string, newMsgIDs *[]string, msgIDOffset *int, forceNewPosts *bool) {
	def, _ := LBDefByKey(lbType)
	guildRows, prevMap := getGuildRows(lbType, snapDate, cfg.GuildID)

	if len(guildRows) == 0 {
		log.Printf("leaderboard: no eligible guild members for %s in guild %s", lbType, cfg.GuildID)
		*newMsgIDs = append(*newMsgIDs, "")
		*msgIDOffset++
		return
	}

	pageSize := 50
	usePagination := len(guildRows) > pageSize
	page := 0

	displayRows := guildRows
	if usePagination {
		displayRows = guildRows[:pageSize]
	}

	blocks := buildMessageBlocks(def, displayRows, snapDate, prevMap, 0)

	// If using pagination, we enforce ONE message for the page.
	if usePagination && len(blocks) > 1 {
		// Truncate rows until it fits in one block?
		// For now, let's just use the first block and add buttons.
		blocks = blocks[:1]
	}

	for _, text := range blocks {
		var components []discordgo.MessageComponent
		if usePagination {
			components = []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Previous",
							Style:    discordgo.SecondaryButton,
							CustomID: fmt.Sprintf("lb_p#%s#%s#%d", lbType, snapDate, page-1),
							Disabled: true,
						},
						discordgo.Button{
							Label:    "Next",
							Style:    discordgo.SecondaryButton,
							CustomID: fmt.Sprintf("lb_p#%s#%s#%d", lbType, snapDate, page+1),
							Disabled: false,
						},
					},
				},
			}
		} else {
			components = []discordgo.MessageComponent{
				&discordgo.TextDisplay{Content: text},
			}
		}

		flags := discordgo.MessageFlagsIsComponentsV2
		if !*forceNewPosts && *msgIDOffset < len(cfg.MessageIDs) && cfg.MessageIDs[*msgIDOffset] != "" {
			msgID := cfg.MessageIDs[*msgIDOffset]
			edit := discordgo.MessageEdit{
				ID:         msgID,
				Channel:    cfg.ChannelID,
				Components: &components,
				Flags:      flags,
			}
			if _, err := s.ChannelMessageEditComplex(&edit); err != nil {
				log.Printf("leaderboard: failed to edit message %s: %v", msgID, err)
				if isChannelNotFound(err) {
					log.Printf("leaderboard: channel %s not found for guild %s - deleting config", cfg.ChannelID, cfg.GuildID)
					_ = DeleteGuildLBConfig(cfg.GuildID, cfg.LBType)
					return
				}
				if rerr, ok := err.(*discordgo.RESTError); ok && rerr.Message != nil && rerr.Message.Code == 10008 {
					// The message was orphaned/deleted. We must create a new one if the channel exists.
					*forceNewPosts = true
					if msg, err := sendNewLBMessage(s, cfg.ChannelID, components, flags); err == nil {
						*newMsgIDs = append(*newMsgIDs, msg.ID)
					} else if isChannelNotFound(err) {
						log.Printf("leaderboard: channel %s not found for guild %s - deleting config", cfg.ChannelID, cfg.GuildID)
						_ = DeleteGuildLBConfig(cfg.GuildID, cfg.LBType)
						return
					} else {
						log.Printf("leaderboard: failed to post new message to channel %s: %v", cfg.ChannelID, err)
						*newMsgIDs = append(*newMsgIDs, "")
					}
				} else {
					// For other errors (e.g. rate limit, transient), retain the ID to keep slot alignment
					*newMsgIDs = append(*newMsgIDs, msgID)
				}
			} else {
				*newMsgIDs = append(*newMsgIDs, msgID)
			}
		} else {
			if msg, err := sendNewLBMessage(s, cfg.ChannelID, components, flags); err == nil {
				*newMsgIDs = append(*newMsgIDs, msg.ID)
			} else if isChannelNotFound(err) {
				log.Printf("leaderboard: channel %s not found for guild %s - deleting config", cfg.ChannelID, cfg.GuildID)
				_ = DeleteGuildLBConfig(cfg.GuildID, cfg.LBType)
				return
			} else {
				log.Printf("leaderboard: failed to post to channel %s: %v", cfg.ChannelID, err)
				*newMsgIDs = append(*newMsgIDs, "")
			}
		}
		*msgIDOffset++
	}
}

// HandleLBPageButton handles pagination buttons for leaderboard posts.
func HandleLBPageButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, "#")
	if len(parts) < 4 {
		return
	}
	lbType := parts[1]
	snapDate := parts[2]
	page, _ := strconv.Atoi(parts[3])

	def, ok := LBDefByKey(lbType)
	if !ok {
		return
	}

	guildRows, prevMap := getGuildRows(lbType, snapDate, i.GuildID)
	pageSize := 50
	start := page * pageSize
	if start < 0 {
		start = 0
		page = 0
	}
	end := start + pageSize
	if end > len(guildRows) {
		end = len(guildRows)
	}

	displayRows := guildRows[start:end]
	blocks := buildMessageBlocks(def, displayRows, snapDate, prevMap, start)
	if len(blocks) == 0 {
		return
	}

	// Use only the first block for paginated view to ensure consistent button behavior.
	text := blocks[0]

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Previous",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("lb_p#%s#%s#%d", lbType, snapDate, page-1),
					Disabled: page <= 0,
				},
				discordgo.Button{
					Label:    "Next",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("lb_p#%s#%s#%d", lbType, snapDate, page+1),
					Disabled: end >= len(guildRows),
				},
			},
		},
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    text,
			Components: components,
			Flags:      discordgo.MessageFlagsIsComponentsV2,
		},
	})
}

func sendNewLBMessage(s *discordgo.Session, channelID string, components []discordgo.MessageComponent, flags discordgo.MessageFlags) (*discordgo.Message, error) {
	data := discordgo.MessageSend{
		Components: components,
		Flags:      flags,
	}
	return s.ChannelMessageSendComplex(channelID, &data)
}

func isChannelNotFound(err error) bool {
	if rerr, ok := err.(*discordgo.RESTError); ok {
		if rerr.Response.StatusCode == 404 || (rerr.Message != nil && rerr.Message.Code == 10003) {
			return true
		}
	}
	return false
}

// buildMessageBlocks formats the leaderboard rows into one or more text blocks
// that each fit within discordMessageCharLimit.
func buildMessageBlocks(def LBDef, rows []LBEntry, snapDate string, prevMap map[string]float64, rankOffset int) []string {
	header := fmt.Sprintf("## 🏆 %s\n", def.DisplayName)
	if def.Key == LBCXPWeeklyDelta {
		header += fmt.Sprintf("### Week of %s\n", snapDate)
	}
	if def.Description != "" {
		header += fmt.Sprintf("> %s\n", def.Description)
	}

	colHeader, rowLines, footer := renderTable(def, rows, prevMap, rankOffset)

	var blocks []string
	current := header + colHeader
	for _, line := range rowLines {
		candidate := current + line
		if len(candidate)+len("```\n")+len(footer) > discordMessageCharLimit {
			blocks = append(blocks, current+"```\n")
			current = header + colHeader + line
		} else {
			current = candidate
		}
	}
	if current != "" {
		blocks = append(blocks, current+"```\n"+footer)
	}
	return blocks
}

// renderTable returns the header, row lines, and footer for a leaderboard table.
func renderTable(def LBDef, rows []LBEntry, prevMap map[string]float64, rankOffset int) (string, []string, string) {
	_ = prevMap

	const maxNameChars = 15
	maxNameWidth := len("Name")

	shortDisplayName := def.DisplayName
	if def.HeaderName != "" {
		shortDisplayName = def.HeaderName
	} else {
		switch def.Key {
		case LBEggsCuriosity:
			shortDisplayName = "Curiosity"
		case LBEggsIntegrity:
			shortDisplayName = "Integrity"
		case LBEggsHumility:
			shortDisplayName = "Humility"
		case LBEggsResilience:
			shortDisplayName = "Resilience"
		case LBEggsKindness:
			shortDisplayName = "Kindness"
		}
	}

	rankHeader := "Rank"
	rankWidth := len(rankHeader)
	maxValOnlyWidth := len(shortDisplayName)

	isEB := def.Key == LBEarningsBonus
	hasCTEComparisonColumn := def.Key == LBCTETotal
	hasTEColumn := def.Key == LBEggsCuriosity || def.Key == LBEggsIntegrity || def.Key == LBEggsHumility || def.Key == LBEggsResilience || def.Key == LBEggsKindness || def.Key == LBVirtueEggsSum
	teWidth := len("TE")
	actualCTEWidth := len("Actual CTE")
	soulMirrorCommonWidth := len("C")
	soulMirrorEpicWidth := len("E")
	soulMirrorLegendaryWidth := len("L")
	const soulMirrorMaxDigits = 5
	if hasCTEComparisonColumn {
		maxValOnlyWidth = len("Future CTE")
	}
	if isEB {
		maxValOnlyWidth = len("Nekkid")
	}
	maxDressedWidth := len("Dressed")

	isTEPerShift := def.Key == LBTEPerShift
	tePerShiftTEWidth := len("TE")
	tePerShiftShiftsWidth := len("Shifts")

	isSEPerPrestige := def.Key == LBSEPerPrestige
	sePerPrestigeSEWidth := len("SE")
	sePerPrestigePrestigesWidth := len("Prestiges")

	isCraftingXP := def.Key == LBCraftingXP
	craftingLevelWidth := len("Lvl")

	type rowInfo struct {
		row                       LBEntry
		rankStr                   string
		nameStr                   string
		displayValStr             string
		dressedValStr             string
		teStr                     string
		actualCTEStr              string
		soulMirrorCommonStr       string
		soulMirrorEpicStr         string
		soulMirrorLegendaryStr    string
		tePerShiftTEStr           string
		tePerShiftShiftsStr       string
		sePerPrestigeSEStr        string
		sePerPrestigePrestigesStr string
		craftingLevelStr          string
	}
	infos := make([]rowInfo, 0, len(rows))

	parseTEFromDetails := func(details string) int {
		var te int
		if _, err := fmt.Sscanf(details, "te:%d", &te); err == nil {
			return te
		}
		if _, err := fmt.Sscanf(details, "%d TE", &te); err == nil {
			return te
		}
		return 0
	}

	formatCTE := func(v float64) string {
		return formatIntWithCommas(int64(math.Round(v)))
	}

	parseActualCTEFromDetails := func(details string) string {
		var actual float64
		if _, err := fmt.Sscanf(details, "actual:%f", &actual); err == nil {
			return formatCTE(actual)
		}
		return "-"
	}

	parseSoulMirrorTiersFromDetails := func(details string) (int, int, int) {
		var common, epic, legendary int
		if _, err := fmt.Sscanf(details, "(%d, %d, %d)", &common, &epic, &legendary); err == nil {
			return common, epic, legendary
		}
		if _, err := fmt.Sscanf(details, "%d, %d, %d", &common, &epic, &legendary); err == nil {
			return common, epic, legendary
		}
		return 0, 0, 0
	}

	parseTEPerShiftDetails := func(details string) (string, string) {
		var te int
		var shifts int
		if _, err := fmt.Sscanf(details, "te:%d shifts:%d", &te, &shifts); err == nil {
			return formatIntWithCommas(int64(te)), formatIntWithCommas(int64(shifts))
		}
		return "-", "-"
	}

	parseSEPerPrestigeDetails := func(details string) (string, string) {
		var se float64
		var prestiges int
		if _, err := fmt.Sscanf(details, "se:%g prestiges:%d", &se, &prestiges); err == nil {
			return FormatLBValue("ei", se), formatIntWithCommas(int64(prestiges))
		}
		return "-", "-"
	}

	lastRank := 0
	lastValue := 0.0
	for i, r := range rows {
		rank := i + 1 + rankOffset
		if i > 0 && r.Value == lastValue {
			rank = lastRank
		}
		rankStr := fmt.Sprintf("#%d", rank)
		if w := len(rankStr); w > rankWidth {
			rankWidth = w
		}

		nameStr := truncateString(r.GameName, maxNameChars)
		if w := len(nameStr); w > maxNameWidth {
			maxNameWidth = w
		}

		displayValStr := FormatLBValue(def.ValueFmt, r.Value)
		if def.Key == LBCXPWeeklyDelta {
			if r.Details == "na" {
				displayValStr = "-"
			} else {
				sign := "+"
				if r.Value < 0 {
					sign = "-"
				}
				displayValStr = sign + FormatLBValue(def.ValueFmt, absFloat(r.Value))
			}
		}
		if hasCTEComparisonColumn {
			displayValStr = formatCTE(r.Value)
		}
		if w := len(displayValStr); w > maxValOnlyWidth {
			maxValOnlyWidth = w
		}

		dressedValStr := ""
		if isEB {
			if idx := strings.Index(r.Details, "dressed:"); idx != -1 {
				var dressed float64
				if _, err := fmt.Sscanf(r.Details[idx:], "dressed:%f", &dressed); err == nil {
					dressedValStr = FormatLBValue(def.ValueFmt, dressed)
				}
			}
			if w := len(dressedValStr); w > maxDressedWidth {
				maxDressedWidth = w
			}
		}

		teStr := ""
		if hasTEColumn {
			teStr = fmt.Sprintf("%d", parseTEFromDetails(r.Details))
			if w := len(teStr); w > teWidth {
				teWidth = w
			}
		}

		actualCTEStr := ""
		if hasCTEComparisonColumn {
			actualCTEStr = parseActualCTEFromDetails(r.Details)
			if w := len(actualCTEStr); w > actualCTEWidth {
				actualCTEWidth = w
			}
		}

		soulMirrorCommonStr, soulMirrorEpicStr, soulMirrorLegendaryStr := "", "", ""
		if def.Key == LBSoulMirrors {
			common, epic, legendary := parseSoulMirrorTiersFromDetails(r.Details)
			soulMirrorCommonStr = fmt.Sprintf("%d", common)
			soulMirrorEpicStr = fmt.Sprintf("%d", epic)
			soulMirrorLegendaryStr = fmt.Sprintf("%d", legendary)

			if w := len(soulMirrorCommonStr); w > soulMirrorCommonWidth {
				soulMirrorCommonWidth = w
				if soulMirrorCommonWidth > soulMirrorMaxDigits {
					soulMirrorCommonWidth = soulMirrorMaxDigits
				}
			}
			if w := len(soulMirrorEpicStr); w > soulMirrorEpicWidth {
				soulMirrorEpicWidth = w
				if soulMirrorEpicWidth > soulMirrorMaxDigits {
					soulMirrorEpicWidth = soulMirrorMaxDigits
				}
			}
			if w := len(soulMirrorLegendaryStr); w > soulMirrorLegendaryWidth {
				soulMirrorLegendaryWidth = w
				if soulMirrorLegendaryWidth > soulMirrorMaxDigits {
					soulMirrorLegendaryWidth = soulMirrorMaxDigits
				}
			}
		}

		tePerShiftTEStr, tePerShiftShiftsStr, sePerPrestigeSEStr, sePerPrestigePrestigesStr := "", "", "", ""
		if isTEPerShift {
			tePerShiftTEStr, tePerShiftShiftsStr = parseTEPerShiftDetails(r.Details)
			if w := len(tePerShiftTEStr); w > tePerShiftTEWidth {
				tePerShiftTEWidth = w
			}
			if w := len(tePerShiftShiftsStr); w > tePerShiftShiftsWidth {
				tePerShiftShiftsWidth = w
			}
		} else if isSEPerPrestige {
			sePerPrestigeSEStr, sePerPrestigePrestigesStr = parseSEPerPrestigeDetails(r.Details)
			if w := len(sePerPrestigeSEStr); w > sePerPrestigeSEWidth {
				sePerPrestigeSEWidth = w
			}
			if w := len(sePerPrestigePrestigesStr); w > sePerPrestigePrestigesWidth {
				sePerPrestigePrestigesWidth = w
			}
		}

		craftingLevelStr := ""
		if isCraftingXP {
			craftingLevelStr = r.Details
			if w := len(craftingLevelStr); w > craftingLevelWidth {
				craftingLevelWidth = w
			}
		}

		infos = append(infos, rowInfo{
			row:                       r,
			rankStr:                   rankStr,
			nameStr:                   nameStr,
			displayValStr:             displayValStr,
			dressedValStr:             dressedValStr,
			teStr:                     teStr,
			actualCTEStr:              actualCTEStr,
			soulMirrorCommonStr:       soulMirrorCommonStr,
			soulMirrorEpicStr:         soulMirrorEpicStr,
			soulMirrorLegendaryStr:    soulMirrorLegendaryStr,
			tePerShiftTEStr:           tePerShiftTEStr,
			tePerShiftShiftsStr:       tePerShiftShiftsStr,
			sePerPrestigeSEStr:        sePerPrestigeSEStr,
			sePerPrestigePrestigesStr: sePerPrestigePrestigesStr,
			craftingLevelStr:          craftingLevelStr,
		})

		lastRank = rank
		lastValue = r.Value
	}

	padField := func(s string, width int, align bottools.StringAlign) string {
		return " " + bottools.AlignString(s, width, align) + " "
	}

	var colHeader string
	if isEB {
		headerLine := strings.Join([]string{
			padField(rankHeader, rankWidth, bottools.StringAlignLeft),
			padField("Name", maxNameWidth, bottools.StringAlignLeft),
			padField("Nekkid", maxValOnlyWidth, bottools.StringAlignRight),
			padField("Dressed", maxDressedWidth, bottools.StringAlignRight),
		}, "|")
		colHeader = fmt.Sprintf("```\n%s\n%s\n", headerLine, strings.Repeat("-", len(headerLine)))
	} else if hasCTEComparisonColumn {
		headerLine := strings.Join([]string{
			padField(rankHeader, rankWidth, bottools.StringAlignLeft),
			padField("Name", maxNameWidth, bottools.StringAlignLeft),
			padField("Pending CTE", maxValOnlyWidth, bottools.StringAlignRight),
			padField("CTE", actualCTEWidth, bottools.StringAlignRight),
		}, "|")
		colHeader = fmt.Sprintf("```\n%s\n%s\n", headerLine, strings.Repeat("-", len(headerLine)))
	} else if hasTEColumn {
		headerLine := strings.Join([]string{
			padField(rankHeader, rankWidth, bottools.StringAlignLeft),
			padField("Name", maxNameWidth, bottools.StringAlignLeft),
			padField(shortDisplayName, maxValOnlyWidth, bottools.StringAlignRight),
			padField("TE", teWidth, bottools.StringAlignRight),
		}, "|")
		colHeader = fmt.Sprintf("```\n%s\n%s\n", headerLine, strings.Repeat("-", len(headerLine)))
	} else if def.Key == LBSoulMirrors {
		headerLine := strings.Join([]string{
			padField(rankHeader, rankWidth, bottools.StringAlignLeft),
			padField("Name", maxNameWidth, bottools.StringAlignLeft),
			padField(shortDisplayName, maxValOnlyWidth, bottools.StringAlignRight),
			padField("C", soulMirrorCommonWidth, bottools.StringAlignRight),
			padField("E", soulMirrorEpicWidth, bottools.StringAlignRight),
			padField("L", soulMirrorLegendaryWidth, bottools.StringAlignRight),
		}, "|")
		colHeader = fmt.Sprintf("```\n%s\n%s\n", headerLine, strings.Repeat("-", len(headerLine)))
	} else if isTEPerShift {
		headerLine := strings.Join([]string{
			padField(rankHeader, rankWidth, bottools.StringAlignLeft),
			padField("Name", maxNameWidth, bottools.StringAlignLeft),
			padField(shortDisplayName, maxValOnlyWidth, bottools.StringAlignRight),
			padField("TE", tePerShiftTEWidth, bottools.StringAlignRight),
			padField("Shifts", tePerShiftShiftsWidth, bottools.StringAlignRight),
		}, "|")
		colHeader = fmt.Sprintf("```\n%s\n%s\n", headerLine, strings.Repeat("-", len(headerLine)))
	} else if isSEPerPrestige {
		headerLine := strings.Join([]string{
			padField(rankHeader, rankWidth, bottools.StringAlignLeft),
			padField("Name", maxNameWidth, bottools.StringAlignLeft),
			padField(shortDisplayName, maxValOnlyWidth, bottools.StringAlignRight),
			padField("SE", sePerPrestigeSEWidth, bottools.StringAlignRight),
			padField("Prestiges", sePerPrestigePrestigesWidth, bottools.StringAlignRight),
		}, "|")
		colHeader = fmt.Sprintf("```\n%s\n%s\n", headerLine, strings.Repeat("-", len(headerLine)))
	} else if isCraftingXP {
		headerLine := strings.Join([]string{
			padField(rankHeader, rankWidth, bottools.StringAlignLeft),
			padField("Name", maxNameWidth, bottools.StringAlignLeft),
			padField(shortDisplayName, maxValOnlyWidth, bottools.StringAlignRight),
			padField("Lvl", craftingLevelWidth, bottools.StringAlignRight),
		}, "|")
		colHeader = fmt.Sprintf("```\n%s\n%s\n", headerLine, strings.Repeat("-", len(headerLine)))
	} else {
		headerLine := strings.Join([]string{
			padField(rankHeader, rankWidth, bottools.StringAlignLeft),
			padField("Name", maxNameWidth, bottools.StringAlignLeft),
			padField(shortDisplayName, maxValOnlyWidth, bottools.StringAlignRight),
		}, "|")
		colHeader = fmt.Sprintf("```\n%s\n%s\n", headerLine, strings.Repeat("-", len(headerLine)))
	}

	rowLines := make([]string, 0, len(infos))
	for _, info := range infos {
		detail := ""
		if info.row.Details != "" && info.row.Details != "na" && !strings.HasPrefix(info.row.Details, "total:") && !strings.Contains(info.row.Details, "dressed:") && !strings.HasPrefix(info.row.Details, "te:") && !strings.HasSuffix(info.row.Details, " TE") && !strings.HasPrefix(info.row.Details, "actual:") && !strings.HasPrefix(info.row.Details, "se:") {
			detail = fmt.Sprintf(" (%s)", info.row.Details)
		}

		if isEB {
			rowLines = append(rowLines, fmt.Sprintf("%s%s\n", strings.Join([]string{
				padField(info.rankStr, rankWidth, bottools.StringAlignLeft),
				padField(info.nameStr, maxNameWidth, bottools.StringAlignLeft),
				padField(info.displayValStr, maxValOnlyWidth, bottools.StringAlignRight),
				padField(info.dressedValStr, maxDressedWidth, bottools.StringAlignRight),
			}, "|"), detail))
			continue
		}

		if hasCTEComparisonColumn {
			rowLines = append(rowLines, fmt.Sprintf("%s%s\n", strings.Join([]string{
				padField(info.rankStr, rankWidth, bottools.StringAlignLeft),
				padField(info.nameStr, maxNameWidth, bottools.StringAlignLeft),
				padField(info.displayValStr, maxValOnlyWidth, bottools.StringAlignRight),
				padField(info.actualCTEStr, actualCTEWidth, bottools.StringAlignRight),
			}, "|"), detail))
			continue
		}

		if hasTEColumn {
			rowLines = append(rowLines, fmt.Sprintf("%s%s\n", strings.Join([]string{
				padField(info.rankStr, rankWidth, bottools.StringAlignLeft),
				padField(info.nameStr, maxNameWidth, bottools.StringAlignLeft),
				padField(info.displayValStr, maxValOnlyWidth, bottools.StringAlignRight),
				padField(info.teStr, teWidth, bottools.StringAlignRight),
			}, "|"), detail))
			continue
		}

		if def.Key == LBSoulMirrors {
			rowLines = append(rowLines, fmt.Sprintf("%s\n", strings.Join([]string{
				padField(info.rankStr, rankWidth, bottools.StringAlignLeft),
				padField(info.nameStr, maxNameWidth, bottools.StringAlignLeft),
				padField(info.displayValStr, maxValOnlyWidth, bottools.StringAlignRight),
				padField(info.soulMirrorCommonStr, soulMirrorCommonWidth, bottools.StringAlignRight),
				padField(info.soulMirrorEpicStr, soulMirrorEpicWidth, bottools.StringAlignRight),
				padField(info.soulMirrorLegendaryStr, soulMirrorLegendaryWidth, bottools.StringAlignRight),
			}, "|")))
			continue
		}

		if isTEPerShift {
			rowLines = append(rowLines, fmt.Sprintf("%s%s\n", strings.Join([]string{
				padField(info.rankStr, rankWidth, bottools.StringAlignLeft),
				padField(info.nameStr, maxNameWidth, bottools.StringAlignLeft),
				padField(info.displayValStr, maxValOnlyWidth, bottools.StringAlignRight),
				padField(info.tePerShiftTEStr, tePerShiftTEWidth, bottools.StringAlignRight),
				padField(info.tePerShiftShiftsStr, tePerShiftShiftsWidth, bottools.StringAlignRight),
			}, "|"), detail))
			continue
		}

		if isSEPerPrestige {
			rowLines = append(rowLines, fmt.Sprintf("%s%s\n", strings.Join([]string{
				padField(info.rankStr, rankWidth, bottools.StringAlignLeft),
				padField(info.nameStr, maxNameWidth, bottools.StringAlignLeft),
				padField(info.displayValStr, maxValOnlyWidth, bottools.StringAlignRight),
				padField(info.sePerPrestigeSEStr, sePerPrestigeSEWidth, bottools.StringAlignRight),
				padField(info.sePerPrestigePrestigesStr, sePerPrestigePrestigesWidth, bottools.StringAlignRight),
			}, "|"), detail))
			continue
		}

		if isCraftingXP {
			rowLines = append(rowLines, fmt.Sprintf("%s\n", strings.Join([]string{
				padField(info.rankStr, rankWidth, bottools.StringAlignLeft),
				padField(info.nameStr, maxNameWidth, bottools.StringAlignLeft),
				padField(info.displayValStr, maxValOnlyWidth, bottools.StringAlignRight),
				padField(info.craftingLevelStr, craftingLevelWidth, bottools.StringAlignRight),
			}, "|")))
			continue
		}

		rowLines = append(rowLines, fmt.Sprintf("%s%s\n", strings.Join([]string{
			padField(info.rankStr, rankWidth, bottools.StringAlignLeft),
			padField(info.nameStr, maxNameWidth, bottools.StringAlignLeft),
			padField(info.displayValStr, maxValOnlyWidth, bottools.StringAlignRight),
		}, "|"), detail))
	}

	footer := fmt.Sprintf("-# Updated: %s\n", bottools.WrapTimestamp(time.Now().Unix(), bottools.TimestampShortDateTime))
	return colHeader, rowLines, footer
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// truncateString ensures a string is at most max characters, adding … if needed
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3]
}

// FormatLBValue formats a numeric leaderboard value according to the LBDef.ValueFmt.
func FormatLBValue(fmtValue string, v float64) string {
	switch fmtValue {
	case "int":
		return fmt.Sprintf("%.0f", v)
	case "float":
		return fmt.Sprintf("%.2f", v)
	case "ei":
		return ei.FormatEIValue(v, map[string]any{"decimals": 3, "trim": true})
	case "eb":
		return ei.FormatEIValue(v, map[string]any{"decimals": 3, "trim": true}) + "%"
	case "cxp":
		return formatIntWithCommas(int64(math.Round(v)))
	case "duration":
		d := time.Duration(v) * time.Second
		days := int(d.Hours() / 24)
		hours := int(d.Hours()) % 24
		minutes := int(d.Minutes()) % 60
		if days > 0 {
			return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
		}
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		return fmt.Sprintf("%g", v)
	}
}

func formatIntWithCommas(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}

	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return sign + s
	}

	first := len(s) % 3
	if first == 0 {
		first = 3
	}

	var b strings.Builder
	b.WriteString(sign)
	b.WriteString(s[:first])
	for i := first; i < len(s); i += 3 {
		b.WriteString(",")
		b.WriteString(s[i : i+3])
	}

	return b.String()
}

// FormatLBDelta formats a numeric difference from the previous week.
func FormatLBDelta(fmtValue string, delta float64) string {
	if delta == 0 {
		return ""
	}
	sign := "+"
	if delta < 0 {
		sign = "-"
		delta = -delta
	}

	valStr := FormatLBValue(fmtValue, delta)
	return sign + valStr
}
