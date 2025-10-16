package boost

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
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
				Name:        "tiled",
				Description: "Display using embedded tiles. Default is false. (sticky)",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "details",
				Description: "Show full details. Default is false. (sticky)",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "private-reply",
				Description: "Respond privately. Default is false.",
				Required:    false,
			},
			/*
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "solo-report-name",
					Description: "egg-inc game name for solo report",
					Required:    false,
				},
			*/
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "use-buffhistory",
				Description: "Use Buff History for unequipped Deflector. Default is false.",
				Required:    false,
			},
		},
	}
}

// HandleStonesCommand will handle the /stones command
func HandleStonesCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var contractID string
	var coopID string
	var soloName string
	privateReply := false
	flags := discordgo.MessageFlags(0)
	details := false
	useTiles := false

	optionMap := bottools.GetCommandOptionsMap(i)

	if opt, ok := optionMap["contract-id"]; ok {
		contractID = opt.StringValue()
		contractID = strings.ReplaceAll(contractID, " ", "")
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = strings.ToLower(opt.StringValue())
		coopID = strings.ReplaceAll(coopID, " ", "")
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
	useBuffHistory := false
	if opt, ok := optionMap["use-buffhistory"]; ok {
		useBuffHistory = opt.BoolValue()
	}
	if opt, ok := optionMap["private-reply"]; ok {
		privateReply = opt.BoolValue()
		if privateReply {
			flags |= discordgo.MessageFlagsEphemeral
		}
	}
	if opt, ok := optionMap["tiled"]; ok {
		useTiles = opt.BoolValue()
		farmerstate.SetMiscSettingFlag(userID, "stone-tiled", useTiles)
	} else {
		useTiles = farmerstate.GetMiscSettingFlag(userID, "stone-tiled")
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
		contractID = contract.ContractID
		coopID = strings.ToLower(contract.CoopID)
	}

	s1, urls, tiles := DownloadCoopStatusStones(contractID, coopID, details, soloName, useBuffHistory)

	contract := findContractByIDs(contractID, coopID)
	if contract != nil {
		if contract.State == ContractStateCompleted {
			// Only refresh if EstimateUpdateTime is within 10 seconds of now
			if math.Abs(time.Since(contract.EstimateUpdateTime).Seconds()) <= 10 {
				refreshBoostListMessage(s, contract)
			}
		}
	}
	if tiles != nil {
		cache := buildStonesCache(s1, urls, tiles)
		// Fill in our calling parameters
		cache.contractID = contractID
		cache.coopID = coopID
		cache.details = details
		cache.soloName = soloName
		cache.private = privateReply
		cache.useBuffHistory = useBuffHistory
		cache.displayTiles = useTiles

		stonesCacheMap[cache.xid] = cache

		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})

		sendStonesPage(s, i, true, cache.xid, false, false, false)

		// Traverse stonesCacheMap and delete expired entries
		for key, cache := range stonesCacheMap {
			if cache.expirationTimestamp.Before(time.Now()) {
				delete(stonesCacheMap, key)
			}
		}
	} else {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: s1,
		})
	}
}

type artifact struct {
	name    string
	abbrev  string
	percent float64
}

type artifactSet struct {
	name             string
	nameRaw          string
	note             []string
	missingResearch  []string
	missingStones    bool
	offline          string
	baseLayingRate   float64
	baseShippingRate float64
	baseHab          float64
	userLayRate      float64
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
	elr2           float64 // ELR after colleggtibles

	tachWant       int
	quantWant      int
	bestELR        float64
	bestSR         float64
	collegg        []string
	soloData       [][]string
	staabArtifacts []string
	colleggSR      float64
	colleggELR     float64
	artifactSlots  []string // Name of artifacts in each slot
	//colleggELR     float64
	//colleggHab     float64
}

