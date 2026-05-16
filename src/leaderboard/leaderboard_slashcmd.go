package leaderboard

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mattn/go-runewidth"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

// GetSlashAdminLBCommand returns the /admin-lb command definition.
func GetSlashAdminLBCommand(cmd string) *discordgo.ApplicationCommand {
	adminPerms := int64(discordgo.PermissionManageGuild)

	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Guild admin commands for leaderboard configuration.",
		DefaultMemberPermissions: &adminPerms,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionSubCommand,
				Name:         "set-channel",
				Description:  "Configure a leaderboard type to post in this channel.",
				Autocomplete: true,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "type",
						Description:  "Leaderboard type or group",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "list",
				Description: "List all configured leaderboards for this guild.",
			},
			{
				Type:         discordgo.ApplicationCommandOptionSubCommand,
				Name:         "remove",
				Description:  "Remove a leaderboard configuration for this guild.",
				Autocomplete: true,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "type",
						Description:  "Leaderboard type to remove",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "run",
				Description: "Trigger an immediate leaderboard collection run (home guild admin only).",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "dry-run",
						Description: "Collect data but skip posting to Discord.",
						Required:    false,
					},
				},
			},
		},
	}
}

// GetSlashLBPlayerCommand returns the /lb command definition.
func GetSlashLBPlayerCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Player commands for leaderboard participation and rankings.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionSubCommand,
				Name:         "opt-in",
				Description:  "Opt into leaderboards.",
				Autocomplete: true,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "type",
						Description:  `Leaderboard type or group, or "all" for everything.`,
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{
				Type:         discordgo.ApplicationCommandOptionSubCommand,
				Name:         "opt-out",
				Description:  "Opt out of leaderboards.",
				Autocomplete: true,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "type",
						Description:  `Leaderboard type or group, or "all" to opt out of everything.`,
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "opt-status",
				Description: "Show your current leaderboard opt-in status.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "opt-list",
				Description: "List all available leaderboard types and their keys.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "rankings",
				Description: "Show your latest leaderboard rankings.",
			},
		},
	}
}

// HandleAdminLB dispatches the /admin-lb slash command.
func HandleAdminLB(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options
	if len(opts) == 0 {
		respondEphemeral(s, i, "Unknown subcommand.")
		return
	}

	userID := bottools.GetInteractionUserID(i)
	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err != nil || perms&discordgo.PermissionManageGuild == 0 {
		respondEphemeral(s, i, "You need the Manage Server permission to use admin commands.")
		return
	}

	switch opts[0].Name {
	case "set-channel":
		handleAdminSetChannel(s, i, opts[0].Options)
	case "list":
		handleAdminList(s, i)
	case "remove":
		handleAdminRemove(s, i, opts[0].Options)
	case "run":
		handleRun(s, i, opts[0].Options)
	default:
		respondEphemeral(s, i, "Unknown admin subcommand.")
	}
}

// HandleLBPlayer dispatches the /lb slash command.
func HandleLBPlayer(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options
	if len(opts) == 0 {
		respondEphemeral(s, i, "Unknown subcommand.")
		return
	}

	switch opts[0].Name {
	case "opt-in":
		handlePlayerOptIn(s, i, opts[0].Options)
	case "opt-out":
		handlePlayerOptOut(s, i, opts[0].Options)
	case "opt-status":
		handlePlayerStatus(s, i)
	case "opt-list":
		handlePlayerList(s, i)
	case "rankings":
		handleRankings(s, i)
	default:
		respondEphemeral(s, i, "Unknown player subcommand.")
	}
}

func handleAdminSetChannel(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	optMap := optionMap(opts)
	lbType := optMap["type"].StringValue()
	// Use the channel where the command was invoked.
	channelID := i.ChannelID

	if !IsValidConfigKey(lbType) {
		respondEphemeral(s, i, fmt.Sprintf("Unknown leaderboard type or group: %q", lbType))
		return
	}

	cfg := LBConfig{
		LBType:    lbType,
		GuildID:   i.GuildID,
		ChannelID: channelID,
	}
	if err := UpsertGuildLBConfig(cfg); err != nil {
		log.Printf("leaderboard: admin set-channel error: %v", err)
		respondEphemeral(s, i, "Failed to save configuration. Please try again.")
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("✅ **%s** leaderboard will post in this channel (<#%s>).",
		DisplayNameForConfigKey(lbType), channelID))
}

