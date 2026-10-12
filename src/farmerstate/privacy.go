package farmerstate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// GetSlashPrivacyCommand creates a new slash command for setting Egg, Inc name
func GetSlashPrivacyCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "Boost bot privacy information.",
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
		Options: []dc.Option{
			dc.IntOption{
				Name:        "enable-data-privacy",
				Description: "Change your data privacy setting.",
				Required:    false,
				Choices: []dc.Choice[int]{
					{Name: "Do not persist bot settings.", Value: 1},
					{Name: "Allow the bot to store some information.", Value: 0},
				},
			},
			dc.BoolOption{
				Name:        "get-settings-data",
				Description: "Retrieve a JSON file with your stored settings.",
				Required:    false,
			},
			dc.BoolOption{
				Name:        "remove-data",
				Description: "Remove my data from the boost bot database.",
				Required:    false,
			},
			dc.BoolOption{
				Name:        "confirm-request",
				Description: "Confirm privacy setting change or data removal.",
				Required:    false,
			},
			dc.StringOption{
				Name:         "alt",
				Description:  "Manage privacy settings or purge data for a linked alternate account.",
				Required:     false,
				Autocomplete: true,
			},
		},
	}
	return &command
}

// HandlePrivacyAutocomplete provides autocomplete suggestions for the alt option on /privacy.
func HandlePrivacyAutocomplete(e *dc.AutocompleteEvent) {
	name, value := e.FocusedOption()
	if name != "alt" {
		_ = e.RespondChoices(nil)
		return
	}

	callerID := e.UserID()
	alts := GetLinkedAltIDs(callerID)
	if len(alts) == 0 {
		_ = e.RespondChoices(nil)
		return
	}

	value = strings.ToLower(strings.TrimSpace(value))
	var choices []dc.Choice[string]
	for _, altID := range alts {
		ign := strings.TrimSpace(GetMiscSettingString(altID, "ei_ign"))
		if ign == "" {
			ign = strings.TrimSpace(GetEggIncName(altID))
		}
		displayName := altID
		if dc.IsSnowflake(altID) {
			if ign != "" {
				displayName = fmt.Sprintf("%s (%s)", ei.NormalizePlayerNameForDisplay(ign), altID)
			}
		} else {
			if ign != "" && !strings.EqualFold(altID, ign) {
				displayName = fmt.Sprintf("%s (%s)", altID, ei.NormalizePlayerNameForDisplay(ign))
			} else if ign != "" {
				displayName = ei.NormalizePlayerNameForDisplay(ign)
			}
		}

		if value != "" && !strings.Contains(strings.ToLower(displayName), value) && !strings.Contains(strings.ToLower(altID), value) {
			continue
		}

		if len(displayName) > 100 {
			displayName = displayName[:100]
		}
		choices = append(choices, dc.Choice[string]{
			Name:  displayName,
			Value: altID,
		})
		if len(choices) >= 25 {
			break
		}
	}
	_ = e.RespondChoices(choices)
}

// GetLinkedAltIDs returns all unique alternate account IDs linked to callerID.
func GetLinkedAltIDs(callerID string) []string {
	seen := make(map[string]bool)
	var result []string

	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || id == callerID || seen[id] {
			return
		}
		seen[id] = true
		result = append(result, id)
	}

	for _, id := range GetAltControllerByMiscString("AltController", callerID) {
		add(id)
	}
	for _, id := range GetAllActAsLinkedUsers(callerID) {
		add(id)
	}

	return result
}

