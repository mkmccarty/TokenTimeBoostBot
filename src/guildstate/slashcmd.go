package guildstate

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	_ "image/jpeg" // Register JPEG decoder for image.Decode.

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

var snowflakeRe = regexp.MustCompile(`\b\d{17,20}\b`)

// knownSettingKeys is a curated list of setting keys always shown in the setting autocomplete.
var knownSettingKeys = []string{
	"admin_logs_channel",
	"banner_override",
}

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
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "setting",
				Description:  "Setting key",
				Required:     true,
				Autocomplete: true,
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

// HandleSetGuildSettingAutoComplete handles autocomplete for the admin-set-guild-setting command.
func HandleSetGuildSettingAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	for _, opt := range data.Options {
		if opt.Name != "setting" || !opt.Focused {
			continue
		}
		searchString := strings.ToLower(opt.StringValue())

		seen := make(map[string]struct{})
		var keys []string
		for _, k := range knownSettingKeys {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				keys = append(keys, k)
			}
		}
		if guild, err := GetGuildState(i.GuildID); err == nil {
			existingKeys := make([]string, 0, len(guild.MiscSettingsString))
			for k := range guild.MiscSettingsString {
				existingKeys = append(existingKeys, k)
			}
			sort.Strings(existingKeys)
			for _, k := range existingKeys {
				if _, ok := seen[k]; !ok {
					seen[k] = struct{}{}
					keys = append(keys, k)
				}
			}
		}
		sort.Strings(keys)

		choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, 25)
		for _, k := range keys {
			if searchString != "" && !strings.Contains(strings.ToLower(k), searchString) {
				continue
			}
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  k,
				Value: k,
			})
			if len(choices) == 25 {
				break
			}
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Content: "Setting Key",
				Choices: choices,
			}})
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

func respondDeferredEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

func followupEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: message,
	})
	if err != nil {
		log.Println(err)
	}
}

func followupEphemeralOrFile(s *discordgo.Session, i *discordgo.InteractionCreate, message, filename string) {
	const maxEphemeralContentLen = 1900

	if len(message) <= maxEphemeralContentLen {
		followupEphemeral(s, i, message)
		return
	}

	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "Guild settings output is too large for an inline message. Attached as a text file.",
		Files: []*discordgo.File{{
			Name:   filename,
			Reader: bytes.NewReader([]byte(message)),
		}},
	})
	if err != nil {
		log.Println(err)
	}
}

func followupEphemeralOrFileWithBanner(s *discordgo.Session, i *discordgo.InteractionCreate, message, filename, bannerPath string) {
	const maxEphemeralContentLen = 1900

	params := &discordgo.WebhookParams{}

	if len(message) <= maxEphemeralContentLen {
		params.Content = message
	} else {
		params.Content = "Guild settings output is too large for an inline message. Attached as a text file."
		params.Files = append(params.Files, &discordgo.File{
			Name:   filename,
			Reader: bytes.NewReader([]byte(message)),
		})
	}

	if strings.TrimSpace(bannerPath) != "" {
		bannerBytes, err := os.ReadFile(bannerPath)
		if err != nil {
			log.Println(err)
		} else {
			bannerFilename := filepath.Base(bannerPath)
			if bannerFilename == "" {
				bannerFilename = "server-banner.png"
			}
			params.Files = append(params.Files, &discordgo.File{
				Name:   bannerFilename,
				Reader: bytes.NewReader(bannerBytes),
			})
			params.Embeds = []*discordgo.MessageEmbed{{
				Title: "Server Banner Preview",
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: "attachment://" + bannerFilename,
				},
			}}
		}
	}

	_, err := s.FollowupMessageCreate(i.Interaction, true, params)
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

	if g, err := s.Guild(id); err == nil && g != nil {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			return "guild"
		}
		return fmt.Sprintf("guild (%s)", name)
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

