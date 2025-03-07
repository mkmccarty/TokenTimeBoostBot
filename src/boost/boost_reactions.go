package boost

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/moby/moby/pkg/namesgenerator"
)

// ReactionAdd is called when a reaction is added to a message
func ReactionAdd(s *discordgo.Session, r *discordgo.MessageReaction) string {
	// Find the message
	keepReaction := false
	returnVal := ""
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
				if (ContractFlagDynamicTokens+ContractFlag8Tokens+ContractFlag6Tokens)&contract.Style == 0 {
					farmerstate.SetTokens(r.UserID, tokenCount)
				}
				b.TokensWanted = tokenCount
				redraw = true
			}
		}
	}

	if userInContract(contract, r.UserID) || creatorOfContract(s, contract, r.UserID) {
		contract.LastInteractionTime = time.Now()

		switch contract.State {
		case ContractStateCRT:
			reactionCRT(s, r, contract)
		case ContractStateBanker:
			return speedrunReactions(s, r, contract)
		case ContractStateCompleted:
			return speedrunReactions(s, r, contract)
		}

		if contract.State != ContractStateSignup && contract.BoostPosition < len(contract.Order) {

			// Catch a condition where BoostPosition got set wrongly
			if contract.BoostPosition >= len(contract.Order) || contract.BoostPosition < 0 {
				if len(contract.Order) > 0 {
					contract.BoostPosition = len(contract.Order) - 1
				} else {
					contract.BoostPosition = 0
				}
				if contract.State == ContractStateFastrun {
					for i, el := range contract.Order {
						if contract.Boosters[el].BoostState == BoostStateTokenTime {
							contract.BoostPosition = i
							break
						}
					}
				}
			}

			switch r.Emoji.Name {
			case boostIconName:
				if r.MessageID == contract.Location[0].ListMsgID {
					result := buttonReactionBoost(s, r.GuildID, r.ChannelID, contract, r.UserID)
					if result {
						return returnVal
					}
				}
			case "🔃":
				result := buttonReactionSwap(s, r.GuildID, r.ChannelID, contract, r.UserID)
				if result {
					return returnVal
				}
			case "⤵️":
				willReturn := false
				willReturn, redraw = buttonReactionLast(s, r.GuildID, r.ChannelID, contract, r.UserID)
				if willReturn {
					return returnVal
				}
			case "🚽":
				if contract.Boosters[r.UserID].BoostState == BoostStateUnboosted {
					// Move Booster position is 1 based, so we need to add 2 to the current position
					err := MoveBooster(s, r.GuildID, r.ChannelID, contract.CreatorID[0], r.UserID, contract.BoostPosition+2, true)
					if err == nil {
						_, _ = s.ChannelMessageSend(r.ChannelID, contract.Boosters[r.UserID].Name+" expressed a desire to go next!")
						returnVal = "!gonow"
					}
				}
			}
		}

		// Anyone can use these reactions
		switch r.Emoji.Name {
		case "🌊":
			if time.Since(contract.ThreadRenameTime) < 3*time.Minute {
				msg, err := s.ChannelMessageSend(r.ChannelID, fmt.Sprintf("🌊 thread renaming is on cooldown, try again <t:%d:R>", contract.ThreadRenameTime.Add(3*time.Minute).Unix()))
				if err == nil {
					time.AfterFunc(10*time.Second, func() {
						err := s.ChannelMessageDelete(msg.ChannelID, msg.ID)
						if err != nil {
							log.Println(err)
						}
					})
				}
			} else {
				UpdateThreadName(s, contract)
			}

		case "🐓":
			if userInContract(contract, r.UserID) {
				redraw, _ = buttonReactionRunChickens(s, contract, r.UserID)
			}
		case "🐿️":
			if creatorOfContract(s, contract, r.UserID) {
				for i := len(contract.Order); i < contract.CoopSize; i++ {
					go func() {
						_ = JoinContract(s, r.GuildID, r.ChannelID, namesgenerator.GetRandomName(0), false)
					}()
				}
			}
		}

		// Token reaction handling
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

		if strings.ToLower(r.Emoji.Name) == tokenReactionStr {
			_, redraw = buttonReactionToken(s, r.GuildID, r.ChannelID, contract, userID, 1)
		}
	} else {
		keepReaction = false
	}

	// Remove extra added emoji
	if !keepReaction {
		go RemoveAddedReaction(s, r)
	}

	if redraw {
		refreshBoostListMessage(s, contract)
	}

	if r.Emoji.Name == "❓" {
		go func() {
			runReady, _, _ := ei.GetBotEmoji("runready")
			outputStr := "## Boost Bot Icon Meanings\n\n"
			outputStr += "See 📌 message to join the contract.\nSet your number of boost tokens there or "
			outputStr += "add a 4️⃣ to 🔟 reaction to the boost list message.\n"
			outputStr += "Active booster reaction of " + boostIcon + " to when spending tokens to boost. Multiple " + boostIcon + " votes by others in the contract will also indicate a boost.\n"
			outputStr += "Farmers react with " + contract.TokenStr + " when sending tokens.\n"
			//outputStr += "Active Booster can react with ➕ or ➖ to adjust number of tokens needed.\n"
			outputStr += "Active booster reaction of 🔃 to exchange position with the next booster.\n"
			outputStr += "Reaction of ⤵️ to move yourself to last in the current boost order.\n"
			outputStr += "Reaction of " + runReady + " when you're ready for others to run chickens on your farm.\n"
			outputStr += "Anyone can add a 🚽 reaction to express your urgency to boost next.\n"
			outputStr += "Additional help through the **/help** command.\n"

			for _, loc := range contract.Location {
				_, _ = s.ChannelMessageSend(loc.ChannelID, outputStr)
			}
		}()
	}

	return returnVal
}

// RemoveAddedReaction removes an added reaction from a message so it can be reactivated
func RemoveAddedReaction(s *discordgo.Session, r *discordgo.MessageReaction) {
	var emojiName = r.Emoji.Name

	if r.Emoji.ID != "" {
		emojiName = r.Emoji.Name + ":" + r.Emoji.ID
	}

	err := s.MessageReactionRemove(r.ChannelID, r.MessageID, emojiName, r.UserID)
	if err != nil {
		fmt.Println(err, emojiName)
		_ = s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
	}

}

// ReactionRemove handles a user removing a reaction from a message
func ReactionRemove(s *discordgo.Session, r *discordgo.MessageReaction) {
	// Don't need to track removal of reactions at this point
}
