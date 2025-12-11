package boost

import (
	"fmt"
	"log"
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

// GetSignupsCommand returns the command for the /signups command
func GetSignupsCommand(cmd string) *discordgo.ApplicationCommand {
	minValue := 1.0
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Get sign-up templates and contract predictions.",
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
				Name:        "predictions",
				Description: "Print predictions for the following weeks contracts.",
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
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "xfs",
				Description: "Print all signup templates for XFSweaty.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "copy-paste",
						Description: "Format for easy copy-paste into Discord (default false).",
						Required:    false,
					},
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "week",
						Description: "The week to get the signup templates for.",
						Choices: func() []*discordgo.ApplicationCommandOptionChoice {
							choices := make([]*discordgo.ApplicationCommandOptionChoice, 13)
							for i := 1; i <= 13; i++ {
								choices[i-1] = &discordgo.ApplicationCommandOptionChoice{
									Name:  "Week " + strconv.Itoa(i),
									Value: strconv.Itoa(i),
								}
							}
							return choices
						}(),
						Required: false,
					},
				},
			},
		},
	}
}

// HandleSignupsCommand will handle the /signups command
func HandleSignupsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
	if _, ok := optionMap["predictions"]; ok {

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
			copyPaste:  false,
			buttonCall: true,
			pt:         pt,
		})
	} else if _, ok := optionMap["xfs"]; ok {
		components = signups(optionMap)
	}

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
		copyPaste:  false,
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

// ==== Signup Subcommand ====

// signups creates signup message components for the /signups xfs" subcommand
func signups(
	optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption,
) []discordgo.MessageComponent {

	currentWeek := ei.GetCurrentWeekNumber(KevinLoc)
	currentSeasonName, currentSeasonYear, _ := ei.GetEggIncCurrentSeason()

	// Set target to next week
	targetWeekRaw := currentWeek + 1
	if opt, ok := optionMap["xfs-week"]; ok {
		targetWeekRaw = int(opt.IntValue())
	}
	// target is next week
	isNextWeek := targetWeekRaw == currentWeek+1

	copyPaste := false
	if opt, ok := optionMap["xfs-copy-paste"]; ok {
		copyPaste = opt.BoolValue()
	}

	// map currentSeasonName ("winter", "spring", "summer", "fall") -> index
	seasonIndex := 0
	if currentSeasonName != "" {
		if info, ok := seasonsByKey[currentSeasonName]; ok {
			for idx, sInfo := range seasonsOrdered {
				if sInfo.Key == info.Key {
					seasonIndex = idx
					break
				}
			}
		}
	}

	// if targetWeekRaw is 14, wrap to week 1 of next season
	displayWeek := targetWeekRaw
	if displayWeek == 14 {
		displayWeek = 1
		// advance season index
		seasonIndex = (seasonIndex + 1) % len(seasonsOrdered)
	}

	// Mon/Wed/Fri 9am PT (Kevin time)
	mondayTime, wedTime, friTime, ok := contractTimes9amPacific(targetWeekRaw)
	if !ok {
		// Fallback to default next week times
		mondayTime, wedTime, friTime, _ = contractTimes9amPacific(0)
	}

	signupComponents := reactionSignupComponents(
		displayWeek,
		seasonIndex,
		currentSeasonYear,
		mondayTime,
		wedTime,
		friTime,
		copyPaste,
	)

	capacity := len(signupComponents)
	if isNextWeek {
		// add space for Leggacy predictions
		capacity += 3
	}
	components := make([]discordgo.MessageComponent, 0, capacity)
	components = append(components, signupComponents...)
	if isNextWeek {
		if copyPaste {
			optionMap["predictions-copy-paste"] = &discordgo.ApplicationCommandInteractionDataOption{
				Name:  "predictions-copy-paste",
				Type:  discordgo.ApplicationCommandOptionBoolean,
				Value: copyPaste,
			}
		}
		components = append(components, bottools.NewSmallSeparatorComponent(!copyPaste))
		components = append(components, predictions(optionMap, predictionCallParameters{
			copyPaste:     copyPaste,
			buttonCall:    false,
			contractCount: 3,
			pt:            predictionAll,
		})...)
	}

	return components
}

