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

// GetPredictionsCommand returns the command for the /signups command
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

// HandlePredictionsCommand will handle the /signups command
func HandlePredictionsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	flags := discordgo.MessageFlagsIsComponentsV2
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

	// Find which subcommand was used and call the appropriate handler
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
	components = predictions(optionMap, predictionCallParameters{
		buttonCall: true,
		pt:         pt,
	})

	content := ""
	if len(components) == 0 {
		content = "No templates available."
		components = nil
	}

	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content:    content,
		Flags:      flags,
		Components: components,
	})
	if err != nil {
		log.Println("Error sending follow-up message:", err)
	}
}

// HandleSignupsPage handles interaction for signup pages
func HandleSignupsPage(s *discordgo.Session, i *discordgo.InteractionCreate) {

	// check if the original message is older than 5 minutes
	const ttl = 5 * time.Minute
	expired := false
	if i.Message != nil {
		createdAt, err := discordgo.SnowflakeTimestamp(i.Message.ID)
		if err != nil {
			log.Println("Error parsing message timestamp:", err)
			return
		}
		expired = time.Since(createdAt) > ttl
	}

	flags := discordgo.MessageFlagsIsComponentsV2 | discordgo.MessageFlagsEphemeral
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      flags,
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		log.Println(err)
	}

	predParams := predictionCallParameters{
		buttonCall: true,
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

// ==== Prediction Subcommand ====

type predictionCallParameters struct {
	buttonCall    bool           // Whether to display buttons
	contractCount int64          // How many contracts to show per type
	pt            predictionType // Which prediction type to show
}

// predictions prints predictions for the following weeks contracts.
func predictions(
	optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption,
	params predictionCallParameters,
) []discordgo.MessageComponent {
	var contractCount int64 = 3
	if optionMap != nil && optionMap["contract-count"] != nil {
		contractCount = optionMap["contract-count"].IntValue()
	} else if params.contractCount > 0 {
		contractCount = params.contractCount
	}

	buttonCall := params.buttonCall
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
				first = writeWednesdayPredictions(wedTime, wednesday, false)
				second = writeFridayPredictions(friTime, fridayNonUltra, fridayUltra, true, showFridayNonUltra, showFridayUltra)
			} else {
				first = writeFridayPredictions(friTime, fridayNonUltra, fridayUltra, false, showFridayNonUltra, showFridayUltra)
				second = writeWednesdayPredictions(wedTime, wednesday, true)
			}
		} else {
			// only Wednesday
			first = writeWednesdayPredictions(wedTime, wednesday, true)
		}
	} else {
		// only Friday
		first = writeFridayPredictions(friTime, fridayNonUltra, fridayUltra, true, showFridayNonUltra, showFridayUltra)
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
		CustomID:    fmt.Sprintf("fd_signups#predtype#%s#%d", predType, contractCount),
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
		},
	}
	// TODO: Update the command name to predictions
	closeButton := discordgo.Button{
		Label:    "💾 Save",
		Style:    discordgo.SuccessButton,
		CustomID: fmt.Sprintf("fd_signups#predclose#%s#%d", predType, contractCount),
	}

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{selectMenu},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{closeButton},
		},
	}
}

// Wednesday Predictions
func writeWednesdayPredictions(
	dropTime time.Time, // when the post is created
	contracts []ei.EggIncContract,
	footer bool,
) *discordgo.TextDisplay {
	var b strings.Builder

	// Icons
	iconCoop := ei.GetBotEmojiMarkdown("icon_coop")

	// Header
	b.WriteString("**📜 Leggacy Prediction 🔮**\n-# ")
	b.WriteString(bottools.WrapTimestamp(dropTime.Unix(), bottools.TimestampLongDate))
	b.WriteByte('\n')

	// Body
	writeContracts(&b, contracts, iconCoop)
	b.WriteByte('\n')

	// Footer
	if footer {
		b.WriteString("-# ")
		b.WriteString(iconCoop)
		b.WriteString(" Coop Size | 🌼Seasonal LB\n")
	}

	return &discordgo.TextDisplay{
		Content: b.String(),
	}
}

