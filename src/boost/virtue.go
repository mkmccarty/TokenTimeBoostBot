package boost

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

type shipData struct {
	Name     string   `json:"Name"`
	Art      string   `json:"Art"`
	ArtDev   string   `json:"ArtDev"`
	Duration []string `json:"Duration"`
}

type missionData struct {
	Ships []shipData
}

const missionJSON = `{"ships":[
	{"name": "Chicken One","art":"<:chicken1:1280045945974951949>","artDev":"<:chicken1:1280390988824576061>","duration":["20m","1h","2h"]},
	{"name": "Chicken Nine","art":"<:chicken9:1280045842442616902>","artDev":"<:chicken9:1280390884575154226>","duration":["30m","1h","3h"]},
	{"name": "Chicken Heavy","art":"<:chickenheavy:1280045643922018315>","artDev":"<:chickenheavy:1280390782783590473>","duration":["45m","1h30m","4h"]},
	{"name": "BCR","art":"<:bcr:1280045542495228008>","artDev":"<:bcr:1280390686461661275>","duration":["1h30m","4h","8h"]},
	{"name": "Quintillion Chicken","art":"<:milleniumchicken:1280045411444326400>","artDev":"<:milleniumchicken:1280390575178383386>","duration":["3h","6h","12h"]},
	{"name": "Cornish-Hen Corvette","art":"<:corellihencorvette:1280045137518657536>","artDev":"<:corellihencorvette:1280390458983452742>","duration":["4h","12h","1d"]},
	{"name": "Galeggtica","art":"<:galeggtica:1280045010917527593>","artDev":"<:galeggtica:1280390347825872916>","duration":["6h","16h","1d6h"]},
	{"name": "Defihent","art":"<:defihent:1280044758001258577>","artDev":"<:defihent:1280390249943666739>","duration":["8h","1d","2d"]},
	{"name": "Voyegger","art":"<:voyegger:1280041822416273420>","artDev":"<:voyegger:1280390114094354472>","duration":["12h","1d12h","3d"]},
	{"name": "Henerprise","art":"<:henerprise:1280038539328749609>","artDev":"<:henerprise:1280390026487664704>","duration":["1d","2d","4d"]},
	{"name": "Atreggies Henliner","art":"<:atreggies:1280038674398183464>","artDev":"<:atreggies:1280389911509340240>","duration":["2d","3d","4d"]}
	]}`

var missionArt missionData

func init() {
	_ = json.Unmarshal([]byte(missionJSON), &missionArt)
}

// GetSlashVirtueCommand returns the command for the /launch-helper command
func GetSlashVirtueCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Evaluate virtue farm and provide detiled EoV overview.",
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
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "reset",
				Description: "Reset stored EI number",
				Required:    false,
			},
		},
	}
}

// HandleVirtue handles the /virtue command
func HandleVirtue(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	percent := -1

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["reset"]; ok {
		if opt.BoolValue() {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
		}
	}

	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")

	Virtue(s, i, percent, eiID, true)
}

// Virtue processes the virtue command
func Virtue(s *discordgo.Session, i *discordgo.InteractionCreate, percent int, eiID string, okayToSave bool) {
	var components []discordgo.MessageComponent

	// Get the Egg Inc ID from the stored settings
	eggIncID := ""
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		decodedData, err := base64.StdEncoding.DecodeString(eiID)
		if err == nil {
			decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
			if err == nil {
				eggIncID = string(decryptedData)
			}
		}
	}
	if eggIncID == "" || len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		RequestEggIncIDModal(s, i, fmt.Sprintf("virtue#%d", 0))
		return
	}

	// Quick reply to buy us some time
	flags := discordgo.MessageFlagsIsComponentsV2

	if i.ChannelID == "571836573243539476" { // ACO- #bot-commands
		flags += discordgo.MessageFlagsEphemeral
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

	userID := bottools.GetInteractionUserID(i)

	backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, userID, okayToSave)

	if backup != nil {
		farmerName := farmerstate.GetMiscSettingString(userID, "ei_ign")
		if farmerName != backup.GetUserName() {
			farmerName = backup.GetUserName()
			farmerstate.SetMiscSettingString(userID, "ei_ign", farmerName)
		}
	}

	farm := backup.GetFarms()[0]
	if farm != nil {
		farmType := farm.GetFarmType()
		if farmType == ei.FarmType_HOME {
			components = printVirtue(backup)
		}
	}
	if len(components) == 0 {
		components = append(components, &discordgo.TextDisplay{
			Content: "Your home farm isn't currently producing Eggs of Virtue. Switch to an Egg of Virtue on your home farm to see this information.",
		})
	}
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      flags,
		Components: components,
	})

}

