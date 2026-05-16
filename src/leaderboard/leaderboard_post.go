package leaderboard

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mattn/go-runewidth"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const discordMessageCharLimit = 1900

// PostLeaderboards triggers the posting task for all configured guilds.
func PostLeaderboards(s *discordgo.Session, snapDate string, onProgress func(string)) {
	configs, err := GetAllLBConfigs()
	if err != nil {
		log.Printf("leaderboard: PostLeaderboards: failed to load configs: %v", err)
		return
	}

	for i, cfg := range configs {
		if onProgress != nil {
			onProgress(fmt.Sprintf("📬 Posting leaderboards to guild %d/%d (%s)...", i+1, len(configs), cfg.GuildID))
		}
		postOneLeaderboard(s, cfg, snapDate, onProgress)
		time.Sleep(2 * time.Second) // Gap between guilds to leave room for other bot activities
	}
	if onProgress != nil {
		onProgress("🏁 Weekly leaderboard update complete!")
	}
}

// postOneLeaderboard handles the expanded posting of a single config (which might be a group).
func postOneLeaderboard(s *discordgo.Session, cfg LBConfig, snapDate string, onProgress func(string)) {
	memberKeys := ExpandConfigKey(cfg.LBType)
	var newMsgIDs []string
	msgIDOffset := 0
	forceNewPosts := false

	for _, lbType := range memberKeys {
		def, ok := LBDefByKey(lbType)
		if !ok {
			log.Printf("leaderboard: unknown lb_type %q in group/config for guild %s", lbType, cfg.GuildID)
			continue
		}

		if onProgress != nil && len(memberKeys) > 1 {
			onProgress(fmt.Sprintf("📬 Guild %s: Updating %s...", cfg.GuildID, def.DisplayName))
		}

		postSingleMetric(s, cfg, lbType, snapDate, &newMsgIDs, &msgIDOffset, &forceNewPosts)
		time.Sleep(1 * time.Second) // Conservative delay to allow room for concurrent bot activities
	}

	UpdateGuildLBConfigMessageIDs(cfg.GuildID, cfg.LBType, newMsgIDs)
}

func getGuildRows(lbType string, snapDate string, guildID string) ([]LBEntry, map[string]float64) {
	allRows := GetLeaderboardRows(lbType, snapDate)
	prevSnapDate := GetPreviousSnapDate(lbType, snapDate)
	prevMap := make(map[string]float64)
	if prevSnapDate != "" {
		prevRows := GetLeaderboardRows(lbType, prevSnapDate)
		for _, r := range prevRows {
			prevMap[r.Player] = r.Value
		}
	}

	guildMemberIgns := farmerstate.GetEiIgnsByGuild(guildID)
	guildIgnSet := make(map[string]struct{}, len(guildMemberIgns))
	for _, ign := range guildMemberIgns {
		guildIgnSet[ign] = struct{}{}
	}

	isShipLB := strings.HasPrefix(lbType, "ship_") || strings.HasPrefix(lbType, "std_ship_")
	var guildRows []LBEntry
	for _, row := range allRows {
		nameToMatch := strings.TrimSuffix(row.GameName, " (SP)")
		inGuild := false
		if len(guildMemberIgns) == 0 {
			inGuild = true
		} else {
			_, inGuild = guildIgnSet[nameToMatch]
		}

		if !inGuild {
			continue
		}

		if isShipLB && row.Value == 0 {
			continue
		}

		guildRows = append(guildRows, row)
	}
	return guildRows, prevMap
}

