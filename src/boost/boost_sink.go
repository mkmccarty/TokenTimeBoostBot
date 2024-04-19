package boost

import "github.com/bwmarrin/discordgo"

// GetSlashVolunteerSink is used to volunteer as token sink for a contract
func GetSlashVolunteerSink(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Volunteer as token sink for this contract",
	}
}

// HandleSlashVolunteerSinkCommand is used to volunteer as token sink for a contract
func HandleSlashVolunteerSinkCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// User interacting with bot, is this first time ?
	str := "Volunteering as token sink for this contract"

	// Find the contract
	var contract = FindContract(i.Interaction.ChannelID)
	if contract == nil {
		str = "No contract found in this channel"
	} else {
		if contract.Speedrun {
			str = "You cannot use this command on a speedrun contract"
		} else {
			// if VolunteerSink is already set, reply with error
			if contract.VolunteerSink != "" {
				str = "Post contract sink already claimed by <@" + contract.VolunteerSink + ">"
			} else {
				// Check if user is already in contract
				if userInContract(contract, i.Interaction.Member.User.ID) {
					contract.VolunteerSink = i.Interaction.Member.User.ID
					if contract.State == ContractStateCompleted {
						RedrawBoostList(s, i.GuildID, i.ChannelID)
					}
				} else {
					str = "You are not in this contract"
				}
			}
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	},
	)
}
