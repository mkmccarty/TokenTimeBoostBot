package boost

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/xhit/go-str2duration/v2"
)

// ThreadRenameCooldown is the cooldown between Discord thread/channel renames (Discord limits PATCH /channels to 2 per 10m).
const ThreadRenameCooldown = 3 * time.Minute

// isThreadFullAndMatched returns true if the contract is full and the contract-id
// and coop-id are real (non-placeholder), meaning no further thread renames are needed.
func isThreadFullAndMatched(contract *Contract) bool {
	if contract == nil {
		return false
	}
	// A predicted contract signup doesn't have the final contract ID yet.
	if contract.PredictionSignup || contract.ContractID == "" || len(contract.PredictionsList) > 0 {
		return false
	}
	// A TBD coop ID doesn't have the final coop ID yet.
	if isTBDCoopID(contract.CoopID) || contract.CoopID == "" {
		return false
	}
	// Check if the thread is full (number of boosters is at or above coop size).
	if contract.CoopSize <= 0 || len(contract.Boosters) < contract.CoopSize {
		return false
	}
	return true
}

// AutoUpdateThreadName will automatically update a thread's name to the current contract state.
// Prediction signups (except predicted past contracts) do not auto-rename the thread.
func AutoUpdateThreadName(client dc.Client, contract *Contract) {
	if contract == nil || client == nil {
		return
	}
	if contract.PredictionSignup && !isPredictedPastContract(contract) {
		return
	}
	UpdateThreadName(client, contract)
}

// UpdateThreadName will update a threads name to the current contract state
func UpdateThreadName(client dc.Client, contract *Contract) {
	if contract == nil || client == nil {
		return
	}

	if contract.ThreadRenameFinalized {
		if !isThreadFullAndMatched(contract) {
			contract.ThreadRenameFinalized = false
		} else {
			return
		}
	}

	contract.mutex.Lock()
	if time.Since(contract.ThreadRenameTime) < ThreadRenameCooldown {
		if contract.renameTimer == nil {
			remaining := ThreadRenameCooldown - time.Since(contract.ThreadRenameTime)
			if remaining < 0 {
				remaining = 0
			}
			contract.renameTimer = time.AfterFunc(remaining, func() {
				contract.mutex.Lock()
				contract.renameTimer = nil
				contract.mutex.Unlock()
				UpdateThreadName(client, contract)
			})
		}
		contract.mutex.Unlock()
		return
	}

	if contract.renameTimer != nil {
		contract.renameTimer.Stop()
		contract.renameTimer = nil
	}
	contract.mutex.Unlock()

	desiredName := generateThreadName(contract)
	allMatchedAndFull := isThreadFullAndMatched(contract)
	hasThreadLocation := false
	allThreadsUpdated := true

	for _, loc := range contract.Location {
		if loc == nil || loc.ChannelID == "" {
			continue
		}
		ch, err := client.Channel(loc.ChannelID)
		if err == nil && ch != nil && ch.IsThread {
			hasThreadLocation = true
			// Skip editing if the thread already has the desired name
			if ch.Name == desiredName {
				continue
			}

			contract.ThreadRenameTime = time.Now()
			_, err := client.EditChannel(loc.ChannelID, desiredName)
			if err != nil {
				log.Println("Error updating thread name", err)
				allThreadsUpdated = false
				contract.mutex.Lock()
				if contract.renameTimer == nil {
					contract.renameTimer = time.AfterFunc(ThreadRenameCooldown, func() {
						contract.mutex.Lock()
						contract.renameTimer = nil
						contract.mutex.Unlock()
						UpdateThreadName(client, contract)
					})
				}
				contract.mutex.Unlock()
			}
		}
	}

	if allMatchedAndFull && hasThreadLocation && allThreadsUpdated {
		contract.ThreadRenameFinalized = true
		saveData(contract.ContractHash)
	}
}

