package boost

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashSpeedrunCommand returns the slash command for speedrun
func GetSlashSpeedrunCommand(cmd string) *dc.Command {
	runsMin, runsMax := 0, 20
	command := guildOnlyCommand(cmd, "Add speedrun features to a contract.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "sink-boosting",
			Description: "Sink during boosting.",
		},
		dc.StringOption{
			Name:        "sink-post",
			Description: "Post contract sink.",
		},
		dc.IntOption{
			Name:        "sink-position",
			Description: "Default is First Booster",
			Choices: []dc.Choice[int]{
				{Name: "First", Value: SinkBoostFirst},
				{Name: "Last", Value: SinkBoostLast},
				{Name: "Follow Order", Value: SinkBoostFollowOrder},
			},
		},
		dc.IntOption{
			Name:        "chicken-runs",
			Description: "Number of chicken runs for this contract. Optional if contract-id was selected via auto fill.",
			MinValue:    &runsMin,
			MaxValue:    &runsMax,
		},
	}
	return &command
}

// GetSlashChangeSpeedRunSinkCommand returns the slash command for changing speedrun sinks
func GetSlashChangeSpeedRunSinkCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Change speedrun sink assignements of a running contract")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "sink-boosting",
			Description: "Sink during boosting.",
		},
		dc.StringOption{
			Name:        "sink-post",
			Description: "Post contract sink.",
		},
	}
	return &command
}

// HandleChangeSpeedrunSinkCommand handles the change speedrun sink command
func HandleChangeSpeedrunSinkCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}

	sinkBoost := ""
	sinkPost := ""

	if opt, ok := e.OptString("sink-boosting"); ok {
		sinkBoost = strings.TrimSpace(opt)
		if mentionID, isMention := parseMentionUserID(sinkBoost); isMention {
			sinkBoost = mentionID
		}
	}
	if opt, ok := e.OptString("sink-post"); ok {
		sinkPost = strings.TrimSpace(opt)
		if mentionID, isMention := parseMentionUserID(sinkPost); isMention {
			sinkPost = mentionID
		}
	}

	str, err := setSpeedrunOptions(client, e.ChannelID(), sinkBoost, sinkPost, -1, -1, true)
	if err != nil {
		str = err.Error()
	}

	_ = e.Respond(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}

// HandleSpeedrunCommand handles the speedrun command
func HandleSpeedrunCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}

	chickenRuns := 0
	sinkBoost := ""
	sinkPost := ""
	sinkPosition := SinkBoostFirst

	if opt, ok := e.OptString("sink-boosting"); ok {
		sinkBoost = strings.TrimSpace(opt)
		if mentionID, isMention := parseMentionUserID(sinkBoost); isMention {
			sinkBoost = mentionID
		}
	}
	if opt, ok := e.OptString("sink-post"); ok {
		sinkPost = strings.TrimSpace(opt)
		if mentionID, isMention := parseMentionUserID(sinkPost); isMention {
			sinkPost = mentionID
		}
	}
	if opt, ok := e.OptInt("chicken-runs"); ok {
		chickenRuns = opt
	}
	if opt, ok := e.OptInt("sink-position"); ok {
		sinkPosition = opt
	}

	str, err := setSpeedrunOptions(client, e.ChannelID(), sinkBoost, sinkPost, sinkPosition, chickenRuns, false)
	if err != nil {
		str = err.Error()
	}

	_ = e.Respond(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}

func setSpeedrunOptions(client dc.Client, channelID string, sinkBoosting string, sinkPost string, sinkPosition int, chickenRuns int, changeSinksOnly bool) (string, error) {
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
		if _, err := client.User(sinkBoosting); err != nil {
			return "", errors.New("boosting sink must be a user mention for Banker style boost lists")
		}

		if _, err := client.User(sinkPost); err != nil {
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
		components := DrawBoostList(contract)
		buttonComponents := getContractReactionsComponents(contract)
		if len(buttonComponents) > 0 {
			components = append(components, buttonComponents...)
		}
		//msgedit.SetComponents(contentStr)
		//msgedit.Flags = discordgo.MessageFlagsSuppressEmbeds
		msg, err := client.EditMessage(loc.ChannelID, loc.ListMsgID, dc.Message{Components: components})
		if err == nil {
			loc.ListMsgID = msg.ID
		}
		updateSignupReactionMessage(client, contract, loc)
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

func speedrunReactions(client dc.Client, e *dc.ReactionEvent, contract *Contract) string {
	returnVal := ""
	keepReaction := false
	redraw := false

	// Token reaction handling
	tokenReactionStr := "token"
	userID := e.UserID()
	// Special handling for alt icons representing token reactions
	if strings.ToLower(e.EmojiName()) == tokenReactionStr {
		_, redraw = buttonReactionToken(client, e.GuildID(), e.ChannelID(), contract, userID, 1, "")
	}

	if contract.State == ContractStateBanker {
		// make sure Boosters[e.UserID()] exists
		if _, ok := contract.Boosters[e.UserID()]; ok {
			idx := slices.Index(contract.Boosters[e.UserID()].Alts, contract.Banker.BoostingSinkUserID)
			if idx != -1 {
				// This is an alternate
				userID = contract.Boosters[e.UserID()].Alts[idx]
			}
		}

		if userID == contract.Banker.BoostingSinkUserID {
			if e.EmojiName() == "💰" {
				_, redraw = buttonReactionBag(client, e.GuildID(), e.ChannelID(), contract, e.UserID())
			}
		}
	}

	if e.EmojiName() == "🌊" {
		if time.Since(contract.ThreadRenameTime) < 3*time.Minute {
			msg, err := client.SendMessage(e.ChannelID(), dc.Message{Content: fmt.Sprintf("🌊 thread renaming is on cooldown, try again <t:%d:R>", contract.ThreadRenameTime.Add(3*time.Minute).Unix())})
			if err == nil {
				time.AfterFunc(10*time.Second, func() {
					err := client.DeleteMessage(msg.ChannelID, msg.ID)
					if err != nil {
						log.Println(err)
					}
				})
			}
		} else {
			UpdateThreadName(client, contract)
		}
	}

	if e.EmojiName() == "⏱️" {
		if contract.State != ContractStateCompleted {
			msg, err := client.SendMessage(e.ChannelID(), dc.Message{
				Content:   "⏱️ can only be used after the contract completes boosting.",
				Ephemeral: true,
			})
			if err == nil {
				time.AfterFunc(10*time.Second, func() {
					err := client.DeleteMessage(msg.ChannelID, msg.ID)
					if err != nil {
						log.Println(err)
					}
				})
			}

		} else {
			if time.Since(contract.EstimateUpdateTime) < 2*time.Minute {
				msg, err := client.SendMessage(e.ChannelID(), dc.Message{
					Content:   fmt.Sprintf("⏱️ duration update on cooldown, try again <t:%d:R>", contract.ThreadRenameTime.Add(3*time.Minute).Unix()),
					Ephemeral: true,
				})
				if err == nil {
					time.AfterFunc(10*time.Second, func() {
						err := client.DeleteMessage(msg.ChannelID, msg.ID)
						if err != nil {
							log.Println(err)
						}
					})
				}
			} else {
				log.Print("Updating estimated time")
				contract.EstimateUpdateTime = time.Now()
				go updateEstimatedTime(client, e.ChannelID(), contract, true, e.UserID())
			}
		}
	}

	// Remove extra added emoji
	if !keepReaction {
		go RemoveAddedReaction(client, e)
	}

	if redraw {
		refreshBoostListMessage(client, contract, false)
	}

	return returnVal
}
