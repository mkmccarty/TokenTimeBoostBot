package boost

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/rs/xid"
)

var playStyles = []string{"Speedrun", "Fastrun", "Casual", "Public"}
var fairShare = []float64{1.5, 1.1, 1, 0.9, 0.5, 0.1}
var tokenSentValueStr = []string{"Tval Met", "3", "1", "0", "Sink"}
var tokenSentValue = []float64{50, 3, 1, 0, 20}
var tokenRecvValueStr = []string{"8", "6", "1", "0", "Sink"}
var tokenRecvValue = []float64{8, 6, 1, 0, 1000}
var chickenRunsStr = []string{"Max", "CoopSize", "CoopSize -1", "None"}
var siabDurationStr = []string{"30m", "45m", "Half Duration", "Full Duration"}
var deflectorDurationsStr = []string{"Full Duration", "CRT Offset", "Boost Time"}

type scoreCalcParams struct {
	xid                  string
	contractID           string
	contract             ei.EggIncContract
	contractInfo         string
	grade                ei.Contract_PlayerGrade
	public               bool
	deflector            int
	deflectorDownMinutes int
	siab                 int
	siabMinutes          int
	style                int
	playStyleValues      []float64
	fairShare            int
	tvalSent             int
	tvalReceived         int
	chickenRuns          int
	chickenRunValues     []int
	siabTimes            []int
	siabIndex            int
	deflTimes            []int
	deflIndex            int
}

var scoreCalcMap = make(map[string]scoreCalcParams)

// GetSlashScoreExplorerCommand returns the slash command for token tracking
func GetSlashScoreExplorerCommand(cmd string) *discordgo.ApplicationCommand {
	//adminPermission := int64(0)
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Start token value tracking for a contract",
		//DefaultMemberPermissions: &adminPermission,
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
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Contract ID",
				Required:     true,
				Autocomplete: true,
			},
			/*
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "grade",
					Description: "Contract Grade. Defaults to AAA",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "AAA",
							Value: ei.Contract_GRADE_AAA,
						},
						{
							Name:  "AA",
							Value: ei.Contract_GRADE_AA,
						},
						{
							Name:  "A",
							Value: ei.Contract_GRADE_A,
						},
						{
							Name:  "B",
							Value: ei.Contract_GRADE_B,
						},
						{
							Name:  "C",
							Value: ei.Contract_GRADE_C,
						},
					},
				},
			*/
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "public",
				Description: "Display this to everyone within this channel. Default is true.",
				Required:    false,
			},
		},
	}
}

// HandleScoreExplorerCommand will handle the /playground command
func HandleScoreExplorerCommand(s *discordgo.Session, i *discordgo.InteractionCreate) { // User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	flags := discordgo.MessageFlagsEphemeral
	grade := ei.Contract_PlayerGrade(ei.Contract_GRADE_AAA)
	var contractID string

	if contractID == "" {
		if opt, ok := optionMap["contract-id"]; ok {
			contractID = opt.StringValue()
		}
	}
	if opt, ok := optionMap["grade"]; ok {
		grade = ei.Contract_PlayerGrade(opt.IntValue())
	}
	if opt, ok := optionMap["public"]; ok {
		if opt.BoolValue() {
			flags = 0
		}
	}

	c := ei.EggIncContractsAll[contractID]

	if c.ID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown contract ID",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		},
		)
		return
	}

	xid := xid.New().String()
	scoreCalcParams := scoreCalcParams{
		xid:                  xid,
		contractID:           contractID,
		contract:             c,
		grade:                grade,
		public:               flags == 0,
		deflector:            0,
		deflectorDownMinutes: 0,
		siab:                 0,
		siabMinutes:          45,
		fairShare:            2,
		tvalSent:             0,
		tvalReceived:         1,
		chickenRuns:          2,
		contractInfo:         getContractEstimateString(contractID),
	}

	playStyleValues := []float64{1.0, 1.0, 1.20, 2.0}
	scoreCalcParams.playStyleValues = append(scoreCalcParams.playStyleValues, playStyleValues...)
	// Calculate CR Values
	crValues := []int{c.ChickenRuns, c.MaxCoopSize, c.MaxCoopSize - 1, 0}
	scoreCalcParams.chickenRunValues = append(scoreCalcParams.chickenRunValues, crValues...)

	siabTimes := []int{30, 45, -1, -1}
	scoreCalcParams.siabTimes = append(scoreCalcParams.siabTimes, siabTimes...)

	runLegs := 0
	legQty := c.MaxCoopSize - 1
	if c.ChickenRuns >= legQty {
		runLegs++
		remainingRuns := c.ChickenRunCooldownMinutes - legQty
		runLegs += int(math.Ceil(float64(remainingRuns) / float64(legQty-1)))
	}
	deflTimes := []int{0, runLegs * 7, 20}
	scoreCalcParams.deflTimes = append(scoreCalcParams.deflTimes, deflTimes...)

	scoreCalcMap[xid] = scoreCalcParams

	_, embed := getScoreExplorerCalculations(scoreCalcParams)

	components := getScoreExplorerComponents(scoreCalcParams)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    scoreCalcParams.contractInfo,
			Flags:      flags,
			Components: components,
			Embeds:     []*discordgo.MessageEmbed{embed},
			CustomID:   "maybe-store-data",
			Title:      "Contract Score Explorer",
		},
	},
	)
	if err != nil {
		log.Println(err)
	}
}