// HandleBoostCommand will handle the /boost command
func HandleBoostCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}
	var str = "Boosting!!"
	targetID := e.UserID()
	contract := FindContract(e.ChannelID())
	if contract != nil {
		targetID = resolveContractUserID(client, contract, e.UserID())
	}
	var err = UserBoost(client, e.GuildID(), e.ChannelID(), targetID)
	if err != nil {
		str = err.Error()
	}
	_ = e.Respond(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}

// HandleUnboostCommand will handle the /unboost command
func HandleUnboostCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}
	var str string
	var farmer = ""

	if opt, ok := e.OptString("farmer"); ok {
		farmer = opt
	}
	contract := FindContract(e.ChannelID())
	if contract != nil {
		if farmer == "" {
			targetID := resolveContractUserID(client, contract, e.UserID())
			farmer = "<@" + targetID + ">"
		} else {
			inputID := normalizeUserIDInput(farmer)
			if inputID == e.UserID() && !UserInContract(contract, e.UserID()) {
				targetID := resolveContractUserID(client, contract, e.UserID())
				if UserInContract(contract, targetID) {
					farmer = "<@" + targetID + ">"
				}
			}
		}
	}
	var err = Unboost(client, e.GuildID(), e.ChannelID(), farmer)
	if err != nil {
		str = err.Error()
	} else {
		str = "Marked " + farmer + " as unboosted."
	}

	_ = e.Respond(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}

// HandleSkipCommand will handle the /skip command
func HandleSkipCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}
	var str = "Skip to Next Booster"
	var err = SkipBooster(client, e.GuildID(), e.ChannelID(), "")
	if err != nil {
		str = err.Error()
	}

	_ = e.Respond(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}

// ParsedFarmer holds the categorized input for a farmer being added to a contract
type ParsedFarmer struct {
	Mention string
	Guest   string
}

// ParseFarmerInput splits and categorizes a comma-separated string of farmers
func ParseFarmerInput(farmerInput string) []ParsedFarmer {
	var parsed []ParsedFarmer
	if farmerInput == "" {
		return parsed
	}
	// Insert commas between mentions that are adjacent or only separated by whitespace
	re := regexp.MustCompile(`>\s*<@`)
	farmerInput = re.ReplaceAllString(farmerInput, ">, <@")

	farmers := strings.Split(farmerInput, ",")
	for _, fRaw := range farmers {
		f := strings.TrimSpace(fRaw)
		if f == "" {
			continue
		}
		var p ParsedFarmer
		if _, isMention := parseMentionUserID(f); isMention {
			p.Mention = f
		} else {
			p.Guest = f
		}
		parsed = append(parsed, p)
	}
	return parsed
}

