package boost

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

var playStyles = []string{"Speedrun", "Fastrun", "Casual", "Public"}

// var fairShare = []float64{1.5, 1.1, 1, 0.9, 0.5, 0.1}
var chickenRunsStr = []string{"Max", "None"}
var siabDurationStr = []string{"30m", "Half Duration", "Full Duration"}
var deflectorDurationsStr = []string{"Full Duration", "Boost Time"}

// ScoreCalcParams is the parameters for the score calculator
type ScoreCalcParams struct {
	xid                  string
	contractID           string
	contract             ei.EggIncContract
	contractInfo         string
	Grade                ei.Contract_PlayerGrade `json:"grade"`
	public               bool
	Deflector            float64   `json:"deflector"`
	DeflectorDownMinutes int       `json:"deflector_down_minutes"`
	Siab                 float64   `json:"siab"`
	SiabMinutes          int       `json:"siab_minutes"`
	Style                int       `json:"style"`
	PlayStyleValues      []float64 `json:"play_style_values"`
	FairShare            float64   `json:"fair_share"`
	ChickenRuns          int
	chickenRunValues     []int
	SiabTimes            []int `json:"siab_times"`
	SiabIndex            int   `json:"siab_index"`
	deflTimes            []int
	DeflIndex            int `json:"defl_index"`
}

var scoreCalcMap = make(map[string]ScoreCalcParams)

// GetSlashScoreExplorerCommand returns the slash command for token tracking
func GetSlashScoreExplorerCommand(cmd string) *dc.Command {
	//adminPermission := int64(0)
	command := anywhereCommand(cmd, "Start token value tracking for a contract")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "contract-id",
			Description:  "Contract ID",
			Required:     true,
			Autocomplete: true,
		},
		dc.BoolOption{
			Name:        "public",
			Description: "Display this to everyone within this channel. Default is true.",
		},
	}
	return &command
}

// HandleScoreExplorerCommand handles the score explorer command through the dc
// facade.
func HandleScoreExplorerCommand(e *dc.CommandEvent) {
	ephemeral := true
	grade := ei.Contract_PlayerGrade(ei.Contract_GRADE_AAA)
	var contractID string

	if contractID == "" {
		if opt, ok := e.OptString("contract-id"); ok {
			contractID = opt
		}
	}
	if opt, ok := e.OptInt("grade"); ok {
		grade = ei.Contract_PlayerGrade(opt)
	}
	if opt, ok := e.OptBool("public"); ok {
		if opt {
			ephemeral = false
		}
	}

	c := ei.EggIncContractsAll[contractID]

	if c.ID == "" {
		_ = e.Respond(dc.Message{
			Content:   "Unknown contract ID",
			Ephemeral: true,
		})
		return
	}
	_ = e.Defer(ephemeral)

	xid := uuid.NewV7().String()
	scoreCalcParams := ScoreCalcParams{
		xid:                  xid,
		contractID:           contractID,
		contract:             c,
		Grade:                grade,
		public:               !ephemeral,
		Deflector:            0,
		DeflectorDownMinutes: 0,
		Siab:                 0,
		SiabMinutes:          45,
		FairShare:            1.0,
		ChickenRuns:          0,
		contractInfo:         GetContractEstimateString(contractID, false),
	}

	playStyleValues := []float64{1.0, 1.0, 1.20, 2.0}
	scoreCalcParams.PlayStyleValues = append(scoreCalcParams.PlayStyleValues, playStyleValues...)
	// Calculate CR Values
	crValues := []int{c.ChickenRuns, 0}
	scoreCalcParams.chickenRunValues = append(scoreCalcParams.chickenRunValues, crValues...)

	siabTimes := []int{30, -1, -1}
	scoreCalcParams.SiabTimes = append(scoreCalcParams.SiabTimes, siabTimes...)

	deflTimes := []int{0, 20}
	scoreCalcParams.deflTimes = append(scoreCalcParams.deflTimes, deflTimes...)

	scoreCalcMap[xid] = scoreCalcParams

	_, embed := getScoreExplorerCalculations(scoreCalcParams)

	components := getScoreExplorerComponents(scoreCalcParams)

	// Content sits beside select menus and an embed here, which is the legacy
	// component model.
	err := e.Followup(dc.Message{
		Content:      scoreCalcParams.contractInfo,
		Ephemeral:    ephemeral,
		Components:   components,
		Embeds:       []dc.Embed{embed},
		ComponentsV1: true,
	})
	if err != nil {
		log.Println(err)
	}
}

