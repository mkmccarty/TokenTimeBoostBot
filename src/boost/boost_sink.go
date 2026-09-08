package boost

import (
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashVolunteerSink is used to volunteer as token sink for a contract
func GetSlashVolunteerSink(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Volunteer as token sink for this contract")
	command.Options = []dc.Option{
		dc.BoolOption{
			Name:        "confirm",
			Description: "Confirm you want to be the token sink. Default is false.",
			Required:    true,
		},
	}
	return &command
}

// GetSlashVoluntellSink is used to volunteer as token sink for a contract
func GetSlashVoluntellSink(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Voluntell guest farmer to assign as token sink for this contract")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "farmer",
			Description: "Guest farmer to use as the token sink for this contract.",
			Required:    true,
		},
		dc.BoolOption{
			Name:        "confirm",
			Description: "Confirm you want to be the token sink.  Default is false.",
			Required:    true,
		},
	}
	return &command
}

// HandleSlashVolunteerSinkCommand is used to volunteer as token sink for a contract.
//
// It still takes a raw session because RedrawBoostList is not on the facade
// yet.
func HandleSlashVolunteerSinkCommand(client dc.Client, e *dc.CommandEvent) {
	str := "Volunteering as token sink for this contract. It will show up on the next boost list refresh."
	confirm := false

	if opt, ok := e.OptBool("confirm"); ok {
		confirm = opt
	}

	// Find the contract
	var contract = FindContract(e.ChannelID())
	if contract == nil {
		str = "No contract found in this channel"
	} else {
		userID := e.UserID()

		isAdmin := false
		perms, err := client.UserChannelPermissions(userID, e.ChannelID())
		if err == nil {
			isAdmin = perms.Administrator()
		}

		if !confirm {
			str = "You must confirm you want to be the token sink"
		} else if contract.Banker.PostSinkUserID != "" && !isAdmin {
			str = "Token sink is already set"
		} else {
			// Check if user is already in contract
			if UserInContract(contract, userID) {
				contract.Banker.PostSinkUserID = userID
				changeContractState(contract, contract.State) // Update the changed sink
				if contract.State == ContractStateCompleted || contract.State == ContractStateWaiting {
					_ = RedrawBoostList(client, e.GuildID(), e.ChannelID())
				}
			} else {
				str = "You are not in this contract"
			}
		}
	}

	_ = e.Respond(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}

// HandleSlashVoluntellSinkCommand is used to volunteer as token sink for a contract.
//
// It still takes a raw session because RedrawBoostList is not on the facade
// yet.
func HandleSlashVoluntellSinkCommand(client dc.Client, e *dc.CommandEvent) {
	str := "Voluntell as token sink for this contract. It will show up on the next boost list refresh."

	var VoluntellName string
	confirm := false

	if opt, ok := e.OptString("farmer"); ok {
		VoluntellName = opt
	}

	if opt, ok := e.OptBool("confirm"); ok {
		confirm = opt
	}

	// Find the contract
	var contract = FindContract(e.ChannelID())
	if contract == nil {
		str = "No contract found in this channel"
	} else {
		userID := e.UserID()

		isAdmin := false
		perms, err := client.UserChannelPermissions(userID, e.ChannelID())
		if err == nil {
			isAdmin = perms.Administrator()
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
			if UserInContract(contract, VoluntellName) {
				contract.Banker.PostSinkUserID = VoluntellName
				changeContractState(contract, contract.State) // Update the changed sink
				if contract.State == ContractStateCompleted || contract.State == ContractStateWaiting {
					_ = RedrawBoostList(client, e.GuildID(), e.ChannelID())
				}
			} else {
				str = "They are not in this contract"
			}
		}
	}

	_ = e.Respond(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}
