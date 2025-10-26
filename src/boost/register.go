package boost

import (
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

// GetSlashRegisterCommand returns the /register command
func GetSlashRegisterCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Register your Egg Inc ID with Boost Bot.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
	}
}

// HandleRegister handles the /register command
func HandleRegister(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	optionMap := bottools.GetCommandOptionsMap(i)
	if opt, ok := optionMap["reset"]; ok {
		if opt.BoolValue() {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
		}
	}
	RequestEggIncIDModal(s, i, "register", optionMap)
}
