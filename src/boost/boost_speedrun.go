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

	contract.Speedrun = true
	contract.SpeedrunStarterUserID = contractStarter
	contract.SinkUserID = sink
	contract.SinkBoostPosition = sinkPosition
	contract.ChickenRuns = chickenRuns
	contract.SpeedrunStyle = speedrunStyle

	var builder strings.Builder
	fmt.Fprintf(&builder, "Speedrun options set for %s/%s\n", contract.ContractID, contract.CoopID)
	fmt.Fprintf(&builder, "Contract Starter: %s\n", contract.Boosters[contract.SpeedrunStarterUserID].Mention)
	fmt.Fprintf(&builder, "Sink CRT: %s\n", contract.Boosters[contract.SinkUserID].Mention)

	// Rebuild the signup message to disable the start button
	msgID := contract.SignupMsgID[channelID]
	msg := discordgo.NewMessageEdit(channelID, msgID)

	disableButton := false
	if contract.Speedrun && contract.CoopSize != len(contract.Boosters) {
		disableButton = true
	}
	if contract.State != ContractStateSignup {
		disableButton = true
	}

	contentStr, comp := GetSignupComponents(disableButton, contract.Speedrun) // True to get a disabled start button
	msg.SetContent(contentStr)
	msg.Components = &comp
	s.ChannelMessageEditComplex(msg)

	return builder.String(), nil
}
