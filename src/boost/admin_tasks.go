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
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

func isHomeGuild(guildID string) bool {
	homeGuild := guildstate.GetGuildSettingString("DEFAULT", "home_guild")
	if homeGuild == "" {
		homeGuild = config.DiscordGuildID
	}
	if homeGuild == "" || homeGuild == "DISABLED" {
		return true
	}
	return guildID == homeGuild
}

type adminTaskDef struct {
	ID          string
	Name        string
	Description string
}

// ThematicComplaintsGeneratorFunc generates contract-themed complaints.
type ThematicComplaintsGeneratorFunc func(eggName string, contractName string, contractDescription string, quantity int) []string

// PeriodicalsRefresherFunc triggers an update of periodicals / contracts / events from Egg Inc API.
type PeriodicalsRefresherFunc func(client dc.Client) bool

// TokenComplaintsRefresherFunc triggers a download and reload of token complaints.
type TokenComplaintsRefresherFunc func() (int, error)

// StatusMessagesRefresherFunc triggers a download and reload of status messages.
type StatusMessagesRefresherFunc func() (int, error)

var thematicComplaintsGenerator ThematicComplaintsGeneratorFunc
var periodicalsRefresher PeriodicalsRefresherFunc
var tokenComplaintsRefresher TokenComplaintsRefresherFunc
var statusMessagesRefresher StatusMessagesRefresherFunc

// SetThematicComplaintsGenerator configures the function used to generate complaints with LLM.
func SetThematicComplaintsGenerator(gen ThematicComplaintsGeneratorFunc) {
	thematicComplaintsGenerator = gen
}

// SetPeriodicalsRefresher configures the function used to refresh periodicals from API.
func SetPeriodicalsRefresher(refresher PeriodicalsRefresherFunc) {
	periodicalsRefresher = refresher
}

// SetTokenComplaintsRefresher configures the function used to refresh token complaints.
func SetTokenComplaintsRefresher(refresher TokenComplaintsRefresherFunc) {
	tokenComplaintsRefresher = refresher
}

// SetStatusMessagesRefresher configures the function used to refresh status messages.
func SetStatusMessagesRefresher(refresher StatusMessagesRefresherFunc) {
	statusMessagesRefresher = refresher
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
	{
		ID:          "refresh-periodicals",
		Name:        "Refresh Periodicals",
		Description: "Download latest contracts and events from Egg Inc API",
	},
	{
		ID:          "check-colleggtible",
		Name:        "Check for Colleggtible",
		Description: "Refresh ei-config.json and check periodicals if a new hatchery is found",
	},
	{
		ID:          "regen-complaints",
		Name:        "Regen Complaints",
		Description: "Regenerate AI contract complaints for a specific contract",
	},
	{
		ID:          "refresh-token-complaints",
		Name:        "Refresh Token Complaints",
		Description: "Replace token-complaints.json with latest and reload internal data",
	},
	{
		ID:          "refresh-status-messages",
		Name:        "Refresh Status Messages",
		Description: "Replace status-messages.json with latest and reload internal data",
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
		dc.StringOption{
			Name:         "param",
			Description:  "Optional parameter (e.g. contract ID for Regen Complaints)",
			Required:     false,
			Autocomplete: true,
		},
	}
	return &command
}

// HandleAdminTasksAutocomplete provides autocomplete suggestions for administrative tasks and parameters.
func HandleAdminTasksAutocomplete(e *dc.AutocompleteEvent) {
	focusedName, focusedValue := e.FocusedOption()
	search := strings.ToLower(strings.TrimSpace(focusedValue))
	isAdminUser := e.UserID() == config.AdminUserID
	isHome := isHomeGuild(e.GuildID())

	if focusedName == "param" {
		taskName, _ := e.OptString("task")
		taskKey := strings.ToLower(strings.TrimSpace(taskName))
		if isRegenComplaintsTask(taskKey) && isAdminUser {
			handleAdminTasksParamContractAutoComplete(e, search)
			return
		}
		choices := []dc.Choice[string]{
			{
				Name:  "No entry needed",
				Value: "no entry needed",
			},
		}
		_ = e.RespondChoices(choices)
		return
	}

	choices := filterAdminTaskChoices(isAdminUser, isHome, search)
	_ = e.RespondChoices(choices)
}

