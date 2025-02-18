package boost

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

// HandleHelpCommand will handle the help command
func HandleHelpCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := ""
	if i.GuildID == "" {
		userID = i.User.ID
	} else {
		userID = i.Member.User.ID
	}

	embed := GetHelp(s, i.GuildID, i.ChannelID, userID)
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Embeds:     embed.Embeds,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
	if err != nil {
		log.Print(err)
	}
}

// GetHelp will return the help string for the contract
func GetHelp(s *discordgo.Session, guildID string, channelID string, userID string) *discordgo.MessageSend {
	var field []*discordgo.MessageEmbedField

	var builder strings.Builder
	var footer strings.Builder

	builder.WriteString("Context aware useful commands for Boost Bot.")

	footer.WriteString("Bold parameters are required. Italic parameters are optional.")

	var contract = FindContract(channelID)
	if contract == nil {

		// No contract, show help for creating a contract
		// Anyone can do this so just give the basic instructions
		str := bottools.GetFormattedCommand("contract")
		str += `
		> * **contract-id** : Select from dropdown of contracts.
		> * **coop-id** : Coop id
		`

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "CREATE CONTRACT",
			Value:  str,
			Inline: false,
		})
	}

	contractCreator := creatorOfContract(s, contract, userID)

	if contract != nil && contractCreator {

		if contract.State == ContractStateSignup {

			// Speedrun info
			speedRunStr := fmt.Sprintf(`__**%s**__  (runs from contract data, Banker style, sink boosts first)
			> * **contract-starter** : Sink during CRT & boosting. 
			>  * If farmer using an alt as the sink, use the farmer's name as sink.
			__**%s**__
			> * After %s, a farmer with an alt can use this to swap in their alt as the contract-starter sink.
			__**%s**__
			> * Set the planned start time for the contract.
			`,
				bottools.GetFormattedCommand("speedrun"),
				bottools.GetFormattedCommand("link-alternate"),
				bottools.GetFormattedCommand("speedrun"),
				bottools.GetFormattedCommand("change-planned-start"))

			field = append(field, &discordgo.MessageEmbedField{
				Name:   "MODIFY CONTRACT TO SPEEDRUN",
				Value:  speedRunStr,
				Inline: false,
			})

			str := `
			Press the 🟩 Green Button to move from the Sign-up phase to the Boost phase.
			`
			field = append(field, &discordgo.MessageEmbedField{
				Name:   "START CONTRACT",
				Value:  str,
				Inline: false,
			})

		}

		// Important commands for contract creators
		str := fmt.Sprintf(`> %s : Add a farmer to the contract.
		> %s : Remove a booster from the contract.
		> %s : Alter aspects of a running contract
		> * *contract-id* : Change the contract-id.
		> * *coop-id* : Change the coop-id.
		> %s : Change the ping role to something else.
		> %s : Move a single booster to a different position.
		> %s : Redraw the Boost List message.
		`,
			bottools.GetFormattedCommand("join-contract"),
			bottools.GetFormattedCommand("prune"),
			bottools.GetFormattedCommand("change"),
			bottools.GetFormattedCommand("change-ping-role"),
			bottools.GetFormattedCommand("change-one-booster"),
			bottools.GetFormattedCommand("bump"))

		if len(str) > 900 {
			str = str[:900]
		}

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "COORDINATOR COMMANDS",
			Value:  str,
			Inline: false,
		})
	}

	if contract != nil {

		if !userInContract(contract, userID) {
			str := ` See the pinned message for buttons to *Join*, *Join w/Ping* or *Leave* the contract.
		You can set your boost tokens wanted by selecting :five: :six: or :eight: and adjusting it with the +Token and -Token buttons.
		`
			field = append(field, &discordgo.MessageEmbedField{
				Name:   "JOIN CONTRACT",
				Value:  str,
				Inline: false,
			})

			// No point in showing the rest of the help
		}

		// Basics for those Boosting
		boosterStr := fmt.Sprintf(`
	> %s : Display what the bot knows about your token values.
	> %s : Out of order boosting, mark yourself as boosted.
	> %s : Mark a booster as unboosted.
	> %s : Display a discord message with a discord timestamp of the contract completion time.
	> %s : Use to set your Egg, Inc game name.
`,
			bottools.GetFormattedCommand("calc-contract-tval"),
			bottools.GetFormattedCommand("boost"),
			bottools.GetFormattedCommand("unboost"),
			bottools.GetFormattedCommand("coopeta"),
			bottools.GetFormattedCommand("seteggincname"))
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "BOOSTER COMMANDS",
			Value:  boosterStr,
			Inline: false,
		})
	}

	if true {
		var builder strings.Builder
		fmt.Fprintf(&builder, "> %s : Launch planning helper.\n", bottools.GetFormattedCommand("launch-helper"))
		fmt.Fprintf(&builder, "> %s : General purpose Token Tracker via DM.\n", bottools.GetFormattedCommand("token"))
		fmt.Fprintf(&builder, "> %s : Last occurrance of every event.\n", bottools.GetFormattedCommand("events"))
		fmt.Fprintf(&builder, "> %s : Timer tool\n", bottools.GetFormattedCommand("timer"))

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "GENERAL COMMANDS",
			Value:  builder.String(),
			Inline: false,
		})

	}

	embed := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Type:        discordgo.EmbedTypeRich,
			Title:       "Boost Bot Help",
			Description: builder.String(),
			Color:       0x888888, // Warm purple color
			Fields:      field,
			Footer: &discordgo.MessageEmbedFooter{
				Text: footer.String(),
			},
		},
		},
	}

	return embed
}
