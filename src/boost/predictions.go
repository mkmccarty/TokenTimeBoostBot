package boost

import (
	"fmt"
	"log"
	"maps"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// ACO's contract channels
const (
	acoGuild        string = "485162044652388384"
	leggacyCategory string = "1148948250381004840"
	ultraCategory   string = "1131573572284973086"
)

type predictionType string

const (
	predictionAll             predictionType = "all"
	predictionWedLegacy       predictionType = "wed_legacy"
	predictionFriPeLegacyBoth predictionType = "fri_pe_legacy_both"
	predictionFriUltraLegacy  predictionType = "fri_ultra_legacy"
	predictionFriNonUltra     predictionType = "fri_non_ultra_legacy"
	predictionCollectibles    predictionType = "collectibles"
)

// flags returns which Leggacy prediction types to show based on the predictionType
func (p predictionType) flags() (showWednesday, showFridayNonUltra, showFridayUltra bool) {
	showWednesday, showFridayNonUltra, showFridayUltra = true, true, true
	switch p {
	case predictionWedLegacy:
		showFridayNonUltra = false
		showFridayUltra = false
	case predictionFriPeLegacyBoth:
		showWednesday = false
	case predictionFriUltraLegacy:
		showWednesday = false
		showFridayNonUltra = false
	case predictionFriNonUltra:
		showWednesday = false
		showFridayUltra = false
	}
	return
}

// GetPredictionsCommand returns the command for the /predictions command
func GetPredictionsCommand(cmd string) *discordgo.ApplicationCommand {
	minValue := 1.0
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Get predictions for the following week's leggacy contracts.",
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
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "prediction-type",
				Description: "Which prediction type to show (default: all).",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "All Leggacies", Value: string(predictionAll)},
					{Name: "Wednesday Leggacy", Value: string(predictionWedLegacy)},
					{Name: "Friday PE Leggacies", Value: string(predictionFriPeLegacyBoth)},
					{Name: "Friday Leggacy", Value: string(predictionFriNonUltra)},
					{Name: "Friday Ultra Leggacy", Value: string(predictionFriUltraLegacy)},
					{Name: "Colleggtibles", Value: string(predictionCollectibles)},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "contract-count",
				Description: "Contract count per category (default 3).",
				Required:    false,
				MinValue:    &minValue,
				MaxValue:    5.0,
			},
		},
	}
}

// HandlePredictionsCommand will handle the /predictions command
func HandlePredictionsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	flags := discordgo.MessageFlagsIsComponentsV2
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

	optionMap := bottools.GetCommandOptionsMap(i)
	var components []discordgo.MessageComponent

	pt := predictionAll
	// Check for ACO guild's categories
	if i.GuildID == acoGuild {
		categoryID, err := bottools.FindCategoryID(s, i.ChannelID)
		if err == nil {
			switch categoryID {
			case ultraCategory:
				pt = predictionFriUltraLegacy
			case leggacyCategory:
				now := time.Now()
				if isNextWedSoonerThanNextFri(now, KevinLoc) {
					pt = predictionWedLegacy
				} else {
					pt = predictionFriNonUltra
				}
			}
		}
	}
	showButtons := true
	if opt, ok := optionMap["prediction-type"]; ok {
		pt = predictionType(opt.StringValue())
		showButtons = false
	}
	components = predictions(optionMap, predictionCallParameters{
		buttonCall:   showButtons,
		pt:           pt,
		guildContext: i.GuildID != "",
	})

	if len(components) == 0 {
		// A text component
		components = []discordgo.MessageComponent{
			&discordgo.TextDisplay{
				Content: "No predictions available at this time.",
			},
		}
	}

	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      flags,
		Components: components,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{},
		},
	})
	if err != nil {
		log.Println("Error sending follow-up message /predictions:", err)
	}
}

