package leaderboard

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

// GetSlashBockLeaderboardCommand returns the /bock-leaderboard command definition.
// The command has three sub-command groups: admin, player, and run (home-guild only).
func GetSlashBockLeaderboardCommand(cmd string) *discordgo.ApplicationCommand {

	// Admin-only home-guild run command

	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Manage and view bock stat leaderboards.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			// ── admin subcommand group ─────────────────────────────────────────
			{
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Name:        "admin",
				Description: "Guild admin commands for leaderboard configuration (requires Manage Server).",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "set-channel",
						Description: "Configure a leaderboard type to post in this channel.",
						Options: []*discordgo.ApplicationCommandOption{
							{
								Type:         discordgo.ApplicationCommandOptionString,
								Name:         "type",
								Description:  "Leaderboard type",
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
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "remove",
						Description: "Remove a leaderboard configuration for this guild.",
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
				},
			},
			// ── player subcommand group ────────────────────────────────────────
			{
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Name:        "player",
				Description: "Player opt-in / opt-out commands.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "optin",
						Description: "Opt into leaderboards.",
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
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "optout",
						Description: "Opt out of leaderboards.",
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
						Name:        "status",
						Description: "Show your current leaderboard opt-in status.",
					},
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "list",
						Description: "List all available leaderboard types and their keys.",
					},
				},
			},
			// ── run subcommand (home-guild admin only) ─────────────────────────
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

// HandleBockLeaderboard dispatches the /bock-leaderboard slash command.
func HandleBockLeaderboard(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := i.ApplicationCommandData().Options
	if len(opts) == 0 {
		respondEphemeral(s, i, "Unknown subcommand.")
		return
	}

	switch opts[0].Name {
	case "admin":
		handleAdminGroup(s, i, opts[0].Options)
	case "player":
		handlePlayerGroup(s, i, opts[0].Options)
	case "run":
		handleRun(s, i, opts[0].Options)
	default:
		respondEphemeral(s, i, "Unknown subcommand group.")
	}
}

// ─── admin group ──────────────────────────────────────────────────────────────

func handleAdminGroup(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	if len(opts) == 0 {
		respondEphemeral(s, i, "Unknown admin subcommand.")
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
	default:
		respondEphemeral(s, i, "Unknown admin subcommand.")
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
	cfgs, err := GetAllGuildLBConfigs(i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to load configuration.")
		return
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

// ─── player group ─────────────────────────────────────────────────────────────

func handlePlayerGroup(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	if len(opts) == 0 {
		respondEphemeral(s, i, "Unknown player subcommand.")
		return
	}
	switch opts[0].Name {
	case "optin":
		handlePlayerOptIn(s, i, opts[0].Options)
	case "optout":
		handlePlayerOptOut(s, i, opts[0].Options)
	case "status":
		handlePlayerStatus(s, i)
	case "list":
		handlePlayerList(s, i)
	default:
		respondEphemeral(s, i, "Unknown player subcommand.")
	}
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

	AddPlayerOptInTypes(userID, types)

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

	RemovePlayerOptInTypes(userID, types)

	if len(types) == 1 && types[0] == OptInAll {
		respondEphemeral(s, i, "✅ You have opted out of **all** leaderboards.")
		return
	}
	names := typeKeysToNames(types)
	respondEphemeral(s, i, fmt.Sprintf("✅ Opted out of: %s", strings.Join(names, ", ")))
}

func handlePlayerStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	storedVal := optInRaw(userID)
	if storedVal == "" {
		respondEphemeral(s, i, "You are not opted into any leaderboards.\nUse `/bock-leaderboard player optin types:all` to join everything.")
		return
	}
	if storedVal == OptInAll {
		respondEphemeral(s, i, "You are opted into **all** leaderboards.")
		return
	}
	types := GetPlayerOptInTypes(userID)
	names := typeKeysToNames(types)
	respondEphemeral(s, i, fmt.Sprintf("**Your leaderboard opt-ins (%d):**\n%s",
		len(names), strings.Join(names, "\n")))
}

func handlePlayerList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var b strings.Builder
	b.WriteString("**Available leaderboard types:**\n")
	b.WriteString("```\n")
	for _, def := range AllLeaderboards {
		fmt.Fprintf(&b, "%-42s  %s\n", def.Key, def.DisplayName)
	}
	b.WriteString("```")
	respondEphemeral(s, i, b.String())
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

	// Acknowledge immediately, run in background.
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	go func() {
		RunLeaderboardCollection(s, dryRun)
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

// parseTypeList parses a comma-separated type string like "te_total,virtue_shifts"
// or the literal "all". Returns (nil, errorMsg) on invalid input.
func parseTypeList(raw string) ([]string, string) {
	raw = strings.TrimSpace(raw)
	if strings.ToLower(raw) == "all" {
		return []string{OptInAll}, ""
	}
	parts := strings.Split(raw, ",")
	var out []string
	var bad []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := LBDefByKey(p); !ok {
			bad = append(bad, p)
			continue
		}
		out = append(out, p)
	}
	if len(bad) > 0 {
		return nil, fmt.Sprintf("Unknown leaderboard key(s): %s\nUse `/bock-leaderboard player list` to see valid keys.",
			strings.Join(bad, ", "))
	}
	if len(out) == 0 {
		return nil, "No valid leaderboard types provided."
	}
	return out, ""
}

func typeKeysToNames(keys []string) []string {
	names := make([]string, 0, len(keys))
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
func optInRaw(userID string) string {
	types := GetPlayerOptInTypes(userID)
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

// HandleBockLeaderboardAutoComplete handles autocomplete for the "type" option on
// admin set-channel and admin remove subcommands.
//
// List order: groups first, then individual types that are NOT covered by any
// group (e.g. virtue_shifts, te_total, cxp_weekly_delta). Types inside a group
// are omitted because they should be configured via the group key. Typing a
// partial string filters across both groups and individuals.
func HandleBockLeaderboardAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	// Walk the option tree to find the focused string option.
	var partial string
	var found bool
	var groupName, subName string
	for _, opt := range data.Options {
		groupName = opt.Name
		for _, sub := range opt.Options {
			subName = sub.Name
			for _, leaf := range sub.Options {
				if leaf.Focused {
					partial = strings.ToLower(strings.TrimSpace(leaf.StringValue()))
					found = true
				}
			}
		}
	}
	if !found {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	matches := func(name, key string) bool {
		return partial == "" ||
			strings.Contains(strings.ToLower(name), partial) ||
			strings.Contains(key, partial)
	}

	const maxChoices = 25
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, maxChoices)

	// Individual types.
	isPlayerCmd := (groupName == "player" && (subName == "optin" || subName == "optout"))

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

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}