func handleAdminList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	allCfgs, err := GetAllLBConfigs()
	if err != nil {
		respondEphemeral(s, i, "Failed to load leaderboard configurations.")
		return
	}

	var cfgs []LBConfig
	for _, c := range allCfgs {
		if c.GuildID == i.GuildID {
			cfgs = append(cfgs, c)
		}
	}
	if len(cfgs) == 0 {
		respondEphemeral(s, i, "No leaderboards configured for this guild.\nUse `/bock-leaderboard admin set-channel` to add one.")
		return
	}

	var b strings.Builder
	b.WriteString("**Configured leaderboards for this guild:**\n")
	for _, cfg := range cfgs {
		fmt.Fprintf(&b, "• **%s** → <#%s>\n", DisplayNameForConfigKey(cfg.LBType), cfg.ChannelID)
	}
	respondEphemeral(s, i, b.String())
}

func handleAdminRemove(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	optMap := optionMap(opts)
	lbType := optMap["type"].StringValue()

	if !IsValidConfigKey(lbType) {
		respondEphemeral(s, i, fmt.Sprintf("Unknown leaderboard type or group: %q", lbType))
		return
	}

	if err := DeleteGuildLBConfig(i.GuildID, lbType); err != nil {
		log.Printf("leaderboard: admin remove error: %v", err)
		respondEphemeral(s, i, "Failed to remove configuration.")
		return
	}
	respondEphemeral(s, i, fmt.Sprintf("✅ Removed **%s** leaderboard configuration.\n-# The Discord messages were not deleted.",
		DisplayNameForConfigKey(lbType)))
}

func handlePlayerOptIn(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	userID := bottools.GetInteractionUserID(i)
	raw := optionMap(opts)["type"].StringValue()

	var types []string
	if strings.ToLower(raw) == "all" {
		types = []string{OptInAll}
	} else {
		types = ExpandConfigKey(raw)
	}

	AddPlayerOptInTypes(i.GuildID, userID, types)

	if len(types) == 1 && types[0] == OptInAll {
		respondEphemeral(s, i, "✅ You are now opted into **all** leaderboards.")
		return
	}
	names := typeKeysToNames(types)
	respondEphemeral(s, i, fmt.Sprintf("✅ Opted into: %s", strings.Join(names, ", ")))
}

func handlePlayerOptOut(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	userID := bottools.GetInteractionUserID(i)
	raw := optionMap(opts)["type"].StringValue()

	var types []string
	if strings.ToLower(raw) == "all" {
		types = []string{OptInAll}
	} else {
		types = ExpandConfigKey(raw)
	}

	RemovePlayerOptInTypes(i.GuildID, userID, types)

	if len(types) == 1 && types[0] == OptInAll {
		respondEphemeral(s, i, "✅ You have opted out of **all** leaderboards.")
		return
	}
	names := typeKeysToNames(types)
	respondEphemeral(s, i, fmt.Sprintf("✅ Opted out of: %s", strings.Join(names, ", ")))
}

func handlePlayerStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	guildID := i.GuildID
	if guildID == "" {
		respondEphemeral(s, i, "This command must be used within a server.")
		return
	}
	storedVal := optInRaw(guildID, userID)
	if storedVal == "" {
		respondEphemeral(s, i, "You are not opted into any leaderboards.\nUse `/bock-leaderboard player optin types:all` to join everything.")
		return
	}
	if storedVal == OptInAll {
		respondEphemeral(s, i, "You are opted into **all** leaderboards.")
		return
	}
	types := GetPlayerOptInTypes(guildID, userID)
	names := typeKeysToNames(types)
	respondEphemeral(s, i, fmt.Sprintf("**Your leaderboard opt-ins (%d):**\n%s",
		len(names), strings.Join(names, "\n")))
}

func handlePlayerList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	showListPage(s, i, 0)
}

