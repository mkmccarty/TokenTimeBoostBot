package boost

import (
	"fmt"
	"log"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/rs/xid"
)

type chartRow struct {
	contractID string
	cxp        float64
	maxCxp     float64
	gap        float64
	percent    float64
	validUntil int64
	dayLabel   string
}

type chartSession struct {
	xid       string
	userID    string
	rows      []chartRow
	page      int
	sortBy    string
	percent   int
	expiresAt time.Time
	hasDayMap bool
}

var chartSessions = make(map[string]*chartSession)

func cleanupChartSessions() {
	now := time.Now()
	for k, v := range chartSessions {
		if now.After(v.expiresAt) {
			delete(chartSessions, k)
		}
	}
}

func printContractChart(userID string, archive []*ei.LocalContract, percent int, page int, contractIDList []string, contractDayMap map[string]string) []discordgo.MessageComponent {
	cleanupChartSessions()
	var rows []chartRow

	eiUserName := farmerstate.GetMiscSettingString(userID, "ei_ign")

	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		return []discordgo.MessageComponent{&discordgo.TextDisplay{
			Content: "No archived contracts found in Egg Inc API response",
		}}
	}
	log.Printf("Downloaded %d archived contracts from Egg Inc API for %s\n", len(archive), eiUserName)

	for _, a := range archive {
		contractID := a.GetContract().GetIdentifier()
		if contractID == "first-contract" {
			continue
		}

		evaluation := a.GetEvaluation()
		evaluationCxp := evaluation.GetCxp()
		c := ei.EggIncContractsAll[contractID]

		if len(contractIDList) > 0 {
			if !slices.Contains(contractIDList, contractID) {
				continue
			}
		}

		if c.ContractVersion == 2 && (percent != -1 || (c.ValidUntil.Unix() > time.Now().Unix())) {
			evalPercent := 0.0
			if c.CxpMax > 0 {
				evalPercent = (evaluationCxp / c.CxpMax) * 100.0
			}

			if percent == -1 || percent == -200 || evalPercent < float64(100-percent) {
				dayLabel := ""
				if len(contractDayMap) > 0 {
					dayLabel = contractDayMap[contractID]
				}
				rows = append(rows, chartRow{
					contractID: contractID,
					cxp:        evaluationCxp,
					maxCxp:     c.CxpMax,
					gap:        c.CxpMax - evaluationCxp,
					percent:    evalPercent,
					validUntil: c.ValidUntil.Unix(),
					dayLabel:   dayLabel,
				})
			}
		}
	}

	session := &chartSession{
		xid:       xid.New().String(),
		userID:    userID,
		rows:      rows,
		page:      page - 1, // Store as 0-indexed internally
		sortBy:    "date",
		percent:   percent,
		expiresAt: time.Now().Add(15 * time.Minute),
		hasDayMap: len(contractDayMap) > 0,
	}
	if session.page < 0 {
		session.page = 0
	}

	chartSessions[session.xid] = session
	return renderChartSession(session)
}

