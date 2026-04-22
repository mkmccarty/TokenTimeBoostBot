package farmerstate

import (
	"log"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

// HandleSetEggIncName handles the /seteggincname command
func HandleSetEggIncName(s *discordgo.Session, i *discordgo.InteractionCreate, isCoordinator func(*discordgo.Session, string) bool) {
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
	var eiName string
	var callerUserID = bottools.GetInteractionUserID(i)
	var userID = bottools.GetInteractionUserID(i)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["discord-name"]; ok {
		farmerMention := opt.UserValue(s).Mention()
		re := regexp.MustCompile(`[\\<>@#&!]`)
		userID = re.ReplaceAllString(farmerMention, "")
	}

	var str = "Setting Egg, IGN for <@" + userID + "> to "

	if opt, ok := optionMap["ei-ign"]; ok {
		eiName = strings.TrimSpace(opt.StringValue())
		str += eiName
	}

	// if eiName matches this regex ^EI[1-9]*$ then it an invalid name
	re := regexp.MustCompile(`^EI[1-9]*$`)
	if re.MatchString(eiName) {
		str = "Don't use your Egg, Inc. EI number."
	} else {
		// Is the user issuing the command a coordinator?
		if userID != callerUserID && !isCoordinator(s, callerUserID) {
			str = "This form of usage is restricted to contract coordinators and administrators."
		} else {
			SetEggIncName(userID, eiName)
		}
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
	if err != nil {
		log.Println(err.Error())
	}
}
