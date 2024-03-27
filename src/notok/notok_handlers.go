package notok

import "github.com/bwmarrin/discordgo"

// FunHandler handles the /fun command
func FunHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
	var gptOption = int64(0)
	var gptText = ""

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["action"]; ok {
		gptOption = opt.IntValue()
	}
	if opt, ok := optionMap["prompt"]; ok {
		gptText = opt.StringValue()
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		//Data: &discordgo.InteractionResponseData{
		//	Content:    "",
		//	Flags:      discordgo.MessageFlagsEphemeral,
		//	Components: []discordgo.MessageComponent{}},
	},
	)

	var _ = Notok(s, i, gptOption, gptText)
}
