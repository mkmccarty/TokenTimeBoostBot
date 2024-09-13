package boost

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
)

// HandleTokenCommand takes the main command and adds the current contract to the message
func HandleTokenCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var contractID string
	var coopID string
	startTime := time.Now()
	var pastTokens *[]ei.TokenUnitLog
	contract := FindContract(i.ChannelID)
	if contract != nil {
		contractID = contract.ContractID
		coopID = contract.CoopID
		pastTokens = &contract.TokenLog
		startTime = contract.StartTime
	}
	track.HandleTokenCommand(s, i, contractID, coopID, startTime, pastTokens)
}
