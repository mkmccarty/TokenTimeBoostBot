package boost

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// RedrawBoostList will move the boost message to the bottom of the channel
func RedrawBoostList(s *discordgo.Session, guildID string, channelID string) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	if contract.State == ContractStateSignup {
		return errors.New(errorContractNotStarted)
	}
	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()
	// Edit the boost list in place
	buttonComponents := getContractReactionsComponents(contract)
	for _, loc := range contract.Location {
		if loc.GuildID == guildID && loc.ChannelID == channelID {
			_ = s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)
			var data discordgo.MessageSend
			var am discordgo.MessageAllowedMentions
			var components []discordgo.MessageComponent
			data.Flags = discordgo.MessageFlagsIsComponentsV2
			data.AllowedMentions = &am
			listComp := DrawBoostList(s, contract)
			components = append(components, listComp...)
			components = append(components, buttonComponents...)
			data.Components = components
			msg, err := s.ChannelMessageSendComplex(loc.ChannelID, &data)
			if err == nil {
				SetListMessageID(contract, loc.ChannelID, msg.ID)
				//				addContractReactionsButtons(s, contract, loc.ChannelID, msg.ID)
			}
		}
	}
	return nil
}

func refreshBoostListMessage(s *discordgo.Session, contract *Contract) {
	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()
	// Edit the boost list in place
	for _, loc := range contract.Location {
		var components []discordgo.MessageComponent
		msgedit := discordgo.NewMessageEdit(loc.ChannelID, loc.ListMsgID)
		msgedit.Flags = discordgo.MessageFlagsIsComponentsV2
		listComponents := DrawBoostList(s, contract)
		buttonComponents := getContractReactionsComponents(contract)
		components = append(components, listComponents...)
		components = append(components, buttonComponents...)
		msgedit.Components = &components

		// Full contract for speedrun
		msg, err := s.ChannelMessageEditComplex(msgedit)
		if err == nil {
			// This is an edit, it should be the same
			loc.ListMsgID = msg.ID
		}
		updateSignupReactionMessage(s, contract, loc)
	}
}

