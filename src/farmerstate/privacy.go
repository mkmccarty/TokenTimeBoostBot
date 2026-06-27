package farmerstate

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

// GetSlashPrivacyCommand creates a new slash command for setting Egg, Inc name
func GetSlashPrivacyCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Boost bot privacy information.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "enable-data-privacy",
				Description: "Change your data privacy setting.",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Do not persist bot settings.",
						Value: 1,
					},
					{
						Name:  "Allow the bot to store some information.",
						Value: 0,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "get-settings-data",
				Description: "Retrieve a JSON file with your stored settings.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "remove-data",
				Description: "Remove my data from the boost bot database.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "confirm-request",
				Description: "Confirm privacy setting change or data removal.",
				Required:    false,
			},
		},
	}
}

// HandlePrivacyCommand will handle the /privacy command
func HandlePrivacyCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	optionMap := bottools.GetCommandOptionsMap(i)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	var builder strings.Builder

	builder.WriteString("# Privacy information for user: <@" + userID + ">\n")
	builder.WriteString("Boost Bot stores some information about your usage to provide you with a better experience and to improve the bot.\n")
	builder.WriteString("Your Discord User ID is used as a key to this saved information.\n")
	builder.WriteString("**This information is never sold.** It will only be shared with other Boost Bot developers or testers within the Bot's development Discord server.\n")
	builder.WriteString("You can view this information at any time using the **get-settings-data** option.\n")
	builder.WriteString("\n")
	var filename string
	var reader *bytes.Reader

	userPrivacy := getDataPrivacy(userID)
	removeData := false
	confirmOption := false

	// User must confirm data removal and/or privacy setting change
	if opt, ok := optionMap["confirm-request"]; ok {
		confirmOption = opt.BoolValue()
	}

	if opt, ok := optionMap["enable-data-privacy"]; ok {
		userPrivacy = opt.IntValue() == 1
		if userPrivacy && confirmOption {
			builder.WriteString("Boost Bot wil no longer store any persistent data about you.")
			builder.WriteString("If you wish to store data again, you will need to re-enable it.")
			builder.WriteString("If you interact with the bot for contracts and token tracking it will be stored temporarily and removed within a week of the last interaction of a contract or tracker.")
			removeData = true
		} else if userPrivacy && !confirmOption {
			builder.WriteString("You have not confirmed the privacy setting change, use the **confirm-request** option.")
		} else {
			setDataPrivacy(userID, userPrivacy)
			builder.WriteString("Boost Bot will store save a small amount of data about you. You can download this data at any time.")
		}
	}

	if opt, ok := optionMap["remove-data"]; ok {
		removeData = opt.BoolValue()
		if removeData && !confirmOption {
			builder.WriteString("You have not confirmed your data removal, use the **confirm-request** option.")
			removeData = false
		}
	}

	if removeData {
		builder.WriteString("Your settings data have been removed from the Boost Bot database.")
		builder.WriteString("A default set of settings is now used along with your preference not to store data.")

		// Want to remove all files from within ttbb-data/eiuserdata/ which contain this userID in the filename
		userFilesPattern := "ttbb-data/eiuserdata/*" + userID + "*"
		matches, err := filepath.Glob(userFilesPattern)
		if err != nil {
			log.Printf("Error finding user data files for deletion: %v", err)
		} else {
			for _, file := range matches {
				if err := os.Remove(file); err != nil {
					log.Printf("Error deleting file %s: %v", file, err)
				}
			}
		}

		DeleteFarmer(userID)

		setDataPrivacy(userID, userPrivacy)
	}

	if opt, ok := optionMap["get-settings-data"]; ok {
		// Return the users settings data in a JSON file to the user
		getData := opt.BoolValue()
		if getData {
			userData := GetFullUserData(userID)

			filename = "boostbot-data-" + userID + ".json"
			buf := &bytes.Buffer{}
			jsonData, err := json.Marshal(userData)
			if err != nil {
				log.Println(err.Error())
				builder.WriteString("Error formatting JSON data. " + err.Error())
			} else {
				err = json.Indent(buf, jsonData, "", "  ")
				if err != nil {
					builder.WriteString("Error formatting JSON data. " + err.Error())
				} else {
					// Create io.Reader from JSON string
					reader = bytes.NewReader(buf.Bytes())
				}
			}
		} else {
			builder.WriteString("\nYou have not requested your settings data, use the **get-settings-data** option with a value of true.")
		}
	}
	if reader != nil {
		builder.WriteString("Your settings data has been saved to a JSON file. You can view and download it.")
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: builder.String(),
				Files:   []*discordgo.File{{Name: filename, Reader: reader}},
			})
	} else {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: builder.String(),
			})
	}
}

func getDataPrivacy(userID string) bool {
	farmer := getFarmer(userID)
	return farmer.DataPrivacy
}

func setDataPrivacy(userID string, dataPrivacy bool) {
	farmer := getFarmer(userID)
	farmer.DataPrivacy = dataPrivacy
	farmer.LastUpdated = time.Now()
	saveSqliteData(userID, farmer)
}

// UserPrivacyData contains all information we store about a user
type UserPrivacyData struct {
	FarmerState           *Farmer                              `json:"farmer_state,omitempty"`
	GuildMemberships      []string                             `json:"guild_memberships,omitempty"`
	CustomBanners         []CustomBanner                       `json:"custom_banners,omitempty"`
	Timers                []Timer                              `json:"timers,omitempty"`
	SuspectMissions       []SuspectMission                     `json:"suspect_missions,omitempty"`
	LeaderboardStats      []LeaderboardStat                    `json:"leaderboard_stats,omitempty"`
	LeaderboardOptins     []GetLeaderboardOptInsForUserRow     `json:"leaderboard_optins,omitempty"`
	LeaderboardExclusions []GetLeaderboardExclusionsForUserRow `json:"leaderboard_exclusions,omitempty"`
	Watches               []Watch                              `json:"watches,omitempty"`
}

func GetFullUserData(userID string) *UserPrivacyData {
	data := &UserPrivacyData{}
	data.FarmerState = getFarmer(userID)

	if queries != nil {
		if guilds, err := queries.GetUserGuilds(ctx, userID); err == nil {
			data.GuildMemberships = guilds
		}
		if banners, err := queries.GetCustomBannersForUser(ctx, userID); err == nil {
			data.CustomBanners = banners
		}
		if timers, err := queries.GetTimersForUser(ctx, userID); err == nil {
			data.Timers = timers
		}
		if missions, err := queries.GetSuspectMissions(ctx, userID); err == nil {
			data.SuspectMissions = missions
		}
		if stats, err := queries.GetStatsForPlayer(ctx, userID); err == nil {
			data.LeaderboardStats = stats
		}
		if optins, err := queries.GetLeaderboardOptInsForUser(ctx, userID); err == nil {
			data.LeaderboardOptins = optins
		}
		if exclusions, err := queries.GetLeaderboardExclusionsForUser(ctx, userID); err == nil {
			data.LeaderboardExclusions = exclusions
		}
		if watches, err := queries.GetWatchesForUser(ctx, userID); err == nil {
			data.Watches = watches
		}
	}
	return data
}
