package boost

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/olekukonko/tablewriter"
	"github.com/xhit/go-str2duration/v2"
	"google.golang.org/protobuf/proto"
)

func fmtDuration(d time.Duration) string {
	str := ""
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d = h / 24
	h -= d * 24

	if d > 0 {
		str = fmt.Sprintf("%dd%dh%dm", d, h, m)
	} else {
		str = fmt.Sprintf("%dh%dm", h, m)
	}
	return strings.Replace(str, "0h0m", "", -1)
}

// GetSlashTeamworkEval will return the discord command for calculating token values of a running contract
func GetSlashTeamworkEval(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Calculate token values of current running contract",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "duration",
				Description: "Total duration of this contract. Example: 19h35m. Ignored if the contract is completed.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "runs",
				Description: "Total number of chicken runs. Default is 0.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "tval",
				Description: "Total token time value delta. default is 0.",
				Required:    false,
			},
		},
	}
}

// HandleTeamworkEvalCommand will handle the /teamwork-eval command
func HandleTeamworkEvalCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var builder strings.Builder

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	isAdmin := false
	for _, el := range AdminUsers[:3] {
		if el == userID {
			isAdmin = true
			break
		}
	}

	if !isAdmin {
		s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: "This feature is currently under test.",
			})
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var duration time.Duration
	var runs int
	var tval float64

	if opt, ok := optionMap["duration"]; ok {
		var err error
		// Timespan of the contract duration
		contractTimespan := strings.TrimSpace(opt.StringValue())
		contractTimespan = strings.Replace(contractTimespan, "day", "d", -1)
		contractTimespan = strings.Replace(contractTimespan, "hr", "h", -1)
		contractTimespan = strings.Replace(contractTimespan, "min", "m", -1)
		contractTimespan = strings.Replace(contractTimespan, "sec", "s", -1)
		// replace all spaces with nothing
		contractTimespan = strings.Replace(contractTimespan, " ", "", -1)
		duration, err = str2duration.ParseDuration(contractTimespan)
		if err != nil {
			// Invalid duration, just assigning a 12h
			duration = 12 * time.Hour
			//invalidDuration = true
		}
	}
	if opt, ok := optionMap["runs"]; ok {
		runs = int(opt.IntValue())
	}

	if opt, ok := optionMap["tval"]; ok {
		tval = float64(opt.FloatValue())
	}

	contract := FindContract(i.ChannelID)
	if contract == nil {
		s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: "No contract found in this channel.",
			})

		return
	}
	if slices.Index(contract.Order, userID) == -1 {
		s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: "User isn't in this contract.",
			})

		return
	}

	builder.WriteString(DownloadCoopStatus(userID, contract, duration, runs, tval))

	s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: builder.String(),
		})
}

