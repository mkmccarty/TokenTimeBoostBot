package boost

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

// GetSlashReplayEvalCommand returns the command for the /launch-helper command
func GetSlashReplayEvalCommand(cmd string) *discordgo.ApplicationCommand {
	minValue := 0.0
	minValueTwo := 2.0
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
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "page",
						Description: "Provide a page number to see additional results",
						MinValue:    &minValueTwo,
						MaxValue:    10,
						Required:    false,
					},
				},
			},
		},
	}
}

// HandleReplayEval handles the /replay-eval command
func HandleReplayEval(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)

	optionMap := bottools.GetCommandOptionsMap(i)
	if opt, ok := optionMap["reset"]; ok {
		if opt.BoolValue() {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
		}
	}
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	ReplayEval(s, i, optionMap, eiID, true)
}

// ReplayEval evaluates the contract history and provides replay guidance
func ReplayEval(s *discordgo.Session, i *discordgo.InteractionCreate, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption, eiID string, okayToSave bool) {
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
	if opt, ok := optionMap["threshold-page"]; ok {
		v := int(opt.UintValue())
		if v < 1 {
			v = 1
		}
		page = v
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

	components := printArchivedContracts(userID, archive, percent, page, contractIDList, contractDayMap)
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

func printArchivedContracts(userID string, archive []*ei.LocalContract, percent int, page int, contractIDList []string, contractDayMap map[string]string) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent
	tvalFooterMessage := false
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
	} else if len(contractIDList) > 1 {
		builder.WriteString("## Displaying contract scores for future predictions:\n")
	} else {
		fmt.Fprintf(&builder, "## Displaying contract scores less than %d%% of speedrun potential:\n", percent)
	}
	components = append(components, &discordgo.TextDisplay{
		Content: builder.String(),
	})
	components = append(components, &discordgo.Separator{
		Divider: &divider,
		Spacing: &spacing,
	})
	builder.Reset()

	contractIDParam := ""
	if len(contractIDList) == 1 {
		contractIDParam = contractIDList[0]
	}

	if len(contractIDList) != 1 {
		if len(contractDayMap) > 0 {
			fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s %3s`\n",
				bottools.AlignString("CONTRACT-ID", 30, bottools.StringAlignCenter),
				bottools.AlignString("CS", 6, bottools.StringAlignCenter),
				bottools.AlignString("HIGH", 6, bottools.StringAlignCenter),
				bottools.AlignString("GAP", 6, bottools.StringAlignRight),
				bottools.AlignString("%", 4, bottools.StringAlignCenter),
				bottools.AlignString("Day", 6, bottools.StringAlignCenter),
			)
		} else {
			fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s`\n",
				bottools.AlignString("CONTRACT-ID", 30, bottools.StringAlignCenter),
				bottools.AlignString("CS", 6, bottools.StringAlignCenter),
				bottools.AlignString("HIGH", 6, bottools.StringAlignCenter),
				bottools.AlignString("GAP", 6, bottools.StringAlignRight),
				bottools.AlignString("%", 4, bottools.StringAlignCenter),
			)
		}
	}

	count := 0
	pagecount := 0

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

		evaluation := a.GetEvaluation()
		coopID := evaluation.GetCoopIdentifier()
		evaluationCxp := evaluation.GetCxp()
		c := ei.EggIncContractsAll[contractID]
		//if c.ContractVersion == 2 {
		if contractIDParam == "" {
			if len(contractIDList) > 0 {
				if !slices.Contains(contractIDList, contractID) {
					continue
				}
			}

			if c.ContractVersion == 2 && (percent != -1 || (c.ValidUntil.Unix() > time.Now().Unix())) {
				// Need to download the coop_status for more details
				evalPercent := evaluationCxp / c.CxpMax * 100.0
				if percent == -1 || (evalPercent < float64(100-percent)) {
					if builder.Len() > 3600 && page > 0 {
						builder.Reset()
						pagecount = 0
						page--
					}

					if builder.Len() < 3600 {
						if len(contractDayMap) > 0 {
							dayLabel := contractDayMap[contractID]
							fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s %3s`\n",
								bottools.AlignString(contractID, 30, bottools.StringAlignLeft),
								bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(evaluationCxp))), 6, bottools.StringAlignRight),
								bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(c.CxpMax))), 6, bottools.StringAlignRight),
								bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(c.CxpMax-evaluationCxp))), 6, bottools.StringAlignRight),
								bottools.AlignString(fmt.Sprintf("%.1f", (evaluationCxp/c.CxpMax)*100), 4, bottools.StringAlignCenter),
								bottools.AlignString(dayLabel, 6, bottools.StringAlignCenter))
						} else {
							fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s` <t:%d:R>\n",
								bottools.AlignString(contractID, 30, bottools.StringAlignLeft),
								bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(evaluationCxp))), 6, bottools.StringAlignRight),
								bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(c.CxpMax))), 6, bottools.StringAlignRight),
								bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(c.CxpMax-evaluationCxp))), 6, bottools.StringAlignRight),
								bottools.AlignString(fmt.Sprintf("%.1f", (evaluationCxp/c.CxpMax)*100), 4, bottools.StringAlignCenter),
								c.ValidUntil.Unix())
						}
					}
					count++
					pagecount++
				}
			}
		} else {
			if contractID != contractIDParam {
				continue
			}
			if c.ContractVersion == 2 { //&& c.ValidUntil.Unix() > time.Now().Unix() {
				artifactIcons := ""
				teamworkIcons := []string{}
				deliveryTarget := c.TargetAmount[len(c.TargetAmount)-1] / float64(c.MaxCoopSize)

				log.Printf("Evaluating contract %s coop %s for user %s\n", contractID, coopID, eiUserName)
				if coopID != "[solo]" {
					coopStatus, _, _, err := ei.GetCoopStatusForCompletedContracts(contractID, a.GetCoopIdentifier())
					if err == nil {
						builder.Reset()
						for _, c := range coopStatus.GetContributors() {
							// Check to see if the name matches the farmer name in the evaluation
							pp := float64(c.GetProductionParams().GetDelivered())
							myRatio := pp / deliveryTarget

							// If the player has changed their game name, then we may need to match on contribution ratio too
							if c.GetUserName() == eiUserName || myRatio == evaluation.GetContributionRatio() {
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
				}

				duration := time.Duration(evaluation.GetCompletionTime() * float64(time.Second))
				startTime := time.Unix(int64(evaluation.GetEvaluationStartTime())-int64(duration.Seconds()), 0)
				ggIcon := ""
				ggEvent := ei.FindGiftEvent(startTime)
				if ggEvent.EventType != "" {
					if ggEvent.Ultra {
						ggIcon = ei.GetBotEmojiMarkdown("ultra_gg")
					} else {
						ggIcon = ei.GetBotEmojiMarkdown("std_gg")
					}
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
				}
				if c.ChickenRuns > c.MaxCoopSize-1 {
					crCheck += "🤡"
				}
				// Token Evaluation
				tvalTarget := GetTargetTval(c.SeasonalScoring, evaluation.GetCompletionTime()/60, float64(c.MinutesPerToken))
				evalTval := evaluation.GetGiftTokenValueSent() - evaluation.GetGiftTokenValueReceived()
				tokCheck := "✅"
				if evalTval < tvalTarget {
					// Show Token Teamwork score vs max.
					myTeamwork := calculateTokenTeamwork(evaluation.GetCompletionTime(), c.MinutesPerToken, evaluation.GetGiftTokenValueSent(), evaluation.GetGiftTokenValueReceived())
					maxTeamwork := calculateTokenTeamwork(evaluation.GetCompletionTime(), c.MinutesPerToken, 1000, 8)
					// Maybe a stoken sync.
					tokenSink := ""
					if myTeamwork <= 2.0 &&
						(evaluation.GetGiftTokensSent() > uint32(4*(c.MaxCoopSize-1)) ||
							evaluation.GetGiftTokensReceived() > 12.0) {
						tokenSink = ei.GetBotEmojiMarkdown("tvalrip")
					}

					tokCheck = fmt.Sprintf("⚠️%s[%.3g/%.3g]", tokenSink, myTeamwork, maxTeamwork)
					tvalFooterMessage = true
				}
				// Duration Check
				//if evaluation.GetCompletionTime() > c.EstimatedDuration.Seconds()*1.10 {
				completionTime := int64(evaluation.GetLastContributionTime())
				if completionTime == 0 {
					completionTime = int64(evaluation.GetEvaluationStartTime())
				}
				fmt.Fprintf(&builder, "**Started:** <t:%d:f> %s\n", startTime.Unix(), ggIcon)
				fmt.Fprintf(&builder, "**Completed:** <t:%d:f>\n", completionTime)
				fmt.Fprintf(&builder, "**Duration:** %s  **Est. Duration:** %s\n", bottools.FmtDuration(time.Duration(evaluation.GetCompletionTime()*float64(time.Second))), bottools.FmtDuration(c.EstimatedDurationMax))
				ggString := ""
				ggicon := ""
				gg, ugg, _ := ei.GetGenerousGiftEvent()
				if gg > 1.0 {
					ggicon = " " + ei.GetBotEmojiMarkdown("std_gg")
				}
				if ugg > 1.0 {
					// farmers with ultra
					//gg = ugg + (float64(contract.UltraCount) / float64(contract.CoopSize))
					ggicon = " " + ei.GetBotEmojiMarkdown("ultra_gg")
				}
				if ggicon != "" {
					ggString = fmt.Sprintf(" / %s %.0f ", ggicon, c.CxpMaxGG)
				}

				fmt.Fprintf(&builder, "**CS:** %d  **Est CS:** %.0f %s(SR estimation)\n", uint32(evaluationCxp), c.CxpMax, ggString)

				if c.SeasonalScoring == ei.SeasonalScoringNerfed {
					fmt.Fprintf(&builder, "**Contrib:** %s  **CR:** %s\n", contribCheck, crCheck)
				} else {
					fmt.Fprintf(&builder, "**Contrib:** %s **TVal**: %s  **CR:** %s\n", contribCheck, tokCheck, crCheck)
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

	if percent != -1 && builder.Len() > 3600 {
		builder.WriteString("Response truncated, too many contracts meet this condition, use the page parameter to see the other pages.\n")
	}

	if percent != -1 {
		fmt.Fprintf(&builder, "Showing %d of your %d contracts that met this condition.\n", pagecount, count)
		builder.WriteString("-# The order is based in your contract archive order.\n")
	}
	if count == 0 {
		builder.Reset()
		builder.WriteString("No contracts met this condition.\n")
	} else if contractIDParam != "" {
		builder.WriteString("-# [brackets] indicate area for improvement.\n")
		builder.WriteString("-# Teamwork scoring artifacts shown after the value..\n")
		if tvalFooterMessage {
			builder.WriteString("-# Token Teamwork scores are 2/10 value sent and 8/10 ∆-value.\n")
		}
	}
	if len(contractDayMap) > 0 {
		fmt.Fprintf(&builder, "-# Predicted contract days: W=Wednesday, F=Friday, U=Friday%s\n", ei.GetBotEmojiMarkdown("ultra"))
	}
	fmt.Fprintf(&builder, "-# Est duration/CS based on 1.0 fair share, 5%s boosts (w/50%sIHR), 6%s/hr rate and leggy artifacts.\n", ei.GetBotEmojiMarkdown("token"), ei.GetBotEmojiMarkdown("egg_truth"), ei.GetBotEmojiMarkdown("token"))

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