func postSingleMetric(s *discordgo.Session, cfg LBConfig, lbType, snapDate string, newMsgIDs *[]string, msgIDOffset *int, forceNewPosts *bool) {
	def, _ := LBDefByKey(lbType)
	guildRows, prevMap := getGuildRows(lbType, snapDate, cfg.GuildID)

	if len(guildRows) == 0 {
		log.Printf("leaderboard: no eligible guild members for %s in guild %s", lbType, cfg.GuildID)
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
		if !*forceNewPosts && *msgIDOffset < len(cfg.MessageIDs) {
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
				if rerr, ok := err.(*discordgo.RESTError); ok && rerr.Message.Code == 10008 {
					*forceNewPosts = true
				}
				if msg, err := sendNewLBMessage(s, cfg.ChannelID, components, flags); err == nil {
					*newMsgIDs = append(*newMsgIDs, msg.ID)
				} else if isChannelNotFound(err) {
					log.Printf("leaderboard: channel %s not found for guild %s - deleting config", cfg.ChannelID, cfg.GuildID)
					_ = DeleteGuildLBConfig(cfg.GuildID, cfg.LBType)
					return
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
		if rerr.Response.StatusCode == 404 || rerr.Message.Code == 10003 {
			return true
		}
	}
	return false
}

// buildMessageBlocks formats the leaderboard rows into one or more text blocks
// that each fit within discordMessageCharLimit.
func buildMessageBlocks(def LBDef, rows []LBEntry, snapDate string, prevMap map[string]float64, rankOffset int) []string {
	header := fmt.Sprintf("## 🏆 %s — Week of %s\n", def.DisplayName, snapDate)
	if def.Description != "" {
		header += fmt.Sprintf("> %s\n", def.Description)
	}
	header += "\n"

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
	if len(rows) == 0 {
		return "", nil, ""
	}

	isEB := def.Key == LBEarningsBonus
	maxRank := len(rows) + rankOffset
	rankWidth := len(fmt.Sprintf("%d", maxRank))
	if rankWidth < 2 {
		rankWidth = 2
	}
	maxNameWidth := runewidth.StringWidth("Name")
	maxValOnlyWidth := runewidth.StringWidth(def.DisplayName)
	if isEB {
		maxValOnlyWidth = runewidth.StringWidth("Nekkid")
	}
	maxDressedWidth := 0
	if isEB {
		maxDressedWidth = runewidth.StringWidth("Dressed")
	}
	maxDeltaWidth := 0

	// Pre-calculate deltas and widths.
	type rowInfo struct {
		rank          int
		row           LBEntry
		valStr        string
		dressedValStr string
		deltaStr      string
	}
	infos := make([]rowInfo, 0, len(rows))

	for i, r := range rows {
		valStr := FormatLBValue(def.ValueFmt, r.Value)
		dressedValStr := ""
		if isEB {
			if idx := strings.Index(r.Details, "dressed:"); idx != -1 {
				var d float64
				if _, err := fmt.Sscanf(r.Details[idx:], "dressed:%f", &d); err == nil {
					dressedValStr = FormatLBValue(def.ValueFmt, d)
				}
			}
		}

		deltaStr := ""
		if prevVal, ok := prevMap[r.Player]; ok {
			deltaStr = FormatLBDelta(def.ValueFmt, r.Value-prevVal)
		}

		infos = append(infos, rowInfo{
			rank:          i + 1 + rankOffset,
			row:           r,
			valStr:        valStr,
			dressedValStr: dressedValStr,
			deltaStr:      deltaStr,
		})

		w := runewidth.StringWidth(r.GameName)
		if w > maxNameWidth {
			maxNameWidth = w
		}

		if len(valStr) > maxValOnlyWidth {
			maxValOnlyWidth = len(valStr)
		}
		if isEB && len(dressedValStr) > maxDressedWidth {
			maxDressedWidth = len(dressedValStr)
		}
		if len(deltaStr) > maxDeltaWidth {
			maxDeltaWidth = len(deltaStr)
		}
	}

	maxValWidth := maxValOnlyWidth
	if maxDeltaWidth > 0 {
		maxValWidth += 1 + maxDeltaWidth
	}

	var colHeader string
	if isEB {
		colHeader = fmt.Sprintf("```\n%s %s %s %s\n%s\n",
			bottools.AlignString("##:", rankWidth+1, bottools.StringAlignRight),
			bottools.AlignString("Name", maxNameWidth, bottools.StringAlignLeft),
			bottools.AlignString("Nekkid", maxValWidth, bottools.StringAlignRight),
			bottools.AlignString("Dressed", maxDressedWidth, bottools.StringAlignRight),
			strings.Repeat("═", rankWidth+1+1+maxNameWidth+1+maxValWidth+1+maxDressedWidth),
		)
	} else {
		colHeader = fmt.Sprintf("```\n%s %s %s\n%s\n",
			bottools.AlignString("##:", rankWidth+1, bottools.StringAlignRight),
			bottools.AlignString("Name", maxNameWidth, bottools.StringAlignLeft),
			bottools.AlignString(def.DisplayName, maxValWidth, bottools.StringAlignRight),
			strings.Repeat("═", rankWidth+1+1+maxNameWidth+1+maxValWidth),
		)
	}

	rowLines := make([]string, 0, len(rows))
	for _, info := range infos {
		detail := ""
		if info.row.Details != "" && !strings.HasPrefix(info.row.Details, "total:") && !strings.Contains(info.row.Details, "dressed:") {
			detail = fmt.Sprintf(" (%s)", info.row.Details)
		}

		displayVal := bottools.AlignString(info.valStr, maxValOnlyWidth, bottools.StringAlignRight)
		if maxDeltaWidth > 0 {
			if info.deltaStr != "" {
				displayVal += " " + bottools.AlignString(info.deltaStr, maxDeltaWidth, bottools.StringAlignLeft)
			} else {
				displayVal += strings.Repeat(" ", maxDeltaWidth+1)
			}
		}

		if isEB {
			line := fmt.Sprintf("%s %s %s %s%s\n",
				bottools.AlignString(fmt.Sprintf("%d:", info.rank), rankWidth+1, bottools.StringAlignRight),
				bottools.AlignString(info.row.GameName, maxNameWidth, bottools.StringAlignLeft),
				displayVal,
				bottools.AlignString(info.dressedValStr, maxDressedWidth, bottools.StringAlignRight),
				detail,
			)
			rowLines = append(rowLines, line)
		} else {
			line := fmt.Sprintf("%s %s %s%s\n",
				bottools.AlignString(fmt.Sprintf("%d:", info.rank), rankWidth+1, bottools.StringAlignRight),
				bottools.AlignString(info.row.GameName, maxNameWidth, bottools.StringAlignLeft),
				displayVal,
				detail,
			)
			rowLines = append(rowLines, line)
		}
	}

	footer := fmt.Sprintf("-# Updated: %s\n",
		bottools.WrapTimestamp(time.Now().Unix(), bottools.TimestampShortDateTime))

	return colHeader, rowLines, footer
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
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%g", v)
	}
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