func resolveLinkedAlt(callerID, altParam string) (string, bool) {
	altParam = strings.TrimSpace(altParam)
	if altParam == "" {
		return "", false
	}

	cleanParam := altParam
	if strings.HasPrefix(cleanParam, "<@") && strings.HasSuffix(cleanParam, ">") {
		cleanParam = strings.TrimPrefix(cleanParam, "<@")
		cleanParam = strings.TrimPrefix(cleanParam, "!")
		cleanParam = strings.TrimSuffix(cleanParam, ">")
		cleanParam = strings.TrimSpace(cleanParam)
	}

	linkedAlts := GetLinkedAltIDs(callerID)
	// 1. Direct or case-insensitive ID match
	for _, id := range linkedAlts {
		if id == cleanParam || strings.EqualFold(id, cleanParam) {
			return id, true
		}
	}

	// 2. Match on IGN, EggIncName, or composite display name
	for _, id := range linkedAlts {
		ign := strings.TrimSpace(GetMiscSettingString(id, "ei_ign"))
		eggName := strings.TrimSpace(GetEggIncName(id))
		if (ign != "" && strings.EqualFold(ign, cleanParam)) ||
			(eggName != "" && strings.EqualFold(eggName, cleanParam)) {
			return id, true
		}

		var dispName string
		if dc.IsSnowflake(id) {
			if ign != "" {
				dispName = fmt.Sprintf("%s (%s)", ei.NormalizePlayerNameForDisplay(ign), id)
			}
		} else {
			if ign != "" && !strings.EqualFold(id, ign) {
				dispName = fmt.Sprintf("%s (%s)", id, ei.NormalizePlayerNameForDisplay(ign))
			} else if ign != "" {
				dispName = ei.NormalizePlayerNameForDisplay(ign)
			}
		}
		if dispName != "" && strings.EqualFold(dispName, cleanParam) {
			return id, true
		}
	}

	return "", false
}

// PurgeAltLinks removes all act-as links, switches, and alt-controller associations for altUserID.
func PurgeAltLinks(mainUserID, altUserID string) {
	_ = RemoveActAsLink(mainUserID, altUserID)
	_ = DeleteAllActAsLinks(altUserID)
	SetMiscSettingString(altUserID, "AltController", "")
	for _, a := range GetAltControllerByMiscString("AltController", altUserID) {
		SetMiscSettingString(a, "AltController", "")
	}
	ClearPendingActAsRegistration(mainUserID)
	ClearPendingActAsRegistration(altUserID)

	ephemeralSwitchesMutex.Lock()
	for msgID, info := range ephemeralMessageSwitches {
		if info.MainUserID == altUserID || info.AltUserID == altUserID ||
			(info.MainUserID == mainUserID && info.AltUserID == altUserID) {
			delete(ephemeralMessageSwitches, msgID)
		}
	}
	ephemeralSwitchesMutex.Unlock()
}

func purgeUserDataFiles(userID string) {
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
}

