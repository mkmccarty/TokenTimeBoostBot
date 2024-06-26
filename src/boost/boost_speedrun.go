package boost

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetSlashChangeSpeedRunSinkCommand returns the slash command for changing speedrun sink assignments
func GetSlashChangeSpeedRunSinkCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Change speedrun sink assignements of a running contract",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "sink-crt",
				Description: "The user to sink during CRT.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "sink-boosting",
				Description: "Sink during boosting.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "sink-post",
				Description: "Post contract sink.",
				Required:    false,
			}},
	}
}

// HandleChangeSpeedrunSinkCommand handles the change speedrun sink command
func HandleChangeSpeedrunSinkCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	sinkCrt := ""
	sinkBoost := ""
	sinkPost := ""
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["sink-crt"]; ok {
		sinkCrt = opt.UserValue(s).Mention()
		sinkCrt = sinkCrt[2 : len(sinkCrt)-1]
	}
	if opt, ok := optionMap["sink-boosting"]; ok {
		sinkPost = strings.TrimSpace(opt.StringValue())
		reMention := regexp.MustCompile(`<@!?(\d+)>`)
		if reMention.MatchString(sinkBoost) {
			sinkBoost = sinkBoost[2 : len(sinkBoost)-1]
		}
	}
	if opt, ok := optionMap["sink-post"]; ok {
		sinkPost = strings.TrimSpace(opt.StringValue())
		reMention := regexp.MustCompile(`<@!?(\d+)>`)
		if reMention.MatchString(sinkPost) {
			sinkPost = sinkPost[2 : len(sinkPost)-1]
		}
	}

	str, err := setSpeedrunOptions(s, i.ChannelID, sinkCrt, sinkBoost, sinkPost, -1, -1, -1, false, true)
	if err != nil {
		str = err.Error()
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

}

