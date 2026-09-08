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
)

// predView describes one navigable view in /pred.
type predView struct {
	id          string
	label       string
	description string
	color       int
	emojiName   string
}

var predViews = []predView{
	{id: "weekly", label: "Weekly Leggacy", description: "All predictions this week", color: 0xFFFFFF, emojiName: "🗓️"},
	{id: "weekly:1", label: "↳ Wednesday only", description: "Wednesday Leggacy contract", color: 0xFF8C00},
	{id: "weekly:2", label: "↳ Both Friday", description: "PE + Ultra PE contracts", color: 0xCC88FF},
	{id: "weekly:3", label: "↳ Non-Ultra PE only", description: "Non-Ultra PE contract only", color: 0x00C800},
	{id: "weekly:4", label: "↳ Ultra PE only", description: "Ultra PE contract only", color: 0x8000FF},
	{id: "collectibles", label: "Colleggtibles", description: "Colleggtible egg drop predictions", color: 0x0080FF, emojiName: "🎁"},
}

// predSaveRow returns an ActionsRow with a single Save button.
// predSaveRow returns an ActionRow with a single Save button.
func predSaveRow() dc.ActionRow {
	return dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "Save",
				Emoji:    &dc.Emoji{Name: "💾"},
				Style:    dc.ButtonSuccess,
				CustomID: "pred#save",
			},
		},
	}
}

// predNavRow returns a select menu component for switching between views.
func predNavRow(currentID string) dc.ActionRow {
	minValues := 1
	options := make([]dc.SelectOption, len(predViews))
	for idx, v := range predViews {
		opt := dc.SelectOption{
			Label:       v.label,
			Description: v.description,
			Value:       v.id,
			Default:     v.id == currentID,
		}
		if v.emojiName != "" {
			opt.Emoji = &dc.Emoji{Name: v.emojiName}
		}
		options[idx] = opt
	}
	return dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.SelectMenu{
				CustomID:    "pred#nav",
				Placeholder: "Switch view…",
				MinValues:   &minValues,
				MaxValues:   1,
				Options:     options,
			},
		},
	}
}

// buildPredEmbeds returns the embeds for a given view ID.
// viewID may be a compound like "weekly:2"; the suffix encodes the weekly filter type.
func buildPredEmbeds(client dc.Client, viewID, userName string, weeklyType int) []dc.Embed {
	_, wedTime, friTime, _ := contractTimes9amPacific(0)
	botName := client.BotUsername()
	botIconURL := client.BotAvatarURL("256")
	baseID, suffix, hasSuffix := strings.Cut(viewID, ":")
	wType := weeklyType
	if hasSuffix {
		if n, err := strconv.Atoi(suffix); err == nil {
			wType = n
		}
	}
	embedColor := 0xFFFFFF
	for _, v := range predViews {
		if v.id == viewID {
			embedColor = v.color
			break
		}
	}
	switch baseID {
	case "weekly":
		return getWeeklyEmbeds(wedTime, friTime, userName, botName, botIconURL, wType, embedColor)
	case "collectibles":
		return getCollectibleEmbeds(predictCollectibles(wedTime, friTime), userName, botName, botIconURL, embedColor)
	}
	return nil
}

// GetPredCommand returns the /pred command definition.
func GetPredCommand(cmd string) *dc.Command {
	command := anywhereCommand(cmd, "Prediction commands.")
	command.Options = []dc.Option{
		dc.SubCommand{
			Name:        "collectibles",
			Description: "Show Colleggtibles drop predictions.",
		},
		dc.SubCommand{
			Name:        "weekly",
			Description: "Show this week's Leggacy contract predictions.",
			Options: []dc.Option{
				dc.IntOption{
					Name:        "type",
					Description: "Filter which contract types to show.",
					Choices: []dc.Choice[int]{
						{Name: "Show all Leggacy contracts", Value: 0},
						{Name: "Wednesday only", Value: 1},
						{Name: "Both Friday (PE + Ultra)", Value: 2},
						{Name: "Non-Ultra PE only", Value: 3},
						{Name: "Ultra PE only", Value: 4},
					},
				},
			},
		},
		dc.SubCommand{
			Name:        "one",
			Description: "Show prediction info for a specific contract.",
			Options: []dc.Option{
				dc.StringOption{
					Name:         "contract-id",
					Description:  "Contract to look up.",
					Required:     true,
					Autocomplete: true,
				},
			},
		},
	}
	return &command
}