// HandleJoinCommand will handle the /join command
func HandleJoinCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}
	var farmerInput string
	var orderValue = ContractOrderTimeBased // Default to Time Based
	var str = "Joining Member"
	var tokenWant = 0
	var alreadyBoosted = false
	var role = ""

	if opt, ok := e.OptString("farmer"); ok {
		farmerInput = opt
		str += " " + farmerInput
	} else {
		effectiveID := farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID())
		farmerInput = "<@" + effectiveID + ">"
		str += " " + farmerInput
	}

	if opt, ok := e.OptInt("token-count"); ok {
		tokenWant = opt
		str += " with " + fmt.Sprintf("%d", tokenWant) + " boost tokens"
	}
	if opt, ok := e.OptInt("boost-order"); ok {
		orderValue = opt
	}
	if opt, ok := e.OptBool("already-boosted"); ok {
		alreadyBoosted = opt
		if alreadyBoosted {
			str += " (already boosted)"
		}
	}
	if opt, ok := e.OptString("role"); ok && opt != "" {
		role = opt
		if strings.EqualFold(role, "Helper") {
			str += " as Helper"
		} else {
			str += " as Main"
		}
	}
	sticky, hasSticky := e.OptBool("sticky")
	if hasSticky && sticky && role != "" {
		str += " (sticky)"
	}

	_ = e.Defer(true)

	if farmerInput != "" {
		parsedFarmers := ParseFarmerInput(farmerInput)
		for _, p := range parsedFarmers {
			if p.Mention != "" {
				mID := normalizeUserIDInput(p.Mention)
				if mID == e.UserID() {
					effID := farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID())
					if effID != e.UserID() {
						p.Mention = "<@" + effID + ">"
					}
				}
			}
			if tokenWant != 0 {
				if p.Guest != "" {
					farmerstate.SetTokens(p.Guest, tokenWant)
				} else if p.Mention != "" {
					farmerstate.SetTokens(normalizeUserIDInput(p.Mention), tokenWant)
				}
			}
			if hasSticky && sticky && role != "" {
				targetID := p.Guest
				if p.Mention != "" {
					targetID = normalizeUserIDInput(p.Mention)
				}
				if targetID != "" {
					farmerstate.SetOrderHelper(targetID, strings.EqualFold(role, "Helper"))
				}
			}
			var err = AddContractMember(client, e.GuildID(), e.ChannelID(), callerMention(e), p.Mention, p.Guest, orderValue, alreadyBoosted, role)
			if err != nil {
				str = err.Error()
			}
		}
	}

	// Refresh the boost list to show updated TE for TE-ordered contracts
	contract := FindContract(e.ChannelID())
	if contract != nil {
		reorderBoosters(contract)
		refreshBoostListMessage(client, contract, false)
		saveData(contract.ContractHash)
	}

	_ = e.Followup(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}

// callerMention is the mention string for the user who ran the command, which
// several contract helpers take as the acting coordinator.
func callerMention(e *dc.CommandEvent) string {
	if u := e.User(); u != nil {
		return u.Mention()
	}
	return ""
}

// HandlePruneCommand will handle the /prune command
func HandlePruneCommand(client dc.Client, e *dc.CommandEvent) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}
	var str = "Prune Booster"
	var farmer = ""

	if opt, ok := e.OptString("farmer"); ok {
		farmer = opt
		str += " " + farmer
	}
	contract := FindContract(e.ChannelID())
	if contract != nil {
		if farmer == "" {
			targetID := resolveContractUserID(client, contract, e.UserID())
			farmer = "<@" + targetID + ">"
		} else {
			inputID := normalizeUserIDInput(farmer)
			if inputID == e.UserID() && !UserInContract(contract, e.UserID()) {
				targetID := resolveContractUserID(client, contract, e.UserID())
				if UserInContract(contract, targetID) {
					farmer = "<@" + targetID + ">"
				}
			}
		}
	}
	_ = e.Defer(true)

	var err = RemoveFarmerByMention(client, e.GuildID(), e.ChannelID(), callerMention(e), farmer)
	if err != nil {
		log.Println("/prune", err.Error())
		str = err.Error()
	}
	_ = e.Followup(dc.Message{Content: str})
}

// GetSlashCoopETACommand returns the command definition for the coopeta command
func GetSlashCoopETACommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Display contract completion estimate.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:        "rate",
			Description: "Hourly production rate (i.e. 15.7q)",
			Required:    true,
		},
		dc.StringOption{
			Name:        "timespan",
			Description: "Time remaining in this contract. Example: 0d7h27m.",
			Required:    true,
		},
	}
	return &command
}

// HandleCoopETACommand will handle the /coopeta command
func HandleCoopETACommand(e *dc.CommandEvent) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}
	var rate = ""
	var t = time.Now()
	var timespan = ""

	if opt, ok := e.OptString("rate"); ok {
		rate = opt
	}
	if opt, ok := e.OptString("timespan"); ok {
		timespan = opt
	}

	dur, _ := str2duration.ParseDuration(timespan)
	endTime := t.Add(dur)

	_ = e.Respond(dc.Message{
		Content: fmt.Sprintf("With a production rate of %s/hr completion <t:%d:R> near <t:%d:f>", rate, endTime.Unix(), endTime.Unix()),
	})
}

