package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func printActiveContractDetails(userID string, archive []*ei.LocalContract, contractIDParam string) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent
	tvalFooterMessage := false
	eiUserName := farmerstate.GetMiscSettingString(userID, "ei_ign")
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	builder := strings.Builder{}

	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		components = append(components, &discordgo.TextDisplay{
			Content: "No archived contracts found in Egg Inc API response",
		})
		return components
	}

	count := 0

	for _, a := range archive {
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

		if contractID != contractIDParam {
			continue
		}
		if c.ContractVersion == 2 { //&& c.ValidUntil.Unix() > time.Now().Unix() {
			artifactIcons := ""
			teamworkIcons := []string{}
			deliveryTarget := c.TargetAmount[len(c.TargetAmount)-1] / float64(c.MaxCoopSize)

			log.Printf("Evaluating contract %s coop %s for user %s\n", contractID, coopID, eiUserName)
			if coopID != "[solo]" {
				coopStatus, _, _, err := ei.GetCoopStatusForCompletedContracts(contractID, a.GetCoopIdentifier(), eiID)
				if err == nil {
					builder.Reset()
					for _, contrib := range coopStatus.GetContributors() {
						// Check to see if the name matches the farmer name in the evaluation
						pp := float64(contrib.GetProductionParams().GetDelivered())
						myRatio := pp / deliveryTarget

						// If the player has changed their game name, then we may need to match on contribution ratio too
						if contrib.GetUserName() == eiUserName || myRatio == evaluation.GetContributionRatio() {
							for _, artifact := range contrib.GetFarmInfo().GetEquippedArtifacts() {
								spec := artifact.GetSpec()
								strType := levels[spec.GetLevel()] + rarity[spec.GetRarity()]
								artifactIcons += ei.GetBotEmojiMarkdown(fmt.Sprintf("%s%s", ei.ShortArtifactName[int32(spec.GetName())], strType))
							}
							for _, buff := range contrib.GetBuffHistory() {
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
			siabIcon := ei.GetBotEmojiMarkdown("SIAB_T4L")
			ggSiabStr := ""
			if ggicon != "" {
				ggString = fmt.Sprintf(" / %s %s %.0f", ggicon, bottools.FmtDuration(c.EstimatedDurationMaxGG), c.CxpMaxGG)
				if c.CxpMaxSiabGG > c.CxpMaxGG {
					ggSiabStr = fmt.Sprintf(" / %s %.0f (SR estimation) %s", siabIcon, c.CxpMaxSiabGG, bottools.FmtDuration(c.EstimatedDurationSIABGG))
				}
			}

			if c.CxpMaxSiab > c.CxpMax {
				fmt.Fprintf(&builder, "**CS:** %d  **Est CS:** %.0f (SR estimation)\n", uint32(evaluationCxp), c.CxpMax)
				fmt.Fprintf(&builder, "%s %.0f (SR estimation) %s%s%s\n", siabIcon, c.CxpMaxSiab, bottools.FmtDuration(c.EstimatedDurationSIAB), ggSiabStr, ggString)
			} else {
				fmt.Fprintf(&builder, "**CS:** %d  **Est CS:** %.0f%s (SR estimation)\n", uint32(evaluationCxp), c.CxpMax, ggString)
			}

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

			count++
		}
		break // Only one contract
	}

	if count == 0 {
		builder.Reset()
		builder.WriteString("No contracts met this condition.\n")
	} else {
		builder.WriteString("-# [brackets] indicate area for improvement.\n")
		builder.WriteString("-# Teamwork scoring artifacts shown after the value..\n")
		if tvalFooterMessage {
			builder.WriteString("-# Token Teamwork scores are 2/10 value sent and 8/10 ∆-value.\n")
		}
	}

	fmt.Fprintf(&builder, "-# Est duration/CS based on 1.0 fair share, 5%s boosts (w/50%sIHR), 6%s/hr rate and leggy artifacts.\n", ei.GetBotEmojiMarkdown("token"), ei.GetBotEmojiMarkdown("egg_truth"), ei.GetBotEmojiMarkdown("token"))

	if builder.Len() > 0 {
		components = append(components, &discordgo.TextDisplay{
			Content: builder.String(),
		})
	}
	return components
}
