package boost

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// ReactionAdd is called when a reaction is added to a message
func ReactionAdd(client dc.Client, e *dc.ReactionEvent) string {
	// Find the message
	keepReaction := false
	returnVal := ""
	redraw := false
	emojiName := e.EmojiName()

	var contract = FindContractByMessageID(e.ChannelID(), e.MessageID())
	if contract == nil {
		return returnVal
	}

	defer saveData(contract.ContractHash)

	// If the user is not in the contract then they can join with a farmer reaction
	if !UserInContract(contract, e.UserID()) {
		var farmerSlice = []string{
			"🧑‍🌾", "🧑🏻‍🌾", "🧑🏼‍🌾", "🧑🏽‍🌾", "🧑🏾‍🌾", "🧑🏿‍🌾", // farmer
			"👩‍🌾", "👩🏻‍🌾", "👩🏼‍🌾", "👩🏽‍🌾", "👩🏾‍🌾", "👩🏿‍🌾", // woman farmer
			"👨‍🌾", "👨🏻‍🌾", "👨🏼‍🌾", "👨🏼‍🌾", "👨🏾‍🌾", "👨🏿‍🌾", // man farmer
		}

		if slices.Contains(farmerSlice, emojiName) {
			err := JoinContract(client, e.GuildID(), e.ChannelID(), e.UserID(), false)
			if err == nil {
				redraw = true
			}
		}
	}

	// If the user is in the contract then they can set their token count
	if UserInContract(contract, e.UserID()) {
		var numberSlice = []string{"0️⃣", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}
		if slices.Contains(numberSlice, emojiName) {
			var b = contract.Boosters[e.UserID()]
			if b != nil {
				var tokenCount = slices.Index(numberSlice, emojiName)
				if (ContractFlagDynamicTokens+ContractFlag8Tokens+ContractFlag6Tokens+ContractFlag4Tokens+ContractFlagThresholdTokens)&contract.Style == 0 {
					farmerstate.SetTokens(e.UserID(), tokenCount)
				}
				b.TokensWanted = tokenCount
				redraw = true
			}
		}
	}

	if UserInContract(contract, e.UserID()) || creatorOfContract(client, contract, e.UserID()) {
		contract.LastInteractionTime = time.Now()

		if contract.State == ContractStateSignup {
			switch e.EmojiName() {
			case "🏎️", "🏎":
				err := SendSandboxDM(client, contract, e.UserID())
				if err != nil {
					u, dmErr := client.CreateUserChannel(e.UserID())
					if dmErr == nil {
						_, _ = client.SendMessage(u.ID, dc.Message{Content: fmt.Sprintf("Unable to generate SR Sandbox link for %s/%s: %v", contract.ContractID, contract.CoopID, err)})
					}
				}
			}
		}

		switch contract.State {
		case ContractStateBanker:
			return speedrunReactions(client, e, contract)
		case ContractStateCompleted:
			return speedrunReactions(client, e, contract)
		}

		currentBoosterID := contract.currentBoosterID()
		if contract.State != ContractStateSignup && currentBoosterID != "" {

			currentBoosterIdx := contract.currentBoosterOrderIndex()
			if currentBoosterIdx < 0 && len(contract.Order) > 0 {
				nextID := findNextBoosterID(contract)
				if nextID != "" {
					contract.setCurrentBoosterByUserIDWithStart(nextID)
					currentBoosterIdx = contract.currentBoosterOrderIndex()
				}
			}

			switch e.EmojiName() {
			case boostIconName:
				if e.MessageID() == contract.Location[0].ListMsgID {
					result := buttonReactionBoost(client, e.GuildID(), e.ChannelID(), contract, e.UserID())
					if result {
						return returnVal
					}
				}
			case "🔃":
				result := buttonReactionSwap(client, e.GuildID(), e.ChannelID(), contract, e.UserID())
				if result {
					return returnVal
				}
			case "⤵️":
				willReturn := false
				willReturn, redraw = buttonReactionLast(client, e.GuildID(), e.ChannelID(), contract, e.UserID())
				if willReturn {
					return returnVal
				}
			case "🚽":
				if contract.Boosters[e.UserID()].BoostState == BoostStateUnboosted {
					// Bounds check: ensure currentBoosterIdx is valid before using it
					if currentBoosterIdx < 0 {
						_, _ = client.SendMessage(e.ChannelID(), dc.Message{Content: "Unable to move booster right now because the current booster position could not be determined."})
					} else {
						// Move Booster position is 1 based, so we need to add 2 to the current position
						err := MoveBooster(client, e.GuildID(), e.ChannelID(), contract.CreatorID[0], e.UserID(), currentBoosterIdx+2, true)
						if err == nil {
							_, _ = client.SendMessage(e.ChannelID(), dc.Message{Content: contract.Boosters[e.UserID()].Name + " expressed a desire to go next!"})
							returnVal = "!gonow"
						}
					}
				}
			}
		}

		// Anyone can use these reactions
		switch e.EmojiName() {
		case "🌊":
			if time.Since(contract.ThreadRenameTime) < 30*time.Second {
				msg, err := client.SendMessage(e.ChannelID(), dc.Message{Content: fmt.Sprintf("🌊 thread renaming is on cooldown, try again <t:%d:R>", contract.ThreadRenameTime.Add(30*time.Second).Unix())})
				if err == nil {
					time.AfterFunc(10*time.Second, func() {
						err := client.DeleteMessage(msg.ChannelID, msg.ID)
						if err != nil {
							log.Println(err)
						}
					})
				}
			} else {
				UpdateThreadName(client, contract)
			}
		case "⏱️":
			if contract.State != ContractStateCompleted {
				msg, err := client.SendMessage(e.ChannelID(), dc.Message{
					Content:   "⏱️ can only be used after the contract completes boosting.",
					Ephemeral: true,
				})
				if err == nil {
					time.AfterFunc(10*time.Second, func() {
						err := client.DeleteMessage(msg.ChannelID, msg.ID)
						if err != nil {
							log.Println(err)
						}
					})
				}

			} else {
				if time.Since(contract.EstimateUpdateTime) < 2*time.Minute {
					msg, err := client.SendMessage(e.ChannelID(), dc.Message{
						Content:   fmt.Sprintf("⏱️ duration update on cooldown, try again <t:%d:R>", contract.ThreadRenameTime.Add(10*time.Second).Unix()),
						Ephemeral: true,
					})
					if err == nil {
						time.AfterFunc(10*time.Second, func() {
							err := client.DeleteMessage(msg.ChannelID, msg.ID)
							if err != nil {
								log.Println(err)
							}
						})
					}
				} else {
					log.Print("Updating estimated time")
					contract.EstimateUpdateTime = time.Now()
					go updateEstimatedTime(client, e.ChannelID(), contract, true, e.UserID())
				}
			}
		case "🐓":
			if UserInContract(contract, e.UserID()) {
				redraw, _ = buttonReactionRunChickens(client, contract, e.UserID())
			}
		case "🐿️":
			if creatorOfContract(client, contract, e.UserID()) {
				for i := len(contract.Order); i < contract.CoopSize; i++ {
					_, err := AddFarmerToContract(client, contract, e.GuildID(), e.ChannelID(), bottools.GetRandomName(0), contract.BoostOrder, true, false)
					if err != nil {
						log.Println(err)
					}
				}
				redraw = true
			}
		}

		// Token reaction handling
		tokenReactionStr := "token"
		userID := e.UserID()

		if strings.ToLower(e.EmojiName()) == tokenReactionStr {
			_, redraw = buttonReactionToken(client, e.GuildID(), e.ChannelID(), contract, userID, 1, "")
		}
	} else {
		keepReaction = false
	}

	// Remove extra added emoji
	if !keepReaction {
		go RemoveAddedReaction(client, e)
	}

	if redraw {
		refreshBoostListMessage(client, contract, false)
	}

	if e.EmojiName() == "❓" {
		contract.HelpGuidanceUntil = time.Now().Add(10 * time.Minute)
		saveData(contract.ContractHash)
		refreshBoostListMessage(client, contract, false)

		go func() {
			runReady, _, _ := ei.GetBotEmoji("runready")
			outputStr := "## Boost Bot Icon Meanings\n\n"
			outputStr += "See 📌 message to join the contract.\nSet your number of boost tokens there or "
			outputStr += "add a 4️⃣ to 🔟 reaction to the boost list message.\n"
			if contract.Style&ContractFlagBanker == 0 {
				outputStr += "Active booster must react with " + boostIcon + " when spending tokens to boost and to advance the list. Multiple " + boostIcon + " votes by others in the contract will also move to the next booster.\n"
			} else {
				outputStr += "The banker will indicate sending tokens with 💰 which will advance the list.\n"
			}
			outputStr += "Farmers react with " + contract.TokenStr + " when sending tokens.\n"
			//outputStr += "Active Booster can react with ➕ or ➖ to adjust number of tokens needed.\n"
			outputStr += "Active booster reaction of 🔃 to exchange position with the next booster.\n"
			outputStr += "Reaction of ⤵️ to move yourself to last in the current boost order.\n"
			outputStr += "Reaction of " + runReady + " when you're ready for others to run chickens on your farm.\n"
			outputStr += "Anyone can add a 🚽 reaction to express your urgency to boost next.\n"
			outputStr += "Additional help through the **/help** command.\n"

			for _, loc := range contract.Location {
				_, _ = client.SendMessage(loc.ChannelID, dc.Message{Content: outputStr})
			}
		}()
	}

	return returnVal
}