func showListPage(s *discordgo.Session, i *discordgo.InteractionCreate, page int) {
	const pageSize = 15
	start := page * pageSize
	if start < 0 {
		start = 0
		page = 0
	}
	if start >= len(AllLeaderboards) {
		start = (len(AllLeaderboards) - 1) / pageSize * pageSize
		page = (len(AllLeaderboards) - 1) / pageSize
	}
	end := start + pageSize
	if end > len(AllLeaderboards) {
		end = len(AllLeaderboards)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**Available leaderboard types (Page %d/%d):**\n", page+1, (len(AllLeaderboards)+pageSize-1)/pageSize)
	b.WriteString("```\n")
	for _, def := range AllLeaderboards[start:end] {
		fmt.Fprintf(&b, "%-22s  %s\n", def.Key, def.DisplayName)
	}
	b.WriteString("```")

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Previous",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("lb_list#%d", page-1),
					Disabled: page <= 0,
				},
				discordgo.Button{
					Label:    "Next",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("lb_list#%d", page+1),
					Disabled: end >= len(AllLeaderboards),
				},
			},
		},
	}

	var flags discordgo.MessageFlags
	if i.Type != discordgo.InteractionMessageComponent {
		flags |= discordgo.MessageFlagsEphemeral
	}

	var err error
	if i.Type == discordgo.InteractionMessageComponent {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    b.String(),
				Components: components,
				Flags:      flags,
			},
		})
	} else {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    b.String(),
				Components: components,
				Flags:      flags,
			},
		})
	}
	if err != nil {
		log.Printf("leaderboard: failed to showListPage: %v", err)
	}
}

// HandleLBListComponent handles button clicks for the leaderboard list pagination.
func HandleLBListComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, "#")
	if len(parts) < 2 {
		return
	}
	page, _ := strconv.Atoi(parts[1])
	showListPage(s, i, page)
}

// ─── run command ──────────────────────────────────────────────────────────────

func handleRun(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	userID := bottools.GetInteractionUserID(i)

	// Restrict to home-guild admin only.
	homeGuild := guildstate.GetGuildSettingString("DEFAULT", "home_guild")
	if i.GuildID != homeGuild && userID != config.AdminUserID {
		respondEphemeral(s, i, "This command is restricted to the bot's home guild admin.")
		return
	}
	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err != nil || (perms&discordgo.PermissionAdministrator == 0 && userID != config.AdminUserID) {
		respondEphemeral(s, i, "You need Administrator permission to trigger a collection run.")
		return
	}

	dryRun := false
	optMap := optionMap(opts)
	if opt, ok := optMap["dry-run"]; ok {
		dryRun = opt.BoolValue()
	}

	// Immediate response to confirm we're starting.
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "**Starting collection run...**",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	onProgress := func(status string) {
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &status,
		})
	}

	go func() {
		RunLeaderboardCollection(s, dryRun, onProgress)
		msg := "✅ Leaderboard collection run complete."
		if dryRun {
			msg = "✅ Dry run complete — data collected, Discord post skipped."
		}
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
	}()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func optionMap(opts []*discordgo.ApplicationCommandInteractionDataOption) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	m := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(opts))
	for _, o := range opts {
		m[o.Name] = o
	}
	return m
}
func typeKeysToNames(keys []string) []string {
	var names []string
	for _, k := range keys {
		if def, ok := LBDefByKey(k); ok {
			names = append(names, def.DisplayName)
		} else {
			names = append(names, k)
		}
	}
	return names
}

// optInRaw returns the raw stored opt-in string for a user (for status display).
func optInRaw(guildID, userID string) string {
	types := GetPlayerOptInTypes(guildID, userID)
	if len(types) == 0 {
		return ""
	}
	return strings.Join(types, ",")
}

// groupMemberKeys is the set of individual lb_type keys that belong to at least
// one group. Built once at package init so the autocomplete can skip them.
var groupMemberKeys = func() map[string]struct{} {
	m := make(map[string]struct{})
	for _, g := range AllGroups {
		for _, k := range g.Members {
			m[k] = struct{}{}
		}
	}
	return m
}()

// HandleAdminLBAutoComplete handles autocomplete for the /admin-lb command.
func HandleAdminLBAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	var partial string
	var found bool
	for _, opt := range data.Options {
		for _, leaf := range opt.Options {
			if leaf.Focused {
				partial = strings.ToLower(strings.TrimSpace(leaf.StringValue()))
				found = true
			}
		}
	}
	if !found {
		respondEmptyAutocomplete(s, i)
		return
	}

	choices := buildAutocompleteChoices(partial, false)
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

