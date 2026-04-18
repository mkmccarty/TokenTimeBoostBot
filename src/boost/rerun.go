package boost

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"log"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

// GetSlashRerunEvalCommand returns the command for the /launch-helper command
func GetSlashRerunEvalCommand(cmd string) *discordgo.ApplicationCommand {
	minValue := 0.0
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Evaluate a contract's history and provide replay guidance.",
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
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "active",
				Description: "Evaluate Active Contract Details",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "contract-id",
						Description:  "Contract ID",
						Required:     true,
						Autocomplete: true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "refresh",
						Description: "If you want to force a refresh due a recent change to your contracts.",
						Required:    false,
					},
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "mobile-friendly",
						Description: "Format output for mobile devices (sticky)",
						Required:    false,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "chart",
				Description: "Summary chart of active contracts evaluations",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "refresh",
						Description: "If you want to force a refresh due a recent change to your contracts.",
						Required:    false,
					},
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "mobile-friendly",
						Description: "Format output for mobile devices (sticky)",
						Required:    false,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "predictions",
				Description: "Summary chart of predicted contracts evaluations",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "refresh",
						Description: "If you want to force a refresh due a recent change to your contracts.",
						Required:    false,
					},
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "mobile-friendly",
						Description: "Format output for mobile devices (sticky)",
						Required:    false,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "threshold",
				Description: "Summarize contracts below a certain % of speedrun score",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "percent",
						Description: "Below % of speedrun score",
						MinValue:    &minValue,
						MaxValue:    50,
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "mobile-friendly",
						Description: "Format output for mobile devices (sticky)",
						Required:    false,
					},
				},
			},
		},
	}
}

// HandleReplayEval handles the /replay-eval command
func HandleReplayEval(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Check if user has permission to use CoopStatus API
	if !CheckCoopStatusPermission(s, i, ei.CoopStatusFixEnabled != nil && ei.CoopStatusFixEnabled()) {
		return
	}

	userID := bottools.GetInteractionUserID(i)

	optionMap := bottools.GetCommandOptionsMap(i)
	if opt, ok := optionMap["reset"]; ok {
		if opt.BoolValue() {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
		}
	}
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	RerunEval(s, i, optionMap, eiID, true)
}

