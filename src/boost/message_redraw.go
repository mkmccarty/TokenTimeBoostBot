package boost

import (
	"errors"
	"fmt"
	"log"
	"maps"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// UpdateAllContractsEggInfo updates the EggName and EggEmoji fields for all active contracts,
// and regenerates banners if necessary.
func UpdateAllContractsEggInfo() {
	mutex.Lock()
	// Copy contracts to avoid holding the global lock during banner generation
	contractsCopy := make([]*Contract, 0, len(Contracts))
	for _, c := range Contracts {
		contractsCopy = append(contractsCopy, c)
	}
	mutex.Unlock()

	for _, contract := range contractsCopy {
		if contract == nil {
			continue
		}

		contract.mutex.Lock()
		if len(contract.Location) == 0 {
			contract.mutex.Unlock()
			continue
		}

		originalContract, exists := ei.EggIncContractsAll[contract.ContractID]
		if !exists {
			contract.mutex.Unlock()
			continue
		}

		contract.Egg = originalContract.Egg
		contract.EggName = originalContract.EggName
		contract.EggEmoji = FindEggEmoji(originalContract.EggName)

		var creatorID string
		if len(contract.CreatorID) > 0 {
			creatorID = contract.CreatorID[0]
		}
		guildID := contract.Location[0].GuildID
		bannerText := contract.Name
		if bannerText == "" {
			bannerText = originalContract.Name
		}
		eggName := contract.EggName
		contractID := contract.ContractID
		contract.mutex.Unlock()

		// Regenerate banners for contracts with a custom banner so they are
		// up to date in case the banner image was updated or never generated.
		if creatorID != "" {
			if bannerText != "" && eggName != "" {
				go bottools.GenerateBanner(contractID, eggName, bannerText, creatorID, guildID, "")
			}
		}
	}
}

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
			} else {
				log.Println("Unable to resend message." + err.Error())

			}
		}
	}
	return nil
}

// bumpCRMessages posts a new CR request message in each contract location, replacing the old one with a redirect.
func bumpCRMessages(s *discordgo.Session, contract *Contract) {
	contract.mutex.Lock()
	locations := make([]*LocationData, len(contract.Location))
	copy(locations, contract.Location)
	crIDs := make(map[string]string, len(contract.CRMessageIDs))
	maps.Copy(crIDs, contract.CRMessageIDs)
	contract.mutex.Unlock()

	for _, location := range locations {
		contract.mutex.Lock()
		components, _ := buildCRMessageComponents(contract, location.RoleMention)
		contract.mutex.Unlock()

		if components == nil {
			continue
		}

		var data discordgo.MessageSend
		data.Flags = discordgo.MessageFlagsIsComponentsV2
		data.Components = components
		data.AllowedMentions = &discordgo.MessageAllowedMentions{}
		newMsg, err := s.ChannelMessageSendComplex(location.ChannelID, &data)
		if err != nil {
			log.Printf("bumpCRMessages send error: contractHash=%s channelID=%s error=%v",
				contract.ContractHash, location.ChannelID, err)
			continue
		}

		contract.mutex.Lock()
		setChickenRunMessageID(contract, location.ChannelID, newMsg.ID)
		contract.mutex.Unlock()

		if existingMsgID := crIDs[location.ChannelID]; existingMsgID != "" {
			newMsgLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", location.GuildID, location.ChannelID, newMsg.ID)
			movedComponents := []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: fmt.Sprintf("-# Chicken Run request moved: [View updated message](%s)", newMsgLink)},
			}
			oldEdit := discordgo.NewMessageEdit(location.ChannelID, existingMsgID)
			oldEdit.Flags = discordgo.MessageFlagsIsComponentsV2
			oldEdit.AllowedMentions = &discordgo.MessageAllowedMentions{}
			oldEdit.Components = &movedComponents
			if _, err := s.ChannelMessageEditComplex(oldEdit); err != nil {
				log.Printf("bumpCRMessages edit old error: contractHash=%s channelID=%s messageID=%s error=%v",
					contract.ContractHash, location.ChannelID, existingMsgID, err)
			}
		}
	}
}

func refreshBoostListMessage(s *discordgo.Session, contract *Contract, updateSignupMessage bool) {
	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()
	// Edit the boost list in place
	for _, loc := range contract.Location {
		var components []discordgo.MessageComponent
		msgedit := discordgo.NewMessageEdit(loc.ChannelID, loc.ListMsgID)
		msgedit.Flags = discordgo.MessageFlagsIsComponentsV2
		listComponents := DrawBoostList(s, contract)
		components = append(components, listComponents...)
		buttonComponents := getContractReactionsComponents(contract)
		if len(buttonComponents) > 0 {
			components = append(components, buttonComponents...)
		}
		msgedit.Components = &components

		// Disable ALL pings during the run
		if contract.State != ContractStateSignup {
			msgedit.AllowedMentions = &discordgo.MessageAllowedMentions{
				Parse:       []discordgo.AllowedMentionType{},
				RepliedUser: false,
			}
		}

		// Full contract for speedrun
		msg, err := s.ChannelMessageEditComplex(msgedit)
		if err == nil {
			// This is an edit, it should be the same
			loc.ListMsgID = msg.ID
		}
		if updateSignupMessage {
			updateSignupReactionMessage(s, contract, loc)
		}
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
			buttonComponents := getContractReactionsComponents(contract)
			if len(buttonComponents) > 0 {
				components = append(components, buttonComponents...)
			}
			msgedit.Components = &components
			msgedit.Flags = discordgo.MessageFlagsIsComponentsV2
			_, err := s.ChannelMessageEditComplex(msgedit)
			if err != nil {
				log.Println("Unable to send this message." + err.Error())
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
			components = append(components, listComp...)
			buttonComponents := getContractReactionsComponents(contract)
			if len(buttonComponents) > 0 {
				components = append(components, buttonComponents...)
			}
			data.Components = components
			msg, err = s.ChannelMessageSendComplex(loc.ChannelID, &data)
			if err == nil {
				SetListMessageID(contract, loc.ChannelID, msg.ID)
			}
			drawn = true
		}
		if err != nil {
			log.Println("Unable to resend message." + err.Error())
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
					currentBoosterID := contract.currentBoosterID()
					if currentBoosterID == "" {
						continue
					}
					var name string
					var einame = farmerstate.GetEggIncName(currentBoosterID)
					if einame != "" {
						einame += " " // Add a space to this
					}

					if contract.Banker.CurrentBanker != "" {
						name = einame + contract.Boosters[contract.Banker.CurrentBanker].Mention
					} else {
						name = einame + contract.Boosters[currentBoosterID].Mention
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