func filterAdminTaskChoices(isAdminUser, isHome bool, search string) []dc.Choice[string] {
	choices := make([]dc.Choice[string], 0)
	for _, task := range adminTaskList {
		if task.ID == "cycle-encryption-key" {
			if !isAdminUser || !isHome || search == "" {
				continue
			}
			if !strings.Contains(strings.ToLower(task.Name), search) &&
				!strings.Contains(strings.ToLower(task.ID), search) {
				continue
			}
			choices = append(choices, dc.Choice[string]{
				Name:  task.Name,
				Value: task.ID,
			})
			continue
		}
		if task.ID == "regen-complaints" && !isAdminUser {
			continue
		}
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

	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices
}

func handleAdminTasksParamContractAutoComplete(e *dc.AutocompleteEvent, search string) {
	if search == "no entry needed" || search == "none" {
		search = ""
	}

	choices := make([]dc.Choice[string], 0)
	contracts := ei.GetEggIncContractsSlice()

	if search == "" {
		for _, c := range contracts {
			if c.Predicted || c.ID == "" {
				continue
			}
			name := c.ID
			if c.Name != "" && !strings.EqualFold(c.Name, c.ID) {
				name = fmt.Sprintf("%s (%s)", c.Name, c.ID)
			}
			choices = append(choices, dc.Choice[string]{
				Name:  name,
				Value: c.ID,
			})
		}
	} else {
		seen := make(map[string]bool)
		for _, c := range contracts {
			if c.Predicted || c.ID == "" {
				continue
			}
			if strings.Contains(strings.ToLower(c.ID), search) || strings.Contains(strings.ToLower(c.Name), search) {
				name := c.ID
				if c.Name != "" && !strings.EqualFold(c.Name, c.ID) {
					name = fmt.Sprintf("%s (%s)", c.Name, c.ID)
				}
				choices = append(choices, dc.Choice[string]{
					Name:  name,
					Value: c.ID,
				})
				seen[c.ID] = true
			}
		}

		ei.EggIncContractsMutex.RLock()
		for id, c := range ei.EggIncContractsAll {
			if seen[id] || c.Predicted || id == "" {
				continue
			}
			if strings.Contains(strings.ToLower(c.ID), search) || strings.Contains(strings.ToLower(c.Name), search) {
				name := c.ID
				if c.Name != "" && !strings.EqualFold(c.Name, c.ID) {
					name = fmt.Sprintf("%s (%s)", c.Name, c.ID)
				}
				choices = append(choices, dc.Choice[string]{
					Name:  name,
					Value: c.ID,
				})
				seen[id] = true
				if len(choices) >= 25 {
					break
				}
			}
		}
		ei.EggIncContractsMutex.RUnlock()

		// Fall back to active contracts if search matched nothing (e.g. leftover text from another task)
		if len(choices) == 0 {
			for _, c := range contracts {
				if c.Predicted || c.ID == "" {
					continue
				}
				name := c.ID
				if c.Name != "" && !strings.EqualFold(c.Name, c.ID) {
					name = fmt.Sprintf("%s (%s)", c.Name, c.ID)
				}
				choices = append(choices, dc.Choice[string]{
					Name:  name,
					Value: c.ID,
				})
			}
		}
	}

	if len(choices) > 25 {
		choices = choices[:25]
	}

	_ = e.RespondChoices(choices)
}

func isRegenComplaintsTask(taskKey string) bool {
	prefixes := []string{
		"regen-complaints",
		"regen complaints",
		"regen-complaings",
		"regen complaings",
		"regen-complaint",
		"regen complaint",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(taskKey, p) {
			return true
		}
	}
	return false
}

func extractContractIDFromTask(taskName string) string {
	lower := strings.ToLower(strings.TrimSpace(taskName))
	prefixes := []string{
		"regen-complaints-",
		"regen-complaints:",
		"regen-complaints ",
		"regen complaints:",
		"regen complaints-",
		"regen complaints ",
		"regen-complaings-",
		"regen-complaings:",
		"regen-complaings ",
		"regen complaings:",
		"regen complaings-",
		"regen complaings ",
		"regen-complaint-",
		"regen-complaint:",
		"regen-complaint ",
		"regen complaint:",
		"regen complaint-",
		"regen complaint ",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			rawID := strings.TrimSpace(taskName[len(p):])
			if idx := strings.Index(rawID, " ("); idx != -1 {
				rawID = strings.TrimSpace(rawID[:idx])
			}
			return rawID
		}
	}
	if lower == "regen-complaints" || lower == "regen complaints" || lower == "regen-complaings" || lower == "regen complaings" || lower == "regen-complaint" || lower == "regen complaint" {
		return ""
	}
	return strings.TrimSpace(taskName)
}

