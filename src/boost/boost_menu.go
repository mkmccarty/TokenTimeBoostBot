package boost

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

func sandboxArtifactLabelsFromBooster(booster *Booster) []string {
	if booster == nil {
		return nil
	}

	labels := make([]string, 0, len(booster.ArtifactSet.Artifacts)+4)

	hasChalice := false
	hasMonocle := false
	hasSIAB := false
	hasIHRDefl := false
	deflQuality := ""

	for _, artifact := range booster.ArtifactSet.Artifacts {
		quality := strings.ToUpper(strings.TrimSpace(artifact.Quality))
		artifactType := strings.TrimSpace(artifact.Type)
		if quality == "" || quality == "NONE" {
			if artifact.Stones >= 3 {
				labels = append(labels, "3 Slot")
			} else if artifact.Stones == 2 {
				labels = append(labels, "2 Slot")
			}
			continue
		}

		suffix := ""
		switch {
		case artifactType == "IHR Deflector":
			suffix = "IHR Defl."
			hasIHRDefl = true
		case strings.Contains(artifactType, "Deflector"):
			suffix = "Defl."
			deflQuality = quality
		case strings.Contains(artifactType, "Metronome"):
			suffix = "Metro"
		case strings.Contains(artifactType, "Compass"):
			suffix = "Comp"
		case strings.Contains(artifactType, "Gusset"):
			suffix = "Gusset"
		case artifactType == "Chalice":
			suffix = "Chalice"
			hasChalice = true
		case artifactType == "Monocle":
			suffix = "Monocle"
			hasMonocle = true
		case artifactType == "SIAB" || strings.Contains(artifactType, "Bottle"):
			suffix = "SIAB"
			hasSIAB = true
		}

		if suffix != "" {
			labels = append(labels, fmt.Sprintf("%s %s", quality, suffix))
		} else if artifact.Stones >= 3 {
			labels = append(labels, "3 Slot")
		} else if artifact.Stones == 2 {
			labels = append(labels, "2 Slot")
		}
	}

	if !hasChalice {
		labels = append(labels, "T4L Chalice")
	}
	if !hasMonocle {
		labels = append(labels, "T4L Monocle")
	}
	if !hasSIAB {
		labels = append(labels, "3 Slot")
	}
	if !hasIHRDefl {
		if deflQuality != "" {
			labels = append(labels, fmt.Sprintf("%s IHR Defl.", deflQuality))
		} else {
			labels = append(labels, "T4L IHR Defl.")
		}
	}

	return labels
}

func sandboxPlayersFromContract(contract *Contract) []SandboxPlayer {
	if contract == nil || len(contract.Boosters) == 0 {
		return nil
	}

	orderedUserIDs := make([]string, 0, len(contract.Boosters))
	seen := make(map[string]bool, len(contract.Boosters))
	for _, userID := range contract.Order {
		if _, ok := contract.Boosters[userID]; ok {
			orderedUserIDs = append(orderedUserIDs, userID)
			seen[userID] = true
		}
	}
	if len(orderedUserIDs) < len(contract.Boosters) {
		extra := make([]string, 0, len(contract.Boosters)-len(orderedUserIDs))
		for userID := range contract.Boosters {
			if !seen[userID] {
				extra = append(extra, userID)
			}
		}
		sort.Slice(extra, func(i, j int) bool {
			return contract.Boosters[extra[i]].Register.Before(contract.Boosters[extra[j]].Register)
		})
		orderedUserIDs = append(orderedUserIDs, extra...)
	}

	players := make([]SandboxPlayer, 0, len(orderedUserIDs))
	for idx, userID := range orderedUserIDs {
		b := contract.Boosters[userID]
		if b == nil {
			continue
		}

		tokensStr := "5"
		if b.TokensWanted > 0 {
			tokensStr = strconv.Itoa(b.TokensWanted)
		}

		teStr := "50"
		if b.TECount > 0 {
			teStr = strconv.Itoa(b.TECount)
		} else if teSaved := farmerstate.GetMiscSettingString(b.UserID, "TE"); teSaved != "" {
			teStr = teSaved
		}

		name := b.Nick
		if name == "" {
			name = b.UserName
		}
		if name == "" {
			name = b.GlobalName
		}
		if name == "" {
			name = b.UserID
		}

		metro, comp, gusset, defl := "00", "00", "00", "00"
		ihrDefl, ihrSIAB, monocle, chalice := "00", "00", "00", "00"
		artifactLabels := sandboxArtifactLabelsFromBooster(b)
		if len(artifactLabels) > 0 {
			metro, comp, gusset, defl = bottools.GetSandboxItemIndices(artifactLabels)
			chalice, monocle, ihrDefl, ihrSIAB = bottools.GetSandboxIHRItemIndices(artifactLabels)
		}

		// We use the old set as we're on v-5
		// Boosted set:  old [metro][comp][gusset][defl]  → new [defl][metro][comp][gusset]
		// IHR set:      old [chal][monocle][ihrDefl][ihrSIAB] → new [ihrDefl][ihrSIAB][monocle][chal]
		players = append(players, SandboxPlayer{
			Name:         name,
			Tokens:       tokensStr,
			TE:           teStr,
			Mirror:       false,
			Colleggtible: true,
			Sink:         idx == len(orderedUserIDs)-1,
			Creator:      idx == 0,
			Item1:        metro,
			Item2:        comp,
			Item3:        gusset,
			Item4:        defl,
			Item5:        chalice,
			Item6:        monocle,
			Item7:        ihrDefl,
			Item8:        ihrSIAB,
		})
	}

	return players
}

