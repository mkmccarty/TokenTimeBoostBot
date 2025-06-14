package boost

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var integerZeroMinValue float64 = 0.0

// GetSlashSpeedrunCommand returns the slash command for speedrun
func GetSlashSpeedrunCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Add speedrun features to a contract.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "sink-crt",
				Description: "The user to sink during CRT. Used for other sink parameters if those are missing.",
				Required:    true,
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
			},
			{
				Name:        "sink-position",
				Description: "Default is First Booster",
				Required:    false,
				Type:        discordgo.ApplicationCommandOptionInteger,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "First",
						Value: SinkBoostFirst,
					},
					{
						Name:  "Last",
						Value: SinkBoostLast,
					},
					{
						Name:  "Follow Order",
						Value: SinkBoostFollowOrder,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "chicken-runs",
				Description: "Number of chicken runs for this contract. Optional if contract-id was selected via auto fill.",
				MinValue:    &integerZeroMinValue,
				MaxValue:    20,
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "self-runs",
				Description: "Self Runs during CRT",
				Required:    false,
			},
		},
	}
}

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
		sinkBoost = strings.TrimSpace(opt.StringValue())
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

	str, err := setSpeedrunOptions(s, i.ChannelID, sinkCrt, sinkBoost, sinkPost, -1, -1, false, true)
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
	if opt, ok := optionMap["chicken-runs"]; ok {
		chickenRuns = int(opt.IntValue())
	}
	if opt, ok := optionMap["self-runs"]; ok {
		selfRuns = opt.BoolValue()
	}
	if opt, ok := optionMap["sink-position"]; ok {
		sinkPosition = int(opt.IntValue())
	}

	str, err := setSpeedrunOptions(s, i.ChannelID, sinkCrt, sinkBoost, sinkPost, sinkPosition, chickenRuns, selfRuns, false)
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
	if contract.Style&ContractFlagCrt != 0 {
		runs := contract.SRData.NoSelfRunLegs
		if contract.Style&ContractFlagSelfRuns != 0 {
			runs = contract.SRData.SelfRunLegs
		}
		if runs > 0 {
			if contract.Style&ContractFlagSelfRuns != 0 {
				if contract.SRData.SelfRunLegs == contract.SRData.NoSelfRunLegs {
					fmt.Fprintf(&b, "> Self-run selected but not needed\n")
				} else {
					fmt.Fprintf(&b, "> --> **Self-run of chickens is required** <--\n")
					if contract.Location[0].GuildID == "485162044652388384" {
						fmt.Fprintf(&b, "> * how-to self-run: %s\n", "https://discord.com/channels/485162044652388384/490151868631089152/1255676641192054834")
					}
				}
			}

			legPlural := "s"
			if contract.Style&ContractFlagSelfRuns == 0 {
				if contract.SRData.NoSelfRunLegs == 1 {
					legPlural = ""
				}
				fmt.Fprintf(&b, "> **%d** Chicken Run Leg%s to reach **%d** total chicken runs.\n", contract.SRData.NoSelfRunLegs, legPlural, contract.SRData.ChickenRuns)
				var crtStrings []string
				for _, num := range contract.SRData.NoSelfRunCrt {
					crtStrings = append(crtStrings, fmt.Sprintf("%d", num))
				}
				fmt.Fprintf(&b, "> **%s**  +%d\n", strings.Join(crtStrings, "🦵"), (len(contract.Order)-2)-contract.SRData.NoSelfRunCrt[len(contract.SRData.NoSelfRunCrt)-1])
			} else {
				if contract.SRData.SelfRunLegs == 1 {
					legPlural = ""
				}
				fmt.Fprintf(&b, "> **%d** Chicken Run Leg%s to reach **%d** total chicken runs.\n", contract.SRData.SelfRunLegs, legPlural, contract.SRData.ChickenRuns)
				var selfRunCrtStrings []string
				for _, num := range contract.SRData.SelfRunCrt {
					selfRunCrtStrings = append(selfRunCrtStrings, fmt.Sprintf("%d", num))
				}
				fmt.Fprintf(&b, "> **%s**  +%d\n", strings.Join(selfRunCrtStrings, "🦵"), (len(contract.Order)-1)-contract.SRData.SelfRunCrt[len(contract.SRData.SelfRunCrt)-1])
			}

		} else {
			farmerPlural := "s"
			if len(contract.Order) == 1 {
				farmerPlural = ""
			}
			if contract.SRData.ChickenRuns > len(contract.Order) {
				fmt.Fprintf(&b, "> It's not possible for all farmers to reach **%d** total chicken runs with only **%d** farmer%s.\n", contract.SRData.ChickenRuns, len(contract.Order), farmerPlural)
			} else {
				fmt.Fprintf(&b, "> **%d** farmer%s don't need CRT to achieve **%d** total chicken runs.\n", len(contract.Order), farmerPlural, contract.SRData.ChickenRuns)

			}
		}
	}
	if len(contract.Order) == 0 {
		return b.String()
	}

	if contract.Style&ContractFlagBanker != 0 {
		fmt.Fprint(&b, "\n## **Banker** style contract\n")
		fmt.Fprintf(&b, "> During boosting the Banker will send you a full set of boost tokens.\n")
	} else {
		fmt.Fprint(&b, "\n## **Boost List** style contract\n")
		fmt.Fprintf(&b, "> During boosting send tokens to farmer with %s\n", contract.TokenStr)
	}

	// CRT Sink
	if contract.Style&ContractFlagCrt != 0 {
		if contract.Banker.CrtSinkUserID != "" {
			fmt.Fprintf(&b, "> * Send CRT tokens to **%s**\n", contract.Boosters[contract.Banker.CrtSinkUserID].Mention)
		} else if contract.SRData.Legs > 0 {
			fmt.Fprintf(&b, "> * Currently there are no farmers assigned for CRT Banker. **CRT will be skipped**.\n")
		}
	}
	// Boosting Sink
	if contract.Style&ContractFlagBanker != 0 {
		if contract.Banker.BoostingSinkUserID != "" {
			fmt.Fprintf(&b, "> * During boosting send all tokens to **%s**\n", contract.Boosters[contract.Banker.BoostingSinkUserID].Mention)
			switch contract.Banker.SinkBoostPosition {
			case SinkBoostFirst:
				fmt.Fprint(&b, ">  * Banker boosts **First**\n")
			case SinkBoostLast:
				fmt.Fprint(&b, ">  * Banker boosts **Last**\n")
			default:
				fmt.Fprint(&b, ">  * Banker folows normal boost order\n")
			}

		} else {
			fmt.Fprintf(&b, "> * **Contract cannot start**. Banker required for boosting phase.\n")
		}
	}
	if contract.Banker.PostSinkUserID != "" {
		fmt.Fprintf(&b, "> * After contract boosting send all tokens to **%s**\n", contract.Boosters[contract.Banker.PostSinkUserID].Mention)
	}

	return b.String()
}

