package boost

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
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

// ScoreCalcParams is the parameters for the score calculator
type ScoreCalcParams struct {
	xid                  string
	contractID           string
	contract             ei.EggIncContract
	contractInfo         string
	Grade                ei.Contract_PlayerGrade `json:"grade"`
	public               bool
	Deflector            int       `json:"deflector"`
	DeflectorDownMinutes int       `json:"deflector_down_minutes"`
	Siab                 int       `json:"siab"`
	SiabMinutes          int       `json:"siab_minutes"`
	Style                int       `json:"style"`
	PlayStyleValues      []float64 `json:"play_style_values"`
	FairShare            int       `json:"fair_share"`
	TvalSent             int       `json:"tval_sent"`
	TvalReceived         int       `json:"tval_received"`
	ChickenRuns          int
	chickenRunValues     []int
	SiabTimes            []int `json:"siab_times"`
	SiabIndex            int   `json:"siab_index"`
	deflTimes            []int
	DeflIndex            int `json:"defl_index"`
}

var scoreCalcMap = make(map[string]ScoreCalcParams)

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
	scoreCalcParams := ScoreCalcParams{
		xid:                  xid,
		contractID:           contractID,
		contract:             c,
		Grade:                grade,
		public:               flags == 0,
		Deflector:            0,
		DeflectorDownMinutes: 0,
		Siab:                 0,
		SiabMinutes:          45,
		FairShare:            2,
		TvalSent:             0,
		TvalReceived:         1,
		ChickenRuns:          2,
		contractInfo:         getContractEstimateString(contractID),
	}

	playStyleValues := []float64{1.0, 1.0, 1.20, 2.0}
	scoreCalcParams.PlayStyleValues = append(scoreCalcParams.PlayStyleValues, playStyleValues...)
	// Calculate CR Values
	crValues := []int{c.ChickenRuns, c.MaxCoopSize, c.MaxCoopSize - 1, 0}
	scoreCalcParams.chickenRunValues = append(scoreCalcParams.chickenRunValues, crValues...)

	siabTimes := []int{30, 45, -1, -1}
	scoreCalcParams.SiabTimes = append(scoreCalcParams.SiabTimes, siabTimes...)

	runLegs := 0
	legQty := c.MaxCoopSize - 1
	if c.ChickenRuns >= legQty {
		runLegs++
		remainingRuns := c.ChickenRuns - legQty
		runLegs += int(math.Floor(float64(remainingRuns) / float64(legQty-1)))
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

func getScoreExplorerCalculations(params ScoreCalcParams) (string, *discordgo.MessageEmbed) {
	var field []*discordgo.MessageEmbedField
	var builder strings.Builder
	grade := params.Grade

	c := params.contract

	if c.ID == "" {
		str := "No contract found in this channel, use the command parameters to pick one."
		return str, nil
	}

	durationSpeed := false
	if params.Style == 0 {
		durationSpeed = true
	}

	ratio := fairShare[params.FairShare]

	contractDur := c.EstimatedDurationLower * time.Duration(params.PlayStyleValues[params.Style])
	if !durationSpeed {
		contractDur = c.EstimatedDuration * time.Duration(params.PlayStyleValues[params.Style])
	}

	params.SiabTimes[len(params.SiabTimes)-1] = int(contractDur.Minutes())
	params.SiabTimes[len(params.SiabTimes)-2] = int(contractDur.Minutes() / 2)

	params.SiabMinutes = params.SiabTimes[params.SiabIndex]
	params.DeflectorDownMinutes = params.deflTimes[params.DeflIndex]

	scoreLower := getContractScoreEstimate(c, grade,
		durationSpeed, params.PlayStyleValues[params.Style], ratio,
		params.Siab, params.SiabMinutes,
		params.Deflector, params.DeflectorDownMinutes,
		params.chickenRunValues[params.ChickenRuns],
		tokenSentValue[params.TvalSent],
		tokenRecvValue[params.TvalReceived])
	fmt.Fprintf(&builder, "**%d**", scoreLower)

	field = append(field, &discordgo.MessageEmbedField{
		Name:   "Contract Score",
		Value:  builder.String(),
		Inline: true,
	})

	// Explain the settings
	builder.Reset()
	fmt.Fprintf(&builder, "Playstyle: %s - duration %v\n", playStyles[params.Style], contractDur.Round(time.Minute).String())
	fmt.Fprintf(&builder, "Deflector: %d%% unequipped for %dm\n", params.Deflector, params.DeflectorDownMinutes)
	fmt.Fprintf(&builder, "SIAB: %d%% equipped for %dm\n", params.Siab, params.SiabMinutes)
	fmt.Fprintf(&builder, "Fair Share: %2.1fx\n", ratio)
	fmt.Fprintf(&builder, "TVal Sent: %s\n", tokenSentValueStr[params.TvalSent])
	fmt.Fprintf(&builder, "TVal Recv: %s\n", tokenRecvValueStr[params.TvalReceived])
	fmt.Fprintf(&builder, "Chicken Runs: %d\n", params.chickenRunValues[params.ChickenRuns])

	embed := &discordgo.MessageEmbed{}
	embed.Title = "Score Explorer"
	embed.Description = builder.String()
	embed.Color = 0x9a8b7c
	embed.Fields = field

	return builder.String(), embed
}

// getTokenValComponents returns the components for the token value
func getScoreExplorerComponents(param ScoreCalcParams) []discordgo.MessageComponent {
	var buttons []discordgo.Button
	var menu []discordgo.SelectMenu

	buttons = append(buttons,
		discordgo.Button{
			Label:    playStyles[param.Style],
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#style", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("Fair Share: %2.1fx", fairShare[param.FairShare]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#fair", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("TVal Sent: %s", tokenSentValueStr[param.TvalSent]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#tvals", param.xid),
		})
	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("TVal Recv: %s", tokenRecvValueStr[param.TvalReceived]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#tvalr", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("Chicken Runs: %s", chickenRunsStr[param.ChickenRuns]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#runs", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("Deflector Use: %s", deflectorDurationsStr[param.DeflIndex]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#defltime", param.xid),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    fmt.Sprintf("SIAB Equp Time: %s", siabDurationStr[param.SiabIndex]),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_playground#%s#siabtime", param.xid),
		})

	if !param.public {
		buttons = append(buttons,
			discordgo.Button{
				Label:    "Load Settings",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("fd_playground#%s#load", param.xid),
			})
		buttons = append(buttons,
			discordgo.Button{
				Label:    "Save Settings",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("fd_playground#%s#save", param.xid),
			})
	}

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
				Default: param.Deflector == 0,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T4L"),
				Label:   "T4L",
				Value:   "20",
				Default: param.Deflector == 20,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T4E"),
				Label:   "T4E",
				Value:   "19",
				Default: param.Deflector == 19,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T4R"),
				Label:   "T4R",
				Value:   "17",
				Default: param.Deflector == 17,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T4C"),
				Label:   "T4C",
				Value:   "15",
				Default: param.Deflector == 15,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T3R"),
				Label:   "T3R",
				Value:   "13",
				Default: param.Deflector == 13,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T3C"),
				Label:   "T3C",
				Value:   "12",
				Default: param.Deflector == 12,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T2C"),
				Label:   "T2C",
				Value:   "8",
				Default: param.Deflector == 8,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("defl_T1C"),
				Label:   "T1C",
				Value:   "5",
				Default: param.Deflector == 5,
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
				Default: param.Siab == 0,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T4L"),
				Label:   "T4L",
				Value:   "100",
				Default: param.Siab == 100,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T4E"),
				Label:   "T4E",
				Value:   "90",
				Default: param.Siab == 90,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T4R"),
				Label:   "T4R",
				Value:   "80",
				Default: param.Siab == 80,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T4C"),
				Label:   "T4C",
				Value:   "70",
				Default: param.Siab == 70,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T3R"),
				Label:   "T3R",
				Value:   "60",
				Default: param.Siab == 60,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T3C"),
				Label:   "T3C",
				Value:   "50",
				Default: param.Siab == 50,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T2C"),
				Label:   "T2C",
				Value:   "30",
				Default: param.Siab == 30,
			},
			{
				Emoji:   ei.GetBotComponentEmoji("siab_T1C"),
				Label:   "T1C",
				Value:   "20",
				Default: param.Siab == 20,
			},
		},
	})

	var components []discordgo.MessageComponent

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
		params.Style++
		if params.Style >= len(playStyles) {
			params.Style = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "fair" {
		params.FairShare++
		if params.FairShare >= len(fairShare) {
			params.FairShare = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "tvals" {
		params.TvalSent++
		if params.TvalSent >= len(tokenSentValue) {
			params.TvalSent = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "tvalr" {
		params.TvalReceived++
		if params.TvalReceived >= len(tokenRecvValue) {
			params.TvalReceived = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "runs" {
		params.ChickenRuns++
		if params.ChickenRuns >= len(chickenRunsStr) {
			params.ChickenRuns = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "defltime" {
		params.DeflIndex++
		if params.DeflIndex >= len(deflectorDurationsStr) {
			params.DeflIndex = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "siabtime" {
		params.SiabIndex++
		if params.SiabIndex >= len(siabDurationStr) {
			params.SiabIndex = 0
		}
	}
	if len(reaction) == 3 && reaction[2] == "deflector" {
		deflectorValue, err := strconv.Atoi(i.MessageComponentData().Values[0])
		if err != nil {
			log.Println("Invalid deflector value:", err)
			return
		}
		params.Deflector = deflectorValue
	}
	if len(reaction) == 3 && reaction[2] == "siab" {
		siabValue, err := strconv.Atoi(i.MessageComponentData().Values[0])
		if err != nil {
			log.Println("Invalid deflector value:", err)
			return
		}
		params.Siab = siabValue
	}
	if len(reaction) == 3 && reaction[2] == "load" {
		// Load settings
		userID := getInteractionUserID(i)
		paramsStr := farmerstate.GetMiscSettingString(userID, "scoreCalcParams")
		if err != nil {
			log.Println("Error loading settings:", err)
			return
		}
		var loadedParams ScoreCalcParams
		err = json.Unmarshal([]byte(paramsStr), &loadedParams)
		if err != nil {
			log.Println("Error unmarshalling settings:", err)
			return
		}
		params.Grade = loadedParams.Grade
		params.Deflector = loadedParams.Deflector
		params.DeflectorDownMinutes = loadedParams.DeflectorDownMinutes
		params.Siab = loadedParams.Siab
		params.SiabMinutes = loadedParams.SiabMinutes
		params.Style = loadedParams.Style
		params.PlayStyleValues = loadedParams.PlayStyleValues
		params.FairShare = loadedParams.FairShare
		params.TvalSent = loadedParams.TvalSent
		params.TvalReceived = loadedParams.TvalReceived
		params.SiabTimes = loadedParams.SiabTimes
		params.SiabIndex = loadedParams.SiabIndex
		params.DeflIndex = loadedParams.DeflIndex
		params.ChickenRuns = loadedParams.ChickenRuns
	}
	if len(reaction) == 3 && reaction[2] == "save" {
		// Save settings
		// Want to save params to farmerstate.
		//farmerstate.SetMiscSettingString()
		paramsBytes, err := json.Marshal(params)
		if err == nil {
			userID := getInteractionUserID(i)
			paramsStr := string(paramsBytes)
			farmerstate.SetMiscSettingString(userID, "scoreCalcParams", paramsStr)
		}
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