// DownloadCoopStatus will download the coop status for a given contract and coop ID
func DownloadCoopStatus(userID string, contract *Contract, duration time.Duration, runs int, tval float64) string {
	eggIncID := config.EIUserID
	reqURL := "https://www.auxbrain.com/ei/coop_status"
	enc := base64.StdEncoding

	var protoData string

	var filename string
	filename = contract.ContractID + "-" + contract.CoopID + ".bin"
	filename = strings.ReplaceAll(filename, " ", "")

	var eiContract *EggIncContract
	for _, c := range EggIncContracts {
		if c.ID == contract.ContractID {
			log.Print("Contract Found: ", c.ID)
			eiContract = &c
			break
		}
	}

	if eiContract == nil {
		return "No contract found in the EI data"
	}

	// Check if the file exists
	if _, err := os.Stat(filename); err == nil {
		// Read the file into a string
		file, err := os.ReadFile(filename)
		if err != nil {
			log.Print(err)
			return err.Error()
		}
		protoData = string(file)
		// Use protoData as needed
	} else {
		coopStatusRequest := ei.ContractCoopStatusRequest{
			ContractIdentifier: &contract.ContractID,
			CoopIdentifier:     &contract.CoopID,
			UserId:             &eggIncID,
		}
		reqBin, err := proto.Marshal(&coopStatusRequest)
		if err != nil {
			return err.Error()
		}
		reqDataEncoded := enc.EncodeToString(reqBin)

		response, err := http.PostForm(reqURL, url.Values{"data": {reqDataEncoded}})

		if err != nil {
			log.Print(err)
			return err.Error()
		}

		defer response.Body.Close()

		// Read the response body
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Print(err)
			return err.Error()
		}
		/*
			file, err := os.Create(filename)
			if err != nil {
				log.Print(err)
				return err.Error()
			}
			defer file.Close()

			_, err = file.Write(body)
			if err != nil {
				log.Print(err)
				return err.Error()
			}
		*/
		protoData = string(body)
	}

	decodedAuthBuf := &ei.AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(protoData)
	err := proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		return err.Error()
	}

	decodeCoopStatus := &ei.ContractCoopStatusResponse{}
	err = proto.Unmarshal(decodedAuthBuf.Message, decodeCoopStatus)
	if err != nil {
		log.Print(err)
		return err.Error()
	}

	type BuffTimeValue struct {
		name            string
		earnings        int
		earningsCalc    float64
		eggRate         int
		eggRateCalc     float64
		timeEquiped     int64
		durationEquiped int64
		buffTimeValue   float64
		tb              int64
		totalValue      float64
	}

	var BuffTimeValues []BuffTimeValue
	var contractDurationSeconds float64
	startTime := time.Now()
	secondsRemaining := int64(decodeCoopStatus.GetSecondsRemaining())
	startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
	startTime = startTime.Add(-time.Duration(eiContract.LengthInSeconds) * time.Second)
	endTime := time.Now()
	if decodeCoopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
		secondsSinceAllGoals := int64(decodeCoopStatus.GetSecondsSinceAllGoalsAchieved())
		endTime = endTime.Add(-time.Duration(secondsSinceAllGoals) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
	} else {
		contractDurationSeconds = duration.Seconds()
		//endTime = startTime.Add(duration)
	}

	log.Print("Contract Duration: ", contractDurationSeconds)

	//serverTimestampUnix := time.Now().Unix()
	contractUserName := contract.Boosters[userID].Nick

	for _, c := range decodeCoopStatus.GetContributors() {

		// Force a few things for testing so they can match up
		farmerstate.SetMiscSettingString("238786501700222986", "EggIncRawName", "\ue10c")     // RAIYC
		farmerstate.SetMiscSettingString("184063956539670528", "EggIncRawName", "Halceyx")    // Hal
		farmerstate.SetMiscSettingString("393477262412087319", "EggIncRawName", "iDaHotBone") // Tbone

		name := c.GetUserName()
		einame := farmerstate.GetMiscSettingString(userID, "EggIncRawName")
		if einame == "" {
			einame = contractUserName
		}

		if einame != name {
			continue
		}

		//cAmt := c.GetContributionAmount()
		//cRate := c.GetContributionRate()

		for _, a := range c.GetBuffHistory() {
			earnings := int(math.Round(a.GetEarnings()*100 - 100))
			eggRate := int(math.Round(a.GetEggLayingRate()*100 - 100))
			serverTimestamp := int64(a.GetServerTimestamp()) // When it was equipped
			if decodeCoopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
				serverTimestamp = int64(a.GetServerTimestamp()) - int64(decodeCoopStatus.GetSecondsSinceAllGoalsAchieved())
			}
			serverTimestamp = int64(contractDurationSeconds) - serverTimestamp
			BuffTimeValues = append(BuffTimeValues, BuffTimeValue{name, earnings, 0.0075 * float64(earnings), eggRate, 0.0075 * float64(eggRate) * 10.0, serverTimestamp, 0, 0, 0, 0})
		}
	}

	//prevServerTimestamp = int64(decodeCoopStatus.GetSecondsRemaining()) + BuffTimeValues[0].timeEquiped
	// If the coop completed, the secondsSinceAllGoalsAchieved (towards the end) is present.
	// If coop isn't complete, you have to back calculate from secondsRemaining,
	// and estimate completion time based off rates

	/*
		Start time can be found via:
		Date.now() + secondsRemaining - contract.gradeSpecs[(grade)].lengthSeconds
		End time can be found via:
		Date.now() - secondsSinceAllGoalsAchieved
		Then use day.js to generate timespan and then create time string
	*/
	for i, b := range BuffTimeValues {
		if i == len(BuffTimeValues)-1 {
			BuffTimeValues[i].durationEquiped = int64(contractDurationSeconds) - b.timeEquiped
		} else {
			BuffTimeValues[i].durationEquiped = BuffTimeValues[i+1].timeEquiped - b.timeEquiped
		}
	}
	var builder strings.Builder

	if len(BuffTimeValues) == 0 {
		builder.WriteString("No buffs found for this contract.")
	} else {

		table := tablewriter.NewWriter(&builder)
		table.SetHeader([]string{"Time", "Duration", "Defl", "SIAB", "BTV-Defl", "BTV-SIAB ", "Buff Val", "TeamWork"})
		table.SetBorder(false)
		//table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_RIGHT)
		//table.SetCenterSeparator("")
		//table.SetColumnSeparator("")
		//table.SetRowSeparator("")
		//table.SetHeaderLine(false)
		//table.EnableBorder(false)
		//table.SetTablePadding(" ") // pad with tabs
		//table.SetNoWhiteSpace(true)

		var buffTimeValue float64
		for _, b := range BuffTimeValues {
			if b.durationEquiped < 0 {
				b.durationEquiped = 0
			}

			b.buffTimeValue = float64(b.durationEquiped)*b.earningsCalc + float64(b.durationEquiped)*b.eggRateCalc
			B := min(b.buffTimeValue/contractDurationSeconds, 2)
			CR := min(0.0, 6.0)
			T := 0.0
			teamworkScore := ((5.0 * B) + CR + T) / 19.0

			dur := time.Duration(b.durationEquiped) * time.Second

			table.Append([]string{fmt.Sprintf("%d", b.timeEquiped), fmt.Sprintf("%v", dur.Round(time.Second)), fmt.Sprintf("%d%%", b.eggRate), fmt.Sprintf("%d%%", b.earnings), fmt.Sprintf("%8.2f", float64(b.durationEquiped)*b.eggRateCalc), fmt.Sprintf("%8.2f", float64(b.durationEquiped)*b.earningsCalc), fmt.Sprintf("%8.2f", b.buffTimeValue), fmt.Sprintf("%1.8f", teamworkScore)})

			buffTimeValue += b.buffTimeValue
		}

		//completionTime :=

		B := min(buffTimeValue/contractDurationSeconds, 2)

		CR := min(0.0, 6.0)
		T := 0.0

		TeamworkScore := ((5.0 * B) + CR + T) / 19.0
		table.SetFooter([]string{"", "", "", "", "", "", fmt.Sprintf("%8.2f", buffTimeValue), fmt.Sprintf("%1.8f", TeamworkScore)})
		log.Printf("Teamwork Score: %f\n", TeamworkScore)

		builder.WriteString("```")
		table.Render()
		builder.WriteString("```")

		log.Printf("\n%s", builder.String())
		log.Print("Buff Time Value: ", buffTimeValue)
	}

	return builder.String()
}

