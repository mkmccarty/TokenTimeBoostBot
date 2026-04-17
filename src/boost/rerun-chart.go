package boost

import (
	"fmt"
	"log"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func printContractChart(userID string, archive []*ei.LocalContract, percent int, page int, contractIDList []string, contractDayMap map[string]string) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent
	eiUserName := farmerstate.GetMiscSettingString(userID, "ei_ign")
	divider := true
	spacing := discordgo.SeparatorSpacingSizeSmall
	builder := strings.Builder{}
	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		components = append(components, &discordgo.TextDisplay{
			Content: "No archived contracts found in Egg Inc API response",
		})
		return components
	}
	log.Printf("Downloaded %d archived contracts from Egg Inc API for %s\n", len(archive), eiUserName)

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

	count := 0
	pagecount := 0

	for _, a := range archive {
		contractID := a.GetContract().GetIdentifier()
		if contractID == "first-contract" {
			continue
		}

		evaluation := a.GetEvaluation()
		evaluationCxp := evaluation.GetCxp()
		c := ei.EggIncContractsAll[contractID]

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
	}

	if builder.Len() > 0 {
		components = append(components, &discordgo.TextDisplay{
			Content: builder.String(),
		})
		builder.Reset()
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
