package boost

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
	"github.com/rs/xid"
)

// ReactionAdd is called when a reaction is added to a message
func ReactionAdd(s *discordgo.Session, r *discordgo.MessageReaction) string {
	// Find the message
	returnVal := ""
	var msg, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return returnVal
	}
	redraw := false
	emojiName := r.Emoji.Name

	var contract = FindContractByMessageID(r.ChannelID, r.MessageID)
	if contract == nil {
		return returnVal
	}

	defer saveData(Contracts)

	// If the user is not in the contract then they can join with a farmer reaction
	if !userInContract(contract, r.UserID) {
		var farmerSlice = []string{
			"🧑‍🌾", "🧑🏻‍🌾", "🧑🏼‍🌾", "🧑🏽‍🌾", "🧑🏾‍🌾", "🧑🏿‍🌾", // farmer
			"👩‍🌾", "👩🏻‍🌾", "👩🏼‍🌾", "👩🏽‍🌾", "👩🏾‍🌾", "👩🏿‍🌾", // woman farmer
			"👨‍🌾", "👨🏻‍🌾", "👨🏼‍🌾", "👨🏼‍🌾", "👨🏾‍🌾", "👨🏿‍🌾", // man farmer
		}

		if slices.Contains(farmerSlice, emojiName) {
			err := JoinContract(s, r.GuildID, r.ChannelID, r.UserID, false)
			if err == nil {
				redraw = true
			}
		}
	}

	// If the user is in the contract then they can set their token count
	if userInContract(contract, r.UserID) {
		var numberSlice = []string{"0️⃣", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}
		if slices.Contains(numberSlice, emojiName) {
			var b = contract.Boosters[r.UserID]
			if b != nil {
				var tokenCount = slices.Index(numberSlice, emojiName)
				farmerstate.SetTokens(r.UserID, tokenCount)
				b.TokensWanted = tokenCount
				redraw = true
			}
		}
	}

	if userInContract(contract, r.UserID) || creatorOfContract(contract, r.UserID) {

		// Isolate Speedrun reactions for safety
		if contract.Speedrun && contract.SRData.SpeedrunState == SpeedrunStateCRT {
			return speedrunReactions(s, r, contract)
		}
		if contract.Speedrun && contract.SRData.SpeedrunStyle == SpeedrunStyleWonky &&
			contract.SRData.SpeedrunState == SpeedrunStateBoosting {
			return speedrunReactions(s, r, contract)
		}
		if contract.Speedrun && contract.SRData.SpeedrunState == SpeedrunStatePost {
			return speedrunReactions(s, r, contract)
		}

		// if contract state is waiting and the reaction is a 🏁 finish the contract
		if r.Emoji.Name == "🏁" {
			if contract.State == ContractStateWaiting {
				var votingElection = (msg.Reactions[0].Count - 1) >= 2
				if votingElection || creatorOfContract(contract, r.UserID) {
					contract.State = ContractStateCompleted
					contract.EndTime = time.Now()
					sendNextNotification(s, contract, true)
				}
				return returnVal
			}

			if !contract.Speedrun && contract.State == ContractStateCompleted && creatorOfContract(contract, r.UserID) {
				// Coordinator can end the contract
				contract.State = ContractStateArchive
				sendNextNotification(s, contract, true)
				return returnVal
			}
		}

		if contract.State != ContractStateSignup && contract.BoostPosition < len(contract.Order) {

			// If Rocket reaction on Boost List, only that boosting user can apply a reaction
			if r.Emoji.Name == boostIconName && contract.State == ContractStateStarted {
				var votingElection = (msg.Reactions[0].Count - 1) >= 2

				userID := r.UserID
				if contract.Speedrun {
					if contract.Boosters[r.UserID] != nil && len(contract.Boosters[r.UserID].Alts) > 0 {
						// Find the most recent boost time among the user and their alts
						for _, altID := range contract.Boosters[r.UserID].Alts {
							if altID == contract.Order[contract.BoostPosition] {
								userID = altID
								break
							}
						}
					}
				}

				if userID == contract.Order[contract.BoostPosition] || votingElection || creatorOfContract(contract, r.UserID) {
					//contract.mutex.Unlock()
					Boosting(s, r.GuildID, r.ChannelID)
				}
				return returnVal
			}

			// Catch a condition where BoostPosition got set wrongly
			if contract.BoostPosition >= len(contract.Order) || contract.BoostPosition < 0 {
				if len(contract.Order) > 0 {
					contract.BoostPosition = len(contract.Order) - 1
				} else {
					contract.BoostPosition = 0
				}
				if contract.State == ContractStateStarted {
					for i, el := range contract.Order {
						if contract.Boosters[el].BoostState == BoostStateTokenTime {
							contract.BoostPosition = i
							break
						}
					}
				}
			}

			// Reaction for current booster to change places
			if r.UserID == contract.Order[contract.BoostPosition] || creatorOfContract(contract, r.UserID) {
				if (contract.BoostPosition + 1) < len(contract.Order) {
					if r.Emoji.Name == "🔃" {
						//contract.mutex.Unlock()
						SkipBooster(s, r.GuildID, r.ChannelID, "")
						return returnVal
					}
				}
			}

			{
				// Reaction to jump to end
				if r.Emoji.Name == "⤵️" {
					//contract.mutex.Unlock()
					var uid = r.UserID // using a variable here for debugging
					if contract.Boosters[uid].BoostState == BoostStateTokenTime {
						currentBoosterPosition := findNextBooster(contract)
						err := MoveBooster(s, r.GuildID, r.ChannelID, contract.CreatorID[0], uid, len(contract.Order), currentBoosterPosition == -1)
						if err == nil && currentBoosterPosition != -1 {
							ChangeCurrentBooster(s, r.GuildID, r.ChannelID, contract.CreatorID[0], contract.Order[currentBoosterPosition], true)
							return returnVal
						}
					} else if contract.Boosters[uid].BoostState == BoostStateUnboosted {
						MoveBooster(s, r.GuildID, r.ChannelID, contract.CreatorID[0], uid, len(contract.Order), true)
					}
				}
				// Reaction to indicate you need to go now
				if r.Emoji.Name == "🚽" && contract.Boosters[r.UserID].BoostState == BoostStateUnboosted {
					// Move Booster position is 1 based, so we need to add 2 to the current position
					err := MoveBooster(s, r.GuildID, r.ChannelID, contract.CreatorID[0], r.UserID, contract.BoostPosition+2, true)
					if err == nil {
						s.ChannelMessageSend(r.ChannelID, contract.Boosters[r.UserID].Name+" expressed a desire to go next!")
						returnVal = "!gonow"
					}
				}
			}

			if contract.State == ContractStateWaiting && r.Emoji.Name == "🔃" {
				contract.State = ContractStateCompleted
				contract.EndTime = time.Now()
				sendNextNotification(s, contract, true)
				return returnVal
			}
		}

		if r.Emoji.Name == "🐓" {
			// Indicate that a farmer is ready for chicken runs
			userID := r.UserID
			if len(contract.Boosters[r.UserID].Alts) > 0 {
				ids := append(contract.Boosters[r.UserID].Alts, r.UserID)
				for _, id := range contract.Order {
					if slices.Index(ids, id) != -1 {
						alt := contract.Boosters[id]
						if alt.BoostState == BoostStateBoosted && alt.RunChickensTime.IsZero() {
							userID = id
							break
						}
					}
				}
			}

			contract.Boosters[userID].RunChickensTime = time.Now()
			str := fmt.Sprintf("%s **%s** is ready for chicken runs, check for incoming trucks before visiting.", contract.Location[0].ChannelPing, contract.Boosters[userID].Mention)
			var data discordgo.MessageSend
			data.Content = str
			msg, _ := s.ChannelMessageSendComplex(contract.Location[0].ChannelID, &data)
			s.MessageReactionAdd(msg.ChannelID, msg.ID, contract.ChickenRunEmoji) // Indicate Chicken Run
			redraw = true
		}

		tokenReactionStr := "token"
		userID := r.UserID
		// Special handling for alt icons representing token reactions
		if slices.Index(contract.AltIcons, r.Emoji.Name) != -1 {
			idx := slices.Index(contract.Boosters[r.UserID].AltsIcons, r.Emoji.Name)
			if idx != -1 {
				userID = contract.Boosters[r.UserID].Alts[idx]
				tokenReactionStr = r.Emoji.Name
			}
		}

		// Token reaction handling
		if strings.ToLower(r.Emoji.Name) == tokenReactionStr {
			if r.Emoji.ID != "" {
				emojiName = r.Emoji.Name + ":" + r.Emoji.ID
			}
			if contract.State == ContractStateWaiting || contract.State == ContractStateCompleted {
				if contract.VolunteerSink != "" {
					rSerial := xid.New().String()
					sSerial := xid.New().String()

					sink := contract.Boosters[contract.VolunteerSink]
					sink.Received = append(sink.Received, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[userID].Nick, Serial: rSerial})
					track.ContractTokenMessage(s, r.ChannelID, sink.UserID, track.TokenReceived, 1, userID, rSerial)
					// Record who sent the token
					contract.Boosters[userID].Sent = append(contract.Boosters[r.UserID].Sent, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[sink.UserID].Nick, Serial: sSerial})
					track.ContractTokenMessage(s, r.ChannelID, userID, track.TokenSent, 1, sink.UserID, sSerial)
				}
			} else if contract.BoostPosition < len(contract.Order) {
				var b = contract.Boosters[contract.Order[contract.BoostPosition]]

				b.TokensReceived++
				if userID != b.UserID {
					// Record the Tokens as received
					rSerial := xid.New().String()
					sSerial := xid.New().String()
					b.Received = append(b.Received, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[userID].Nick, Serial: rSerial})
					track.ContractTokenMessage(s, r.ChannelID, b.UserID, track.TokenReceived, 1, contract.Boosters[userID].Nick, rSerial)

					// Record who sent the token
					if contract.Boosters[userID] != nil {
						// Make sure this isn't an admin user who's sending on behalf of an alt
						contract.Boosters[userID].Sent = append(contract.Boosters[userID].Sent, TokenUnit{Time: time.Now(), Value: 0.0, UserID: b.Nick, Serial: sSerial})
					}
					track.ContractTokenMessage(s, r.ChannelID, userID, track.TokenSent, 1, b.Nick, sSerial)
				} else {
					b.TokensFarmedTime = append(b.TokensFarmedTime, time.Now())
					track.FarmedToken(s, r.ChannelID, userID)
				}

				if b.TokensReceived >= b.TokensWanted && userID == b.Name && b.AltController == "" {
					// Guest farmer auto boosts
					Boosting(s, r.GuildID, r.ChannelID)
				}

				redraw = true
			}
		}
	} else {
		// Custon token reaction from user not in contract
		if strings.ToLower(r.Emoji.Name) == "token" {
			emojiName = r.Emoji.Name + ":" + r.Emoji.ID
			redraw = true
		}
	}

	// Remove extra added emoji
	err = s.MessageReactionRemove(r.ChannelID, r.MessageID, emojiName, r.UserID)
	if err != nil {
		fmt.Println(err, emojiName)
		s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
	}

	if redraw {
		refreshBoostListMessage(s, contract)
	}

	if r.Emoji.Name == "❓" {
		for _, loc := range contract.Location {
			outputStr := "## Boost Bot Icon Meanings\n\n"
			outputStr += "See 📌 message to join the contract.\nSet your number of boost tokens there or "
			outputStr += "add a 4️⃣ to 🔟 reaction to the boost list message.\n"
			outputStr += "Active booster reaction of " + boostIcon + " to when spending tokens to boost. Multiple " + boostIcon + " votes by others in the contract will also indicate a boost.\n"
			outputStr += "Farmers react with " + loc.TokenStr + " when sending tokens.\n"
			//outputStr += "Active Booster can react with ➕ or ➖ to adjust number of tokens needed.\n"
			outputStr += "Active booster reaction of 🔃 to exchange position with the next booster.\n"
			outputStr += "Reaction of ⤵️ to move yourself to last in the current boost order.\n"
			outputStr += "Reaction of 🐓 when you're ready for others to run chickens on your farm.\n"
			outputStr += "Anyone can add a 🚽 reaction to express your urgency to boost next.\n"
			outputStr += "Additional help through the **/help** command.\n"
			s.ChannelMessageSend(loc.ChannelID, outputStr)
		}
	}

	return returnVal
}

// ReactionRemove handles a user removing a reaction from a message
func ReactionRemove(s *discordgo.Session, r *discordgo.MessageReaction) {
	var _, err = s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		return
	}

	var contract = FindContractByMessageID(r.ChannelID, r.MessageID)
	if contract == nil {
		return
	}

	//contract.mutex.Lock()
	//defer contract.mutex.Unlock()
	defer saveData(Contracts)

	if !userInContract(contract, r.UserID) {
		return
	}
}