// HandlePredictionsPage handles interaction for prediction pages
func HandlePredictionsPage(s *discordgo.Session, i *discordgo.InteractionCreate) {

	// check if the original message is older than 5 minutes
	const ttl = 5 * time.Minute
	expired := false
	if i.Message != nil {
		createdAt, err := discordgo.SnowflakeTimestamp(i.Message.ID)
		if err != nil {
			log.Println("Error parsing message timestamp:", err)
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "This prediction interaction is invalid or expired. Run /predictions again to get a fresh panel.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		expired = time.Since(createdAt) > ttl
	}

	reaction := strings.Split(i.MessageComponentData().CustomID, "#")

	flags := discordgo.MessageFlagsIsComponentsV2 | discordgo.MessageFlagsEphemeral

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      flags,
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		log.Println("Error responding to interaction /predictions:", err)
	}

	predParams := predictionCallParameters{
		buttonCall:   true,
		guildContext: i.GuildID != "",
	}

	// Determine the new prediction type based on the interaction
	if len(reaction) == 4 {
		switch {
		case reaction[1] == "predclose" || (reaction[1] == "predtype" && expired):
			predParams.pt = predictionType(reaction[2])
			predParams.buttonCall = false
		case reaction[1] == "predtype":
			predParams.pt = predictionType(i.MessageComponentData().Values[0])
		}
		if count, err := strconv.Atoi(reaction[3]); err == nil {
			predParams.contractCount = int64(count)
		}
	}

	components := predictions(nil, predParams)

	edit := discordgo.WebhookEdit{
		Components: &components,
	}

	_, err = s.FollowupMessageEdit(i.Interaction, i.Message.ID, &edit)
	if err != nil {
		log.Println(err)
	}
}

type predictionCallParameters struct {
	buttonCall    bool           // Whether to display buttons
	contractCount int64          // How many contracts to show per type
	pt            predictionType // Which prediction type to show
	guildContext  bool           // Whether the interaction is in a guild (server) context
}

// predictions prints predictions for the following weeks contracts.
func predictions(optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption, params predictionCallParameters) []discordgo.MessageComponent {
	var contractCount int64 = 3
	if optionMap != nil && optionMap["contract-count"] != nil {
		contractCount = optionMap["contract-count"].IntValue()
	} else if params.contractCount > 0 {
		contractCount = params.contractCount
	}

	buttonCall := params.buttonCall
	pw := newPredictionsWriter(params.guildContext)

	// Collectibles view is handled separately
	if params.pt == predictionCollectibles {
		pw.showTokenRate = false
		_, wedTime, friTime, _ := contractTimes9amPacific(0)
		collectibles := predictCollectibles(wedTime, friTime)
		components := make([]discordgo.MessageComponent, 0, 3)
		components = append(components, pw.writeCollectiblesPredictions(collectibles))
		if buttonCall {
			components = append(components, getPredictionsButtonsComponents(params.pt, contractCount)...)
		}
		return components
	}

	// Determine which predictions to show
	showWednesday, showFridayNonUltra, showFridayUltra := params.pt.flags()
	hasFriday := showFridayNonUltra || showFridayUltra

	// Get predictions
	fridayNonUltra, fridayUltra, wednesday := predictJeli(int(contractCount))

	// Get the next Wednesday and Friday times; 0 means current week
	_, wedTime, friTime, _ := contractTimes9amPacific(0)

	var first, second discordgo.MessageComponent
	if showWednesday {
		if hasFriday {
			// both Wednesday and Friday
			if wedTime.Before(friTime) {
				first = pw.writeWednesdayPredictions(wedTime, wednesday, false)
				second = pw.writeFridayPredictions(friTime, fridayNonUltra, fridayUltra, true, showFridayNonUltra, showFridayUltra)
			} else {
				first = pw.writeFridayPredictions(friTime, fridayNonUltra, fridayUltra, false, showFridayNonUltra, showFridayUltra)
				second = pw.writeWednesdayPredictions(wedTime, wednesday, true)
			}
		} else {
			// only Wednesday
			first = pw.writeWednesdayPredictions(wedTime, wednesday, true)
		}
	} else {
		// only Friday
		first = pw.writeFridayPredictions(friTime, fridayNonUltra, fridayUltra, true, showFridayNonUltra, showFridayUltra)
	}

	// Build components slice
	cap := 2
	if second != nil {
		cap++
	}
	if buttonCall {
		cap += 2
	}

	components := make([]discordgo.MessageComponent, 0, cap)
	components = append(components, first)
	if second != nil {
		components = append(components, bottools.NewSmallSeparatorComponent(true), second)
	}

	// Add select menu if not a button call
	if buttonCall {
		components = append(components, getPredictionsButtonsComponents(params.pt, contractCount)...)
	}

	return components
}

