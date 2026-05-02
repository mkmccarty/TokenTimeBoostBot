package boost

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
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
func predSaveRow() discordgo.ActionsRow {
	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Save",
				Emoji:    &discordgo.ComponentEmoji{Name: "💾"},
				Style:    discordgo.SuccessButton,
				CustomID: "pred#save",
			},
		},
	}
}

// predNavRow returns a select menu component for switching between views.
func predNavRow(currentID string) discordgo.ActionsRow {
	min := 1
	options := make([]discordgo.SelectMenuOption, len(predViews))
	for idx, v := range predViews {
		opt := discordgo.SelectMenuOption{
			Label:       v.label,
			Description: v.description,
			Value:       v.id,
			Default:     v.id == currentID,
		}
		if v.emojiName != "" {
			opt.Emoji = &discordgo.ComponentEmoji{Name: v.emojiName}
		}
		options[idx] = opt
	}
	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				MenuType:    discordgo.StringSelectMenu,
				CustomID:    "pred#nav",
				Placeholder: "Switch view…",
				MinValues:   &min,
				MaxValues:   1,
				Options:     options,
			},
		},
	}
}

// buildPredEmbeds returns the embeds for a given view ID.
// viewID may be a compound like "weekly:2"; the suffix encodes the weekly filter type.
func buildPredEmbeds(s *discordgo.Session, viewID, userName string, weeklyType int) []*discordgo.MessageEmbed {
	_, wedTime, friTime, _ := contractTimes9amPacific(0)
	botName := s.State.User.Username
	botIconURL := s.State.User.AvatarURL("256")
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
func GetPredCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Prediction commands.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "collectibles",
				Description: "Show Colleggtibles drop predictions.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "weekly",
				Description: "Show this week's Leggacy contract predictions.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "type",
						Description: "Filter which contract types to show.",
						Required:    false,
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{Name: "Show all Leggacy contracts", Value: int64(0)},
							{Name: "Wednesday only", Value: int64(1)},
							{Name: "Both Friday (PE + Ultra)", Value: int64(2)},
							{Name: "Non-Ultra PE only", Value: int64(3)},
							{Name: "Ultra PE only", Value: int64(4)},
						},
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "one",
				Description: "Show prediction info for a specific contract.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "contract-id",
						Description:  "Contract to look up.",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
		},
	}
}

// HandlePredCommand dispatches /pred subcommands.
func HandlePredCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		return
	}
	subCmd := options[0]
	switch subCmd.Name {
	case "weekly":
		weeklyType := 0
		for _, opt := range subCmd.Options {
			if opt.Name == "type" {
				weeklyType = int(opt.IntValue())
			}
		}
		sendPredView(s, i, subCmd.Name, weeklyType)
	case "one":
		contractID := ""
		for _, opt := range subCmd.Options {
			if opt.Name == "contract-id" {
				contractID = opt.StringValue()
			}
		}
		sendPredOne(s, i, contractID)
	default:
		sendPredView(s, i, subCmd.Name, 0)
	}
}

// HandlePredPage handles select menu and button interactions for /pred.
func HandlePredPage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Message != nil {
		if createdAt, err := discordgo.SnowflakeTimestamp(i.Message.ID); err == nil && time.Since(createdAt) > 5*time.Minute {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			empty := []discordgo.MessageComponent{}
			if i.GuildID != "" {
				_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
					Channel:    i.Message.ChannelID,
					ID:         i.Message.ID,
					Components: &empty,
				})
			} else {
				_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Components: &empty,
				})
			}
			return
		}
	}

	if i.MessageComponentData().CustomID == "pred#save" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		empty := []discordgo.MessageComponent{}
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Components: &empty,
		})
		return
	}

	values := i.MessageComponentData().Values
	if len(values) == 0 {
		return
	}
	viewID := values[0]
	embeds := buildPredEmbeds(s, viewID, interactionUserName(i), 0)
	nav := predNavRow(viewID)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     embeds,
			Components: []discordgo.MessageComponent{nav, predSaveRow()},
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Parse: []discordgo.AllowedMentionType{},
			},
		},
	})
}