// HandleAdminTasksCommand executes the chosen administrative task.
func HandleAdminTasksCommand(client dc.Client, e *dc.CommandEvent) {
	if !isAdminCommandCaller(client, e) && !isAdminBotController(client, e) {
		_ = e.Respond(dc.Message{
			Content:   "You are not authorized to use this command.",
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
	param, _ := e.OptString("param")

	taskKey := strings.ToLower(strings.TrimSpace(taskName))

	switch {
	case taskKey == "cycle-encryption-key" || taskKey == "cycle encryption key":
		if e.UserID() != config.AdminUserID {
			_ = e.Followup(dc.Message{
				Content: "❌ You are not authorized to execute this task. This task is restricted to the bot owner.",
			})
			return
		}
		if !isHomeGuild(e.GuildID()) {
			_ = e.Followup(dc.Message{
				Content: "❌ This task can only be executed on the bot's home server.",
			})
			return
		}
		handleCycleEncryptionKeyTask(e)
	case taskKey == "reload-emojis" || taskKey == "reload emojis" || taskKey == "reload emoji cache" || taskKey == "refresh-emojis" || taskKey == "refresh emojis":
		handleReloadEmojisTask(client, e)
	case taskKey == "refresh-periodicals" || taskKey == "refresh periodicals" || taskKey == "periodicals" || taskKey == "refresh-events" || taskKey == "refresh events":
		handleRefreshPeriodicalsTask(client, e)
	case taskKey == "check-colleggtible" || taskKey == "check for colleggtible" || taskKey == "check-for-colleggtible" ||
		taskKey == "check-colleggtibles" || taskKey == "check for colleggtibles" || taskKey == "check-for-colleggtibles" ||
		taskKey == "check colleggtible" || taskKey == "check colleggtibles":
		handleCheckColleggtibleTask(client, e)
	case isRegenComplaintsTask(taskKey):
		if e.UserID() != config.AdminUserID {
			_ = e.Followup(dc.Message{
				Content: "❌ You are not authorized to execute this task. This task is restricted to the bot owner.",
			})
			return
		}
		handleRegenComplaintsTask(e, taskName, param)
	case taskKey == "refresh-token-complaints" || taskKey == "refresh token complaints" ||
		taskKey == "refresh-token-complaints.json" || taskKey == "token-complaints" ||
		taskKey == "token-complaints.json" || taskKey == "token complaints":
		handleRefreshTokenComplaintsTask(e)
	case taskKey == "refresh-status-messages" || taskKey == "refresh status messages" ||
		taskKey == "refresh-status-messages.json" || taskKey == "status-messages" ||
		taskKey == "status-messages.json" || taskKey == "status messages":
		handleRefreshStatusMessagesTask(e)
	default:
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("Unknown administrative task: `%s`", taskName),
		})
	}
}

func handleRegenComplaintsTask(e *dc.CommandEvent, taskName string, param string) {
	contractID := strings.TrimSpace(param)
	if strings.EqualFold(contractID, "no entry needed") || strings.EqualFold(contractID, "none") {
		contractID = ""
	}
	if contractID == "" {
		contractID = extractContractIDFromTask(taskName)
	}
	if contractID == "" || isRegenComplaintsTask(strings.ToLower(contractID)) {
		_ = e.Followup(dc.Message{
			Content: "❌ Please specify a contract ID using the `param` option (e.g. `/admin-tasks task:Regen Complaints param:<contract-id>`).",
		})
		return
	}

	eiContract, found := ei.GetEggIncContract(contractID)
	if !found {
		// Try case-insensitive lookup in active contracts
		for _, c := range ei.GetEggIncContractsSlice() {
			if strings.EqualFold(c.ID, contractID) || strings.EqualFold(c.Name, contractID) {
				eiContract = c
				contractID = c.ID
				found = true
				break
			}
		}
	}

	if !found {
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("❌ Contract `%s` not found.", contractID),
		})
		return
	}

	if thematicComplaintsGenerator == nil {
		_ = e.Followup(dc.Message{
			Content: "❌ Thematic complaints generator is not initialized.",
		})
		return
	}

	if config.GoogleAPIKey == "" {
		_ = e.Followup(dc.Message{
			Content: "❌ Google API key is not configured.",
		})
		return
	}

	const complaintQuantity = 12
	complaints := thematicComplaintsGenerator(eiContract.EggName, eiContract.Name, eiContract.Description, complaintQuantity)
	if len(complaints) == 0 {
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("❌ Failed to generate complaints for `%s` (%s).", eiContract.Name, contractID),
		})
		return
	}

	if err := SaveThematicComplaints(map[string][]string{contractID: complaints}); err != nil {
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("❌ Failed to save regenerated complaints to database: %v", err),
		})
		return
	}

	ReplaceThematicComplaintsForContractID(contractID, complaints)

	var sb strings.Builder
	for i, c := range complaints {
		if i >= 5 {
			fmt.Fprintf(&sb, "- ...and %d more\n", len(complaints)-5)
			break
		}
		fmt.Fprintf(&sb, "- %s\n", c)
	}

	responseMsg := fmt.Sprintf(
		"## 🔄 Regenerated Complaints for `%s`\n"+
			"- **Contract ID**: `%s`\n"+
			"- **Egg**: %s\n"+
			"- **Complaints Generated**: %d\n\n"+
			"**Sample Complaints:**\n%s",
		eiContract.Name,
		contractID,
		eiContract.EggName,
		len(complaints),
		sb.String(),
	)

	_ = e.Followup(dc.Message{Content: responseMsg})
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