func sendNextNotification(s *discordgo.Session, contract *Contract, pingUsers bool) {
	// Start boosting contract
	drawn := false
	for _, loc := range contract.Location {
		var msg *discordgo.Message
		var err error

		if contract.State == ContractStateSignup {
			var components []discordgo.MessageComponent
			msgedit := discordgo.NewMessageEdit(loc.ChannelID, loc.ListMsgID)
			// Full contract for speedrun
			listComp := DrawBoostList(s, contract)
			components = append(components, listComp...)
			msgedit.Components = &components
			msgedit.Flags = discordgo.MessageFlagsIsComponentsV2
			_, err := s.ChannelMessageEditComplex(msgedit)
			if err != nil {
				fmt.Println("Unable to send this message." + err.Error())
			}
			updateSignupReactionMessage(s, contract, loc)

		} else {
			var components []discordgo.MessageComponent

			// Unpin message once the contract is completed
			if contract.State == ContractStateArchive {
				_ = s.ChannelMessageUnpin(loc.ChannelID, loc.ReactionID)
			}
			_ = s.ChannelMessageDelete(loc.ChannelID, loc.ListMsgID)

			// Compose the message without a Ping
			var data discordgo.MessageSend
			var am discordgo.MessageAllowedMentions
			data.Flags = discordgo.MessageFlagsIsComponentsV2
			data.AllowedMentions = &am
			listComp := DrawBoostList(s, contract)
			buttonComponents := getContractReactionsComponents(contract)
			components = append(components, listComp...)
			components = append(components, buttonComponents...)
			data.Components = components
			msg, err = s.ChannelMessageSendComplex(loc.ChannelID, &data)
			if err == nil {
				SetListMessageID(contract, loc.ChannelID, msg.ID)
			}
			drawn = true
		}
		if err != nil {
			fmt.Println("Unable to resend message." + err.Error())
		}
		var str = ""
		if msg == nil {
			// Maybe this location is broken
			continue
		}

		switch contract.State {
		case ContractStateWaiting, ContractStateBanker, ContractStateFastrun:
			//addContractReactionsButtons(s, contract, loc.ChannelID, msg.ID)
			if pingUsers {
				if contract.State == ContractStateFastrun || contract.State == ContractStateBanker {
					var name string
					var einame = farmerstate.GetEggIncName(contract.Order[contract.BoostPosition])
					if einame != "" {
						einame += " " // Add a space to this
					}

					if contract.Banker.CurrentBanker != "" {
						name = einame + contract.Boosters[contract.Banker.CurrentBanker].Mention
					} else {
						name = einame + contract.Boosters[contract.Order[contract.BoostPosition]].Mention
					}

					str = fmt.Sprintf(loc.RoleMention+" send tokens to %s", name)
				} else {
					if contract.Banker.CurrentBanker == "" {
						str = loc.RoleMention + " contract boosting complete. Hold your tokens for late joining farmers."
					} else {
						str = "Contract boosting complete. There may late joining farmers. "
						if contract.State == ContractStateCompleted || contract.State == ContractStateWaiting {
							var einame = farmerstate.GetEggIncName(contract.Banker.CurrentBanker)
							if einame != "" {
								einame += " " // Add a space to this
							}
							name := einame + contract.Boosters[contract.Banker.CurrentBanker].Mention
							str = fmt.Sprintf(loc.RoleMention+" send tokens to our volunteer sink **%s**", name)
						}
					}
				}
			}
		case ContractStateCompleted:
			//addContractReactionsButtons(s, contract, loc.ChannelID, msg.ID)
			t1 := contract.EndTime
			t2 := contract.StartTime
			duration := t1.Sub(t2)
			str = fmt.Sprintf(loc.RoleMention+" contract boosting complete in %s ", duration.Round(time.Second))
			if contract.Banker.CurrentBanker != "" {
				var einame = farmerstate.GetEggIncName(contract.Banker.CurrentBanker)
				if einame != "" {
					einame += " " // Add a space to this
				}
				if contract.State != ContractStateArchive {
					name := einame + contract.Boosters[contract.Banker.CurrentBanker].Mention
					str += fmt.Sprintf("\nSend tokens to our volunteer sink **%s**", name)
				}
			}
		default:
		}

		// Sending the update message
		if contract.Style&ContractFlagBanker == 0 {
			_, _ = s.ChannelMessageSend(loc.ChannelID, str)
		} else if !drawn {
			_ = RedrawBoostList(s, loc.GuildID, loc.ChannelID)
		}
	}
	if pingUsers {
		notifyBellBoosters(s, contract)
	}
	/*
		if contract.State == ContractStateArchive {
			// Only purge the contract from memory if /calc isn't being used
			if contract.CalcOperations == 0 || time.Since(contract.CalcOperationTime).Minutes() > 20 {
				FinishContract(s, contract)
			}
		}
	*/
}

func updateSignupReactionMessage(s *discordgo.Session, contract *Contract, loc *LocationData) {
	//if contract.State != ContractStateSignup {
	//	return
	//	}
	// Only want to update this when we have to change the Join button
	//if len(contract.Order) == contract.CoopSize || len(contract.Order) == (contract.CoopSize-1) {
	var components []discordgo.MessageComponent
	msgID := loc.ReactionID
	msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)
	// Full contract for speedrun
	contentStr, comp := GetSignupComponents(contract)
	components = append(components, &discordgo.TextDisplay{
		Content: contentStr,
	})
	components = append(components, comp...)
	msg.Flags = discordgo.MessageFlagsIsComponentsV2
	msg.Components = &components
	_, err := s.ChannelMessageEditComplex(msg)
	if err != nil {
		log.Printf("unable to send this message: %v", err)
	}
	//}
}