func formatResolvedDetail(detail string) string {
	parts := strings.SplitN(detail, " -> ", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return detail
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

func getGuildDisplayName(s *discordgo.Session, guildID string) string {
	guildName := guildID
	if dgGuild, guildErr := s.Guild(guildID); guildErr == nil && dgGuild != nil {
		if strings.TrimSpace(dgGuild.Name) != "" {
			guildName = dgGuild.Name
		}
	}
	return guildName
}

// SetGuildSettingForGuild sets or clears a guild setting for a specific guild ID.
func SetGuildSettingForGuild(s *discordgo.Session, i *discordgo.InteractionCreate, guildID, setting, value string) {
	if !isAdminCaller(s, i) {
		respondEphemeral(s, i, "You are not authorized to use this command.")
		return
	}

	if !respondDeferredEphemeral(s, i) {
		return
	}

	guildID = strings.TrimSpace(guildID)
	setting = strings.TrimSpace(setting)
	value = strings.TrimSpace(value)

	if guildID == "" {
		followupEphemeral(s, i, "Guild ID is required.")
		return
	}

	guildName := getGuildDisplayName(s, guildID)

	if setting == "" {
		followupEphemeral(s, i, "setting is required.")
		return
	}

	SetGuildSettingString(guildID, setting, value)
	if value == "" {
		followupEphemeral(s, i, fmt.Sprintf("Cleared setting '%s' for guild '%s'.", setting, guildName))
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

	followupEphemeralOrFile(s, i, builder.String(), fmt.Sprintf("guild-settings-%s.txt", guildID))
}

// GetGuildSettingsForGuild retrieves all persisted guild settings for a specific guild ID.
func GetGuildSettingsForGuild(s *discordgo.Session, i *discordgo.InteractionCreate, guildID string) {
	if !isAdminCaller(s, i) {
		respondEphemeral(s, i, "You are not authorized to use this command.")
		return
	}

	if !respondDeferredEphemeral(s, i) {
		return
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		followupEphemeral(s, i, "Guild ID is required.")
		return
	}

	guildName := getGuildDisplayName(s, guildID)

	guild, err := GetGuildState(guildID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			followupEphemeral(s, i, fmt.Sprintf("No persisted settings found for guild '%s'.", guildName))
			return
		}
		followupEphemeral(s, i, fmt.Sprintf("Error loading guild settings for '%s': %v", guildName, err))
		return
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Guild settings for '%s'\n", guildName)

	bannerPath := filepath.Join(config.BannerPath, fmt.Sprintf("banner_%s.png", guildID))
	_, bannerErr := os.Stat(bannerPath)
	hasServerBanner := bannerErr == nil

	if len(guild.MiscSettingsString) == 0 && len(guild.MiscSettingsFlag) == 0 && !hasServerBanner {
		builder.WriteString("No persisted settings found.")
		followupEphemeral(s, i, builder.String())
		return
	}

	if len(guild.MiscSettingsString) == 0 && len(guild.MiscSettingsFlag) == 0 {
		builder.WriteString("No persisted settings found.\n")
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
			fmt.Fprintf(&builder, "- %s = %s\n", key, value)
			if len(items) > 1 {
				fmt.Fprintf(&builder, "  - parsed items (%d):\n", len(items))
				for _, item := range items {
					details := getSnowflakeDetails(s, guildID, item)
					if len(details) == 0 {
						fmt.Fprintf(&builder, "    - %s\n", item)
						continue
					}
					for _, detail := range details {
						fmt.Fprintf(&builder, "    - resolved: %s\n", formatResolvedDetail(detail))
					}
				}
				continue
			}
			details := getSnowflakeDetails(s, guildID, value)
			if len(details) == 0 {
				continue
			}
			for _, detail := range details {
				fmt.Fprintf(&builder, "  - resolved: %s\n", formatResolvedDetail(detail))
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

	if hasServerBanner {
		builder.WriteString("\nServer banner: custom default is configured.")
	}

	bannerPreviewPath := ""
	if hasServerBanner {
		bannerPreviewPath = bannerPath
	}

	followupEphemeralOrFileWithBanner(s, i, builder.String(), fmt.Sprintf("guild-settings-%s.txt", guildID), bannerPreviewPath)

	if !guild.LastUpdated.IsZero() {
		fmt.Fprintf(&builder, "\nLast updated: %s", guild.LastUpdated.Format("2006-01-02 15:04:05 MST"))
	}
}

// knownFlagKeys is a curated list of flag keys always shown in the flag autocomplete.
var knownFlagKeys = []string{
	"active-contracts-show-completed",
}

// SlashSetGuildFlagCommand creates an admin slash command to set a guild boolean flag.
func SlashSetGuildFlagCommand(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Set a guild boolean flag",
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
				Name:         "flag",
				Description:  "Flag key",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "value",
				Description: "Flag value (true or false)",
				Required:    true,
			},
		},
	}
}

// SlashGetGuildFlagCommand creates an admin slash command to get a guild boolean flag.
func SlashGetGuildFlagCommand(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Get a guild boolean flag value",
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
				Name:         "flag",
				Description:  "Flag key",
				Required:     true,
				Autocomplete: true,
			},
		},
	}
}

