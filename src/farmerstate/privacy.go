package farmerstate

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// GetSlashPrivacyCommand creates a new slash command for setting Egg, Inc name
func GetSlashPrivacyCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Boost bot privacy information.",
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

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

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

	// Convert userData to JSON string
	var userData *Farmer
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

		DeleteFarmer(userID)
		setDataPrivacy(userID, userPrivacy)
	}

	if opt, ok := optionMap["get-settings-data"]; ok {
		// Return the users settings data in a JSON file to the user
		getData := opt.BoolValue()
		if getData {
			userData = farmerstate[userID]

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
	if farmerstate, ok := farmerstate[userID]; ok {
		return farmerstate.DataPrivacy
	}
	return false
}

func setDataPrivacy(userID string, dataPrivacy bool) {
	if farmerstate[userID] == nil {
		newFarmer(userID)
	}

	farmerstate[userID].DataPrivacy = dataPrivacy
	farmerstate[userID].LastUpdated = time.Now()
	saveData(farmerstate)
}
