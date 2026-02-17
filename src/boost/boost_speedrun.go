package boost

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

var integerZeroMinValue float64 = 0.0

// GetSlashSpeedrunCommand returns the slash command for speedrun
func GetSlashSpeedrunCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Add speedrun features to a contract.",
		Options: []*discordgo.ApplicationCommandOption{
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

	sinkBoost := ""
	sinkPost := ""
	optionMap := bottools.GetCommandOptionsMap(i)

	if opt, ok := optionMap["sink-boosting"]; ok {
		sinkBoost = strings.TrimSpace(opt.StringValue())
		if mentionID, isMention := parseMentionUserID(sinkBoost); isMention {
			sinkBoost = mentionID
		}
	}
	if opt, ok := optionMap["sink-post"]; ok {
		sinkPost = strings.TrimSpace(opt.StringValue())
		if mentionID, isMention := parseMentionUserID(sinkPost); isMention {
			sinkPost = mentionID
		}
	}

	str, err := setSpeedrunOptions(s, i.ChannelID, sinkBoost, sinkPost, -1, -1, true)
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
	sinkBoost := ""
	sinkPost := ""
	sinkPosition := SinkBoostFirst

	optionMap := bottools.GetCommandOptionsMap(i)

	if opt, ok := optionMap["sink-boosting"]; ok {
		sinkPost = strings.TrimSpace(opt.StringValue())
		if mentionID, isMention := parseMentionUserID(sinkBoost); isMention {
			sinkBoost = mentionID
		}
	}
	if opt, ok := optionMap["sink-post"]; ok {
		sinkPost = strings.TrimSpace(opt.StringValue())
		if mentionID, isMention := parseMentionUserID(sinkPost); isMention {
			sinkPost = mentionID
		}
	}
	if opt, ok := optionMap["chicken-runs"]; ok {
		chickenRuns = int(opt.IntValue())
	}
	//if opt, ok := optionMap["self-runs"]; ok {
	//	selfRuns = opt.BoolValue()
	//}
	if opt, ok := optionMap["sink-position"]; ok {
		sinkPosition = int(opt.IntValue())
	}

	str, err := setSpeedrunOptions(s, i.ChannelID, sinkBoost, sinkPost, sinkPosition, chickenRuns, false)
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

func setSpeedrunOptions(s *discordgo.Session, channelID string, sinkBoosting string, sinkPost string, sinkPosition int, chickenRuns int, changeSinksOnly bool) (string, error) {
	var contract = FindContract(channelID)
	if contract == nil {
		return "", errors.New(errorNoContract)
	}

	if contract.State != ContractStateSignup && !changeSinksOnly {
		return "", errors.New("contract must be in the Sign-up state to set speedrun options")
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

	if changeSinksOnly {
		var builder strings.Builder
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

	contract.Banker.BoostingSinkUserID = sinkBoosting
	contract.Banker.PostSinkUserID = sinkPost
	contract.Banker.SinkBoostPosition = sinkPosition
	contract.BoostOrder = ContractOrderFair

	contract.Style = ContractStyleFastrunBanker

	contract.Style &= ^ContractFlagSelfRuns

	// Chicken Runs Calc
	// Info from https://egg-inc.fandom.com/wiki/Contracts
	if chickenRuns != 0 {
		contract.ChickenRuns = chickenRuns
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Speedrun options set for %s/%s\n", contract.ContractID, contract.CoopID)
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
		updateSignupReactionMessage(s, contract, loc)
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
		// make sure Boosters[r.UserID] exists
		if _, ok := contract.Boosters[r.UserID]; ok {
			idx := slices.Index(contract.Boosters[r.UserID].Alts, contract.Banker.BoostingSinkUserID)
			if idx != -1 {
				// This is an alternate
				userID = contract.Boosters[r.UserID].Alts[idx]
			}
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
		refreshBoostListMessage(s, contract, false)
	}

	return returnVal
}
