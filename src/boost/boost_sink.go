package boost

import (
	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

// GetSlashVolunteerSink is used to volunteer as token sink for a contract
func GetSlashVolunteerSink(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Volunteer as token sink for this contract",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "confirm",
				Description: "Confirm you want to be the token sink. Default is false.",
				Required:    true,
			},
		},
	}
}

// GetSlashVoluntellSink is used to volunteer as token sink for a contract
func GetSlashVoluntellSink(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Voluntell guest farmer to assign as token sink for this contract",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "farmer",
				Description: "Guest farmer to use as the token sink for this contract.",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "confirm",
				Description: "Confirm you want to be the token sink.  Default is false.",
				Required:    true,
			},
		},
	}
}

// HandleSlashVolunteerSinkCommand is used to volunteer as token sink for a contract
func HandleSlashVolunteerSinkCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "Volunteering as token sink for this contract. It will show up on the next boost list refresh."
	confirm := false

	optionMap := bottools.GetCommandOptionsMap(i)
	if opt, ok := optionMap["confirm"]; ok {
		confirm = opt.BoolValue()
	}

	// Find the contract
	var contract = FindContract(i.ChannelID)
	if contract == nil {
		str = "No contract found in this channel"
	} else {

		var userID string
		if i.GuildID != "" {
			userID = i.Member.User.ID
		} else {
			userID = i.User.ID
		}

		isAdmin := false
		perms, err := s.UserChannelPermissions(userID, i.ChannelID)
		if err == nil {
			if perms&discordgo.PermissionAdministrator != 0 {
				isAdmin = true
			}
		}

		if !confirm {
			str = "You must confirm you want to be the token sink"
		} else if contract.Banker.PostSinkUserID != "" && !isAdmin {
			str = "Token sink is already set"
		} else {
			// Check if user is already in contract
			if userInContract(contract, i.Member.User.ID) {
				contract.Banker.PostSinkUserID = i.Member.User.ID
				changeContractState(contract, contract.State) // Update the changed sink
				if contract.State == ContractStateCompleted || contract.State == ContractStateWaiting {
					_ = RedrawBoostList(s, i.GuildID, i.ChannelID)
				}
			} else {
				str = "You are not in this contract"
			}
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	},
	)
}

// HandleSlashVoluntellSinkCommand is used to volunteer as token sink for a contract
func HandleSlashVoluntellSinkCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "Voluntell as token sink for this contract. It will show up on the next boost list refresh."

	optionMap := bottools.GetCommandOptionsMap(i)

	var VoluntellName string
	confirm := false

	if opt, ok := optionMap["farmer"]; ok {
		VoluntellName = opt.StringValue()
	}

	if opt, ok := optionMap["confirm"]; ok {
		confirm = opt.BoolValue()
	}

	// Find the contract
	var contract = FindContract(i.ChannelID)
	if contract == nil {
		str = "No contract found in this channel"
	} else {

		var userID string
		if i.GuildID != "" {
			userID = i.Member.User.ID
		} else {
			userID = i.User.ID
		}

		isAdmin := false
		perms, err := s.UserChannelPermissions(userID, i.ChannelID)
		if err == nil {
			if perms&discordgo.PermissionAdministrator != 0 {
				isAdmin = true
			}
		}

		if !confirm {
			str = "You must confirm you want to be the token sink"
		} else if _, isMention := parseMentionUserID(VoluntellName); isMention {
			str = "This should be a guest farmer within this contract and not a user mention."
		} else if contract.Banker.PostSinkUserID != "" && !isAdmin {
			str = "Token sink is already set"
		} else {
			// if VolunteerSink is already set, reply with error
			// Check if user is already in contract
			if userInContract(contract, VoluntellName) {
				contract.Banker.PostSinkUserID = VoluntellName
				changeContractState(contract, contract.State) // Update the changed sink
				if contract.State == ContractStateCompleted || contract.State == ContractStateWaiting {
					_ = RedrawBoostList(s, i.GuildID, i.ChannelID)
				}
			} else {
				str = "They are not in this contract"
			}
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	},
	)
}