func printVirtue(backup *ei.Backup) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent
	divider := true
	spacing := discordgo.SeparatorSpacingSizeSmall

	farm := backup.GetFarms()[0]
	eggType := farm.GetEggType()
	virtue := backup.GetVirtue()
	//pe := backup.GetGame().GetEggsOfProphecy()
	se := backup.GetGame().GetSoulEggsD()
	if virtue == nil {
		components = append(components, &discordgo.TextDisplay{
			Content: "No virtue backup data found in Egg Inc API response",
		})
		return components
	}
	header := strings.Builder{}
	eggs := strings.Builder{}
	stats := strings.Builder{}
	rockets := strings.Builder{}
	footer := strings.Builder{}

	var onVirtueFarm = false
	if eggType >= ei.Egg(int(ei.Egg_CURIOSITY)) && eggType <= ei.Egg(int(ei.Egg_KINDNESS)) {
		onVirtueFarm = true
	}

	shiftCost := getShiftCost(virtue.GetShiftCount(), se)

	fmt.Fprintf(&header, "# Eggs of Virtue Helper\n")
	fmt.Fprintf(&header, "**Resets**: %d  **Shifts**: %d  %s%s\n",
		virtue.GetResets(),
		virtue.GetShiftCount(),
		ei.GetBotEmojiMarkdown("egg_soul"),
		ei.FormatEIValue(shiftCost, map[string]interface{}{"decimals": 3, "trim": true}))
	// Ship icon uses last fueled ship art
	lastFueled := virtue.GetAfx().GetLastFueledShip()
	craftArt := missionArt.Ships[lastFueled].Art
	if config.IsDevBot() {
		craftArt = missionArt.Ships[lastFueled].ArtDev
	}

	//fleetSize, trainLength := ei.GetFleetSize(farm.GetCommonResearch())
	habArt, habArray := getHabIconStrings(farm.GetHabs(), ei.GetBotEmojiMarkdown)
	VehicleArt, VehicleArray := getVehicleIconStrings(farm.GetVehicles(), farm.GetTrainLength(), ei.GetBotEmojiMarkdown)

	DepotArt := ei.GetBotEmojiMarkdown("depot")
	//fmt.Fprintf(&builder, "Inventory Score %.0f\n", virtue.GetAfx().GetInventoryScore())
	virtueEggs := []string{"CURIOSITY", "INTEGRITY", "HUMILITY", "RESILIENCE", "KINDNESS"}
	eggEffects := []string{"🔬", habArt, craftArt, ei.GetBotEmojiMarkdown("silo"), VehicleArt}
	// Use highest Hab for Hab emoji

	var allEov uint32 = 0
	var futureEov uint32 = 0

	selectedTarget := 0.0
	selectedDelivered := 0.0
	selectedEggIndex := -1
	selectedEggEmote := ""

	for i, egg := range virtueEggs {
		eov := virtue.GetEovEarned()[i] // Assuming Eggs is the correct field for accessing egg virtues
		delivered := virtue.GetEggsDelivered()[i]

		eovEarned := countTETiersPassed(delivered)
		// pendingTruthEggs calculates the number of pending Truth Eggs based on delivered and earnedTE.
		eovPending := pendingTruthEggs(delivered, eov)
		nextTier := nextTruthEggThreshold(delivered, eov)
		selected := ""
		if eggType == ei.Egg(int(ei.Egg_CURIOSITY)+i) {
			selected = " (farm)"
			selectedEggIndex = i
			selectedTarget = nextTier
			selectedDelivered = delivered
			selectedEggEmote = ei.GetBotEmojiMarkdown("egg_" + strings.ToLower(egg))
		}

		allEov += max(eovEarned-eovPending, 0)
		futureEov += eovPending

		fmt.Fprintf(&eggs, "%s%s`%3s %5s %9s `%s%s\n",
			bottools.AlignString(ei.GetBotEmojiMarkdown("egg_"+strings.ToLower(egg)), 1, bottools.StringAlignCenter),
			bottools.AlignString(eggEffects[i], 1, bottools.StringAlignCenter),
			bottools.AlignString(fmt.Sprintf("%d", eovEarned-eovPending), 3, bottools.StringAlignRight),
			bottools.AlignString(fmt.Sprintf("(%d)", eovPending), 5, bottools.StringAlignLeft),
			bottools.AlignString(fmt.Sprintf("🥚 %s", ei.FormatEIValue(delivered, map[string]interface{}{"decimals": 1, "trim": false})), 9, bottools.StringAlignLeft),
			bottools.AlignString(fmt.Sprintf("%s%s", ei.GetBotEmojiMarkdown("egg_truth"), ei.FormatEIValue(nextTier, map[string]interface{}{"decimals": 1, "trim": false})), 1, bottools.StringAlignLeft),
			bottools.AlignString(selected, 1, bottools.StringAlignLeft),
		)
	}

	eb := ei.GetEarningsBonus(backup, float64(allEov))
	ebFuture := ei.GetEarningsBonus(backup, float64(allEov+futureEov))
	fmt.Fprintf(&header, "**PE**: %d  **SE**: %s  **TE**: %d  (+%d)\n",
		backup.GetGame().GetEggsOfProphecy(),
		ei.FormatEIValue(backup.GetGame().GetSoulEggsD(), map[string]interface{}{"decimals": 3, "trim": true}),
		allEov,
		futureEov)

	fmt.Fprintf(&header, "**EB**: %s%%  (+%s%%) ->  **%s%%**\n",
		ei.FormatEIValue(eb, map[string]interface{}{"decimals": 3, "trim": true}),
		ei.FormatEIValue(ebFuture-eb, map[string]interface{}{"decimals": 2, "trim": true}),
		ei.FormatEIValue(ebFuture, map[string]interface{}{"decimals": 3, "trim": true}),
	)

	// What are my artifacts.
	virtueArtifactDB := backup.ArtifactsDb.GetVirtueAfxDb()
	virtueArtifacts := virtueArtifactDB.GetInventoryItems()
	activeAfx := virtueArtifactDB.GetActiveArtifacts()
	virtueSlots := activeAfx.GetSlots()
	inUseArtifacts := []uint64{}
	for _, slot := range virtueSlots {
		if slot.GetOccupied() {
			artifactID := slot.GetItemId()
			inUseArtifacts = append(inUseArtifacts, artifactID)
		}
	}
	artifactSetInUse := []*ei.CompleteArtifact{}

	artifactIcons := ""

	for _, artifact := range virtueArtifacts {
		artifactID := artifact.GetItemId()
		if slices.Contains(inUseArtifacts, artifactID) {
			artifactSetInUse = append(artifactSetInUse, artifact.GetArtifact())

			spec := artifact.GetArtifact().GetSpec()
			strType := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
			artifactIcons += ei.GetBotEmojiMarkdown(fmt.Sprintf("%s%s", ei.ShortArtifactName[int32(spec.GetName())], strType))
		}
	}

	artifactBuffs := ei.GetArtifactBuffs(artifactSetInUse)

	// Get Colleggtible Buffs
	contracts := backup.GetContracts()
	colBuffs := ei.GetColleggtibleBuffs(contracts)

	shippingRate := ei.GetShippingRateFromBackup(farm, backup.GetGame())
	eggLayingRate, habPop, habCap := ei.GetEggLayingRateFromBackup(farm, backup.GetGame())
	//deliveryRate := math.Min(eggLayingRate, shippingRate)
	eggLayingRate *= artifactBuffs.ELR * colBuffs.ELR
	shippingRate *= artifactBuffs.SR * colBuffs.SR
	habCap *= artifactBuffs.Hab * colBuffs.Hab

	_, onlineRate, _, offlineRate := ei.GetInternalHatcheryFromBackup(farm.GetCommonResearch(), backup.GetGame(), artifactBuffs.IHR*colBuffs.IHR, allEov)
	siloMinutes := ei.GetSiloMinutes(farm, backup.GetGame().GetEpicResearch())

	fuelingEnabled := virtue.GetAfx().GetTankFillingEnabled()
	fuelRate := 0.0
	fuelPercentage := virtue.GetAfx().GetFlowPercentageArtifacts()
	recommendedFuelRate := 0.0
	if fuelingEnabled {
		fuelRate = eggLayingRate * fuelPercentage
	}
	fuelingTiers := []float64{0.1, 0.5, 0.9}
	// Check if IHR is high enough over Shipping Rate to enable fueling
	for _, tier := range fuelingTiers {
		if eggLayingRate*(1.0-tier) > shippingRate {
			recommendedFuelRate = tier
		}
	}

	// Now with our rates we can figure out earnings numbers
	_, offlineRateHr := ei.GetFarmEarningRates(backup, math.Min(shippingRate, eggLayingRate-fuelRate), artifactBuffs, colBuffs, allEov)

	// Handle tank limits if on virtue farm and fueling
	if selectedEggIndex != -1 {
		tankLevels := []float64{2e9, 200e9, 10e12, 100e12, 200e12, 300e12, 400e12, 500e12}
		fuelingTankLevel := backup.GetArtifacts().GetTankLevel()
		fuelLimits := virtue.GetAfx().GetTankLimits()
		tankLimit := tankLevels[fuelingTankLevel]

		// Only use the last 5 elements of fuelLimits and tankLevels
		if len(fuelLimits) > 5 {
			fuelLimits = fuelLimits[len(fuelLimits)-5:]
		}
		fuels := virtue.GetAfx().GetTankFuels()
		if len(fuels) > 5 {
			fuels = fuels[len(fuels)-5:]
		}
		totalFuel := 0.0
		for _, fuel := range fuels {
			totalFuel += fuel
		}
		maxFill := tankLimit * fuelLimits[selectedEggIndex]

		if selectedEggIndex >= 0 && selectedEggIndex < len(fuels) {
			fuelQuantity := fuels[selectedEggIndex]
			if fuelQuantity >= maxFill || totalFuel >= tankLimit {
				fuelingEnabled = false
				recommendedFuelRate = 0.0
				fuelRate = 0.0
			}
		}

	}

	habPercent := 0.0
	if habCap > 0 {
		habPercent = (habPop / habCap) * 100
	}
	onlineFillTime := ei.TimeForLinearGrowth(habPop, habCap, onlineRate/60)
	offlineFillTime := ei.TimeForLinearGrowth(habPop, habCap, offlineRate/60)
	syncTime := time.Unix(int64(backup.GetApproxTime()), 0)
	elapsed := time.Since(syncTime).Seconds()
	remainingTime := ei.TimeToDeliverEggs(habPop, habCap, offlineRate, eggLayingRate-fuelRate, shippingRate, selectedTarget-selectedDelivered)
	adjustedRemainingTime := remainingTime - elapsed
	offlineEggs := min(eggLayingRate, shippingRate) * (elapsed / 3600)

	if onVirtueFarm {
		fmt.Fprintf(&stats, "%s %s\n", VehicleArray, strings.Join(habArray, ""))

		// Want time from now when those minutes elapse
		if shippingRate > eggLayingRate {
			fmt.Fprintf(&stats, "%s %s/hr  %s **%s/hr**",
				VehicleArt,
				ei.FormatEIValue(shippingRate, map[string]interface{}{"decimals": 2, "trim": true}),
				selectedEggEmote,
				ei.FormatEIValue(eggLayingRate-fuelRate, map[string]interface{}{"decimals": 2, "trim": true}))
		} else {
			fmt.Fprintf(&stats, "%s **%s/hr**  %s %s/hr",
				VehicleArt,
				ei.FormatEIValue(shippingRate, map[string]interface{}{"decimals": 2, "trim": true}),
				selectedEggEmote,
				ei.FormatEIValue(eggLayingRate-fuelRate, map[string]interface{}{"decimals": 2, "trim": true}))
		}
		if fuelingEnabled {
			fuelLamp := ""
			if fuelPercentage == 1.0 {
				fuelLamp = "🚨"
			}
			if recommendedFuelRate > fuelPercentage {
				fuelLamp = "🎚️"
			}
			if shippingRate > eggLayingRate {
				fmt.Fprintf(&stats, " %s%s **%s/hr**\n",
					DepotArt,
					fuelLamp,
					ei.FormatEIValue(fuelRate, map[string]interface{}{"decimals": 2, "trim": true}))
			} else {
				fmt.Fprintf(&stats, " %s%s %s/hr\n",
					DepotArt,
					fuelLamp,
					ei.FormatEIValue(fuelRate, map[string]interface{}{"decimals": 2, "trim": true}))
			}
		} else if recommendedFuelRate > 0.0 {
			fmt.Fprintf(&stats, " %s🎚️ %.0f%%\n", DepotArt, recommendedFuelRate*100)
		} else {
			fmt.Fprint(&stats, "\n")
		}

		fmt.Fprintf(&stats, "**IHR** %s/min  💤 %s/min  %s %s\n",
			ei.FormatEIValue(onlineRate, map[string]interface{}{"decimals": 2, "trim": true}),
			ei.FormatEIValue(offlineRate, map[string]interface{}{"decimals": 2, "trim": true}),
			ei.GetBotEmojiMarkdown("silo"),
			bottools.FmtDuration(time.Duration(siloMinutes)*time.Minute),
		)

		if habPop >= habCap || habPercent >= 99.9 {
			fmt.Fprintf(&stats, "%s %d%% %s ⚠️🔒\n",
				//strings.Join(habArray, ""),
				habArt,
				int(habPercent),
				ei.FormatEIValue(habPop, map[string]interface{}{"decimals": 2, "trim": true}))
		} else {
			fmt.Fprintf(&stats, "%s %s %d%% 🔒<t:%d:R> or 💤<t:%d:R>\n",
				//strings.Join(habArray, ""),
				habArt,
				ei.FormatEIValue(habPop, map[string]interface{}{"decimals": 2, "trim": true}),
				int(habPercent),
				time.Now().Add(time.Duration(int64(onlineFillTime))*time.Second).Unix(),
				time.Now().Add(time.Duration(int64(offlineFillTime))*time.Second).Unix())
		}
		// Loop to show time to next several Truth Egg thresholds
		loopCount := 0
		currentSelectedTarget := selectedTarget
		bold := "**"
		prefix := ""
		for {
			header.WriteString(bold)
			header.WriteString(prefix)
			if remainingTime == -1.0 {
				fmt.Fprintf(&header, "Deliver %s%s in more than a year 💤",
					ei.FormatEIValue(currentSelectedTarget, map[string]interface{}{"decimals": 1, "trim": true}),
					selectedEggEmote)
			} else if adjustedRemainingTime < 43200.0 { // 12 hours
				fmt.Fprintf(&header, "Deliver %s%s <t:%d:t>💤",
					ei.FormatEIValue(currentSelectedTarget, map[string]interface{}{"decimals": 1, "trim": true}),
					selectedEggEmote,
					time.Now().Add(time.Duration(int64(adjustedRemainingTime))*time.Second).Unix())
			} else {
				fmt.Fprintf(&header, "Deliver %s%s <t:%d:f>💤",
					ei.FormatEIValue(currentSelectedTarget, map[string]interface{}{"decimals": 1, "trim": true}),
					selectedEggEmote,
					time.Now().Add(time.Duration(int64(adjustedRemainingTime))*time.Second).Unix())
			}
			header.WriteString(bold)

			// Prepare for next threshold
			currentSelectedTarget = nextTruthEggThreshold(currentSelectedTarget, 0)
			if currentSelectedTarget == math.Inf(1) {
				break
			}
			remainingTime = ei.TimeToDeliverEggs(habPop, habCap, offlineRate, eggLayingRate-fuelRate, shippingRate, currentSelectedTarget-selectedDelivered)
			adjustedRemainingTime = remainingTime - elapsed

			loopCount++
			// Stop if remainingTime is -1 or adjustedRemainingTime is more than 2 weeks (1209600 seconds), or after 5 iterations to avoid infinite loop
			if remainingTime == -1.0 || (adjustedRemainingTime > 1209600 && loopCount > 1) || loopCount >= 8 {
				break
			}
			header.WriteString("\n")
			bold = ""
			prefix = "-# "
		}

		fmt.Fprintf(&header, "\n-# includes %s offline eggs", ei.FormatEIValue(offlineEggs, map[string]interface{}{"decimals": 3, "trim": true}))
	} else {
		fmt.Fprint(&header, "**Ascend to visit your Eggs of Virtue farm.**")
	}

	fmt.Fprintf(&stats, "%s **Offline** %s/hr  %s/s\n",
		ei.GetBotEmojiMarkdown("gem"),
		ei.FormatEIValue(offlineRateHr, map[string]interface{}{"decimals": 3, "trim": true}),
		ei.FormatEIValue(offlineRateHr/3600, map[string]interface{}{"decimals": 3, "trim": true}),
	)

	// If we have a selected egg type, show time to next TE
	if artifactIcons == "" {
		artifactIcons = "**Artifacts**"
	}

	fmt.Fprintf(&stats, "%s SR:%v%%  ELR:%v%%  IHR:%v%%  H:%v%% %s%v%% 💤%v%%\n",
		artifactIcons,
		math.Round((artifactBuffs.SR-1)*100),
		math.Round((artifactBuffs.ELR-1)*100),
		math.Round((artifactBuffs.IHR-1)*100),
		math.Round((artifactBuffs.Hab-1)*100),
		ei.GetBotEmojiMarkdown("gem"),
		math.Round((artifactBuffs.Earnings-1)*100),
		math.Round((artifactBuffs.AwayEarnings-1)*100),
	)
	fmt.Fprintf(&stats, "%s  SR:%v%%  ELR:%v%%  IHR:%v%%  H:%v%% %s%v%% 💤%v%%\n",
		ei.GetBotEmojiMarkdown("collegg"),
		math.Round((colBuffs.SR-1)*100),
		math.Round((colBuffs.ELR-1)*100),
		math.Round((colBuffs.IHR-1)*100),
		math.Round((colBuffs.Hab-1)*100),
		ei.GetBotEmojiMarkdown("gem"),
		math.Round((colBuffs.Earnings-1)*100),
		math.Round((colBuffs.AwayEarnings-1)*100),
	)

	fmt.Fprintf(&footer, "-# Report run <t:%d:t>, last sync <t:%d:t>\n", time.Now().Unix(), syncTime.Unix())

	// Line for fuel
	fuels := virtue.GetAfx().GetTankFuels()
	fuels = fuels[len(fuels)-5:]
	fmt.Fprintf(&rockets, "\n%s", DepotArt)
	for i, fuel := range fuels {
		fmt.Fprintf(&rockets, " %s:%s",
			ei.GetBotEmojiMarkdown("egg_"+strings.ToLower(virtueEggs[i])),
			ei.FormatEIValue(fuel, map[string]interface{}{"decimals": 1, "trim": true}))
	}
	rockets.WriteString("\n")
	artifacts := backup.GetArtifactsDb()
	missions := artifacts.GetMissionInfos()
	for _, mission := range missions {
		missionType := mission.GetType()
		//missionStatus := mission.GetStatus()
		if missionType == ei.MissionInfo_VIRTUE {
			shipType := mission.GetShip()
			craft := missionArt.Ships[shipType]
			art := craft.Art
			if config.IsDevBot() {
				art = craft.ArtDev
			}
			missionEnd := uint32(mission.GetStartTimeDerived()) + uint32(mission.GetDurationSeconds())
			fmt.Fprintf(&rockets, "%s <t:%d:R> \n", art, missionEnd)
		}
	}

	components = append(components, &discordgo.Section{
		Components: []discordgo.MessageComponent{
			&discordgo.TextDisplay{
				Content: header.String(),
			},
		},
		Accessory: &discordgo.Thumbnail{
			Media: discordgo.UnfurledMediaItem{
				URL: "https://cdn.discordapp.com/emojis/1418022084205875210.webp?size=128",
			},
		},
	})
	components = append(components, &discordgo.Separator{
		Divider: &divider,
		Spacing: &spacing,
	})
	components = append(components, &discordgo.TextDisplay{
		Content: eggs.String(),
	})

	components = append(components, &discordgo.TextDisplay{
		Content: stats.String(),
	})
	components = append(components, &discordgo.Separator{
		Divider: &divider,
		Spacing: &spacing,
	})

	components = append(components, &discordgo.TextDisplay{
		Content: rockets.String(),
	})
	components = append(components, &discordgo.TextDisplay{
		Content: footer.String(),
	})

	return components
}

