package leaderboard

import (
	"fmt"
	"log"
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
func PostLeaderboards(s *discordgo.Session, snapDate string) {
	configs, err := GetAllLBConfigs()
	if err != nil {
		log.Printf("leaderboard: PostLeaderboards: failed to load configs: %v", err)
		return
	}

	for _, cfg := range configs {
		postOneLeaderboard(s, cfg, snapDate)
	}
}

// postOneLeaderboard handles the expanded posting of a single config (which might be a group).
func postOneLeaderboard(s *discordgo.Session, cfg LBConfig, snapDate string) {
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

		allRows := GetLeaderboardRows(lbType, snapDate)
		if len(allRows) == 0 {
			log.Printf("leaderboard: no data for %s on %s - skipping for guild %s",
				lbType, snapDate, cfg.GuildID)
			msgIDOffset++
			continue
		}

		// Fetch previous week's rows for delta calculation.
		prevSnapDate := GetPreviousSnapDate(lbType, snapDate)
		prevMap := make(map[string]float64)
		if prevSnapDate != "" {
			prevRows := GetLeaderboardRows(lbType, prevSnapDate)
			for _, r := range prevRows {
				prevMap[r.Player] = r.Value
			}
		}

		// Filter to guild members.
		guildMemberIgns := farmerstate.GetEiIgnsByGuild(cfg.GuildID)
		guildIgnSet := make(map[string]struct{}, len(guildMemberIgns))
		for _, ign := range guildMemberIgns {
			guildIgnSet[ign] = struct{}{}
		}
		var guildRows []LBEntry
		if len(guildMemberIgns) == 0 {
			guildRows = allRows
		} else {
			for _, row := range allRows {
				if _, ok := guildIgnSet[row.GameName]; ok {
					guildRows = append(guildRows, row)
				}
			}
		}
		if len(guildRows) == 0 {
			log.Printf("leaderboard: no guild members on %s for guild %s", lbType, cfg.GuildID)
			msgIDOffset++
			continue
		}

		blocks := buildMessageBlocks(def, guildRows, snapDate, prevMap)
		for _, text := range blocks {
			components := []discordgo.MessageComponent{
				&discordgo.TextDisplay{Content: text},
			}
			flags := discordgo.MessageFlagsIsComponentsV2

			if !forceNewPosts && msgIDOffset < len(cfg.MessageIDs) {
				msgID := cfg.MessageIDs[msgIDOffset]
				edit := discordgo.MessageEdit{
					ID:         msgID,
					Channel:    cfg.ChannelID,
					Components: &components,
					Flags:      flags,
				}
				if _, err := s.ChannelMessageEditComplex(&edit); err != nil {
					log.Printf("leaderboard: failed to edit message %s: %v", msgID, err)

					// If the message was deleted (404/10008), force new posts for the rest to keep order.
					if rerr, ok := err.(*discordgo.RESTError); ok && (rerr.Response.StatusCode == 404 || rerr.Message.Code == 10008) {
						forceNewPosts = true
					}

					if msg, err := sendNewLBMessage(s, cfg.ChannelID, components, flags); err == nil {
						newMsgIDs = append(newMsgIDs, msg.ID)
					}
				} else {
					newMsgIDs = append(newMsgIDs, msgID)
				}
			} else {
				if msg, err := sendNewLBMessage(s, cfg.ChannelID, components, flags); err == nil {
					newMsgIDs = append(newMsgIDs, msg.ID)
				} else {
					log.Printf("leaderboard: failed to post to channel %s: %v", cfg.ChannelID, err)
				}
			}
			msgIDOffset++
		}
	}

	UpdateGuildLBConfigMessageIDs(cfg.GuildID, cfg.LBType, newMsgIDs)
}

func sendNewLBMessage(s *discordgo.Session, channelID string, components []discordgo.MessageComponent, flags discordgo.MessageFlags) (*discordgo.Message, error) {
	data := discordgo.MessageSend{
		Components: components,
		Flags:      flags,
	}
	return s.ChannelMessageSendComplex(channelID, &data)
}