// HandlePredCommand dispatches /pred subcommands.
func HandlePredCommand(client dc.Client, e *dc.CommandEvent) {
	subCmd, ok := e.Subcommand()
	if !ok {
		return
	}
	switch subCmd {
	case "weekly":
		weeklyType := 0
		if opt, ok := e.OptInt("weekly-type"); ok {
			weeklyType = opt
		}
		sendPredView(client, e, subCmd, weeklyType)
	case "one":
		contractID := ""
		if opt, ok := e.OptString("one-contract-id"); ok {
			contractID = opt
		}
		sendPredOne(client, e, contractID)
	default:
		sendPredView(client, e, subCmd, 0)
	}
}

// HandlePredPage handles select menu and button interactions for /pred.
func HandlePredPage(client dc.Client, e *dc.ComponentEvent) {
	if messageID := e.MessageID(); messageID != "" {
		if createdAt, err := dc.SnowflakeTimestamp(messageID); err == nil && time.Since(createdAt) > 5*time.Minute {
			_ = e.DeferUpdate()
			if e.GuildID() != "" {
				_, _ = client.EditMessage(e.ChannelID(), messageID, dc.Message{ClearComponents: true})
			} else {
				_ = e.EditResponse(dc.Message{})
			}
			return
		}
	}

	if e.CustomID() == "pred#save" {
		_ = e.DeferUpdate()
		_ = e.EditResponse(dc.Message{})
		return
	}

	values := e.Values()
	if len(values) == 0 {
		return
	}
	viewID := values[0]
	embeds := buildPredEmbeds(client, viewID, eventUserName(e), 0)
	nav := predNavRow(viewID)

	_ = e.Update(dc.Message{
		Embeds:          embeds,
		Components:      []dc.LayoutComponent{nav, predSaveRow()},
		ComponentsV1:    true,
		AllowedMentions: &dc.AllowedMentions{},
	})
}

// sendPredView defers, builds the requested view, and sends it as a single message.
func sendPredView(client dc.Client, e *dc.CommandEvent, viewID string, weeklyType int) {
	_ = e.Defer(false)

	compoundID := viewID
	if weeklyType > 0 {
		compoundID = fmt.Sprintf("%s:%d", viewID, weeklyType)
	}
	embeds := buildPredEmbeds(client, compoundID, eventUserName(e), 0)
	nav := predNavRow(compoundID)

	msg, err := e.FollowupMessage(dc.Message{
		Embeds:          embeds,
		Components:      []dc.LayoutComponent{nav, predSaveRow()},
		ComponentsV1:    true,
		AllowedMentions: &dc.AllowedMentions{},
	})
	if err != nil {
		log.Printf("Error sending /pred %s: %v", viewID, err)
		return
	}

	if e.GuildID() != "" {
		go func(channelID, messageID string) {
			// Debug string for the routine
			// log.Printf("Started cleanup routine for message %s in channel %s", messageID, channelID)
			time.Sleep(5 * time.Minute)
			_, _ = client.EditMessage(channelID, messageID, dc.Message{ClearComponents: true})
		}(msg.ChannelID, msg.ID)
	}
}