func getScoreExplorerCalculations(params ScoreCalcParams) (string, dc.Embed) {
	var field []dc.EmbedField
	var builder strings.Builder
	grade := params.Grade

	c := params.contract

	if c.ID == "" {
		str := "No contract found in this channel, use the command parameters to pick one."
		return str, dc.Embed{}
	}

	ratio := params.FairShare

	contractDur := c.EstimatedDurationLower
	switch params.Style {
	case 0: // Speedrun
		contractDur = c.EstimatedDurationMax
	case 1: // Fastrun
		contractDur = c.EstimatedDurationLower
	case 2: // Casual
		contractDur = c.EstimatedDuration
	case 3: // Public
		contractDur = time.Duration(float64(c.EstimatedDuration) * 1.1)
	}

	params.SiabTimes[len(params.SiabTimes)-1] = int(contractDur.Minutes())
	params.SiabTimes[len(params.SiabTimes)-2] = int(contractDur.Minutes() / 2)

	params.SiabMinutes = params.SiabTimes[params.SiabIndex]
	params.DeflectorDownMinutes = params.deflTimes[params.DeflIndex]

	scoreLower := getContractScoreEstimateWithDuration(c, grade,
		contractDur, ratio,
		params.Siab, params.SiabMinutes,
		params.Deflector, params.DeflectorDownMinutes,
		params.chickenRunValues[params.ChickenRuns],
		0,
		0)
	fmt.Fprintf(&builder, "**%d**", scoreLower)

	field = append(field, dc.EmbedField{
		Name:   "Contract Score",
		Value:  builder.String(),
		Inline: true,
	})

	// Explain the settings
	builder.Reset()
	fmt.Fprintf(&builder, "Playstyle: %s - duration %s\n", playStyles[params.Style], bottools.FmtDuration(contractDur))
	fmt.Fprintf(&builder, "Deflector: %.1f%% unequipped for %s\n", params.Deflector, bottools.FmtDuration(time.Duration(params.DeflectorDownMinutes)*time.Minute))
	fmt.Fprintf(&builder, "SIAB: %.1f%% equipped for %s\n", params.Siab, bottools.FmtDuration(time.Duration(params.SiabMinutes)*time.Minute))
	fmt.Fprintf(&builder, "Fair Share: %2.3g\n", ratio)
	fmt.Fprintf(&builder, "Chicken Runs: %d\n", params.chickenRunValues[params.ChickenRuns])

	embed := dc.Embed{
		Title:       "Score Explorer",
		Description: builder.String(),
		Color:       0x9a8b7c,
		Fields:      field,
	}

	return builder.String(), embed
}

