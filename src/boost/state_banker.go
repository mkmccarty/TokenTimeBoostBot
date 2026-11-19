package boost

import (
	"fmt"
	"log"
	"slices"
	"time"
	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func buttonReactionBag(client dc.Client, GuildID string, ChannelID string, contract *Contract, cUserID string) (bool, bool) {
	redraw := false

	if _, members := contractLinkedGroup(contract, cUserID); slices.Contains(members, contract.Banker.BoostingSinkUserID) {
		cUserID = contract.Banker.BoostingSinkUserID
	}

	if cUserID == contract.Banker.BoostingSinkUserID {
		var b, sink *Booster
		b = contract.currentBooster()
		sink = contract.Boosters[cUserID]
		if b == nil {
			return false, redraw
		}

		if cUserID == b.UserID {
			// Current booster subtract number of tokens wanted
			log.Printf("Sink indicating they are boosting with %d tokens.\n", b.TokensWanted)
			//	sink.TokensReceived -= b.TokensWanted
			//	sink.TokensReceived = max(0, sink.TokensReceived) // Avoid missing self farmed tokens
			contract.TokenLog = append(contract.TokenLog, ei.TokenUnitLog{Time: time.Now(), Quantity: b.TokensWanted, FromUserID: cUserID, FromNick: contract.Boosters[cUserID].Nick, ToUserID: b.UserID, ToNick: b.Nick, Serial: uuid.NewV7().String(), Boost: true})
			sink.TokensReceived = getTokensReceivedFromLog(contract, sink.UserID) - getTokensSentFromLog(contract, sink.UserID)
		} else {
			log.Printf("Sink sent %d tokens to booster\n", b.TokensWanted)
			// Current booster number of tokens wanted
			// How many tokens does booster want?  Check to see if sink has that many
			tokensToSend := b.TokensWanted // If Sink is pressing 💰 they are assumed to be sending that many
			b.TokensReceived += tokensToSend
			sink.TokensReceived = getTokensReceivedFromLog(contract, sink.UserID) - getTokensSentFromLog(contract, sink.UserID)
			//sink.TokensReceived = max(0, sink.TokensReceived) // Avoid missing self farmed tokens
			// Record the Tokens as received
			tokenSerial := uuid.NewV7().String()
			contract.mutex.Lock()

			contract.TokenLog = append(contract.TokenLog, ei.TokenUnitLog{Time: time.Now(), Quantity: tokensToSend, FromUserID: cUserID, FromNick: contract.Boosters[cUserID].Nick, ToUserID: b.UserID, ToNick: b.Nick, Serial: tokenSerial, Boost: true})
			contract.mutex.Unlock()
			//if contract.BoostOrder == ContractOrderTVal {
			tval := bottools.GetTokenValue(time.Since(contract.StartTime).Seconds(), contract.EstimatedDuration.Seconds())
			contract.Boosters[cUserID].TokenValue += tval * float64(tokensToSend)
			contract.Boosters[b.UserID].TokenValue -= tval * float64(tokensToSend)

			// Set the estimated boost timings for the booster
			setEstimatedBoostTimings(b, contract)
			/*
				if contract.Style&ContractFlagDynamicTokens != 0 {
					// Determine the dynamic tokens
					determineDynamicTokens(contract)
				}
			*/
		}

		str := fmt.Sprintf("**%s** ", contract.Boosters[b.UserID].Mention)
		if ctrlID := contract.Boosters[b.UserID].AltController; ctrlID != "" {
			if ctrl := contract.Boosters[ctrlID]; ctrl != nil {
				altLabel := b.Nick
				if altLabel == "" {
					altLabel = b.Name
				}
				if altLabel == "" {
					altLabel = b.UserID
				}
				str = fmt.Sprintf("%s **(%s)** ", ctrl.Mention, altLabel)
			}
		}
		str += fmt.Sprintf("you've been sent %d tokens to boost with!", b.TokensWanted)

		_, _ = client.SendMessage(contract.Location[0].ChannelID, dc.Message{Content: str})

		_ = Boosting(client, GuildID, ChannelID)

		return false, redraw
	}
	return false, redraw
}