func getPredictionsButtonsComponents(predType predictionType, contractCount int64) []discordgo.MessageComponent {
	min := 1
	selectMenu := discordgo.SelectMenu{
		MenuType:    discordgo.StringSelectMenu,
		CustomID:    fmt.Sprintf("predictions#predtype#%s#%d", predType, contractCount),
		Placeholder: "Prediction Type",
		MinValues:   &min,
		MaxValues:   1,
		Options: []discordgo.SelectMenuOption{
			{
				Label:       "All Leggacies",
				Description: "Show all Leggacy contracts",
				Value:       string(predictionAll),
				Default:     predType == predictionAll,
			},
			{
				Label:       "Wednesday Leggacy",
				Description: "Show only Wednesday Leggacy contracts",
				Value:       string(predictionWedLegacy),
				Default:     predType == predictionWedLegacy,
			},
			{
				Label:       "Friday PE Leggacies",
				Description: "Show both Friday PE Leggacy contracts",
				Value:       string(predictionFriPeLegacyBoth),
				Default:     predType == predictionFriPeLegacyBoth,
			},
			{
				Label:       "Friday Leggacy",
				Description: "Show Friday non-Ultra Leggacy contracts",
				Value:       string(predictionFriNonUltra),
				Default:     predType == predictionFriNonUltra,
			},
			{
				Label:       "Friday Ultra Leggacy",
				Description: "Show Friday Ultra Leggacy contracts",
				Value:       string(predictionFriUltraLegacy),
				Default:     predType == predictionFriUltraLegacy,
			},
			{
				Label:       "Colleggtibles",
				Description: "Show Colleggtible contracts",
				Value:       string(predictionCollectibles),
				Default:     predType == predictionCollectibles,
			},
		},
	}
	closeButton := discordgo.Button{
		Label:    "Save",
		Emoji:    &discordgo.ComponentEmoji{Name: "💾"},
		Style:    discordgo.SuccessButton,
		CustomID: fmt.Sprintf("predictions#predclose#%s#%d", predType, contractCount),
	}

	bottomRow := []discordgo.MessageComponent{closeButton}

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{selectMenu},
		},
		discordgo.ActionsRow{
			Components: bottomRow,
		},
	}
}

type predictionsWriter struct {
	guildContext  bool
	showTokenRate bool
	iconCoop      string
	iconUltra     string
	iconPE        string
	botIcon       string
	cmd           string
}

func newPredictionsWriter(guildContext bool) predictionsWriter {
	cmd := bottools.GetFormattedCommand("predictions")
	if cmd == "" {
		cmd = "/predictions"
	}
	botIcon := ei.GetBotEmojiMarkdown("boostbot")
	if botIcon != "" {
		botIcon += " "
	}
	return predictionsWriter{
		guildContext:  guildContext,
		showTokenRate: true,
		iconCoop:      ei.GetBotEmojiMarkdown("icon_coop"),
		iconUltra:     ei.GetBotEmojiMarkdown("ultra"),
		iconPE:        ei.GetBotEmojiMarkdown("egg_prophecy"),
		botIcon:       botIcon,
		cmd:           cmd,
	}
}

func (pw predictionsWriter) writeWednesdayPredictions(dropTime time.Time, contracts []ei.EggIncContract, footer bool) *discordgo.TextDisplay {
	var b strings.Builder

	b.WriteString("**📜 Leggacy Prediction 🔮**\n-# ")
	b.WriteString(bottools.WrapTimestamp(dropTime.Unix(), bottools.TimestampShortDateTime))
	b.WriteByte('\n')

	usedSeasons := pw.writeContracts(&b, contracts)
	b.WriteByte('\n')

	if footer {
		pw.writeFooter(&b, usedSeasons)
	}

	return &discordgo.TextDisplay{Content: b.String()}
}

