package boost

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// HandleSpeedrunCommand handles the speedrun command
func HandleSpeedrunCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}

	chickenRuns := 0
	contractStarter := ""
	sink := ""
	sinkPosition := SinkBoostFirst
	speedrunStyle := 0

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["contract-starter"]; ok {
		contractStarter = opt.UserValue(s).Mention()
		contractStarter = contractStarter[2 : len(contractStarter)-1]
		sink = contractStarter
	}
	if opt, ok := optionMap["sink"]; ok {
		sink = strings.TrimSpace(opt.StringValue())
		reMention := regexp.MustCompile(`<@!?(\d+)>`)
		if reMention.MatchString(sink) {
			sink = sink[2 : len(sink)-1]
		}
	}
	if opt, ok := optionMap["chicken-runs"]; ok {
		chickenRuns = int(opt.IntValue())
	}
	if opt, ok := optionMap["sink-position"]; ok {
		sinkPosition = int(opt.IntValue())
	}
	if opt, ok := optionMap["style"]; ok {
		speedrunStyle = int(opt.IntValue())
	}

	str, err := setSpeedrunOptions(s, i.GuildID, i.ChannelID, contractStarter, sink, sinkPosition, chickenRuns, speedrunStyle)
	if err != nil {
		str = err.Error()
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func setSpeedrunOptions(s *discordgo.Session, guildID string, channelID string, contractStarter string, sink string, sinkPosition int, chickenRuns int, speedrunStyle int) (string, error) {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	if contract.State != ContractStateSignup {
		return "", errors.New("contract must be in the Sign-up state to set speedrun options")
	}

	contract.Speedrun = true
	contract.SRData.SpeedrunStarterUserID = contractStarter
	contract.SRData.SinkUserID = sink
	contract.SRData.SinkBoostPosition = sinkPosition
	contract.SRData.ChickenRuns = chickenRuns
	contract.SRData.SpeedrunStyle = speedrunStyle
	contract.SRData.SpeedrunState = SpeedrunStateSignup

	// Set up the details for the Chicken Run Tango
	// first lap is CoopSize -1, every following lap is CoopSize -2

	contract.SRData.Tango[0] = max(0, contract.CoopSize-1)        // First Leg
	contract.SRData.Tango[1] = max(0, contract.SRData.Tango[0]-1) // Middle Legs
	contract.SRData.Tango[2] = 0                                  // Last Leg

	runs := contract.SRData.ChickenRuns
	contract.SRData.Legs = 0
	for runs > 0 {
		if contract.SRData.Legs == 0 {
			runs -= contract.SRData.Tango[0]
		} else if runs > contract.SRData.Tango[1] {
			runs -= contract.SRData.Tango[1]
		} else {
			contract.SRData.Tango[2] = runs
			runs = 0
		}
		contract.SRData.Legs++
	}

	var b strings.Builder
	fmt.Fprint(&b, "> Speedrun can be started once the contract is full.\n\n")
	fmt.Fprintf(&b, "> **%d** Chicken Run Legs to reach **%d** total chicken runs.\n", contract.SRData.Legs, contract.SRData.ChickenRuns)
	if contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
		fmt.Fprint(&b, "> **Wonky** style speed run:\n")
		fmt.Fprintf(&b, "> * Send all tokens to <@%s>\n", contract.SRData.SpeedrunStarterUserID)
		if contract.SRData.SpeedrunStarterUserID != contract.SRData.SinkUserID {
			fmt.Fprintf(&b, "> * After contract boosting send all tokens to: <@%s> (This is unusual)\n", contract.SRData.SinkUserID)
		}
	} else {
		fmt.Fprint(&b, "> **Boost List** style speed run:\n")
		fmt.Fprintf(&b, "> * During CRT send tokens to <@%s>\n", contract.SRData.SpeedrunStarterUserID)
		fmt.Fprint(&b, "> * Follow the Boost List for Token Passing.\n")
		fmt.Fprintf(&b, "> * After contract boosting send all tokens to <@%s>\n", contract.SRData.SinkUserID)
	}
	contract.SRData.StatusStr = b.String()

	var builder strings.Builder
	fmt.Fprintf(&builder, "Speedrun options set for %s/%s\n", contract.ContractID, contract.CoopID)
	fmt.Fprintf(&builder, "Contract Starter: <@%s>\n", contract.SRData.SpeedrunStarterUserID)
	fmt.Fprintf(&builder, "Sink CRT: <@%s>\n", contract.SRData.SinkUserID)

	disableButton := false
	if contract.Speedrun && contract.CoopSize != len(contract.Boosters) {
		disableButton = true
	}
	if contract.State != ContractStateSignup {
		disableButton = true
	}

	// For each contract location, update the signup message
	refreshBoostListMessage(s, contract)

	for _, loc := range contract.Location {
		// Rebuild the signup message to disable the start button
		msgID := loc.ReactionID
		msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)

		contentStr, comp := GetSignupComponents(disableButton, contract.Speedrun) // True to get a disabled start button
		msg.SetContent(contentStr)
		msg.Components = &comp
		s.ChannelMessageEditComplex(msg)
	}

	return builder.String(), nil
}