// sendPredOne responds with a prediction embed for a single contract looked up by ID.
func sendPredOne(client dc.Client, e *dc.CommandEvent, contractID string) {
	_ = e.Defer(false)

	c, ok := ei.EggIncContractsAll[contractID]
	if !ok {
		_ = e.Followup(dc.Message{Content: fmt.Sprintf("Contract `%s` not found.", contractID)})
		return
	}

	_, wedTime, friTime, _ := contractTimes9amPacific(0)

	wed, friPE, friUltra := GetPredictionBrackets()

	var bracket []ei.EggIncContract
	var baseTime time.Time
	var bracketLabel string
	switch {
	case c.HasPE && !c.Ultra:
		bracket, baseTime, bracketLabel = friUltra, friTime, "Ultra PE Leggacy (Friday)"
	case c.HasPE && c.Ultra:
		bracket, baseTime, bracketLabel = friPE, friTime, "PE Leggacy (Friday)"
	default:
		bracket, baseTime, bracketLabel = wed, wedTime, "Wednesday Leggacy"
	}

	pos := -1
	for idx, bc := range bracket {
		if bc.ID == contractID {
			pos = idx
			break
		}
	}

	botName := client.BotUsername()
	botIconURL := client.BotAvatarURL("256")
	userName := eventUserName(e)
	iconCoop := ei.GetBotEmojiMarkdown("icon_coop")

	seasonLabel := ""
	if c.SeasonID != "" {
		if idx := strings.IndexByte(c.SeasonID, '_'); idx > 0 && idx < len(c.SeasonID)-1 {
			if info, ok := seasonsByKey[c.SeasonID[:idx]]; ok {
				yearShort := c.SeasonID[idx+1:]
				if len(yearShort) >= 2 {
					yearShort = yearShort[len(yearShort)-2:]
				}
				seasonLabel = fmt.Sprintf("%s %s %s", info.Emoji, info.Name, yearShort)
			}
		}
	}

	var modifiers strings.Builder
	if c.ModifierSR != 1.0 && c.ModifierSR > 0.0 {
		fmt.Fprintf(&modifiers, "🚚 Shipping Capacity %1.3gx  ", c.ModifierSR)
	}
	if c.ModifierELR != 1.0 && c.ModifierELR > 0.0 {
		fmt.Fprintf(&modifiers, "🥚 Egg Laying %1.3gx  ", c.ModifierELR)
	}
	if c.ModifierHabCap != 1.0 && c.ModifierHabCap > 0.0 {
		fmt.Fprintf(&modifiers, "🏠 Hab Capacity %1.3gx  ", c.ModifierHabCap)
	}
	if c.ModifierEarnings != 1.0 && c.ModifierEarnings > 0.0 {
		fmt.Fprintf(&modifiers, "💸 Earnings %1.3gx  ", c.ModifierEarnings)
	}
	if c.ModifierIHR != 1.0 && c.ModifierIHR > 0.0 {
		fmt.Fprintf(&modifiers, "🐣 Int. Hatchery Rate %1.3gx  ", c.ModifierIHR)
	}
	if c.ModifierAwayEarnings != 1.0 && c.ModifierAwayEarnings > 0.0 {
		fmt.Fprintf(&modifiers, "💸💤 Away Earnings %1.3gx  ", c.ModifierAwayEarnings)
	}
	if c.ModifierVehicleCost != 1.0 && c.ModifierVehicleCost > 0.0 {
		fmt.Fprintf(&modifiers, "🚗💲 Vehicle Cost %1.3gx  ", c.ModifierVehicleCost)
	}
	if c.ModifierResearchCost != 1.0 && c.ModifierResearchCost > 0.0 {
		fmt.Fprintf(&modifiers, "🔬💲 Research Cost %1.3gx  ", c.ModifierResearchCost)
	}
	if c.ModifierHabCost != 1.0 && c.ModifierHabCost > 0.0 {
		fmt.Fprintf(&modifiers, "🏗️💲 Hab Cost %1.3gx  ", c.ModifierHabCost)
	}

	var contractVal strings.Builder
	if seasonLabel != "" || modifiers.Len() > 0 {
		fmt.Fprintf(&contractVal, "_  _↳ %s %s\n", seasonLabel, strings.TrimRight(modifiers.String(), " "))
	}
	if c.Description != "" {
		fmt.Fprintf(&contractVal, "-# _  _↳ %s", c.Description)
	}

	var fields []dc.EmbedField
	fields = append(fields, dc.EmbedField{
		Name:   fmt.Sprintf("%s %s `%s` %s `%dp`", ei.FindEggEmoji(c.EggName), c.Name, c.ID, iconCoop, c.MaxCoopSize),
		Value:  contractVal.String(),
		Inline: false,
	})
	fields = append(fields, dc.EmbedField{
		Name:   "Bracket",
		Value:  "_ _ " + bracketLabel,
		Inline: true,
	})
	if pos >= 0 {
		predictedDrop := baseTime.AddDate(0, 0, 7*pos)
		fields = append(fields, dc.EmbedField{
			Name:   "Predicted Drop",
			Value:  "_ _ " + bottools.WrapTimestamp(predictedDrop.Unix(), bottools.TimestampLongDate),
			Inline: true,
		})
		fields = append(fields, dc.EmbedField{
			Name:   "Queue Position",
			Value:  fmt.Sprintf("_ _ %d of %d", pos+1, len(bracket)),
			Inline: true,
		})
		if c.HasPE && !c.Ultra {
			pePos := pos + len(friPE)
			peDrop := friTime.AddDate(0, 0, 7*pePos)
			fields = append(fields, dc.EmbedField{
				Name:   "Non-Ultra Bracket",
				Value:  "_ _ PE Leggacy (Friday)",
				Inline: true,
			})
			fields = append(fields, dc.EmbedField{
				Name:   "Non-Ultra Drop",
				Value:  "_ _ " + bottools.WrapTimestamp(peDrop.Unix(), bottools.TimestampLongDate),
				Inline: true,
			})
			fields = append(fields, dc.EmbedField{
				Name:   "Non-Ultra Position",
				Value:  fmt.Sprintf("_ _ %d of %d", pePos+1, len(friUltra)+len(friPE)),
				Inline: true,
			})
		}
	}
	fields = append(fields, dc.EmbedField{
		Name:   "Last Seen",
		Value:  "_ _ " + bottools.WrapTimestamp(c.ValidFrom.Unix(), bottools.TimestampLongDate),
		Inline: true,
	})
	fields = append(fields, dc.EmbedField{
		Name:   "Duration",
		Value:  "_ _ " + bottools.FmtDuration(c.EstimatedDuration.Round(time.Minute)),
		Inline: true,
	})
	fields = append(fields, dc.EmbedField{
		Name:   "Speedrun CS",
		Value:  fmt.Sprintf("_ _ %.0f", c.Cxp),
		Inline: true,
	})
	/*
		fields = append(fields, dc.EmbedField{
			Name:   "Leggy Score",
			Value:  fmt.Sprintf("%.0f", c.CxpMax),
			Inline: true,
		})
	*/

	footer := fmt.Sprintf("%s • /pred one • User: %s", botName, userName)
	embed := dc.Embed{
		Color:     0xFFFFFF,
		Title:     "🔮 Contract Prediction",
		Fields:    fields,
		Timestamp: time.Now().UTC(),
		Footer:    &dc.EmbedFooter{Text: footer, IconURL: botIconURL},
	}

	_ = e.Followup(dc.Message{
		Embeds:          []dc.Embed{embed},
		AllowedMentions: &dc.AllowedMentions{},
	})
}

