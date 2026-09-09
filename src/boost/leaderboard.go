package boost

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

type leaderboardSeason struct {
	name  string
	value string
}

var leaderboardFallbackSeasons = []leaderboardSeason{
	{"All Time", "ALL_TIME"},
	{"Summer 2026", "summer_2026"},
	{"Spring 2026", "spring_2026"},
	{"Winter 2026", "winter_2026"},
	{"Fall 2025", "fall_2025"},
	{"Summer 2025", "summer_2025"},
	{"Spring 2025", "spring_2025"},
	{"Winter 2025", "winter_2025"},
	{"Fall 2024", "fall_2024"},
	{"Summer 2024", "summer_2024"},
	{"Spring 2024", "spring_2024"},
	{"Winter 2024", "winter_2024"},
	{"Fall 2023", "fall_2023"},
	{"Summer 2023", "summer_2023"},
	{"Spring 2023", "spring_2023"},
}

const leaderboardAllTimeScope = "ALL_TIME"
const leaderboardMaxSeasonOptions = 25
const leaderboardMaxAutocompleteChoices = 25
const leaderboardSeasonStartYear = 2023
const leaderboardSeasonStartName = "spring"

var leaderboardSeasonOrder = []string{"winter", "spring", "summer", "fall"}

func leaderboardSeasonLabel(seasonID string) string {
	parts := strings.SplitN(seasonID, "_", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return seasonID
	}

	seasonName := map[string]string{
		"winter": "Winter",
		"spring": "Spring",
		"summer": "Summer",
		"fall":   "Fall",
	}[strings.ToLower(parts[0])]
	if seasonName == "" {
		return seasonID
	}

	return fmt.Sprintf("%s %s", seasonName, parts[1])
}

func leaderboardParseSeasonID(seasonID string) (string, int, bool) {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(seasonID)), "_", 2)
	if len(parts) != 2 {
		return "", 0, false
	}

	if parts[0] != "winter" && parts[0] != "spring" && parts[0] != "summer" && parts[0] != "fall" {
		return "", 0, false
	}

	year := 0
	_, err := fmt.Sscanf(parts[1], "%d", &year)
	if err != nil {
		return "", 0, false
	}

	return parts[0], year, true
}

func leaderboardSeasonID(name string, year int) string {
	return fmt.Sprintf("%s_%d", name, year)
}

func leaderboardSeasonIndex(name string) int {
	for i, n := range leaderboardSeasonOrder {
		if n == name {
			return i
		}
	}
	return -1
}

func leaderboardIsBeforeStart(name string, year int) bool {
	if year < leaderboardSeasonStartYear {
		return true
	}
	if year > leaderboardSeasonStartYear {
		return false
	}

	return leaderboardSeasonIndex(name) < leaderboardSeasonIndex(leaderboardSeasonStartName)
}

func leaderboardPreviousSeason(name string, year int) (string, int, bool) {
	idx := leaderboardSeasonIndex(name)
	if idx < 0 {
		return "", 0, false
	}

	idx--
	if idx < 0 {
		idx = len(leaderboardSeasonOrder) - 1
		year--
	}

	return leaderboardSeasonOrder[idx], year, true
}

func leaderboardMostRecentSeason() (string, int, bool) {
	if currentName, currentYear, _ := ei.GetEggIncCurrentSeason(); currentYear >= leaderboardSeasonStartYear {
		currentName = strings.ToLower(strings.TrimSpace(currentName))
		if leaderboardSeasonIndex(currentName) >= 0 {
			return currentName, currentYear, true
		}
	}

	bestName := ""
	bestYear := 0
	bestIdx := -1

	consider := func(seasonID string) {
		name, year, ok := leaderboardParseSeasonID(seasonID)
		if !ok || leaderboardIsBeforeStart(name, year) {
			return
		}

		idx := leaderboardSeasonIndex(name)
		if year > bestYear || (year == bestYear && idx > bestIdx) {
			bestName = name
			bestYear = year
			bestIdx = idx
		}
	}

	for _, c := range ei.GetEggIncContractsSlice() {
		consider(c.SeasonID)
	}
	for _, s := range leaderboardFallbackSeasons {
		consider(s.value)
	}

	if bestName == "" {
		return "", 0, false
	}

	return bestName, bestYear, true
}

// leaderboardSeasons returns all known seasonal scopes from periodicals-loaded contracts,
// with All Time always pinned first. Falls back to a static list until data is loaded.
func leaderboardSeasons() []leaderboardSeason {
	seasons := []leaderboardSeason{{name: "All Time", value: leaderboardAllTimeScope}}
	seen := map[string]struct{}{leaderboardAllTimeScope: {}}

	if name, year, ok := leaderboardMostRecentSeason(); ok {
		for !leaderboardIsBeforeStart(name, year) {
			seasonID := leaderboardSeasonID(name, year)
			if _, exists := seen[seasonID]; !exists {
				seasons = append(seasons, leaderboardSeason{
					name:  leaderboardSeasonLabel(seasonID),
					value: seasonID,
				})
				seen[seasonID] = struct{}{}
			}

			prevName, prevYear, hasPrev := leaderboardPreviousSeason(name, year)
			if !hasPrev {
				break
			}
			name, year = prevName, prevYear
		}
	}

	if len(seasons) > 1 {
		return seasons
	}

	fallback := make([]leaderboardSeason, len(leaderboardFallbackSeasons))
	copy(fallback, leaderboardFallbackSeasons)
	return fallback
}