// sendPredView defers, builds the requested view, and sends it as a single message.
func sendPredView(s *discordgo.Session, i *discordgo.InteractionCreate, viewID string, weeklyType int) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	compoundID := viewID
	if weeklyType > 0 {
		compoundID = fmt.Sprintf("%s:%d", viewID, weeklyType)
	}
	embeds := buildPredEmbeds(s, compoundID, interactionUserName(i), 0)
	nav := predNavRow(compoundID)

	msg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds:     embeds,
		Components: []discordgo.MessageComponent{nav, predSaveRow()},
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{},
		},
	})
	if err != nil {
		log.Printf("Error sending /pred %s: %v", viewID, err)
		return
	}

	if i.GuildID != "" {
		go func(channelID, messageID string) {
			// Debug string for the routine
			// log.Printf("Started cleanup routine for message %s in channel %s", messageID, channelID)
			time.Sleep(5 * time.Minute)
			empty := []discordgo.MessageComponent{}
			_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				Channel:    channelID,
				ID:         messageID,
				Components: &empty,
			})
		}(msg.ChannelID, msg.ID)
	}
}

// sendPredOne responds with a prediction embed for a single contract looked up by ID.
func sendPredOne(s *discordgo.Session, i *discordgo.InteractionCreate, contractID string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	c, ok := ei.EggIncContractsAll[contractID]
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Contract `%s` not found.", contractID),
		})
		return
	}

	_, wedTime, friTime, _ := contractTimes9amPacific(0)

	var wed, friPE, friUltra []ei.EggIncContract
	for _, bc := range ei.EggIncContractsAll {
		switch {
		case bc.HasPE && !bc.Ultra:
			friUltra = append(friUltra, bc)
		case bc.HasPE && bc.Ultra:
			friPE = append(friPE, bc)
		default:
			wed = append(wed, bc)
		}
	}
	sort.Slice(wed, func(a, b int) bool { return sortValidFrom(wed[a], wed[b]) })
	sort.Slice(friPE, func(a, b int) bool { return sortValidFrom(friPE[a], friPE[b]) })
	sort.Slice(friUltra, func(a, b int) bool { return sortValidFrom(friUltra[a], friUltra[b]) })

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

	botName := s.State.User.Username
	botIconURL := s.State.User.AvatarURL("256")
	userName := interactionUserName(i)
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

	var fields []*discordgo.MessageEmbedField
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%s %s `%s` %s `%dp`", ei.FindEggEmoji(c.EggName), c.Name, c.ID, iconCoop, c.MaxCoopSize),
		Value:  contractVal.String(),
		Inline: false,
	})
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Bracket",
		Value:  "_ _ " + bracketLabel,
		Inline: true,
	})
	if pos >= 0 {
		predictedDrop := baseTime.AddDate(0, 0, 7*pos)
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Predicted Drop",
			Value:  "_ _ " + bottools.WrapTimestamp(predictedDrop.Unix(), bottools.TimestampLongDate),
			Inline: true,
		})
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Queue Position",
			Value:  fmt.Sprintf("_ _ %d of %d", pos+1, len(bracket)),
			Inline: true,
		})
		if c.HasPE && !c.Ultra {
			pePos := pos + len(friPE)
			peDrop := friTime.AddDate(0, 0, 7*pePos)
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Non-Ultra Bracket",
				Value:  "_ _ PE Leggacy (Friday)",
				Inline: true,
			})
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Non-Ultra Drop",
				Value:  "_ _ " + bottools.WrapTimestamp(peDrop.Unix(), bottools.TimestampLongDate),
				Inline: true,
			})
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Non-Ultra Position",
				Value:  fmt.Sprintf("_ _ %d of %d", pePos+1, len(friUltra)+len(friPE)),
				Inline: true,
			})
		}
	}
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Last Seen",
		Value:  "_ _ " + bottools.WrapTimestamp(c.ValidFrom.Unix(), bottools.TimestampLongDate),
		Inline: true,
	})
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Duration",
		Value:  "_ _ " + bottools.FmtDuration(c.EstimatedDuration.Round(time.Minute)),
		Inline: true,
	})
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Speedrun CS",
		Value:  fmt.Sprintf("_ _ %.0f", c.Cxp),
		Inline: true,
	})
	/*
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Leggy Score",
			Value:  fmt.Sprintf("%.0f", c.CxpMax),
			Inline: true,
		})
	*/

	footer := fmt.Sprintf("%s • /pred one • User: %s", botName, userName)
	embed := &discordgo.MessageEmbed{
		Type:      discordgo.EmbedTypeRich,
		Color:     0xFFFFFF,
		Title:     "🔮 Contract Prediction",
		Fields:    fields,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Footer:    &discordgo.MessageEmbedFooter{Text: footer, IconURL: botIconURL},
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{},
		},
	})
}

