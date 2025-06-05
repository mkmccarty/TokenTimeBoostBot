package boost

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

// HandleMenuReactions handles the menu reactions for the contract
func HandleMenuReactions(s *discordgo.Session, i *discordgo.InteractionCreate) {

	//userID := getInteractionUserID(i)

	data := i.MessageComponentData()
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")
	contractHash := reaction[len(reaction)-1]
	contract := Contracts[contractHash]
	// menu # HASH
	values := data.Values
	if len(values) == 0 || contract == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Content:    "",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})
		return
	}

	cmd := strings.Split(values[0], ":")

	switch cmd[0] {
	case "tools":
		var outputStrBuilder strings.Builder
		outputStrBuilder.WriteString("## Boost Tools\n")
		outputStrBuilder.WriteString(fmt.Sprintf("> **Boost Bot:** %s %s %s\n", bottools.GetFormattedCommand("stones"), bottools.GetFormattedCommand("calc-contract-tval"), bottools.GetFormattedCommand("coop-tval")))
		outputStrBuilder.WriteString("> **Wonky:** </auditcoop:1231383614701174814> </optimizestones:1235003878886342707> </srtracker:1158969351702069328>\n")
		outputStrBuilder.WriteString(fmt.Sprintf("> **Web:** \n> * [%s](%s)\n> * [%s](%s)\n",
			"Staabmia Stone Calc", "https://srsandbox-staabmia.netlify.app/stone-calc",
			"Kaylier Coop Laying Assistant", "https://ei-coop-assistant.netlify.app/laying-set"))
		outputStr := outputStrBuilder.String()
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: outputStr,
				Flags:   discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsSuppressEmbeds,
			},
		})
	case "xpost":
		var outputStrBuilder strings.Builder

		// ecoopad easter-2020-refill act
		outputStrBuilder.WriteString("\\## X-Post\n")
		outputStrBuilder.WriteString("\\# When you join:\n")
		outputStrBuilder.WriteString("\\* Equip Deflector.\n")
		outputStrBuilder.WriteString("\\* State the number of tokens needed to boost with.\n")
		outputStrBuilder.WriteString("\\* Boost.\n")
		outputStrBuilder.WriteString(fmt.Sprintf("\necoopad %s %s\n", contract.ContractID, contract.CoopID))

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "",
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "X-Post",
						Description: outputStrBuilder.String(),
						Color:       0x00cc00,
					},
				},
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})
	case "tlog":
		field := []*discordgo.MessageEmbedField{}
		var embed []*discordgo.MessageEmbed

		var logs []string
		for _, line := range contract.TokenLog {
			boostStr := ""
			if line.Boost {
				boostStr = " 🚀"
			}
			logs = append(logs, fmt.Sprintf("`%v %s %d->%s %s`", line.Time.Sub(contract.StartTime).Round(time.Second), line.FromNick, line.Quantity, boostStr, line.ToNick))
		}

		// Trin logs to the last 30 lines
		if len(logs) > 30 {
			logs = logs[len(logs)-30:]
		}

		var currentField strings.Builder
		for _, line := range logs {
			if currentField.Len()+len(line)+1 > 950 { // +1 for the newline character
				field = append(field, &discordgo.MessageEmbedField{
					Name:  "",
					Value: currentField.String(),
				})
				currentField.Reset()
			}
			currentField.WriteString(line + "\n")
		}
		if currentField.Len() > 0 {
			field = append(field, &discordgo.MessageEmbedField{
				Name:  "",
				Value: currentField.String(),
			})
		}
		embed = []*discordgo.MessageEmbed{{
			Type:        discordgo.EmbedTypeRich,
			Title:       "Token Log",
			Description: "",
			Fields:      field,
		}}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "",
				Embeds:  embed,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

	case "time":
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Updating boost list with estimated time...",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		contract.EstimateUpdateTime = time.Now()
		go updateEstimatedTime(s, i.ChannelID, contract, false)
	case "prev":
		prevUser := cmd[1]
		_, redraw := buttonReactionToken(s, i.GuildID, i.ChannelID, contract, i.Member.User.ID, 1, prevUser)
		if redraw {
			refreshBoostListMessage(s, contract)
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Token sent to %s", contract.Boosters[prevUser].Nick),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return

	}
}
