package boost

import (
	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/track"
)

// HandleTokenCommand takes the main command and adds the current contract to the message
func HandleTokenCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var contractID string
	contract := FindContract(i.ChannelID)
	if contract != nil {
		contractID = contract.ContractID
	}
	track.HandleTokenCommand(s, i, contractID)
}