func updateEstimatedTime(client dc.Client, channelID string, contract *Contract, displayMsg bool, userID string) {
	if !displayMsg {
		eeidOverride := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		coopStartTime, coopDurationSeconds, err := ei.GetCoopStatusStartTimeAndDuration(contract.ContractID, contract.CoopID, eeidOverride)
		if err == nil {
			contract.StartTime = coopStartTime
			contract.EstimatedDuration = time.Duration(coopDurationSeconds) * time.Second
			contract.EstimateUpdateTime = time.Now()
			refreshBoostListMessage(client, contract, false)
		}
		return
	}
	msg, msgErr := client.SendMessage(channelID, dc.Message{
		Content:   "⏱️ reaction received, updating contract duration.",
		Ephemeral: true,
	})
	eeidOverride := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	coopStartTime, coopDurationSeconds, err := ei.GetCoopStatusStartTimeAndDuration(contract.ContractID, contract.CoopID, eeidOverride)
	if err == nil {
		if msgErr == nil {
			_ = client.DeleteMessage(msg.ChannelID, msg.ID)
		}
		contract.StartTime = coopStartTime
		contract.EstimatedDuration = time.Duration(coopDurationSeconds) * time.Second
		contract.EstimateUpdateTime = time.Now()
		refreshBoostListMessage(client, contract, false)
	}
}

// RemoveAddedReaction removes an added reaction from a message so it can be reactivated
func RemoveAddedReaction(client dc.Client, e *dc.ReactionEvent) {
	emojiRef := e.EmojiRef()

	err := client.RemoveMessageReaction(e.ChannelID(), e.MessageID(), emojiRef, e.UserID())
	if err != nil {
		log.Println(err, emojiRef)
		_ = client.RemoveMessageReaction(e.ChannelID(), e.MessageID(), e.EmojiName(), e.UserID())
	}
}

// ReactionRemove handles a user removing a reaction from a message
func ReactionRemove(_ dc.Client, _ *dc.ReactionEvent) {
	// Don't need to track removal of reactions at this point
}