func (pw predictionsWriter) writeFridayPredictions(dropTime time.Time, peContracts, ultraContracts []ei.EggIncContract, footer, showNonUltra, showUltra bool) *discordgo.TextDisplay {
	var b strings.Builder

	b.WriteString("**PE Leggacies Predictions 🔮**\n-# ")
	b.WriteString(bottools.WrapTimestamp(dropTime.Unix(), bottools.TimestampShortDateTime))
	b.WriteByte('\n')

	usedSeasons := make(map[string]bool)

	if showNonUltra && len(peContracts) != 0 {
		b.WriteString("**")
		b.WriteString(pw.iconPE)
		b.WriteString(" PE Leggacy**\n")
		maps.Copy(usedSeasons, pw.writeContracts(&b, peContracts))
		b.WriteByte('\n')
	}

	if showUltra && len(ultraContracts) != 0 {
		b.WriteString("**")
		b.WriteString(pw.iconUltra)
		b.WriteString(" Ultra PE Leggacy **\n")
		maps.Copy(usedSeasons, pw.writeContracts(&b, ultraContracts))
		b.WriteByte('\n')
	}

	if footer {
		pw.writeFooter(&b, usedSeasons)
	}

	return &discordgo.TextDisplay{Content: b.String()}
}

func (pw predictionsWriter) writeFooter(b *strings.Builder, usedSeasons map[string]bool) {
	if pw.showTokenRate {
		fmt.Fprintf(b, "-# %s Coop Size | %s Tokens/min", pw.iconCoop, ei.GetBotEmojiMarkdown("token"))
	} else {
		fmt.Fprintf(b, "-# %s Coop Size", pw.iconCoop)
	}
	var seasonEmojis strings.Builder
	for _, s := range seasonsOrdered {
		if usedSeasons[s.Key] {
			seasonEmojis.WriteString(s.Emoji)
		}
	}
	if seasonEmojis.Len() > 0 {
		fmt.Fprintf(b, " | %s Seasonal LB", seasonEmojis.String())
	}
	if !pw.guildContext {
		fmt.Fprintf(b, "\n-# %sBoost Bot | %s | %s\n", pw.botIcon, pw.cmd, bottools.WrapTimestamp(time.Now().Unix(), bottools.TimestampShortDateTime))
	}
}

const timeSaverContractID = "time-saver-2021"

// writeContracts prints only the contract lines and returns the set of season keys that appeared.
func (pw predictionsWriter) writeContracts(b *strings.Builder, contracts []ei.EggIncContract) map[string]bool {
	usedSeasons := make(map[string]bool)

	// (The time-saver-2021 hack is now applied centrally in GetPredictionBrackets)

	for _, c := range contracts {

		// Season label "🍂 FL25", "☀️ SU23", "🌷 SP23", "❄️ WI24"
		seasonLabel := ""
		if c.SeasonID != "" {
			if idx := strings.IndexByte(c.SeasonID, '_'); idx > 0 && idx < len(c.SeasonID)-1 {
				seasonKey := c.SeasonID[:idx]
				seasonYear := c.SeasonID[idx+1:]

				if info, ok := seasonsByKey[seasonKey]; ok {
					yearShort := seasonYear
					if len(yearShort) >= 2 {
						yearShort = yearShort[len(yearShort)-2:]
					}
					seasonLabel = fmt.Sprintf("%s %s%s", info.Emoji, info.Code, yearShort)
					usedSeasons[info.Key] = true
				}
			}
		}

		// First line
		fmt.Fprintf(
			b,
			"%s **[%s](https://eicoop-carpet.netlify.app/?q=%s)** %s `%d`",
			ei.FindEggEmoji(c.EggName),
			c.Name,
			c.ID,
			pw.iconCoop,
			c.MaxCoopSize,
		)
		if seasonLabel != "" {
			b.WriteString("  ")
			b.WriteString(seasonLabel)
		}
		b.WriteByte('\n')

		// Second line
		fmt.Fprintf(
			b,
			"-# _       _ Dur: **%s** CS: **%.0f** %s/%dm\n",
			bottools.FmtDuration(c.EstimatedDuration.Round(time.Minute)),
			c.Cxp,
			ei.GetBotEmojiMarkdown("token"),
			c.MinutesPerToken,
		)

		// Third line for AWOL contracts
		if c.ID == timeSaverContractID {
			if c.ValidFrom.Before(time.Unix(1774454400, 0)) {
				fmt.Fprintf(b, "-# _       _ Missing since: **%s** (%s)🕯️\n", bottools.WrapTimestamp(1774454400, bottools.TimestampShortDate), bottools.WrapTimestamp(1774454400, bottools.TimestampRelativeTime))
			}
		}

		// TODO: implement a debug mode for these
		/*
			// Third line
			fmt.Fprintf(
				b,
				"-# _       _ Published: **%s** Expired: **%s**\n",
				bottools.WrapTimestamp(c.ValidFrom.Unix(), bottools.TimestampShortDate),
				bottools.WrapTimestamp(c.ValidUntil.Unix(), bottools.TimestampShortDate),
			)
		*/
	}
	return usedSeasons
}

