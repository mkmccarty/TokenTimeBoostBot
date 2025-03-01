package boost

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/rs/xid"
)

var playStyles = []string{"Speedrun", "Fastrun", "Casual", "Public"}
var playStyleValues = []float64{1.0, 1.0, 1.2, 3.0}
var fairShare = []float64{1.5, 1.1, 1, 0.9, 0.5, 0.1}
var tokenSentValueStr = []string{"50", "3", "1", "0", "Sink"}
var tokenSentValue = []float64{50, 3, 1, 0, 20}
var tokenRecvValueStr = []string{"8", "6", "1", "0", "Sink"}
var tokenRecvValue = []float64{8, 6, 1, 0, 1000}
var chickenRunsStr = []string{"Met", "CoopSize", "CoopSize -1", "None"}

type scoreCalcParams struct {
	xid              string
	contractID       string
	contract         ei.EggIncContract
	contractInfo     string
	grade            ei.Contract_PlayerGrade
	public           bool
	style            int
	playStyleValues  []float64
	fairShare        int
	tvalSent         int
	tvalReceived     int
	chickenRuns      int
	chickenRunValues []int
}

var scoreCalcMap = make(map[string]scoreCalcParams)

// GetSlashScorePlaygroundCommand returns the slash command for token tracking
func GetSlashScorePlaygroundCommand(cmd string) *discordgo.ApplicationCommand {
	adminPermission := int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Start token value tracking for a contract",
		DefaultMemberPermissions: &adminPermission,
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
				Required:     false,
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

// HandleScorePlaygroundCommand will handle the /playground command
func HandleScorePlaygroundCommand(s *discordgo.Session, i *discordgo.InteractionCreate) { // User interacting with bot, is this first time ?
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

	xid := xid.New().String()
	scoreCalcParams := scoreCalcParams{
		xid:          xid,
		contractID:   contractID,
		contract:     c,
		grade:        grade,
		public:       flags == 0,
		fairShare:    2,
		tvalSent:     0,
		tvalReceived: 1,
		chickenRuns:  2,
		contractInfo: getContractEstimateString(contractID, false),
	}

	playStyleValues := []float64{1.0, 1.0, 1.20, 2.0}
	scoreCalcParams.playStyleValues = append(scoreCalcParams.playStyleValues, playStyleValues...)
	// Calculate CR Values
	crValues := []int{c.ChickenRuns, c.MaxCoopSize, c.MaxCoopSize - 1, 0}
	scoreCalcParams.chickenRunValues = append(scoreCalcParams.chickenRunValues, crValues...)
	scoreCalcMap[xid] = scoreCalcParams

	_, embed := getScorePlaygroundCalculations(scoreCalcParams)

	components := getScorePlaygroundComponents(scoreCalcParams)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    scoreCalcParams.contractInfo,
			Flags:      flags,
			Components: components,
			Embeds:     []*discordgo.MessageEmbed{embed},
			CustomID:   "maybe-store-data",
			Title:      "Contract Score Playground",
		},
	},
	)
	if err != nil {
		log.Println(err)
	}
}

func getScorePlaygroundCalculations(params scoreCalcParams) (string, *discordgo.MessageEmbed) {
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

	scoreLower := getContractScoreEstimate(c, grade,
		durationSpeed, params.playStyleValues[params.style], ratio, 100, 45, 20, 0,
		params.chickenRunValues[params.chickenRuns],
		tokenSentValue[params.tvalSent],
		tokenRecvValue[params.tvalReceived])
	//score := getContractScoreEstimate(c, grade, false, ratio, 60, 45, 15, 0, c.MaxCoopSize-1, 100, 5)
	//scoreSink := getContractScoreEstimate(c, grade, false, ratio, 60, 45, 15, 0, c.MaxCoopSize-1, 3, 100)
	fmt.Fprintf(&builder, "Contract Score Top: **%d** (100%%/20%%/CR/TVAL)\n", scoreLower)
	//fmt.Fprintf(&builder, "Contract Score ACO Fastrun: **%d**(60%%/15%%/TVAL)\n", score)
	//fmt.Fprintf(&builder, "Contract Score Sink: **%d**(60%%/15%%)\n", scoreSink)

	field = append(field, &discordgo.MessageEmbedField{
		Name:   "Contract Score",
		Value:  builder.String(),
		Inline: true,
	})

	embed := &discordgo.MessageEmbed{}
	embed.Title = fmt.Sprintf("Score Playground")
	embed.Description = fmt.Sprintf("Calculations for contract %s", params.contractID)
	embed.Fields = field

	return builder.String(), embed
}

// getTokenValComponents returns the components for the token value
func getScorePlaygroundComponents(param scoreCalcParams) []discordgo.MessageComponent {
	var buttons []discordgo.Button

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
			Label:    fmt.Sprintf("%s", playStyles[param.style]),
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
	/*
		deflectorQuality := []string{"T4L", "T4E", "T4R", "T4C", "T3R"}
		buttons = append(buttons,
			discordgo.Button{
				Label:    fmt.Sprintf("Deflector: %s", deflectorQuality[0]),
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("fd_playground#%s#deflector", param.xid),
			})

		siabQuality := []string{"T4L", "T4E", "T4R", "T4C", "T3R"}
		buttons = append(buttons,
			discordgo.Button{
				Label:    fmt.Sprintf("SIAB: %s", siabQuality[0]),
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("fd_playground#%s#siab", param.xid),
			})
	*/
	/*
		buttons = append(buttons,
			discordgo.Button{
				Label:    "Tile/Table",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("fd_stones#%s#toggle", name),
			})
	*/
	buttons = append(buttons,
		discordgo.Button{
			Label:    "Close",
			Style:    discordgo.DangerButton,
			CustomID: fmt.Sprintf("fd_playground#%s#close", param.xid),
		})

	var components []discordgo.MessageComponent

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

// HandleScorePlaygroundPage steps a page of cached teamwork data
func HandleScorePlaygroundPage(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
		s.InteractionResponseDelete(i.Interaction)
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
	if len(reaction) == 3 && reaction[2] == "close" {
		s.InteractionResponseDelete(i.Interaction)
		return
	}
	scoreCalcMap[params.xid] = params

	_, embed := getScorePlaygroundCalculations(params)

	components := getScorePlaygroundComponents(params)
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
