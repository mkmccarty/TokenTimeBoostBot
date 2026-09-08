package boost

import (
	"log"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	// LeaderboardPermissionKey is the key used to store the timestamp of when the user allowed leaderboard API permissions
	LeaderboardPermissionKey = "allow_leaderboard_api"
	// LeaderboardPermissionSpanKey holds the selected permission duration ("24h" or "forever")
	LeaderboardPermissionSpanKey = "allow_leaderboard_api_span"
	// LeaderboardPermission24h is the duration for 24 hours permission
	LeaderboardPermission24h = 24 * time.Hour
)

// CheckLeaderboardPermission checks if a user has granted permission for leaderboard API calls.
// Returns true if permission is valid, false if the permission dialog was shown.
func CheckLeaderboardPermission(e dc.InteractionEvent) bool {
	userID := e.UserID()

	span := farmerstate.GetMiscSettingString(userID, LeaderboardPermissionSpanKey)
	if span == "forever" {
		return true
	}

	timeStr := farmerstate.GetMiscSettingString(userID, LeaderboardPermissionKey)
	if timeStr == "" {
		ShowLeaderboardPermissionDialog(e)
		return false
	}

	parseTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		ShowLeaderboardPermissionDialog(e)
		return false
	}

	if time.Since(parseTime) > LeaderboardPermission24h {
		ShowLeaderboardPermissionDialog(e)
		return false
	}

	return true
}

// ShowLeaderboardPermissionDialog shows the ephemeral dialog with Allow and Close buttons.
func ShowLeaderboardPermissionDialog(e dc.InteractionEvent) {
	err := e.Respond(dc.Message{
		Content:   "This command makes an authenticated request using your saved Egg Inc ID. What do you want to do?",
		Ephemeral: true,
		Components: []dc.LayoutComponent{
			dc.ActionRow{Components: []dc.InteractiveComponent{
				dc.Button{
					Label:    "Allow for 24 hours",
					Style:    dc.ButtonSuccess,
					CustomID: "leaderboard_perm#allow24h",
				},
				dc.Button{
					Label:    "Allow forever",
					Style:    dc.ButtonSuccess,
					CustomID: "leaderboard_perm#allowforever",
				},
				dc.Button{
					Label:    "Close",
					Style:    dc.ButtonDanger,
					CustomID: "leaderboard_perm#close",
				},
			}},
		},
		ComponentsV1: true,
	})
	if err != nil {
		log.Println("Error sending leaderboard permission dialog:", err)
	}
}

// HandleLeaderboardPermissionButton records the answer to the permission
// dialog and closes it.
func HandleLeaderboardPermissionButton(e *dc.ComponentEvent) {
	userID := e.UserID()
	respondAndClose := func(content string) {
		if err := e.DeferUpdate(); err != nil {
			log.Println("Error acknowledging leaderboard permission dialog:", err)
			return
		}

		// EditResponse always sends a component list, so the buttons come off
		// with the text swap.
		if err := e.EditResponse(dc.Message{Content: content}); err != nil {
			log.Println("Error updating leaderboard permission dialog:", err)
		}
	}

	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		_ = e.Respond(dc.Message{
			Content:   "Invalid permission action. Use Allow for 24 hours, Allow forever, or Close from the permission dialog.",
			Ephemeral: true,
		})
		return
	}
	action := parts[1]

	switch action {
	case "allow24h":
		farmerstate.SetMiscSettingString(userID, LeaderboardPermissionKey, time.Now().Format(time.RFC3339))
		farmerstate.SetMiscSettingString(userID, LeaderboardPermissionSpanKey, "24h")

		respondAndClose("Permission granted for 24 hours. Please run your command again.")

	case "allowforever":
		farmerstate.SetMiscSettingString(userID, LeaderboardPermissionKey, time.Now().Format(time.RFC3339))
		farmerstate.SetMiscSettingString(userID, LeaderboardPermissionSpanKey, "forever")

		respondAndClose("Permission granted permanently. Please run your command again.")

	case "close":
		respondAndClose("I understand")
	default:
		_ = e.Respond(dc.Message{
			Content:   "Unknown permission action. Use Allow for 24 hours, Allow forever, or Close from the permission dialog.",
			Ephemeral: true,
		})
	}
}
