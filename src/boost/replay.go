package boost

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

// SavedOptionsMap stores the options from a slash command for later retrieval
var SavedOptionsMap = make(map[string]map[string]*discordgo.ApplicationCommandInteractionDataOption)

// SaveOptions saves the options from a slash command into a map for later retrieval
func SaveOptions(options []*discordgo.ApplicationCommandInteractionDataOption) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		if opt.Type == discordgo.ApplicationCommandOptionSubCommand {
			for _, subOpt := range opt.Options {
				optionMap[opt.Name+"-"+subOpt.Name] = subOpt
			}
			optionMap[opt.Name] = opt
		}
	}
	return optionMap
}

// GetSlashReplayEvalCommand returns the command for the /launch-helper command
func GetSlashReplayEvalCommand(cmd string) *discordgo.ApplicationCommand {
	//minValue := 0.0
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Evaluate contract history and provide replay guidance.",
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
				},
			},
			/*
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
					},
				},
			*/
		},
	}
}

// HandleReplayEval handles the /replay-eval command
func HandleReplayEval(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	onlyActiveContracts := false
	percent := -1
	contractID := ""

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		if opt.Type == discordgo.ApplicationCommandOptionSubCommand {
			for _, subOpt := range opt.Options {
				optionMap[opt.Name+"-"+subOpt.Name] = subOpt
			}
			optionMap[opt.Name] = opt
		}
	}

	if _, ok := optionMap["active"]; ok {
		// No parameters on this
		onlyActiveContracts = true
	}
	if opt, ok := optionMap["threshold-percent"]; ok {

		percent = int(opt.UintValue())
	}
	if opt, ok := optionMap["active-contract-id"]; ok {
		contractID = opt.StringValue()
	}

	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")

	if onlyActiveContracts {
		ReplayEval(s, i, -1, eiID, contractID, false)
	} else {
		ReplayEval(s, i, percent, eiID, contractID, true)
	}
}

