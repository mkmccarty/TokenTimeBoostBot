package boost

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/moby/moby/pkg/namesgenerator"
)

// GetSlashContractCommand returns the slash command for creating a contract
func GetSlashContractCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Create a contract boost list.",
		/*
			Contexts: &[]discordgo.InteractionContextType{
				discordgo.InteractionContextGuild,
				discordgo.InteractionContextBotDM,
				discordgo.InteractionContextPrivateChannel,
			},
			IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
				discordgo.ApplicationIntegrationGuildInstall,
				discordgo.ApplicationIntegrationUserInstall,
			},
		*/
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Contract ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "coop-id",
				Description: "Coop ID",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "coop-size",
				Description: "Co-op Size. This will be pulled from EI Contract data if unset.",
				Required:    false,
			},
			{
				Name:        "boost-order",
				Description: "Select how boost list is ordered. Default is Sign-up order.",
				Required:    false,
				Type:        discordgo.ApplicationCommandOptionInteger,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Sign-up Ordering",
						Value: ContractOrderSignup,
					},
					{
						Name:  "Fair Ordering",
						Value: ContractOrderFair,
					},
					{
						Name:  "Random Ordering",
						Value: ContractOrderRandom,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "ping-role",
				Description: "Role to use to ping for this contract. Default is @here.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "make-thread",
				Description: "Create a thread for this contract? (default: false)",
				Required:    false,
			},
		},
	}
}

// HandleContractCommand will handle the /contract command
func HandleContractCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}
	var contractID = i.GuildID
	var coopID = i.GuildID // Default to the Guild ID
	var boostOrder = ContractOrderSignup
	var coopSize = 0
	var ChannelID = i.ChannelID
	var pingRole = "@here"
	makeThread := false

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["coop-size"]; ok {
		coopSize = int(opt.IntValue())
	}
	if opt, ok := optionMap["ping-role"]; ok {
		role := opt.RoleValue(nil, "")
		pingRole = role.Mention()
	}
	if opt, ok := optionMap["boost-order"]; ok {
		boostOrder = int(opt.IntValue())
	}
	if opt, ok := optionMap["contract-id"]; ok {
		contractID = opt.StringValue()
		contractID = strings.Replace(contractID, " ", "", -1)
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = opt.StringValue()
		coopID = strings.Replace(coopID, " ", "", -1)
	} else {
		var c, err = s.Channel(ChannelID)
		if err != nil {
			coopID = c.Name
		}
	}
	if opt, ok := optionMap["make-thread"]; ok {
		makeThread = opt.BoolValue()
	}

	if coopSize == 0 {
		found := false
		for _, x := range ei.EggIncContracts {
			if x.ID == contractID {
				found = true
			}
		}
		if !found {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    "Select a contract-id from the dropdown list.\nIf the contract-id list doesn't have your contract then supply a coop-size parameter.",
					Flags:      discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{}},
			})
			return
		}
	}

	// Create a new thread for this contract
	if makeThread {
		ch, err := s.Channel(ChannelID)
		if err == nil {
			if !ch.IsThread() {
				// Default to 1 day timeout
				var builder strings.Builder
				builder.WriteString(coopID)
				info := ei.EggIncContractsAll[contractID]
				if info.ID != "" {
					fmt.Fprintf(&builder, " (0/%d)", info.MaxCoopSize)
				}

				thread, err := s.ThreadStart(ChannelID, builder.String(), discordgo.ChannelTypeGuildPublicThread, 60*24)
				if err == nil {
					ChannelID = thread.ID
					_ = s.ThreadJoin(getInteractionUserID(i))
				} else {
					log.Print(err)
				}
			}
		}
	}

	mutex.Lock()
	contract, err := CreateContract(s, contractID, coopID, coopSize, boostOrder, i.GuildID, ChannelID, getInteractionUserID(i), pingRole)
	mutex.Unlock()
	if err != nil {

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    err.Error(),
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}

	var builder strings.Builder
	builder.WriteString("Contract created. Use the Contract button if you have to recycle it.\n")
	builder.WriteString("This fastrun contract can be converted to a `/speedrun` anytime during the sign-up list.\n")
	builder.WriteString("If this contract isn't an immediate start use `/change-planned-start` to add the time to the sign-up message.\n")
	builder.WriteString("React with 🌊 to automaticaly update the thread name.")

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: builder.String(),
			Flags:   discordgo.MessageFlagsEphemeral,
			// Buttons and other components are specified in Components field.
		},
	})
	if err != nil {
		log.Print(err)
	}

	var createMsg = DrawBoostList(s, contract)
	var data discordgo.MessageSend
	data.Content = createMsg
	data.Flags = discordgo.MessageFlagsSuppressEmbeds
	msg, err := s.ChannelMessageSendComplex(ChannelID, &data)
	if err == nil {
		SetListMessageID(contract, ChannelID, msg.ID)
		var data discordgo.MessageSend
		data.Content, data.Components = GetSignupComponents(false, contract.Speedrun)
		reactionMsg, err := s.ChannelMessageSendComplex(ChannelID, &data)

		if err != nil {
			log.Print(err)
		} else {
			SetReactionID(contract, msg.ChannelID, reactionMsg.ID)
			_ = s.ChannelMessagePin(msg.ChannelID, reactionMsg.ID)
		}
	} else {
		log.Print(err)
	}
}

