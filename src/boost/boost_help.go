package boost

import (
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
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
		str := `__**/contract**__
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
			speedRunStr := `__**/speedrun**__  (runs from contract data, Wonky style, sink boosts first)
			> * **contract-starter** : Sink during CRT & boosting. 
			>  * If farmer using an alt as the sink, use the farmer's name as sink.
			__**/link-alternate**__
			> * After */speedrun*, a farmer with an alt can use this to swap in their alt as the contract-starter sink.
			__**/change-planned-start**__
			> * Set the planned start time for the contract.
			`

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
		str := `> __**/join-contract**__ : Add a farmer to the contract.
		> __**/prune**__ : Remove a booster from the contract.
		> __**/change**__ : Alter aspects of a running contract
		> * *contract-id* : Change the contract-id.
		> * *coop-id* : Change the coop-id.
		> __**/change-ping-role**__ : Change the ping role to something else.
		> __**/change-one-booster**__ : Move a single booster to a different position.
		> __**/bump**__ : Redraw the Boost List message.
		`

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
		boosterStr := `
	> __**/calc-contract-tval**__ : Display what the bot knows about your token values.
	> __**/boost**__ : Out of order boosting, mark yourself as boosted.
	> __**/unboost**__ : Mark a booster as unboosted.
	> __**/coopeta**__ : Display a discord message with a discord timestamp of the contract completion time.
	> __**/seteggincname**__ : Use to set your Egg, Inc game name.
`
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "BOOSTER COMMANDS",
			Value:  boosterStr,
			Inline: false,
		})
	}

	if true {
		str := `
		> __**/launch-helper**__ : Launch planning helper.
		> __**/token**__ : General purpose Token Tracker via DM.
		> __**/fun**__ : Some fun commands that use LLM to create wishes and images.
		`

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "GENERAL COMMANDS",
			Value:  str,
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