// HandleGuildFlagAutoComplete handles autocomplete for the flag key option on set/get flag commands.
func HandleGuildFlagAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	for _, opt := range data.Options {
		if opt.Name != "flag" || !opt.Focused {
			continue
		}
		searchString := strings.ToLower(opt.StringValue())

		seen := make(map[string]struct{})
		var keys []string
		for _, k := range knownFlagKeys {
			seen[k] = struct{}{}
			keys = append(keys, k)
		}
		if guild, err := GetGuildState(i.GuildID); err == nil && len(guild.MiscSettingsFlag) > 0 {
			for k := range guild.MiscSettingsFlag {
				if _, ok := seen[k]; !ok {
					seen[k] = struct{}{}
					keys = append(keys, k)
				}
			}
		}
		sort.Strings(keys)

		choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, 25)
		for _, k := range keys {
			if searchString != "" && !strings.Contains(strings.ToLower(k), searchString) {
				continue
			}
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  k,
				Value: k,
			})
			if len(choices) == 25 {
				break
			}
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Content: "Flag Key",
				Choices: choices,
			}})
	}
}

// SetGuildFlag handles the admin slash command for setting a guild boolean flag.
func SetGuildFlag(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !isAdminCaller(s, i) {
		respondEphemeral(s, i, "You are not authorized to use this command.")
		return
	}

	optionMap := bottools.GetCommandOptionsMap(i)
	flag := ""
	value := false

	if opt, ok := optionMap["flag"]; ok {
		flag = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["value"]; ok {
		value = opt.BoolValue()
	}

	if flag == "" {
		respondEphemeral(s, i, "flag is required.")
		return
	}

	guildName := getGuildDisplayName(s, i.GuildID)
	SetGuildSettingFlag(i.GuildID, flag, value)
	respondEphemeral(s, i, fmt.Sprintf("Set flag '%s' for guild '%s' to %t.", flag, guildName, value))
}

// GetGuildFlag handles the admin slash command for getting a guild boolean flag.
func GetGuildFlag(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !isAdminCaller(s, i) {
		respondEphemeral(s, i, "You are not authorized to use this command.")
		return
	}

	optionMap := bottools.GetCommandOptionsMap(i)
	flag := ""

	if opt, ok := optionMap["flag"]; ok {
		flag = strings.TrimSpace(opt.StringValue())
	}

	if flag == "" {
		respondEphemeral(s, i, "flag is required.")
		return
	}

	guildName := getGuildDisplayName(s, i.GuildID)
	value := GetGuildSettingFlag(i.GuildID, flag)
	respondEphemeral(s, i, fmt.Sprintf("Flag '%s' for guild '%s' is %t.", flag, guildName, value))
}

// SetGuildSetting handles the admin slash command for setting or clearing guild settings.
func SetGuildSetting(s *discordgo.Session, i *discordgo.InteractionCreate) {
	optionMap := bottools.GetCommandOptionsMap(i)
	setting := ""
	value := ""

	if opt, ok := optionMap["setting"]; ok {
		setting = strings.TrimSpace(opt.StringValue())
	}
	if opt, ok := optionMap["value"]; ok {
		value = strings.TrimSpace(opt.StringValue())
	}
	SetGuildSettingForGuild(s, i, i.GuildID, setting, value)
}

// GetGuildSettings handles the admin slash command for retrieving all guild settings.
func GetGuildSettings(s *discordgo.Session, i *discordgo.InteractionCreate) {
	GetGuildSettingsForGuild(s, i, i.GuildID)
}

func respondBannerFollowup(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: message,
	})
}