// CreateContract creates a new contract or joins an existing contract if run from a different location
func CreateContract(s *discordgo.Session, contractID string, coopID string, coopSize int, BoostOrder int, guildID string, channelID string, userID string, pingRole string) (*Contract, error) {
	// When creating contracts, we can make sure to clean up and archived ones
	// Just in case a contract was immediately recreated
	for _, c := range Contracts {
		if c.State == ContractStateArchive {
			if c.CalcOperations == 0 || time.Since(c.CalcOperationTime).Minutes() > 20 {
				FinishContract(s, c)
			}
		}
	}
	/*
		if boostIcon == "🚀" {
			boostIconName = "chickenboost"
			boostIconReaction = findBoostBotGuildEmoji(s, boostIconName, true)
			boostIcon = boostIconReaction + ">"
		}
	*/

	// Make sure this channel doesn't already have a contract
	existingContract := FindContract(channelID)
	if existingContract != nil {
		return nil, errors.New("this channel already has a contract named: " + existingContract.ContractID + "/" + existingContract.CoopID)
	}

	var contract *Contract
	// Does a coop already exist for this contract-id and coop-id
	for _, c := range Contracts {
		if c.ContractID == contractID && c.CoopID == coopID {
			// We have a coop, add this channel to the coop
			contract = c
		}
	}

	loc := new(LocationData)
	loc.GuildID = guildID
	loc.ChannelID = channelID
	var g, gerr = s.Guild(guildID)
	if gerr == nil {
		loc.GuildName = g.Name

	}
	var c, cerr = s.Channel(channelID)
	if cerr == nil {
		loc.ChannelMention = c.Mention()
		loc.ChannelPing = pingRole
	}
	loc.ListMsgID = ""
	loc.ReactionID = ""

	if contract == nil {
		var ContractHash = namesgenerator.GetRandomName(0)
		for Contracts[ContractHash] != nil {
			ContractHash = namesgenerator.GetRandomName(0)
		}

		// We don't have this contract on this channel, it could exist in another channel
		contract = new(Contract)
		contract.Location = append(contract.Location, loc)
		contract.ContractHash = ContractHash
		contract.UseInteractionButtons = config.GetTestMode() // Featuer under test

		contract.Style = ContractStyleFastrun

		//GlobalContracts[ContractHash] = append(GlobalContracts[ContractHash], loc)
		contract.Boosters = make(map[string]*Booster)
		contract.ContractID = contractID
		contract.CoopID = coopID
		contract.BoostOrder = BoostOrder
		contract.BoostVoting = 0
		contract.OrderRevision = 0
		changeContractState(contract, ContractStateSignup)
		contract.CreatorID = append(contract.CreatorID, userID)               // starting userid
		contract.CreatorID = append(contract.CreatorID, "650743870253957160") // Mugwump
		contract.Speedrun = false
		contract.Banker.VolunteerSink = ""
		contract.StartTime = time.Now()
		contract.ChickenRunEmoji = findBoostBotGuildEmoji(s, "icon_chicken_run", true)

		contract.RegisteredNum = 0
		contract.CoopSize = coopSize
		contract.Name = contractID
		updateContractWithEggIncData(contract)
		Contracts[ContractHash] = contract
	} else { //if !creatorOfContract(contract, userID) {
		contract.CreatorID = append(contract.CreatorID, userID) // starting userid
		contract.Location = append(contract.Location, loc)
	}

	// Find our Token emoji
	contract.TokenStr = FindTokenEmoji(s)
	// set TokenReactionStr to the TokenStr without first 2 characters and last character
	contract.TokenReactionStr = contract.TokenStr[2 : len(contract.TokenStr)-1]

	return contract, nil
}