// DownloadCoopStatusStones will download the coop status for a given contract and coop ID
func DownloadCoopStatusStones(contractID string, coopID string, details bool, soloName string, useBuffHistory bool) (string, string, []*discordgo.MessageEmbedField) {
	var builderURL strings.Builder
	var field []*discordgo.MessageEmbedField

	coopStatus, _, dataTimestampStr, err := ei.GetCoopStatus(contractID, coopID)
	if err != nil {
		return err.Error(), "", field
	}
	var builder strings.Builder
	eiContract := ei.EggIncContractsAll[contractID]
	dimension := ei.GameModifier_INVALID
	rate := 1.0
	if eiContract.ModifierSR != 1.0 {
		dimension = ei.GameModifier_SHIPPING_CAPACITY
		rate = eiContract.ModifierSR
	}
	if eiContract.ModifierELR != 1.0 {
		dimension = ei.GameModifier_EGG_LAYING_RATE
		rate = eiContract.ModifierELR
	}
	if eiContract.ModifierHabCap != 1.0 {
		dimension = ei.GameModifier_HAB_CAPACITY
		rate = eiContract.ModifierHabCap
	}

	skipArtifact := false
	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		return ei.ContractCoopStatusResponse_ResponseStatus_name[int32(coopStatus.GetResponseStatus())], "", nil
	}

	coopID = coopStatus.GetCoopIdentifier()

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

	var artifactSets []artifactSet

	// Find maximum Colleggtibles
	maxCollectibleELR, maxColllectibleShip, maxColleggtibleHab := ei.GetColleggtibleValues()
	colleggtibleStr := []string{}
	if maxColllectibleShip > 1.0 {
		colleggtibleStr = append(colleggtibleStr, fmt.Sprintf("🚚%2.4g%%", (maxColllectibleShip-1.0)*100))
	}
	if maxCollectibleELR > 1.0 {
		colleggtibleStr = append(colleggtibleStr, fmt.Sprintf("📦%2.4g%%", (maxCollectibleELR-1.0)*100))
	}
	if maxColleggtibleHab > 1.0 {
		colleggtibleStr = append(colleggtibleStr, fmt.Sprintf("🛖%2.4g%%", (maxColleggtibleHab-1.0)*100))
	}

	//baseLaying := 3.772
	//baseShipping := 7.148

	var totalContributions float64
	var contributionRatePerSecond float64
	setContractEstimate := true

	//alternateStr := ""

	grade := int(coopStatus.GetGrade())

	artifactPercentLevels := []float64{1.02, 1.04, 1.05}

	everyoneDeflectorPercent := 0.0

	contributors := coopStatus.GetContributors()
	sort.Slice(contributors, func(i, j int) bool {
		return strings.ToLower(contributors[i].GetUserName()) < strings.ToLower(contributors[j].GetUserName())
	})

	for _, c := range contributors {

		//for _, c := range coopStatus.GetContributors() {

		totalContributions += c.GetContributionAmount()
		totalContributions += -(c.GetContributionRate() * c.GetFarmInfo().GetTimestamp()) // offline eggs
		contributionRatePerSecond += c.GetContributionRate()

		as := artifactSet{}
		as.name = c.GetUserName()
		as.nameRaw = as.name
		// Strip any multibyte characters from as.name and replace with ~
		/*
			cleanName := make([]rune, len(as.name))
			for i, r := range as.name {
				if r > 127 {
					cleanName[i] = '?'
				} else {
					cleanName[i] = r
				}
			}
			as.name = strings.ReplaceAll(string(cleanName), "\x00", "")
		*/
		p := c.GetProductionParams()
		as.farmCapacity = p.GetFarmCapacity()
		as.farmPopulation = p.GetFarmPopulation()
		as.elr = p.GetElr()
		as.ihr = p.GetIhr()
		as.sr = p.GetSr() // This is per second, convert to hour
		as.sr *= 3600.0

		as.elr2 = as.elr * as.farmPopulation * 3600.0

		// To simulate colleggtibles we can multiply these Production Params by a percentage.
		//as.farmCapacity *= 1.05
		//as.elr *= 1.03
		//as.elr *= 1.05

		as.tachStones = make([]int, 3)
		as.tachStonesPercent = 1.0
		as.quantStones = make([]int, 3)
		as.quantStonesPercent = 1.0

		as.deflector.percent = 0.0
		as.compass.percent = 0.0
		as.metronome.percent = 0.0
		as.metronome.percent = 0.0

		fi := c.GetFarmInfo()
		researchComplete := true
		var missingResearch []string

		userLayRate := 1 / 30.0 // 1 chicken per 30 seconds
		hoverOnlyMultiplier := 1.0
		hyperloopOnlyMultiplier := 1.0
		universalShippingMultiplier := 1.0

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

		as.userLayRate = userLayRate
		as.baseHab = math.Round(baseHab)

		// Compare production hab capacity and production hab population
		if as.farmCapacity != habCapacity || habPopulation != as.farmPopulation {
			log.Print("Farm Capacity and Farm Population do not match")

			// Probably a colleggtible
		}

		//userLayRate *= 3600 // convert to hr rate
		as.baseLayingRate = as.userLayRate * as.baseHab * 3600.0 / 1e15
		//as.baseLayingRate = userLayRate * min(habPopulation, as.baseHab) * 3600.0 / 1e15

		userShippingCap, shippingNote := ei.GetVehiclesShippingCapacity(fi.GetVehicles(), fi.GetTrainLength(), universalShippingMultiplier, hoverOnlyMultiplier, hyperloopOnlyMultiplier)
		as.baseShippingRate = userShippingCap * 60 / 1e15

		offlineTime := -c.GetFarmInfo().GetTimestamp() / 60
		if offlineTime >= 5 {
			as.offline = fmt.Sprintf("🎣%s ", bottools.FmtDuration(time.Duration(offlineTime)*time.Minute))
		}
		if !researchComplete {
			as.missingResearch = append(as.missingResearch, strings.Join(missingResearch, ", "))
		}
		if len(shippingNote) > 0 {
			as.note = append(as.note, shippingNote)
		}

		as.staabArtifacts = make([]string, 4)
		as.artifactSlots = make([]string, 4)

		for i, artifact := range fi.GetEquippedArtifacts() {
			spec := artifact.GetSpec()
			strType := levels[spec.GetLevel()] + rarity[spec.GetRarity()]

			numStones, _ := ei.GetStones(spec.GetName(), spec.GetLevel(), spec.GetRarity())
			if numStones != len(artifact.GetStones()) {
				as.missingStones = true
				//as.note = append(as.note, fmt.Sprintf("%s %d/%d slots used", ei.ArtifactSpec_Name_name[int32(spec.GetName())], len(artifact.GetStones()), numStones))
			}

			as.artifactSlots[i] = fmt.Sprintf("%s%s", ei.ShortArtifactName[int32(spec.GetName())], strType)

			switch spec.GetName() {
			case ei.ArtifactSpec_TACHYON_DEFLECTOR:
				as.deflector.percent = deflector[strType]
				as.deflector.name = fmt.Sprintf("%s %s %2.0f%% %d slots", "Deflector", strType, as.deflector.percent, numStones)
				as.deflector.abbrev = strType
				as.staabArtifacts[i] = fmt.Sprintf("%s Defl.", strType)
				everyoneDeflectorPercent += as.deflector.percent
			case ei.ArtifactSpec_QUANTUM_METRONOME:
				as.metronome.percent = metronome[strType]
				as.metronome.name = fmt.Sprintf("%s %s %2.0f%% %d slots", "Metronome", strType, as.metronome.percent, numStones)
				as.metronome.abbrev = strType
				as.staabArtifacts[i] = fmt.Sprintf("%s Metro", strType)
			case ei.ArtifactSpec_INTERSTELLAR_COMPASS:
				as.compass.percent = compass[strType]
				as.metronome.name = fmt.Sprintf("%s %s %2.0f%% %d slots", "Compass", strType, as.compass.percent, numStones)
				as.compass.abbrev = strType
				as.staabArtifacts[i] = fmt.Sprintf("%s Comp", strType)
			case ei.ArtifactSpec_ORNATE_GUSSET:
				as.gusset.percent = gussett[strType]
				as.metronome.name = fmt.Sprintf("%s %s %2.0f%% %d slots", "Gusset", strType, as.metronome.percent, numStones)
				as.gusset.abbrev = strType
				as.staabArtifacts[i] = fmt.Sprintf("%s Gusset", strType)
			case ei.ArtifactSpec_SHIP_IN_A_BOTTLE:
				as.staabArtifacts[i] = fmt.Sprintf("%s SIAB", strType)
			case ei.ArtifactSpec_UNKNOWN:
				as.staabArtifacts[i] = "Empty"
			default:
				if numStones == 3 {
					as.staabArtifacts[i] = "3 Slot"
				} else {
					as.staabArtifacts[i] = "Empty"
				}
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
			lastDeflectorPercent := 0.0
			for _, b := range c.BuffHistory {
				if b.GetEggLayingRate() > 1.0 {
					buffElr := math.Round((b.GetEggLayingRate() - 1.0) * 100.0)
					if buffElr > bestDeflectorPercent {
						bestDeflectorPercent = buffElr
					}
					lastDeflectorPercent = buffElr
				}
			}
			if bestDeflectorPercent == 0.0 {
				if as.name != "[departed]" {
					as.note = append(as.note, "Missing Deflector")
				}
			} else if !useBuffHistory {
				if as.baseLayingRate == 0 && as.baseShippingRate == 0 {
					everyoneDeflectorPercent += lastDeflectorPercent
				}
			} else if useBuffHistory {
				as.note = append(as.note, fmt.Sprintf("DEFL from BuffHist %2.0f%%", bestDeflectorPercent))
				as.deflector.abbrev = fmt.Sprintf("%2.0f%%", bestDeflectorPercent)
				as.deflector.percent = bestDeflectorPercent
				everyoneDeflectorPercent += as.deflector.percent
			}
		}
		artifactSets = append(artifactSets, as)
	}

	var tableHeader string
	var tableData []string

	needLegend := false
	showGlitch := false

	// 1e15
	for _, as := range artifactSets {

		if soloName != "" && strings.ToLower(as.nameRaw) != soloName {
			continue
		}
		// need to reduce the farm population by the gusset percent
		unmodifiedPop := as.farmPopulation / (1 + as.gusset.percent/100.0)
		as.baseLayingRate = as.userLayRate * min(unmodifiedPop, as.baseHab) * 3600.0 / 1e15

		layingRate := (as.baseLayingRate) * (1 + as.metronome.percent/100.0) * (1 + as.gusset.percent/100.0) * eiContract.Grade[grade].ModifierELR
		shippingRate := (as.baseShippingRate) * (1 + as.compass.percent/100.0) * eiContract.Grade[grade].ModifierSR

		privateFarm := false
		// Determine Colleggtible Increase

		collegHab := as.farmCapacity / (as.baseHab * (1 + as.gusset.percent/100.0))
		// Check for hab size
		if collegHab < 1.00 {
			collegHab = 1.00
		}
		//as.colleggHab = collegHab

		if maxColleggtibleHab > 1.0 {
			if collegHab > 1.000 && collegHab < maxColleggtibleHab {
				//log.Printf("Colleggtible Egg Laying Rate Factored in with %2.2f%%\n", collegELR)
				//as.collegg = append(as.collegg, fmt.Sprintf("ELR:%2.0f%%", (collegELR-1.0)*100.0))
				//farmerstate.SetMiscSettingString(as.name, "coll-elr", fmt.Sprintf("%2.0f%%", (collegELR-1.0)*100.0))
				val := fmt.Sprintf("%2.2f🛖", (collegHab-1.0)*100.0)
				val = strings.ReplaceAll(val, ".00", "")
				val = strings.ReplaceAll(val, ".25", "¼")
				val = strings.ReplaceAll(val, ".5", "½")
				val = strings.ReplaceAll(val, ".75", "¾")
				as.collegg = append(as.collegg, val)
				//anyColleggtiblesToShow = true
			} else if collegHab <= 1.0 {
				as.collegg = append(as.collegg, "🛖")
				//anyColleggtiblesToShow = true
			}
		}

		stoneLayRateNow := layingRate * (1 + (everyoneDeflectorPercent-as.deflector.percent)/100.0)
		stoneLayRateNow *= math.Pow(1.02, float64(as.tachStones[ei.ArtifactSpec_INFERIOR]))
		stoneLayRateNow *= math.Pow(1.04, float64(as.tachStones[ei.ArtifactSpec_LESSER]))
		stoneLayRateNow *= math.Pow(1.05, float64(as.tachStones[ei.ArtifactSpec_NORMAL]))
		chickELR := as.elr * as.farmPopulation * eiContract.Grade[grade].ModifierHabCap * 3600.0 / 1e15

		if stoneLayRateNow == 0.0 {
			privateFarm = true
			stoneLayRateNow = chickELR
			as.bestELR = stoneLayRateNow
			layingRate = stoneLayRateNow
		}
		collegELR := chickELR / stoneLayRateNow
		//log.Printf("Calc ELR: %2.3f  Param.Elr: %2.3f   Diff:%2.2f\n", stoneLayRateNow, chickELR, (chickELR / stoneLayRateNow))
		// No IHR Egg yet, this will need to be revisited
		as.colleggELR = math.Round(collegELR*1000) / 1000

		if maxCollectibleELR > 1.0 {
			roundedCollegELR := math.Round(collegELR*1000) / 1000
			if roundedCollegELR > 1.000 && roundedCollegELR < maxCollectibleELR {
				//log.Printf("Colleggtible Egg Laying Rate Factored in with %2.2f%%\n", collegELR)
				//as.collegg = append(as.collegg, fmt.Sprintf("ELR:%2.0f%%", (collegELR-1.0)*100.0))
				//farmerstate.SetMiscSettingString(as.name, "coll-elr", fmt.Sprintf("%2.0f%%", (collegELR-1.0)*100.0))
				val := fmt.Sprintf("%2.2f📦", (roundedCollegELR-1.0)*100.0)
				val = strings.ReplaceAll(val, ".00", "")
				val = strings.ReplaceAll(val, ".25", "¼")
				val = strings.ReplaceAll(val, ".5", "½")
				val = strings.ReplaceAll(val, ".75", "¾")
				as.collegg = append(as.collegg, val)
				//anyColleggtiblesToShow = true
			} else if collegELR == 1.0 {
				as.collegg = append(as.collegg, "📦")
				//anyColleggtiblesToShow = true
			}
		}

		stoneShipRateNow := shippingRate
		stoneShipRateNow *= math.Pow(1.02, float64(as.quantStones[ei.ArtifactSpec_INFERIOR]))
		stoneShipRateNow *= math.Pow(1.04, float64(as.quantStones[ei.ArtifactSpec_LESSER]))
		stoneShipRateNow *= math.Pow(1.05, float64(as.quantStones[ei.ArtifactSpec_NORMAL]))

		if stoneShipRateNow == 0.0 {
			stoneShipRateNow = as.sr / 1e15
			as.bestSR = stoneShipRateNow
			shippingRate = stoneShipRateNow
		}
		//log.Printf("Calc SR: %2.3f  param.Sr: %2.3f   Diff:%2.2f\n", stoneShipRateNow, as.sr/1e15, (as.sr/1e15)/stoneShipRateNow)
		collegShip := (as.sr / 1e15) / stoneShipRateNow
		as.colleggSR = math.Round(collegShip*10000) / 10000

		if maxColllectibleShip > 1.0 {
			roundedCollegShip := math.Round(collegShip*10000) / 10000
			if roundedCollegShip > 1.000 && roundedCollegShip < maxColllectibleShip {
				val := fmt.Sprintf("%2.2f🚚", (roundedCollegShip-1.0)*100.0)
				val = strings.ReplaceAll(val, ".00", "")
				val = strings.ReplaceAll(val, ".25", "¼")
				val = strings.ReplaceAll(val, ".5", "½")
				val = strings.ReplaceAll(val, ".75", "¾")
				as.collegg = append(as.collegg, val)

				//anyColleggtiblesToShow = true
			} else if collegShip == 1.0 {
				as.collegg = append(as.collegg, "🚚")
				//anyColleggtiblesToShow = true
			}
		}
		/*
			else{
				// Likely because of a change in in a coop deflector value since they last synced
				artifactSets[i].note = append(artifactSets[i].note, fmt.Sprintf("SR: %2.4f(q:%d) - ei.sr: %2.4f  ratio:%2.4f", shippingRate, as.quantStones, (as.sr/1e15), collegShip))
			}
		*/

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
			stoneLayRate := layingRate
			if !privateFarm {
				stoneLayRate *= (1 + (everyoneDeflectorPercent-as.deflector.percent)/100.0)
			}
			stoneLayRate *= math.Pow(stoneBonusIncrease, float64(i)) * collegELR

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
			//log.Printf("Stone %d/%d: %2.3f %2.3f  min:%2.3f\n", i, (as.stones - i), stoneLayRate, stoneShipRate, min(stoneLayRate, stoneShipRate))
		}

		for i := 0; i <= as.stones; i++ {
			stoneLayRate := layingRate * (1 + (everyoneDeflectorPercent-as.deflector.percent)/100.0)
			stoneLayRate = stoneLayRate * math.Pow(1.05, float64(i)) * collegELR

			stoneShipRate := shippingRate * math.Pow(1.05, float64((as.stones-i))) * collegShip

			soloData := []string{
				bottools.AlignString(as.name, 12, bottools.StringAlignCenter),
				bottools.AlignString(fmt.Sprintf("%d", i), 3, bottools.StringAlignCenter),
				bottools.AlignString(fmt.Sprintf("%d", as.stones-i), 3, bottools.StringAlignCenter),
				bottools.AlignString(fmt.Sprintf("%2.3f", stoneLayRate), 6, bottools.StringAlignCenter),
				bottools.AlignString(fmt.Sprintf("%2.3f", stoneShipRate), 6, bottools.StringAlignCenter),
				bottools.AlignString(as.deflector.abbrev, 3, bottools.StringAlignRight),
				bottools.AlignString(as.metronome.abbrev, 3, bottools.StringAlignRight),
				bottools.AlignString(as.compass.abbrev, 3, bottools.StringAlignRight),
				bottools.AlignString(as.gusset.abbrev, 3, bottools.StringAlignRight),
			}
			if len(as.collegg) > 0 {
				combined := append(as.collegg, as.note...)
				as.note = combined
			}
			soloData = append(soloData, bottools.AlignString(strings.Join(as.note, ","), 3, bottools.StringAlignLeft))
			as.soloData = append(as.soloData, soloData)

		}
		var notes string
		if len(as.missingResearch) > 0 {
			notes += "🚩"
			needLegend = true
		}
		if as.missingStones {
			needLegend = true
			notes += "💎"
		}

		if as.farmPopulation < as.farmCapacity {
			needLegend = true
			notes += "🏠"
			if as.farmPopulation/as.farmCapacity < 0.95 {
				notes += "🐣"
			}
		} else if as.farmPopulation > as.farmCapacity {
			needLegend = true
			showGlitch = true
			notes += "🤥"
		}

		qStones := as.quantStones[ei.ArtifactSpec_INFERIOR] + as.quantStones[ei.ArtifactSpec_LESSER] + as.quantStones[ei.ArtifactSpec_NORMAL]
		tStones := as.tachStones[ei.ArtifactSpec_INFERIOR] + as.tachStones[ei.ArtifactSpec_LESSER] + as.tachStones[ei.ArtifactSpec_NORMAL]
		if as.quantWant != qStones || as.tachWant != tStones {
			notes += fmt.Sprintf("🧩%dT/%dQ", tStones, qStones)
			setContractEstimate = false
		}

		displayQ := fmt.Sprintf("%d", as.quantWant)
		paddingQ := bottools.StringAlignCenter
		if as.quantWant == qStones {
			displayQ += "√"
			paddingQ = bottools.StringAlignRight
		}

		displayT := fmt.Sprintf("%d", as.tachWant)
		paddingT := bottools.StringAlignCenter
		if as.tachWant == tStones {
			displayT += "√"
			paddingT = bottools.StringAlignRight
		}
		if stoneBonusIncrease != 1.05 {
			if notes != "" {
				notes += " "
			}
			//notes += fmt.Sprintf("(%1.2f^%d)", stoneBonusIncrease, as.stones)
		}

		if as.offline != "" {
			needLegend = true
			notes += as.offline
		}

		if soloName != "" {
			for _, d := range as.soloData {
				//log.Print(d)
				tableData = append(tableData, strings.Join(d, " "))
			}
		} else {

			lBestELR := fmt.Sprintf("%2.3f", as.bestELR)
			if as.bestELR < 1.0 {
				lBestELR = fmt.Sprintf("%2.2fT", as.bestELR*1000.0)
			}
			lBestSR := fmt.Sprintf("%2.3f", as.bestSR)
			if as.bestSR < 1.0 {
				lBestSR = fmt.Sprintf("%2.2fT", as.bestSR*1000.0)
			}

			// Build tile info
			var tileBuilder strings.Builder
			prefix := ">>> "
			for _, c := range as.artifactSlots {
				md := ei.GetBotEmojiMarkdown(c)
				if strings.Contains(md, "unknown") {
					md = ""
				}
				fmt.Fprintf(&tileBuilder, "%s%s", prefix, md)
				prefix = ""
			}
			fmt.Fprintf(&tileBuilder, "\n %s %s\n %s %s\n", ei.GetBotEmojiMarkdown("afx_tachyon_stone_4"), strings.Replace(displayT, "√", " ⭐️", 1), ei.GetBotEmojiMarkdown("afx_quantum_stone_4"), strings.Replace(displayQ, "√", " ⭐️", 1))
			fmt.Fprintf(&tileBuilder, "**ELR:** %2.3f\n**SR:** %2.3f\n", as.bestELR, as.bestSR)
			if len(as.collegg) > 0 {
				fmt.Fprintf(&tileBuilder, "🥚: %s\n", strings.Join(as.collegg, ","))
			}
			if len(notes) > 0 {
				for _, n := range strings.Split(notes, ",") {
					fmt.Fprintf(&tileBuilder, "%s\n", n)
				}
			}

			//url := bottools.GetStaabmiaLink(true, dimension, rate, int(everyoneDeflectorPercent), as.staabArtifacts, as.colleggSR)
			//link := fmt.Sprintf("[%s](%s)", ei.GetBotEmojiMarkdown("staab"), url)
			//link := fmt.Sprintf("[%s](%s)", "🌌", url)

			statsLine := []string{
				bottools.AlignString(truncateString(as.name, 12), 12, bottools.StringAlignRight),
				bottools.AlignString(displayT, 3, paddingT),
				bottools.AlignString(displayQ, 3, paddingQ),
			}

			if details {
				// Details
				statsLine = append(statsLine,
					bottools.AlignString(lBestELR, 6, bottools.StringAlignCenter),
					bottools.AlignString(lBestSR, 6, bottools.StringAlignCenter),
				)

				if !skipArtifact {
					statsLine = append(statsLine,
						bottools.AlignString(as.deflector.abbrev, 3, bottools.StringAlignRight),
						bottools.AlignString(as.metronome.abbrev, 3, bottools.StringAlignRight),
						bottools.AlignString(as.compass.abbrev, 3, bottools.StringAlignRight),
						bottools.AlignString(as.gusset.abbrev, 3, bottools.StringAlignRight),
					)
				}
			}

			if len(as.collegg) > 0 {
				statsLine = append(statsLine, bottools.AlignString(strings.Join(as.collegg, ",")+","+notes, 3, bottools.StringAlignLeft))
			} else {
				statsLine = append(statsLine, bottools.AlignString(notes, 3, bottools.StringAlignLeft))
			}

			tableData = append(tableData, strings.Join(statsLine, " "))

			if as.name != "[departed]" {
				url := bottools.GetStaabmiaLink(true, dimension, rate, int(everyoneDeflectorPercent), as.staabArtifacts, as.colleggSR, as.colleggELR)
				builderURL.WriteString(fmt.Sprintf("🔗[%s](%s)\n", as.nameRaw, url))
				fmt.Fprintf(&tileBuilder, "[%sCalc](%s)", ei.GetBotEmojiMarkdown("staab"), url)
			}

			safeName := as.nameRaw
			if strings.HasPrefix(safeName, ">") {
				safeName = "\\" + safeName
			}

			field = append(field, &discordgo.MessageEmbedField{
				Name:   safeName,
				Value:  tileBuilder.String(),
				Inline: true,
			})
		}
	}
	// Now add headers since we know if we have all the columns
	if soloName != "" {
		headerStr := []string{
			bottools.AlignString("Name", 12, bottools.StringAlignCenter),
			bottools.AlignString("T", 3, bottools.StringAlignCenter),
			bottools.AlignString("Q", 3, bottools.StringAlignCenter),
			bottools.AlignString("ELR", 6, bottools.StringAlignCenter),
			bottools.AlignString("SR", 6, bottools.StringAlignCenter),
			bottools.AlignString("Dfl", 3, bottools.StringAlignRight),
			bottools.AlignString("Met", 3, bottools.StringAlignRight),
			bottools.AlignString("Com", 3, bottools.StringAlignRight),
			bottools.AlignString("Gus", 3, bottools.StringAlignRight),
		}

		headerStr = append(headerStr, bottools.AlignString("📓", 3, bottools.StringAlignLeft))

		for i, h := range headerStr {
			headerStr[i] = strings.ToUpper(h)
		}
		tableHeader = strings.Join(headerStr, " ")
	} else {
		headerStr := []string{
			bottools.AlignString("Name", 12, bottools.StringAlignCenter),
			bottools.AlignString("T", 3, bottools.StringAlignCenter),
			bottools.AlignString("Q", 3, bottools.StringAlignCenter)}

		if details {
			headerStr = append(headerStr,
				bottools.AlignString("ELR", 6, bottools.StringAlignCenter),
				bottools.AlignString("SR", 6, bottools.StringAlignCenter))

			// Skip Artifacts
			if !skipArtifact {
				headerStr = append(headerStr,
					bottools.AlignString("Dfl", 3, bottools.StringAlignRight),
					bottools.AlignString("Met", 3, bottools.StringAlignRight),
					bottools.AlignString("Com", 3, bottools.StringAlignRight),
					bottools.AlignString("Gus", 3, bottools.StringAlignRight),
				)
			}

			// Show Colleggtibles only if there are anyone not at max
		}
		headerStr = append(headerStr, bottools.AlignString("📓", 3, bottools.StringAlignCenter))

		for i, h := range headerStr {
			headerStr[i] = strings.ToUpper(h)
		}
		tableHeader = strings.Join(headerStr, " ")
	}

	var contractDurationSeconds float64
	var calcSecondsRemaining float64

	fmt.Fprintf(&builder, "Stones Report for %s %s/[**%s**](%s)\n", ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)), contractID, coopID, fmt.Sprintf("%s/%s/%s", "https://eicoop-carpet.netlify.app", contractID, coopID))
	if eiContract.ID != "" {
		nowTime := time.Now()
		startTime := nowTime
		endTime := nowTime
		endStr := "End:"
		secondsRemaining := int64(coopStatus.GetSecondsRemaining())
		if coopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
			startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
			startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
			calcSecondsRemaining = -coopStatus.GetSecondsSinceAllGoalsAchieved()
			endTime = endTime.Add(time.Duration(calcSecondsRemaining) * time.Second)
			contractDurationSeconds = endTime.Sub(startTime).Seconds()
		} else {
			startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
			startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
			totalReq := eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1]
			calcSecondsRemaining = (totalReq - totalContributions) / contributionRatePerSecond
			endTime = nowTime.Add(time.Duration(calcSecondsRemaining) * time.Second)
			endStr = "Est End:"
			contractDurationSeconds = endTime.Sub(startTime).Seconds()
			if setContractEstimate {
				c := FindContract(contractID)
				if c != nil {
					c.EstimatedDuration = time.Duration(calcSecondsRemaining) * time.Second
					c.EstimatedDurationValid = true
					c.StartTime = startTime
					c.EstimatedEndTime = endTime
				}
			}
		}

		UpdateContractTime(coopStatus.GetContractIdentifier(), coopStatus.GetCoopIdentifier(), startTime, contractDurationSeconds)

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

	dropDeflector := getDeflectorDropPerc(artifactSets, everyoneDeflectorPercent)

	fmt.Fprintf(&builder, "Coop Deflector Bonus: %2.0f%%\n", everyoneDeflectorPercent)
	if dropDeflector > 0 {
		fmt.Fprintf(&builder, "Unused Deflector: %2.0f%%\n", dropDeflector)
	}
	if soloName == "" {
		fmt.Fprintf(&builder, "%s & %s show optimal quantity.\n", ei.GetBotEmojiMarkdown("afx_tachyon_stone_4"), ei.GetBotEmojiMarkdown("afx_quantum_stone_4"))
	} else {
		fmt.Fprint(&builder, "Showing all stone variations for solo report.\n")
	}
	if dataTimestampStr != "" {
		builder.WriteString(dataTimestampStr)
	}

	// Write out the table Header and Data
	builder.WriteString("```")
	builder.WriteString(fmt.Sprintf("`%s`\n", tableHeader))
	for _, d := range tableData {
		builder.WriteString(fmt.Sprintf("`%v`\n", d))
	}
	builder.WriteString("```")

	for _, as := range artifactSets {
		if len(as.note) > 0 {
			builder.WriteString(fmt.Sprintf("**%s** Notes: %s\n", as.name, strings.Join(as.note, ", ")))
		}
	}

	// Need to write out a legend for the stones
	if needLegend {
		habGlitch := ""
		if showGlitch {
			habGlitch = " / 🤥 HabGlitch"
		}
		builder.WriteString("√ Match / 🚩Research / 💎Missing / 🏠Filling(🐣CR) / 🧩Slotted / 🎣Away" + habGlitch + "\n")
	}
	builder.WriteString(fmt.Sprintf("Colleggtibles show when less than %s\n", strings.Join(colleggtibleStr, ", ")))

	return builder.String(), builderURL.String(), field
}

func truncateString(s string, length int) string {
	if len(s) > length {
		return s[:length]
	}
	return s
}

// getDeflectorDropPerc will calculate the deflector drop percentage based on the players' ELR/SR ratio
// From staabmia https://srsandbox-staabmia.netlify.app/scripts.js
func getDeflectorDropPerc(players []artifactSet, totDeflector float64) float64 {
	if len(players) < 2 {
		return totDeflector
	}
	elrDivSrMin := players[0].elr2 / players[0].sr
	def := (totDeflector-players[0].deflector.percent)/100 + 1

	for _, p := range players {
		currentPlayer := p.elr2 / p.sr
		if currentPlayer < elrDivSrMin {
			elrDivSrMin = currentPlayer
			def = (totDeflector-p.deflector.percent)/100 + 1
		}
	}

	if elrDivSrMin < 1 {
		return 0
	}

	val1 := (def-1)*100 - (def/elrDivSrMin-1)*100
	return math.Min(math.Floor(val1), math.Round(totDeflector))
}
