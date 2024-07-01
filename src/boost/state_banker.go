package boost

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
	"github.com/rs/xid"
)

func buttonReactionBag(s *discordgo.Session, GuildID string, ChannelID string, contract *Contract, cUserID string) (bool, bool) {
	redraw := false
	if cUserID == contract.SRData.BoostingSinkUserID {
		var b, sink *Booster
		b = contract.Boosters[contract.Order[contract.BoostPosition]]
		sink = contract.Boosters[contract.SRData.BoostingSinkUserID]

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
			rSerial := xid.New().String()
			sSerial := xid.New().String()
			for i := 0; i < tokensToSend; i++ {
				b.Received = append(b.Received, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[cUserID].Nick, Serial: rSerial})
				contract.Boosters[cUserID].Sent = append(contract.Boosters[cUserID].Sent, TokenUnit{Time: time.Now(), Value: 0.0, UserID: contract.Boosters[b.UserID].Nick, Serial: sSerial})
			}
			track.ContractTokenMessage(s, ChannelID, b.UserID, track.TokenReceived, b.TokensReceived, contract.Boosters[cUserID].Nick, rSerial)
			track.ContractTokenMessage(s, ChannelID, cUserID, track.TokenSent, b.TokensReceived, contract.Boosters[b.UserID].Nick, sSerial)
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

func addReactionButtonsBanker(contract *Contract) ([]string, []string) {
	iconsRowA := []string{}
	iconsRowB := []string{} //mainly for alt icons
	iconsRowA = append(iconsRowA, []string{contract.TokenStr, "🐓", "💰"}...)
	iconsRowB = append(iconsRowB, contract.AltIcons...)
	return iconsRowA, iconsRowB
}