// HandleSpeedrunCommand handles the speedrun command
func HandleSpeedrunCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	chickenRuns := 0
	sinkCrt := ""
	sinkBoost := ""
	sinkPost := ""
	sinkPosition := SinkBoostFirst
	speedrunStyle := 0
	selfRuns := false

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["sink-crt"]; ok {
		sinkCrt = opt.UserValue(s).Mention()
		sinkCrt = sinkCrt[2 : len(sinkCrt)-1]
		sinkBoost = sinkCrt
		sinkPost = sinkCrt
	}
	if opt, ok := optionMap["sink-boosting"]; ok {
		sinkPost = strings.TrimSpace(opt.StringValue())
		reMention := regexp.MustCompile(`<@!?(\d+)>`)
		if reMention.MatchString(sinkBoost) {
			sinkBoost = sinkBoost[2 : len(sinkBoost)-1]
		}
	}
	if opt, ok := optionMap["sink-post"]; ok {
		sinkPost = strings.TrimSpace(opt.StringValue())
		reMention := regexp.MustCompile(`<@!?(\d+)>`)
		if reMention.MatchString(sinkPost) {
			sinkPost = sinkPost[2 : len(sinkPost)-1]
		}
	}
	if opt, ok := optionMap["style"]; ok {
		speedrunStyle = int(opt.IntValue())
	}
	if opt, ok := optionMap["chicken-runs"]; ok {
		chickenRuns = int(opt.IntValue())
	}
	if opt, ok := optionMap["self-runs"]; ok {
		selfRuns = opt.BoolValue()
	}
	if opt, ok := optionMap["sink-position"]; ok {
		sinkPosition = int(opt.IntValue())
	}

	str, err := setSpeedrunOptions(s, i.ChannelID, sinkCrt, sinkBoost, sinkPost, sinkPosition, chickenRuns, speedrunStyle, selfRuns, false)
	if err != nil {
		str = err.Error()
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func getSpeedrunStatusStr(contract *Contract) string {
	var b strings.Builder
	//fmt.Fprint(&b, "> Speedrun can be started once the contract is full.\n\n")
	if contract.SRData.Tango[0] > 1 {
		if contract.Style&ContractFlagSelfRuns != 0 {
			fmt.Fprintf(&b, "> --> **Self-run of chickens is required** <--\n")
			if contract.Location[0].GuildID == "485162044652388384" {
				fmt.Fprintf(&b, "> * how-to self-run: %s\n", "https://discord.com/channels/485162044652388384/490151868631089152/1255676641192054834")
			}
		}

		fmt.Fprintf(&b, "> **%d** Chicken Run Legs to reach **%d** total chicken runs.\n", contract.SRData.Legs, contract.SRData.ChickenRuns)
	} else {
		farmerPlural := "s"
		if len(contract.Order) == 1 {
			farmerPlural = ""
		}

		fmt.Fprintf(&b, "> It's not possible to reach **%d** total chicken runs with only **%d** farmer%s.\n\n", contract.SRData.ChickenRuns, len(contract.Order), farmerPlural)
	}
	if len(contract.Order) == 0 {
		return b.String()
	}
	if contract.SRData.SpeedrunStyle == SpeedrunStyleBanker {
		fmt.Fprint(&b, "> **Banker** style speed run:\n")
		if contract.SRData.CrtSinkUserID == contract.SRData.BoostingSinkUserID && contract.SRData.CrtSinkUserID == contract.SRData.PostSinkUserID {
			fmt.Fprintf(&b, "> * Send all tokens to **%s**\n", contract.Boosters[contract.SRData.CrtSinkUserID].Mention)
		} else if contract.SRData.CrtSinkUserID == contract.SRData.BoostingSinkUserID {
			fmt.Fprintf(&b, "> * Send CRT & Boosting tokens to **%s**\n", contract.Boosters[contract.SRData.CrtSinkUserID].Mention)
		} else {
			fmt.Fprintf(&b, "> * Send CRT tokens to **%s**\n", contract.Boosters[contract.SRData.CrtSinkUserID].Mention)
			fmt.Fprintf(&b, "> * Send Boosting tokens to **%s**\n", contract.Boosters[contract.SRData.BoostingSinkUserID].Mention)
		}
		fmt.Fprintf(&b, "> The sink will send you a full set of boost tokens.\n")
		if contract.SRData.BoostingSinkUserID != contract.SRData.PostSinkUserID {
			fmt.Fprintf(&b, "> * After contract boosting send all tokens to: %s (This is unusual)\n", contract.Boosters[contract.SRData.PostSinkUserID].Mention)
		}
	} else {
		fmt.Fprint(&b, "> **Boost List** style speed run:\n")
		fmt.Fprintf(&b, "> * During CRT send tokens to %s\n", contract.Boosters[contract.SRData.CrtSinkUserID].Mention)
		fmt.Fprint(&b, "> * Follow the Boost List for Token Passing.\n")
		fmt.Fprintf(&b, "> * After contract boosting send all tokens to %s\n", contract.Boosters[contract.SRData.PostSinkUserID].Mention)
	}
	return b.String()
}

func calculateTangoLegs(contract *Contract, setStatus bool) {
	selfRunMod := 0
	if contract.Style&ContractFlagSelfRuns != 0 {
		selfRunMod = 1
	}

	contract.SRData.Tango[0] = max(0, len(contract.Order)-selfRunMod) // First Leg
	contract.SRData.Tango[1] = max(0, contract.SRData.Tango[0]-1)     // Middle Legs
	contract.SRData.Tango[2] = 0                                      // Last Leg

	runs := contract.SRData.ChickenRuns
	contract.SRData.Legs = 0
	for runs > 0 {
		if contract.SRData.Legs == 0 {
			runs -= contract.SRData.Tango[0]
			if runs <= 0 {
				break
			}
		} else if contract.SRData.Tango[1] == 0 {
			// Not possible to do any CRT
			contract.SRData.Legs = 0 // Reset the legs here
			break
		} else if runs > contract.SRData.Tango[1] {
			runs -= contract.SRData.Tango[1]
		} else {
			contract.SRData.Tango[2] = runs
			break // No more runs to do, skips the Legs++ below
		}
		contract.SRData.Legs++
	}

	if setStatus {
		contract.SRData.StatusStr = getSpeedrunStatusStr(contract)
	}
}

func setSpeedrunOptions(s *discordgo.Session, channelID string, sinkCrt string, sinkBoosting string, sinkPost string, sinkPosition int, chickenRuns int, speedrunStyle int, selfRuns bool, changeSinksOnly bool) (string, error) {
	var contract = FindContract(channelID)
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	if contract.State != ContractStateSignup && !changeSinksOnly {
		return "", errors.New("contract must be in the Sign-up state to set speedrun options")
	}

	if sinkCrt != "" {
		// is contractStarter and sink in the contract
		if _, ok := contract.Boosters[sinkCrt]; !ok {
			return "", errors.New("crt sink not in the contract")
		}
	}
	if sinkBoosting != "" {
		if _, ok := contract.Boosters[sinkBoosting]; !ok {
			return "", errors.New("boosting sink not in the contract")
		}
	}
	if sinkPost != "" {
		if _, ok := contract.Boosters[sinkPost]; !ok {
			return "", errors.New("post contract sink not in the contract")
		}
	}

	if speedrunStyle == SpeedrunStyleBanker && !changeSinksOnly {

		// Verify that the sink is a snowflake id
		if _, err := s.User(sinkBoosting); err != nil {
			return "", errors.New("boosting sink must be a user mention for Banker style boost lists")
		}

		if _, err := s.User(sinkPost); err != nil {
			return "", errors.New("post contract sink must be a user mention for Banker style boost lists")
		}
	}

	if changeSinksOnly && !contract.Speedrun {
		return "", errors.New("sinks can only be changed for an existing speedrun contract")
	}

	if changeSinksOnly && contract.Speedrun {
		var builder strings.Builder
		if sinkCrt != "" {
			contract.SRData.CrtSinkUserID = sinkCrt
			fmt.Fprintf(&builder, "CRT Sink set to %s\n", contract.Boosters[contract.SRData.CrtSinkUserID].Mention)
		}
		if sinkBoosting != "" {
			contract.SRData.BoostingSinkUserID = sinkBoosting
			fmt.Fprintf(&builder, "Boosting Sink set to %s\n", contract.Boosters[contract.SRData.BoostingSinkUserID].Mention)
		}
		if sinkPost != "" {
			contract.SRData.PostSinkUserID = sinkPost
			fmt.Fprintf(&builder, "Post Sink set to %s\n", contract.Boosters[contract.SRData.PostSinkUserID].Mention)
		}
		return builder.String(), nil
	}

	contract.SRData.CrtSinkUserID = sinkCrt
	contract.SRData.BoostingSinkUserID = sinkBoosting
	contract.SRData.PostSinkUserID = sinkPost
	contract.SRData.SinkBoostPosition = sinkPosition
	contract.SRData.SpeedrunStyle = speedrunStyle
	contract.BoostOrder = ContractOrderFair

	// This kind of contract is always a CRT
	contract.Style = ContractStyleSpeedrunBoostList

	if speedrunStyle == SpeedrunStyleBanker {
		contract.Style = ContractStyleSpeedrunBanker
	}
	if selfRuns {
		contract.Style |= ContractFlagSelfRuns
	} else {
		contract.Style &= ^ContractFlagSelfRuns
	}

	contract.Speedrun = contract.Style&ContractFlagBanker != 0
	contract.Speedrun = true // TODO: this will be removed in favor of flags

	// Chicken Runs Calc
	// Info from https://egg-inc.fandom.com/wiki/Contracts
	if chickenRuns != 0 {
		contract.SRData.ChickenRuns = chickenRuns
	}

	calculateTangoLegs(contract, true)

	var builder strings.Builder
	fmt.Fprintf(&builder, "Speedrun options set for %s/%s\n", contract.ContractID, contract.CoopID)
	fmt.Fprintf(&builder, "CRT Sink: %s\n", contract.Boosters[contract.SRData.CrtSinkUserID].Mention)
	fmt.Fprintf(&builder, "Boosting Sink: %s\n", contract.Boosters[contract.SRData.BoostingSinkUserID].Mention)
	fmt.Fprintf(&builder, "Post Sink: %s\n", contract.Boosters[contract.SRData.PostSinkUserID].Mention)

	disableButton := false
	if contract.State != ContractStateSignup {
		disableButton = true
	}

	// For each contract location, update the signup message
	refreshBoostListMessage(s, contract)

	for _, loc := range contract.Location {
		// Rebuild the signup message to disable the start button
		msgID := loc.ReactionID
		msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)

		contentStr, comp := GetSignupComponents(disableButton, contract.Style&ContractFlagCrt != 0) // True to get a disabled start button
		msg.SetContent(contentStr)
		msg.Components = &comp
		_, _ = s.ChannelMessageEditComplex(msg)
	}

	return builder.String(), nil
}