func renderChartSession(session *chartSession) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent
	divider := true
	spacing := discordgo.SeparatorSpacingSizeSmall
	builder := strings.Builder{}

	// Sort rows
	sort.SliceStable(session.rows, func(i, j int) bool {
		switch session.sortBy {
		case "gap":
			if session.rows[i].gap == session.rows[j].gap {
				return session.rows[i].validUntil > session.rows[j].validUntil
			}
			return session.rows[i].gap > session.rows[j].gap
		case "gap_asc":
			if session.rows[i].gap == session.rows[j].gap {
				return session.rows[i].validUntil > session.rows[j].validUntil
			}
			return session.rows[i].gap < session.rows[j].gap
		case "percent":
			if session.rows[i].percent == session.rows[j].percent {
				return session.rows[i].validUntil > session.rows[j].validUntil
			}
			return session.rows[i].percent < session.rows[j].percent
		case "percent_desc":
			if session.rows[i].percent == session.rows[j].percent {
				return session.rows[i].validUntil > session.rows[j].validUntil
			}
			return session.rows[i].percent > session.rows[j].percent
		case "cs":
			if session.rows[i].cxp == session.rows[j].cxp {
				return session.rows[i].validUntil > session.rows[j].validUntil
			}
			return session.rows[i].cxp > session.rows[j].cxp
		case "cs_asc":
			if session.rows[i].cxp == session.rows[j].cxp {
				return session.rows[i].validUntil > session.rows[j].validUntil
			}
			return session.rows[i].cxp < session.rows[j].cxp
		case "date_asc":
			return session.rows[i].validUntil < session.rows[j].validUntil
		case "date":
			fallthrough
		default:
			return session.rows[i].validUntil > session.rows[j].validUntil
		}
	})

	// Pagination bounds
	pageSize := 15
	totalPages := int(math.Ceil(float64(len(session.rows)) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	if session.page >= totalPages {
		session.page = totalPages - 1
	}
	if session.page < 0 {
		session.page = 0
	}

	startIdx := session.page * pageSize
	endIdx := min(startIdx+pageSize, len(session.rows))
	pageRows := session.rows[startIdx:endIdx]

	switch session.percent {
	case -1:
		builder.WriteString("## Contract CS eval of active contracts\n")
	case -200:
		builder.WriteString("## Displaying contract scores for future predictions:\n")
	default:
		fmt.Fprintf(&builder, "## Displaying contract scores less than %d%% of speedrun potential:\n", session.percent)
	}

	components = append(components, &discordgo.TextDisplay{Content: builder.String()})
	components = append(components, &discordgo.Separator{Divider: &divider, Spacing: &spacing})
	builder.Reset()

	if len(pageRows) == 0 {
		components = append(components, &discordgo.TextDisplay{Content: "No contracts met this condition.\n"})
		return components
	}

	if session.hasDayMap {
		fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s %3s`\n",
			bottools.AlignString("CONTRACT-ID", 30, bottools.StringAlignCenter),
			bottools.AlignString("CS", 6, bottools.StringAlignCenter),
			bottools.AlignString("HIGH", 6, bottools.StringAlignCenter),
			bottools.AlignString("GAP", 6, bottools.StringAlignRight),
			bottools.AlignString("%", 4, bottools.StringAlignCenter),
			bottools.AlignString("Day", 6, bottools.StringAlignCenter),
		)
	} else {
		fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s`\n",
			bottools.AlignString("CONTRACT-ID", 30, bottools.StringAlignCenter),
			bottools.AlignString("CS", 6, bottools.StringAlignCenter),
			bottools.AlignString("HIGH", 6, bottools.StringAlignCenter),
			bottools.AlignString("GAP", 6, bottools.StringAlignRight),
			bottools.AlignString("%", 4, bottools.StringAlignCenter),
		)
	}

	for _, r := range pageRows {
		if session.hasDayMap {
			fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s %3s`\n",
				bottools.AlignString(r.contractID, 30, bottools.StringAlignLeft),
				bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.cxp))), 6, bottools.StringAlignRight),
				bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.maxCxp))), 6, bottools.StringAlignRight),
				bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.gap))), 6, bottools.StringAlignRight),
				bottools.AlignString(fmt.Sprintf("%.1f", r.percent), 4, bottools.StringAlignCenter),
				bottools.AlignString(r.dayLabel, 6, bottools.StringAlignCenter))
		} else {
			fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s` <t:%d:R>\n",
				bottools.AlignString(r.contractID, 30, bottools.StringAlignLeft),
				bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.cxp))), 6, bottools.StringAlignRight),
				bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.maxCxp))), 6, bottools.StringAlignRight),
				bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(r.gap))), 6, bottools.StringAlignRight),
				bottools.AlignString(fmt.Sprintf("%.1f", r.percent), 4, bottools.StringAlignCenter),
				r.validUntil)
		}
	}

	fmt.Fprintf(&builder, "\nShowing page %d of %d (%d total contracts).\n", session.page+1, totalPages, len(session.rows))
	if session.hasDayMap {
		fmt.Fprintf(&builder, "-# Predicted contract days: W=Wednesday, F=Friday, U=Friday%s\n", ei.GetBotEmojiMarkdown("ultra"))
	}
	fmt.Fprintf(&builder, "-# Est duration/CS based on 1.0 fair share, 5%s boosts (w/50%sIHR), 6%s/hr rate and leggy artifacts.\n", ei.GetBotEmojiMarkdown("token"), ei.GetBotEmojiMarkdown("egg_truth"), ei.GetBotEmojiMarkdown("token"))

	components = append(components, &discordgo.TextDisplay{Content: builder.String()})
	components = append(components, buildChartControls(session, totalPages)...)

	return components
}