/*
	// sortValidUntil returns true if a should be ordered before b.
	// Priority:
	// 1. ValidUntil (older first)
	// 2. ValidFrom  (older first)
	func sortValidUntil(a, b ei.EggIncContract) bool {
		if a.ValidUntil.Equal(b.ValidUntil) {
			return a.ValidFrom.Before(b.ValidFrom)
		}
		return a.ValidUntil.Before(b.ValidUntil)
	}
*/

// sortValidFrom returns true if a should be ordered before b.
// Priority:
// 1. ValidFrom  (older first)
// 2. ValidUntil (older first)
func sortValidFrom(a, b ei.EggIncContract) bool {
	if a.ValidFrom.Equal(b.ValidFrom) {
		return a.ValidUntil.Before(b.ValidUntil)
	}
	return a.ValidFrom.Before(b.ValidFrom)
}

// GetPredictionBrackets returns the sorted brackets for all Leggacy contracts.
func GetPredictionBrackets() (wed, friPE, friUltra []ei.EggIncContract) {
	for _, c := range ei.EggIncContractsAll {
		switch {
		case c.HasPE && !c.Ultra:
			friUltra = append(friUltra, c)
		case c.HasPE && c.Ultra:
			friPE = append(friPE, c)
		default:
			wed = append(wed, c)
		}
	}

	sortBracket := func(bracket []ei.EggIncContract) {
		sort.Slice(bracket, func(i, j int) bool {
			return sortValidFrom(bracket[i], bracket[j])
		})
		for i, c := range bracket {
			if c.ID == timeSaverContractID && i+1 < len(bracket) {
				bracket[i], bracket[i+1] = bracket[i+1], c
				break
			}
		}
	}

	sortBracket(wed)
	sortBracket(friPE)
	sortBracket(friUltra)

	return wed, friPE, friUltra
}

// predictJeli returns up to N oldest contracts per legacy contract type.
// Contributed by jelibean84
func predictJeli(contractCount int) (fridayNonUltra, fridayUltra, wednesday []ei.EggIncContract) {
	wednesday, fridayNonUltra, fridayUltra = GetPredictionBrackets()

	if len(fridayNonUltra) > contractCount {
		fridayNonUltra = fridayNonUltra[:contractCount]
	}
	if len(fridayUltra) > contractCount {
		fridayUltra = fridayUltra[:contractCount]
	}
	if len(wednesday) > contractCount {
		wednesday = wednesday[:contractCount]
	}

	return
}

// collectiblePrediction holds a contract and its predicted drop time for a custom egg (Egg_CUSTOM_EGG).
type collectiblePrediction struct {
	ei.EggIncContract
	predictedTime time.Time
}

// predictCollectibles sorts ALL contracts into the 3 leggacy brackets (same
// classification as predictJeli), computes each custom egg's predicted drop date
// from its bracket position, and keeps the soonest occurrence per egg ID.
func predictCollectibles(nextWed, nextFri time.Time) map[string]collectiblePrediction {
	wed, friPE, friUltra := GetPredictionBrackets()
	/*
		debugBracket := func(name string, bracket []ei.EggIncContract, base time.Time) {
			log.Printf("[collectibles debug] === %s (%d contracts) ===", name, len(bracket))
			for week, c := range bracket {
				if c.Egg == int32(ei.Egg_CUSTOM_EGG) {
					log.Printf("[collectibles debug]  week %d  *** CUSTOM EGG egg=%q id=%q predicted=%s",
						week, c.EggName, c.ID, base.AddDate(0, 0, 7*week).Format("2006-01-02"))
				} else {
					log.Printf("[collectibles debug]  week %d  egg=%q id=%q validFrom=%s",
						week, c.EggName, c.ID, c.ValidFrom.Format("2006-01-02"))
				}
			}
		}
		debugBracket("Wednesday", wed, nextWed)
		debugBracket("Friday PE", friPE, nextFri)
		debugBracket("Friday Ultra PE", friUltra, nextFri)
	*/
	result := make(map[string]collectiblePrediction)
	scan := func(bracket []ei.EggIncContract, base time.Time) {
		for week, c := range bracket {
			eggName := c.EggName
			if c.Egg == int32(ei.Egg_CUSTOM_EGG) {
				if eggName == "" {
					continue
				}
			} else {
				// Old contracts used a regular egg enum for what is now a custom egg.
				// Match by lowercasing the enum name against the CustomEggMap keys.
				lowered := strings.ToLower(eggName)
				if _, exists := ei.CustomEggMap[lowered]; !exists {
					continue
				}
				eggName = lowered
			}
			predicted := base.AddDate(0, 0, 7*week)
			if existing, seen := result[eggName]; !seen || predicted.Before(existing.predictedTime) {
				contract := c
				contract.EggName = eggName
				result[eggName] = collectiblePrediction{contract, predicted}
			}
		}
	}
	scan(wed, nextWed)
	scan(friPE, nextFri)
	scan(friUltra, nextFri)
	return result
}