// getTokenValComponents returns the components for the token value
func getScoreExplorerComponents(param ScoreCalcParams) []dc.LayoutComponent {
	var buttons []dc.Button
	var menu []dc.SelectMenu

	buttons = append(buttons,
		dc.Button{
			Label:    playStyles[param.Style],
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("fd_playground#%s#style", param.xid),
		})

	buttons = append(buttons,
		dc.Button{
			Label:    fmt.Sprintf("Chicken Runs: %s", chickenRunsStr[param.ChickenRuns]),
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("fd_playground#%s#runs", param.xid),
		})

	buttons = append(buttons,
		dc.Button{
			Label:    fmt.Sprintf("Deflector Use: %s", deflectorDurationsStr[param.DeflIndex]),
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("fd_playground#%s#defltime", param.xid),
		})

	buttons = append(buttons,
		dc.Button{
			Label:    fmt.Sprintf("SIAB Equip Time: %s", siabDurationStr[param.SiabIndex]),
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("fd_playground#%s#siabtime", param.xid),
		})

	if !param.public {
		buttons = append(buttons,
			dc.Button{
				Label:    "Load Settings",
				Style:    dc.ButtonPrimary,
				CustomID: fmt.Sprintf("fd_playground#%s#load", param.xid),
			})
		buttons = append(buttons,
			dc.Button{
				Label:    "Save Settings",
				Style:    dc.ButtonPrimary,
				CustomID: fmt.Sprintf("fd_playground#%s#save", param.xid),
			})
	}

	buttons = append(buttons,
		dc.Button{
			Label:    "Close",
			Style:    dc.ButtonDanger,
			CustomID: fmt.Sprintf("fd_playground#%s#close", param.xid),
		})

	MinValues := 1

	fairShareOptions := []dc.SelectOption{}
	// I want to create a float64 array with several values
	var fairShare = []float64{1.5, 1.2, 1.1}
	for ratio := 1.07; ratio >= 0.95; ratio -= 0.01 {
		fairShare = append(fairShare, ratio)
	}
	fairShare = append(fairShare, 0.9, 0.8, 0.5, 0.1, 0.0)
	for _, ratio := range fairShare {
		str := ""
		if ratio == 1.0 {
			str = "Fair Share "
		}
		fairShareOptions = append(fairShareOptions, dc.SelectOption{
			Label:   fmt.Sprintf("%s%2.2f", str, ratio),
			Value:   fmt.Sprintf("%1.2f", ratio),
			Default: param.FairShare == ratio,
		})
	}

	menu = append(menu, dc.SelectMenu{
		CustomID:    fmt.Sprintf("fd_playground#%s#fair", param.xid),
		Placeholder: "Fair Share",
		MaxValues:   1,
		MinValues:   &MinValues,
		Options:     fairShareOptions,
	})

	menu = append(menu, dc.SelectMenu{
		CustomID:    fmt.Sprintf("fd_playground#%s#deflector", param.xid),
		Placeholder: "Deflector Quality",
		MaxValues:   1,
		MinValues:   &MinValues,
		Options: []dc.SelectOption{
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

	menu = append(menu, dc.SelectMenu{
		CustomID:    fmt.Sprintf("fd_playground#%s#siab", param.xid),
		Placeholder: "SIAB Quality",
		MaxValues:   1,
		MinValues:   &MinValues,

		Options: []dc.SelectOption{
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

	var components []dc.LayoutComponent

	components = append(components, dc.ActionRow{Components: []dc.InteractiveComponent{menu[0]}})
	components = append(components, dc.ActionRow{Components: []dc.InteractiveComponent{menu[1]}})
	components = append(components, dc.ActionRow{Components: []dc.InteractiveComponent{menu[2]}})

	for i := 0; i < len(buttons); i += 5 {
		end := i + 5
		if end > len(buttons) {
			end = len(buttons)
		}
		var rowComponents []dc.InteractiveComponent
		for _, button := range buttons[i:end] {
			rowComponents = append(rowComponents, button)
		}
		components = append(components, dc.ActionRow{Components: rowComponents})
	}

	return components
}

// HandleScoreExplorerPage steps a page of the score explorer through the dc
// facade.
func HandleScoreExplorerPage(e *dc.ComponentEvent) {
	// cs_#Name # cs_#ID # HASH
	reaction := strings.Split(e.CustomID(), "#")

	err := e.DeferUpdate()

	params, exists := scoreCalcMap[reaction[1]]
	if !exists {
		log.Println("Invalid reaction ID")
		_ = e.DeleteResponse()
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
		data := e.Values()
		values := data
		fairShareValue, err := strconv.ParseFloat(values[0], 64)
		if err != nil {
			log.Println("Invalid fair share value:", err)
		} else {
			params.FairShare = fairShareValue
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
		deflectorValue, err := strconv.Atoi(e.Values()[0])
		if err != nil {
			log.Println("Invalid deflector value:", err)
			return
		}
		params.Deflector = float64(deflectorValue)
	}
	if len(reaction) == 3 && reaction[2] == "siab" {
		siabValue, err := strconv.Atoi(e.Values()[0])
		if err != nil {
			log.Println("Invalid deflector value:", err)
			return
		}
		params.Siab = float64(siabValue)
	}
	if len(reaction) == 3 && reaction[2] == "load" {
		// Load settings
		userID := e.UserID()
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
			userID := e.UserID()
			paramsStr := string(paramsBytes)
			farmerstate.SetMiscSettingString(userID, "scoreCalcParams", paramsStr)
		}
	}

	if len(reaction) == 3 && reaction[2] == "close" {
		_ = e.DeleteResponse()
		return
	}

	scoreCalcMap[params.xid] = params

	_, embed := getScoreExplorerCalculations(params)

	components := getScoreExplorerComponents(params)
	err = e.EditFollowup(e.MessageID(), dc.Message{
		Content:      params.contractInfo,
		Components:   components,
		Embeds:       []dc.Embed{embed},
		ComponentsV1: true,
	})
	if err != nil {
		log.Println(err)
	}
}
