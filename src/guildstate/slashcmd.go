package guildstate

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
)

var snowflakeRe = regexp.MustCompile(`\b\d{17,20}\b`)

// SlashSetGuildSettingCommand creates an admin slash command to set/clear a guild string setting.
func SlashSetGuildSettingCommand(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Set or clear a guild setting value",
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "setting",
				Description: "Setting key",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "value",
				Description: "Setting value (leave blank to clear)",
				Required:    false,
			},
		},
	}
}

// SlashGetGuildSettingsCommand creates an admin slash command to get all settings for a guild.
func SlashGetGuildSettingsCommand(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Get all settings for a guild",
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{},
	}
}

func getInteractionUserID(i *discordgo.InteractionCreate) string {
	if i.GuildID != "" && i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    message,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		log.Println(err)
	}
}

func isAdminCaller(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	userID := getInteractionUserID(i)
	perms, err := s.UserChannelPermissions(userID, i.ChannelID)
	if err != nil {
		log.Println(err)
	}
	return perms&discordgo.PermissionAdministrator != 0 || userID == config.AdminUserID
}

func classifySnowflake(s *discordgo.Session, guildID, id string) string {
	// Search guild channels first — more reliable than the state-cache lookup.
	if guildID != "" {
		if channels, err := s.GuildChannels(guildID); err == nil {
			for _, ch := range channels {
				if ch.ID == id {
					switch ch.Type {
					case discordgo.ChannelTypeGuildPublicThread, discordgo.ChannelTypeGuildPrivateThread, discordgo.ChannelTypeGuildNewsThread:
						name := strings.TrimSpace(ch.Name)
						if name == "" {
							return "thread"
						}
						return fmt.Sprintf("thread (%s)", name)
					default:
						name := strings.TrimSpace(ch.Name)
						if name == "" {
							return "channel"
						}
						return fmt.Sprintf("channel (%s)", name)
					}
				}
			}
		}
	}

	// Fall back to direct channel lookup (handles DM channels, cross-guild, etc.).
	if ch, err := s.Channel(id); err == nil && ch != nil {
		switch ch.Type {
		case discordgo.ChannelTypeGuildPublicThread, discordgo.ChannelTypeGuildPrivateThread, discordgo.ChannelTypeGuildNewsThread:
			name := strings.TrimSpace(ch.Name)
			if name == "" {
				return "thread"
			}
			return fmt.Sprintf("thread (%s)", name)
		default:
			name := strings.TrimSpace(ch.Name)
			if name == "" {
				return "channel"
			}
			return fmt.Sprintf("channel (%s)", name)
		}
	}

	if guildID != "" {
		if member, err := s.GuildMember(guildID, id); err == nil && member != nil {
			name := strings.TrimSpace(member.Nick)
			if name == "" && member.User != nil {
				name = strings.TrimSpace(member.User.GlobalName)
			}
			if name == "" && member.User != nil {
				name = strings.TrimSpace(member.User.Username)
			}
			if name == "" {
				return "user"
			}
			return fmt.Sprintf("user (%s)", name)
		}
	}

	if usr, err := s.User(id); err == nil && usr != nil {
		name := strings.TrimSpace(usr.GlobalName)
		if name == "" {
			name = strings.TrimSpace(usr.Username)
		}
		if name == "" {
			return "user"
		}
		return fmt.Sprintf("user (%s)", name)
	}

	return "unknown snowflake"
}

func getSnowflakeDetails(s *discordgo.Session, guildID, value string) []string {
	matches := snowflakeRe.FindAllString(value, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	details := make([]string, 0, len(matches))
	for _, id := range matches {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		details = append(details, fmt.Sprintf("%s -> %s", id, classifySnowflake(s, guildID, id)))
	}
	return details
}

// splitCSV splits a comma-separated value into trimmed, non-empty items.
func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			items = append(items, t)
		}
	}
	return items
}

