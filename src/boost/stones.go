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

// GetSlashStones will return the discord command for calculating ideal stone set
func GetSlashStones(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Optimal stone set for running contract. Optional params for non-BoostBot coop use.",
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
				Description:  "Select a contract-id",
				Required:     false,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "coop-id",
				Description: "Your coop-id",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "details",
				Description: "Show full details. Default is false. (sticky)",
				Required:    false,
			},
		},
	}
}

// HandleStonesCommand will handle the /stones command
func HandleStonesCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var builder strings.Builder

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			//Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	var contractID string
	var coopID string
	details := false

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["contract-id"]; ok {
		contractID = strings.ToLower(opt.StringValue())
		contractID = strings.Replace(contractID, " ", "", -1)
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = strings.ToLower(opt.StringValue())
		coopID = strings.Replace(coopID, " ", "", -1)
	}
	userID := getInteractionUserID(i)
	if opt, ok := optionMap["details"]; ok {
		details = opt.BoolValue()
		farmerstate.SetMiscSettingFlag(userID, "stone-details", details)
	} else {
		details = farmerstate.GetMiscSettingFlag(userID, "stone-details")
	}

	// Unser contractID and coopID means we want the Boost Bot contract
	if contractID == "" || coopID == "" {
		contract := FindContract(i.ChannelID)
		if contract == nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: "No contract found in this channel. Please provide a contract-id and coop-id.",
				})

			return
		}
		contractID = strings.ToLower(contract.ContractID)
		coopID = strings.ToLower(contract.CoopID)
	}

	builder.WriteString(DownloadCoopStatusStones(contractID, coopID, details))

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: builder.String(),
		})
}