func getShiftCost(shiftCount uint32, soulEggs float64) float64 {
	X := float64(soulEggs) * (0.02*math.Pow(float64(shiftCount)/120, 3) + 0.0001)
	C := math.Pow(10, 11) + 0.6*X + math.Pow(0.4*X, 0.9)

	return C
}

func init() {
	// Build a Breakpoint table for Truth Eggs
	// Used for calculating required eggs laid for TE above 16.
	// a_m = 1e17 + (m-17)*5e16 + ((m-17)*(m-18)/2)*1e16

	for m := 17; m <= 98; m++ {
		am := 1e17 + float64(m-17)*5e16 + float64((m-17)*(m-18)/2)*1e16
		TruthEggBreakpoints = append(TruthEggBreakpoints, am)
	}
}

// TruthEggBreakpoints is a slice containing all known tiers to 16 TE
var TruthEggBreakpoints = []float64{
	5e7,    // 50M
	1e9,    // 1B
	1e10,   // 10B
	7e10,   // 70B
	5e11,   // 500B
	2e12,   // 2T
	7e12,   // 7T
	2e13,   // 20T
	6e13,   // 60T
	1.5e14, // 150T
	5e14,   // 500T
	1.5e15, // 1.5q
	4e15,   // 4q
	1e16,   // 10q
	2.5e16, // 25q
	5e16,   // 50q
}