func getScoreExplorerCalculations(params scoreCalcParams) (string, *discordgo.MessageEmbed) {
	var field []*discordgo.MessageEmbedField
	var builder strings.Builder
	grade := params.grade

	c := params.contract

	if c.ID == "" {
		str := "No contract found in this channel, use the command parameters to pick one."
		return str, nil
	}

	durationSpeed := false
	if params.style == 0 {
		durationSpeed = true
	}

	ratio := fairShare[params.fairShare]

	contractDur := c.EstimatedDurationLower * time.Duration(params.playStyleValues[params.style])
	if !durationSpeed {
		contractDur = c.EstimatedDuration * time.Duration(params.playStyleValues[params.style])
	}

	params.siabTimes[len(params.siabTimes)-1] = int(contractDur.Minutes())
	params.siabTimes[len(params.siabTimes)-2] = int(contractDur.Minutes() / 2)

	params.siabMinutes = params.siabTimes[params.siabIndex]
	params.deflectorDownMinutes = params.deflTimes[params.deflIndex]

	scoreLower := getContractScoreEstimate(c, grade,
		durationSpeed, params.playStyleValues[params.style], ratio,
		params.siab, params.siabMinutes,
		params.deflector, params.deflectorDownMinutes,
		params.chickenRunValues[params.chickenRuns],
		tokenSentValue[params.tvalSent],
		tokenRecvValue[params.tvalReceived])
	fmt.Fprintf(&builder, "**%d**", scoreLower)

	field = append(field, &discordgo.MessageEmbedField{
		Name:   "Contract Score",
		Value:  builder.String(),
		Inline: true,
	})

	// Explain the settings
	builder.Reset()
	fmt.Fprintf(&builder, "Playstyle: %s - duration %v\n", playStyles[params.style], contractDur.Round(time.Minute).String())
	fmt.Fprintf(&builder, "Deflector: %d%% unequipped for %dm\n", params.deflector, params.deflectorDownMinutes)
	fmt.Fprintf(&builder, "SIAB: %d%% equipped for %dm\n", params.siab, params.siabMinutes)
	fmt.Fprintf(&builder, "Fair Share: %2.1fx\n", ratio)
	fmt.Fprintf(&builder, "TVal Sent: %s\n", tokenSentValueStr[params.tvalSent])
	fmt.Fprintf(&builder, "TVal Recv: %s\n", tokenRecvValueStr[params.tvalReceived])
	fmt.Fprintf(&builder, "Chicken Runs: %d\n", params.chickenRunValues[params.chickenRuns])

	embed := &discordgo.MessageEmbed{}
	embed.Title = "Score Explorer"
	embed.Description = builder.String()
	embed.Color = 0x9a8b7c
	embed.Fields = field

	return builder.String(), embed
}