// writeCollectiblesPredictions renders the Colleggtibles prediction,
// showing one entry per custom egg sorted by predicted drop date.
func (pw predictionsWriter) writeCollectiblesPredictions(collectibles map[string]collectiblePrediction) *discordgo.TextDisplay {
	collectibleContracts := make([]collectiblePrediction, 0, len(collectibles))
	for _, p := range collectibles {
		collectibleContracts = append(collectibleContracts, p)
	}
	sort.Slice(collectibleContracts, func(i, j int) bool {
		if collectibleContracts[i].predictedTime.Equal(collectibleContracts[j].predictedTime) {
			return collectibleContracts[i].ID < collectibleContracts[j].ID
		}
		return collectibleContracts[i].predictedTime.Before(collectibleContracts[j].predictedTime)
	})

	var b strings.Builder

	b.WriteString("**Colleggtibles Prediction 🔮**\n")

	usedSeasons := make(map[string]bool)

	for _, cc := range collectibleContracts {

		// Season label "🍂 FL25", "☀️ SU23", "🌷 SP23", "❄️ WI24"
		seasonLabel := ""
		if cc.SeasonID != "" {
			if idx := strings.IndexByte(cc.SeasonID, '_'); idx > 0 && idx < len(cc.SeasonID)-1 {
				seasonKey := cc.SeasonID[:idx]
				seasonYear := cc.SeasonID[idx+1:]
				if info, ok := seasonsByKey[seasonKey]; ok {
					yearShort := seasonYear
					if len(yearShort) >= 2 {
						yearShort = yearShort[len(yearShort)-2:]
					}
					seasonLabel = fmt.Sprintf("%s %s%s", info.Emoji, info.Code, yearShort)
					usedSeasons[info.Key] = true
				}
			}
		}

		// First line
		fmt.Fprintf(&b, "%s **[%s](https://eicoop-carpet.netlify.app/?q=%s)** %s `%d`",
			ei.FindEggEmoji(cc.EggName),
			cc.Name,
			cc.ID,
			pw.iconCoop,
			cc.MaxCoopSize,
		)
		if seasonLabel != "" {
			b.WriteString("  ")
			b.WriteString(seasonLabel)
		}
		b.WriteByte('\n')

		// Second line
		if egg, ok := ei.CustomEggMap[cc.EggName]; ok && len(egg.DimensionValueString) > 0 {
			fmt.Fprintf(&b, "-# _       _ %s %s %s\n",
				colleggtibleDimensionEmoji(egg.Dimension),
				egg.DimensionValueString[len(egg.DimensionValueString)-1],
				colleggtibleDimensionName(egg.Dimension))
		}
		// Third line
		fmt.Fprintf(&b, "-# _       _ Seen: **%s** Predicted: **%s**\n",
			bottools.WrapTimestamp(cc.ValidFrom.Unix(), bottools.TimestampShortDate),
			bottools.WrapTimestamp(cc.predictedTime.Unix(), bottools.TimestampShortDate),
		)
		/*
			fmt.Fprintf(&b, "-# _       _ Dur: **%s** CS: **%.0f**",
				bottools.FmtDuration(cc.EstimatedDuration.Round(time.Minute)),
				cc.Cxp,
			)
		*/
	}
	b.WriteByte('\n')
	pw.writeFooter(&b, usedSeasons)

	return &discordgo.TextDisplay{Content: b.String()}
}

