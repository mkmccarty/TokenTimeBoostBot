package boost

import (
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
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
func RedrawBoostList(client dc.Client, guildID string, channelID string) error {
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
			oldMsgID := loc.ListMsgID
			if oldMsgID != "" {
				if err := client.DeleteMessage(loc.ChannelID, oldMsgID); err != nil {
					log.Printf("RedrawBoostList: unable to delete old boost list message %s in channel %s for contract %s: %v",
						oldMsgID, loc.ChannelID, contract.ContractHash, err)
					time.AfterFunc(2*time.Second, func() {
						if retryErr := client.DeleteMessage(loc.ChannelID, oldMsgID); retryErr != nil {
							log.Printf("RedrawBoostList: retry delete old boost list message %s failed: %v", oldMsgID, retryErr)
						}
					})
				}
			}
			var components []dc.LayoutComponent
			listComp := DrawBoostList(contract)
			components = append(components, listComp...)
			components = append(components, buttonComponents...)
			msg, err := client.SendMessage(loc.ChannelID, dc.Message{
				Components:      components,
				AllowedMentions: &dc.AllowedMentions{},
			})
			if err == nil {
				SetListMessageID(contract, loc.ChannelID, msg.ID)
				contract.mutex.Lock()
				for _, booster := range contract.Boosters {
					booster.NonTokenMsgID = ""
				}
				contract.mutex.Unlock()
				//				addContractReactionsButtons(s, contract, loc.ChannelID, msg.ID)
			} else {
				log.Println("Unable to resend message." + err.Error())

			}
		}
	}
	return nil
}

// bumpCRMessages posts a new CR request message in each contract location, replacing the old one with a redirect.
func bumpCRMessages(client dc.Client, contract *Contract) {
	contract.mutex.Lock()
	locations := make([]*LocationData, len(contract.Location))
	copy(locations, contract.Location)
	crIDs := make(map[string]string, len(contract.CRMessageIDs))
	maps.Copy(crIDs, contract.CRMessageIDs)
	prevRequesters := append([]string(nil), contract.CRActiveRequesters...)
	contract.mutex.Unlock()

	for _, location := range locations {
		contract.mutex.Lock()
		msg, _ := buildCRMessage(contract, location.RoleMention)
		newRequesters := getActiveCRRequesters(contract)
		contract.mutex.Unlock()

		if msg.Content == "" && len(msg.Embeds) == 0 {
			continue
		}

		msg.AllowedMentions = &dc.AllowedMentions{}
		newMsg, err := client.SendMessage(location.ChannelID, msg)
		if err != nil {
			log.Printf("bumpCRMessages send error: contractHash=%s channelID=%s error=%v",
				contract.ContractHash, location.ChannelID, err)
			continue
		}

		setChickenRunMessageID(contract, location.ChannelID, newMsg.ID)
		contract.mutex.Lock()
		contract.CRNoticeCount++
		contract.CRActiveRequesters = newRequesters
		contract.mutex.Unlock()

		if existingMsgID := crIDs[location.ChannelID]; existingMsgID != "" {
			var droppedCompleted []string
			for _, reqID := range prevRequesters {
				if !slices.Contains(newRequesters, reqID) {
					_, missing, _ := buildChickenRunLists(contract, reqID)
					if len(missing) == 0 {
						droppedCompleted = append(droppedCompleted, reqID)
					}
				}
			}

			newMsgLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", location.GuildID, location.ChannelID, newMsg.ID)
			moveText := fmt.Sprintf("-# Chicken Run request moved: [View updated message](%s)", newMsgLink)
			var embeds []dc.Embed
			clearEmbeds := true
			if compText := formatCompletedCRRequestersText(contract, droppedCompleted); compText != "" {
				embeds = []dc.Embed{
					{
						Description: fmt.Sprintf("%s\n%s", compText, moveText),
						Color:       0x00ff00,
					},
				}
				clearEmbeds = false
				moveText = ""
			}
			if _, err := client.EditMessage(location.ChannelID, existingMsgID, dc.Message{
				Content:         moveText,
				Embeds:          embeds,
				ComponentsV1:    true,
				ClearComponents: true,
				ClearEmbeds:     clearEmbeds,
				AllowedMentions: &dc.AllowedMentions{},
			}); err != nil {
				fallbackText := moveText
				if len(embeds) > 0 {
					fallbackText = embeds[0].Description
				}
				movedComponents := []dc.LayoutComponent{
					dc.TextDisplay{Content: fallbackText},
				}
				if _, err2 := client.EditMessage(location.ChannelID, existingMsgID, dc.Message{
					Components:      movedComponents,
					ClearEmbeds:     true,
					AllowedMentions: &dc.AllowedMentions{},
				}); err2 != nil {
					log.Printf("bumpCRMessages edit old error: contractHash=%s channelID=%s messageID=%s error=%v",
						contract.ContractHash, location.ChannelID, existingMsgID, err2)
				}
			}
		}
	}
}

