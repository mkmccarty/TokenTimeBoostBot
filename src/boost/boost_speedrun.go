package boost

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
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

	str, err := setSpeedrunOptions(s, i.ChannelID, contractStarter, sink, sinkPosition, chickenRuns, speedrunStyle)
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

func setSpeedrunOptions(s *discordgo.Session, channelID string, contractStarter string, sink string, sinkPosition int, chickenRuns int, speedrunStyle int) (string, error) {
	var contract = FindContract(channelID)
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	if contract.State != ContractStateSignup {
		return "", errors.New("contract must be in the Sign-up state to set speedrun options")
	}

	if speedrunStyle == SpeedrunStyleWonky {
		// Verify that the sink is a snowflake id
		if _, err := s.User(sink); err != nil {
			return "", errors.New("sink must user mention for Wonky style boost lists")
		}
	}

	contract.Speedrun = true
	contract.SRData.SpeedrunStarterUserID = contractStarter
	contract.SRData.SinkUserID = sink
	contract.SRData.SinkBoostPosition = sinkPosition
	contract.SRData.ChickenRuns = chickenRuns
	contract.SRData.SpeedrunStyle = speedrunStyle
	contract.SRData.SpeedrunState = SpeedrunStateSignup
	contract.BoostOrder = ContractOrderFair

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
		} else if contract.SRData.Tango[1] == 0 {
			// Not possible to do any CRT
			break
		} else if runs > contract.SRData.Tango[1] {
			runs -= contract.SRData.Tango[1]
		} else {
			contract.SRData.Tango[2] = runs
			runs = 0
			break // No more runs to do, skips the Legs++ below
		}
		contract.SRData.Legs++
	}

	var b strings.Builder
	fmt.Fprint(&b, "> Speedrun can be started once the contract is full.\n\n")
	if contract.SRData.Tango[0] != 1 {
		fmt.Fprintf(&b, "> **%d** Chicken Run Legs to reach **%d** total chicken runs.\n", contract.SRData.Legs, contract.SRData.ChickenRuns)
	} else {
		fmt.Fprintf(&b, "> It's not possible to reach **%d** total chicken runs with only **%d** farmers.\n", contract.SRData.ChickenRuns, contract.CoopSize)
	}
	if contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
		fmt.Fprint(&b, "> **Wonky** style speed run:\n")
		fmt.Fprintf(&b, "> * Send all tokens to <@%s>\n", contract.SRData.SpeedrunStarterUserID)
		fmt.Fprintf(&b, "> The sink will send you a full set of boost tokens.\n")
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
		fmt.Fprintf(&builder, "# Chicken Run Tango - Leg %d of %d\n", contract.SRData.CurrentLeg+1, contract.SRData.Legs)
		fmt.Fprintf(&builder, "### Tips\n")
		fmt.Fprintf(&builder, "- Don't use any boosts\n")
		fmt.Fprintf(&builder, "- Equip coop artifacts: Deflector and SIAB\n")
		fmt.Fprintf(&builder, "- A chicken run on <@%s> can be saved for the boost phase.\n", contract.SRData.SpeedrunStarterUserID)
		fmt.Fprintf(&builder, "- :truck: reaction will indicate truck arriving and request a later kick. Send tokens through the boost menu if doing this.\n")
		if contract.SRData.CurrentLeg == contract.SRData.Legs-1 {
			fmt.Fprintf(&builder, "### Final Kick Leg\n")
			fmt.Fprintf(&builder, "- After this kick you can build up your farm as you would for boosting\n")
		}
		fmt.Fprintf(&builder, "## Tasks\n")
		fmt.Fprintf(&builder, "1. Upgrade habs\n")
		fmt.Fprintf(&builder, "2. Build up your farm to at least 20 chickens\n")
		fmt.Fprintf(&builder, "3. Equip shiny artifact to force a server sync\n")
		fmt.Fprintf(&builder, "4. Run chickens on all the other farms and react with :white_check_mark: after all runs\n")
	}
	fmt.Fprintf(&builder, "\n**Send %s to <@%s>**\n", tokenStr, contract.SRData.SpeedrunStarterUserID)

	return builder.String()
}

