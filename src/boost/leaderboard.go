package boost

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mattn/go-runewidth"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
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
		for {
			if leaderboardIsBeforeStart(name, year) {
				break
			}

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
func GetSlashLeaderboard(cmd string) *discordgo.ApplicationCommand {
	adminPermission := int64(0)

	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Show the leaderboard.",
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "season",
				Description:  "Season to display. Default is All Time.",
				Required:     false,
				Autocomplete: true,
			},
		},
	}
}

// HandleLeaderboardAutoComplete provides typed season suggestions for /leaderboard.
func HandleLeaderboardAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	search := ""
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "season" && opt.Focused {
			search = strings.ToLower(strings.TrimSpace(opt.StringValue()))
			break
		}
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, leaderboardMaxAutocompleteChoices)
	for _, season := range leaderboardSeasons() {
		if search != "" {
			name := strings.ToLower(season.name)
			value := strings.ToLower(season.value)
			if !strings.Contains(name, search) && !strings.Contains(value, search) {
				continue
			}
		}

		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  season.name,
			Value: season.value,
		})
		if len(choices) >= leaderboardMaxAutocompleteChoices {
			break
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

// HandleLeaderboard handles the /leaderboard command
func HandleLeaderboard(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !CheckLeaderboardPermission(s, i) {
		return
	}

	season := leaderboardAllTimeScope
	optionMap := bottools.GetCommandOptionsMap(i)
	if opt, ok := optionMap["season"]; ok {
		season = opt.StringValue()
	}

	flags := discordgo.MessageFlagsIsComponentsV2
	// Acknowledge the command
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: flags,
		},
	})

	userID := bottools.GetInteractionUserID(i)
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")

	components := leaderboardFetchAndBuild(eiID, season, i.GuildID)

	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      flags,
		Components: components,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{},
		},
	})
	if err != nil {
		log.Println("Error sending follow-up message /leaderboard:", err)
	}
}

// HandleLeaderboardPage handles the season select menu, refresh, and close button interactions
func HandleLeaderboardPage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	flags := discordgo.MessageFlagsIsComponentsV2
	respondUsage := func(msg string) {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: msg,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 2 {
		respondUsage("Invalid leaderboard action. Use the season selector, Refresh, or Close controls from a /leaderboard response.")
		return
	}

	userID := bottools.GetInteractionUserID(i)
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")

	switch parts[1] {
	case "close":
		// Acknowledge update interactions that mutate the existing leaderboard message.
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Flags:      flags,
				Components: []discordgo.MessageComponent{},
			},
		})
		if err != nil {
			log.Println("Error responding to leaderboard close interaction:", err)
			return
		}

		// Keep only the TextDisplay, drop the ActionsRows
		var kept []discordgo.MessageComponent
		for _, c := range i.Message.Components {
			if _, ok := c.(*discordgo.TextDisplay); ok {
				kept = append(kept, c)
			}
		}
		edit := discordgo.WebhookEdit{Components: &kept}
		_, err = s.FollowupMessageEdit(i.Interaction, i.Message.ID, &edit)
		if err != nil {
			log.Println("Error closing leaderboard:", err)
		}

	case "refresh":
		if len(parts) < 3 {
			respondUsage("Invalid refresh action. Use the Refresh button from a /leaderboard response.")
			return
		}
		if !CheckLeaderboardPermission(s, i) {
			return
		}

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Flags:      flags,
				Components: []discordgo.MessageComponent{},
			},
		})
		if err != nil {
			log.Println("Error responding to leaderboard refresh interaction:", err)
			return
		}

		if eiID == "" {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("Your Egg Inc ID is needed to update the leaderboard. Use %s to register.", bottools.GetFormattedCommand("register")),
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			return
		}
		season := parts[2]
		components := leaderboardFetchAndBuild(eiID, season, i.GuildID)
		edit := discordgo.WebhookEdit{Components: &components}
		_, err = s.FollowupMessageEdit(i.Interaction, i.Message.ID, &edit)
		if err != nil {
			log.Println("Error refreshing leaderboard:", err)
		}

	case "season":
		if !CheckLeaderboardPermission(s, i) {
			return
		}

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Flags:      flags,
				Components: []discordgo.MessageComponent{},
			},
		})
		if err != nil {
			log.Println("Error responding to leaderboard season interaction:", err)
			return
		}

		if eiID == "" {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("Your Egg Inc ID is needed to update the leaderboard. Use %s to register.", bottools.GetFormattedCommand("register")),
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			return
		}
		season := i.MessageComponentData().Values[0]
		components := leaderboardFetchAndBuild(eiID, season, i.GuildID)
		edit := discordgo.WebhookEdit{Components: &components}
		_, err = s.FollowupMessageEdit(i.Interaction, i.Message.ID, &edit)
		if err != nil {
			log.Println("Error editing leaderboard message:", err)
		}
	default:
		respondUsage("Unknown leaderboard action. Use the season selector, Refresh, or Close controls from a /leaderboard response.")
	}
}

// leaderboardFetchAndBuild fetches the leaderboard data and returns the full component tree.
func leaderboardFetchAndBuild(eiID, season, guildID string) []discordgo.MessageComponent {
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

	min := 1
	seasons := leaderboardSeasons()
	if len(seasons) > leaderboardMaxSeasonOptions {
		seasons = seasons[:leaderboardMaxSeasonOptions]
	}
	options := make([]discordgo.SelectMenuOption, 0, len(seasons))
	for _, s := range seasons {
		options = append(options, discordgo.SelectMenuOption{
			Label:   s.name,
			Value:   s.value,
			Default: s.value == season,
		})
	}

	return []discordgo.MessageComponent{
		&discordgo.TextDisplay{Content: content},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					MenuType:    discordgo.StringSelectMenu,
					CustomID:    fmt.Sprintf("leaderboard#season#%s", season),
					Placeholder: "Select Season",
					MinValues:   &min,
					MaxValues:   1,
					Options:     options,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Refresh",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("leaderboard#refresh#%s", season),
					Emoji:    &discordgo.ComponentEmoji{Name: "🔄"},
				},
				discordgo.Button{
					Label:    "Close",
					Style:    discordgo.DangerButton,
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
	b.WriteString(header + "\n")
	b.WriteString(strings.Repeat("—", len(header)) + "\n")

	for _, r := range rows {
		row := strings.Join([]string{
			bottools.AlignString(fmt.Sprintf("#%d", r.serverRank), serverRankWidth+1, bottools.StringAlignLeft),
			bottools.AlignString(fmt.Sprintf("#%d", r.eiRank), eiRankWidth+1, bottools.StringAlignLeft),
			bottools.AlignString(r.name, nameWidth, bottools.StringAlignLeft),
			bottools.AlignString(fmt.Sprintf("%.0f", r.score), scoreWidth, bottools.StringAlignRight),
		}, "|")
		b.WriteString(row + "\n")
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