func buildChartControls(session *chartSession, totalPages int) []discordgo.MessageComponent {
	var rows []discordgo.MessageComponent

	sortOptions := []discordgo.SelectMenuOption{
		{Label: "Sort by Date (Most Recent First)", Value: "date", Default: session.sortBy == "date"},
		{Label: "Sort by Date (Oldest First)", Value: "date_asc", Default: session.sortBy == "date_asc"},
		{Label: "Sort by CS Gap (Highest First)", Value: "gap", Default: session.sortBy == "gap"},
		{Label: "Sort by CS Gap (Lowest First)", Value: "gap_asc", Default: session.sortBy == "gap_asc"},
		{Label: "Sort by % of Max (Lowest First)", Value: "percent", Default: session.sortBy == "percent"},
		{Label: "Sort by % of Max (Highest First)", Value: "percent_desc", Default: session.sortBy == "percent_desc"},
		{Label: "Sort by CS (Highest First)", Value: "cs", Default: session.sortBy == "cs"},
		{Label: "Sort by CS (Lowest First)", Value: "cs_asc", Default: session.sortBy == "cs_asc"},
	}

	minValues := 1
	rows = append(rows, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    fmt.Sprintf("chart#sort#%s", session.xid),
				Placeholder: "Sort order...",
				Options:     sortOptions,
				MinValues:   &minValues,
				MaxValues:   1,
			},
		},
	})

	var buttons []discordgo.MessageComponent
	buttons = append(buttons, discordgo.Button{
		Label:    "Prev",
		Style:    discordgo.SecondaryButton,
		CustomID: fmt.Sprintf("chart#prev#%s", session.xid),
		Disabled: session.page <= 0,
	})
	buttons = append(buttons, discordgo.Button{
		Label:    "Next",
		Style:    discordgo.SecondaryButton,
		CustomID: fmt.Sprintf("chart#next#%s", session.xid),
		Disabled: session.page >= totalPages-1,
	})

	rows = append(rows, discordgo.ActionsRow{Components: buttons})
	return rows
}

// HandleChartReactions handles button and select menu interactions for the chart view
func HandleChartReactions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 3 {
		return
	}

	action := parts[1]
	xidPart := parts[2]
	userID := bottools.GetInteractionUserID(i)

	session, ok := chartSessions[xidPart]
	if !ok {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This chart session has expired. Please run the command again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if session.userID != userID {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Only the user who requested this chart can interact with it.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	session.expiresAt = time.Now().Add(15 * time.Minute)

	switch action {
	case "sort":
		values := i.MessageComponentData().Values
		if len(values) > 0 {
			session.sortBy = values[0]
			session.page = 0 // Reset to first page on sort
		}
	case "prev":
		session.page--
	case "next":
		session.page++
	}

	components := renderChartSession(session)

	flags := discordgo.MessageFlags(0)
	if i.Message != nil && i.Message.Flags&discordgo.MessageFlagsEphemeral != 0 {
		flags |= discordgo.MessageFlagsEphemeral
	}
	flags |= discordgo.MessageFlagsIsComponentsV2

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags:      flags,
			Components: components,
		},
	})
}