func addSpeedrunContractReactions(s *discordgo.Session, contract *Contract, channelID string, messageID string, tokenStr string) {
	if contract.SRData.SpeedrunState == SpeedrunStateCRT {
		s.MessageReactionAdd(channelID, messageID, tokenStr) // Token Reaction
		s.MessageReactionAdd(channelID, messageID, "✅")      // Run Reaction
		s.MessageReactionAdd(channelID, messageID, "🚚")      // Truck Reaction
		s.MessageReactionAdd(channelID, messageID, "🦵")      // Kick Reaction
		s.MessageReactionAdd(channelID, messageID, "💃")      // Tango Reaction
	}
	if contract.SRData.SpeedrunState == SpeedrunStateBoosting {
		s.MessageReactionAdd(channelID, messageID, tokenStr) // Send token to Sink
		s.MessageReactionAdd(channelID, messageID, "🐓")      // Want Chicken Run
		s.MessageReactionAdd(channelID, messageID, "💰")      // Sink sent requested number of tokens to booster
	}
	if contract.SRData.SpeedrunState == SpeedrunStatePost {
		s.MessageReactionAdd(channelID, messageID, tokenStr) // Send token to Sink
		s.MessageReactionAdd(channelID, messageID, "🐓")      // Want Chicken Run
		s.MessageReactionAdd(channelID, messageID, "🏁")      // Run Reaction
	}
}