// GetSlashLeaderboard returns the /leaderboard command
func GetSlashLeaderboard(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Show the leaderboard.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "season",
			Description:  "Season to display. Default is All Time.",
			Autocomplete: true,
		},
	}
	return &command
}

// HandleLeaderboardAutoComplete suggests seasons for the /leaderboard season option.
func HandleLeaderboardAutoComplete(e *dc.AutocompleteEvent) {
	search := ""
	if name, value := e.FocusedOption(); name == "season" {
		search = strings.ToLower(strings.TrimSpace(value))
	}

	choices := make([]dc.Choice[string], 0, leaderboardMaxAutocompleteChoices)
	for _, season := range leaderboardSeasons() {
		if search != "" {
			name := strings.ToLower(season.name)
			value := strings.ToLower(season.value)
			if !strings.Contains(name, search) && !strings.Contains(value, search) {
				continue
			}
		}

		choices = append(choices, dc.Choice[string]{
			Name:  season.name,
			Value: season.value,
		})
		if len(choices) >= leaderboardMaxAutocompleteChoices {
			break
		}
	}

	_ = e.RespondChoices(choices)
}

// HandleLeaderboard posts the leaderboard for the requested season.
func HandleLeaderboard(e *dc.CommandEvent) {
	if !CheckLeaderboardPermission(e) {
		return
	}

	season := leaderboardAllTimeScope
	if opt, ok := e.OptString("season"); ok {
		season = opt
	}

	// Acknowledge the command
	_ = e.Defer(false)

	eiID := farmerstate.GetMiscSettingString(e.UserID(), "encrypted_ei_id")

	components := leaderboardFetchAndBuild(eiID, season, e.GuildID())

	if err := e.Followup(dc.Message{
		Components:      components,
		AllowedMentions: &dc.AllowedMentions{},
	}); err != nil {
		log.Println("Error sending follow-up message /leaderboard:", err)
	}
}

// HandleLeaderboardPage drives the season selector, Refresh and Close controls.
func HandleLeaderboardPage(e *dc.ComponentEvent) {
	respondUsage := func(msg string) {
		_ = e.Respond(dc.Message{Content: msg, Ephemeral: true})
	}

	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		respondUsage("Invalid leaderboard action. Use the season selector, Refresh, or Close controls from a /leaderboard response.")
		return
	}

	userID := e.UserID()
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")

	switch parts[1] {
	case "close":
		// Keep the leaderboard text, drop the two rows of controls.
		kept := e.MessageComponentsWithoutActionRows()

		// Acknowledge update interactions that mutate the existing leaderboard message.
		if err := e.DeferUpdate(); err != nil {
			log.Println("Error responding to leaderboard close interaction:", err)
			return
		}

		if err := e.EditResponse(dc.Message{Components: kept}); err != nil {
			log.Println("Error closing leaderboard:", err)
		}

	case "refresh":
		if len(parts) < 3 {
			respondUsage("Invalid refresh action. Use the Refresh button from a /leaderboard response.")
			return
		}
		if !CheckLeaderboardPermission(e) {
			return
		}

		if err := e.DeferUpdate(); err != nil {
			log.Println("Error responding to leaderboard refresh interaction:", err)
			return
		}

		if eiID == "" {
			_ = e.Followup(dc.Message{
				Content:   fmt.Sprintf("Your Egg Inc ID is needed to update the leaderboard. Use %s to register.", bottools.GetFormattedCommand("register")),
				Ephemeral: true,
			})
			return
		}
		season := parts[2]
		components := leaderboardFetchAndBuild(eiID, season, e.GuildID())
		if err := e.EditResponse(dc.Message{Components: components}); err != nil {
			log.Println("Error refreshing leaderboard:", err)
		}

	case "season":
		if !CheckLeaderboardPermission(e) {
			return
		}

		if err := e.DeferUpdate(); err != nil {
			log.Println("Error responding to leaderboard season interaction:", err)
			return
		}

		if eiID == "" {
			_ = e.Followup(dc.Message{
				Content:   fmt.Sprintf("Your Egg Inc ID is needed to update the leaderboard. Use %s to register.", bottools.GetFormattedCommand("register")),
				Ephemeral: true,
			})
			return
		}
		season := e.Values()[0]
		components := leaderboardFetchAndBuild(eiID, season, e.GuildID())
		if err := e.EditResponse(dc.Message{Components: components}); err != nil {
			log.Println("Error editing leaderboard message:", err)
		}
	default:
		respondUsage("Unknown leaderboard action. Use the season selector, Refresh, or Close controls from a /leaderboard response.")
	}
}