func interactionUserName(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.Nick != "" {
		return i.Member.Nick
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.Username
	}
	if i.User != nil {
		return i.User.Username
	}
	return ""
}

// getWeeklyEmbeds builds a single embed with all three Leggacy types.
// Each type gets a full-width (non-inline) header field followed by inline contract fields.
// A non-inline field in Discord takes the full row width, acting as a section divider.
func getWeeklyEmbeds(wedTime, friTime time.Time, userName, botName, botIconURL string, weeklyType, embedColor int) []*discordgo.MessageEmbed {
	fridayNonUltra, fridayUltra, wednesday := predictJeli(3)
	iconCoop := ei.GetBotEmojiMarkdown("icon_coop")
	iconPE := ei.GetBotEmojiMarkdown("egg_prophecy")
	iconUltra := ei.GetBotEmojiMarkdown("ultra")

	usedSeasons := make(map[string]bool)
	timeSaverMissing := false
	var fields []*discordgo.MessageEmbedField

	addSection := func(title string, dropTime time.Time, contracts []ei.EggIncContract) {
		if len(contracts) == 0 {
			return
		}

		// Bump time-saver to second position, same as writeContracts.
		for i, c := range contracts {
			if c.ID == timeSaverContractID && i+1 < len(contracts) {
				contracts[i], contracts[i+1] = contracts[i+1], c
				break
			}
		}

		// Full-width header field, breaks the inline grid and labels the section.
		fields = append(fields, &discordgo.MessageEmbedField{
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
			if c.ID == timeSaverContractID && c.ValidFrom.Before(time.Unix(1774454400, 0)) {
				timeSaverMissing = true
				fmt.Fprintf(&v, "-# _ _🕯️ %s\n", bottools.WrapTimestamp(1774454400, bottools.TimestampShortDate))
			}
			if idx == len(contracts)-1 {
				fmt.Fprintf(&v, "_ _")
			}

			fields = append(fields, &discordgo.MessageEmbedField{
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
		legend.WriteString(seasonalEmojis.String() + " Seasonal LB")
	}
	if timeSaverMissing {
		if legend.Len() > 0 {
			legend.WriteString("  •  ")
		}
		legend.WriteString("🕯️ Missing since")
	}
	if legend.Len() > 0 {
		footer.WriteString("Legend: " + legend.String() + "\n")
	}
	footer.WriteString(botName + " • /pred weekly • User: " + userName)

	var title string
	if weeklyType == 0 {
		title = "🔮 Weekly Leggacy Prediction"
	}

	return []*discordgo.MessageEmbed{
		{
			Type:      discordgo.EmbedTypeRich,
			Color:     embedColor,
			Title:     title,
			Fields:    fields,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Footer:    &discordgo.MessageEmbedFooter{Text: footer.String(), IconURL: botIconURL},
		},
	}
}

func getCollectibleEmbeds(collectibles map[string]collectiblePrediction, userName, botName, botIconURL string, embedColor int) []*discordgo.MessageEmbed {
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

	var embeds []*discordgo.MessageEmbed
	var fields []*discordgo.MessageEmbedField
	embedSize := len("🔮 Colleggtibles Prediction")

	buildFooter := func() *discordgo.MessageEmbedFooter {
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
			footer.WriteString("Legend: " + legend.String() + "\n")
		}
		footer.WriteString(botName + " • /pred collectibles • User: " + userName)
		return &discordgo.MessageEmbedFooter{Text: footer.String(), IconURL: botIconURL}
	}

	flushEmbed := func() {
		if len(fields) == 0 {
			return
		}
		embeds = append(embeds, &discordgo.MessageEmbed{
			Type:      discordgo.EmbedTypeRich,
			Color:     embedColor,
			Title:     "🔮 Colleggtibles Prediction",
			Fields:    fields,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
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
		fields = append(fields, &discordgo.MessageEmbedField{
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
