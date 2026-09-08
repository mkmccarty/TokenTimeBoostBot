package boost

import (
	"fmt"
	"log"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashHelpCommand returns the command for the /help command
func GetSlashHelpCommand(cmd string) *dc.Command {
	command := anywhereCommand(cmd, "Help with Boost Bot commands.")
	return &command
}

// HandleHelpCommand will handle the help command
func HandleHelpCommand(client dc.Client, e *dc.CommandEvent) {
	embed := GetHelp(client, e.GuildID(), e.ChannelID(), e.UserID())
	err := e.Respond(dc.Message{
		Embeds:    []dc.Embed{*embed},
		Ephemeral: true,
	})
	if err != nil {
		log.Print(err)
	}
}

// GetHelp will return the help string for the contract
func GetHelp(client dc.Client, guildID string, channelID string, userID string) *dc.Embed {
	userCmd := false
	var field []dc.EmbedField

	var builder strings.Builder
	var footer strings.Builder

	_, errch := client.Channel(channelID)
	if errch != nil {
		userCmd = true
	}

	if !userCmd {
		builder.WriteString("Context aware useful commands for Boost Bot.")
		footer.WriteString("Bold parameters are required. Italic parameters are optional.")
		var contract = FindContract(channelID)
		if contract == nil {

			// No contract, show help for creating a contract
			// Anyone can do this so just give the basic instructions
			str := fmt.Sprintf(">>> %s\n", bottools.GetFormattedCommand("contract"))
			str += "* **contract-id** : Select from dropdown of contracts.\n"
			str += "* **coop-id** : Coop id"

			field = append(field, dc.EmbedField{
				Name:   "CREATE CONTRACT",
				Value:  str,
				Inline: false,
			})
		}

		contractCreator := creatorOfContract(client, contract, userID)

		if contract != nil && contractCreator {

			if contract.State == ContractStateSignup {

				// Speedrun info
				speedRunStr := fmt.Sprintf(`
			> * Use %s to bring up the contract settings.
			> * Use %s or %s to set the planned start time for the contract.
			`,
					bottools.GetFormattedCommand("contract-settings"),
					bottools.GetFormattedCommand("change-start offset"),
					bottools.GetFormattedCommand("change-start timestamp"),
				)

				field = append(field, dc.EmbedField{
					Name:   "Basic Contract Info",
					Value:  speedRunStr,
					Inline: false,
				})

				str := `
			Press the 🟩 Green Button to move from the Sign-up phase to the Boost phase.
			`
				field = append(field, dc.EmbedField{
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

			field = append(field, dc.EmbedField{
				Name:   "COORDINATOR COMMANDS",
				Value:  strBuilder.String(),
				Inline: false,
			})
		}

		if contract != nil {

			if !UserInContract(contract, userID) {
				str := ` See the pinned message for buttons to *Join* or *Leave* the contract.
		You can set your boost tokens wanted by selecting :five: :six: or :eight: and adjusting it with the +Token and -Token buttons.
		`
				field = append(field, dc.EmbedField{
					Name:   "JOIN CONTRACT",
					Value:  str,
					Inline: false,
				})

				// No point in showing the rest of the help
			}

			// Basics for those Boosting
			var boosterStrBuilder strings.Builder
			fmt.Fprintf(&boosterStrBuilder, ">>> %s : Add a farmer to the contract (don't use a mention for guest/alt).\n", bottools.GetFormattedCommand("join-contract"))
			fmt.Fprintf(&boosterStrBuilder, "%s : To link an alternate to a main account.\n", bottools.GetFormattedCommand("link-alternate"))
			fmt.Fprintf(&boosterStrBuilder, "%s : To set your artifacts for ELR boost order.\n", bottools.GetFormattedCommand("artifact"))
			fmt.Fprintf(&boosterStrBuilder, "%s : Display what the bot knows about your token values.\n", bottools.GetFormattedCommand("calc-contract-tval"))
			fmt.Fprintf(&boosterStrBuilder, "%s : Out of order boosting, mark yourself as boosted.\n", bottools.GetFormattedCommand("boost"))
			fmt.Fprintf(&boosterStrBuilder, "%s : Mark a booster as unboosted.\n", bottools.GetFormattedCommand("unboost"))
			fmt.Fprintf(&boosterStrBuilder, "%s : Display a discord message with a discord timestamp of the contract completion time.\n", bottools.GetFormattedCommand("coopeta"))
			fmt.Fprintf(&boosterStrBuilder, "%s : Use to set your Egg, Inc game name.\n", bottools.GetFormattedCommand("seteggincname"))

			field = append(field, dc.EmbedField{
				Name:   "BOOSTER COMMANDS",
				Value:  boosterStrBuilder.String(),
				Inline: false,
			})
		}
	}

	if true {
		var builder strings.Builder
		fmt.Fprintf(&builder, "%s : Contract completion estimate.\n", bottools.GetFormattedCommand("estimate-contract-time"))
		fmt.Fprintf(&builder, "%s : Launch planning helper.\n", bottools.GetFormattedCommand("launch-helper"))
		fmt.Fprintf(&builder, "%s : Contract stones use\n", bottools.GetFormattedCommand("stones"))
		fmt.Fprintf(&builder, "%s : Contract teamwork evaluation\n", bottools.GetFormattedCommand("teamwork"))
		fmt.Fprintf(&builder, "%s : Contract score estimates\n", bottools.GetFormattedCommand("cs-estimate"))
		fmt.Fprintf(&builder, "%s : Eggs of Virtue Helper\n", bottools.GetFormattedCommand("virtue"))
		fmt.Fprintf(&builder, "%s : Rerun evaluation\n", bottools.GetFormattedCommand("rerun-eval active"))
		fmt.Fprintf(&builder, "%s : Last occurrance of every event.\n", bottools.GetFormattedCommand("events"))
		fmt.Fprintf(&builder, "%s : Timer tool\n", bottools.GetFormattedCommand("timer"))

		field = append(field, dc.EmbedField{
			Name:   "GENERAL COMMANDS",
			Value:  builder.String(),
			Inline: false,
		})

	}

	embed := &dc.Embed{
		Title:       "Boost Bot Help",
		Description: builder.String(),
		Color:       0x888888, // Warm purple color
		Fields:      field,
		Footer: &dc.EmbedFooter{
			Text: footer.String(),
		},
	}

	return embed
}