// getTokenValComponents returns the components for the token value
func getScoreExplorerComponents(param scoreCalcParams) []discordgo.MessageComponent {
	var buttons []discordgo.Button
	var menu []discordgo.SelectMenu

	/*
		Boost Style: Speedrun, Fastrun, Casual
		Boost Position: [Mid/Early/Late]
		Deflector Quality: [range] [duration]
		SIAB Quality: [range] [duration]
		Tokens: Met Tval, Not Quite, Low, Sink.
		Chicken Runs: CRT, Coop-size, none.
	*/

	buttons = append(buttons,
		discordgo.Button{
			Label:    playStyles[param.style],
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#style", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("Fair Share: %2.1fx", fairShare[param.fairShare]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#fair", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("TVal Sent: %s", tokenSentValueStr[param.tvalSent]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#tvals", param.xid),
		})
	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("TVal Recv: %s", tokenRecvValueStr[param.tvalReceived]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#tvalr", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("Chicken Runs: %s", chickenRunsStr[param.chickenRuns]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#runs", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("Defl Offset: %s", deflectorDurationsStr[param.deflIndex]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#defltime", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("SIAB: %s", siabDurationStr[param.siabIndex]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#siabtime", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    "Close",
			Style:    discordgo.DangerButton,
			CustomID: fmt.Sprintf("fd_playground#%s#close", param.xid),
		})

	MinValues := 1

	menu = append(menu, discordgo.SelectMenu{
		CustomID:    fmt.Sprintf("fd_playground#%s#deflector", param.xid),
		Placeholder: "Deflector Quality",
		MaxValues:   1,
		MinValues:   &MinValues,
		Options: []discordgo.SelectMenuOption{
			{
				Label:   "No Deflector Used",
				Value:   "0",
				Default: param.deflector == 0,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T4L"),
				Label:   "T4L",
				Value:   "20",
				Default: param.deflector == 20,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T4E"),
				Label:   "T4E",
				Value:   "19",
				Default: param.deflector == 19,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T4R"),
				Label:   "T4R",
				Value:   "17",
				Default: param.deflector == 17,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T4C"),
				Label:   "T4C",
				Value:   "15",
				Default: param.deflector == 15,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T3R"),
				Label:   "T3R",
				Value:   "13",
				Default: param.deflector == 13,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T3C"),
				Label:   "T3C",
				Value:   "12",
				Default: param.deflector == 12,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T2C"),
				Label:   "T2C",
				Value:   "8",
				Default: param.deflector == 8,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T1C"),
				Label:   "T1C",
				Value:   "5",
				Default: param.deflector == 5,
			},
		},
	})

	menu = append(menu, discordgo.SelectMenu{
		CustomID:    fmt.Sprintf("fd_playground#%s#siab", param.xid),
		Placeholder: "SIAB Quality",
		MaxValues:   1,
		MinValues:   &MinValues,

		Options: []discordgo.SelectMenuOption{
			{
				Label:   "No SIAB Used",
				Value:   "0",
				Default: param.siab == 0,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T4L"),
				Label:   "T4L",
				Value:   "100",
				Default: param.siab == 100,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T4E"),
				Label:   "T4E",
				Value:   "90",
				Default: param.siab == 90,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T4R"),
				Label:   "T4R",
				Value:   "80",
				Default: param.siab == 80,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T4C"),
				Label:   "T4C",
				Value:   "70",
				Default: param.siab == 70,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T3R"),
				Label:   "T3R",
				Value:   "60",
				Default: param.siab == 60,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T3C"),
				Label:   "T3C",
				Value:   "50",
				Default: param.siab == 50,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T2C"),
				Label:   "T2C",
				Value:   "30",
				Default: param.siab == 30,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T1C"),
				Label:   "T1C",
				Value:   "20",
				Default: param.siab == 20,
			},
		},
	})
	/*
		buttons = append(buttons,
			discordgo.Button{
				Label:    "Tile/Table",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("fd_stones#%s#toggle", name),
			})
	*/

	var components []discordgo.MessageComponent
	/*
		for _, m := range menu {
			components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{m}})
		}
	*/

	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu[0]}})
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu[1]}})

	for i := 0; i < len(buttons); i += 5 {
		end := i + 5
		if end > len(buttons) {
			end = len(buttons)
		}
		var rowComponents []discordgo.MessageComponent
		for _, button := range buttons[i:end] {
			rowComponents = append(rowComponents, button)
		}
		components = append(components, discordgo.ActionsRow{Components: rowComponents})
	}
	/*

		buttons = append(,
			discordgo.Button{
				Label:    "Close",
				Style:    discordgo.DangerButton,
				CustomID: fmt.Sprintf("fd_playground#%s#close", param.xid),
			})
	*/

	return components

}

// HandleScoreExplorerPage steps a page of cached teamwork data
func HandleScoreExplorerPage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// cs_#Name # cs_#ID # HASH
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

	params, exists := scoreCalcMap[reaction[1]]
	if !exists {
		log.Println("Invalid reaction ID")
		_ = s.InteractionResponseDelete(i.Interaction)
		return
	}

	if err != nil {
		log.Println(err)
	}
	if len(reaction) == 3 && reaction[2] == "style" {
		params.style++
		if params.style >= len(playStyles) {
			params.style = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "fair" {
		params.fairShare++
		if params.fairShare >= len(fairShare) {
			params.fairShare = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "tvals" {
		params.tvalSent++
		if params.tvalSent >= len(tokenSentValue) {
			params.tvalSent = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "tvalr" {
		params.tvalReceived++
		if params.tvalReceived >= len(tokenRecvValue) {
			params.tvalReceived = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "runs" {
		params.chickenRuns++
		if params.chickenRuns >= len(chickenRunsStr) {
			params.chickenRuns = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "defltime" {
		params.deflIndex++
		if params.deflIndex >= len(deflectorDurationsStr) {
			params.deflIndex = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "siabtime" {
		params.siabIndex++
		if params.siabIndex >= len(siabDurationStr) {
			params.siabIndex = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "deflector" {
		deflectorValue, err := strconv.Atoi(i.MessageComponentData().Values[0])
		if err != nil {
			log.Println("Invalid deflector value:", err)
			return
		}
		params.deflector = deflectorValue
	}
	if len(reaction) == 3 && reaction[2] == "siab" {
		siabValue, err := strconv.Atoi(i.MessageComponentData().Values[0])
		if err != nil {
			log.Println("Invalid deflector value:", err)
			return
		}
		params.siab = siabValue
	}

	if len(reaction) == 3 && reaction[2] == "close" {
		_ = s.InteractionResponseDelete(i.Interaction)
		return
	}
	scoreCalcMap[params.xid] = params

	_, embed := getScoreExplorerCalculations(params)

	components := getScoreExplorerComponents(params)
	embeds := []*discordgo.MessageEmbed{embed}

	edit := discordgo.WebhookEdit{
		Content:    &params.contractInfo,
		Components: &components,
		Embeds:     &embeds,
	}

	_, err = s.FollowupMessageEdit(i.Interaction, i.Message.ID, &edit)
	if err != nil {
		log.Println(err)
	}
}