// eventUserName is the display name to credit an interaction to: the guild
// nickname when there is one, the account name otherwise.
func eventUserName(e dc.InteractionEvent) string {
	if member := e.Member(); member != nil {
		if member.Nick != "" {
			return member.Nick
		}
		if member.User != nil {
			return member.User.Username
		}
	}
	if user := e.User(); user != nil {
		return user.Username
	}
	return ""
}

// getWeeklyEmbeds builds a single embed with all three Leggacy types.
// Each type gets a full-width (non-inline) header field followed by inline contract fields.
// A non-inline field in Discord takes the full row width, acting as a section divider.
func getWeeklyEmbeds(wedTime, friTime time.Time, userName, botName, botIconURL string, weeklyType, embedColor int) []dc.Embed {
	fridayNonUltra, fridayUltra, wednesday := predictJeli(3)
	iconCoop := ei.GetBotEmojiMarkdown("icon_coop")
	iconPE := ei.GetBotEmojiMarkdown("egg_prophecy")
	iconUltra := ei.GetBotEmojiMarkdown("ultra")

	usedSeasons := make(map[string]bool)
	timeSaverMissing := false
	var fields []dc.EmbedField

	addSection := func(title string, dropTime time.Time, contracts []ei.EggIncContract) {
		if len(contracts) == 0 {
			return
		}

		// (The time-saver-2021 hack is now applied centrally in GetPredictionBrackets)

		// Full-width header field, breaks the inline grid and labels the section.
		fields = append(fields, dc.EmbedField{
			Name:   fmt.Sprintf("%s ", title),
			Value:  fmt.Sprintf("-# _  _↳ %s", bottools.WrapTimestamp(dropTime.Unix(), bottools.TimestampShortDateTime)),
			Inline: false,
		})
		for idx, c := range contracts {
			seasonLabel := ""
			if c.SeasonID != "" {
				if idx := strings.IndexByte(c.SeasonID, '_'); idx > 0 && idx < len(c.SeasonID)-1 {
					if info, ok := seasonsByKey[c.SeasonID[:idx]]; ok {
						yearShort := c.SeasonID[idx+1:]
						if len(yearShort) >= 2 {
							yearShort = yearShort[len(yearShort)-2:]
						}
						seasonLabel = fmt.Sprintf("%s %s%s", info.Emoji, info.Code, yearShort)
						usedSeasons[info.Key] = true
					}
				}
			}

			var v strings.Builder
			if seasonLabel != "" {
				fmt.Fprintf(&v, "_ _%s `%dp` %s [⧉](https://eicoop-carpet.netlify.app/?q=%s)\n", iconCoop, c.MaxCoopSize, seasonLabel, c.ID)
			} else {
				fmt.Fprintf(&v, "_ _%s `%dp` [⧉](https://eicoop-carpet.netlify.app/?q=%s)\n", iconCoop, c.MaxCoopSize, c.ID)
			}
			fmt.Fprintf(&v, "-# _ _⏱️ Dur: **%s**\n-# _ _🏎️ CS: **%.0f**\n", bottools.FmtDuration(c.EstimatedDuration.Round(time.Minute)), c.Cxp)
			if missingMap, err := LoadMissingContracts(); err == nil {
				if ts, isMissing := missingMap[c.ID]; isMissing && c.ValidFrom.Before(time.Unix(ts, 0)) && time.Now().Unix()-ts > 15*86400 {
					timeSaverMissing = true
					fmt.Fprintf(&v, "-# _ _🕯️ %s\n", bottools.WrapTimestamp(ts, bottools.TimestampShortDate))
				}
			}
			if idx == len(contracts)-1 {
				fmt.Fprintf(&v, "_ _")
			}

			fields = append(fields, dc.EmbedField{
				Name:   fmt.Sprintf("%d. %s %s", idx+1, c.Name, ei.FindEggEmoji(c.EggName)),
				Value:  v.String(),
				Inline: true,
			})
		}
	}

	addWed := func() {
		if weeklyType == 0 || weeklyType == 1 {
			addSection("📜 Wednesday Leggacy", wedTime, wednesday)
		}
	}
	addFri := func() {
		if weeklyType == 0 || weeklyType == 2 || weeklyType == 3 {
			addSection(iconPE+" PE Leggacy", friTime, fridayNonUltra)
		}
		if weeklyType == 0 || weeklyType == 2 || weeklyType == 4 {
			addSection(iconUltra+" Ultra PE Leggacy", friTime, fridayUltra)
		}
	}
	if !wedTime.After(friTime) {
		addWed()
		addFri()
	} else {
		addFri()
		addWed()
	}

	var footer, legend, seasonalEmojis strings.Builder
	for _, s := range seasonsOrdered {
		if usedSeasons[s.Key] {
			seasonalEmojis.WriteString(s.Emoji)
		}
	}
	if seasonalEmojis.Len() > 0 {
		legend.WriteString(seasonalEmojis.String())
		legend.WriteString(" Seasonal LB")
	}
	if timeSaverMissing {
		if legend.Len() > 0 {
			legend.WriteString("  •  ")
		}
		legend.WriteString("🕯️ Missing since")
	}
	if legend.Len() > 0 {
		footer.WriteString("Legend: ")
		footer.WriteString(legend.String())
		footer.WriteString("\n")
	}
	footer.WriteString(botName)
	footer.WriteString(" • /pred weekly • User: ")
	footer.WriteString(userName)

	var title string
	if weeklyType == 0 {
		title = "🔮 Weekly Leggacy Prediction"
	}

	return []dc.Embed{
		{
			Color:     embedColor,
			Title:     title,
			Fields:    fields,
			Timestamp: time.Now().UTC(),
			Footer:    &dc.EmbedFooter{Text: footer.String(), IconURL: botIconURL},
		},
	}
}