// countTETiersPassed returns the number of TE tiers passed for a given delivered value.
func countTETiersPassed(delivered float64) uint32 {
	i := 0
	for i < len(TruthEggBreakpoints) && delivered >= TruthEggBreakpoints[i] {
		i++
	}
	return uint32(i)
}

// pendingTruthEggs calculates the number of pending Truth Eggs for a given delivered value and earned Truth Eggs.
func pendingTruthEggs(delivered float64, earnedTE uint32) uint32 {
	tiersPassed := countTETiersPassed(delivered)
	if tiersPassed <= earnedTE {
		return 0
	}
	return tiersPassed - earnedTE
}

// nextTruthEggThreshold returns the next Truth Egg threshold for a given delivered value.
// If all tiers are passed, it returns math.Inf(1).
func nextTruthEggThreshold(delivered float64, eov uint32) float64 {
	tiersPassed := countTETiersPassed(delivered)
	if tiersPassed != 0 && tiersPassed < eov {
		tiersPassed = eov
	}
	if int(tiersPassed) >= len(TruthEggBreakpoints) {
		return math.Inf(1)
	}
	return TruthEggBreakpoints[tiersPassed]
}

// Return highest hab icon and array of hab icons
func getHabIconStrings(habs []uint32, getBotEmojiMarkdown func(string) string) (string, []string) {
	highestHab := 1
	var habArray []string
	for _, h := range habs {
		id := h // h is already a uint32 representing the habitat ID
		if int(id) > highestHab && int(id) < 19 {
			highestHab = int(id + 1)
		}
		if int(id) < 19 {
			habArray = append(habArray, getBotEmojiMarkdown(fmt.Sprintf("hab%d", id+1)))
		}
	}
	habArt := getBotEmojiMarkdown(fmt.Sprintf("hab%d", highestHab))
	return habArt, habArray
}