// DownloadCoopStatusStones will download the coop status for a given contract and coop ID
func DownloadCoopStatusStones(contractID string, coopID string, details bool) string {
	eggIncID := config.EIUserID
	reqURL := "https://www.auxbrain.com/ei/coop_status"
	enc := base64.StdEncoding

	var builder strings.Builder
	var dataTimestampStr string
	var protoData string

	cacheID := contractID + ":" + coopID
	cachedData := eiDatas[cacheID]

	// Check if the file exists
	if cachedData != nil && time.Now().Before(cachedData.expirationTimestamp) {
		protoData = cachedData.protoData
		dataTimestampStr = fmt.Sprintf("\nUsing cached data retrieved <t:%d:R>, refresh <t:%d:R>", cachedData.timestamp.Unix(), cachedData.expirationTimestamp.Unix())
	} else {

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
		//dataTimestampStr = ""
		protoData = string(body)
		data := eiData{ID: cacheID, timestamp: time.Now(), expirationTimestamp: time.Now().Add(1 * time.Minute), contractID: contractID, coopID: coopID, protoData: protoData}
		eiDatas[cacheID] = &data
		//nowTime = time.Now()
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

	levels := []string{"T1", "T2", "T3", "T4", "T5"}
	rarity := []string{"C", "R", "E", "L"}
	deflector := map[string]float64{
		"T1C": 5.0,
		"T2C": 8.0,
		"T3C": 12.0, "T3R": 13.0,
		"T4C": 15.0, "T4R": 17.0, "T4E": 19.0, "T4L": 20.0,
	}
	metronome := map[string]float64{
		"T1C": 5.0,
		"T2C": 10.0, "T2R": 12.0,
		"T3C": 15.0, "T3R": 17.0, "T3E": 20.0,
		"T4C": 25.0, "T4R": 27.0, "T4E": 30.0, "T4L": 35.0,
	}
	compass := map[string]float64{
		"T1C": 5.0,
		"T2C": 10.0,
		"T3C": 20.0, "T3R": 22.0,
		"T4C": 30.0, "T4R": 35.0, "T4E": 40.0, "T4L": 50.0,
	}
	gussett := map[string]float64{
		"T1C": 5.0,
		"T2C": 10.0, "T2R": 12.0,
		"T3C": 19.0, "T3R": 16.0,
		"T4C": 20.0, "T4E": 22.0, "T4L": 25.0,
	}

	type artifact struct {
		name    string
		abbrev  string
		percent float64
	}

	type artifactSet struct {
		name             string
		baseLayingRate   float64
		baseShippingRate float64
		stones           int
		deflector        artifact
		metronome        artifact
		compass          artifact
		gusset           artifact

		tachStones  int
		quantStones int

		elr            float64
		ihr            float64
		sr             float64
		farmCapacity   float64
		farmPopulation float64

		tachWant  int
		quantWant int
		bestELR   float64
		bestSR    float64
		collegg   []string
	}
	var artifactSets []artifactSet

	//baseLaying := 3.772
	//baseShipping := 7.148

	everyoneDeflectorPercent := 0.0
	for _, c := range decodeCoopStatus.GetContributors() {
		as := artifactSet{}
		as.name = c.GetUserName()

		p := c.GetProductionParams()
		as.farmCapacity = p.GetFarmCapacity()
		as.farmPopulation = p.GetFarmPopulation()
		as.elr = p.GetElr()
		as.ihr = p.GetIhr()
		as.sr = p.GetSr() // This is per second, convert to hour
		as.sr *= 3600.0
		//log.Print(fCapacity, elr, ihr, sr)

		//totalStones := 0
		as.deflector.percent = 0.0
		as.compass.percent = 0.0
		as.metronome.percent = 0.0
		as.metronome.percent = 0.0
		//fmt.Printf("Farm: %s\n", as.name)

		fi := c.GetFarmInfo()

		userLayRate := 1 / 30.0 // 1 chicken per 30 seconds
		userShippingCap := 500000000.0
		for _, cr := range fi.GetCommonResearch() {
			switch cr.GetId() {
			case "comfy_nests":
				userLayRate *= (1 + 0.1*float64(cr.GetLevel())) // Comfortable Nests 10%
			case "hen_house_ac":
				userLayRate *= (1 + 0.05*float64(cr.GetLevel())) // Hen House Expansion 10%
			case "improved_genetics":
				userLayRate *= (1 + 0.15*float64(cr.GetLevel())) // Internal Hatcheries 15%
			case "time_compress":
				userLayRate *= (1 + 0.1*float64(cr.GetLevel())) // Time Compression 10%
			case "timeline_diversion":
				userLayRate *= (1 + 0.02*float64(cr.GetLevel())) // Timeline Diversion 2%
			case "relativity_optimization":
				userLayRate *= (1 + 0.1*float64(cr.GetLevel())) // Relativity Optimization 10%
			case "leafsprings":
				userShippingCap *= (1 + 0.05*float64(cr.GetLevel())) // Leafsprings 5%
			case "lightweight_boxes":
				userShippingCap *= (1 + 0.1*float64(cr.GetLevel())) // Lightweight Boxes 10%
			case "driver_training":
				userShippingCap *= (1 + 0.05*float64(cr.GetLevel())) // Driver Training 5%
			case "super_alloy":
				userShippingCap *= (1 + 0.05*float64(cr.GetLevel())) // Super Alloy 5%
			case "quantum_storage":
				userShippingCap *= (1 + 0.05*float64(cr.GetLevel())) // Quantum Storage 5%
			case "hover_upgrades":
				userShippingCap *= (1 + 0.05*float64(cr.GetLevel())) // Hover Upgrades 5%
			case "dark_containment":
				userShippingCap *= (1 + 0.05*float64(cr.GetLevel())) // Dark Containment 5%
			case "neural_net_refine":
				userShippingCap *= (1 + 0.05*float64(cr.GetLevel())) // Neural Net Refine 5%
			case "hyper_portalling":
				userShippingCap *= (1 + 0.05*float64(cr.GetLevel())) // Hyper Portalling 5%
			}
		}

		for _, er := range fi.GetEpicResearch() {
			switch er.GetId() {
			case "epic_egg_laying":
				userLayRate *= (1 + 0.05*float64(er.GetLevel())) // Epic Egg Laying 5%
			case "transportation_lobbyist":
				userShippingCap *= (1 + 0.05*float64(er.GetLevel())) // Transportation Lobbyist 5%
			}
		}

		//userLayRate *= 3600 // convert to hr rate
		as.baseLayingRate = userLayRate * 11340000000.0 * 3600.0 / 1e15
		as.baseShippingRate = userShippingCap * 10.0 * float64(len(fi.GetTrainLength())) * 60 / 1e16

		for _, a := range fi.GetEquippedArtifacts() {
			spec := a.GetSpec()
			strType := levels[spec.GetLevel()] + rarity[spec.GetRarity()]

			//var name string
			numStones := len(a.GetStones())
			switch spec.GetName() {
			case ei.ArtifactSpec_TACHYON_DEFLECTOR:
				as.deflector.percent = deflector[strType]
				as.deflector.name = fmt.Sprintf("%s %s %2.0f%% %d slots", "Deflector", strType, as.deflector.percent, numStones)
				as.deflector.abbrev = strType
				everyoneDeflectorPercent += as.deflector.percent
			case ei.ArtifactSpec_QUANTUM_METRONOME:
				as.metronome.percent = metronome[strType]
				as.metronome.name = fmt.Sprintf("%s %s %2.0f%% %d slots", "Metronome", strType, as.metronome.percent, numStones)
				as.metronome.abbrev = strType
			case ei.ArtifactSpec_INTERSTELLAR_COMPASS:
				as.compass.percent = compass[strType]
				as.metronome.name = fmt.Sprintf("%s %s %2.0f%% %d slots", "Compass", strType, as.compass.percent, numStones)
				as.compass.abbrev = strType
			case ei.ArtifactSpec_ORNATE_GUSSET:
				as.gusset.percent = gussett[strType]
				as.metronome.name = fmt.Sprintf("%s %s %2.0f%% %d slots", "Gusset", strType, as.metronome.percent, numStones)
				as.gusset.abbrev = strType
			default:
				//name = fmt.Sprintf("%s %s %2.0f%% %d slots", "Other", strType, 0.0, numStones)
			}

			for _, st := range a.GetStones() {
				if st.GetName() == ei.ArtifactSpec_TACHYON_STONE {
					as.tachStones += 1.0
				}
				if st.GetName() == ei.ArtifactSpec_QUANTUM_STONE {
					as.quantStones += 1.0
				}
			}

			as.stones += len(a.GetStones())
			/*
				for i := 0; i < as.stones; i++ {
					stoneLayRate := layingRate * math.Pow(1.05, float64(i))
					stoneLayRate = stoneLayRate * (1 + as.deflector.percent/100.0)
					stoneShipRate := shippingRate * math.Pow(1.05, float64((as.stones-i)))
					fmt.Printf("Stone %d: %2.3f %2.3f\n", i, stoneLayRate, stoneShipRate)
				}*/
			//totalStones += as.stones
			//fmt.Println(name)
		}
		//fmt.Printf("Total Stones: %d\n", totalStones)
		artifactSets = append(artifactSets, as)

		//fmt.Printf("%s  %2.0f\n", decodeCoopStatus.GetCoopIdentifier(), everyoneDeflectorPercent)
	}

	table := tablewriter.NewWriter(&builder)
	if details {
		table.SetHeader([]string{"Name", "Def", "Met", "Com", "Gus", "Tach", "Quant", "ELR", "SR", "Delivery", "Collegg", "Notes"})
	} else {
		table.SetHeader([]string{"Name", "Tach", "Quant", "Notes"})
	}
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)
	table.SetAlignment(tablewriter.ALIGN_RIGHT)

	// 1e15
	for _, as := range artifactSets {

		//fmt.Printf("name:\"%s\"  Stones:%d  elr:%f egg/chicken/s  sr:%f egg/s\n", as.name, as.stones, as.elr, as.sr)
		layingRate := (as.baseLayingRate) * (1 + as.metronome.percent/100.0) * (1 + as.gusset.percent/100.0)
		shippingRate := (as.baseShippingRate) * (1 + as.compass.percent/100.0)

		// Determine Colleggtible Increase
		stoneLayRateNow := layingRate * (1 + (everyoneDeflectorPercent-as.deflector.percent)/100.0)
		stoneLayRateNow = stoneLayRateNow * math.Pow(1.05, float64(as.tachStones))
		chickELR := as.elr * as.farmCapacity * 3600.0 / 1e15
		collegELR := math.Round(chickELR/stoneLayRateNow*100.0) / 100.0
		//fmt.Printf("Calc ELR: %2.3f  Param.Elr: %2.3f   Diff:%2.2f\n", stoneLayRateNow, chickELR, (chickELR / stoneLayRateNow))
		if collegELR > 1.000 {
			//fmt.Printf("Colleggtible Egg Laying Rate Factored in with %2.2f%%\n", collegELR)
			as.collegg = append(as.collegg, fmt.Sprintf("ELR:%2.0f%%", (collegELR-1.0)*100.0))
			//farmerstate.SetMiscSettingString(as.name, "coll-elr", fmt.Sprintf("%2.0f%%", (collegELR-1.0)*100.0))
		} /*else {
			hasColl := farmerstate.GetMiscSettingString(as.name, "coll-elr")
			if hasColl != "" {
				as.collegg = append(as.collegg, fmt.Sprintf("(ELR:%s)", hasColl))
				collegELR *= 1.05

			}
		}*/

		stoneShipRateNow := shippingRate * math.Pow(1.05, float64((as.quantStones)))
		//fmt.Printf("Calc SR: %2.3f  param.Sr: %2.3f   Diff:%2.2f\n", stoneShipRateNow, as.sr/1e15, (as.sr/1e15)/stoneShipRateNow)
		collegShip := math.Round((as.sr/1e15)/stoneShipRateNow*100.0) / 100.0
		if collegShip > 1.000 {
			//fmt.Printf("Colleggtible Shipping Rate Factored in with %2.2f%%\n", collegShip)
			as.collegg = append(as.collegg, fmt.Sprintf("SR:%2.0f%%", (collegShip-1.0)*100.0))
			//farmerstate.SetMiscSettingString(as.name, "coll-SR", fmt.Sprintf("%2.0f%%", (collegShip-1.0)*100.0))
		} /* else {
			hasColl := farmerstate.GetMiscSettingString(as.name, "coll-SR")
			if hasColl != "" {
				as.collegg = append(as.collegg, fmt.Sprintf("(SR:%s)", hasColl))
				collegShip = collegShip * 1.05
			}
		}*/
		bestTotal := 0.0
		//bestString := ""

		for i := 0; i < as.stones; i++ {
			stoneLayRate := layingRate * (1 + (everyoneDeflectorPercent-as.deflector.percent)/100.0)
			stoneLayRate = stoneLayRate * math.Pow(1.05, float64(i)) * collegELR

			//stoneShipRate := shippingRate * math.Pow(1.05, float64((as.stones-i)))
			stoneShipRate := shippingRate * math.Pow(1.05, float64((as.stones-i))) * collegShip
			bestMin := min(stoneLayRate, stoneShipRate)
			if bestMin > bestTotal {
				bestTotal = bestMin
				as.tachWant = i
				as.quantWant = as.stones - i
				as.bestELR = stoneLayRate
				as.bestSR = stoneShipRate
				//bestString = fmt.Sprintf("T-%d Q-%d %2.3f %2.3f  min:%2.3f\n", i, (as.stones - i), stoneLayRate, stoneShipRate, min(stoneLayRate, stoneShipRate))
			}
			//fmt.Printf("Stone %d/%d: %2.3f %2.3f  min:%2.3f\n", i, (as.stones - i), stoneLayRate, stoneShipRate, min(stoneLayRate, stoneShipRate))
		}
		var notes string
		matchQ := ""
		if as.quantWant == as.quantStones {
			matchQ = "*"
		} else if as.quantWant > as.quantStones {
			notes = fmt.Sprintf("%d more quant", as.quantWant-as.quantStones)
		}
		matchT := ""
		if as.tachWant == as.tachStones {
			matchT = "*"
		} else if as.tachWant > as.tachStones {
			notes = fmt.Sprintf("%d more tach", as.tachWant-as.tachStones)
		}

		if details {
			table.Append([]string{as.name,
				as.deflector.abbrev, as.metronome.abbrev, as.compass.abbrev, as.gusset.abbrev,
				fmt.Sprintf("%d%s", as.tachWant, matchT), fmt.Sprintf("%d%s", as.quantWant, matchQ),
				fmt.Sprintf("%2.3f", as.bestELR), fmt.Sprintf("%2.3f", as.bestSR),
				fmt.Sprintf("%2.3f", bestTotal),
				strings.Join(as.collegg, ","), notes})
		} else if matchT != "*" {
			table.Append([]string{as.name,
				fmt.Sprintf("%d%s", as.tachWant, matchT), fmt.Sprintf("%d%s", as.quantWant, matchQ),
				notes})
		}

	}
	fmt.Fprintf(&builder, "Stones Report for **%s**/**%s**\n", contractID, coopID)
	fmt.Fprintf(&builder, "Coop Deflector Bonus: %2.0f%%\n", everyoneDeflectorPercent)
	fmt.Fprint(&builder, "Tachyon & Quantum columns show the optimal mix.\n")
	if !details {
		fmt.Fprint(&builder, "Only showing farmers needing to swap stones.\n")

	}

	builder.WriteString("```")
	table.Render()
	builder.WriteString("```")

	if dataTimestampStr != "" {
		builder.WriteString(dataTimestampStr)
	}

	return builder.String()
}
