package leaderboard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

// GetSlashAdminLBCommand returns the /admin-lb command definition.
func GetSlashAdminLBCommand(cmd string) *dc.Command {
	adminPerms := dc.PermissionManageGuild

	command := dc.Command{
		Name:                     cmd,
		Description:              "Guild admin commands for leaderboard configuration.",
		DefaultMemberPermissions: &adminPerms,
		Contexts:                 []dc.InteractionContext{dc.ContextGuild},
		IntegrationTypes:         []dc.IntegrationType{dc.IntegrationGuildInstall},
		Options: []dc.Option{
			dc.SubCommand{
				Name:        "set-channel",
				Description: "Configure a leaderboard type to post in this channel.",
				Options: []dc.Option{
					dc.StringOption{
						Name:         "type",
						Description:  "Leaderboard type or group",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			dc.SubCommand{
				Name:        "backfill-eggday",
				Description: "Manually set Egg Day start/end SE for a player and recalculate.",
				Options: []dc.Option{
					dc.UserOption{Name: "user", Description: "Discord user to backfill", Required: true},
					dc.StringOption{Name: "start", Description: "Start SE value (e.g., 23.45s or 12345)"},
					dc.StringOption{Name: "end", Description: "End SE value (e.g., 23.45s or 12345)"},
					dc.IntOption{Name: "year", Description: "Year for the Egg Day snap (e.g., 2026). Defaults to current year."},
				},
			},
			dc.SubCommand{
				Name:        "list",
				Description: "List all configured leaderboards for this guild.",
			},
			dc.SubCommand{
				Name:        "remove",
				Description: "Remove a leaderboard configuration for this guild.",
				Options: []dc.Option{
					dc.StringOption{
						Name:         "type",
						Description:  "Leaderboard type to remove",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
			dc.SubCommand{
				Name:        "run",
				Description: "Trigger an immediate leaderboard collection run (home guild admin only).",
				Options: []dc.Option{
					dc.BoolOption{
						Name:        "dry-run",
						Description: "Collect data but skip posting to Discord.",
					},
					dc.StringOption{
						Name:         "target",
						Description:  "Select any group or single leaderboard to update",
						Autocomplete: true,
					},
					dc.StringOption{
						Name:        "action",
						Description: "Update behavior for Discord messages",
						Choices: []dc.Choice[string]{
							{Name: "Update Original Messages", Value: "update"},
							{Name: "Bump Messages", Value: "bump"},
							{Name: "New Messages", Value: "new"},
						},
					},
				},
			},
		},
	}
	return &command
}

// GetSlashLBPlayerCommand returns the /lb command definition.
func GetSlashLBPlayerCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:             cmd,
		Description:      "Player commands for leaderboard participation and rankings.",
		Contexts:         []dc.InteractionContext{dc.ContextGuild},
		IntegrationTypes: []dc.IntegrationType{dc.IntegrationGuildInstall},
		Options: []dc.Option{
			dc.SubCommand{
				Name:        "opt-in",
				Description: "Opt into leaderboards.",
				Options: []dc.Option{
					dc.StringOption{
						Name:         "type",
						Description:  `Leaderboard type or group, or "all" for everything.`,
						Required:     true,
						Autocomplete: true,
					},
					dc.StringOption{
						Name:         "alt",
						Description:  "The name of the alternate account (optional).",
						Autocomplete: true,
					},
				},
			},
			dc.SubCommand{
				Name:        "opt-out",
				Description: "Opt out of leaderboards.",
				Options: []dc.Option{
					dc.StringOption{
						Name:         "type",
						Description:  `Leaderboard type or group, or "all" to opt out of everything.`,
						Required:     true,
						Autocomplete: true,
					},
					dc.StringOption{
						Name:         "alt",
						Description:  "The name of the alternate account (optional).",
						Autocomplete: true,
					},
				},
			},
			dc.SubCommand{
				Name:        "opt-status",
				Description: "Show your current leaderboard opt-in status.",
				Options: []dc.Option{
					dc.StringOption{
						Name:         "alt",
						Description:  "The name of the alternate account (optional).",
						Autocomplete: true,
					},
				},
			},
			dc.SubCommand{
				Name:        "opt-list",
				Description: "List all available leaderboard types and their keys.",
			},
			dc.SubCommand{
				Name:        "rankings",
				Description: "Show your latest leaderboard rankings.",
			},
		},
	}
	return &command
}

func HandleAdminLB(client dc.Client, e *dc.CommandEvent) {
	sub, ok := e.Subcommand()
	if !ok {
		respondEphemeral(e, "Unknown subcommand.")
		return
	}

	userID := e.UserID()
	perms, err := client.UserChannelPermissions(userID, e.ChannelID())
	if err != nil || !perms.Administrator() {
		respondEphemeral(e, "You need the Administrator permission to use admin commands.")
		return
	}

	switch sub {
	case "set-channel":
		handleAdminSetChannel(e)
	case "list":
		handleAdminList(e)
	case "remove":
		handleAdminRemove(e)
	case "run":
		handleRun(client, e)
	case "backfill-eggday":
		handleAdminBackfillEggDay(client, e)
	default:
		respondEphemeral(e, "Unknown admin subcommand.")
	}
}

func handleAdminBackfillEggDay(client dc.Client, e *dc.CommandEvent) {
	// Get user
	var userID string
	if u, ok := e.OptUser("backfill-eggday-user"); ok {
		userID = u.ID
	}
	if userID == "" {
		respondEphemeral(e, "Please specify a user to backfill.")
		return
	}

	startStr := ""
	endStr := ""
	if so, ok := e.OptString("backfill-eggday-start"); ok {
		startStr = strings.TrimSpace(so)
	}
	if eo, ok := e.OptString("backfill-eggday-end"); ok {
		endStr = strings.TrimSpace(eo)
	}

	if startStr == "" && endStr == "" {
		respondEphemeral(e, "No start or end value provided — nothing to do.")
		return
	}

	loc, _ := time.LoadLocation("America/Los_Angeles")
	year := time.Now().In(loc).Year()
	if y, ok := e.OptInt("backfill-eggday-year"); ok {
		// Use provided year if set
		if y > 0 {
			year = y
		}
	}
	yearStr := fmt.Sprintf("%d", year)

	// Determine gameName from existing stats or API fallback
	var gameName string
	if st := GetStatForPlayerAndSnapDate("egg_day_se_start", userID, yearStr); st != nil {
		gameName = st.GameName
	}
	if gameName == "" {
		if et := GetStatForPlayerAndSnapDate("egg_day_se_end", userID, yearStr); et != nil {
			gameName = et.GameName
		}
	}
	if gameName == "" {
		enc := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		if enc != "" {
			if backup, _ := ei.GetFirstContactFromAPI(enc, userID, true); backup != nil && backup.GetGame() != nil {
				gameName = ei.NormalizePlayerNameForDisplay(backup.GetUserName())
				if backup.GetGame().GetPermitLevel() != 1 {
					gameName += " (SP)"
				}
			}
		}
	}
	if gameName == "" {
		gameName = userID
	}

	var updated []string

	// Parse and upsert start
	if startStr != "" {
		v, err := ei.ParseValueWithUnit(startStr, false)
		if err != nil {
			respondEphemeral(e, fmt.Sprintf("Failed to parse start value: %v", err))
			return
		}
		if err := farmerstate.UpsertLeaderboardStat("egg_day_se_start", userID, gameName, yearStr, v, sql.NullString{}); err != nil {
			respondEphemeral(e, fmt.Sprintf("Failed to save start stat: %v", err))
			return
		}
		updated = append(updated, fmt.Sprintf("start=%g", v))
	}

	// Parse and upsert end
	if endStr != "" {
		v, err := ei.ParseValueWithUnit(endStr, false)
		if err != nil {
			respondEphemeral(e, fmt.Sprintf("Failed to parse end value: %v", err))
			return
		}
		if err := farmerstate.UpsertLeaderboardStat("egg_day_se_end", userID, gameName, yearStr, v, sql.NullString{}); err != nil {
			respondEphemeral(e, fmt.Sprintf("Failed to save end stat: %v", err))
			return
		}
		updated = append(updated, fmt.Sprintf("end=%g", v))
	}

	// Recalculate gain/pct if both start and end present
	startStat := GetStatForPlayerAndSnapDate("egg_day_se_start", userID, yearStr)
	endStat := GetStatForPlayerAndSnapDate("egg_day_se_end", userID, yearStr)
	if startStat != nil && endStat != nil {
		seStart := startStat.Value
		seEnd := endStat.Value
		gain := seEnd - seStart
		if gain < 0 {
			gain = 0
		}
		var pct float64
		if seStart > 0 {
			pct = (gain / seStart) * 100.0
		}
		if err := farmerstate.UpsertLeaderboardStat(LBEggDaySEGain, userID, gameName, yearStr, gain, sql.NullString{}); err != nil {
			respondEphemeral(e, fmt.Sprintf("Failed to save gain stat: %v", err))
			return
		}
		if err := farmerstate.UpsertLeaderboardStat(LBEggDaySEPct, userID, gameName, yearStr, pct, sql.NullString{}); err != nil {
			respondEphemeral(e, fmt.Sprintf("Failed to save pct stat: %v", err))
			return
		}
		updated = append(updated, fmt.Sprintf("gain=%g", gain))
		updated = append(updated, fmt.Sprintf("pct=%.2f", pct))
	}

	// Redraw Egg Day leaderboards for everyone
	go PostLeaderboards(client, yearStr, "", "group_egg_day", "update", nil)

	respondEphemeral(e, fmt.Sprintf("Backfill complete for <@%s>: %s", userID, strings.Join(updated, ", ")))
}

func HandleLBPlayer(client dc.Client, e *dc.CommandEvent) {
	sub, ok := e.Subcommand()
	if !ok {
		respondEphemeral(e, "Unknown subcommand.")
		return
	}

	switch sub {
	case "opt-in":
		handlePlayerOptIn(client, e)
	case "opt-out":
		handlePlayerOptOut(e)
	case "opt-status":
		handlePlayerStatus(e)
	case "opt-list":
		handlePlayerList(e)
	case "rankings":
		handleRankings(e)
	default:
		respondEphemeral(e, "Unknown player subcommand.")
	}
}

func handleAdminSetChannel(e *dc.CommandEvent) {
	lbType, _ := e.OptString("set-channel-type")
	// Use the channel where the command was invoked.
	channelID := e.ChannelID()

	if !IsValidConfigKey(lbType) {
		respondEphemeral(e, fmt.Sprintf("Unknown leaderboard type or group: %q", lbType))
		return
	}

	cfg := LBConfig{
		LBType:    lbType,
		GuildID:   e.GuildID(),
		ChannelID: channelID,
	}
	if err := UpsertGuildLBConfig(cfg); err != nil {
		log.Printf("leaderboard: admin set-channel error: %v", err)
		respondEphemeral(e, "Failed to save configuration. Please try again.")
		return
	}

	respondEphemeral(e, fmt.Sprintf("✅ **%s** leaderboard will post in this channel (<#%s>).",
		DisplayNameForConfigKey(lbType), channelID))
}

func handleAdminList(e *dc.CommandEvent) {
	allCfgs, err := GetAllLBConfigs()
	if err != nil {
		respondEphemeral(e, "Failed to load leaderboard configurations.")
		return
	}

	var cfgs []LBConfig
	for _, c := range allCfgs {
		if c.GuildID == e.GuildID() {
			cfgs = append(cfgs, c)
		}
	}
	if len(cfgs) == 0 {
		respondEphemeral(e, "No leaderboards configured for this guild.\nUse `/bock-leaderboard admin set-channel` to add one.")
		return
	}

	var b strings.Builder
	b.WriteString("**Configured leaderboards for this guild:**\n")
	for _, cfg := range cfgs {
		fmt.Fprintf(&b, "• **%s** → <#%s>\n", DisplayNameForConfigKey(cfg.LBType), cfg.ChannelID)
	}
	respondEphemeral(e, b.String())
}

func handleAdminRemove(e *dc.CommandEvent) {
	lbType, _ := e.OptString("remove-type")

	if !IsValidConfigKey(lbType) {
		respondEphemeral(e, fmt.Sprintf("Unknown leaderboard type or group: %q", lbType))
		return
	}

	if err := DeleteGuildLBConfig(e.GuildID(), lbType); err != nil {
		log.Printf("leaderboard: admin remove error: %v", err)
		respondEphemeral(e, "Failed to remove configuration.")
		return
	}
	respondEphemeral(e, fmt.Sprintf("✅ Removed **%s** leaderboard configuration.\n-# The Discord messages were not deleted.",
		DisplayNameForConfigKey(lbType)))
}

func handlePlayerOptIn(client dc.Client, e *dc.CommandEvent) {
	userID := e.UserID()
	raw, _ := e.OptString("opt-in-type")
	if alt, ok := e.OptString("opt-in-alt"); ok && alt != "" {
		userID = alt
	}

	var types []string
	if strings.ToLower(raw) == "all" {
		types = []string{OptInAll}
	} else {
		types = ExpandConfigKey(raw)
	}

	AddPlayerOptInTypes(e.GuildID(), userID, types)

	guildID := e.GuildID()
	go func() {
		snapDate := GetLatestSnapDate(LBContractExp)
		if snapDate == "" {
			snapDate = SnapDateNow()
		}
		log.Printf("leaderboard: pulling stats for newly opted-in user %s in guild %s with snapDate %s", userID, guildID, snapDate)
		if err := CollectSinglePlayer(userID, snapDate); err != nil {
			log.Printf("leaderboard: failed to collect stats for user %s on opt-in: %v", userID, err)
			return
		}
		log.Printf("leaderboard: refreshing leaderboard messages for guild %s with snapDate %s", guildID, snapDate)
		PostLeaderboards(client, snapDate, guildID, "", "update", nil)
	}()

	if len(types) == 1 && types[0] == OptInAll {
		respondEphemeral(e, "✅ You are now opted into **all** leaderboards.")
		return
	}
	names := typeKeysToNames(types)
	respondEphemeral(e, fmt.Sprintf("✅ Opted into: %s", strings.Join(names, ", ")))
}

func handlePlayerOptOut(e *dc.CommandEvent) {
	userID := e.UserID()
	raw, _ := e.OptString("opt-out-type")
	if alt, ok := e.OptString("opt-out-alt"); ok && alt != "" {
		userID = alt
	}

	var types []string
	if strings.ToLower(raw) == "all" {
		types = []string{OptInAll}
	} else {
		types = ExpandConfigKey(raw)
	}

	RemovePlayerOptInTypes(e.GuildID(), userID, types)

	if len(types) == 1 && types[0] == OptInAll {
		respondEphemeral(e, "✅ You have opted out of **all** leaderboards.")
		return
	}
	names := typeKeysToNames(types)
	respondEphemeral(e, fmt.Sprintf("✅ Opted out of: %s", strings.Join(names, ", ")))
}

func handlePlayerStatus(e *dc.CommandEvent) {
	userID := e.UserID()
	if alt, ok := e.OptString("opt-status-alt"); ok && alt != "" {
		userID = alt
	}

	guildID := e.GuildID()
	if guildID == "" {
		respondEphemeral(e, "This command must be used within a server.")
		return
	}
	storedVal := optInRaw(guildID, userID)
	if storedVal == "" {
		respondEphemeral(e, "You are not opted into any leaderboards.\nUse `/bock-leaderboard player optin types:all` to join everything.")
		return
	}
	if storedVal == OptInAll {
		respondEphemeral(e, "You are opted into **all** leaderboards.")
		return
	}
	types := GetPlayerOptInTypes(guildID, userID)
	names := typeKeysToNames(types)
	respondEphemeral(e, fmt.Sprintf("**Your leaderboard opt-ins (%d):**\n%s",
		len(names), strings.Join(names, "\n")))
}

func handlePlayerList(e *dc.CommandEvent) {
	showListPage(e, 0)
}

// showListPage answers either the /lb opt-list command or a click on one of
// its pagination buttons: a command gets a fresh ephemeral message, a button
// replaces the message it lives on.
func showListPage(e dc.InteractionEvent, page int) {
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

	// Content alongside a button row, so this stays on Discord's v1 component
	// model.
	msg := dc.Message{
		Content:      b.String(),
		ComponentsV1: true,
		Components: []dc.LayoutComponent{
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.Button{
						Label:    "Previous",
						Style:    dc.ButtonSecondary,
						CustomID: fmt.Sprintf("lb_list#%d", page-1),
						Disabled: page <= 0,
					},
					dc.Button{
						Label:    "Next",
						Style:    dc.ButtonSecondary,
						CustomID: fmt.Sprintf("lb_list#%d", page+1),
						Disabled: end >= len(AllLeaderboards),
					},
				},
			},
		},
	}

	var err error
	switch ev := e.(type) {
	case *dc.ComponentEvent:
		err = ev.Update(msg)
	case *dc.CommandEvent:
		msg.Ephemeral = true
		err = ev.Respond(msg)
	}
	if err != nil {
		log.Printf("leaderboard: failed to showListPage: %v", err)
	}
}

func HandleLBListComponent(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	page, _ := strconv.Atoi(parts[1])
	showListPage(e, page)
}

// ─── run command ──────────────────────────────────────────────────────────────

func handleRun(client dc.Client, e *dc.CommandEvent) {
	userID := e.UserID()

	perms, err := client.UserChannelPermissions(userID, e.ChannelID())
	if err != nil || !perms.Administrator() {
		respondEphemeral(e, "You need Administrator permission to trigger a collection run.")
		return
	}

	dryRun := false
	target := ""
	action := "update"
	if opt, ok := e.OptBool("run-dry-run"); ok {
		dryRun = opt
	}
	if opt, ok := e.OptString("run-target"); ok {
		target = opt
	}
	if opt, ok := e.OptString("run-action"); ok {
		action = opt
	}

	// Immediate response to confirm we're starting.
	msg := "**Starting collection run...**"
	if dryRun {
		msg += "\n-# Dry-run skips Discord posting."
	}
	respondEphemeral(e, msg)

	onProgress := func(status string) {
		_ = e.EditResponse(dc.Message{Content: status})
	}

	guildID := e.GuildID()
	go func() {
		RunLeaderboardCollection(client, dryRun, guildID, target, action, onProgress)
		finalMsg := "✅ Leaderboard collection run complete."
		if dryRun {
			finalMsg = "✅ Dry run complete — data collected, Discord post skipped."
		}
		_ = e.EditResponse(dc.Message{Content: finalMsg})
	}()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func respondEphemeral(e *dc.CommandEvent, msg string) {
	_ = e.Respond(dc.Message{Content: msg, Ephemeral: true})
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

// lbGroupsByKey maps individual leaderboard keys to the short labels of groups
// they belong to (for autocomplete display tags like "(CS,MISC)").
var lbGroupsByKey = func() map[string][]string {
	m := make(map[string][]string)
	for _, g := range AllGroups {
		abbr := shortGroupLabel(g)
		for _, k := range g.Members {
			m[k] = append(m[k], abbr)
		}
	}
	return m
}()

func shortGroupLabel(g LBGroup) string {
	switch g.Key {
	case "group_core":
		return "CORE"
	case "group_cs_stats":
		return "CS"
	case "group_misc":
		return "MISC"
	case "group_virtue_eggs":
		return "VIRTUE"
	case "group_ships_std":
		return "SHIP_STD"
	case "group_ships_virtue":
		return "SHIP_VIRTUE"
	default:
		if g.DisplayName == "" {
			return "GROUP"
		}
		parts := strings.Fields(g.DisplayName)
		if len(parts) == 0 {
			return "GROUP"
		}
		return strings.ToUpper(parts[0])
	}
}

func leaderboardChoiceName(def LBDef) string {
	if groups := lbGroupsByKey[def.Key]; len(groups) > 0 {
		return fmt.Sprintf("%s (%s)", def.DisplayName, strings.Join(groups, ","))
	}
	return def.DisplayName
}

func HandleAdminLBAutoComplete(e *dc.AutocompleteEvent) {
	name, value := e.FocusedOption()
	if name == "" {
		respondAutocomplete(e, nil)
		return
	}

	partial := strings.ToLower(strings.TrimSpace(value))
	respondAutocomplete(e, buildAutocompleteChoices(partial, false))
}

func HandleLBPlayerAutoComplete(e *dc.AutocompleteEvent) {
	focusedName, value := e.FocusedOption()
	if focusedName == "" {
		respondAutocomplete(e, nil)
		return
	}

	partial := strings.ToLower(strings.TrimSpace(value))

	if focusedName == "alt" {
		alts := farmerstate.GetAltControllerByMiscString("AltController", e.UserID())
		var choices []dc.Choice[string]
		for _, alt := range alts {
			if partial == "" || strings.Contains(strings.ToLower(alt), partial) {
				choices = append(choices, dc.Choice[string]{Name: alt, Value: alt})
			}
		}
		respondAutocomplete(e, choices)
		return
	}

	respondAutocomplete(e, buildAutocompleteChoices(partial, true))
}

func respondAutocomplete(e *dc.AutocompleteEvent, choices []dc.Choice[string]) {
	if err := e.RespondChoices(choices); err != nil {
		log.Printf("leaderboard: failed to answer autocomplete: %v", err)
	}
}

func buildAutocompleteChoices(partial string, isPlayerCmd bool) []dc.Choice[string] {
	matches := func(name, key string) bool {
		return partial == "" ||
			strings.Contains(strings.ToLower(name), partial) ||
			strings.Contains(key, partial)
	}

	const maxChoices = 25
	choices := make([]dc.Choice[string], 0, maxChoices)

	if isPlayerCmd && matches("All Leaderboards", "all") {
		choices = append(choices, dc.Choice[string]{Name: "All Leaderboards", Value: "all"})
	}

	// Groups first.
	for _, g := range AllGroups {
		if len(choices) >= maxChoices {
			break
		}
		if matches(g.DisplayName, g.Key) {
			choices = append(choices, dc.Choice[string]{Name: g.DisplayName + " (Group)", Value: g.Key})
		}
	}

	if partial == "" {
		return choices
	}

	// Individual types.
	for _, def := range AllLeaderboards {
		if len(choices) >= maxChoices {
			break
		}
		choiceName := leaderboardChoiceName(def)
		if matches(choiceName, def.Key) {
			choices = append(choices, dc.Choice[string]{Name: choiceName, Value: def.Key})
		}
	}
	return choices
}

func handleRankings(e *dc.CommandEvent) {
	// Acknowledge immediately to avoid timeout.
	_ = e.Defer(true)

	showRankingsPage(e, 0)
}

// showRankingsPage answers either the deferred /lb rankings command or a click
// on one of its pagination buttons.
func showRankingsPage(e dc.InteractionEvent, page int) {
	userID := e.UserID()
	guildID := e.GuildID()
	if guildID == "" {
		if err := e.Followup(dc.Message{Content: "This command must be used within a server."}); err != nil {
			log.Printf("leaderboard: failed to report missing guild: %v", err)
		}
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
		if ev, ok := e.(*dc.ComponentEvent); ok {
			_ = ev.Update(dc.Message{Content: content})
		} else {
			_ = e.Followup(dc.Message{Content: content})
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
	if guildID != "" {
		cfgs, _ := guildstate.GetAllLeaderboardConfigsForGuild(guildID)
		for _, c := range cfgs {
			keys := ExpandConfigKey(c.LbType)
			var messageIDs []string
			if c.MessageIds.Valid && c.MessageIds.String != "" {
				_ = json.Unmarshal([]byte(c.MessageIds.String), &messageIDs)
			}
			if len(messageIDs) > 0 {
				link := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, c.ChannelID, messageIDs[0])
				for _, k := range keys {
					lbLinks[k] = link
				}
			}
		}
	}

	for i, st := range stats[start:end] {
		v := FormatLBValue(st.Def.ValueFmt, st.Current.Value)
		d := ""
		if st.HasPrev && st.Def.Key != LBContractExp && st.Def.Key != LBCXPWeeklyDelta {
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
	b.WriteString(strings.Repeat("—", maxRankWidth+maxNameWidth+maxValWidth+2))
	b.WriteByte('\n')

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
			formattedDetails := r.details
			if idx := strings.Index(formattedDetails, "dressed:"); idx != -1 {
				var dressed float64
				if _, err := fmt.Sscanf(formattedDetails[idx:], "dressed:%f", &dressed); err == nil {
					// Use FormatLBValue for the EB formatting
					originalStr := fmt.Sprintf("dressed:%.6f", dressed)
					formattedDetails = strings.Replace(formattedDetails, originalStr, "dressed:"+FormatLBValue("eb", dressed), 1)
				}
			}
			if idx := strings.Index(formattedDetails, "actual:"); idx != -1 {
				var actual float64
				if _, err := fmt.Sscanf(formattedDetails[idx:], "actual:%f", &actual); err == nil {
					originalStr := fmt.Sprintf("actual:%f", actual)
					formattedDetails = strings.Replace(formattedDetails, originalStr, fmt.Sprintf("actual:%.2f", actual), 1)
				}
			}
			detail = fmt.Sprintf(" (%s)", formattedDetails)
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

	// Content alongside a button row, so this stays on Discord's v1 component
	// model.
	msg := dc.Message{
		Content:      b.String(),
		ComponentsV1: true,
		Components: []dc.LayoutComponent{
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.Button{
						Label:    "Previous",
						Style:    dc.ButtonSecondary,
						CustomID: fmt.Sprintf("lb_stats#%d", page-1),
						Disabled: page <= 0,
					},
					dc.Button{
						Label:    "Next",
						Style:    dc.ButtonSecondary,
						CustomID: fmt.Sprintf("lb_stats#%d", page+1),
						Disabled: end >= len(stats),
					},
				},
			},
		},
	}

	var err error
	if ev, ok := e.(*dc.ComponentEvent); ok {
		err = ev.Update(msg)
	} else {
		err = e.EditResponse(msg)
	}
	if err != nil {
		log.Printf("leaderboard: failed to showRankingsPage for %s: %v", userID, err)
	}
}

func HandleLBStatsComponent(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	page, _ := strconv.Atoi(parts[1])
	showRankingsPage(e, page)
}