// HandleLBPlayerAutoComplete handles autocomplete for the /lb command.
func HandleLBPlayerAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	var partial string
	var found bool
	for _, opt := range data.Options {
		for _, leaf := range opt.Options {
			if leaf.Focused {
				partial = strings.ToLower(strings.TrimSpace(leaf.StringValue()))
				found = true
			}
		}
	}
	if !found {
		respondEmptyAutocomplete(s, i)
		return
	}

	choices := buildAutocompleteChoices(partial, true)
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

func respondEmptyAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
	})
}

func buildAutocompleteChoices(partial string, isPlayerCmd bool) []*discordgo.ApplicationCommandOptionChoice {
	matches := func(name, key string) bool {
		return partial == "" ||
			strings.Contains(strings.ToLower(name), partial) ||
			strings.Contains(key, partial)
	}

	const maxChoices = 25
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, maxChoices)

	if isPlayerCmd && matches("All Leaderboards", "all") {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "All Leaderboards",
			Value: "all",
		})
	}

	// Groups first.
	for _, g := range AllGroups {
		if len(choices) >= maxChoices {
			break
		}
		if matches(g.DisplayName, g.Key) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  g.DisplayName,
				Value: g.Key,
			})
		}
	}

	// Individual types.
	for _, def := range AllLeaderboards {
		if len(choices) >= maxChoices {
			break
		}
		// In admin commands, hide types that are covered by groups to declutter.
		// In player commands, show everything.
		if !isPlayerCmd {
			if _, inGroup := groupMemberKeys[def.Key]; inGroup {
				continue
			}
		}
		if matches(def.DisplayName, def.Key) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  def.DisplayName,
				Value: def.Key,
			})
		}
	}
	return choices
}

func handleRankings(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Acknowledge immediately to avoid timeout.
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	showRankingsPage(s, i, 0)
}