// HandlePrivacy will handle the /privacy command through the dc facade.
func HandlePrivacy(e *dc.CommandEvent) {
	callerID := e.UserID()
	targetUserID := callerID
	isAlt := false

	_ = e.Defer(true)

	if altParam, ok := e.OptString("alt"); ok && strings.TrimSpace(altParam) != "" {
		altParam = strings.TrimSpace(altParam)
		resolvedID, isLinked := resolveLinkedAlt(callerID, altParam)
		if !isLinked {
			_ = e.Followup(dc.Message{
				Content: fmt.Sprintf("Account \"%s\" is not linked to you as an alternate account.", altParam),
			})
			return
		}
		targetUserID = resolvedID
		isAlt = true
	}

	var builder strings.Builder

	if isAlt {
		if dc.IsSnowflake(targetUserID) {
			builder.WriteString("# Privacy information for alternate account: <@")
			builder.WriteString(targetUserID)
			builder.WriteString(">\n")
		} else {
			builder.WriteString("# Privacy information for alternate account: ")
			builder.WriteString(targetUserID)
			builder.WriteString("\n")
		}
		builder.WriteString("Boost Bot stores some information about alternate account usage to provide you with a better experience.\n")
		builder.WriteString("The alternate account ID is used as a key to this saved information.\n")
		builder.WriteString("**This information is never sold.** It will only be shared with other Boost Bot developers or testers within the Bot's development Discord server.\n")
		builder.WriteString("You can view this information at any time using the **get-settings-data** option.\n")
		builder.WriteString("\n")
	} else {
		builder.WriteString("# Privacy information for user: <@")
		builder.WriteString(callerID)
		builder.WriteString(">\n")
		builder.WriteString("Boost Bot stores some information about your usage to provide you with a better experience and to improve the bot.\n")
		builder.WriteString("Your Discord User ID is used as a key to this saved information.\n")
		builder.WriteString("**This information is never sold.** It will only be shared with other Boost Bot developers or testers within the Bot's development Discord server.\n")
		builder.WriteString("You can view this information at any time using the **get-settings-data** option.\n")
		builder.WriteString("\n")
	}

	var filename string
	var reader *bytes.Reader

	userPrivacy := getDataPrivacy(targetUserID)
	removeData := false
	confirmOption := false

	// User must confirm data removal and/or privacy setting change
	if opt, ok := e.OptBool("confirm-request"); ok {
		confirmOption = opt
	}

	if opt, ok := e.OptInt("enable-data-privacy"); ok {
		userPrivacy = opt == 1
		if userPrivacy && confirmOption {
			if isAlt {
				builder.WriteString("Boost Bot will no longer store any persistent data about this alternate account. ")
				builder.WriteString("If you wish to store data again, you will need to re-enable it. ")
				builder.WriteString("If it interacts with the bot for contracts and token tracking it will be stored temporarily and removed within a week of the last interaction of a contract or tracker.\n")
			} else {
				builder.WriteString("Boost Bot wil no longer store any persistent data about you.")
				builder.WriteString("If you wish to store data again, you will need to re-enable it.")
				builder.WriteString("If you interact with the bot for contracts and token tracking it will be stored temporarily and removed within a week of the last interaction of a contract or tracker.")
			}
			removeData = true
		} else if userPrivacy && !confirmOption {
			builder.WriteString("You have not confirmed the privacy setting change, use the **confirm-request** option.")
		} else {
			setDataPrivacy(targetUserID, userPrivacy)
			if isAlt {
				builder.WriteString("Boost Bot will save a small amount of data about this alternate account. You can download this data at any time.")
			} else {
				builder.WriteString("Boost Bot will store save a small amount of data about you. You can download this data at any time.")
			}
		}
	}

	if opt, ok := e.OptBool("remove-data"); ok {
		removeData = opt
		if removeData && !confirmOption {
			builder.WriteString("You have not confirmed your data removal, use the **confirm-request** option.")
			removeData = false
		}
	}

	if removeData {
		if isAlt {
			builder.WriteString("Alternate account settings data and alt links have been removed from the Boost Bot database.\n")
			builder.WriteString("A default set of settings is now used along with the preference not to store data.\n")

			purgeUserDataFiles(targetUserID)
			DeleteFarmer(targetUserID)
			PurgeAltLinks(callerID, targetUserID)
			setDataPrivacy(targetUserID, userPrivacy)
		} else {
			linkedAlts := GetLinkedAltIDs(callerID)
			for _, altID := range linkedAlts {
				purgeUserDataFiles(altID)
				DeleteFarmer(altID)
				PurgeAltLinks(callerID, altID)
				setDataPrivacy(altID, userPrivacy)
			}

			purgeUserDataFiles(callerID)
			DeleteFarmer(callerID)
			_ = DeleteAllActAsLinks(callerID)
			setDataPrivacy(callerID, userPrivacy)

			if len(linkedAlts) > 0 {
				builder.WriteString("Your settings data and all linked alternate accounts' data have been removed from the Boost Bot database.\n")
			} else {
				builder.WriteString("Your settings data have been removed from the Boost Bot database.\n")
			}
			builder.WriteString("A default set of settings is now used along with your preference not to store data.\n")
		}
	}

	if opt, ok := e.OptBool("get-settings-data"); ok {
		// Return the users settings data in a JSON file to the user
		getData := opt
		if getData {
			userData := GetFullUserData(targetUserID)

			filename = "boostbot-data-" + targetUserID + ".json"
			buf := &bytes.Buffer{}
			jsonData, err := json.Marshal(userData)
			if err != nil {
				log.Println(err.Error())
				builder.WriteString("Error formatting JSON data. ")
				builder.WriteString(err.Error())
			} else {
				err = json.Indent(buf, jsonData, "", "  ")
				if err != nil {
					builder.WriteString("Error formatting JSON data. ")
					builder.WriteString(err.Error())
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
		if isAlt {
			builder.WriteString("Alternate account settings data has been saved to a JSON file. You can view and download it.")
		} else {
			builder.WriteString("Your settings data has been saved to a JSON file. You can view and download it.")
		}
		_ = e.Followup(dc.Message{
			Content: builder.String(),
			Files:   []dc.File{{Name: filename, Reader: reader}},
		})
	} else {
		_ = e.Followup(dc.Message{Content: builder.String()})
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
	ActAsLinks            []string                             `json:"act_as_links,omitempty"`
	ActAsMains            []string                             `json:"act_as_mains,omitempty"`
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
	if links := GetActAsLinks(userID); len(links) > 0 {
		data.ActAsLinks = links
	}
	if mains := GetActAsMains(userID); len(mains) > 0 {
		data.ActAsMains = mains
	}
	return data
}
