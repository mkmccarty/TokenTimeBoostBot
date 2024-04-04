package boost

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// HandleSpeedrunCommand handles the speedrun command
func HandleSpeedrunCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Protection against DM use
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "This command can only be run in a server.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{}},
		})
		return
	}

	chickenRuns := 0
	contractStarter := ""
	sink := ""
	sinkPosition := SinkBoostFirst
	speedrunStyle := 0

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["contract-starter"]; ok {
		contractStarter = opt.UserValue(s).Mention()
		contractStarter = contractStarter[2 : len(contractStarter)-1]
		sink = contractStarter
	}
	if opt, ok := optionMap["sink"]; ok {
		sink = strings.TrimSpace(opt.StringValue())
		reMention := regexp.MustCompile(`<@!?(\d+)>`)
		if reMention.MatchString(sink) {
			sink = sink[2 : len(sink)-1]
		}
	}
	if opt, ok := optionMap["chicken-runs"]; ok {
		chickenRuns = int(opt.IntValue())
	}
	if opt, ok := optionMap["sink-position"]; ok {
		sinkPosition = int(opt.IntValue())
	}
	if opt, ok := optionMap["style"]; ok {
		speedrunStyle = int(opt.IntValue())
	}

	str, err := setSpeedrunOptions(s, i.GuildID, i.ChannelID, contractStarter, sink, sinkPosition, chickenRuns, speedrunStyle)
	if err != nil {
		str = err.Error()
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

}

func setSpeedrunOptions(s *discordgo.Session, guildID string, channelID string, contractStarter string, sink string, sinkPosition int, chickenRuns int, speedrunStyle int) (string, error) {
	var contract = FindContract(guildID, channelID)
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	if contract.State != ContractStateSignup {
		return "", error.New("Contract must be in the Signu-up state to set speedrun options.")
	}

	contract.Speedrun = true
	contract.SRData.SpeedrunStarterUserID = contractStarter
	contract.SRData.SinkUserID = sink
	contract.SRData.SinkBoostPosition = sinkPosition
	contract.SRData.ChickenRuns = chickenRuns
	contract.SRData.SpeedrunStyle = speedrunStyle
	contract.SRData.SpeedrunState = SpeedrunStateSignup

	// Set up the details for the Chicken Run Tango
	// first lap is CoopSize -1, every following lap is CoopSize -2

	contract.SRData.Tango[0] = max(0, contract.CoopSize-1)        // First Leg
	contract.SRData.Tango[1] = max(0, contract.SRData.Tango[0]-1) // Middle Legs
	contract.SRData.Tango[2] = 0                                  // Last Leg

	runs := contract.SRData.ChickenRuns
	contract.SRData.Legs = 0
	for runs > 0 {
		if contract.SRData.Legs == 0 {
			runs -= contract.SRData.Tango[0]
		} else if runs > contract.SRData.Tango[1] {
			runs -= contract.SRData.Tango[1]
		} else {
			contract.SRData.Tango[2] = runs
			runs = 0
		}
		contract.SRData.Legs++
	}

	var b strings.Builder
	fmt.Fprint(&b, "> Speedrun can be started once the contract is full.\n\n")
	fmt.Fprintf(&b, "> **%d** Chicken Run Legs to reach **%d** total chicken runs.\n", contract.SRData.Legs, contract.SRData.ChickenRuns)
	if contract.SRData.SpeedrunStyle == SpeedrunStyleWonky {
		fmt.Fprint(&b, "> **Wonky** style speed run:\n")
		fmt.Fprintf(&b, "> * Send all tokens to <@%s>\n", contract.SRData.SpeedrunStarterUserID)
		if contract.SRData.SpeedrunStarterUserID != contract.SRData.SinkUserID {
			fmt.Fprintf(&b, "> * After contract boosting send all tokens to: <@%s> (This is unusual)\n", contract.SRData.SinkUserID)
		}
	} else {
		fmt.Fprint(&b, "> **Boost List** style speed run:\n")
		fmt.Fprintf(&b, "> * During CRT send tokens to <@%s>\n", contract.SRData.SpeedrunStarterUserID)
		fmt.Fprint(&b, "> * Follow the Boost List for Token Passing.\n")
		fmt.Fprintf(&b, "> * After contract boosting send all tokens to <@%s>\n", contract.SRData.SinkUserID)
	}
	contract.SRData.StatusStr = b.String()

	var builder strings.Builder
	fmt.Fprintf(&builder, "Speedrun options set for %s/%s\n", contract.ContractID, contract.CoopID)
	fmt.Fprintf(&builder, "Contract Starter: <@%s>\n", contract.SRData.SpeedrunStarterUserID)
	fmt.Fprintf(&builder, "Sink CRT: <@%s>\n", contract.SRData.SinkUserID)

	disableButton := false
	if contract.Speedrun && contract.CoopSize != len(contract.Boosters) {
		disableButton = true
	}
	if contract.State != ContractStateSignup {
		disableButton = true
	}

	// For each contract location, update the signup message
	refreshBoostListMessage(s, contract)

	for _, loc := range contract.Location {
		// Rebuild the signup message to disable the start button
		msgID := loc.ReactionID
		msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)

		contentStr, comp := GetSignupComponents(disableButton, contract.Speedrun) // True to get a disabled start button
		msg.SetContent(contentStr)
		msg.Components = &comp
		s.ChannelMessageEditComplex(msg)
	}

	return builder.String(), nil
}