func showRankingsPage(s *discordgo.Session, i *discordgo.InteractionCreate, page int) {
	userID := bottools.GetInteractionUserID(i)
	guildID := i.GuildID
	if guildID == "" {
		respondEphemeral(s, i, "This command must be used within a server.")
		return
	}

	optedIn := GetPlayerOptInTypes(guildID, userID)
	optedSet := make(map[string]struct{}, len(optedIn))
	for _, k := range optedIn {
		optedSet[k] = struct{}{}
	}

	allStats := GetPlayerStats(guildID, userID)
	var stats []PlayerStat
	for _, st := range allStats {
		if _, ok := optedSet[st.Def.Key]; ok {
			stats = append(stats, st)
		}
	}

	if len(stats) == 0 {
		content := "You don't have any leaderboard rankings recorded yet for metrics you are opted into in this server."
		if i.Type == discordgo.InteractionMessageComponent {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: content,
				},
			})
		} else {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: content,
			})
		}
		return
	}

	const pageSize = 10
	start := page * pageSize
	if start < 0 {
		start = 0
		page = 0
	}
	if start >= len(stats) {
		start = (len(stats) - 1) / pageSize * pageSize
		page = (len(stats) - 1) / pageSize
	}
	end := start + pageSize
	if end > len(stats) {
		end = len(stats)
	}

	maxRankWidth := 3 // "#"
	maxNameWidth := 6 // "Metric"
	maxValOnlyWidth := 5
	maxDeltaWidth := 0

	type row struct {
		rank    string
		name    string
		val     string
		delta   string
		details string
		link    string
		label   string
	}
	var pageRows []row

	// Map lbType -> Discord link for jump-to functionality.
	lbLinks := make(map[string]string)
	if i.GuildID != "" {
		cfgs, _ := guildstate.GetAllLeaderboardConfigsForGuild(i.GuildID)
		for _, c := range cfgs {
			keys := ExpandConfigKey(c.LbType)
			var messageIDs []string
			if c.MessageIds.Valid && c.MessageIds.String != "" {
				_ = json.Unmarshal([]byte(c.MessageIds.String), &messageIDs)
			}
			if len(messageIDs) > 0 {
				link := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, c.ChannelID, messageIDs[0])
				for _, k := range keys {
					lbLinks[k] = link
				}
			}
		}
	}

	for i, st := range stats[start:end] {
		v := FormatLBValue(st.Def.ValueFmt, st.Current.Value)
		d := ""
		if st.HasPrev {
			d = FormatLBDelta(st.Def.ValueFmt, st.Current.Value-st.PrevVal)
		}

		rankStr := fmt.Sprintf("#%d", st.Rank)
		if st.Rank == 0 {
			rankStr = "-"
		}
		if len(rankStr) > maxRankWidth {
			maxRankWidth = len(rankStr)
		}

		displayName := fmt.Sprintf("%d. %s", i+1, st.Def.DisplayName)
		w := runewidth.StringWidth(displayName)
		if w > maxNameWidth {
			maxNameWidth = w
		}

		if len(v) > maxValOnlyWidth {
			maxValOnlyWidth = len(v)
		}
		if len(d) > maxDeltaWidth {
			maxDeltaWidth = len(d)
		}

		pageRows = append(pageRows, row{
			rank:    rankStr,
			name:    displayName,
			val:     v,
			delta:   d,
			details: st.Current.Details,
			link:    lbLinks[st.Def.Key],
			label:   fmt.Sprintf("[%d]", i+1),
		})
	}

	maxValWidth := maxValOnlyWidth
	if maxDeltaWidth > 0 {
		maxValWidth += 1 + maxDeltaWidth
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## 📊 Rankings for %s (Page %d/%d)\n", stats[0].Current.GameName, page+1, (len(stats)+pageSize-1)/pageSize)
	b.WriteString("```\n")
	fmt.Fprintf(&b, "%s|%s|%s\n",
		bottools.AlignString("#", maxRankWidth, bottools.StringAlignLeft),
		bottools.AlignString("Metric", maxNameWidth, bottools.StringAlignLeft),
		bottools.AlignString("Value", maxValWidth, bottools.StringAlignRight))
	b.WriteString(strings.Repeat("—", maxRankWidth+maxNameWidth+maxValWidth+2) + "\n")

	for _, r := range pageRows {
		displayVal := bottools.AlignString(r.val, maxValOnlyWidth, bottools.StringAlignRight)
		if maxDeltaWidth > 0 {
			if r.delta != "" {
				displayVal += " " + bottools.AlignString(r.delta, maxDeltaWidth, bottools.StringAlignLeft)
			} else {
				displayVal += strings.Repeat(" ", maxDeltaWidth+1)
			}
		}

		detail := ""
		if r.details != "" && !strings.HasPrefix(r.details, "total:") {
			detail = fmt.Sprintf(" (%s)", r.details)
		}

		fmt.Fprintf(&b, "%s|%s|%s%s\n",
			bottools.AlignString(r.rank, maxRankWidth, bottools.StringAlignLeft),
			bottools.AlignString(r.name, maxNameWidth, bottools.StringAlignLeft),
			displayVal,
			detail)
	}
	b.WriteString("```")

	var links []string
	for _, r := range pageRows {
		if r.link != "" {
			links = append(links, fmt.Sprintf("[%s](%s)", r.label, r.link))
		}
	}
	if len(links) > 0 {
		fmt.Fprintf(&b, "\n**Jump to:** %s", strings.Join(links, " | "))
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Previous",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("lb_stats#%d", page-1),
					Disabled: page <= 0,
				},
				discordgo.Button{
					Label:    "Next",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("lb_stats#%d", page+1),
					Disabled: end >= len(stats),
				},
			},
		},
	}

	var flags discordgo.MessageFlags
	var err error
	fullText := b.String()
	if i.Type == discordgo.InteractionMessageComponent {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    fullText,
				Components: components,
				Flags:      flags,
			},
		})
	} else {
		_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content:    &fullText,
			Components: &components,
		})
	}
	if err != nil {
		log.Printf("leaderboard: failed to showRankingsPage for %s: %v", userID, err)
	}
}

// HandleLBStatsComponent handles button clicks for the leaderboard rankings pagination.
func HandleLBStatsComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, "#")
	if len(parts) < 2 {
		return
	}
	page, _ := strconv.Atoi(parts[1])
	showRankingsPage(s, i, page)
}
