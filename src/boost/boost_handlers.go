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
	sinkCRT := ""
	sinkEnd := ""
	sinkPosition := 0

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["sink-crt"]; ok {
		sinkCRT = opt.UserValue(s).Mention()
		sinkCRT = sinkCRT[2 : len(sinkCRT)-1]
		sinkEnd = sinkCRT
	}
	if opt, ok := optionMap["sink-end"]; ok {
		sinkEnd = strings.TrimSpace(opt.StringValue())
		reMention := regexp.MustCompile(`<@!?(\d+)>`)
		if reMention.MatchString(sinkEnd) {
			sinkEnd = sinkEnd[2 : len(sinkEnd)-1]
		}
	}
	if opt, ok := optionMap["chicken-runs"]; ok {
		chickenRuns = int(opt.IntValue())
	}
	if opt, ok := optionMap["sink-position"]; ok {
		sinkPosition = int(opt.IntValue())
	}

	str, err := setSpeedrunOptions(i.GuildID, i.ChannelID, sinkCRT, sinkEnd, sinkPosition, chickenRuns)
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

func setSpeedrunOptions(gID string, cID string, sinkCRT string, sinkEnd string, sinkPosition int, chickenRuns int) (string, error) {
	var contract = FindContract(gID, cID)
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	contract.SinkCrtUserID = sinkCRT
	contract.SinkEndUserID = sinkEnd
	contract.SinkBoostPosition = sinkPosition
	contract.ChickenRuns = chickenRuns

	var builder strings.Builder
	fmt.Fprintf(&builder, "Speedrun options set for %s/%s\n", contract.ContractID, contract.CoopID)
	fmt.Fprintf(&builder, "Sink CRT: %s\n", contract.Boosters[contract.SinkCrtUserID].Mention)

	return builder.String(), nil
}
