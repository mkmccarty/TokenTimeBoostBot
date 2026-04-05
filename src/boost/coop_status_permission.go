package boost

import (
	"log"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
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
func CheckCoopStatusPermission(s *discordgo.Session, i *discordgo.InteractionCreate, coopStatusFixEnabled bool) bool {
	// If the coop_status_fix is not enabled, permission is not needed
	if !coopStatusFixEnabled {
		return true
	}

	userID := bottools.GetInteractionUserID(i)

	// Check if user has a valid "allow_coop_status" timestamp
	timeStr := farmerstate.GetMiscSettingString(userID, CoopStatusPermissionKey)
	if timeStr == "" {
		// Timestamp doesn't exist, show permission dialog
		ShowCoopStatusPermissionDialog(s, i)
		return false
	}

	// Parse the timestamp
	parseTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		// Invalid timestamp format, show permission dialog
		ShowCoopStatusPermissionDialog(s, i)
		return false
	}

	// Check if timestamp is older than selected permission window.
	if time.Since(parseTime) > getCoopStatusPermissionDuration(userID) {
		// Timestamp is too old, show permission dialog
		ShowCoopStatusPermissionDialog(s, i)
		return false
	}

	// Permission is valid
	return true
}

// ShowCoopStatusPermissionDialog shows the ephemeral dialog with Allow and Close buttons
func ShowCoopStatusPermissionDialog(s *discordgo.Session, i *discordgo.InteractionCreate) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Due to a game API issue, your saved game ID is needed to make the request. You can allow this for 24 hours, allow this for 7 days, or close this dialog.\n\nUsing your EI number when you're in a contract and expecting to receive tokens or chickens can cause those deliveries to be lost.\n\nBecause of this you can only query about the contracts you're participating in.",
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Allow 24h",
							Style:    discordgo.SuccessButton,
							CustomID: "coop_status#allow24h",
						},
						discordgo.Button{
							Label:    "Allow 7d",
							Style:    discordgo.SuccessButton,
							CustomID: "coop_status#allow7d",
						},
						discordgo.Button{
							Label:    "Close",
							Style:    discordgo.SecondaryButton,
							CustomID: "coop_status#close",
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Println("Error sending coop status permission dialog:", err)
	}
}

// HandleCoopStatusPermissionButton handles button interactions for the permission dialog
func HandleCoopStatusPermissionButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	customID := i.MessageComponentData().CustomID

	// Extract the action part after the "#"
	parts := strings.Split(customID, "#")
	if len(parts) < 2 {
		return
	}
	action := parts[1]

	switch action {
	case "allow", "allow24h":
		// Set the timestamp to now
		farmerstate.SetMiscSettingString(userID, CoopStatusPermissionKey, time.Now().Format(time.RFC3339))
		farmerstate.SetMiscSettingString(userID, CoopStatusPermissionSpanKey, "24h")

		// Respond with ephemeral message asking to retry
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Permission granted for 24 hours. You can now run your command again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Println("Error responding to allow button:", err)
		}

	case "allow7d":
		// Set the timestamp to now
		farmerstate.SetMiscSettingString(userID, CoopStatusPermissionKey, time.Now().Format(time.RFC3339))
		farmerstate.SetMiscSettingString(userID, CoopStatusPermissionSpanKey, "7d")

		// Respond with ephemeral message asking to retry
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Permission granted for 7 days. You can now run your command again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Println("Error responding to allow 7d button:", err)
		}

	case "close":
		// Respond with ephemeral close message
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Dialog closed. You can enable this permission later when you're ready.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Println("Error responding to close button:", err)
		}
	}
}
