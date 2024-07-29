package boost

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"github.com/olekukonko/tablewriter"
	"google.golang.org/protobuf/proto"
)

// GetSlashTeamworkEval will return the discord command for calculating token values of a running contract
func GetSlashTeamworkEval(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Evaluate teamwork values a contract",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "egginc-ign",
				Description: "Egg Inc, in game name to evaluate.",
				Required:    false,
			},
		},
	}
}

// HandleTeamworkEvalCommand will handle the /teamwork-eval command
func HandleTeamworkEvalCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var builder strings.Builder

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

	var eggign string
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["egginc-ign"]; ok {
		eggign = opt.StringValue()
	} else {
		name := farmerstate.GetMiscSettingString(userID, "EggIncRawName")
		if name != "" {
			eggign = name
		}

	}

	contract := FindContract(i.ChannelID)
	if contract == nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: "No contract found in this channel.",
			})

		return
	}
	/*
		if slices.Index(contract.Order, userID) == -1 {
			s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: "User isn't in this contract.",
				})

			return
		}
	*/
	if config.EIUserID != "" {
		builder.WriteString(DownloadCoopStatus(userID, eggign, contract))
	} else {
		builder.WriteString("This command is missing a configuration option necessary to function.")
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: builder.String(),
		})
}

// DownloadCoopStatus will download the coop status for a given contract and coop ID
func DownloadCoopStatus(userID string, einame string, contract *Contract) string {
	eggIncID := config.EIUserID
	reqURL := "https://www.auxbrain.com/ei/coop_status"
	enc := base64.StdEncoding

	var protoData string
	var dataTimestampStr string

	eiContract := ei.EggIncContractsAll[contract.ContractID]
	if eiContract.ID == "" {
		return "Invalid contract ID."
	}

	cacheID := contract.ContractID + ":" + contract.CoopID
	cachedData := eiDatas[cacheID]
	var nowTime time.Time

	// Check if the file exists
	if cachedData != nil && time.Now().Before(cachedData.expirationTimestamp) {
		protoData = cachedData.protoData
		dataTimestampStr = fmt.Sprintf("\nUsing cached data retrieved <t:%d:R>, refresh <t:%d:R>", cachedData.timestamp.Unix(), cachedData.expirationTimestamp.Unix())
		nowTime = cachedData.timestamp
		// Use protoData as needed
	} else {
		coopID := strings.ToLower(contract.CoopID)
		contractID := strings.ToLower(contract.ContractID)

		coopStatusRequest := ei.ContractCoopStatusRequest{
			ContractIdentifier: &contractID,
			CoopIdentifier:     &coopID,
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
		dataTimestampStr = ""
		protoData = string(body)
		data := eiData{ID: cacheID, timestamp: time.Now(), expirationTimestamp: time.Now().Add(2 * time.Minute), contractID: contract.ContractID, coopID: contract.CoopID, protoData: protoData}
		eiDatas[cacheID] = &data
		nowTime = time.Now()
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
	var calcSecondsRemaining int64

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
	var builder strings.Builder

	startTime := nowTime
	secondsRemaining := int64(decodeCoopStatus.GetSecondsRemaining())
	endTime := nowTime
	if decodeCoopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.LengthInSeconds) * time.Second)
		secondsSinceAllGoals := int64(decodeCoopStatus.GetSecondsSinceAllGoalsAchieved())
		endTime = endTime.Add(-time.Duration(secondsSinceAllGoals) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
		builder.WriteString(fmt.Sprintf("Completed contract **%s/%s**\n", contract.ContractID, contract.CoopID))
		builder.WriteString(fmt.Sprintf("Start Time: <t:%d:f>\n", startTime.Unix()))
		builder.WriteString(fmt.Sprintf("End Time: <t:%d:f>\n", endTime.Unix()))
		builder.WriteString(fmt.Sprintf("Duration: %v\n", (endTime.Sub(startTime)).Round(time.Second)))

	} else {
		var totalContributions float64
		var contributionRatePerSecond float64
		// Need to figure out the
		for _, c := range decodeCoopStatus.GetContributors() {
			totalContributions += c.GetContributionAmount()
			totalContributions += -(c.GetContributionRate() * c.GetFarmInfo().GetTimestamp()) // offline eggs
			contributionRatePerSecond += c.GetContributionRate()
		}
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.LengthInSeconds) * time.Second)
		totalReq := contract.TargetAmount[len(contract.TargetAmount)-1]
		calcSecondsRemaining = int64((totalReq - totalContributions) / contributionRatePerSecond)
		endTime = nowTime.Add(time.Duration(calcSecondsRemaining) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
		builder.WriteString(fmt.Sprintf("In Progress **%s/%s** on target to complete <t:%d:R>\n", contract.ContractID, contract.CoopID, endTime.Unix()))
		builder.WriteString(fmt.Sprintf("Start Time: <t:%d:f>\n", startTime.Unix()))
		builder.WriteString(fmt.Sprintf("Est. End Time: <t:%d:f>\n", endTime.Unix()))
		builder.WriteString(fmt.Sprintf("Est. Duration: %v\n", (endTime.Sub(startTime)).Round(time.Second)))
	}
	builder.WriteString(fmt.Sprintf("Evaluating data for **%s**\n", einame))

	// Take care of other parameter calculations here

	// Chicken Runs
	/*
		contractDurationInDays := int(contractDurationSeconds / 86400.0)
		fCR := max(12.0/float64(contract.CoopSize*contractDurationInDays), 0.3)
		CR := min(fCR, 6.0)

		// Token Values
		BTA := contractDurationSeconds / float64(contract.MinutesPerToken)
		T := 0.0
		if BTA <= 42.0 {
			T = (2.0 / 3.0 * (3.0)) + ((8.0 / 3.0) * min(tval, 3.0))
		} else {
			T = (200.0/(7.0*BTA))*(0.07*BTA) + (800.0 / (7.0 * BTA) * min(tval, 0.07*BTA))
		}
	*/

	if einame == "" {
		einame = contract.Boosters[userID].Nick
	}

	found := false
	var teamworkNames []string

	for _, c := range decodeCoopStatus.GetContributors() {

		name := c.GetUserName()

		if einame != name {
			teamworkNames = append(teamworkNames, name)
			continue
		}
		found = true

		for _, a := range c.GetBuffHistory() {
			earnings := int(math.Round(a.GetEarnings()*100 - 100))
			eggRate := int(math.Round(a.GetEggLayingRate()*100 - 100))
			serverTimestamp := int64(a.GetServerTimestamp()) // When it was equipped
			if decodeCoopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
				serverTimestamp -= int64(decodeCoopStatus.GetSecondsSinceAllGoalsAchieved())
			} else {
				serverTimestamp += calcSecondsRemaining
			}
			serverTimestamp = int64(contractDurationSeconds) - serverTimestamp
			BuffTimeValues = append(BuffTimeValues, BuffTimeValue{name, earnings, 0.0075 * float64(earnings), eggRate, 0.0075 * float64(eggRate) * 10.0, serverTimestamp, 0, 0, 0, 0})
		}
	}

	remainingTime := contractDurationSeconds
	for i, b := range BuffTimeValues {
		if i == len(BuffTimeValues)-1 {
			BuffTimeValues[i].durationEquiped = int64(contractDurationSeconds) - b.timeEquiped
		} else {
			BuffTimeValues[i].durationEquiped = BuffTimeValues[i+1].timeEquiped - b.timeEquiped
		}
		remainingTime -= float64(BuffTimeValues[i].durationEquiped)
	}

	if len(BuffTimeValues) == 0 {
		builder.WriteString("**No buffs found for this contract.**\n")
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
			when := time.Duration(b.timeEquiped) * time.Second

			table.Append([]string{fmt.Sprintf("%v", when.Round(time.Second)), fmt.Sprintf("%v", dur.Round(time.Second)), fmt.Sprintf("%d%%", b.eggRate), fmt.Sprintf("%d%%", b.earnings), fmt.Sprintf("%8.2f", float64(b.durationEquiped)*b.eggRateCalc), fmt.Sprintf("%8.2f", float64(b.durationEquiped)*b.earningsCalc), fmt.Sprintf("%8.2f", b.buffTimeValue), fmt.Sprintf("%1.8f", teamworkScore)})

			buffTimeValue += b.buffTimeValue
		}

		//completionTime :=

		B := min(buffTimeValue/contractDurationSeconds, 2)
		CR := min(0.0, 6.0)
		T := 0.0

		TeamworkScore := ((5.0 * B) + (CR * 0) + (T * 0)) / 19.0
		table.SetFooter([]string{"", "", "", "", "", "", fmt.Sprintf("%8.2f", buffTimeValue), fmt.Sprintf("%1.8f", TeamworkScore)})
		log.Printf("Teamwork Score: %f\n", TeamworkScore)

		builder.WriteString("```")
		table.Render()
		builder.WriteString("```")

		// Calculate the ChickenRun score
		tableCR := tablewriter.NewWriter(&builder)
		tableCR.ClearRows()

		tableCR.SetHeader([]string{"1 Run", "N*Runs", fmt.Sprintf("%d Runs", eiContract.ChickenRuns)})
		tableCR.SetBorder(false)
		tableCR.SetAlignment(tablewriter.ALIGN_RIGHT)

		fCR := max(12.0/float64(eiContract.MaxCoopSize*eiContract.ContractDurationInDays), 0.3)
		//for i := range eiContract.ChickenRuns {
		CR = min(fCR*float64(1), 6.0)
		CRTeamworkScore := ((5.0 * B * 0) + CR + (T * 0)) / 19.0
		CRMax := min(fCR*float64(eiContract.ChickenRuns), 6.0)
		CRMaxTeamworkScore := ((5.0 * B * 0) + CRMax + (T * 0)) / 19.0
		tableCR.Append([]string{
			fmt.Sprintf("%1.6f", CRTeamworkScore),
			fmt.Sprintf("N * %1.6f", CRTeamworkScore),
			fmt.Sprintf("%1.6f", CRMaxTeamworkScore)})
		//}
		builder.WriteString("```")
		tableCR.Render()
		builder.WriteString("```")

		if builder.Len() < 1500 {

			tableT := tablewriter.NewWriter(&builder)
			tableT.ClearRows()

			BTA := (contractDurationSeconds / 60) / float64(eiContract.MinutesPerToken)

			tableT.SetHeader([]string{"△ TVAL", "sent 0 tval", "2.5 tval", "3 tval"})
			tableT.SetBorder(false)
			tableT.SetAlignment(tablewriter.ALIGN_RIGHT)

			for tval := -3.0; tval <= 3.0; tval += 1.0 {
				var items []string
				items = append(items, fmt.Sprintf("%1.1f", tval))
				for tSent := 0.0; tSent <= 3.0; tSent += 1.5 {
					if BTA <= 42.0 {
						T = (2.0 / 3.0 * min(tSent, 3.0)) + ((8.0 / 3.0) * min(tval, 3.0))
					} else {
						T = (200.0/(7.0*BTA))*min(tSent, 0.07*BTA) + (800.0 / (7.0 * BTA) * min(tval, 0.07*BTA))
					}
					TTeamworkScore := ((5.0 * B * 0) + (CR * 0) + T) / 19.0
					if tSent < tval {
						items = append(items, "")
					} else {
						items = append(items, fmt.Sprintf("%1.4f", TTeamworkScore))
					}
					//		tableT.Append([]string{fmt.Sprintf("%1.1f", tval), fmt.Sprintf("%1.6f", tSent), fmt.Sprintf("%1.6f", TTeamworkScore)})
				}
				tableT.Append(items)
			}
			//builder.Reset()
			builder.WriteString("```")
			tableT.Render()
			builder.WriteString("```")
		}

		log.Printf("\n%s", builder.String())

		log.Print("Buff Time Value: ", buffTimeValue)
	}

	if !found {
		// Write to builder a message about using /seteiname to associate your discorn name with your Eggs IGN
		// create string of teamworkNames
		teamworkNamesStr := strings.Join(teamworkNames, ", ")
		builder.WriteString("\n")
		builder.WriteString("Your discord name must be different from your EggInc IGN.\n")
		builder.WriteString("Use **/seteggincname** to make this association.\n\n")
		builder.WriteString(fmt.Sprintf("Farmers in this contract are:\n> %s", teamworkNamesStr))
	}

	if dataTimestampStr != "" {
		builder.WriteString(dataTimestampStr)
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
