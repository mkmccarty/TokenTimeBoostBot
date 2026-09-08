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

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

var snowflakeRe = regexp.MustCompile(`\b\d{17,20}\b`)

// maxAutocompleteChoices is the number of suggestions Discord accepts in one
// autocomplete response.
const maxAutocompleteChoices = 25

// knownSettingKeys is a curated list of setting keys always shown in the setting autocomplete.
var knownSettingKeys = []string{
	"admin_logs_channel",
	"amqp_url",
}

// adminGuildCommand is the shape every command in this package takes: guild
// only, guild-installed, and hidden from anyone without administrator
// permission.
func adminGuildCommand(cmd, description string) dc.Command {
	var adminPermission = int64(0)
	return dc.Command{
		Name:                     cmd,
		Description:              description,
		DefaultMemberPermissions: &adminPermission,
		Contexts:                 []dc.InteractionContext{dc.ContextGuild},
		IntegrationTypes:         []dc.IntegrationType{dc.IntegrationGuildInstall},
	}
}

// SlashSetGuildSettingCommand creates an admin slash command to set/clear a guild string setting.
func SlashSetGuildSettingCommand(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Set or clear a guild setting value")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "setting",
			Description:  "Setting key",
			Required:     true,
			Autocomplete: true,
		},
		dc.StringOption{
			Name:        "value",
			Description: "Setting value (leave blank to clear)",
		},
	}
	return &command
}

func HandleSetGuildSettingAutoComplete(e *dc.AutocompleteEvent) {
	name, value := e.FocusedOption()
	if name != "setting" {
		return
	}
	searchString := strings.ToLower(value)

	seen := make(map[string]struct{})
	var keys []string
	for _, k := range knownSettingKeys {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			keys = append(keys, k)
		}
	}
	if guild, err := GetGuildState(e.GuildID()); err == nil {
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

	choices := make([]dc.Choice[string], 0, maxAutocompleteChoices)
	for _, k := range keys {
		if searchString != "" && !strings.Contains(strings.ToLower(k), searchString) {
			continue
		}
		choices = append(choices, dc.Choice[string]{Name: k, Value: k})
		if len(choices) == maxAutocompleteChoices {
			break
		}
	}
	if err := e.RespondChoices(choices); err != nil {
		log.Println(err)
	}
}

// SlashGetGuildSettingsCommand creates an admin slash command to get all settings for a guild.
func SlashGetGuildSettingsCommand(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Get all settings for a guild")
	return &command
}

func respondEphemeral(e *dc.CommandEvent, message string) {
	err := e.Respond(dc.Message{Content: message, Ephemeral: true})
	if err != nil {
		log.Println(err)
	}
}