func refreshBoostListMessage(client dc.Client, contract *Contract, updateSignupMessage bool) {
	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()
	// Edit the boost list in place
	for _, loc := range contract.Location {
		var components []dc.LayoutComponent
		listComponents := DrawBoostList(contract)
		components = append(components, listComponents...)
		buttonComponents := getContractReactionsComponents(contract)
		if len(buttonComponents) > 0 {
			components = append(components, buttonComponents...)
		}

		edit := dc.Message{Components: components}

		// Disable ALL pings during the run
		if contract.State != ContractStateSignup {
			edit.AllowedMentions = &dc.AllowedMentions{}
		}

		// Full contract for speedrun
		if loc.ListMsgID != "" {
			msg, err := client.EditMessage(loc.ChannelID, loc.ListMsgID, edit)
			if err == nil {
				// This is an edit, it should be the same
				loc.ListMsgID = msg.ID
			} else {
				log.Printf("refreshBoostListMessage: failed to edit boost list message %s in channel %s for contract %s: %v",
					loc.ListMsgID, loc.ChannelID, contract.ContractHash, err)
			}
		}
		if updateSignupMessage {
			updateSignupReactionMessage(client, contract, loc)
		}
	}
}

func sendNextNotification(client dc.Client, contract *Contract, pingUsers bool) {
	// Start boosting contract
	drawn := false
	for _, loc := range contract.Location {
		var msg *dc.MessageRef
		var err error

		if contract.State == ContractStateSignup {
			var components []dc.LayoutComponent
			// Full contract for speedrun
			listComp := DrawBoostList(contract)
			components = append(components, listComp...)
			buttonComponents := getContractReactionsComponents(contract)
			if len(buttonComponents) > 0 {
				components = append(components, buttonComponents...)
			}
			_, err := client.EditMessage(loc.ChannelID, loc.ListMsgID, dc.Message{Components: components})
			if err != nil {
				log.Printf("sendNextNotification: unable to edit boost list message %s in channel %s for contract %s: %v",
					loc.ListMsgID, loc.ChannelID, contract.ContractHash, err)
			}
			updateSignupReactionMessage(client, contract, loc)

		} else {
			var components []dc.LayoutComponent

			// Unpin message once the contract is completed
			if contract.State == ContractStateArchive {
				_ = client.UnpinMessage(loc.ChannelID, loc.ReactionID)
			}
			oldMsgID := loc.ListMsgID
			if oldMsgID != "" {
				if err := client.DeleteMessage(loc.ChannelID, oldMsgID); err != nil {
					log.Printf("sendNextNotification: unable to delete old boost list message %s in channel %s for contract %s: %v",
						oldMsgID, loc.ChannelID, contract.ContractHash, err)
					time.AfterFunc(2*time.Second, func() {
						if retryErr := client.DeleteMessage(loc.ChannelID, oldMsgID); retryErr != nil {
							log.Printf("sendNextNotification: retry delete old boost list message %s failed: %v", oldMsgID, retryErr)
						}
					})
				}
			}

			// Compose the message without a Ping
			listComp := DrawBoostList(contract)
			components = append(components, listComp...)
			buttonComponents := getContractReactionsComponents(contract)
			if len(buttonComponents) > 0 {
				components = append(components, buttonComponents...)
			}
			msg, err = client.SendMessage(loc.ChannelID, dc.Message{
				Components:      components,
				AllowedMentions: &dc.AllowedMentions{},
			})
			if err == nil {
				SetListMessageID(contract, loc.ChannelID, msg.ID)
				contract.mutex.Lock()
				for _, booster := range contract.Boosters {
					booster.NonTokenMsgID = ""
				}
				contract.mutex.Unlock()
			}
			drawn = true
		}
		if err != nil {
			log.Printf("sendNextNotification: unable to send message in channel %s for contract %s: %v",
				loc.ChannelID, contract.ContractHash, err)
		}
		var str = ""
		if msg == nil {
			// Maybe this location is broken
			continue
		}

		mentionSuffix := ""
		if loc.RoleMention != "" {
			mentionSuffix = " " + loc.RoleMention
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

					str = fmt.Sprintf("Send tokens to **%s** %s", name, mentionSuffix)
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
							str = fmt.Sprintf("Send tokens to our volunteer sink **%s** %s", name, mentionSuffix)
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
			_, _ = client.SendMessage(loc.ChannelID, dc.Message{Content: str})
		} else if !drawn {
			_ = RedrawBoostList(client, loc.GuildID, loc.ChannelID)
		}
	}
	if pingUsers {
		notifyBellBoosters(client, contract)
	}
	/*
		if contract.State == ContractStateArchive {
			// Only purge the contract from memory if /calc isn't being used
			if contract.CalcOperations == 0 || time.Since(contract.CalcOperationTime).Minutes() > 20 {
				FinishContract(client, contract)
			}
		}
	*/
}

func updateSignupReactionMessage(client dc.Client, contract *Contract, loc *LocationData) {
	//if contract.State != ContractStateSignup {
	//	return
	//	}
	// Only want to update this when we have to change the Join button
	//if len(contract.Order) == contract.CoopSize || len(contract.Order) == (contract.CoopSize-1) {
	var components []dc.LayoutComponent
	msgID := loc.ReactionID
	if msgID == "" {
		return
	}
	// Full contract for speedrun
	contentStr, comp := GetSignupComponents(contract)
	components = append(components, dc.TextDisplay{
		Content: contentStr,
	})
	components = append(components, comp...)
	_, err := client.EditMessage(loc.ChannelID, msgID, dc.Message{Components: components})
	if err != nil {
		log.Printf("updateSignupReactionMessage: unable to edit message %s in channel %s for contract %s: %v",
			msgID, loc.ChannelID, contract.ContractHash, err)
	}
	//}
}