func saveDefaultGuildBanner(s *discordgo.Session, i *discordgo.InteractionCreate, guildID string, imageOpt *discordgo.ApplicationCommandInteractionDataOption) {
	attachment := bottools.GetCommandAttachment(i, imageOpt)
	if attachment == nil {
		respondBannerFollowup(s, i, "Failed to read the image attachment.")
		return
	}

	imgBytes, err := bottools.DownloadAttachmentBytes(attachment)
	if err != nil {
		respondBannerFollowup(s, i, "Failed to download the image attachment.")
		return
	}

	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		respondBannerFollowup(s, i, "Invalid image format. Please upload a valid PNG or JPG.")
		return
	}

	pngBytes, feedback, err := bottools.NormalizeBannerImage(img)
	if err != nil {
		respondBannerFollowup(s, i, "Failed to encode the image to PNG.")
		return
	}

	if err := os.MkdirAll(config.BannerPath, 0755); err != nil {
		log.Println("Error creating banner directory:", err)
	}

	outPath := filepath.Join(config.BannerPath, fmt.Sprintf("banner_%s.png", guildID))
	if err := os.WriteFile(outPath, pngBytes, 0644); err != nil {
		respondBannerFollowup(s, i, "Failed to save the image on the server.")
		return
	}

	_ = farmerstate.SetCustomBanner(guildID, "DEFAULT", pngBytes)
	if bottools.RefreshGuildContractsForBannerCallback != nil {
		bottools.RefreshGuildContractsForBannerCallback(guildID)
	}
	message := "Default guild banner successfully uploaded and saved."
	if feedback != "" {
		message += " " + feedback
	}
	respondBannerFollowup(s, i, message)
}

// SlashAdminSetServerBannerCommand creates an admin slash command to set a default guild banner.
func SlashAdminSetServerBannerCommand(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Set or remove the default guild banner (auto-fitted to 640x85)",
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
				Name:        "guild-id",
				Description: "Guild ID to set the default banner for (defaults to current guild)",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionAttachment,
				Name:        "image",
				Description: "Default guild banner image (PNG or JPG, auto-fitted to 640x85); omit to remove",
				Required:    false,
			},
		},
	}
}

// SlashSetServerBannerCommand creates an admin-only command to set a guild default banner.
func SlashSetServerBannerCommand(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Set this server's default banner (auto-fitted to 640x85)",
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionAttachment,
				Name:        "image",
				Description: "Server banner image (PNG or JPG, auto-fitted to 640x85)",
				Required:    true,
			},
		},
	}
}

// HandleSetServerBanner handles the /set-server-banner command.
func HandleSetServerBanner(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	if !isAdminCaller(s, i) {
		respondBannerFollowup(s, i, "You are not authorized to use this command.")
		return
	}

	guildID := strings.TrimSpace(i.GuildID)
	if guildID == "" {
		respondBannerFollowup(s, i, "This command must be used in a guild.")
		return
	}

	optionMap := bottools.GetCommandOptionsMap(i)
	imageOpt, hasImage := optionMap["image"]
	if !hasImage {
		respondBannerFollowup(s, i, "image is required.")
		return
	}

	saveDefaultGuildBanner(s, i, guildID, imageOpt)
}

// HandleAdminSlashAdminSetServerBannerCommand handles the /admin-set-server-banner command.
// The banner is stored with guildID in the user_id column and "DEFAULT" in the guild_id column.
func HandleAdminSlashAdminSetServerBannerCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	if !isAdminCaller(s, i) {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "You are not authorized to use this command.",
		})
		return
	}

	optionMap := bottools.GetCommandOptionsMap(i)

	guildID := i.GuildID
	if opt, ok := optionMap["guild-id"]; ok {
		if v := strings.TrimSpace(opt.StringValue()); v != "" {
			guildID = v
		}
	}
	if guildID == "" {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "This command must be used in a guild or a guild-id must be provided.",
		})
		return
	}

	const defaultGuildID = "DEFAULT"

	imageOpt, hasImage := optionMap["image"]
	if !hasImage {
		outPath := filepath.Join(config.BannerPath, fmt.Sprintf("banner_%s.png", guildID))
		if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
			respondBannerFollowup(s, i, "Failed to remove the default guild banner.")
			return
		}
		_ = farmerstate.RemoveCustomBanner(guildID, defaultGuildID)
		respondBannerFollowup(s, i, "Default guild banner removed.")
		return
	}

	saveDefaultGuildBanner(s, i, guildID, imageOpt)
}
