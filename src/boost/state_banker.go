package boost

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
	"github.com/rs/xid"
)

func buttonReactionBag(s *discordgo.Session, GuildID string, ChannelID string, contract *Contract, cUserID string) (bool, bool) {
	redraw := false

	if contract.Boosters[cUserID] != nil && len(contract.Boosters[cUserID].Alts) > 0 {
		// Find the most recent boost time among the user and their alts
		for _, altID := range contract.Boosters[cUserID].Alts {
			if altID == contract.Banker.BoostingSinkUserID {
				cUserID = altID
				break
			}
		}
	}

	if cUserID == contract.Banker.BoostingSinkUserID || cUserID == contract.Banker.CrtSinkUserID {
		var b, sink *Booster
		b = contract.Boosters[contract.Order[contract.BoostPosition]]
		// Sink could be CRT Sink or Boosting Sink
		sink = contract.Boosters[cUserID]

		// If this is the CRT then the Bag indicates the sink is boosting
		if contract.State == ContractStateCRT {
			b = sink
		}

		if cUserID == b.UserID {
			// Current booster subtract number of tokens wanted
			log.Printf("Sink indicating they are boosting with %d tokens.\n", b.TokensWanted)
			sink.TokensReceived -= b.TokensWanted
			sink.TokensReceived = max(0, sink.TokensReceived) // Avoid missing self farmed tokens
		} else {
			log.Printf("Sink sent %d tokens to booster\n", b.TokensWanted)
			// Current booster number of tokens wanted
			// How many tokens does booster want?  Check to see if sink has that many
			tokensToSend := b.TokensWanted // If Sink is pressing 💰 they are assumed to be sending that many
			b.TokensReceived += tokensToSend
			sink.TokensReceived -= tokensToSend
			sink.TokensReceived = max(0, sink.TokensReceived) // Avoid missing self farmed tokens
			// Record the Tokens as received
			tokenSerial := xid.New().String()
			track.ContractTokenMessage(s, ChannelID, b.UserID, track.TokenReceived, b.TokensReceived, contract.Boosters[cUserID].Nick, tokenSerial)
			track.ContractTokenMessage(s, ChannelID, cUserID, track.TokenSent, b.TokensReceived, contract.Boosters[b.UserID].Nick, tokenSerial)
			contract.mutex.Lock()

			contract.TokenLog = append(contract.TokenLog, ei.TokenUnitLog{Time: time.Now(), Quantity: tokensToSend, FromUserID: cUserID, FromNick: contract.Boosters[cUserID].Nick, ToUserID: b.UserID, ToNick: b.Nick, Serial: tokenSerial})
			contract.mutex.Unlock()
			if contract.BoostOrder == ContractOrderTVal {
				tval := getTokenValue(time.Since(contract.StartTime).Seconds(), contract.EstimatedDuration.Seconds())
				contract.Boosters[cUserID].TokenValue += tval * float64(tokensToSend)
				contract.Boosters[b.UserID].TokenValue -= tval * float64(tokensToSend)
				// Don't reorder on the bag send as we need a tiny amount of stability for the send to get to the correct person
				//reorderBoosters(contract)
			}
			if contract.Style&ContractFlagDynamicTokens != 0 {
				// Determine the dynamic tokens
				determineDynamicTokens(contract)
			}
		}

		str := fmt.Sprintf("**%s** ", contract.Boosters[b.UserID].Mention)
		if contract.Boosters[b.UserID].AltController != "" {
			str = fmt.Sprintf("%s **(%s)** ", contract.Boosters[contract.Boosters[b.UserID].AltController].Mention, b.UserID)
		}
		str += fmt.Sprintf("you've been sent %d tokens to boost with!", b.TokensWanted)

		_, _ = s.ChannelMessageSend(contract.Location[0].ChannelID, str)

		_ = Boosting(s, GuildID, ChannelID)

		return false, redraw
	}
	return false, redraw
}
