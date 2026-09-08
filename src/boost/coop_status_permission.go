package boost

import (
	"log"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	// CoopStatusPermissionKey is the key used to store the timestamp of when the user allowed CoopStatus permissions
	CoopStatusPermissionKey = "allow_coop_status"
	// CoopStatusPermissionSpanKey is the key used to store the selected permission duration (24h or 7d)
	CoopStatusPermissionSpanKey = "allow_coop_status_span"
	// CoopStatusPermission24h is the duration for 24 hours permission
	CoopStatusPermission24h = 24 * time.Hour
	// CoopStatusPermission7d is the duration for 7 days permission
	CoopStatusPermission7d = 7 * 24 * time.Hour
)

func getCoopStatusPermissionDuration(userID string) time.Duration {
	span := farmerstate.GetMiscSettingString(userID, CoopStatusPermissionSpanKey)
	switch span {
	case "7d":
		return CoopStatusPermission7d
	default:
		return CoopStatusPermission24h
	}
}

// CheckCoopStatusPermission checks if a user needs permission for CoopStatus API calls
// Returns true if permission is valid or not needed, false if permission dialog is needed
// If permission is not valid, it shows the permission dialog and returns false
func CheckCoopStatusPermission(e *dc.CommandEvent, coopStatusFixEnabled bool) bool {
	// If the coop_status_fix is not enabled, permission is not needed
	if !coopStatusFixEnabled {
		return true
	}

	userID := e.UserID()

	// Check if user has a valid "allow_coop_status" timestamp
	timeStr := farmerstate.GetMiscSettingString(userID, CoopStatusPermissionKey)
	if timeStr == "" {
		// Timestamp doesn't exist, show permission dialog
		ShowCoopStatusPermissionDialog(e)
		return false
	}

	// Parse the timestamp
	parseTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		// Invalid timestamp format, show permission dialog
		ShowCoopStatusPermissionDialog(e)
		return false
	}

	// Check if timestamp is older than selected permission window.
	if time.Since(parseTime) > getCoopStatusPermissionDuration(userID) {
		// Timestamp is too old, show permission dialog
		ShowCoopStatusPermissionDialog(e)
		return false
	}

	// Permission is valid
	return true
}

// ShowCoopStatusPermissionDialog shows the ephemeral dialog with Allow and Close buttons
func ShowCoopStatusPermissionDialog(e *dc.CommandEvent) {
	err := e.Respond(dc.Message{
		Content:      "Due to a game API issue, your saved game ID is needed to make the request. You can allow this for 24 hours, allow this for 7 days, or close this dialog.\n\nUsing your EI number when you're in a contract and expecting to receive tokens or chickens can cause those deliveries to be lost.\n\nBecause of this you can only query about the contracts you're participating in.",
		Ephemeral:    true,
		ComponentsV1: true,
		Components: []dc.LayoutComponent{
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.Button{
						Label:    "Allow 24h",
						Style:    dc.ButtonSuccess,
						CustomID: "coop_status#allow24h",
					},
					dc.Button{
						Label:    "Allow 7d",
						Style:    dc.ButtonSuccess,
						CustomID: "coop_status#allow7d",
					},
					dc.Button{
						Label:    "Close",
						Style:    dc.ButtonDanger,
						CustomID: "coop_status#close",
					},
				},
			},
		},
	})
	if err != nil {
		log.Println("Error sending coop status permission dialog:", err)
	}
}

// HandleCoopStatusPermissionButton handles button interactions for the
// permission dialog through the dc facade.
func HandleCoopStatusPermissionButton(e *dc.ComponentEvent) {
	userID := e.UserID()
	customID := e.CustomID()
	respondAndClose := func(content string) {
		err := e.DeferUpdate()
		if err != nil {
			log.Println("Error acknowledging coop status permission dialog:", err)
			return
		}

		err = e.EditResponse(dc.Message{Content: content})
		if err != nil {
			log.Println("Error updating coop status permission dialog:", err)
		}
	}

	// Extract the action part after the "#"
	parts := strings.Split(customID, "#")
	if len(parts) < 2 {
		_ = e.Respond(dc.Message{
			Content:   "Invalid permission action. Use Allow 24h, Allow 7d, or Close from the permission dialog.",
			Ephemeral: true,
		})
		return
	}
	action := parts[1]

	switch action {
	case "allow", "allow24h":
		// Set the timestamp to now
		farmerstate.SetMiscSettingString(userID, CoopStatusPermissionKey, time.Now().Format(time.RFC3339))
		farmerstate.SetMiscSettingString(userID, CoopStatusPermissionSpanKey, "24h")

		respondAndClose("Permission granted for 24 hours. You can now run your command again.")

	case "allow7d":
		// Set the timestamp to now
		farmerstate.SetMiscSettingString(userID, CoopStatusPermissionKey, time.Now().Format(time.RFC3339))
		farmerstate.SetMiscSettingString(userID, CoopStatusPermissionSpanKey, "7d")

		respondAndClose("Permission granted for 7 days. You can now run your command again.")

	case "close":
		respondAndClose("I understand")
	default:
		_ = e.Respond(dc.Message{
			Content:   "Unknown permission action. Use Allow 24h, Allow 7d, or Close from the permission dialog.",
			Ephemeral: true,
		})
	}
}