// RerunEval evaluates the contract history and provides replay guidance
func RerunEval(s *discordgo.Session, i *discordgo.InteractionCreate, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption, eiID string, okayToSave bool) {
	// Get the Egg Inc ID from the stored settings
	eggIncID := ""
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		decodedData, err := base64.StdEncoding.DecodeString(eiID)
		if err == nil {
			decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
			if err == nil {
				eggIncID = string(decryptedData)
			}
		}
	}
	if eggIncID == "" || len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		RequestEggIncIDModal(s, i, "replay", optionMap)
		return
	}

	percent := -1
	page := 1
	contractID := ""
	forceRefresh := false
	contractIDList := []string{}

	if opt, ok := optionMap["threshold-percent"]; ok {
		percent = int(opt.UintValue())
	}
	if opt, ok := optionMap["active-contract-id"]; ok {
		contractID = opt.StringValue()
		contractIDList = append(contractIDList, contractID)
	}
	contractDayMap := make(map[string]string)
	if _, ok := optionMap["predictions"]; ok {
		fridayNonUltra, fridayUltra, wednesdayNonUltra := predictJeli(3)
		// for each of these 3 I want to collect the contract IDs
		for _, c := range fridayNonUltra {
			if slices.Contains(contractIDList, c.ID) {
				continue
			}
			contractIDList = append(contractIDList, c.ID)
			contractDayMap[c.ID] = "F"
		}
		for _, c := range fridayUltra {
			if slices.Contains(contractIDList, c.ID) {
				continue
			}
			contractIDList = append(contractIDList, c.ID)
			contractDayMap[c.ID] = "U"
		}
		for _, c := range wednesdayNonUltra {
			if slices.Contains(contractIDList, c.ID) {
				continue
			}
			contractIDList = append(contractIDList, c.ID)
			contractDayMap[c.ID] = "W"
		}
		percent = -200
	}
	if opt, ok := optionMap["chart-refresh"]; ok {
		forceRefresh = opt.BoolValue()
	}
	if opt, ok := optionMap["predictions-refresh"]; ok {
		forceRefresh = opt.BoolValue()
	}
	if opt, ok := optionMap["active-refresh"]; ok {
		forceRefresh = opt.BoolValue()
	}

	// Quick reply to buy us some time
	flags := discordgo.MessageFlagsIsComponentsV2
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

	userID := bottools.GetInteractionUserID(i)

	mobileFriendly := farmerstate.GetMiscSettingString(userID, "rerunMobileFriendly") == "true"
	if opt, ok := optionMap["chart-mobile-friendly"]; ok {
		mobileFriendly = opt.BoolValue()
		farmerstate.SetMiscSettingString(userID, "rerunMobileFriendly", strconv.FormatBool(mobileFriendly))
	} else if opt, ok := optionMap["predictions-mobile-friendly"]; ok {
		mobileFriendly = opt.BoolValue()
		farmerstate.SetMiscSettingString(userID, "rerunMobileFriendly", strconv.FormatBool(mobileFriendly))
	} else if opt, ok := optionMap["threshold-mobile-friendly"]; ok {
		mobileFriendly = opt.BoolValue()
		farmerstate.SetMiscSettingString(userID, "rerunMobileFriendly", strconv.FormatBool(mobileFriendly))
	} else if val := farmerstate.GetMiscSettingString(userID, "rerunMobileFriendly"); val != "" {
		mobileFriendly, _ = strconv.ParseBool(val)
	}

	// Do I know the user's IGN?
	farmerName := farmerstate.GetMiscSettingString(userID, "ei_ign")
	if farmerName == "" {
		backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, userID, okayToSave)
		if backup != nil {
			farmerName = backup.GetUserName()
			farmerstate.SetMiscSettingString(userID, "ei_ign", farmerName)
		}
	}
	archive, cached := ei.GetContractArchiveFromAPI(s, eggIncID, userID, forceRefresh, okayToSave)

	cxpVersion := ""

	var components []discordgo.MessageComponent
	if len(contractIDList) == 1 {
		components = printActiveContractDetails(userID, archive, contractIDList[0])
	} else {
		components = printContractChart(userID, archive, percent, page, contractIDList, contractDayMap, mobileFriendly)
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      flags,
		Components: components,
	})
	if err != nil {
		log.Println("Error sending follow-up message:", err)
	}

	if !cached && okayToSave {
		for _, c := range archive {
			eval := c.GetEvaluation()
			if eval != nil {
				cxpVersion = eval.GetVersion()
				// Replace all non-numeric characters in cxpVersion with underscores
				cxpVersion = strings.Map(func(r rune) rune {
					if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
						return r
					}
					return '_'
				}, cxpVersion)

				if cxpVersion != "cxp_v0_2_0" {
					log.Printf("CXP version is %s, not 0.2.0, cannot evaluate contracts\n", cxpVersion)
				}
				break
			}
		}
		jsonData, err := json.Marshal(archive)

		if err != nil {
			log.Println("Error marshalling archive to JSON:", err)
			return
		}

		discordID := userID
		fileName := fmt.Sprintf("ttbb-data/eiuserdata/archive-%s-%s.json", discordID, cxpVersion)
		// Replace eggIncID with userID in the JSON data
		jsonString := string(jsonData)
		jsonString = strings.ReplaceAll(jsonString, eggIncID, userID)
		jsonData = []byte(jsonString)
		err = os.WriteFile(fileName, jsonData, 0644)
		if err != nil {
			log.Println("Error saving contract archive to file:", err)
		}
	}
}