func reorderSpeedrunBoosters(contract *Contract) {
	// Speedrun contracts are always fair ordering over last 3 contracts
	newOrder := farmerstate.GetOrderHistory(contract.Order, 3)

	index := slices.Index(newOrder, contract.SRData.BoostingSinkUserID)
	// Remove the speedrun starter from the list
	newOrder = append(newOrder[:index], newOrder[index+1:]...)

	if contract.SRData.SinkBoostPosition == SinkBoostFirst {
		newOrder = append([]string{contract.SRData.BoostingSinkUserID}, newOrder...)
	} else {
		newOrder = append(newOrder, contract.SRData.BoostingSinkUserID)
	}
	contract.Order = removeDuplicates(newOrder)
}

func addSpeedrunContractReactions(s *discordgo.Session, contract *Contract, channelID string, messageID string) {
	if contract.State == ContractStateCRT {
		addReactionIconsCRT(s, contract, channelID, messageID)
	}
	if contract.SRData.SpeedrunState == SpeedrunStateBoosting {
		_ = s.MessageReactionAdd(channelID, messageID, contract.TokenStr) // Send token to Sink
		for _, el := range contract.AltIcons {
			_ = s.MessageReactionAdd(channelID, messageID, el)
		}
		_ = s.MessageReactionAdd(channelID, messageID, "🐓") // Want Chicken Run
		_ = s.MessageReactionAdd(channelID, messageID, "💰") // Sink sent requested number of tokens to booster
	}
	if contract.SRData.SpeedrunState == SpeedrunStatePost {
		_ = s.MessageReactionAdd(channelID, messageID, contract.TokenStr) // Send token to Sink
		for _, el := range contract.AltIcons {
			_ = s.MessageReactionAdd(channelID, messageID, el)
		}
		_ = s.MessageReactionAdd(channelID, messageID, "🐓") // Want Chicken Run
	}
}