// Reaction signup posts as Discord components:
//   - Seasonal signup (week N)
//   - Wednesday Leggacy signup (week N)
//   - Friday PE Leggacies signup (week N)
func reactionSignupComponents(
	week int,
	seasonIndex, seasonYear int,
	seasonalTime, wedLeggacyTime, friPETime time.Time,
	copyPaste bool,
) []discordgo.MessageComponent {
	capHint := 3
	if !copyPaste {
		capHint = 5
	}
	components := make([]discordgo.MessageComponent, 0, capHint)

	seasonal := writeSeasonalSignupDisplay(week, seasonIndex, seasonYear, seasonalTime, copyPaste)
	legacy := writeLegacySignupDisplay(wedLeggacyTime, copyPaste)
	peLegacy := writePELegacySignupDisplay(friPETime, copyPaste)

	components = append(components, seasonal)
	if !copyPaste {
		components = append(
			components,
			bottools.NewSmallSeparatorComponent(true),
			legacy,
			bottools.NewSmallSeparatorComponent(true),
			peLegacy,
		)
	} else {
		components = append(components, legacy, peLegacy)
	}

	return components
}

func writeSeasonalSignupDisplay(
	week int,
	seasonIndex, seasonYear int,
	dropTime time.Time,
	copyPaste bool,
) *discordgo.TextDisplay {
	deadlineTime := dropTime // .Add(5 * time.Minute)
	season := seasonsOrdered[seasonIndex]

	content := fmt.Sprintf(
		`## %s %s %d Week %d/13 Sign-up: %s %s ##
**%s %d Seasonal Sign-up:** Contract Name TBD
**Contract Drop (+0):** Time listed in title
**Sign-up Deadline:** %s

**Which __Co-Op Role__ applies to your account? (required)**
:icon_token: — Want to **bank**
:chickenrun: — Want to **just play**
🐣 — Is an **alt/mini** that needs this contract
:care: — Is just **filling a spot** if needed

**What __Start Time__ works for you? (required)** 
0️⃣ — **+0**
1️⃣ — **+3**
🔀 — **+0 or +3**`,
		season.Emoji,
		season.Name,
		seasonYear,
		week,
		bottools.WrapTimestamp(dropTime.Unix(), bottools.TimestampLongDateTime),
		season.Emoji,

		season.Name,
		seasonYear,

		bottools.WrapTimestamp(deadlineTime.Unix(), bottools.TimestampShortTime),
	)

	if copyPaste {
		content = "```\n" + content + "\n```"
	}

	return &discordgo.TextDisplay{Content: content}
}

func writeLegacySignupDisplay(
	dropTime time.Time,
	copyPaste bool,
) *discordgo.TextDisplay {
	deadlineTime := dropTime // .Add(5 * time.Minute)

	content := fmt.Sprintf(
		`## 📜 Leggacy Sign-up: %s 📜 ##
**Contract Drop (+0):** Time listed in title
**Sign-up Deadline:** %s

**Which __Co-Op Role__ applies to your account? (required)**
:icon_token: — Want to **bank/sink**
:chickenrun: — Want to **just play**
🐣 — Is an **alt/mini** that needs this contract
:care: — Is just **filling a spot** if needed

**What __Start Time__ works for you? (required)** 
0️⃣ — **+0**
1️⃣ — **+3**
🔀 — **+0 or +3**`,
		bottools.WrapTimestamp(dropTime.Unix(), bottools.TimestampLongDateTime),
		bottools.WrapTimestamp(deadlineTime.Unix(), bottools.TimestampShortTime),
	)

	if copyPaste {
		content = "```\n" + content + "\n```"
	}

	return &discordgo.TextDisplay{Content: content}
}

func writePELegacySignupDisplay(
	dropTime time.Time,
	copyPaste bool,
) *discordgo.TextDisplay {
	deadlineTime := dropTime //.Add(5 * time.Minute)

	content := fmt.Sprintf(
		`## 🟢🟪  PE Leggacies Sign-up: %s 🟢🟪 ##
-# PE Leggacy (Ultra or non-Ultra) that is harder to fill will be prioritized.
**Contract Drop (+0):** Time listed in title
**Sign-up Deadline:** %s

**Does this account have an active __Ultra Subscription__? (required)**
🟪 — yes
🟢 — no

**Any additional __Co-Op Roles__ for this account? (optional) **
:icon_token: — Want to **bank/sink**
🐣 — Is an **alt/mini** that needs this contract
:care: — Is just **filling a spot** if needed`,
		bottools.WrapTimestamp(dropTime.Unix(), bottools.TimestampLongDateTime),
		bottools.WrapTimestamp(deadlineTime.Unix(), bottools.TimestampShortTime),
	)

	if copyPaste {
		content = "```\n" + content + "\n```"
	}

	return &discordgo.TextDisplay{Content: content}
}