func sumIntSlice(numbers []int) int {
	sum := 0
	for _, number := range numbers {
		sum += number
	}
	return sum
}

// func calculateTangoLegs(contract *Contract, setStatus bool) ([]int, []int, error) {
func calculateTangoLegs(contract *Contract, setStatus bool) {
	if contract.State != ContractStateSignup {
		// We don't want this changing after the CRT starts
		//return nil, nil, errors.New("contract must be in the Sign-up state to calculate tango legs")
		return
	}
	var tango []int
	var tangoSelfRun []int

	runs := 0
	if len(contract.Order) == 1 {
		//return tango, tangoSelfRun, nil
		return
	}

	//	r := contract.CoopSize
	r := len(contract.Order) - 1

	for runs < contract.SRData.ChickenRuns {
		if r > 0 {
			tango = append(tango, min(r, contract.SRData.ChickenRuns-sumIntSlice(tango)))
		}
		if sumIntSlice(tangoSelfRun) < contract.SRData.ChickenRuns {
			tangoSelfRun = append(tangoSelfRun, min(r+1, contract.SRData.ChickenRuns-sumIntSlice(tangoSelfRun)))
		}
		runs += r
		r = len(contract.Order) - 2
		if r <= 0 {
			// main loop won't finish, but self-runs can succeed
			if sumIntSlice(tangoSelfRun) >= contract.SRData.ChickenRuns {
				break
			}
		}
	}

	contract.SRData.NoSelfRunLegs = len(tango) - 1
	contract.SRData.NoSelfRunCrt = tango
	contract.SRData.SelfRunLegs = len(tangoSelfRun) - 1
	contract.SRData.SelfRunCrt = tangoSelfRun

	if setStatus {
		contract.SRData.StatusStr = getSpeedrunStatusStr(contract)
	}

	log.Printf("Tango: Legs=%d  - %v, Extra Runs=%d\n", len(tango)-1, tango, (len(contract.Order)-2)-tango[len(tango)-1])
	log.Printf("Self-Run Tango: Legs=%d - %v, Extra Runs=%d\n", len(tangoSelfRun)-1, tangoSelfRun, (len(contract.Order)-1)-tango[len(tango)-1])
	//return tango, tangoSelfRun, nil
}

