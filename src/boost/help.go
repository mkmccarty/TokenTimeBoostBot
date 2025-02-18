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
		str := fmt.Sprintf(">>> %s\n", bottools.GetFormattedCommand("contract"))
		str += fmt.Sprint("* **contract-id** : Select from dropdown of contracts.\n")
		str += fmt.Sprint("* **coop-id** : Coop id")

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
			speedRunStr := fmt.Sprintf(`
			> * Use %s to bring up the contract settings.
			> * Use %s to set the planned start time for the contract.
			`,
				bottools.GetFormattedCommand("contract-settings"),
				bottools.GetFormattedCommand("change-planned-start"))

			field = append(field, &discordgo.MessageEmbedField{
				Name:   "Basic Contract Info",
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
		var strBuilder strings.Builder
		fmt.Fprintf(&strBuilder, ">>> %s : Add a farmer to the contract (don't use a mention for guest/alt).\n", bottools.GetFormattedCommand("join-contract"))
		fmt.Fprintf(&strBuilder, "%s : Remove a booster from the contract.\n", bottools.GetFormattedCommand("prune"))
		fmt.Fprintf(&strBuilder, "%s : Alter aspects of a running contract\n", bottools.GetFormattedCommand("change"))
		fmt.Fprintf(&strBuilder, "* *contract-id* : Change the contract-id.\n")
		fmt.Fprintf(&strBuilder, "* *coop-id* : Change the coop-id.\n")
		fmt.Fprintf(&strBuilder, "%s : Change the ping role to something else.\n", bottools.GetFormattedCommand("change-ping-role"))
		fmt.Fprintf(&strBuilder, "%s : Move a single booster to a different position.\n", bottools.GetFormattedCommand("change-one-booster"))
		fmt.Fprintf(&strBuilder, "%s : Redraw the Boost List message.\n", bottools.GetFormattedCommand("bump"))

		str := strBuilder.String()
		if len(str) >= 1000 {
			str = str[:1000]
		}

		field = append(field, &discordgo.MessageEmbedField{
			Name:   "COORDINATOR COMMANDS",
			Value:  strBuilder.String(),
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
	>>> %s : Add a farmer to the contract (don't use a mention for guest/alt).
	%s : To link an alternate to a main account.
	%s : Display what the bot knows about your token values.
	%s : Out of order boosting, mark yourself as boosted.
	%s : Mark a booster as unboosted.
	%s : Display a discord message with a discord timestamp of the contract completion time.
	%s : Use to set your Egg, Inc game name.
`,
			bottools.GetFormattedCommand("join-contract"),
			bottools.GetFormattedCommand("link-alternate"),
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
		fmt.Fprintf(&builder, ">>> %s : Launch planning helper.\n", bottools.GetFormattedCommand("launch-helper"))
		fmt.Fprintf(&builder, "%s : General purpose Token Tracker via DM.\n", bottools.GetFormattedCommand("token"))
		fmt.Fprintf(&builder, "%s : Last occurrance of every event.\n", bottools.GetFormattedCommand("events"))
		fmt.Fprintf(&builder, "%s : Timer tool\n", bottools.GetFormattedCommand("timer"))

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