func speedrunReactions(s *discordgo.Session, r *discordgo.MessageReaction, contract *Contract) string {
	returnVal := ""
	keepReaction := false
	redraw := false
	emojiName := r.Emoji.Name

	// Token reaction handling
	if strings.ToLower(r.Emoji.Name) == "token" {
		var b *Booster
		if contract.SRData.SpeedrunState == SpeedrunStateCRT {
			b = contract.Boosters[contract.SRData.SpeedrunStarterUserID]
		} else {
			b = contract.Boosters[contract.SRData.SinkUserID]
		}

		b.TokensReceived++
		emojiName = r.Emoji.Name + ":" + r.Emoji.ID
		if r.UserID != b.UserID {
			// Record the Tokens as received
			b.TokenReceivedTime = append(b.TokenReceivedTime, time.Now())
			track.ContractTokenMessage(s, r.ChannelID, b.UserID, track.TokenReceived, 1, r.UserID)

			// Record who sent the token
			track.ContractTokenMessage(s, r.ChannelID, r.UserID, track.TokenSent, 1, b.UserID)
			contract.Boosters[r.UserID].TokenSentTime = append(contract.Boosters[r.UserID].TokenSentTime, time.Now())
		} else {
			track.FarmedToken(s, r.ChannelID, r.UserID)
		}
		redraw = true
	}

	if contract.SRData.SpeedrunState == SpeedrunStateCRT {

		if r.Emoji.Name == "✅" {
			keepReaction = true
			var msg, err = s.ChannelMessage(r.ChannelID, r.MessageID)
			if err == nil {
				if msg.Reactions[0].Count > contract.CoopSize {
					str := fmt.Sprintf("All players have run chickens. <@%s> can now react with 🦵 then kick all farmers and go to the next CRT leg with 💃.", r.UserID)
					s.ChannelMessageSend(r.ChannelID, str)
				}

			}
			// Indicate that the farmer has completed running chickens
		}

		if r.Emoji.Name == "🚚" {
			keepReaction = true
			// Indicate that the farmer has a truck incoming
			str := fmt.Sprintf("Truck arriving for <@%s>. The sink may or may not pause kicks.", r.UserID)
			s.ChannelMessageSend(contract.Location[0].ChannelID, str)
		}

		if r.UserID == contract.SRData.SpeedrunStarterUserID {
			if r.Emoji.Name == "🦵" {
				keepReaction = true
				// Indicate that the Sink is starting to kick users
				str := "Starting to kick users. Swap shiny artifacts if you need to force a server sync."
				s.ChannelMessageSend(contract.Location[0].ChannelID, str)
			}

			if r.Emoji.Name == "💃" {
				keepReaction = true

				// Indicate that this Tango Leg is complete
				str := "Kicks completed."
				contract.SRData.CurrentLeg++ // Move to the next leg
				if contract.SRData.CurrentLeg == contract.SRData.Legs {
					contract.SRData.SpeedrunState = SpeedrunStateBoosting
					str += " This was the final kick. Build up your farm as you would for boosting.\n"
				}
				//if contract.SRData.SpeedrunStyle == SpeedrunStyleFastrun {
				contract.Boosters[contract.Order[contract.BoostPosition]].BoostState = BoostStateTokenTime
				contract.Boosters[contract.Order[contract.BoostPosition]].StartTime = time.Now()
				//}
				s.ChannelMessageSend(contract.Location[0].ChannelID, str)
				sendNextNotification(s, contract, true)
			}
		}
	}

	if contract.SRData.SpeedrunState == SpeedrunStateBoosting {
		if r.UserID == contract.SRData.SinkUserID {
			if r.Emoji.Name == "💰" {
				fmt.Println("Sink sent requested number of tokens to booster")
				var b, sink *Booster
				b = contract.Boosters[contract.Order[contract.BoostPosition]]
				//var sink *Booster
				sink = contract.Boosters[contract.SRData.SinkUserID]

				if r.UserID == b.UserID {
					// Current booster subtract number of tokens wanted
					sink.TokensReceived -= b.TokensWanted
					sink.TokensReceived = max(0, sink.TokensReceived) // Avoid missing self farmed tokens
				} else {
					// Current booster number of tokens wanted
					b.TokensReceived += b.TokensWanted
					sink.TokensReceived -= b.TokensWanted
					sink.TokensReceived = max(0, sink.TokensReceived) // Avoid missing self farmed tokens
					// Record the Tokens as received
					// Append TokensReceived number of time.Now() to the TokenReceivedTime slice
					for i := 0; i < b.TokensWanted; i++ {
						b.TokenReceivedTime = append(b.TokenReceivedTime, time.Now())
						contract.Boosters[r.UserID].TokenSentTime = append(contract.Boosters[r.UserID].TokenSentTime, time.Now())
					}
					track.ContractTokenMessage(s, r.ChannelID, b.UserID, track.TokenReceived, b.TokensReceived, r.UserID)
					track.ContractTokenMessage(s, r.ChannelID, r.UserID, track.TokenSent, b.TokensReceived, b.UserID)
				}

				Boosting(s, r.GuildID, r.ChannelID)

				str := fmt.Sprintf("<@%s> you've been sent %d tokens to boost with!", b.UserID, b.TokensWanted)
				s.ChannelMessageSend(contract.Location[0].ChannelID, str)

				redraw = false
			}
		}
	}

	if contract.SRData.SpeedrunState == SpeedrunStateBoosting || contract.SRData.SpeedrunState == SpeedrunStatePost {
		if r.Emoji.Name == "🐓" {
			// Indicate that a farmer is ready for chicken runs
			str := fmt.Sprintf("%s <@%s> is ready for chicken runs.", contract.Location[0].ChannelPing, r.UserID)
			var data discordgo.MessageSend
			var am discordgo.MessageAllowedMentions
			data.AllowedMentions = &am
			data.Content = str
			msg, _ := s.ChannelMessageSendComplex(contract.Location[0].ChannelID, &data)
			s.MessageReactionAdd(msg.ChannelID, msg.ID, "🐣") // Indicate Chicken Run
			keepReaction = true
		}
	}

	if contract.SRData.SpeedrunState == SpeedrunStatePost && creatorOfContract(contract, r.UserID) {
		// Coordinator can end the contract
		if r.Emoji.Name == "🏁" {
			contract.State = ContractStateArchive
			contract.SRData.SpeedrunState = SpeedrunStateComplete
			sendNextNotification(s, contract, true)
			return returnVal
		}
	}

	// Remove extra added emoji
	if !keepReaction {
		s.MessageReactionRemove(r.ChannelID, r.MessageID, emojiName, r.UserID)
	}

	if redraw {
		refreshBoostListMessage(s, contract)
	}

	return returnVal
}