// HandleBumpCommand will handle the /bump command
func HandleBumpCommand(client dc.Client, e *dc.CommandEvent) {
	str := "Contract not found"
	_ = e.Defer(true)
	contract := FindContract(e.ChannelID())
	if contract != nil {

		{
			str = "Boost list moved."
			err := RedrawBoostList(client, e.GuildID(), e.ChannelID())
			if err != nil {
				str = err.Error()
			}
		}
		if contract.CoopTokenValueMsgID != "" {
			HandleCoopTvalCommand(client, e)
		}

	}

	msg, err := e.FollowupMessage(dc.Message{
		Ephemeral: true,
		Content:   str,
	})
	if err == nil {
		_ = e.DeleteFollowup(msg.ID)
	}
}

// HandleBumpCRCommand will handle the /bump-cr command
func HandleBumpCRCommand(client dc.Client, e *dc.CommandEvent) {
	str := "Contract not found"
	_ = e.Defer(true)
	contract := FindContract(e.ChannelID())
	if contract != nil {
		str = "CR messages moved."
		bumpCRMessages(client, contract)
	}
	// Wait a moment
	time.Sleep(2000 * time.Millisecond)
	msg, err := e.FollowupMessage(dc.Message{
		Ephemeral: true,
		Content:   str,
	})
	if err == nil {
		_ = e.DeleteFollowup(msg.ID)
	}
}

// HandleToggleContractPingsCommand will handle the /toggle-contract-pings command
func HandleToggleContractPingsCommand(client dc.Client, e *dc.CommandEvent) {
	str := "Contract not found"
	_ = e.Defer(true)
	userID := e.UserID()
	guildID := e.GuildID()
	contract := FindContract(e.ChannelID())

	UserInContract := UserInContract(contract, userID)
	if contract != nil && UserInContract {
		value := farmerstate.GetMiscSettingFlag(userID, "SuppressContractPings")
		value = !value
		farmerstate.SetMiscSettingFlag(userID, "SuppressContractPings", value)
		for _, loc := range contract.Location {
			if loc.GuildID == guildID {
				if value {
					str = fmt.Sprintf("Suppressing contract pings.\nRemoving you from this contract's %s role.", loc.GuildContractRole.Name)
					_ = client.RemoveGuildMemberRole(guildID, userID, loc.GuildContractRole.ID)
				} else {
					str = fmt.Sprintf("Enabling contract pings.\nAdding you to this contract's %s role.", loc.GuildContractRole.Name)
					_ = client.AddGuildMemberRole(guildID, userID, loc.GuildContractRole.ID)
				}
			}
		}
	} else {
		str = "You are not in this contract."
	}

	_ = e.Followup(dc.Message{
		Ephemeral: true,
		Content:   str,
	})
}

