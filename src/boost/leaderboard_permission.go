package boost

import (
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	// LeaderboardPermissionKey is the key used to store the timestamp of when the user allowed leaderboard API permissions
	LeaderboardPermissionKey = "allow_leaderboard_apix"
	// LeaderboardPermissionSpanKey holds the selected permission duration ("24h" or "forever")
	LeaderboardPermissionSpanKey = "allow_leaderboard_apix_span"
	// LeaderboardPermission24h is the duration for 24 hours permission
	LeaderboardPermission24h = 24 * time.Hour
)

// CheckLeaderboardPermission checks if a user has granted permission for leaderboard API calls.
// Returns true if permission is valid, false if the permission dialog was shown.
func CheckLeaderboardPermission(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	userID := bottools.GetInteractionUserID(i)

	span := farmerstate.GetMiscSettingString(userID, LeaderboardPermissionSpanKey)
	if span == "forever" {
		return true
	}

	timeStr := farmerstate.GetMiscSettingString(userID, LeaderboardPermissionKey)
	if timeStr == "" {
		ShowLeaderboardPermissionDialog(s, i)
		return false
	}

	parseTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		ShowLeaderboardPermissionDialog(s, i)
		return false
	}

	if time.Since(parseTime) > LeaderboardPermission24h {
		ShowLeaderboardPermissionDialog(s, i)
		return false
	}

	return true
}

// ShowLeaderboardPermissionDialog shows the ephemeral dialog with Allow and Close buttons.
func ShowLeaderboardPermissionDialog(s *discordgo.Session, i *discordgo.InteractionCreate) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "This command makes an authenticated request using your saved Egg Inc ID. What do you want to do?",
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Allow for 24 hours",
							Style:    discordgo.SuccessButton,
							CustomID: "leaderboard_perm#allow24h",
						},
						discordgo.Button{
							Label:    "Allow forever",
							Style:    discordgo.SuccessButton,
							CustomID: "leaderboard_perm#allowforever",
						},
						discordgo.Button{
							Label:    "Close",
							Style:    discordgo.DangerButton,
							CustomID: "leaderboard_perm#close",
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Println("Error sending leaderboard permission dialog:", err)
	}
}

// HandleLeaderboardPermissionButton handles button interactions for the leaderboard permission dialog.
func HandleLeaderboardPermissionButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	customID := i.MessageComponentData().CustomID
	respondAndClose := func(content string) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{},
			},
		})
		if err != nil {
			log.Println("Error acknowledging leaderboard permission dialog:", err)
			return
		}

		emptyComponents := []discordgo.MessageComponent{}
		edit := discordgo.WebhookEdit{
			Content:    &content,
			Components: &emptyComponents,
		}
		_, err = s.InteractionResponseEdit(i.Interaction, &edit)
		if err != nil {
			log.Println("Error updating leaderboard permission dialog:", err)
		}
	}

	parts := strings.Split(customID, "#")
	if len(parts) < 2 {
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
	}
}