func handleRefreshPeriodicalsTask(client dc.Client, e *dc.CommandEvent) {
	if periodicalsRefresher == nil {
		_ = e.Followup(dc.Message{
			Content: "❌ Periodicals refresher is not initialized.",
		})
		return
	}

	_ = periodicalsRefresher(client)

	activeContracts := len(ei.GetEggIncContractsSlice())
	responseMsg := fmt.Sprintf(
		"## 🔄 Periodicals Refreshed Successfully\n"+
			"- **Active Contracts in Memory**: %d\n"+
			"- **Events & Contract Data**: Updated from Egg Inc API",
		activeContracts,
	)
	_ = e.Followup(dc.Message{Content: responseMsg})
}

func handleCheckColleggtibleTask(client dc.Client, e *dc.CommandEvent) {
	hasNewHatchery, diff, err := ei.RefreshConfigAndCheckNewHatchery(client)
	if err != nil {
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("❌ Failed to refresh Egg Inc config: %v", err),
		})
		return
	}

	if hasNewHatchery {
		periodicalsMsg := "Periodicals refresher not initialized."
		if periodicalsRefresher != nil {
			_ = periodicalsRefresher(client)
			periodicalsMsg = "Periodicals checked and updated with new contract data."
		}

		diffPreview := diff
		if len(diffPreview) > 500 {
			diffPreview = diffPreview[:500] + "\n..."
		}

		responseMsg := fmt.Sprintf(
			"## 🥚 New Colleggtible Hatchery Found!\n"+
				"- **Config**: `ttbb-data/ei-config.json` refreshed\n"+
				"- **Periodicals**: %s\n\n"+
				"**Diff:**\n```diff\n%s\n```",
			periodicalsMsg,
			diffPreview,
		)
		_ = e.Followup(dc.Message{Content: responseMsg})
		return
	}

	responseMsg := "## 🥚 Checked for Colleggtibles\n" +
		"- **Config**: `ttbb-data/ei-config.json` refreshed\n" +
		"- **Status**: No new custom hatcheries found"
	_ = e.Followup(dc.Message{Content: responseMsg})
}

func handleRefreshTokenComplaintsTask(e *dc.CommandEvent) {
	if tokenComplaintsRefresher == nil {
		_ = e.Followup(dc.Message{
			Content: "❌ Token complaints refresher is not initialized.",
		})
		return
	}

	count, err := tokenComplaintsRefresher()
	if err != nil {
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("❌ Failed to refresh token complaints: %v", err),
		})
		return
	}

	responseMsg := fmt.Sprintf(
		"## 🔄 Token Complaints Refreshed Successfully\n"+
			"- **File**: `ttbb-data/token-complaints.json` replaced\n"+
			"- **Token Complaints Loaded**: %d\n"+
			"- **Internal Data**: Updated in memory",
		count,
	)
	_ = e.Followup(dc.Message{Content: responseMsg})
}

func handleRefreshStatusMessagesTask(e *dc.CommandEvent) {
	if statusMessagesRefresher == nil {
		_ = e.Followup(dc.Message{
			Content: "❌ Status messages refresher is not initialized.",
		})
		return
	}

	count, err := statusMessagesRefresher()
	if err != nil {
		_ = e.Followup(dc.Message{
			Content: fmt.Sprintf("❌ Failed to refresh status messages: %v", err),
		})
		return
	}

	responseMsg := fmt.Sprintf(
		"## 🔄 Status Messages Refreshed Successfully\n"+
			"- **File**: `ttbb-data/status-messages.json` replaced\n"+
			"- **Status Messages Loaded**: %d\n"+
			"- **Internal Data**: Updated in memory",
		count,
	)
	_ = e.Followup(dc.Message{Content: responseMsg})
}
