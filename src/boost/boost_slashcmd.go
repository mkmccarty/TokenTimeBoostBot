package boost

import (
	"encoding/binary"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/xhit/go-str2duration/v2"
)

// tokenSerialToInt extracts a stable int identifier from a token serial string,
// supporting both uuid strings (using the low 31 bits) and legacy xid strings.
func tokenSerialToInt(serial string) int32 {
	if u, err := uuid.Parse(serial); err == nil {
		// u is a [16]byte array; extract the last 4 bytes as a positive int32
		raw := binary.BigEndian.Uint32(u[12:16])
		return int32(raw & 0x7FFFFFFF)
	}
	// Fallback for legacy serial formats if any
	return 0
}

// UpdateThreadName will update a threads name to the current contract state
func UpdateThreadName(client dc.Client, contract *Contract) {
	if contract == nil {
		return
	}

	contract.ThreadRenameTime = time.Now()

	var builder strings.Builder
	builder.WriteString(generateThreadName(contract))
	contract.ThreadRenameTime = time.Now()

	for _, loc := range contract.Location {
		ch, err := client.Channel(loc.ChannelID)
		if err == nil {

			if ch.IsThread {
				_, err := client.EditChannel(loc.ChannelID, builder.String())
				if err != nil {
					log.Println("Error updating thread name", err)
				}
			}
		}
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
	var err = UserBoost(client, e.GuildID(), e.ChannelID(), e.UserID())
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
	var farmerInput = ""
	var orderValue = ContractOrderTimeBased // Default to Time Based
	var str = "Joining Member"
	var tokenWant = 0
	var alreadyBoosted = false

	if opt, ok := e.OptString("farmer"); ok {
		farmerInput = opt
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

	_ = e.Defer(true)

	if farmerInput != "" {
		parsedFarmers := ParseFarmerInput(farmerInput)
		for _, p := range parsedFarmers {
			if tokenWant != 0 {
				if p.Guest != "" {
					farmerstate.SetTokens(p.Guest, tokenWant)
				} else if p.Mention != "" {
					farmerstate.SetTokens(normalizeUserIDInput(p.Mention), tokenWant)
				}
			}
			var err = AddContractMember(client, e.GuildID(), e.ChannelID(), callerMention(e), p.Mention, p.Guest, orderValue, alreadyBoosted)
			if err != nil {
				str = err.Error()
			}
		}
	}

	// Refresh the boost list to show updated TE for TE-ordered contracts
	contract := FindContract(e.ChannelID())
	if contract != nil {
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

// tokenListAutoCompleteChoices offers the contract this channel is running as
// the tracker to adjust.
func tokenListAutoCompleteChoices(e *dc.AutocompleteEvent) []dc.Choice[string] {
	choices := make([]dc.Choice[string], 0)

	c := FindContract(e.ChannelID())
	if c == nil {
		return choices
	}

	choices = append(choices, dc.Choice[string]{
		Name:  c.ContractID + "/" + c.CoopID,
		Value: c.CoopID,
	})

	return choices
}

// tokenIDAutoCompleteChoices offers the caller's own recent tokens, newest
// last, identified by the counter embedded in the token serial.
func tokenIDAutoCompleteChoices(e *dc.AutocompleteEvent) []dc.Choice[int] {
	choices := make([]dc.Choice[int], 0)

	c := FindContract(e.ChannelID())
	if c == nil {
		return choices
	}

	var myTokes []ei.TokenUnitLog
	for _, t := range c.TokenLog {
		if t.FromUserID == e.UserID() {
			t.Value = bottools.GetTokenValue(t.Time.Sub(c.StartTime).Seconds(), c.EstimatedDuration.Seconds()) * float64(t.Quantity)
			myTokes = append(myTokes, t)
		}
	}
	// Trim myTokes to last 10
	if len(myTokes) > 15 {
		myTokes = myTokes[len(myTokes)-15:]
	}

	for _, t := range myTokes {
		choices = append(choices, dc.Choice[int]{
			Name:  fmt.Sprintf("%ds ago %s - %d @ %2.3f", int(time.Since(t.Time).Seconds()), t.ToNick, t.Quantity, t.Value),
			Value: int(tokenSerialToInt(t.Serial)),
		})
	}

	return choices
}

// tokenReceiverAutoCompleteChoices offers the coop's boosters, filtered by
// whatever the caller has typed so far.
func tokenReceiverAutoCompleteChoices(e *dc.AutocompleteEvent) []dc.Choice[string] {
	choices := make([]dc.Choice[string], 0)

	c := FindContract(e.ChannelID())
	if c == nil {
		return choices
	}
	searchString := ""

	if opt, ok := e.OptString("new-receiver"); ok {
		searchString = opt
	}

	// Want a set of sorted keys from c.Boosters
	// Sort by Nick
	keys := make([]string, 0, len(c.Boosters))
	for k := range c.Boosters {

		if searchString == "" || strings.Contains(strings.ToLower(c.Boosters[k].Nick), strings.ToLower(searchString)) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, b := range keys {
		choices = append(choices, dc.Choice[string]{
			Name:  c.Boosters[b].Nick,
			Value: b,
		})
		if len(choices) > 15 {
			break
		}
	}
	return choices
}

// HandleTokenEditAutoComplete will handle the /token-edit autocomplete
func HandleTokenEditAutoComplete(e *dc.AutocompleteEvent) {
	focused, _ := e.FocusedOption()
	switch focused {
	case "list":
		_ = e.RespondChoices(tokenListAutoCompleteChoices(e))
	case "id":
		if err := e.RespondChoicesInt(tokenIDAutoCompleteChoices(e)); err != nil {
			log.Println(err.Error())
		}
	case "new-receiver":
		if err := e.RespondChoices(tokenReceiverAutoCompleteChoices(e)); err != nil {
			log.Println(err.Error())
		}
	}
}

// HandleTokenEditCommand will handle the /token-edit command
func HandleTokenEditCommand(client dc.Client, e *dc.CommandEvent) string {
	userID := e.UserID()
	c := FindContract(e.ChannelID())
	if c == nil {
		return "Contract not found."
	}
	if !UserInContract(c, userID) {
		return "You are not in this contract."
	}
	var action int // 0:Move, 1: Delete, 2 Modify Count
	var tokenIndex int32
	var boosterIndex string
	var tokenCount int
	if opt, ok := e.OptInt("action"); ok {
		action = opt
	}
	if opt, ok := e.OptInt("id"); ok {
		tokenIndex = int32(opt)
	}
	if opt, ok := e.OptString("new-receiver"); ok {
		boosterIndex = opt
	}
	if opt, ok := e.OptInt("new-quantity"); ok {
		tokenCount = opt
	}

	str := "Token not found"
	c.mutex.Lock()
	if action == 0 { // Move
		for i, t := range c.TokenLog {
			if tokenSerialToInt(t.Serial) == tokenIndex {
				c.TokenLog[i].ToUserID = c.Boosters[boosterIndex].UserID
				c.TokenLog[i].ToNick = c.Boosters[boosterIndex].Nick
				str = fmt.Sprintf("Token moved to %s", c.TokenLog[i].ToNick)
				break
			}
		}
	} else if action == 1 { // Delete str = "Token not found"
		for i, t := range c.TokenLog {
			if tokenSerialToInt(t.Serial) == tokenIndex {
				c.TokenLog = append(c.TokenLog[:i], c.TokenLog[i+1:]...)
				str = "Token deleted"
				break
			}
		}
	} else if action == 2 { // Modify Count
		for i, t := range c.TokenLog {
			if tokenSerialToInt(t.Serial) == tokenIndex {
				c.TokenLog[i].Quantity = tokenCount
				c.TokenLog[i].Value = bottools.GetTokenValue(c.TokenLog[i].Time.Sub(c.StartTime).Seconds(), c.EstimatedDuration.Seconds()) * float64(c.TokenLog[i].Quantity)
				str = "Token count modified"
				break
			}
		}
	}
	// Recalculate token values after the change
	calculateTokenValueCoopLog(c, c.EstimatedDuration)

	c.mutex.Unlock()
	saveData(c.ContractHash)
	refreshBoostListMessage(client, c, false)
	return str
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