// ==== Prediction Subcommand ====

type predictionCallParameters struct {
	copyPaste     bool           // Whether to wrap content in code blocks
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
	if optionMap != nil && optionMap["predictions-contract-count"] != nil {
		contractCount = optionMap["predictions-contract-count"].IntValue()
	} else if params.contractCount > 0 {
		contractCount = params.contractCount
	}

	copyPaste := params.copyPaste
	buttonCall := params.buttonCall
	// Determine which predictions to show
	showWednesday, showFridayNonUltra, showFridayUltra := params.pt.flags()
	hasFriday := showFridayNonUltra || showFridayUltra

	// Get predictions
	fridayNonUltra, fridayUltra, wednesday := predictJeli(contractCount)

	// Get the next Wednesday and Friday times; 0 means current week
	_, wedTime, friTime, _ := contractTimes9amPacific(0)

	var first, second discordgo.MessageComponent
	if showWednesday {
		if hasFriday {
			// both Wednesday and Friday
			if wedTime.Before(friTime) {
				first = writeWednesdayPredictions(wedTime, wednesday, copyPaste, false)
				second = writeFridayPredictions(friTime, fridayNonUltra, fridayUltra, copyPaste, false, showFridayNonUltra, showFridayUltra)
			} else {
				first = writeFridayPredictions(friTime, fridayNonUltra, fridayUltra, copyPaste, false, showFridayNonUltra, showFridayUltra)
				second = writeWednesdayPredictions(wedTime, wednesday, copyPaste, true)
			}
		} else {
			// only Wednesday
			first = writeWednesdayPredictions(wedTime, wednesday, copyPaste, true)
		}
	} else {
		// only Friday
		first = writeFridayPredictions(friTime, fridayNonUltra, fridayUltra, copyPaste, true, showFridayNonUltra, showFridayUltra)
	}

	// Build components slice
	cap := 1
	if !copyPaste {
		cap++
	}
	if second != nil {
		cap++
	}
	if buttonCall {
		cap += 2
	}

	components := make([]discordgo.MessageComponent, 0, cap)
	components = append(components, first)
	if second != nil {
		if !copyPaste {
			components = append(components, bottools.NewSmallSeparatorComponent(true))
		}
		components = append(components, second)
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
	copyPaste, footer bool,
) *discordgo.TextDisplay {
	var b strings.Builder

	// Icons
	iconCoop := ei.GetBotEmojiMarkdown("icon_coop")
	iconCR := ei.GetBotEmojiMarkdown("icon_chicken_run")
	if copyPaste {
		iconCoop = "👪"
		iconCR = ":chickenrun:"
		b.WriteString("```\n")
	}

	// Header
	b.WriteString("**📜 Leggacy Prediction 🔮**\n-# ")
	b.WriteString(bottools.WrapTimestamp(dropTime.Unix(), bottools.TimestampLongDate))
	b.WriteByte('\n')

	// Body
	writeContracts(&b, contracts, copyPaste, iconCoop, iconCR)

	// Footer: show for copy-paste mode or when not first
	if copyPaste || footer {
		b.WriteString("\n-# ")
		b.WriteString(iconCoop)
		b.WriteString(" coop size | 🎏 parade alts | ")
		b.WriteString(iconCR)
		b.WriteString(" target CRs | 🌼Seasonal LB\n")
		b.WriteString("-# Prediction formula by jelibean84\n")
	}

	if copyPaste {
		b.WriteString("```")
	}

	return &discordgo.TextDisplay{
		Content: b.String(),
	}
}

// Friday Predictions
func writeFridayPredictions(
	dropTime time.Time,
	peContracts []ei.EggIncContract, // non-Ultra PE
	ultraContracts []ei.EggIncContract, // Ultra PE
	copyPaste, footer, showNonUltra, showUltra bool,
) *discordgo.TextDisplay {
	var b strings.Builder

	// Icons
	iconCoop := ei.GetBotEmojiMarkdown("icon_coop")
	iconCR := ei.GetBotEmojiMarkdown("icon_chicken_run")
	if copyPaste {
		iconCoop = "👪"
		iconCR = ":chickenrun:"
		b.WriteString("```\n")
	}

	// Header
	b.WriteString("**PE Leggacies Predictions 🔮**\n-# ")
	b.WriteString(bottools.WrapTimestamp(dropTime.Unix(), bottools.TimestampLongDate))
	b.WriteByte('\n')

	// Non-ultra section
	if showNonUltra && len(peContracts) > 0 {
		b.WriteString("**🟢 PE Leggacy**\n")
		writeContracts(&b, peContracts, copyPaste, iconCoop, iconCR)
		b.WriteByte('\n')
	}

	// Ultra section
	if showUltra && len(ultraContracts) > 0 {
		b.WriteString("**🟪 Ultra PE Leggacy **\n")
		writeContracts(&b, ultraContracts, copyPaste, iconCoop, iconCR)
		b.WriteByte('\n')
	}

	// Footer
	if copyPaste || footer {
		b.WriteString("-# ")
		b.WriteString(iconCoop)
		b.WriteString(" coop size | 🎏 parade alts | ")
		b.WriteString(iconCR)
		b.WriteString(" target CRs | 🌼Seasonal LB\n")
		b.WriteString("-# Prediction formula by jelibean84\n")
	}

	if copyPaste {
		b.WriteString("```")
	}

	return &discordgo.TextDisplay{
		Content: b.String(),
	}
}

// writeContracts prints only the contract lines, reusing shared season metadata.
func writeContracts(
	b *strings.Builder,
	contracts []ei.EggIncContract,
	copyPaste bool,
	iconCoop, iconCR string,
) {
	for _, c := range contracts {
		paradeAlts := max(0, c.ChickenRuns-c.MaxCoopSize+1)

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

		eggEmoji := ei.FindEggEmoji(c.EggName)
		// Egg Server emoji format
		if copyPaste {
			eggEmoji = ":egg_" + strings.ToLower(strings.ReplaceAll(ei.Egg_name[c.Egg], "_", "")) + ":"
		}

		// First line
		fmt.Fprintf(
			b,
			"%s **[%s](https://eicoop-carpet.netlify.app/?q=%s)**",
			eggEmoji,
			c.Name,
			c.ID,
		)
		if seasonLabel != "" {
			b.WriteString("  ")
			b.WriteString(seasonLabel)
		}
		b.WriteByte('\n')

		// Second line
		fmt.Fprintf(
			b,
			"_      _%s `%2d`  🎏 `%2d`  %s `%2d`\n",
			iconCoop,
			c.MaxCoopSize,
			paradeAlts,
			iconCR,
			c.ChickenRuns,
		)
	}
}

// predictJeli returns up to 5 oldest in each leggacy contract type.
func predictJeli(
	contractCount int64,
) (fridayNonUltra, fridayUltra, wednesday []ei.EggIncContract) {
	for _, c := range ei.EggIncContractsAll {
		if c.HasPE {
			if !c.Ultra {
				fridayUltra = findOldestNContracts(fridayUltra, c, contractCount)
			} else {
				fridayNonUltra = findOldestNContracts(fridayNonUltra, c, contractCount)
			}
		} else {
			wednesday = findOldestNContracts(wednesday, c, contractCount)
		}
	}

	return
}

// findOldestNContracts sorts and keeps only the oldest N contracts in the slice.
func findOldestNContracts(
	top []ei.EggIncContract,
	c ei.EggIncContract,
	contractCount int64,
) []ei.EggIncContract {
	top = append(top, c)

	// Sort newly added contract into place
	i := len(top) - 1
	for i > 0 && top[i].ValidUntil.Before(top[i-1].ValidUntil) {
		top[i], top[i-1] = top[i-1], top[i]
		i--
	}

	// keep contractCount oldest
	if len(top) > int(contractCount) {
		top = top[:contractCount]
	}
	return top
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