/*
func calculateTangoLegs(contract *Contract, setStatus bool) {

		if contract.State != ContractStateSignup {
			// We don't want this changing after the CRT starts
			return
		}
		selfRunMod := 1
		contract.SRData.NoSelfRunLegs = 0
		for selfRunMod >= 0 {
			// First calculate without speedrun flag
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
			if selfRunMod == 1 {
				contract.SRData.NoSelfRunLegs = contract.SRData.Legs
			}
			selfRunMod--
			// If not self runs, then we don't need to do this again
			if contract.Style&ContractFlagSelfRuns == 0 {
				break
			}
		}

		if setStatus {
			contract.SRData.StatusStr = getSpeedrunStatusStr(contract)
		}
	}
*/

func setSpeedrunOptions(s *discordgo.Session, channelID string, sinkCrt string, sinkBoosting string, sinkPost string, sinkPosition int, chickenRuns int, selfRuns bool, changeSinksOnly bool) (string, error) {
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

	if contract.Style&SpeedrunStyleBanker != 0 && !changeSinksOnly {

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
			contract.Banker.CrtSinkUserID = sinkCrt
			fmt.Fprintf(&builder, "CRT Sink set to %s\n", contract.Boosters[contract.Banker.CrtSinkUserID].Mention)
		}
		if sinkBoosting != "" {
			contract.Banker.BoostingSinkUserID = sinkBoosting
			if contract.State == ContractStateBanker {
				contract.Banker.CurrentBanker = contract.Banker.BoostingSinkUserID
			}
			fmt.Fprintf(&builder, "Boosting Sink set to %s\n", contract.Boosters[contract.Banker.BoostingSinkUserID].Mention)
		}
		if sinkPost != "" {
			contract.Banker.PostSinkUserID = sinkPost
			fmt.Fprintf(&builder, "Post Sink set to %s\n", contract.Boosters[contract.Banker.PostSinkUserID].Mention)
		}
		return builder.String(), nil
	}

	contract.Banker.CrtSinkUserID = sinkCrt
	contract.Banker.BoostingSinkUserID = sinkBoosting
	contract.Banker.PostSinkUserID = sinkPost
	contract.Banker.SinkBoostPosition = sinkPosition
	contract.BoostOrder = ContractOrderFair

	// This kind of contract is always a CRT
	contract.Style = ContractStyleSpeedrunBoostList

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
	fmt.Fprintf(&builder, "CRT Sink: %s\n", contract.Boosters[contract.Banker.CrtSinkUserID].Mention)
	fmt.Fprintf(&builder, "Boosting Sink: %s\n", contract.Boosters[contract.Banker.BoostingSinkUserID].Mention)
	fmt.Fprintf(&builder, "Post Sink: %s\n", contract.Boosters[contract.Banker.PostSinkUserID].Mention)

	for _, loc := range contract.Location {
		msgedit := discordgo.NewMessageEdit(loc.ChannelID, loc.ListMsgID)
		components := DrawBoostList(s, contract)
		msgedit.Components = &components
		msgedit.Flags = discordgo.MessageFlagsIsComponentsV2
		//msgedit.SetComponents(contentStr)
		//msgedit.Flags = discordgo.MessageFlagsSuppressEmbeds
		msg, err := s.ChannelMessageEditComplex(msgedit)
		if err == nil {
			loc.ListMsgID = msg.ID
		}
		if contract.State == ContractStateSignup {
			var components []discordgo.MessageComponent

			msgID := loc.ReactionID
			msg := discordgo.NewMessageEdit(loc.ChannelID, msgID)
			msg.Flags = discordgo.MessageFlagsIsComponentsV2

			contentStr, comp := GetSignupComponents(contract.State != ContractStateSignup, contract) // True to get a disabled start button
			components = append(components, &discordgo.TextDisplay{
				Content: contentStr,
			})
			components = append(components, comp...)
			msg.Components = &components
			_, _ = s.ChannelMessageEditComplex(msg)
		}
	}
	return builder.String(), nil
}