// HandleRestartContract recycles the current contract and recreates it with the same
// participants, planned start time, and run style.
func HandleRestartContract(client dc.Client, e *dc.ComponentEvent) {
	_ = e.DeferUpdate()

	channelID := e.ChannelID()
	guildID := e.GuildID()
	var str = "Contract not found."
	contract := FindContract(channelID)

	if contract != nil {
		if !creatorOfContract(client, contract, e.UserID()) {
			str = "Only the coordinator can restart this contract."
		} else {
			// Capture state before deletion
			contractID := contract.ContractID
			coopID := contract.CoopID
			playStyle := contract.PlayStyle
			coopSize := contract.CoopSize
			progenitors := contract.Order
			if contract.State != ContractStateSignup && len(contract.OriginalOrder) > 0 {
				progenitors = contract.OriginalOrder
			}
			validFrom := contract.ValidFrom
			savedStyle := contract.Style
			thresholdX := contract.ThresholdTokensX
			thresholdY := contract.ThresholdTokensY
			thresholdA := contract.ThresholdTokensA

			// Original coordinator
			originalCoordinatorID := contract.CreatorID[0]

			_, err := DeleteContract(client, guildID, channelID)
			if err != nil {
				str = "Failed to delete contract: " + err.Error()
			} else {
				mutex.Lock()
				newContract, err := CreateContract(client, contractID, coopID, playStyle, coopSize, ContractOrderSignup, guildID, channelID, progenitors, originalCoordinatorID, time.Time{}, validFrom)
				mutex.Unlock()

				if err != nil {
					str = "Failed to restart contract: " + err.Error()
				} else {
					newContract.Style = savedStyle
					newContract.BoostOrder = ContractOrderSignup
					newContract.ThresholdTokensX = thresholdX
					newContract.ThresholdTokensY = thresholdY
					newContract.ThresholdTokensA = thresholdA

					reorderBoosters(newContract)
					saveData(newContract.ContractHash)

					CheckAndPublishAMQPContractUpdate(newContract)

					createMsg := DrawBoostList(newContract)
					buttonComponents := getContractReactionsComponents(newContract)
					if len(buttonComponents) > 0 {
						createMsg = append(createMsg, buttonComponents...)
					}
					msg, err := client.SendMessage(channelID, dc.Message{Components: createMsg})
					if err == nil {
						SetListMessageID(newContract, channelID, msg.ID)

						contentStr, comp := GetSignupComponents(newContract)
						var components []dc.LayoutComponent
						components = append(components, dc.TextDisplay{Content: contentStr})
						components = append(components, comp...)

						reactionMsg, err := client.SendMessage(channelID, dc.Message{Components: components})
						if err == nil {
							SetReactionID(newContract, channelID, reactionMsg.ID)
							_ = client.PinMessage(channelID, reactionMsg.ID)
						}
					}

					boostOrder := ContractOrderSignup
					orderName := fmt.Sprintf("%d", boostOrder)
					if boostOrder >= 0 && boostOrder < len(contractOrderNames) {
						orderName = contractOrderNames[boostOrder]
					}
					playStyleName := contractPlaystyleNames[ContractPlaystyleUnset]
					if playStyle >= 0 && playStyle < len(contractPlaystyleNames) {
						playStyleName = contractPlaystyleNames[playStyle]
					}

					var styleFlags []string
					for _, f := range contractFlagNames {
						if savedStyle&f.Flag != 0 {
							styleFlags = append(styleFlags, f.Name)
						}
					}

					var b strings.Builder
					fmt.Fprintf(&b, "**%s/%s** restarted with **%d** farmers\n", contractID, coopID, len(progenitors))
					fmt.Fprintf(&b, "Style: %s | Order: %s", playStyleName, orderName)
					if len(styleFlags) > 0 {
						fmt.Fprintf(&b, " | Flags: %s", strings.Join(styleFlags, ", "))
					}
					str = b.String()
				}
			}
		}
	}
	_, _ = client.SendMessage(channelID, dc.Message{Content: str})
}

// HandleContractDelete facilitates the deletion of a channel contract
func HandleContractDelete(client dc.Client, e *dc.ComponentEvent) {
	// Delete coop
	var str = "Contract not found."
	// if user is contract coordinator
	contract := FindContract(e.ChannelID())

	if contract != nil {

		if creatorOfContract(client, contract, e.UserID()) {

			coopName, err := DeleteContract(client, e.GuildID(), e.ChannelID())
			if err == nil {
				str = fmt.Sprintf("Contract %s recycled.", coopName)
			}
			for _, loc := range contract.Location {
				_ = client.UnpinMessage(loc.ChannelID, loc.ReactionID)
			}
			_ = client.DeleteMessage(e.ChannelID(), e.MessageID())
		} else {
			str = "Only the coordinator can recycle this contract."
		}
	}

	_ = e.Respond(dc.Message{
		Content:   str,
		Ephemeral: true,
	})
}