func writeFridayPredictions(
	dropTime time.Time,
	peContracts []ei.EggIncContract,
	ultraContracts []ei.EggIncContract,
	footer, showNonUltra, showUltra bool,
) *discordgo.TextDisplay {
	var b strings.Builder

	// Icons BB vs Egg Server
	iconCoop := ei.GetBotEmojiMarkdown("icon_coop")
	iconUltra := ei.GetBotEmojiMarkdown("ultra")
	iconPE := ei.GetBotEmojiMarkdown("egg_prophecy")

	// Header
	b.WriteString("**PE Leggacies Predictions 🔮**\n-# ")
	b.WriteString(bottools.WrapTimestamp(dropTime.Unix(), bottools.TimestampLongDate))
	b.WriteByte('\n')

	// Non-Ultra
	if showNonUltra && len(peContracts) != 0 {
		b.WriteString("**")
		b.WriteString(iconPE)
		b.WriteString(" PE Leggacy**\n")
		writeContracts(&b, peContracts, iconCoop)
		b.WriteByte('\n')
	}

	// Ultra
	if showUltra && len(ultraContracts) != 0 {
		b.WriteString("**")
		b.WriteString(iconUltra)
		b.WriteString(" Ultra PE Leggacy **\n")
		writeContracts(&b, ultraContracts, iconCoop)
		b.WriteByte('\n')
	}

	// Footer
	if footer {
		b.WriteString("-# ")
		b.WriteString(iconCoop)
		b.WriteString(" Coop Size | 🌼Seasonal LB\n")
	}

	return &discordgo.TextDisplay{
		Content: b.String(),
	}
}

// writeContracts prints only the contract lines, reusing shared season metadata.
func writeContracts(
	b *strings.Builder,
	contracts []ei.EggIncContract,
	iconCoop string,
) {
	for _, c := range contracts {

		// Season label "🍂 25FL", "☀️ 23SU", "🌼 23SP", "❄️ 24WI"
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
					seasonLabel = fmt.Sprintf("%s %s%s", info.Emoji, yearShort, info.Code)
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
			iconCoop,
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
			"-# _       _ Dur: **%s** CS: **%.0f**\n",
			bottools.FmtDuration(c.EstimatedDuration.Round(time.Minute)),
			c.Cxp,
		)
	}
}

// older returns true if a should be ordered before b.
// Priority:
// 1. ValidUntil (older first)
// 2. ValidFrom  (older first)
func older(a, b ei.EggIncContract) bool {
	if a.ValidUntil.Equal(b.ValidUntil) {
		return a.ValidFrom.Before(b.ValidFrom)
	}
	return a.ValidUntil.Before(b.ValidUntil)
}

// predictJeli returns up to N oldest contracts per legacy contract type.
// Contributed by jelibean84
func predictJeli(contractCount int) (fridayNonUltra, fridayUltra, wednesday []ei.EggIncContract) {

	for _, c := range ei.EggIncContractsAll {
		switch {
		case c.HasPE && !c.Ultra:
			fridayUltra = append(fridayUltra, c)
		case c.HasPE && c.Ultra:
			fridayNonUltra = append(fridayNonUltra, c)
		default:
			wednesday = append(wednesday, c)
		}
	}

	sort.Slice(fridayUltra, func(i, j int) bool {
		return older(fridayUltra[i], fridayUltra[j])
	})
	sort.Slice(fridayNonUltra, func(i, j int) bool {
		return older(fridayNonUltra[i], fridayNonUltra[j])
	})
	sort.Slice(wednesday, func(i, j int) bool {
		return older(wednesday[i], wednesday[j])
	})

	if len(fridayUltra) > contractCount {
		fridayUltra = fridayUltra[:contractCount]
	}
	if len(fridayNonUltra) > contractCount {
		fridayNonUltra = fridayNonUltra[:contractCount]
	}
	if len(wednesday) > contractCount {
		wednesday = wednesday[:contractCount]
	}

	return
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
	{Key: "spring", Name: "Spring", Emoji: "🌼", Code: "SP"},
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
func isNextWedSoonerThanNextFri(
	now time.Time,
	loc *time.Location,
) bool {
	local := now.In(loc)

	wd := local.Weekday()
	after9 := local.Hour() > 9 ||
		(local.Hour() == 9 && (local.Minute() > 0 || local.Second() > 0 || local.Nanosecond() > 0))

	switch wd {
	case time.Monday, time.Tuesday, time.Saturday, time.Sunday:
		return true
	case time.Wednesday:
		return !after9
	case time.Thursday:
		return false
	case time.Friday:
		return after9
	default:
		return false
	}
}

// findNextContractDropTime returns the next specified date based on given time at 9:00 AM in the given location
func findNextContractDropTime(
	now time.Time,
	target time.Weekday,
	loc *time.Location,
) time.Time {
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
