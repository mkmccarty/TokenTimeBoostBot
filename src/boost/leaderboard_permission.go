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
	LeaderboardPermissionKey = "allow_leaderboard_api"
	// LeaderboardPermissionSpanKey holds the selected permission duration ("24h" or "forever")
	LeaderboardPermissionSpanKey = "allow_leaderboard_api_span"
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

	parts := strings.Split(customID, "#")
	if len(parts) < 2 {
		return
	}
	action := parts[1]

	switch action {
	case "allow24h":
		farmerstate.SetMiscSettingString(userID, LeaderboardPermissionKey, time.Now().Format(time.RFC3339))
		farmerstate.SetMiscSettingString(userID, LeaderboardPermissionSpanKey, "24h")

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Permission granted for 24 hours. Please run your command again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Println("Error responding to leaderboard allow24h button:", err)
		}

	case "allowforever":
		farmerstate.SetMiscSettingString(userID, LeaderboardPermissionKey, time.Now().Format(time.RFC3339))
		farmerstate.SetMiscSettingString(userID, LeaderboardPermissionSpanKey, "forever")

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Permission granted permanently. Please run your command again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Println("Error responding to leaderboard allowforever button:", err)
		}

	case "close":
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Dialog closed.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Println("Error responding to leaderboard close button:", err)
		}
	}
}