func getCollectibleEmbeds(collectibles map[string]collectiblePrediction, userName, botName, botIconURL string, embedColor int) []dc.Embed {
	contracts := make([]collectiblePrediction, 0, len(collectibles))
	for _, p := range collectibles {
		contracts = append(contracts, p)
	}
	sort.Slice(contracts, func(i, j int) bool {
		if contracts[i].predictedTime.Equal(contracts[j].predictedTime) {
			return contracts[i].ID < contracts[j].ID
		}
		return contracts[i].predictedTime.Before(contracts[j].predictedTime)
	})

	iconCoop := ei.GetBotEmojiMarkdown("icon_coop")
	usedSeasons := make(map[string]bool)

	var embeds []dc.Embed
	var fields []dc.EmbedField
	embedSize := len("🔮 Colleggtibles Prediction")

	buildFooter := func() *dc.EmbedFooter {
		var footer, legend strings.Builder
		for _, s := range seasonsOrdered {
			if usedSeasons[s.Key] {
				legend.WriteString(s.Emoji)
			}
		}
		if legend.Len() > 0 {
			legend.WriteString(" Seasonal LB")
		}
		if legend.Len() > 0 {
			footer.WriteString("Legend: ")
			footer.WriteString(legend.String())
			footer.WriteString("\n")
		}
		footer.WriteString(botName)
		footer.WriteString(" • /pred collectibles • User: ")
		footer.WriteString(userName)
		return &dc.EmbedFooter{Text: footer.String(), IconURL: botIconURL}
	}

	flushEmbed := func() {
		if len(fields) == 0 {
			return
		}
		embeds = append(embeds, dc.Embed{
			Color:     embedColor,
			Title:     "🔮 Colleggtibles Prediction",
			Fields:    fields,
			Timestamp: time.Now().UTC(),
			Footer:    buildFooter(),
		})
		fields = nil
		embedSize = len("🔮 Colleggtibles Prediction")
	}

	for _, cc := range contracts {
		displayName := cc.EggName
		if egg, ok := ei.CustomEggMap[cc.EggName]; ok {
			displayName = egg.Name
		}

		seasonLine := ""
		if cc.SeasonID != "" {
			if idx := strings.IndexByte(cc.SeasonID, '_'); idx > 0 && idx < len(cc.SeasonID)-1 {
				if info, ok := seasonsByKey[cc.SeasonID[:idx]]; ok {
					yearShort := cc.SeasonID[idx+1:]
					if len(yearShort) >= 2 {
						yearShort = yearShort[len(yearShort)-2:]
					}
					seasonLine = fmt.Sprintf("%s %s %s", info.Emoji, info.Name, yearShort)
					usedSeasons[info.Key] = true
				}
			}
		}

		var v strings.Builder
		if egg, ok := ei.CustomEggMap[cc.EggName]; ok && len(egg.DimensionValueString) > 0 {
			fmt.Fprintf(&v, "-# _ _%s %s %s\n",
				colleggtibleDimensionEmoji(egg.Dimension),
				egg.DimensionValueString[len(egg.DimensionValueString)-1],
				colleggtibleDimensionName(egg.Dimension))
		}
		fmt.Fprintf(&v, "_ _%s `%dp` [%s](https://eicoop-carpet.netlify.app/?q=%s)\n", iconCoop, cc.MaxCoopSize, cc.Name, cc.ID)
		fmt.Fprintf(&v, "-# _ _🔮 Pred Date: %s\n", bottools.WrapTimestamp(cc.predictedTime.Unix(), bottools.TimestampShortDate))
		fmt.Fprintf(&v, "-# _ _🗓️ Last Seen: %s", bottools.WrapTimestamp(cc.ValidFrom.Unix(), bottools.TimestampShortDate))
		if seasonLine != "" {
			fmt.Fprintf(&v, "\n-# _ _%s", seasonLine)
		}

		fieldName := displayName + " " + ei.FindEggEmoji(cc.EggName)
		fieldValue := v.String()
		fieldSize := len(fieldName) + len(fieldValue)
		if len(fields) >= 24 || embedSize+fieldSize > 3900 {
			flushEmbed()
		}
		fields = append(fields, dc.EmbedField{
			Name:   fieldName,
			Value:  fieldValue,
			Inline: true,
		})
		embedSize += fieldSize
	}
	flushEmbed()
	return embeds
}

func colleggtibleDimensionName(d ei.GameModifier_GameDimension) string {
	switch d {
	case ei.GameModifier_INTERNAL_HATCHERY_RATE:
		return "Int. Hatchery Rate"
	default:
		return ei.GetGameDimensionString(d)
	}
}

func colleggtibleDimensionEmoji(d ei.GameModifier_GameDimension) string {
	switch d {
	case ei.GameModifier_EARNINGS:
		return "💸"
	case ei.GameModifier_AWAY_EARNINGS:
		return "💸💤"
	case ei.GameModifier_INTERNAL_HATCHERY_RATE:
		return "🐣"
	case ei.GameModifier_EGG_LAYING_RATE:
		return "🥚"
	case ei.GameModifier_SHIPPING_CAPACITY:
		return "🚚"
	case ei.GameModifier_HAB_CAPACITY:
		return "🏠"
	case ei.GameModifier_VEHICLE_COST:
		return "🚗💲"
	case ei.GameModifier_HAB_COST:
		return "🏗️💲"
	case ei.GameModifier_RESEARCH_COST:
		return "🔬💲"
	default:
		return "✨"
	}
}