// ***** Helpers *****

// ==== Season Info ====

type seasonInfo struct {
	Key   string // "winter"
	Name  string // "Winter"
	Emoji string // ❄️
	Code  string // WI/SP/SU/FL
}

// Order: 0=winter, 1=spring, 2=summer, 3=fall, 4=autumn
var seasonsOrdered = []seasonInfo{
	{Key: "winter", Name: "Winter", Emoji: "❄️", Code: "WI"},
	{Key: "spring", Name: "Spring", Emoji: "🌷", Code: "SP"},
	{Key: "summer", Name: "Summer", Emoji: "☀️", Code: "SU"},
	{Key: "fall", Name: "Fall", Emoji: "🍂", Code: "FL"},
}

// seasonsByKey maps season key to seasonInfo
var seasonsByKey = func() map[string]seasonInfo {
	m := make(map[string]seasonInfo, len(seasonsOrdered))
	for _, s := range seasonsOrdered {
		m[s.Key] = s
	}
	return m
}()

// ==== Time Keepers ====

// KevinLoc is the time.Location for America/Los_Angeles, falls back to UTC on error
var KevinLoc = func() *time.Location {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return time.UTC
	}
	return loc
}()

// isNextWedSoonerThanNextFri returns true if the next Wednesday 09:00
// occurs before the next Friday 09:00 based on the given time and location
func isNextWedSoonerThanNextFri(now time.Time, loc *time.Location) bool {
	return findNextContractDropTime(now, time.Wednesday, loc).
		Before(findNextContractDropTime(now, time.Friday, loc))
}

// findNextContractDropTime returns the next specified date based on given time at 9:00 AM in the given location
func findNextContractDropTime(now time.Time, target time.Weekday, loc *time.Location) time.Time {
	local := now.In(loc)

	daysAhead := (int(target) - int(local.Weekday()) + 7) % 7
	candidateDate := local.AddDate(0, 0, daysAhead)

	dropTime := time.Date(
		candidateDate.Year(),
		candidateDate.Month(),
		candidateDate.Day(),
		9, 0, 0, 0,
		loc,
	)

	// If the candidate dropTime alreday passed, move to next week
	if dropTime.Before(local) {
		dropTime = dropTime.AddDate(0, 0, 7)
	}

	return dropTime
}

// contractTimes9amPacific returns the contract drop times for Monday, Wednesday, and Friday at 9:00 AM Pacific Time
// It has two modes:
//
//   - week > 0:
//     Uses ei.EggIncCurrentSeason.StartTime (week 1 Monday +0) to compute
//     Monday, Wednesday, Friday at 9:00 AM America/Los_Angeles for that week.
//     Returns (monday, wednesday, friday, ok=true) if season is configured.
//     If season is not configured (_, _, _, ok=false).
//
//   - week <= 0:
//     Uses time.Now() to compute next Wednesday and Friday at 9:00 AM
//     America/Los_Angeles. Monday is returned as zero-value.
//     Returns (_, wednesday, friday, ok=true).
func contractTimes9amPacific(week int) (monday, wednesday, friday time.Time, ok bool) {

	var baseLocal time.Time

	// Current-date-based
	if week <= 0 {
		wednesday = findNextContractDropTime(time.Now(), time.Wednesday, KevinLoc)
		friday = findNextContractDropTime(time.Now(), time.Friday, KevinLoc)
		return time.Time{}, wednesday, friday, true
	}

	// Week-based
	season := ei.EggIncCurrentSeason
	if season.StartTime == 0 || season.ID == ei.SeasonUnknownID {
		return time.Time{}, time.Time{}, time.Time{}, false
	}

	baseLocal = time.Unix(int64(season.StartTime), 0).In(KevinLoc)
	baseLocal = time.Date(
		baseLocal.Year(),
		baseLocal.Month(),
		baseLocal.Day(),
		9, 0, 0, 0,
		KevinLoc,
	)

	// Calculate Monday/Wed/Fri of the target week
	daysOffset := 7 * (week - 1)
	monday = baseLocal.AddDate(0, 0, daysOffset)
	wednesday = monday.AddDate(0, 0, 2)
	friday = monday.AddDate(0, 0, 4)

	return monday, wednesday, friday, true
}