// HandleMenuReactions handles the menu reactions for the contract.
//
// It still takes a raw session because the boost list redraw, the token
// helpers and the sandbox DM are not on the facade yet.
func HandleMenuReactions(client dc.Client, e *dc.ComponentEvent) {
	reaction := strings.Split(e.CustomID(), "#")
	contractHash := reaction[len(reaction)-1]
	contract := FindContractByHash(contractHash)

	// menu # HASH
	values := e.Values()
	if len(values) == 0 || contract == nil {
		_ = e.DeferUpdate()
		_ = e.Followup(dc.Message{})
		return
	}

	userID := e.UserID()
	cmd := strings.Split(values[0], ":")

	switch cmd[0] {
	case "tools":
		var outputStrBuilder strings.Builder
		outputStrBuilder.WriteString("## Boost Tools\n")
		fmt.Fprintf(&outputStrBuilder, "> **Boost Bot:** %s %s %s\n", bottools.GetFormattedCommand("stones"), bottools.GetFormattedCommand("calc-contract-tval"), bottools.GetFormattedCommand("coop-tval"))
		outputStrBuilder.WriteString("> **Wonky:** </auditcoop:1231383614701174814> </optimizestones:1235003878886342707> </srtracker:1158969351702069328>\n")
		outputStrBuilder.WriteString("> **Web:** \n")
		fmt.Fprintf(&outputStrBuilder, "> * [%s](%s)\n", "Staabmia Stone Calc", "https://srsandbox-staabmia.netlify.app/stone-calc")
		fmt.Fprintf(&outputStrBuilder, "> * [%s](%s)\n", "Kaylier Coop Laying Assistant", "https://ei-coop-assistant.netlify.app/laying-set")
		fmt.Fprintf(&outputStrBuilder, "> * [%s](%s)\n", "Token Farmer", "http://t-farmer.gigalixirapp.com/")
		fmt.Fprintf(&outputStrBuilder, "> * [%s](%s)\n", "Tokification: Android App for Speedrunners!", "https://github.com/ItsJustSomeDude/tokification-android/releases")
		_ = e.Respond(dc.Message{
			Content:        outputStrBuilder.String(),
			Ephemeral:      true,
			SuppressEmbeds: true,
		})
	case "sandbox":
		_ = e.DeferUpdate()

		err := SendSandboxDM(client, contract, userID)
		if err != nil {
			_ = e.Followup(dc.Message{
				Content:   fmt.Sprintf("Unable to generate SR Sandbox link: %v", err),
				Ephemeral: true,
			})
			return
		}
	case "xpost":
		var outputStrBuilder strings.Builder

		// ecoopad easter-2020-refill act
		outputStrBuilder.WriteString("\\## X-Post\n")
		outputStrBuilder.WriteString("\\# When you join:\n")
		outputStrBuilder.WriteString("\\* Equip Deflector.\n")
		outputStrBuilder.WriteString("\\* State the number of tokens needed to boost with.\n")
		outputStrBuilder.WriteString("\\* Boost.\n")
		fmt.Fprintf(&outputStrBuilder, "\necoopad %s %s\n", contract.ContractID, contract.CoopID)

		_ = e.Respond(dc.Message{
			Embeds: []dc.Embed{
				{
					Title:       "X-Post",
					Description: outputStrBuilder.String(),
					Color:       0x00cc00,
				},
			},
			Ephemeral: true,
		})
	case "tlog":
		var field []dc.EmbedField

		var logs []string
		for _, line := range contract.TokenLog {
			boostStr := ""
			if line.Boost {
				boostStr = " 🚀"
			}
			logs = append(logs, fmt.Sprintf("`%v %s %d->%s %s`", line.Time.Sub(contract.StartTime).Round(time.Second), line.FromNick, line.Quantity, boostStr, line.ToNick))
		}

		// Trim logs to the last 30 lines
		if len(logs) > 30 {
			logs = logs[len(logs)-30:]
		}

		var currentField strings.Builder
		for _, line := range logs {
			if currentField.Len()+len(line)+1 > 950 { // +1 for the newline character
				field = append(field, dc.EmbedField{
					Name:  "",
					Value: currentField.String(),
				})
				currentField.Reset()
			}
			currentField.WriteString(line)
			currentField.WriteString("\n")
		}
		if currentField.Len() > 0 {
			field = append(field, dc.EmbedField{
				Name:  "",
				Value: currentField.String(),
			})
		}

		_ = e.Respond(dc.Message{
			Embeds: []dc.Embed{{
				Title:       "Token Log",
				Description: "",
				Fields:      field,
			}},
			Ephemeral: true,
		})

	case "time":
		_ = e.Respond(dc.Message{
			Content:   "Updating boost list with estimated time...",
			Ephemeral: true,
		})
		contract.EstimateUpdateTime = time.Now()
		go updateEstimatedTime(client, e.ChannelID(), contract, false, userID)
	case "want":
		message := "**%s** wants at least 1 more token."
		contract.Boosters[userID].TokenRequestFlag = !contract.Boosters[userID].TokenRequestFlag
		if !contract.Boosters[userID].TokenRequestFlag {
			message = "**%s** now has the tokens they need."
		}
		_ = e.Respond(dc.Message{
			Content: fmt.Sprintf(message, contract.Boosters[userID].Nick),
		})
		refreshBoostListMessage(client, contract, false)
	case "send":
		_ = e.DeferUpdate()
		wantUser := cmd[1]
		_, redraw := buttonReactionToken(client, e.GuildID(), e.ChannelID(), contract, userID, 1, wantUser)
		if redraw {
			refreshBoostListMessage(client, contract, false)
		}
		sendOrUpdateUserReactionSummary(e, contract, e.UserID())
	case "next":
		_ = e.DeferUpdate()
		nextUser := cmd[1]
		_, redraw := buttonReactionToken(client, e.GuildID(), e.ChannelID(), contract, userID, 1, nextUser)
		if redraw {
			refreshBoostListMessage(client, contract, false)
		}
		sendOrUpdateUserReactionSummary(e, contract, e.UserID())
	case "grange":
		// Create a list of the grange members from contract.BoostList, with each line formatted as "MemberName (UserID)" and the join timestamp
		var grangeMembers []string
		// Create a slice of booster entries to sort
		type boosterEntry struct {
			userID  string
			booster *Booster
		}

		var entries []boosterEntry
		for userID, booster := range contract.Boosters {
			entries = append(entries, boosterEntry{userID: userID, booster: booster})
		}

		// Sort by Register time
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].booster.Register.Before(entries[j].booster.Register)
		})

		// Build the sorted list
		for _, entry := range entries {
			grangeMembers = append(grangeMembers, fmt.Sprintf("`%s` joined: <t:%d:T>", entry.booster.Nick, entry.booster.Register.Unix()))
		}
		var components []dc.LayoutComponent
		components = append(components, dc.TextDisplay{
			Content: fmt.Sprintf("# %s Grange Members\n%s", contract.Location[0].GuildContractRole.Name, strings.Join(grangeMembers, "\n")),
		})
		_ = e.Respond(dc.Message{
			Ephemeral:  true,
			Components: components,
		})
	case "mychickens":
		booster := contract.Boosters[userID]
		if booster == nil {
			_ = e.Respond(dc.Message{
				Content:   "You are not part of this contract.",
				Ephemeral: true,
			})
			return
		}

		var chickenRunList strings.Builder
		fmt.Fprintf(&chickenRunList, "# %s My Chicken Runs\n", ei.GetBotEmojiMarkdown("icon_chicken_run"))

		if len(booster.RanChickensOn) == 0 {
			chickenRunList.WriteString("You haven't run chickens for anyone yet.")
		} else {
			fmt.Fprintf(&chickenRunList, "You have run chickens for %d farmer(s):\n\n", len(booster.RanChickensOn))
			for _, requesterID := range booster.RanChickensOn {
				if requester := contract.Boosters[requesterID]; requester != nil {
					// Find the position in the boost order
					var position int
					for idx, orderUserID := range contract.Order {
						if orderUserID == requesterID {
							position = idx + 1
							break
						}
					}
					fmt.Fprintf(&chickenRunList, "* #%d - %s\n", position, requester.Mention)
				}
			}
		}

		_ = e.Respond(dc.Message{
			Content:   chickenRunList.String(),
			Ephemeral: true,
		})
	case "rancoop":
		if !UserInContract(contract, userID) {
			_ = e.Respond(dc.Message{
				Content:   "You are not part of this contract.",
				Ephemeral: true,
			})
			return
		}
		_ = e.Respond(dc.Message{
			Content:   fmt.Sprintf("%s Marked all farms that you've run chickens.", ei.GetBotEmojiMarkdown("icon_chicken_run")),
			Ephemeral: true,
		})
		buttonReactionRanCoop(client, e, contract, userID)
	case "togglerxlog":
		contract.mutex.Lock()
		booster := contract.Boosters[userID]
		msgStr := "You are not part of this contract."
		if booster != nil {
			booster.DisableEphemeralLog = !booster.DisableEphemeralLog
			if booster.DisableEphemeralLog {
				booster.NonTokenMsgID = ""
				msgStr = "🚫 **Reaction Summary Log:** Disabled for your button reactions."
			} else {
				msgStr = "📊 **Reaction Summary Log:** Enabled for your button reactions."
			}
		}
		contract.mutex.Unlock()

		_ = e.Respond(dc.Message{
			Content:   msgStr,
			Ephemeral: true,
		})
	case "adminlogs":
		creatorIDs := make([]string, 0)
		var locations []LocationData

		contract.mutex.Lock()
		creatorIDs = append(creatorIDs, contract.CreatorID...)
		locations = make([]LocationData, 0, len(contract.Location))
		for _, loc := range contract.Location {
			if loc != nil {
				locations = append(locations, *loc)
			}
		}
		contract.mutex.Unlock()

		// Check if the user is a coordinator for the contract
		isCoordinator := false
		for _, creatorID := range creatorIDs {
			if creatorID == userID {
				isCoordinator = true
				break
			}
		}
		if !isCoordinator {
			for _, loc := range locations {
				if guildstate.IsGuildCoordinator(loc.GuildID, userID) {
					isCoordinator = true
					break
				}
				perms, err := client.UserChannelPermissions(userID, loc.ChannelID)
				if err != nil {
					log.Println(err)
					continue
				}
				if perms.Administrator() {
					isCoordinator = true
					break
				}
			}
		}

		if !isCoordinator {
			_ = e.Respond(dc.Message{
				Content:   "You are not a coordinator for this contract.",
				Ephemeral: true,
			})
			return
		}

		// Fetch target channel from guild settings
		guildID := ""
		if len(locations) > 0 {
			guildID = locations[0].GuildID
		}
		if guildID == "" {
			_ = e.Respond(dc.Message{
				Content:   "Unable to determine the guild for this contract.",
				Ephemeral: true,
			})
			return
		}
		targetChannelID := guildstate.GetGuildSettingString(guildID, "admin_logs_channel")
		if targetChannelID == "" {
			_ = e.Respond(dc.Message{
				Content:   "Admin logs are not configured for this server. Contact an admin to set the `admin_logs_channel` guildstate.",
				Ephemeral: true,
			})
			return
		}

		// Pull up the contract data in the target channel
		AdminContractReport(client, e, contract, targetChannelID)
	case "swap":
		redraw := buttonReactionSwap(client, e.GuildID(), e.ChannelID(), contract, userID)
		_ = e.DeferUpdate()
		if redraw {
			refreshBoostListMessage(client, contract, false)
		}
	case "last":
		_, redraw := buttonReactionLast(client, e.GuildID(), e.ChannelID(), contract, userID)
		_ = e.DeferUpdate()
		if redraw {
			refreshBoostListMessage(client, contract, false)
		}
	case "ihrlog":
		var logLines []string
		contract.mutex.Lock()
		orderList := contract.Order
		if len(contract.OriginalOrder) > 0 {
			orderList = contract.OriginalOrder
		}
		for _, userID := range orderList {
			if b, ok := contract.Boosters[userID]; ok {
				name := b.Nick
				if name == "" {
					name = b.UserName
				}
				if name == "" {
					name = b.GlobalName
				}
				if name == "" {
					name = userID
				}
				if b.IHRCalcLog != "" {
					logLines = append(logLines, fmt.Sprintf("%s: %s", name, b.IHRCalcLog))
				} else {
					logLines = append(logLines, fmt.Sprintf("%s: IHR Rate = %0.2f", name, b.IHRRate))
				}
			}
		}
		contract.mutex.Unlock()

		if len(logLines) == 0 {
			_ = e.Respond(dc.Message{
				Content:   "No IHR calculation logs recorded for this contract.",
				Ephemeral: true,
			})
			return
		}

		contentStr := strings.Join(logLines, "\n") + "\n"

		_ = e.Respond(dc.Message{
			Content: "### IHR Calculation Details",
			Files: []dc.File{{
				Name:        "ihr_calculations.txt",
				ContentType: "text/plain",
				Reader:      strings.NewReader(contentStr),
			}},
			Ephemeral: true,
		})
	case "help":
		_ = e.Defer(true)
		buttonReactionHelp(client, e, contract)
	}
}