func speedrunReactions(s *discordgo.Session, r *discordgo.MessageReaction, contract *Contract) string {
	returnVal := ""
	keepReaction := false
	redraw := false

	// Token reaction handling
	tokenReactionStr := "token"
	userID := r.UserID
	// Special handling for alt icons representing token reactions
	if slices.Index(contract.AltIcons, r.Emoji.Name) != -1 {
		idx := slices.Index(contract.Boosters[r.UserID].AltsIcons, r.Emoji.Name)
		if idx != -1 {
			userID = contract.Boosters[r.UserID].Alts[idx]
			tokenReactionStr = r.Emoji.Name
		}
	}
	if strings.ToLower(r.Emoji.Name) == tokenReactionStr {
		_, redraw = buttonReactionToken(s, r.GuildID, r.ChannelID, contract, userID)
	}

	if contract.SRData.SpeedrunState == SpeedrunStateBoosting {
		idx := slices.Index(contract.Boosters[r.UserID].Alts, contract.SRData.BoostingSinkUserID)
		if idx != -1 {
			// This is an alternate
			userID = contract.Boosters[r.UserID].Alts[idx]
		}
		if userID == contract.SRData.BoostingSinkUserID {
			if r.Emoji.Name == "💰" {
				_, redraw = buttonReactionBag(s, r.GuildID, r.ChannelID, contract, r.UserID)
			}
		}
	}

	if contract.SRData.SpeedrunState == SpeedrunStateBoosting || contract.SRData.SpeedrunState == SpeedrunStatePost {
		if r.Emoji.Name == "🐓" && userInContract(contract, r.UserID) {
			// Indicate that a farmer is ready for chicken runs
			redraw = buttonReactionRunChickens(s, contract, r.UserID)
		}
	}

	if r.Emoji.Name == "🌊" {
		UpdateThreadName(s, contract)
	}

	// Remove extra added emoji
	if !keepReaction {
		go RemoveAddedReaction(s, r)
	}

	if redraw {
		refreshBoostListMessage(s, contract)
	}

	return returnVal
}