// Return highest vehicle icon and array of vehicle icons
func getVehicleIconStrings(vehicles []uint32, trainLength []uint32, getBotEmojiMarkdown func(string) string) (string, string) {
	highestVehicle := 0
	for _, v := range vehicles {
		id := v // v is already a uint32 representing the vehicle ID
		if int(id) > highestVehicle {
			highestVehicle = int(id)
		}
	}
	VehicleArt := getBotEmojiMarkdown(fmt.Sprintf("veh%d", highestVehicle))
	vehicleCounts := make(map[int]int)
	for i, v := range vehicles {
		id := int(v) // v is already a uint32 representing the vehicle ID
		if id == 11 {
			// need to check the train cars
			trainCarCount := int(trainLength[i])
			vehicleCounts[id*100+trainCarCount]++
		} else {
			vehicleCounts[id*100]++
		}
	}
	// Sort vehicle IDs
	var vehicleIDs []int
	for id := range vehicleCounts {
		vehicleIDs = append(vehicleIDs, id)
	}
	slices.Sort(vehicleIDs)
	var vehicleArtParts []string
	trainCar := getBotEmojiMarkdown("tl")
	for _, id := range vehicleIDs {
		count := vehicleCounts[id]
		trainCount := id % 100
		part := getBotEmojiMarkdown(fmt.Sprintf("veh%d", id/100))
		if count > 1 {
			if id/100 == 11 && trainCount > 1 {
				part += fmt.Sprintf("%sx%d", strings.Repeat(trainCar, trainCount), count)
			} else {
				part += fmt.Sprintf("x%d", count)
			}
		} else if trainCount > 1 {
			part += strings.Repeat(trainCar, trainCount)
		}
		vehicleArtParts = append(vehicleArtParts, part)
	}
	VehicleArray := strings.Join(vehicleArtParts, "")
	return VehicleArt, VehicleArray
}