/*
// GetEggIncEvents will download the events from the Egg Inc API
func GetEggIncEvents() {
	userID := "EI6374748324102144"
	//userID := "EI5086937666289664"
	reqURL := "https://www.auxbrain.com/ei/get_periodicals"
	enc := base64.StdEncoding
	clientVersion := uint32(99)

	periodicalsRequest := ei.GetPeriodicalsRequest{
		UserId:               &userID,
		CurrentClientVersion: &clientVersion,
	}
	reqBin, err := proto.Marshal(&periodicalsRequest)
	if err != nil {
		log.Print(err)
		return
	}
	reqDataEncoded := enc.EncodeToString(reqBin)
	response, err := http.PostForm(reqURL, url.Values{"data": {reqDataEncoded}})

	if err != nil {
		log.Print(err)
		return
	}

	defer response.Body.Close()

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Print(err)
		return
	}

	protoData := string(body)

	decodedAuthBuf := &ei.AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(protoData)
	err = proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		return
	}

	periodicalsResponse := &ei.PeriodicalsResponse{}
	err = proto.Unmarshal(decodedAuthBuf.Message, periodicalsResponse)
	if err != nil {
		log.Print(err)
		return
	}

	for _, event := range periodicalsResponse.GetEvents().GetEvents() {
		log.Print("event details: ")
		log.Printf("  type: %s", event.GetType())
		log.Printf("  text: %s", event.GetSubtitle())
		log.Printf("  multiplier: %f", event.GetMultiplier())

		startTimestamp := int64(math.Round(event.GetStartTime()))
		startTime := time.Unix(startTimestamp, 0)
		endTime := startTime.Add(time.Duration(event.GetDuration()) * time.Second)
		log.Printf("  start time: %s", startTime)
		log.Printf("  end time: %s", endTime)

		log.Printf("ultra: %t", event.GetCcOnly())
	}

}
*/
