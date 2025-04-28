package boost

import (
	"fmt"
	"strings"

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

	switch values[0] {
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

	}

}