func respondDeferredEphemeral(e *dc.CommandEvent) bool {
	err := e.Defer(true)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

func followupEphemeral(e *dc.CommandEvent, message string) {
	err := e.Followup(dc.Message{Content: message})
	if err != nil {
		log.Println(err)
	}
}

func followupEphemeralOrFile(e *dc.CommandEvent, message, filename string) {
	const maxEphemeralContentLen = 1900

	if len(message) <= maxEphemeralContentLen {
		followupEphemeral(e, message)
		return
	}

	err := e.Followup(dc.Message{
		Content: "Guild settings output is too large for an inline message. Attached as a text file.",
		Files: []dc.File{{
			Name:   filename,
			Reader: bytes.NewReader([]byte(message)),
		}},
	})
	if err != nil {
		log.Println(err)
	}
}

func followupEphemeralOrFileWithBanner(e *dc.CommandEvent, message, filename, bannerPath string) {
	const maxEphemeralContentLen = 1900

	var msg dc.Message

	if len(message) <= maxEphemeralContentLen {
		msg.Content = message
	} else {
		msg.Content = "Guild settings output is too large for an inline message. Attached as a text file."
		msg.Files = append(msg.Files, dc.File{
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
			msg.Files = append(msg.Files, dc.File{
				Name:   bannerFilename,
				Reader: bytes.NewReader(bannerBytes),
			})
			msg.Embeds = []dc.Embed{{
				Title:     "Server Banner Preview",
				Thumbnail: "attachment://" + bannerFilename,
			}}
		}
	}

	err := e.Followup(msg)
	if err != nil {
		log.Println(err)
	}
}

func isAdminCaller(client dc.Client, e *dc.CommandEvent) bool {
	userID := e.UserID()
	perms, err := client.UserChannelPermissions(userID, e.ChannelID())
	if err != nil {
		log.Println(err)
	}
	return perms.Administrator() || userID == config.AdminUserID
}

// describeChannel names a channel as either a thread or a plain channel.
func describeChannel(ch *dc.Channel) string {
	kind := "channel"
	if ch.IsThread {
		kind = "thread"
	}
	name := strings.TrimSpace(ch.Name)
	if name == "" {
		return kind
	}
	return fmt.Sprintf("%s (%s)", kind, name)
}

func classifySnowflake(client dc.Client, guildID, id string) string {
	// Search guild channels first — more reliable than the state-cache lookup.
	if guildID != "" {
		if channels, err := client.GuildChannels(guildID); err == nil {
			for _, ch := range channels {
				if ch.ID == id {
					return describeChannel(&ch)
				}
			}
		}
	}

	// Fall back to direct channel lookup (handles DM channels, cross-guild, etc.).
	if ch, err := client.Channel(id); err == nil && ch != nil {
		return describeChannel(ch)
	}

	if guildID != "" {
		if member, err := client.GuildMember(guildID, id); err == nil && member != nil {
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

	if usr, err := client.User(id); err == nil && usr != nil {
		name := strings.TrimSpace(usr.GlobalName)
		if name == "" {
			name = strings.TrimSpace(usr.Username)
		}
		if name == "" {
			return "user"
		}
		return fmt.Sprintf("user (%s)", name)
	}

	if g, err := client.Guild(id); err == nil && g != nil {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			return "guild"
		}
		return fmt.Sprintf("guild (%s)", name)
	}

	return "unknown snowflake"
}

func getSnowflakeDetails(client dc.Client, guildID, value string) []string {
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
		details = append(details, fmt.Sprintf("%s -> %s", id, classifySnowflake(client, guildID, id)))
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

func getGuildDisplayName(client dc.Client, guildID string) string {
	guildName := guildID
	if guild, guildErr := client.Guild(guildID); guildErr == nil && guild != nil {
		if strings.TrimSpace(guild.Name) != "" {
			guildName = guild.Name
		}
	}
	return guildName
}

// SetGuildSettingForGuild sets or clears a guild setting for a specific guild ID.
func SetGuildSettingForGuild(client dc.Client, e *dc.CommandEvent, guildID, setting, value string) {
	if !isAdminCaller(client, e) {
		respondEphemeral(e, "You are not authorized to use this command.")
		return
	}

	if !respondDeferredEphemeral(e) {
		return
	}

	guildID = strings.TrimSpace(guildID)
	setting = strings.TrimSpace(setting)
	value = strings.TrimSpace(value)

	if guildID == "" {
		followupEphemeral(e, "Guild ID is required.")
		return
	}

	guildName := getGuildDisplayName(client, guildID)

	if setting == "" {
		followupEphemeral(e, "setting is required.")
		return
	}

	SetGuildSettingString(guildID, setting, value)
	if value == "" {
		followupEphemeral(e, fmt.Sprintf("Cleared setting '%s' for guild '%s'.", setting, guildName))
		return
	}

	var builder strings.Builder
	items := splitCSV(value)
	if len(items) > 1 {
		fmt.Fprintf(&builder, "Set setting '%s' for guild '%s' (%d items):", setting, guildName, len(items))
		for _, item := range items {
			details := getSnowflakeDetails(client, guildID, item)
			if len(details) == 1 {
				fmt.Fprintf(&builder, "\n- %s", details[0])
			} else {
				fmt.Fprintf(&builder, "\n- %s", item)
			}
		}
	} else {
		fmt.Fprintf(&builder, "Set setting '%s' for guild '%s' to '%s'.", setting, guildName, value)
		for _, detail := range getSnowflakeDetails(client, guildID, value) {
			fmt.Fprintf(&builder, "\n- resolved: %s", detail)
		}
	}

	followupEphemeralOrFile(e, builder.String(), fmt.Sprintf("guild-settings-%s.txt", guildID))
}

// GetGuildSettingsForGuild retrieves all persisted guild settings for a specific guild ID.
func GetGuildSettingsForGuild(client dc.Client, e *dc.CommandEvent, guildID string) {
	if !isAdminCaller(client, e) {
		respondEphemeral(e, "You are not authorized to use this command.")
		return
	}

	if !respondDeferredEphemeral(e) {
		return
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		followupEphemeral(e, "Guild ID is required.")
		return
	}

	guildName := getGuildDisplayName(client, guildID)

	guild, err := GetGuildState(guildID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			followupEphemeral(e, fmt.Sprintf("No persisted settings found for guild '%s'.", guildName))
			return
		}
		followupEphemeral(e, fmt.Sprintf("Error loading guild settings for '%s': %v", guildName, err))
		return
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Guild settings for '%s'\n", guildName)

	bannerPath := filepath.Join(config.BannerPath, fmt.Sprintf("banner_%s.png", guildID))
	_, bannerErr := os.Stat(bannerPath)
	hasServerBanner := bannerErr == nil

	if len(guild.MiscSettingsString) == 0 && len(guild.MiscSettingsFlag) == 0 && !hasServerBanner {
		builder.WriteString("No persisted settings found.")
		followupEphemeral(e, builder.String())
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
					details := getSnowflakeDetails(client, guildID, item)
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
			details := getSnowflakeDetails(client, guildID, value)
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

	followupEphemeralOrFileWithBanner(e, builder.String(), fmt.Sprintf("guild-settings-%s.txt", guildID), bannerPreviewPath)

	if !guild.LastUpdated.IsZero() {
		fmt.Fprintf(&builder, "\nLast updated: %s", guild.LastUpdated.Format("2006-01-02 15:04:05 MST"))
	}
}

// knownFlagKeys is a curated list of flag keys always shown in the flag autocomplete.
var knownFlagKeys = []string{
	"active-contracts-show-completed",
	"coopid_suggestions",
}

// SlashSetGuildFlagCommand creates an admin slash command to set a guild boolean flag.
func SlashSetGuildFlagCommand(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Set a guild boolean flag")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "flag",
			Description:  "Flag key",
			Required:     true,
			Autocomplete: true,
		},
		dc.BoolOption{
			Name:        "value",
			Description: "Flag value (true or false)",
			Required:    true,
		},
	}
	return &command
}

// SlashGetGuildFlagCommand creates an admin slash command to get a guild boolean flag.
func SlashGetGuildFlagCommand(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Get a guild boolean flag value")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "flag",
			Description:  "Flag key",
			Required:     true,
			Autocomplete: true,
		},
	}
	return &command
}

func HandleGuildFlagAutoComplete(e *dc.AutocompleteEvent) {
	name, value := e.FocusedOption()
	if name != "flag" {
		return
	}
	searchString := strings.ToLower(value)

	seen := make(map[string]struct{})
	var keys []string
	for _, k := range knownFlagKeys {
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	if guild, err := GetGuildState(e.GuildID()); err == nil && len(guild.MiscSettingsFlag) > 0 {
		for k := range guild.MiscSettingsFlag {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)

	choices := make([]dc.Choice[string], 0, maxAutocompleteChoices)
	for _, k := range keys {
		if searchString != "" && !strings.Contains(strings.ToLower(k), searchString) {
			continue
		}
		choices = append(choices, dc.Choice[string]{Name: k, Value: k})
		if len(choices) == maxAutocompleteChoices {
			break
		}
	}
	if err := e.RespondChoices(choices); err != nil {
		log.Println(err)
	}
}

// SetGuildFlag handles the admin slash command for setting a guild boolean flag.
func SetGuildFlag(client dc.Client, e *dc.CommandEvent) {
	if !isAdminCaller(client, e) {
		respondEphemeral(e, "You are not authorized to use this command.")
		return
	}

	flag := ""
	value := false

	if opt, ok := e.OptString("flag"); ok {
		flag = strings.TrimSpace(opt)
	}
	if opt, ok := e.OptBool("value"); ok {
		value = opt
	}

	if flag == "" {
		respondEphemeral(e, "flag is required.")
		return
	}

	guildName := getGuildDisplayName(client, e.GuildID())
	SetGuildSettingFlag(e.GuildID(), flag, value)
	respondEphemeral(e, fmt.Sprintf("Set flag '%s' for guild '%s' to %t.", flag, guildName, value))
}

// GetGuildFlag handles the admin slash command for getting a guild boolean flag.
func GetGuildFlag(client dc.Client, e *dc.CommandEvent) {
	if !isAdminCaller(client, e) {
		respondEphemeral(e, "You are not authorized to use this command.")
		return
	}

	flag := ""

	if opt, ok := e.OptString("flag"); ok {
		flag = strings.TrimSpace(opt)
	}

	if flag == "" {
		respondEphemeral(e, "flag is required.")
		return
	}

	guildName := getGuildDisplayName(client, e.GuildID())
	value := GetGuildSettingFlag(e.GuildID(), flag)
	respondEphemeral(e, fmt.Sprintf("Flag '%s' for guild '%s' is %t.", flag, guildName, value))
}

// SetGuildSetting handles the admin slash command for setting or clearing guild settings.
func SetGuildSetting(client dc.Client, e *dc.CommandEvent) {
	setting := ""
	value := ""

	if opt, ok := e.OptString("setting"); ok {
		setting = strings.TrimSpace(opt)
	}
	if opt, ok := e.OptString("value"); ok {
		value = strings.TrimSpace(opt)
	}
	SetGuildSettingForGuild(client, e, e.GuildID(), setting, value)
}

// GetGuildSettings handles the admin slash command for retrieving all guild settings.
func GetGuildSettings(client dc.Client, e *dc.CommandEvent) {
	GetGuildSettingsForGuild(client, e, e.GuildID())
}

func respondBannerFollowup(e *dc.CommandEvent, message string) {
	if err := e.Followup(dc.Message{Content: message}); err != nil {
		log.Println(err)
	}
}

func saveDefaultGuildBanner(e *dc.CommandEvent, guildID string, attachment *dc.Attachment) {
	if attachment == nil {
		respondBannerFollowup(e, "Failed to read the image attachment.")
		return
	}

	imgBytes, err := bottools.DownloadAttachmentBytesDC(attachment)
	if err != nil {
		respondBannerFollowup(e, "Failed to download the image attachment.")
		return
	}

	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		respondBannerFollowup(e, "Invalid image format. Please upload a valid PNG or JPG.")
		return
	}

	pngBytes, feedback, err := bottools.NormalizeBannerImage(img)
	if err != nil {
		respondBannerFollowup(e, "Failed to encode the image to PNG.")
		return
	}

	if err := os.MkdirAll(config.BannerPath, 0755); err != nil {
		log.Println("Error creating banner directory:", err)
	}

	outPath := filepath.Join(config.BannerPath, fmt.Sprintf("banner_%s.png", guildID))
	if err := os.WriteFile(outPath, pngBytes, 0644); err != nil {
		respondBannerFollowup(e, "Failed to save the image on the server.")
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
	respondBannerFollowup(e, message)
}

// SlashAdminSetServerBannerCommand creates an admin slash command to set a default guild banner.
func SlashAdminSetServerBannerCommand(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Set or remove the default guild banner (auto-fitted to 640x85)")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "guild-id",
			Description: "Guild ID to set the default banner for (defaults to current guild)",
		},
		dc.AttachmentOption{
			Name:        "image",
			Description: "Default guild banner image (PNG or JPG, auto-fitted to 640x85); omit to remove",
		},
	}
	return &command
}

// SlashSetServerBannerCommand creates an admin-only command to set a guild default banner.
func SlashSetServerBannerCommand(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Set this server's default banner (auto-fitted to 640x85)")
	command.Options = []dc.Option{
		dc.AttachmentOption{
			Name:        "image",
			Description: "Server banner image (PNG or JPG, auto-fitted to 640x85)",
			Required:    true,
		},
	}
	return &command
}

func HandleSetServerBanner(client dc.Client, e *dc.CommandEvent) {
	if err := e.Defer(true); err != nil {
		log.Println(err)
	}

	if !isAdminCaller(client, e) {
		respondBannerFollowup(e, "You are not authorized to use this command.")
		return
	}

	guildID := strings.TrimSpace(e.GuildID())
	if guildID == "" {
		respondBannerFollowup(e, "This command must be used in a guild.")
		return
	}

	attachment, hasImage := e.OptAttachment("image")
	if !hasImage {
		respondBannerFollowup(e, "image is required.")
		return
	}

	saveDefaultGuildBanner(e, guildID, attachment)
}

func HandleAdminSlashAdminSetServerBannerCommand(client dc.Client, e *dc.CommandEvent) {
	if err := e.Defer(true); err != nil {
		log.Println(err)
	}

	if !isAdminCaller(client, e) {
		respondBannerFollowup(e, "You are not authorized to use this command.")
		return
	}

	guildID := e.GuildID()
	if opt, ok := e.OptString("guild-id"); ok {
		if v := strings.TrimSpace(opt); v != "" {
			guildID = v
		}
	}
	if guildID == "" {
		respondBannerFollowup(e, "This command must be used in a guild or a guild-id must be provided.")
		return
	}

	const defaultGuildID = "DEFAULT"

	attachment, hasImage := e.OptAttachment("image")
	if !hasImage {
		outPath := filepath.Join(config.BannerPath, fmt.Sprintf("banner_%s.png", guildID))
		if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
			respondBannerFollowup(e, "Failed to remove the default guild banner.")
			return
		}
		_ = farmerstate.RemoveCustomBanner(guildID, defaultGuildID)
		respondBannerFollowup(e, "Default guild banner removed.")
		return
	}

	saveDefaultGuildBanner(e, guildID, attachment)
}