func repositionSinkBoostPosition(contract *Contract) {
	if contract.Banker.SinkBoostPosition == SinkBoostFollowOrder {
		return
	}
	// Speedrun contracts are always fair ordering over last 3 contracts
	newOrder := contract.Order

	index := slices.Index(newOrder, contract.Banker.BoostingSinkUserID)
	// Remove the speedrun starter from the list
	newOrder = append(newOrder[:index], newOrder[index+1:]...)

	if contract.Banker.SinkBoostPosition == SinkBoostFirst {
		newOrder = append([]string{contract.Banker.BoostingSinkUserID}, newOrder...)
	} else {
		newOrder = append(newOrder, contract.Banker.BoostingSinkUserID)
	}
	contract.Order = removeDuplicates(newOrder)
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
		_, redraw = buttonReactionToken(s, r.GuildID, r.ChannelID, contract, userID, 1, "")
	}

	if contract.State == ContractStateBanker {
		idx := slices.Index(contract.Boosters[r.UserID].Alts, contract.Banker.BoostingSinkUserID)
		if idx != -1 {
			// This is an alternate
			userID = contract.Boosters[r.UserID].Alts[idx]
		}
		if userID == contract.Banker.BoostingSinkUserID {
			if r.Emoji.Name == "💰" {
				_, redraw = buttonReactionBag(s, r.GuildID, r.ChannelID, contract, r.UserID)
			}
		}
	}

	if r.Emoji.Name == "🌊" {
		if time.Since(contract.ThreadRenameTime) < 3*time.Minute {
			msg, err := s.ChannelMessageSend(r.ChannelID, fmt.Sprintf("🌊 thread renaming is on cooldown, try again <t:%d:R>", contract.ThreadRenameTime.Add(3*time.Minute).Unix()))
			if err == nil {
				time.AfterFunc(10*time.Second, func() {
					err := s.ChannelMessageDelete(msg.ChannelID, msg.ID)
					if err != nil {
						log.Println(err)
					}
				})
			}
		} else {
			UpdateThreadName(s, contract)
		}
	}

	if r.Emoji.Name == "⏱️" {
		if contract.State != ContractStateCompleted {
			var data discordgo.MessageSend
			data.Content = "⏱️ can only be used after the contract completes boosting."
			data.Flags = discordgo.MessageFlagsEphemeral
			msg, err := s.ChannelMessageSendComplex(r.ChannelID, &data)
			if err == nil {
				time.AfterFunc(10*time.Second, func() {
					err := s.ChannelMessageDelete(msg.ChannelID, msg.ID)
					if err != nil {
						log.Println(err)
					}
				})
			}

		} else {
			if time.Since(contract.EstimateUpdateTime) < 2*time.Minute {
				var data discordgo.MessageSend
				data.Content = fmt.Sprintf("⏱️ duration update on cooldown, try again <t:%d:R>", contract.ThreadRenameTime.Add(3*time.Minute).Unix())
				data.Flags = discordgo.MessageFlagsEphemeral
				msg, err := s.ChannelMessageSendComplex(r.ChannelID, &data)
				if err == nil {
					time.AfterFunc(10*time.Second, func() {
						err := s.ChannelMessageDelete(msg.ChannelID, msg.ID)
						if err != nil {
							log.Println(err)
						}
					})
				}
			} else {
				log.Print("Updating estimated time")
				contract.EstimateUpdateTime = time.Now()
				go updateEstimatedTime(s, r.ChannelID, contract, true)
			}
		}
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