// ReplayEval evaluates the contract history and provides replay guidance
func ReplayEval(s *discordgo.Session, i *discordgo.InteractionCreate, percent int, eiID string, contractID string, okayToSave bool) {
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
		RequestEggIncIDModal(s, i, fmt.Sprintf("replay#%d#%s", percent, contractID))
		return
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
	// Do I know the user's IGN?
	farmerName := farmerstate.GetMiscSettingString(userID, "ei_ign")
	if farmerName == "" {
		backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, userID, okayToSave)
		if backup != nil {
			farmerName = backup.GetUserName()
			farmerstate.SetMiscSettingString(userID, "ei_ign", farmerName)
		}
	}
	archive, cached := ei.GetContractArchiveFromAPI(s, eggIncID, userID, okayToSave)

	cxpVersion := ""
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

	components := printArchivedContracts(userID, archive, percent, contractID)
	if len(components) == 0 {
		components = []discordgo.MessageComponent{
			&discordgo.TextDisplay{Content: "No archived contracts found in Egg Inc API response"},
		}
	}
	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      flags,
		Components: components,
	})
	if err != nil {
		log.Println("Error sending follow-up message:", err)
	}

	if !cached && okayToSave {
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

func printArchivedContracts(userID string, archive []*ei.LocalContract, percent int, contractIDParam string) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent
	eiUserName := farmerstate.GetMiscSettingString(userID, "ei_ign")
	divider := true
	spacing := discordgo.SeparatorSpacingSizeSmall
	builder := strings.Builder{}
	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		components = append(components, &discordgo.TextDisplay{
			Content: builder.String(),
		})
		return components
	}
	log.Printf("Downloaded %d archived contracts from Egg Inc API\n", len(archive))

	// Want a preamble string for builder for what we're displaying
	if percent == -1 {
		builder.WriteString("## Contract CS eval of active contracts\n")
	} else {
		builder.WriteString(fmt.Sprintf("## Displaying contract scores less than %d%% of speedrun potential:\n", percent))
	}
	components = append(components, &discordgo.TextDisplay{
		Content: builder.String(),
	})
	components = append(components, &discordgo.Separator{
		Divider: &divider,
		Spacing: &spacing,
	})
	builder.Reset()

	/*
		fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s`\n",
			bottools.AlignString("CONTRACT-ID", 30, bottools.StringAlignCenter),
			bottools.AlignString("CS", 6, bottools.StringAlignCenter),
			bottools.AlignString("HIGH", 6, bottools.StringAlignCenter),
			bottools.AlignString("GAP", 6, bottools.StringAlignRight),
			bottools.AlignString("%", 4, bottools.StringAlignCenter),
		)
	*/

	count := 0
	for _, a := range archive {
		// Completed
		// What is the current tval
		// CR Total
		// Contribution Ratio
		// BuffTimeValue
		// Duration
		// Artifact use
		levels := []string{"T1", "T2", "T3", "T4", "T5"}
		rarity := []string{"C", "R", "E", "L"}

		siabmap := map[float64]string{
			1.2: "T1C",
			1.3: "T2C",
			1.5: "T3C", 1.6: "T3R",
			1.7: "T4C", 1.8: "T4R", 1.9: "T4E", 2.0: "T4L",
		}
		deflmap := map[float64]string{
			1.05: "T1C",
			1.08: "T2C",
			1.12: "T3C", 1.13: "T3R",
			1.15: "T4C", 1.17: "T4R", 1.19: "T4E", 1.20: "T4L",
		}

		contractID := a.GetContract().GetIdentifier()
		if contractID == "first-contract" {
			continue
		}

		coopID := a.GetCoopIdentifier()
		evaluation := a.GetEvaluation()
		evaluationCxp := evaluation.GetCxp()
		c := ei.EggIncContractsAll[contractID]
		//if c.ContractVersion == 2 {
		if percent != -1 {
			/*
				if cxp < c.Cxp*(1-float64(percent)/100) || c.Cxp == 0 {
					// Need to download the coop_status for more details
					aResp, , _, err := ei.GetCoopStatusForRerun(contractID, a.GetCoopIdentifier())
					if err == nil {
						// if aResp.GetStatus() == 3 {
					}
					if builder.Len() < 3500 {
						fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s`\n",
							bottools.AlignString(contractID, 30, bottools.StringAlignLeft),
							bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(cxp))), 6, bottools.StringAlignRight),
							bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(c.Cxp))), 6, bottools.StringAlignRight),
							bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(c.Cxp-cxp))), 6, bottools.StringAlignRight),
							bottools.AlignString(fmt.Sprintf("%.1f", (cxp/c.Cxp)*100), 4, bottools.StringAlignCenter))
					}
					count++
				}*/
		} else {
			if contractID != contractIDParam {
				continue
			}
			if c.ContractVersion == 2 && c.ExpirationTime.Unix() > time.Now().Unix() {
				artifactIcons := ""
				teamworkIcons := []string{}
				log.Printf("Evaluating contract %s coop %s for user %s\n", contractID, coopID, eiUserName)
				coopStatus, _, _, err := ei.GetCoopStatusForRerun(contractID, a.GetCoopIdentifier())
				if err == nil {
					builder.Reset()
					for _, c := range coopStatus.GetContributors() {
						// Check to see if the name matches the farmer name in the evaluation
						if c.GetUserName() == eiUserName {
							for _, artifact := range c.GetFarmInfo().GetEquippedArtifacts() {
								spec := artifact.GetSpec()
								strType := levels[spec.GetLevel()] + rarity[spec.GetRarity()]
								artifactIcons += ei.GetBotEmojiMarkdown(fmt.Sprintf("%s%s", ei.ShortArtifactName[int32(spec.GetName())], strType))
							}
							for _, buff := range c.GetBuffHistory() {
								siab := buff.GetEarnings()
								defl := buff.GetEggLayingRate()
								if siab > 1.0 {
									teamworkIcons = append(teamworkIcons, ei.GetBotEmojiMarkdown(fmt.Sprintf("SIAB_%s", siabmap[siab])))
								}
								if defl > 1.0 {
									teamworkIcons = append(teamworkIcons, ei.GetBotEmojiMarkdown(fmt.Sprintf("DEFL_%s", deflmap[defl])))
								}
							}
							// make teamworkIcons unique and sort them in alpha order
							uniqueIcons := make(map[string]struct{})
							for _, icon := range teamworkIcons {
								uniqueIcons[icon] = struct{}{}
							}
							teamworkIcons = teamworkIcons[:0]
							for icon := range uniqueIcons {
								teamworkIcons = append(teamworkIcons, icon)
							}
							// sort in alpha order
							sort.Strings(teamworkIcons)
							break
						}
					}
				} else {
					log.Println("Error getting coop status for contract:", err)
				}

				eggImg := FindEggEmoji(c.EggName)
				fmt.Fprintf(&builder, "## %s %dp [%s](%s/%s/%s)\n", eggImg, c.MaxCoopSize, contractID, "https://eicoop-carpet.netlify.app", contractID, coopID)
				// Check marks Contribution near 1 & CR & TVal
				contribCheck := "✅"
				if evaluation.GetContributionRatio() < 0.90 {
					contribCheck = fmt.Sprintf("[%.3g]", evaluation.GetContributionRatio())
				}
				// Chicken Runs
				crCheck := "✅"
				if evaluation.GetChickenRunsSent() < uint32(c.ChickenRuns) {
					crCheck = fmt.Sprintf("[%d/%d]", evaluation.GetChickenRunsSent(), c.ChickenRuns)
					if c.ChickenRuns > c.MaxCoopSize-1 {
						crCheck += "🤡"
					}
				}
				// Token Evaluation
				tvalTarget := GetTargetTval(c.SeasonalScoring, evaluation.GetCompletionTime()/60, float64(c.MinutesPerToken))
				evalTval := evaluation.GetGiftTokenValueSent() - evaluation.GetGiftTokenValueReceived()
				tokCheck := "✅"
				if evalTval < tvalTarget {
					// Show Token Teamwork score vs max.
					myTeamwork := calculateTokenTeamwork(evaluation.GetCompletionTime(), c.MinutesPerToken, evaluation.GetGiftTokenValueSent(), evaluation.GetGiftTokenValueReceived())
					maxTeamwork := calculateTokenTeamwork(evaluation.GetCompletionTime(), c.MinutesPerToken, 1000, 8)

					tokCheck = fmt.Sprintf("[%.3g/%.3g]", myTeamwork, maxTeamwork)
				}
				// Duration Check
				//if evaluation.GetCompletionTime() > c.EstimatedDuration.Seconds()*1.10 {
				fmt.Fprintf(&builder, "**Completed:** <t:%d:f>\n", int64(evaluation.GetLastContributionTime()))
				fmt.Fprintf(&builder, "**Duration:** %s  **Est. Duration:** %s\n", bottools.FmtDuration(time.Duration(evaluation.GetCompletionTime()*float64(time.Second))), bottools.FmtDuration(c.EstimatedDuration))
				fmt.Fprintf(&builder, "**CS:** %d  **Est Max CS:** %.0f\n", uint32(evaluationCxp), c.Cxp)
				if c.SeasonalScoring == 1 {
					fmt.Fprintf(&builder, "**Contrib:** %s  **CR:** %s\n", contribCheck, crCheck)
				} else {
					fmt.Fprintf(&builder, "**Contrib:** %s **Tval**: %s  **CR:** %s\n", contribCheck, tokCheck, crCheck)
				}
				fmt.Fprintf(&builder, "%s  **Teamwork:** %.3f  %s\n", artifactIcons, evaluation.GetTeamworkScore(), strings.Join(teamworkIcons, ""))

				builder.WriteString("\n\n")
				components = append(components, &discordgo.TextDisplay{
					Content: builder.String(),
				})
				builder.Reset()

				/*
					components = append(components, &discordgo.Separator{
						Divider: &divider,
						Spacing: &spacing,
					})
				*/
				count++
			}
		}
	}

	if percent != -1 {
		builder.WriteString(fmt.Sprintf("%d of %d contracts met this condition.\n", count, len(archive)))
	}
	if count == 0 {
		builder.Reset()
		builder.WriteString("No contracts met this condition.\n")
	} else {
		builder.WriteString("-# [brackets] indicate area for improvement.\n")
		builder.WriteString("-# 🤡 indicates alt-parade needed to hit CR target.\n")
		builder.WriteString("-# Teamwork scoring artifacts shown after the value..\n")
	}

	if builder.Len() > 0 {
		components = append(components, &discordgo.TextDisplay{
			Content: builder.String(),
		})
	}
	return components
}

/*
{
	"evaluation": {
		"contract_identifier": "birthday-cake-2023",
		"coop_identifier": "happy-token",
		"cxp": 24702,
		"old_league": 0,
		"grade": 0,
		"contribution_ratio": 5.815095492301126,
		"completion_percent": 1,
		"original_length": 432000,
		"coop_size": 10,
		"solo": false,
		"soul_power": 30.02439174202951,
		"last_contribution_time": 1680055626.437586,
		"completion_time": 91932.26965808868,
		"chicken_runs_sent": 5,
		"gift_tokens_sent": 7,
		"gift_tokens_received": 0,
		"gift_token_value_sent": 0.7000000000000001,
		"gift_token_value_received": 0,
		"boost_token_allotment": 25,
		"buff_time_value": 38309.730632150175,
		"teamwork_score": 0.31141672867206993,
		"counted_in_season": false,
		"season_id": "",
		"time_cheats": 0,
		"version": "cxp-v0.2.0",
		"evaluation_start_time": 1696778185.855627,
		"status": 3
	}
}
*/