// leaderboardFetchAndBuild fetches the leaderboard data and returns the full component tree.
func leaderboardFetchAndBuild(eiID, season, guildID string) []dc.LayoutComponent {
	var content string

	if eiID == "" {
		content = fmt.Sprintf("Your Egg Inc ID is needed to update the leaderboard. Use %s to register.", bottools.GetFormattedCommand("register"))
	} else {
		resp := ei.GetLeaderboardFromAPI(eiID, season, ei.Contract_GRADE_AAA)
		if resp == nil {
			content = "Failed to fetch leaderboard. Please try again."
		} else {
			content = leaderboardTable(resp, season, farmerstate.GetEiIgnsByGuild(guildID))
		}
	}

	minValues := 1
	seasons := leaderboardSeasons()
	if len(seasons) > leaderboardMaxSeasonOptions {
		seasons = seasons[:leaderboardMaxSeasonOptions]
	}
	options := make([]dc.SelectOption, 0, len(seasons))
	for _, s := range seasons {
		options = append(options, dc.SelectOption{
			Label:   s.name,
			Value:   s.value,
			Default: s.value == season,
		})
	}

	return []dc.LayoutComponent{
		dc.TextDisplay{Content: content},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    fmt.Sprintf("leaderboard#season#%s", season),
					Placeholder: "Select Season",
					MinValues:   &minValues,
					MaxValues:   1,
					Options:     options,
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{
					Label:    "Refresh",
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("leaderboard#refresh#%s", season),
					Emoji:    &dc.Emoji{Name: "🔄"},
				},
				dc.Button{
					Label:    "Close",
					Style:    dc.ButtonDanger,
					CustomID: "leaderboard#close",
				},
			},
		},
	}
}

// leaderboardTable formats the LeaderboardResponse into a markdown string,
// filtered to only guild members when guildNames is non-empty.
func leaderboardTable(resp *ei.LeaderboardResponse, season string, guildNames []string) string {
	var b strings.Builder

	// Collect filtered entries first so we can compute column widths
	type row struct {
		serverRank int
		eiRank     uint32
		name       string
		score      float64
	}
	var rows []row
	serverRank := 0
	for _, entry := range resp.GetTopEntries() {
		alias := ei.NormalizePlayerNameForDisplay(entry.GetAlias())
		if len(guildNames) > 0 && !slices.Contains(guildNames, entry.GetAlias()) {
			continue
		}
		serverRank++
		rows = append(rows, row{serverRank, entry.GetRank(), alias, entry.GetScore()})
	}

	maxNameLen := 0
	maxScore := 0.0
	maxEIRank := uint32(0)
	maxServerRank := len(rows)
	for _, r := range rows {
		if runewidth.StringWidth(r.name) > maxNameLen {
			maxNameLen = runewidth.StringWidth(r.name)
		}
		if r.score > maxScore {
			maxScore = r.score
		}
		if r.eiRank > maxEIRank {
			maxEIRank = r.eiRank
		}
	}
	scoreWidth := max(len(fmt.Sprintf("%.0f", maxScore)), len("Score"))
	eiRankWidth := max(len(fmt.Sprintf("%d", maxEIRank)), len("EI #"))
	serverRankWidth := max(len(fmt.Sprintf("%d", maxServerRank)), len("Rank"))
	nameWidth := max(maxNameLen, len("Name"))

	fmt.Fprintf(&b, "**Leaderboard %s %s**\n-# %d players ranked\n",
		leaderboardSeasonName(season),
		ei.GetBotEmojiMarkdown("contract_grade_AAA"),
		resp.GetCount())
	b.WriteString("```\n")

	// Header row using | as column separator
	header := strings.Join([]string{
		bottools.AlignString("Rank", serverRankWidth+1, bottools.StringAlignLeft),
		bottools.AlignString("EI #", eiRankWidth+1, bottools.StringAlignLeft),
		bottools.AlignString("Name", nameWidth, bottools.StringAlignLeft),
		bottools.AlignString("Score", scoreWidth, bottools.StringAlignRight),
	}, "|")
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("—", len(header)))
	b.WriteString("\n")

	for _, r := range rows {
		row := strings.Join([]string{
			bottools.AlignString(fmt.Sprintf("#%d", r.serverRank), serverRankWidth+1, bottools.StringAlignLeft),
			bottools.AlignString(fmt.Sprintf("#%d", r.eiRank), eiRankWidth+1, bottools.StringAlignLeft),
			bottools.AlignString(r.name, nameWidth, bottools.StringAlignLeft),
			bottools.AlignString(fmt.Sprintf("%.0f", r.score), scoreWidth, bottools.StringAlignRight),
		}, "|")
		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("```")
	return b.String()
}

// leaderboardSeasonName returns the display name for a season scope value.
func leaderboardSeasonName(scope string) string {
	for _, s := range leaderboardSeasons() {
		if s.value == scope {
			return s.name
		}
	}
	if scope == leaderboardAllTimeScope {
		return "All Time"
	}
	if label := leaderboardSeasonLabel(scope); label != scope {
		return label
	}
	return scope
}
