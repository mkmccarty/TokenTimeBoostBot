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
	"strconv"
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
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "solo-report-name",
				Description: "egg-inc game name for solo report",
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

	var contractID string
	var coopID string
	var soloName string
	flags := discordgo.MessageFlags(0)
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
	if opt, ok := optionMap["solo-report-name"]; ok {
		soloName = strings.ToLower(opt.StringValue())
		flags = discordgo.MessageFlagsEphemeral
	}
	userID := getInteractionUserID(i)
	if opt, ok := optionMap["details"]; ok {
		details = opt.BoolValue()
		farmerstate.SetMiscSettingFlag(userID, "stone-details", details)
	} else {
		details = farmerstate.GetMiscSettingFlag(userID, "stone-details")
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

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

	s1 := DownloadCoopStatusStones(contractID, coopID, details, soloName)
	builder.WriteString(s1)

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: builder.String(),
			Flags:   discordgo.MessageFlagsSupressEmbeds,
		})
	// Split s1 message into 2000 character chunks separated by newlines

}

// DownloadCoopStatusStones will download the coop status for a given contract and coop ID
func DownloadCoopStatusStones(contractID string, coopID string, details bool, soloName string) string {
	eggIncID := config.EIUserIdBasic
	reqURL := "https://www.auxbrain.com/ei/coop_status"
	enc := base64.StdEncoding

	var builder strings.Builder
	var dataTimestampStr string
	var protoData string

	eiContract := ei.EggIncContractsAll[contractID]
	cacheID := contractID + ":" + coopID
	cachedData := eiDatas[cacheID]

	skipArtifact := false

	if eiContract.MaxCoopSize > 15 {
		// Larger contracts take more text, skip artifacts
		skipArtifact = true
	}

	// Check if the file exists
	if strings.HasPrefix(coopID, "!!") {
		basename := coopID[2:]
		coopID = coopID[2:]
		fname := fmt.Sprintf("ttbb-data/%s-%s.pb", contractID, basename)

		if strings.HasPrefix(basename, "!") {
			coopID = coopID[1:]
			index, err := strconv.Atoi(basename[1:2])
			if err == nil {
				files, err := os.ReadDir("ttbb-data")
				if err != nil {
					return err.Error()
				}

				var filenames []string
				for _, file := range files {
					if strings.HasPrefix(file.Name(), fmt.Sprintf("%s-%s", contractID, coopID)) {
						filenames = append(filenames, file.Name())
					}
				}

				if len(filenames) > 0 && index < len(filenames) {
					fname = fmt.Sprintf("ttbb-data/%s-%s", contractID, filenames[index])
				} else {
					return fmt.Sprint("Files: ", strings.Join(filenames, ", "))
				}
			}

			// Process the filenames...
		}

		// read the contents of filename into protoData
		protoDataBytes, _ := os.ReadFile(fname)
		protoData = string(protoDataBytes)

		fileNameParts := strings.Split(fname[:len(fname)-3], "-")
		timestampStr := fileNameParts[len(fileNameParts)-1]
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			return err.Error()
		}
		dataTimestampStr = fmt.Sprintf("\nUsing cached data from file %s", fname)
		dataTimestampStr += fmt.Sprintf(", timestamp: %s", time.Unix(timestamp, 0).Format("2006-01-02 15:04:05"))

	} else if cachedData != nil && time.Now().Before(cachedData.expirationTimestamp) {
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

		// Save protoData into a file
		fileName := fmt.Sprintf("ttbb-data/%s-%s-%d.pb", contractID, coopID, time.Now().Unix())
		err = os.WriteFile(fileName, []byte(protoData), 0644)
		if err != nil {
			log.Print(err)
			return err.Error()
		}
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
		"T2C": 10.0, "T2E": 12.0,
		"T3C": 15.0, "T3R": 16.0,
		"T4C": 20.0, "T4E": 22.0, "T4L": 25.0,
	}

	type artifact struct {
		name    string
		abbrev  string
		percent float64
	}

	type artifactSet struct {
		name             string
		note             []string
		missingResearch  []string
		offline          string
		baseLayingRate   float64
		baseShippingRate float64
		baseHab          float64
		stones           int
		deflector        artifact
		metronome        artifact
		compass          artifact
		gusset           artifact

		tachStoneSlotted   int
		tachStones         []int
		tachStonesPercent  float64
		tachStonesBest     float64
		quantStones        []int
		quantStoneSlotted  int
		quantStonesPercent float64
		quantStonesBest    float64

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
		soloData  [][]string
	}
	var artifactSets []artifactSet

	//baseLaying := 3.772
	//baseShipping := 7.148

	var totalContributions float64
	var contributionRatePerSecond float64
	setContractEstimage := true

	alternateStr := ""

	grade := int(decodeCoopStatus.GetGrade())

	artifactPercentLevels := []float64{1.02, 1.04, 1.05}

	everyoneDeflectorPercent := 0.0
	for _, c := range decodeCoopStatus.GetContributors() {

		totalContributions += c.GetContributionAmount()
		totalContributions += -(c.GetContributionRate() * c.GetFarmInfo().GetTimestamp()) // offline eggs
		contributionRatePerSecond += c.GetContributionRate()

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
		researchComplete := true
		var missingResearch []string

		userLayRate := 1 / 30.0 // 1 chicken per 30 seconds
		//userShippingCap := 50000000.0
		hoverOnlyMultiplier := 1.0
		hyperloopOnlyMultiplier := 1.0
		universalShippingMultiplier := 1.0

		//commonLayRate := 1.0
		universalHabCapacity := 1.0
		portalHabCapacity := 1.0

		for _, cr := range fi.GetCommonResearch() {
			switch cr.GetId() {
			case "comfy_nests": // 50
				userLayRate *= (1 + 0.1*float64(cr.GetLevel())) // Comfortable Nests 10%
				if cr.GetLevel() != 50 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("comfy_nests %d/50", cr.GetLevel()))
				}
			case "hen_house_ac": // 50
				userLayRate *= (1 + 0.05*float64(cr.GetLevel())) // Hen House Expansion 10%
				if cr.GetLevel() != 50 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("hen_house_ac %d/50", cr.GetLevel()))
				}
			case "improved_genetics": // 30
				userLayRate *= (1 + 0.15*float64(cr.GetLevel())) // Internal Hatcheries 15%
				if cr.GetLevel() != 30 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("improved_genetics %d/30", cr.GetLevel()))
				}
			case "time_compress": // 20
				userLayRate *= (1 + 0.1*float64(cr.GetLevel())) // Time Compression 10%
				if cr.GetLevel() != 20 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("time_compress %d/20", cr.GetLevel()))
				}
			case "timeline_diversion": // 50
				userLayRate *= (1 + 0.02*float64(cr.GetLevel())) // Timeline Diversion 2%
				if cr.GetLevel() != 50 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("timeline_diversion %d/50", cr.GetLevel()))
				}
			case "relativity_optimization": // 10
				userLayRate *= (1 + 0.1*float64(cr.GetLevel())) // Relativity Optimization 10%
				if cr.GetLevel() != 10 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("relativity_optimization %d/10", cr.GetLevel()))
				}
			case "leafsprings": // 30
				universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Leafsprings 5%
				if cr.GetLevel() != 30 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("leafsprings %d/30", cr.GetLevel()))
				}
			case "lightweight_boxes": // 40
				universalShippingMultiplier *= (1 + 0.1*float64(cr.GetLevel())) // Lightweight Boxes 10%
				if cr.GetLevel() != 40 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("lightweight_boxes %d/40", cr.GetLevel()))
				}
			case "driver_training": // 30
				universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Driver Training 5%
				if cr.GetLevel() != 30 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("driver_training %d/30", cr.GetLevel()))
				}
			case "super_alloy": // 50
				universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Super Alloy 5%
				if cr.GetLevel() != 50 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("super_alloy %d/50", cr.GetLevel()))
				}
			case "quantum_storage": // 20
				universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Quantum Storage 5%
				if cr.GetLevel() != 20 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("quantum_storage %d/20", cr.GetLevel()))
				}
			case "hover_upgrades": // 25
				// Need to only do this for the vehicles that have hover upgrades
				hoverOnlyMultiplier = (1 + 0.05*float64(cr.GetLevel())) // Hover Upgrades 5%
				if cr.GetLevel() != 25 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("hover_upgrades %d/25", cr.GetLevel()))
				}
			case "dark_containment": // 25
				universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Dark Containment 5%
				if cr.GetLevel() != 25 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("dark_containment %d/25", cr.GetLevel()))
				}
			case "neural_net_refine": // 25
				universalShippingMultiplier *= (1 + 0.05*float64(cr.GetLevel())) // Neural Net Refine 5%
				if cr.GetLevel() != 25 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("neural_net_refine %d/25", cr.GetLevel()))
				}
			case "hyper_portalling": // 25
				hyperloopOnlyMultiplier = (1 + 0.05*float64(cr.GetLevel())) // Hyper Portalling 5%
				if cr.GetLevel() != 25 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("hyper_portalling %d/25", cr.GetLevel()))
				}
			case "hab_capacity1": // 8
				universalHabCapacity *= (1.0 + 0.05*float64(cr.GetLevel())) // Hab Capacity 5%
				if cr.GetLevel() != 8 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("hab_capacity %d/8", cr.GetLevel()))
				}
			case "microlux": // 10
				universalHabCapacity *= (1.0 + 0.05*float64(cr.GetLevel())) // Microlux 5%
				if cr.GetLevel() != 10 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("microlux %d/10", cr.GetLevel()))
				}
			case "grav_plating": // 25
				universalHabCapacity *= (1.0 + 0.02*float64(cr.GetLevel())) // Grav Plating 2%
				if cr.GetLevel() != 25 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("grav_plating %d/25", cr.GetLevel()))
				}
			case "wormhole_dampening": // 25
				portalHabCapacity = (1.0 + 0.02*float64(cr.GetLevel())) // Wormhole Dampening 2%
				if cr.GetLevel() != 25 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("wormhole_dampening %d/25", cr.GetLevel()))
				}
			}

		}

		for _, er := range fi.GetEpicResearch() {
			switch er.GetId() {
			case "epic_egg_laying": // 20
				userLayRate *= (1 + 0.05*float64(er.GetLevel())) // Epic Egg Laying 5%
				if er.GetLevel() != 20 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("epic_egg_laying %d/20", er.GetLevel()))
				}
			case "transportation_lobbyist": // 30
				universalShippingMultiplier *= (1 + 0.05*float64(er.GetLevel())) // Transportation Lobbyist 5%
				if er.GetLevel() != 30 {
					researchComplete = false
					missingResearch = append(missingResearch, fmt.Sprintf("transportation_lobbyist %d/30", er.GetLevel()))
				}
			}
		}

		//userLayRate *= 3600 // convert to hr rate
		habPopulation := 0.0
		for _, hab := range fi.GetHabPopulation() {
			habPopulation += float64(hab)
		}
		habCapacity := 0.0
		for _, hab := range fi.GetHabCapacity() {
			habCapacity += float64(hab)
		}

		baseHab := 0.0
		for _, hab := range fi.GetHabs() {
			// Values 1->18 for each of these
			value := 0.0
			if hab != 19 {
				value = float64(ei.Habs[hab].BaseCapacity)
				if ei.IsPortalHab(ei.Habs[hab]) {
					value *= portalHabCapacity
				}
				value *= universalHabCapacity
			}
			baseHab += value
		}
		as.baseHab = math.Round(baseHab)

		// Compare production hab capacity and production hab population
		if as.farmCapacity != habCapacity || habPopulation != as.farmPopulation {
			log.Print("Farm Capacity and Farm Population do not match")
		}

		//userLayRate *= 3600 // convert to hr rate
		as.baseLayingRate = userLayRate * habPopulation * 3600.0 / 1e15
		//as.baseShippingRate = userShippingCap * 10.0 * float64(len(fi.GetTrainLength())) * 60 / 1e16

		userShippingCap, shippingNote := ei.GetVehiclesShippingCapacity(fi.GetVehicles(), fi.GetTrainLength(), universalShippingMultiplier, hoverOnlyMultiplier, hyperloopOnlyMultiplier)
		as.baseShippingRate = userShippingCap * 60 / 1e15

		offlineTime := -c.GetFarmInfo().GetTimestamp() / 60
		if offlineTime > 5 {
			as.offline = fmt.Sprintf("🎣%1.0fm ", math.Round(offlineTime))
		}
		if !researchComplete {
			as.missingResearch = append(as.missingResearch, strings.Join(missingResearch, ", "))
		}
		if len(shippingNote) > 0 {
			as.note = append(as.note, shippingNote)
		}

		for _, artifact := range fi.GetEquippedArtifacts() {
			spec := artifact.GetSpec()
			strType := levels[spec.GetLevel()] + rarity[spec.GetRarity()]

			numStones, _ := ei.GetStones(spec.GetName(), spec.GetLevel(), spec.GetRarity())
			if numStones != len(artifact.GetStones()) {
				as.note = append(as.note, fmt.Sprintf("%s %d/%d slots used", ei.ArtifactSpec_Name_name[int32(spec.GetName())], len(artifact.GetStones()), numStones))
			}

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
				//if numStones < 3 {
				//	artifactName := int32(spec.GetName())
				//	as.note = append(as.note, fmt.Sprintf("%s only %d slots", ei.ArtifactSpec_Name_name[artifactName], numStones))
				//}
			}

			if as.tachStones == nil {
				as.tachStones = make([]int, 3)
				as.tachStonesPercent = 1.0
			}
			if as.quantStones == nil {
				as.quantStones = make([]int, 3)
				as.quantStonesPercent = 1.0
			}
			for _, stone := range artifact.GetStones() {
				if stone.GetName() == ei.ArtifactSpec_TACHYON_STONE {
					as.tachStones[stone.GetLevel()]++
					value := artifactPercentLevels[stone.GetLevel()]
					as.tachStonesPercent *= value
					if value > as.tachStonesBest {
						as.tachStonesBest = value
					}
					as.tachStoneSlotted++
				}
				if stone.GetName() == ei.ArtifactSpec_QUANTUM_STONE {
					as.quantStones[stone.GetLevel()]++
					value := artifactPercentLevels[stone.GetLevel()]
					as.quantStonesPercent *= value
					if value > as.quantStonesBest {
						as.quantStonesBest = value
					}
					as.quantStoneSlotted++
				}
			}

			// Now the count of stone slots
			as.stones += len(artifact.GetStones())
		}

		if as.deflector.percent == 0.0 {

			bestDeflectorPercent := 0.0
			for _, b := range c.BuffHistory {
				if b.GetEggLayingRate() > 1.0 {
					buffElr := (b.GetEggLayingRate() - 1.0) * 100.0
					if buffElr > bestDeflectorPercent {
						bestDeflectorPercent = buffElr
					}
				}
			}
			if bestDeflectorPercent == 0.0 {
				as.note = append(as.note, "Missing Deflector")
			} else {
				as.note = append(as.note, fmt.Sprintf("DEFL from BuffHist %2.0f%%", bestDeflectorPercent))
				as.deflector.abbrev = fmt.Sprintf("%d%%", int(bestDeflectorPercent))
				as.deflector.percent = bestDeflectorPercent
				everyoneDeflectorPercent += as.deflector.percent
			}
		}
		artifactSets = append(artifactSets, as)
	}

	table := tablewriter.NewWriter(&builder)
	if soloName != "" {
		table.SetHeader([]string{"Name", "Dfl", "Met", "Com", "Gus", "T", "Q", "ELR", "SR", "DLVRY", "egg%", "Notes"})
	} else {
		if details {
			if !skipArtifact {
				table.SetHeader([]string{"Name", "Dfl", "Met", "Com", "Gus", "Tach", "Quant", "ELR", "SR", "Delivery", "Collegg", "Notes"})
			} else {
				table.SetHeader([]string{"Name", "Tach", "Quant", "ELR", "SR", "Delivery", "Collegg", "Notes"})
			}
		} else {
			table.SetHeader([]string{"Name", "Tach", "Quant", "Notes"})
		}
	}
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding(" ") // pad with tabs
	table.SetNoWhiteSpace(true)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	// 1e15
	for i, as := range artifactSets {

		if soloName != "" && strings.ToLower(as.name) != soloName {
			continue
		}

		//fmt.Printf("name:\"%s\"  Stones:%d  elr:%f egg/chicken/s  sr:%f egg/s\n", as.name, as.stones, as.elr, as.sr)
		layingRate := (as.baseLayingRate) * (1 + as.metronome.percent/100.0) * (1 + as.gusset.percent/100.0) * eiContract.Grade[grade].ModifierELR
		shippingRate := (as.baseShippingRate) * (1 + as.compass.percent/100.0) * eiContract.Grade[grade].ModifierSR

		// Determine Colleggtible Increase
		stoneLayRateNow := layingRate * (1 + (everyoneDeflectorPercent-as.deflector.percent)/100.0)
		stoneLayRateNow *= math.Pow(1.02, float64(as.tachStones[ei.ArtifactSpec_INFERIOR]))
		stoneLayRateNow *= math.Pow(1.04, float64(as.tachStones[ei.ArtifactSpec_LESSER]))
		stoneLayRateNow *= math.Pow(1.05, float64(as.tachStones[ei.ArtifactSpec_NORMAL]))
		chickELR := as.elr * as.farmPopulation * eiContract.Grade[grade].ModifierHabCap * 3600.0 / 1e15
		collegELR := chickELR / stoneLayRateNow
		//fmt.Printf("Calc ELR: %2.3f  Param.Elr: %2.3f   Diff:%2.2f\n", stoneLayRateNow, chickELR, (chickELR / stoneLayRateNow))
		if collegELR < 1.00 {
			// Possible due to being offline
			//as.notes = "sync needed."
			// Maybe not fully boosted
			collegELR = 1.00
		}
		if collegELR > 1.000 {
			//fmt.Printf("Colleggtible Egg Laying Rate Factored in with %2.2f%%\n", collegELR)
			//as.collegg = append(as.collegg, fmt.Sprintf("ELR:%2.0f%%", (collegELR-1.0)*100.0))
			//farmerstate.SetMiscSettingString(as.name, "coll-elr", fmt.Sprintf("%2.0f%%", (collegELR-1.0)*100.0))
			collegELR = 1.00
		} /*else {
			hasColl := farmerstate.GetMiscSettingString(as.name, "coll-elr")
			if hasColl != "" {
				as.collegg = append(as.collegg, fmt.Sprintf("(ELR:%s)", hasColl))
				collegELR *= 1.05

			}
		}*/

		//	if collegELR < 1.00 {
		// If the user hasn't build up the farm much then the ELR will be low
		//	collegELR = 1.00
		//	}

		stoneShipRateNow := shippingRate
		stoneShipRateNow *= math.Pow(1.02, float64(as.quantStones[ei.ArtifactSpec_INFERIOR]))
		stoneShipRateNow *= math.Pow(1.04, float64(as.quantStones[ei.ArtifactSpec_LESSER]))
		stoneShipRateNow *= math.Pow(1.05, float64(as.quantStones[ei.ArtifactSpec_NORMAL]))

		//fmt.Printf("Calc SR: %2.3f  param.Sr: %2.3f   Diff:%2.2f\n", stoneShipRateNow, as.sr/1e15, (as.sr/1e15)/stoneShipRateNow)
		collegShip := (as.sr / 1e15) / stoneShipRateNow
		if collegShip > 1.000 {
			val := fmt.Sprintf("%2.2f🚚", (collegShip-1.0)*100.0)
			as.collegg = append(as.collegg, strings.Replace(strings.Replace(val, ".00", "", -1), ".25", "¼", -1))
		} else {
			// Likely because of a change in in a coop deflector value since they last synced
			artifactSets[i].note = append(artifactSets[i].note, fmt.Sprintf("SR: %2.4f(q:%d) - ei.sr: %2.4f  ratio:%2.4f", shippingRate, as.quantStones, (as.sr/1e15), collegShip))
		}

		bestTotal := 0.0

		// Default this to the maximum
		stoneBonusIncrease := 1.05

		hasLesserStones := as.quantStones[ei.ArtifactSpec_INFERIOR] + as.tachStones[ei.ArtifactSpec_INFERIOR] +
			as.quantStones[ei.ArtifactSpec_LESSER] + as.tachStones[*ei.ArtifactSpec_LESSER.Enum()]

		if hasLesserStones > 0 {
			maxPercentage := as.quantStonesPercent * as.tachStonesPercent

			// Empty stone slots assume lowest seen artifact quality.
			if as.stones != (as.quantStoneSlotted + as.tachStoneSlotted) {
				// Missing stone value, assign it the lowest seen quantity
				stoneDiff := as.stones - (as.quantStoneSlotted + as.tachStoneSlotted)
				if as.quantStones[ei.ArtifactSpec_INFERIOR] > 0 || as.tachStones[ei.ArtifactSpec_INFERIOR] > 0 {
					maxPercentage *= float64(stoneDiff) * artifactPercentLevels[ei.ArtifactSpec_INFERIOR]
					as.note = append(as.note, fmt.Sprintf("%d missing stones valued at %1.2f", stoneDiff, artifactPercentLevels[ei.ArtifactSpec_INFERIOR]))
				} else if as.quantStones[ei.ArtifactSpec_LESSER] > 0 || as.tachStones[ei.ArtifactSpec_LESSER] > 0 {
					maxPercentage *= float64(stoneDiff) * artifactPercentLevels[ei.ArtifactSpec_LESSER]
					as.note = append(as.note, fmt.Sprintf("%d missing stones valued at %1.2f", stoneDiff, artifactPercentLevels[ei.ArtifactSpec_LESSER]))
				}
			}
			// we know we have a certain number of stones
			// knowing our exponent we need the average value of each stone
			stoneBonusIncrease = math.Pow(math.E, math.Log(maxPercentage)/float64(as.stones))
		}

		// Simple search for those with only the 5% stones
		for i := 0; i <= as.stones; i++ {
			stoneLayRate := layingRate * (1 + (everyoneDeflectorPercent-as.deflector.percent)/100.0)
			stoneLayRate = stoneLayRate * math.Pow(stoneBonusIncrease, float64(i)) * collegELR

			stoneShipRate := shippingRate * math.Pow(stoneBonusIncrease, float64((as.stones-i))) * collegShip

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

		for i := 0; i <= as.stones; i++ {
			stoneLayRate := layingRate * (1 + (everyoneDeflectorPercent-as.deflector.percent)/100.0)
			stoneLayRate = stoneLayRate * math.Pow(1.05, float64(i)) * collegELR

			stoneShipRate := shippingRate * math.Pow(1.05, float64((as.stones-i))) * collegShip

			best := ""
			if as.tachWant == i {
				best = "⭐️"
			}
			as.soloData = append(as.soloData, []string{as.name,
				as.deflector.abbrev, as.metronome.abbrev, as.compass.abbrev, as.gusset.abbrev,
				fmt.Sprintf("%d%s", i, ""), fmt.Sprintf("%d%s", as.stones-i, ""),
				fmt.Sprintf("%2.3f", stoneLayRate), fmt.Sprintf("%2.3f", stoneShipRate),
				fmt.Sprintf("%s%2.3f", best, min(stoneLayRate, stoneShipRate)),
				strings.Join(as.collegg, ","), strings.Join(as.note, ",")})

		}
		var notes string
		if len(as.missingResearch) > 0 {
			notes += "🚩"
		}

		if as.offline != "" {
			notes += as.offline
		}

		matchQ := ""
		qStones := as.quantStones[ei.ArtifactSpec_INFERIOR] + as.quantStones[ei.ArtifactSpec_LESSER] + as.quantStones[ei.ArtifactSpec_NORMAL]
		if as.quantWant == qStones {
			matchQ = "⭐️"
		} else if as.quantWant > qStones {
			notes += fmt.Sprintf("+%d quant", as.quantWant-qStones)
			setContractEstimage = false
		}
		matchT := ""
		tStones := as.tachStones[ei.ArtifactSpec_INFERIOR] + as.tachStones[ei.ArtifactSpec_LESSER] + as.tachStones[ei.ArtifactSpec_NORMAL]

		if as.tachWant == tStones {
			matchT = "⭐️"
		} else if as.tachWant > tStones {
			notes += fmt.Sprintf("+%d tach", as.tachWant-tStones)
			setContractEstimage = false
		}
		if stoneBonusIncrease != 1.05 {
			if notes != "" {
				notes += " "
			}
			notes += fmt.Sprintf("(%1.2f^%d)", stoneBonusIncrease, as.stones)
		}

		if soloName != "" {
			for _, d := range as.soloData {
				table.Append(d)
			}
		} else {

			if details {
				if !skipArtifact {
					table.Append([]string{as.name,
						as.deflector.abbrev, as.metronome.abbrev, as.compass.abbrev, as.gusset.abbrev,
						fmt.Sprintf("%d%s", as.tachWant, matchT), fmt.Sprintf("%d%s", as.quantWant, matchQ),
						fmt.Sprintf("%2.3f", as.bestELR), fmt.Sprintf("%2.3f", as.bestSR),
						fmt.Sprintf("%2.3f", bestTotal),
						strings.Join(as.collegg, ","), notes})
				} else {
					table.Append([]string{as.name,
						fmt.Sprintf("%d%s", as.tachWant, matchT), fmt.Sprintf("%d%s", as.quantWant, matchQ),
						fmt.Sprintf("%2.3f", as.bestELR), fmt.Sprintf("%2.3f", as.bestSR),
						fmt.Sprintf("%2.3f", bestTotal),
						strings.Join(as.collegg, ","), notes})
				}
			} else if matchT != "⭐️" {
				table.Append([]string{as.name,
					fmt.Sprintf("%d%s", as.tachWant, matchT), fmt.Sprintf("%d%s", as.quantWant, matchQ),
					notes})
				alternateStr += as.name + ": T" + strconv.Itoa(as.tachWant) + " / Q" + strconv.Itoa(as.quantWant) + "\n"
			} else {
				alternateStr += as.name + ": T" + strconv.Itoa(as.tachWant) + " / Q" + strconv.Itoa(as.quantWant) + "⭐️\n"
			}
		}

	}

	fmt.Fprintf(&builder, "Stones Report for %s [**%s/%s**](%s)\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), contractID, coopID, fmt.Sprintf("%s/%s/%s", "https://eicoop-carpet.netlify.app", contractID, coopID))
	if eiContract.ID != "" {
		nowTime := time.Now()
		startTime := nowTime
		endTime := nowTime
		endStr := "End:"
		secondsRemaining := int64(decodeCoopStatus.GetSecondsRemaining())
		if decodeCoopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
			startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
			startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
			secondsSinceAllGoals := int64(decodeCoopStatus.GetSecondsSinceAllGoalsAchieved())
			endTime = endTime.Add(-time.Duration(secondsSinceAllGoals) * time.Second)
			//contractDurationSeconds = endTime.Sub(startTime).Seconds()
		} else {
			startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
			startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
			totalReq := eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1]
			calcSecondsRemaining := int64((totalReq - totalContributions) / contributionRatePerSecond)
			endTime = nowTime.Add(time.Duration(calcSecondsRemaining) * time.Second)
			endStr = "Est End:"
			//contractDurationSeconds := endTime.Sub(startTime).Seconds()
			if setContractEstimage {
				c := FindContract(contractID)
				if c != nil {
					c.EstimatedDuration = time.Duration(calcSecondsRemaining) * time.Second
					c.EstimatedDurationValid = true
					c.StartTime = startTime
					c.EstimatedEndTime = endTime
				}
			}
		}
		builder.WriteString(fmt.Sprintf("Start: **<t:%d:t>**   %s: **<t:%d:t>** for **%v**\n", startTime.Unix(), endStr, endTime.Unix(), endTime.Sub(startTime).Round(time.Second)))
		if eiContract.ModifierELR != 1.0 {
			fmt.Fprintf(&builder, "ELR Modifier: %2.1fx\n", eiContract.ModifierELR)
		}
		if eiContract.ModifierSR != 1.0 {
			fmt.Fprintf(&builder, "SR Modifier: %2.1fx\n", eiContract.ModifierSR)
		}
		if eiContract.ModifierHabCap != 1.0 {
			fmt.Fprintf(&builder, "Hab Capacity Modifier: %2.1fx\n", eiContract.ModifierHabCap)
		}

	}

	fmt.Fprintf(&builder, "Coop Deflector Bonus: %2.0f%%\n", everyoneDeflectorPercent)
	if soloName == "" {
		fmt.Fprint(&builder, "Tachyon & Quantum columns show the optimal mix.\n")
		if !details {
			fmt.Fprint(&builder, "Only showing farmers needing to swap stones.\n")
		}
	} else {
		fmt.Fprint(&builder, "Showing all stone variations for solo report.\n")
	}
	alternateStrHeader := builder.String()

	builder.WriteString("```")
	table.Render()
	builder.WriteString("```")

	for _, as := range artifactSets {
		if len(as.note) > 0 && len(builder.String()) < 2000 {
			builder.WriteString(fmt.Sprintf("**%s** Notes: %s\n", as.name, strings.Join(as.note, ", ")))
		}
	}

	if dataTimestampStr != "" {
		builder.WriteString(dataTimestampStr)
	}

	if len(builder.String()) > 2000 {
		if !details {
			return alternateStrHeader + "\n" + alternateStr
		}
		return DownloadCoopStatusStones(contractID, coopID, false, soloName)
	}

	return builder.String()
}