// buildMessageBlocks formats the leaderboard rows into one or more text blocks
// that each fit within discordMessageCharLimit.
func buildMessageBlocks(def LBDef, rows []LBEntry, snapDate string, prevMap map[string]float64) []string {
	header := fmt.Sprintf("## 🏆 %s — Week of %s\n", def.DisplayName, snapDate)
	if def.Description != "" {
		header += fmt.Sprintf("> %s\n", def.Description)
	}
	header += "\n"

	colHeader, rowLines, footer := renderTable(def, rows, prevMap)

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
func renderTable(def LBDef, rows []LBEntry, prevMap map[string]float64) (string, []string, string) {
	if len(rows) == 0 {
		return "", nil, ""
	}

	maxRank := len(rows)
	rankWidth := max(len(fmt.Sprintf("%d", maxRank)), 4)
	maxNameWidth := len("Name")
	maxValWidth := len("Value")

	// Pre-calculate deltas and widths.
	type rowInfo struct {
		rank     int
		row      LBEntry
		valStr   string
		deltaStr string
	}
	infos := make([]rowInfo, 0, len(rows))

	for i, r := range rows {
		valStr := formatValue(def.ValueFmt, r.Value)
		deltaStr := ""
		if prevVal, ok := prevMap[r.Player]; ok {
			deltaStr = formatDelta(def.ValueFmt, r.Value-prevVal)
		}

		infos = append(infos, rowInfo{
			rank:     i + 1,
			row:      r,
			valStr:   valStr,
			deltaStr: deltaStr,
		})

		w := runewidth.StringWidth(r.GameName)
		if w > maxNameWidth {
			maxNameWidth = w
		}

		fullValLen := len(valStr)
		if deltaStr != "" {
			fullValLen += 1 + len(deltaStr) // space + delta
		}
		if fullValLen > maxValWidth {
			maxValWidth = fullValLen
		}
	}

	colHeader := fmt.Sprintf("```\n%s|%s|%s\n%s\n",
		bottools.AlignString("Rank", rankWidth+1, bottools.StringAlignLeft),
		bottools.AlignString("Name", maxNameWidth, bottools.StringAlignLeft),
		bottools.AlignString("Value", maxValWidth, bottools.StringAlignRight),
		strings.Repeat("—", rankWidth+1+maxNameWidth+maxValWidth+2),
	)

	rowLines := make([]string, 0, len(rows))
	for _, info := range infos {
		detail := ""
		if info.row.Details != "" && !strings.HasPrefix(info.row.Details, "total:") {
			detail = fmt.Sprintf(" (%s)", info.row.Details)
		}

		displayVal := info.valStr
		if info.deltaStr != "" {
			displayVal += " " + info.deltaStr
		}

		line := fmt.Sprintf("%s|%s|%s%s\n",
			bottools.AlignString(fmt.Sprintf("#%d", info.rank), rankWidth+1, bottools.StringAlignLeft),
			bottools.AlignString(info.row.GameName, maxNameWidth, bottools.StringAlignLeft),
			bottools.AlignString(displayVal, maxValWidth, bottools.StringAlignRight),
			detail,
		)
		rowLines = append(rowLines, line)
	}

	footer := fmt.Sprintf("-# Updated: %s\n",
		bottools.WrapTimestamp(time.Now().Unix(), bottools.TimestampShortDateTime))

	return colHeader, rowLines, footer
}

// formatValue formats a numeric leaderboard value according to the LBDef.ValueFmt.
func formatValue(fmt_ string, v float64) string {
	switch fmt_ {
	case "int":
		return fmt.Sprintf("%.0f", v)
	case "float":
		return fmt.Sprintf("%.2f", v)
	case "ei":
		// Use the same formatting as virtue.go — EI large-number display.
		return ei.FormatEIValue(v, map[string]any{"decimals": 3, "trim": true})
	case "eb":
		return ei.FormatEIValue(v, map[string]any{"decimals": 3, "trim": true}) + "%"
	case "cxp":
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%g", v)
	}
}

// formatDelta formats a numeric difference from the previous week.
func formatDelta(fmt_ string, delta float64) string {
	if delta == 0 {
		return ""
	}
	sign := "+"
	if delta < 0 {
		sign = "-"
		delta = -delta
	}

	valStr := formatValue(fmt_, delta)
	return sign + valStr
}