// SetGuildSetting handles the admin slash command for setting or clearing guild settings.
func SetGuildSetting(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !isAdminCaller(s, i) {
		respondEphemeral(s, i, "You are not authorized to use this command.")
		return
	}

	optionMap := bottools.GetCommandOptionsMap(i)
	guildID := i.GuildID
	setting := ""
	value := ""

	if opt, ok := optionMap["setting"]; ok {
		setting = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["value"]; ok {
		value = strings.TrimSpace(opt.StringValue())
	}

	if guildID == "" {
		respondEphemeral(s, i, "This command can only be used in a server.")
		return
	}

	guildName := guildID
	if dgGuild, guildErr := s.Guild(guildID); guildErr == nil && dgGuild != nil {
		if strings.TrimSpace(dgGuild.Name) != "" {
			guildName = dgGuild.Name
		}
	}

	if setting == "" {
		respondEphemeral(s, i, "setting is required.")
		return
	}

	SetGuildSettingString(guildID, setting, value)
	if value == "" {
		respondEphemeral(s, i, fmt.Sprintf("Cleared setting '%s' for guild '%s'.", setting, guildName))
		return
	}

	var builder strings.Builder
	items := splitCSV(value)
	if len(items) > 1 {
		fmt.Fprintf(&builder, "Set setting '%s' for guild '%s' (%d items):", setting, guildName, len(items))
		for _, item := range items {
			details := getSnowflakeDetails(s, guildID, item)
			if len(details) == 1 {
				fmt.Fprintf(&builder, "\n- %s", details[0])
			} else {
				fmt.Fprintf(&builder, "\n- %s", item)
			}
		}
	} else {
		fmt.Fprintf(&builder, "Set setting '%s' for guild '%s' to '%s'.", setting, guildName, value)
		for _, detail := range getSnowflakeDetails(s, guildID, value) {
			fmt.Fprintf(&builder, "\n- resolved: %s", detail)
		}
	}

	respondEphemeral(s, i, builder.String())
}

// GetGuildSettings handles the admin slash command for retrieving all guild settings.
func GetGuildSettings(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !isAdminCaller(s, i) {
		respondEphemeral(s, i, "You are not authorized to use this command.")
		return
	}

	guildID := i.GuildID
	guildName := guildID
	if dgGuild, guildErr := s.Guild(guildID); guildErr == nil && dgGuild != nil {
		if strings.TrimSpace(dgGuild.Name) != "" {
			guildName = dgGuild.Name
		}
	}

	if guildID == "" {
		respondEphemeral(s, i, "This command can only be used in a server.")
		return
	}

	guild, err := GetGuildState(guildID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondEphemeral(s, i, fmt.Sprintf("No persisted settings found for guild '%s'.", guildName))
			return
		}
		respondEphemeral(s, i, fmt.Sprintf("Error loading guild settings for '%s': %v", guildName, err))
		return
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Guild settings for '%s'\n", guildName)

	if len(guild.MiscSettingsString) == 0 && len(guild.MiscSettingsFlag) == 0 {
		builder.WriteString("No persisted settings found.")
		respondEphemeral(s, i, builder.String())
		return
	}

	if len(guild.MiscSettingsString) > 0 {
		builder.WriteString("\nString settings:\n")
		keys := make([]string, 0, len(guild.MiscSettingsString))
		for key := range guild.MiscSettingsString {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := guild.MiscSettingsString[key]
			items := splitCSV(value)
			if len(items) > 1 {
				fmt.Fprintf(&builder, "- %s (%d items)\n", key, len(items))
				for _, item := range items {
					details := getSnowflakeDetails(s, guildID, item)
					if len(details) == 1 {
						fmt.Fprintf(&builder, "  - %s\n", details[0])
					} else {
						fmt.Fprintf(&builder, "  - %s\n", item)
					}
				}
			} else {
				fmt.Fprintf(&builder, "- %s = %s\n", key, value)
				for _, detail := range getSnowflakeDetails(s, guildID, value) {
					fmt.Fprintf(&builder, "  - resolved: %s\n", detail)
				}
			}
		}
	}

	if len(guild.MiscSettingsFlag) > 0 {
		builder.WriteString("\nFlag settings:\n")
		keys := make([]string, 0, len(guild.MiscSettingsFlag))
		for key := range guild.MiscSettingsFlag {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&builder, "- %s = %t\n", key, guild.MiscSettingsFlag[key])
		}
	}

	if !guild.LastUpdated.IsZero() {
		fmt.Fprintf(&builder, "\nLast updated: %s", guild.LastUpdated.Format("2006-01-02 15:04:05 MST"))
	}

	respondEphemeral(s, i, builder.String())
}
