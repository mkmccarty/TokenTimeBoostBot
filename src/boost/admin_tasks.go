package boost

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

type adminTaskDef struct {
	ID          string
	Name        string
	Description string
}

var adminTaskList = []adminTaskDef{
	{
		ID:          "cycle-encryption-key",
		Name:        "Cycle Encryption Key",
		Description: "Rotate AES-256 key, re-encrypt all farmer EIDs, and update config files",
	},
	{
		ID:          "reload-emojis",
		Name:        "Reload Emojis",
		Description: "Clear local emoji cache, re-fetch all application emojis from Discord",
	},
}

// SlashAdminTasksCommand returns the /admin-tasks command definition.
func SlashAdminTasksCommand(cmd string) *dc.Command {
	command := adminGuildCommand(cmd, "Execute administrative maintenance tasks")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "task",
			Description:  "Administrative task to execute",
			Required:     true,
			Autocomplete: true,
		},
	}
	return &command
}

// HandleAdminTasksAutocomplete provides autocomplete suggestions for administrative tasks.
func HandleAdminTasksAutocomplete(e *dc.AutocompleteEvent) {
	_, value := e.FocusedOption()
	search := strings.ToLower(strings.TrimSpace(value))

	choices := make([]dc.Choice[string], 0, len(adminTaskList))
	for _, task := range adminTaskList {
		if search == "" ||
			strings.Contains(strings.ToLower(task.ID), search) ||
			strings.Contains(strings.ToLower(task.Name), search) ||
			strings.Contains(strings.ToLower(task.Description), search) {
			choices = append(choices, dc.Choice[string]{
				Name:  task.Name,
				Value: task.ID,
			})
		}
	}

	_ = e.RespondChoices(choices)
}

// HandleAdminTasksCommand executes the chosen administrative task.
func HandleAdminTasksCommand(client dc.Client, e *dc.CommandEvent) {
	if !isAdminCommandCaller(client, e) {
		_ = e.Respond(dc.Message{
			Content:   "You are not authorized to use this command.",
			Ephemeral: true,
		})
		return
	}

	homeGuild := guildstate.GetGuildSettingString("DEFAULT", "home_guild")
	if homeGuild == "" {
		homeGuild = config.DiscordGuildID
	}
	if homeGuild != "" && homeGuild != "DISABLED" && e.GuildID() != homeGuild && e.UserID() != config.AdminUserID {
		_ = e.Respond(dc.Message{
			Content:   "This admin command can only be executed on the bot's home server.",
			Ephemeral: true,
		})
		return
	}

	if err := e.Defer(true); err != nil {
		log.Println("admin-tasks defer error:", err)
		return
	}

	taskName, ok := e.OptString("task")
	if !ok || strings.TrimSpace(taskName) == "" {
		_ = e.Followup(dc.Message{Content: "Please select a valid administrative task."})
		return
	}

	taskKey := strings.ToLower(strings.TrimSpace(taskName))

	switch taskKey {
	case "cycle-encryption-key", "cycle encryption key":
		handleCycleEncryptionKeyTask(e)
	case "reload-emojis", "reload emojis", "reload emoji cache", "refresh-emojis", "refresh emojis":
		handleReloadEmojisTask(client, e)
	default:
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("Unknown administrative task: `%s`", taskName),
		})
	}
}

func handleCycleEncryptionKeyTask(e *dc.CommandEvent) {
	oldKeyB64 := config.Key
	if oldKeyB64 == "" {
		_ = e.Followup(dc.Message{
			Content: "❌ Current encryption key is not loaded or is empty.",
		})
		return
	}

	// 1. Generate new 32-byte AES key
	newKeyBytes, err := config.GenerateKey()
	if err != nil {
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("❌ Failed to generate new encryption key: %v", err),
		})
		return
	}
	newKeyB64 := base64.StdEncoding.EncodeToString(newKeyBytes)

	// 2. Re-encrypt all farmer EIDs
	migratedCount, totalCount, err := farmerstate.ReencryptFarmerEIDs(oldKeyB64, newKeyB64)
	if err != nil {
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("❌ Failed during database re-encryption migration: %v", err),
		})
		return
	}

	// 3. Clear temporary user cache files
	_ = os.RemoveAll("ttbb-data/eiuserdata")

	// 4. Update and persist new key
	updatedFiles, err := config.UpdateKey(newKeyB64)
	if err != nil {
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("⚠️ Re-encryption finished (%d/%d records), but saving key to disk failed: %v", migratedCount, totalCount, err),
		})
		return
	}

	filesSummary := "None"
	if len(updatedFiles) > 0 {
		filesSummary = strings.Join(updatedFiles, ", ")
	}

	responseMsg := fmt.Sprintf(
		"## 🔐 Encryption Key Cycled Successfully\n"+
			"- **Farmer EID Records Migrated**: %d of %d\n"+
			"- **Key Storage Files Updated**: `%s`\n"+
			"- **Temporary API Cache**: Cleared\n"+
			"- **Active Key**: Updated in memory",
		migratedCount,
		totalCount,
		filesSummary,
	)

	_ = e.Followup(dc.Message{Content: responseMsg})
}

func handleReloadEmojisTask(client dc.Client, e *dc.CommandEvent) {
	count, err := bottools.ReloadEmotes(client)
	if err != nil {
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("❌ Failed to reload emojis: %v", err),
		})
		return
	}

	responseMsg := fmt.Sprintf(
		"## 🎨 Emojis Reloaded Successfully\n"+
			"- **Cache File**: `ttbb-data/Emotes.json` refreshed\n"+
			"- **Application Emojis Loaded**: %d",
		count,
	)

	_ = e.Followup(dc.Message{Content: responseMsg})
}