func reorderSpeedrunBoosters(contract *Contract) {
	// Speedrun contracts are always fair ordering over last 3 contracts
	newOrder := farmerstate.GetOrderHistory(contract.Order, 3)

	index := slices.Index(newOrder, contract.SRData.SpeedrunStarterUserID)
	// Remove the speedrun starter from the list
	newOrder = append(newOrder[:index], newOrder[index+1:]...)

	if contract.SRData.SinkBoostPosition == SinkBoostFirst {
		newOrder = append([]string{contract.SRData.SpeedrunStarterUserID}, newOrder...)
	} else {
		newOrder = append(newOrder, contract.SRData.SpeedrunStarterUserID)
	}
	contract.Order = newOrder
}

func drawSpeedrunCRT(contract *Contract, tokenStr string) string {
	var builder strings.Builder
	if contract.SRData.SpeedrunState == SpeedrunStateCRT {
		fmt.Fprintf(&builder, "# Chicken Run Tango Leg %d of %d\n", contract.SRData.CurrentLeg+1, contract.SRData.Legs)
		fmt.Fprintf(&builder, "### Tips\n")
		fmt.Fprintf(&builder, "- Don't use any boosts\n")
		fmt.Fprintf(&builder, "- Equip coop artifacts: Deflector and SIAB\n")
		fmt.Fprintf(&builder, "- A chicken run on <@%s> can be saved for the boost phase.\n", contract.SRData.SpeedrunStarterUserID)
		fmt.Fprintf(&builder, "- :truck: reaction will indicate truck arriving and request a later kick. Send tokens through the boost menu if doing this.\n")
		if contract.SRData.CurrentLeg == contract.SRData.Legs-1 {
			fmt.Fprintf(&builder, "### Final Kick Leg\n")
			fmt.Fprintf(&builder, "- After this kick build up your farm  as you would for boosting\n")
		}
		fmt.Fprintf(&builder, "## Tasks\n")
		fmt.Fprintf(&builder, "- Upgrade habs\n")
		fmt.Fprintf(&builder, "- Build up your farm to at least 20 chickens\n")
		fmt.Fprintf(&builder, "- Equip shiny artifacts to force a server sync\n")
		fmt.Fprintf(&builder, "- Run chickens on all the other farms and react with :white_check_mark: after all runs\n")
	}
	fmt.Fprintf(&builder, "> **Send %s to <@%s>**\n", tokenStr, contract.SRData.SpeedrunStarterUserID)

	return builder.String()
}

func addSpeedrunContractReactions(s *discordgo.Session, contract *Contract, channelID string, messageID string, tokenStr string) {
	if contract.SRData.SpeedrunState == SpeedrunStateCRT {
		s.MessageReactionAdd(channelID, messageID, tokenStr) // Token Reaction
		s.MessageReactionAdd(channelID, messageID, "✅")      // Run Reaction
		s.MessageReactionAdd(channelID, messageID, "🚚")      // Truck Reaction
		s.MessageReactionAdd(channelID, messageID, "🦵")      // Kick Reaction
	}
	if contract.SRData.SpeedrunState == SpeedrunStateBoosting {
		s.MessageReactionAdd(channelID, messageID, tokenStr) // Send token to Sink
		s.MessageReactionAdd(channelID, messageID, "🚀")      // Indicate boosting
		s.MessageReactionAdd(channelID, messageID, "💰")      // Sink sent requested number of tokens to booster
	}
	if contract.SRData.SpeedrunState == SpeedrunStatePost {
		s.MessageReactionAdd(channelID, messageID, tokenStr) // Send token to Sink
	}
	s.MessageReactionAdd(channelID, messageID, "❓") // Finish

}
