package boost

import (
	"encoding/base64"
	"fmt"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/bwmarrin/discordgo"
)

// GetSlashVirtueCommand returns the command for the /launch-helper command
func GetSlashVirtueCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Evaluate virtue farm and provide detailed EoV overview.",
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
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "simulate-shift",
				Description: "What does a 0 pop shift look like for this egg?",
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Curiosity",
						Value: 50,
					},
					{
						Name:  "Integrity",
						Value: 51,
					},
					{
						Name:  "Humility",
						Value: 52,
					},
					{
						Name:  "Resilience",
						Value: 53,
					},
					{
						Name:  "Kindness",
						Value: 54,
					},
				},
				Required: false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "simulate-shift-target-te",
				Description: "Target Truth Eggs for simulated shift (requires simulate-shift).",
				MinValue:    func() *float64 { v := 1.0; return &v }(), // Why a pointer??
				MaxValue:    98.0,
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "reset",
				Description: "Reset stored EI number",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "compact",
				Description: "Compact display (sticky)",
				Required:    false,
			},
		},
	}
}

// HandleVirtue handles the /virtue command
func HandleVirtue(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)

	optionMap := bottools.GetCommandOptionsMap(i)
	if opt, ok := optionMap["reset"]; ok {
		if opt.BoolValue() {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
		}
	}

	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")

	Virtue(s, i, optionMap, eiID, true)
}

// Virtue processes the virtue command
func Virtue(s *discordgo.Session, i *discordgo.InteractionCreate, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption, eiID string, okayToSave bool) {
	userID := bottools.GetInteractionUserID(i)
	simulatedEgg := ei.Egg(-1)
	var components []discordgo.MessageComponent

	var targetTE uint64 = 0
	if opt, ok := optionMap["simulate-shift"]; ok {
		simulatedEgg = ei.Egg(opt.IntValue())
		if opt2, ok2 := optionMap["simulate-shift-target-te"]; ok2 {
			targetTE = opt2.UintValue()
		}
	}
	compact := false
	if opt, ok := optionMap["compact"]; ok {
		compact = opt.BoolValue()
		farmerstate.SetMiscSettingString(userID, "virtueCompactMode", fmt.Sprintf("%t", compact))
	} else {
		savedCompact := farmerstate.GetMiscSettingString(userID, "virtueCompactMode")
		if savedCompact != "" {
			compact, _ = strconv.ParseBool(savedCompact)
		}
	}

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
		RequestEggIncIDModal(s, i, "virtue", optionMap)
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
			components = printVirtue(userID, backup, simulatedEgg, targetTE, compact)
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

func printVirtue(userID string, backup *ei.Backup, simulatedEgg ei.Egg, targetTE uint64, compact bool) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent
	divider := true
	spacing := discordgo.SeparatorSpacingSizeSmall
	virtueEggs := []string{"CURIOSITY", "INTEGRITY", "HUMILITY", "RESILIENCE", "KINDNESS"}

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
	notes := strings.Builder{}
	rockets := strings.Builder{}
	footer := strings.Builder{}

	var onVirtueFarm = false
	if eggType >= ei.Egg(int(ei.Egg_CURIOSITY)) && eggType <= ei.Egg(int(ei.Egg_KINDNESS)) {
		onVirtueFarm = true
	}

	shiftCost := getShiftCost(virtue.GetShiftCount(), se)

	fmt.Fprint(&header, "# Eggs of Virtue Helper\n")
	if onVirtueFarm {
		fmt.Fprintf(&header, "**__%s the Ascender__**\n", backup.GetUserName())
	} else {
		fmt.Fprintf(&header, "**__%s, the Prestiged One__**\n", backup.GetUserName())
	}
	fmt.Fprintf(&header, "**Resets**: %d  **Shifts**: %d  %s%s\n",
		virtue.GetResets(),
		virtue.GetShiftCount(),
		ei.GetBotEmojiMarkdown("egg_soul"),
		ei.FormatEIValue(shiftCost, map[string]any{"decimals": 3, "trim": true}))
	// Ship icon uses last fueled ship art
	lastFueled := virtue.GetAfx().GetLastFueledShip()
	craftArt := ei.GetBotEmojiMarkdown(ei.MissionArt.Ships[lastFueled].Art)

	// print the fleet size and train length
	habArt, habArray := getHabIconStrings(farm.GetHabs(), ei.GetBotEmojiMarkdown)
	VehicleArt, VehicleArray := getVehicleIconStrings(farm.GetVehicles(), farm.GetTrainLength(), ei.GetBotEmojiMarkdown)

	DepotArt := ei.GetBotEmojiMarkdown("depot")
	//fmt.Fprintf(&builder, "Inventory Score %.0f\n", virtue.GetAfx().GetInventoryScore())
	eggEffects := []string{"🔬", habArt, craftArt, ei.GetBotEmojiMarkdown("silo"), VehicleArt}
	// Use highest Hab for Hab emoji

	var allEov uint32 = 0
	var futureEov uint32 = 0

	selectedTarget := 0.0
	selectedDelivered := 0.0
	selectedEggIndex := -1
	selectedEggEmote := ""

	// Names could use a rework but current egg means the current EoV egg on the farm
	currentEggTarget := 0.0
	currentDelivered := 0.0
	currentEggIndex := -1
	currentEggEmote := ""

	for i, egg := range virtueEggs {
		eov := virtue.GetEovEarned()[i] // Assuming Eggs is the correct field for accessing egg virtues
		delivered := virtue.GetEggsDelivered()[i]

		eovEarned := ei.CountTruthEggTiersPassed(delivered)
		// pendingTruthEggs calculates the number of pending Truth Eggs based on delivered and earnedTE.
		eovPending := ei.PendingTruthEggs(delivered, eov)
		nextTier := ei.NextTruthEggThreshold(delivered, eov)
		selected := ""
		if simulatedEgg != -1 {
			if simulatedEgg == ei.Egg(int(ei.Egg_CURIOSITY)+i) {
				selected = " (simulated)"
				selectedEggIndex = i
				selectedTarget = nextTier
				selectedDelivered = delivered
				selectedEggEmote = ei.GetBotEmojiMarkdown("egg_" + strings.ToLower(egg))
			}
			if targetTE != 0 && eggType == ei.Egg(int(ei.Egg_CURIOSITY)+i) {
				currentEggIndex = i
				currentDelivered = delivered
				currentEggEmote = ei.GetBotEmojiMarkdown("egg_" + strings.ToLower(egg))
			}
		} else {
			if eggType == ei.Egg(int(ei.Egg_CURIOSITY)+i) {
				selected = " (farm)"
				selectedEggIndex = i
				selectedTarget = nextTier
				selectedDelivered = delivered
				selectedEggEmote = ei.GetBotEmojiMarkdown("egg_" + strings.ToLower(egg))
			}
		}

		allEov += max(eovEarned-eovPending, 0)
		futureEov += eovPending
		farmerstate.SetMiscSettingString(userID, "TE", fmt.Sprintf("%d", allEov))

		fmt.Fprintf(&eggs, "%s%s`%3s %5s %9s `%s%s\n",
			bottools.AlignString(ei.GetBotEmojiMarkdown("egg_"+strings.ToLower(egg)), 1, bottools.StringAlignCenter),
			bottools.AlignString(eggEffects[i], 1, bottools.StringAlignCenter),
			bottools.AlignString(fmt.Sprintf("%d", eovEarned-eovPending), 3, bottools.StringAlignRight),
			bottools.AlignString(fmt.Sprintf("(%d)", eovPending), 5, bottools.StringAlignLeft),
			bottools.AlignString(fmt.Sprintf("🥚 %s", ei.FormatEIValue(delivered, map[string]any{"decimals": 1, "trim": false})), 9, bottools.StringAlignLeft),
			bottools.AlignString(fmt.Sprintf("%s%s", ei.GetBotEmojiMarkdown("egg_truth"), ei.FormatEIValue(nextTier, map[string]any{"decimals": 1, "trim": false})), 1, bottools.StringAlignLeft),
			bottools.AlignString(selected, 1, bottools.StringAlignLeft),
		)
	}

	eb := ei.GetEarningsBonus(backup, float64(allEov))
	ebFuture := ei.GetEarningsBonus(backup, float64(allEov+futureEov))
	fmt.Fprintf(&header, "**PE**: %d  **SE**: %s  **TE**: %d  (+%d)\n",
		backup.GetGame().GetEggsOfProphecy(),
		ei.FormatEIValue(backup.GetGame().GetSoulEggsD(), map[string]any{"decimals": 3, "trim": true}),
		allEov,
		futureEov)

	fmt.Fprintf(&header, "**EB**: %s%%  (+%s%%) ->  **%s%%**\n",
		ei.FormatEIValue(eb, map[string]any{"decimals": 3, "trim": true}),
		ei.FormatEIValue(ebFuture-eb, map[string]any{"decimals": 2, "trim": true}),
		ei.FormatEIValue(ebFuture, map[string]any{"decimals": 3, "trim": true}),
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

	artifactDB := backup.GetArtifactsDb()

	/*
		if config.IsDevBot() {
			artifacts := artifactDb.GetInventoryItems()
			log.Println("All Artifacts:")
			ei.ExamineArtifacts(artifacts)
			log.Println("Virtue Artifacts:")
			ei.ExamineArtifacts(virtueArtifacts)
		}
	*/

	cte := ei.CalculateClothedTE(backup)
	maxCTEResult := ei.CalculateMaxClothedTEWithSlotHint(backup, len(inUseArtifacts))
	maxCTE := maxCTEResult.ClothedTE
	cteDelta := maxCTE - cte
	if config.IsDevBot() {
		log.Printf("Calculated Clothed TE: %f, Max Clothed TE: %f\n", cte, maxCTE)
		for _, line := range ei.DescribeArtifactSetWithStones(maxCTEResult.Artifacts) {
			log.Printf("Max CTE set: %s\n", line)
		}
	}
	fmt.Fprintf(&header, "**CTE**: %.0f  **Max CTE**: %.0f\n", cte, maxCTE)
	artifactIcons := ""
	maxArtifactIcons := ""

	for _, artifact := range virtueArtifacts {
		artifactID := artifact.GetItemId()
		// Ensure artifactIcons follow the order in inUseArtifacts
		_ = artifactID
		if len(artifactSetInUse) == 0 && len(inUseArtifacts) > 0 {
			itemsByID := make(map[uint64]*ei.CompleteArtifact, len(virtueArtifacts))
			for _, it := range virtueArtifacts {
				itemsByID[it.GetItemId()] = it.GetArtifact()
			}
			for _, id := range inUseArtifacts {
				if a := itemsByID[id]; a != nil {
					artifactSetInUse = append(artifactSetInUse, a)
					spec := a.GetSpec()
					strType := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
					artifactIcons += ei.GetBotEmojiMarkdown(fmt.Sprintf("%s%s", ei.ShortArtifactName[int32(spec.GetName())], strType))
				}
			}
		}
	}

	if config.IsDevBot() {
		for _, a := range maxCTEResult.Artifacts {
			if a == nil || a.GetSpec() == nil {
				continue
			}
			spec := a.GetSpec()
			strType := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
			maxArtifactIcons += ei.GetBotEmojiMarkdown(fmt.Sprintf("%s%s", ei.ShortArtifactName[int32(spec.GetName())], strType))
		}
	}

	artifactBuffs := ei.GetArtifactBuffs(artifactSetInUse)

	// Get Colleggtible Buffs
	contracts := backup.GetContracts()
	colBuffs := ei.GetColleggtibleBuffs(contracts)

	shippingRate := ei.GetShippingRateFromBackup(farm, backup.GetGame())
	eggLayingRate, habPop, habCap := ei.GetEggLayingRateFromBackup(farm, backup.GetGame())
	if simulatedEgg != -1 {
		eggLayingRate /= habPop // Remove population from the ELR
		if targetTE == 0 {
			habPop = math.Min(1, habCap)
		}
		eggLayingRate *= habPop // Reset to 0 for new egg
	}
	//deliveryRate := math.Min(eggLayingRate, shippingRate)
	eggLayingRate *= artifactBuffs.ELR * colBuffs.ELR
	shippingRate *= artifactBuffs.SR * colBuffs.SR
	habCap *= artifactBuffs.Hab * colBuffs.Hab

	gemsOnHand := backup.GetFarms()[0].GetCashEarned() - backup.GetFarms()[0].GetCashSpent()
	_, onlineRate, _, offlineRate := ei.GetInternalHatcheryFromBackup(farm.GetCommonResearch(), backup.GetGame(), artifactBuffs.IHR*colBuffs.IHR, allEov)
	siloMinutes := ei.GetSiloMinutes(farm.GetSilosOwned(), backup.GetGame().GetEpicResearch())

	fuelingEnabled := virtue.GetAfx().GetTankFillingEnabled()
	if simulatedEgg != -1 {
		// When simulating a shift, turn off fueling
		fuelingEnabled = false
	}
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

	// Now with our rates we can figure out earnings numbers
	_, offlineRateHr := ei.GetFarmEarningRates(backup, math.Min(shippingRate, eggLayingRate-fuelRate), artifactBuffs, colBuffs, allEov)

	habPercent := 0.0
	if habCap > 0 {
		habPercent = (habPop / habCap) * 100
	}
	onlineFillTime := ei.TimeForLinearGrowth(habPop, habCap, onlineRate/60)
	offlineFillTime := ei.TimeForLinearGrowth(habPop, habCap, offlineRate/60)
	activeDuration := time.Duration(farm.GetTotalStepTime()) * time.Second
	syncTime := time.Unix(int64(farm.GetLastStepTime()), 0)
	elapsed := time.Since(syncTime).Seconds()
	offlineEggs := min(eggLayingRate-fuelRate, shippingRate) * (elapsed / 3600)
	if simulatedEgg != -1 && targetTE == 0 {
		offlineEggs = 0
	}

	if onVirtueFarm {
		fmt.Fprintf(&stats, "%s %s\n", VehicleArray, strings.Join(habArray, ""))

		shipFmt := ei.FormatEIValue(shippingRate, map[string]any{"decimals": 2, "trim": true})
		elrFmt := ei.FormatEIValue(eggLayingRate-fuelRate, map[string]any{"decimals": 2, "trim": true})

		// Want time from now when those minutes elapse
		if shippingRate > eggLayingRate {
			fmt.Fprintf(&stats, "%s %s/hr  %s **%s/hr**",
				VehicleArt,
				shipFmt,
				selectedEggEmote,
				elrFmt)
		} else {
			fmt.Fprintf(&stats, "%s **%s/hr**  %s %s/hr",
				VehicleArt,
				shipFmt,
				selectedEggEmote,
				elrFmt)
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
					ei.FormatEIValue(fuelRate, map[string]any{"decimals": 2, "trim": true}))
			} else {
				fmt.Fprintf(&stats, " %s%s %s/hr\n",
					DepotArt,
					fuelLamp,
					ei.FormatEIValue(fuelRate, map[string]any{"decimals": 2, "trim": true}))
			}
		} else if recommendedFuelRate > 0.0 {
			fmt.Fprintf(&stats, " %s🎚️ %.0f%%\n", DepotArt, recommendedFuelRate*100)
		} else {
			fmt.Fprint(&stats, "\n")
		}
		// Calculate offline hab time until SR capacity
		if habPop < habCap && habPercent < 99.9 && shippingRate > eggLayingRate {
			limitPop := habPop * (shippingRate / eggLayingRate)

			if habCap > limitPop {
				offlineCapTime := ei.TimeForLinearGrowth(habPop, limitPop, offlineRate/60)
				fmt.Fprintf(&stats, "**SR cap:** %s %s 💤<t:%d:R>\n",
					habArt,
					ei.FormatEIValue(limitPop, map[string]any{"decimals": 2, "trim": true}),
					time.Now().Add(time.Duration(int64(offlineCapTime))*time.Second).Unix(),
				)
			}
		}

		fmt.Fprintf(&stats, "**IHR** %s/min  💤 %s/min  %s %s\n",
			ei.FormatEIValue(onlineRate, map[string]any{"decimals": 2, "trim": true}),
			ei.FormatEIValue(offlineRate, map[string]any{"decimals": 2, "trim": true}),
			ei.GetBotEmojiMarkdown("silo"),
			bottools.FmtDuration(time.Duration(siloMinutes)*time.Minute),
		)

		if habPop >= habCap || habPercent >= 99.9 {
			fmt.Fprintf(&stats, "%s %d%% %s ⚠️🔒\n",
				//strings.Join(habArray, ""),
				habArt,
				int(habPercent),
				ei.FormatEIValue(habPop, map[string]any{"decimals": 2, "trim": true}))
		} else {
			fmt.Fprintf(&stats, "%s %s %d%% 🔒<t:%d:R> or 💤<t:%d:R>\n",
				//strings.Join(habArray, ""),
				habArt,
				ei.FormatEIValue(habPop, map[string]any{"decimals": 2, "trim": true}),
				int(habPercent),
				time.Now().Add(time.Duration(int64(onlineFillTime))*time.Second).Unix(),
				time.Now().Add(time.Duration(int64(offlineFillTime))*time.Second).Unix())
		}

		//  If targetTE is set, calculate offset for current egg to reach that TE
		offsetRemainingTime := -1.0
		if currentEggIndex != -1 && targetTE != 0 {

			currentEggTarget = ei.TruthEggThresholdByIndex(uint32(targetTE))

			remainingTime := ei.TimeToDeliverEggs(habPop, habCap, offlineRate, eggLayingRate-fuelRate, shippingRate, currentEggTarget-currentDelivered)
			adjustedRemainingTime := remainingTime - elapsed

			if remainingTime != -1.0 {
				offsetRemainingTime = adjustedRemainingTime
			}

			// if we have a target TE and an offsetted remaining time, show that in the header
			if offsetRemainingTime == -1.0 {
				fmt.Fprintf(&header,
					"## Skipping simulation for %[1]d%[2]s\n**Too far away or already delivered.**\n",
					targetTE,
					currentEggEmote,
				)
			} else {
				fmt.Fprintf(&header,
					"## Simulating shift after TE %[1]d%[2]s\n**Deliver %[3]s%[2]s <t:%[4]d:f>💤**\n",
					targetTE,
					currentEggEmote,
					ei.FormatEIValue(currentEggTarget, map[string]any{"decimals": 1, "trim": true}),
					time.Now().Add(time.Duration(int64(offsetRemainingTime))*time.Second).Unix(),
				)
			}
			// Empty the habs after the shift to simulate
			habPop = 1
		} else if simulatedEgg != -1 && targetTE == 0 {
			c := cases.Title(language.Und)
			fmt.Fprintf(&header, "## Simulating a shift to %s\n", c.String(strings.ToLower(virtueEggs[simulatedEgg-50])))
		}

		// Loop to show time to next several Truth Egg thresholds
		remainingTime := ei.TimeToDeliverEggs(habPop, habCap, offlineRate, eggLayingRate-fuelRate, shippingRate, selectedTarget-selectedDelivered)
		adjustedRemainingTime := remainingTime - elapsed

		loopCount := 0
		currentSelectedTarget := selectedTarget
		bold := "**"
		prefix := ""
		for {
			header.WriteString(bold)
			header.WriteString(prefix)
			if remainingTime == -1.0 {
				if selectedTarget-selectedDelivered-offlineEggs <= 0.0 {
					fmt.Fprintf(&header, "Offline deliveries complete %s%s",
						ei.FormatEIValue(currentSelectedTarget, map[string]any{"decimals": 1, "trim": true}),
						selectedEggEmote)
				} else {
					fmt.Fprintf(&header, "Deliver %s%s in more than a year 💤",
						ei.FormatEIValue(currentSelectedTarget, map[string]any{"decimals": 1, "trim": true}),
						selectedEggEmote)
				}
			} else if adjustedRemainingTime < 43200.0 && targetTE == 0 { // 12 hours
				fmt.Fprintf(&header, "Deliver %s%s <t:%d:t>💤",
					ei.FormatEIValue(currentSelectedTarget, map[string]any{"decimals": 1, "trim": true}),
					selectedEggEmote,
					time.Now().Add(time.Duration(int64(adjustedRemainingTime))*time.Second).Unix())
			} else if targetTE != 0 && offsetRemainingTime != -1.0 { // Show the estimated time/duration with offset from TE target on the current egg
				if !compact {
					fmt.Fprintf(&header, "Sim %s%s <t:%d:f>💤 %s",
						ei.FormatEIValue(currentSelectedTarget, map[string]any{"decimals": 1, "trim": true}),
						selectedEggEmote,
						time.Now().Add(time.Duration(int64(adjustedRemainingTime+offsetRemainingTime))*time.Second).Unix(),
						bottools.FmtDuration(time.Duration(int64(adjustedRemainingTime+offsetRemainingTime))*time.Second))
				} else { // Remove duration in compact mode
					fmt.Fprintf(&header, "Sim %s%s <t:%d:f>💤",
						ei.FormatEIValue(currentSelectedTarget, map[string]any{"decimals": 1, "trim": true}),
						selectedEggEmote,
						time.Now().Add(time.Duration(int64(adjustedRemainingTime+offsetRemainingTime))*time.Second).Unix())
				}
			} else {
				fmt.Fprintf(&header, "Deliver %s%s <t:%d:f>💤",
					ei.FormatEIValue(currentSelectedTarget, map[string]any{"decimals": 1, "trim": true}),
					selectedEggEmote,
					time.Now().Add(time.Duration(int64(adjustedRemainingTime))*time.Second).Unix())
			}
			header.WriteString(bold)

			// Prepare for next threshold
			currentSelectedTarget = ei.NextTruthEggThreshold(currentSelectedTarget, 0)
			if currentSelectedTarget == math.Inf(1) {
				break
			}
			remainingTime = ei.TimeToDeliverEggs(habPop, habCap, offlineRate, eggLayingRate-fuelRate, shippingRate, currentSelectedTarget-selectedDelivered)
			adjustedRemainingTime = remainingTime - elapsed

			loopCount++
			// Stop if remainingTime is -1 or adjustedRemainingTime is more than 2 weeks (1209600 seconds), or after 5 iterations to avoid infinite loop
			if remainingTime == -1.0 || (adjustedRemainingTime > 1209600 && loopCount > 1) || loopCount >= 9 {
				break
			}
			header.WriteString("\n")
			bold = ""
			prefix = "-# "
		}

		if offlineEggs > 0 {
			fmt.Fprintf(&header, "\n-# includes %s offline eggs", ei.FormatEIValue(offlineEggs, map[string]any{"decimals": 3, "trim": true}))
		}
	} else {
		fmt.Fprint(&header, "**Ascend to visit your Eggs of Virtue farm.**")
	}

	// Try and have an accurate gem total including offline earnings
	offlineGems := 0.0
	if elapsed > 60.0 {
		offlineGems = (offlineRateHr / 3600) * math.Floor(elapsed-60)
	}

	if onVirtueFarm {
		fmt.Fprintf(&stats, "%s %s est. **%sOffline** %s/hr  %s/s\n",
			ei.GetBotEmojiMarkdown("gem"),
			ei.FormatEIValue(gemsOnHand+offlineGems, map[string]any{"decimals": 3, "trim": true}),
			ei.GetBotEmojiMarkdown("gem"),
			ei.FormatEIValue(offlineRateHr, map[string]any{"decimals": 3, "trim": true}),
			ei.FormatEIValue(offlineRateHr/3600, map[string]any{"decimals": 3, "trim": true}),
		)
	}

	// Display Artifact buffs if any
	hasArtifactBuffs := artifactBuffs.SR != 1 || artifactBuffs.ELR != 1 || artifactBuffs.IHR != 1 || artifactBuffs.Hab != 1 || artifactBuffs.Earnings != 1 || artifactBuffs.AwayEarnings != 1
	if hasArtifactBuffs {
		fmt.Fprintf(&stats, "%s", artifactIcons)

		if artifactBuffs.SR != 1 {
			fmt.Fprintf(&stats, " SR:%s", ei.FormatModifierValue(artifactBuffs.SR))
		}
		if artifactBuffs.ELR != 1 {
			fmt.Fprintf(&stats, " ELR:%s", ei.FormatModifierValue(artifactBuffs.ELR))
		}
		if artifactBuffs.IHR != 1 {
			fmt.Fprintf(&stats, " IHR:%s", ei.FormatModifierValue(artifactBuffs.IHR))
		}
		if artifactBuffs.Hab != 1 {
			fmt.Fprintf(&stats, " H:%s", ei.FormatModifierValue(artifactBuffs.Hab))
		}
		if artifactBuffs.Earnings != 1 || artifactBuffs.AwayEarnings != 1 {
			fmt.Fprintf(&stats, " %s", ei.GetBotEmojiMarkdown("gem"))
			if artifactBuffs.Earnings != 1 {
				fmt.Fprintf(&stats, "%s", ei.FormatModifierValue(artifactBuffs.Earnings))
			}
			if artifactBuffs.AwayEarnings != 1 {
				fmt.Fprintf(&stats, " 💤%s", ei.FormatModifierValue(artifactBuffs.AwayEarnings))
			}
		}

		fmt.Fprint(&stats, "\n")
	}

	if config.IsDevBot() {
		if maxArtifactIcons != "" {
			fmt.Fprintf(&stats, "%s **Max CTE** %.0f\n", maxArtifactIcons, maxCTE)
			for _, line := range ei.DescribeArtifactSetWithStones(maxCTEResult.Artifacts) {
				fmt.Fprintf(&stats, "-# %s\n", line)
			}
		}

		if artifactIcons != "" && maxArtifactIcons != "" && artifactIcons != maxArtifactIcons && cteDelta > 0 {
			fmt.Fprintf(&stats, "%s ➜ %s  **+%.0f CTE**\n", artifactIcons, maxArtifactIcons, cteDelta)
		}
	}

	fmt.Fprintf(&stats, "%s", ei.GetBotEmojiMarkdown("collegg"))
	if colBuffs.SR != 1 {
		fmt.Fprintf(&stats, " SR:%s", ei.FormatModifierValue(colBuffs.SR))
	}
	if colBuffs.ELR != 1 {
		fmt.Fprintf(&stats, " ELR:%s", ei.FormatModifierValue(colBuffs.ELR))
	}
	if colBuffs.IHR != 1 {
		fmt.Fprintf(&stats, " IHR:%s", ei.FormatModifierValue(colBuffs.IHR))
	}
	if colBuffs.Hab != 1 {
		fmt.Fprintf(&stats, " H:%s", ei.FormatModifierValue(colBuffs.Hab))
	}
	if colBuffs.Earnings != 1 || colBuffs.AwayEarnings != 1 {
		fmt.Fprintf(&stats, " %s", ei.GetBotEmojiMarkdown("gem"))
		if colBuffs.Earnings != 1 {
			fmt.Fprintf(&stats, "%s", ei.FormatModifierValue(colBuffs.Earnings))
		}
		if colBuffs.AwayEarnings != 1 {
			fmt.Fprintf(&stats, " 💤%s", ei.FormatModifierValue(colBuffs.AwayEarnings))
		}
	}
	fmt.Fprint(&stats, "\n")

	if onVirtueFarm {
		fmt.Fprintf(&footer, "-# Report:%s  Sync:%s  🗓️%s:%s\n",
			bottools.WrapTimestamp(time.Now().Unix(), bottools.TimestampShortTime),
			bottools.WrapTimestamp(syncTime.Unix(), bottools.TimestampShortTime),
			ei.GetBotEmojiMarkdown("silo"),
			bottools.FmtDuration(activeDuration.Round(time.Hour)))
	} else {
		syncTime := time.Unix(int64(backup.GetApproxTime()), 0)
		fmt.Fprintf(&footer, "-# Report:%s  Backup Sync:%s\n",
			bottools.WrapTimestamp(time.Now().Unix(), bottools.TimestampShortTime),
			bottools.WrapTimestamp(syncTime.Unix(), bottools.TimestampShortTime))
	}

	// Line for fuel
	fuels := virtue.GetAfx().GetTankFuels()
	fuels = fuels[len(fuels)-5:]
	fmt.Fprintf(&rockets, "\n%s", DepotArt)
	for i, fuel := range fuels {
		fmt.Fprintf(&rockets, " %s:%s",
			ei.GetBotEmojiMarkdown("egg_"+strings.ToLower(virtueEggs[i])),
			ei.FormatEIValue(fuel, map[string]any{"decimals": 1, "trim": true}))
	}
	rockets.WriteString("\n")
	missions := artifactDB.GetMissionInfos()
	for _, mission := range missions {
		missionType := mission.GetType()
		//missionStatus := mission.GetStatus()
		if missionType == ei.MissionInfo_VIRTUE {
			shipType := mission.GetShip()
			craft := ei.MissionArt.Ships[shipType]
			art := ei.GetBotEmojiMarkdown(craft.Art)
			missionEnd := uint32(mission.GetStartTimeDerived()) + uint32(mission.GetDurationSeconds())
			fmt.Fprintf(&rockets, "%s<t:%d:R> ", art, missionEnd)
		}
	}

	// Notes section
	// Add notes only if not compact, default is false
	if !compact {
		// Determine the costs of the next research items
		// Only for Curiosity egg
		prefixLinefeed := ""
		if selectedEggIndex == 0 {
			researchStr := ei.GatherCommonResearchCosts(gemsOnHand, offlineRateHr, backup.GetGame().GetEpicResearch(), backup.GetFarms()[0].GetCommonResearch(), colBuffs.ResearchDiscount, artifactBuffs.ResearchDiscount)
			if researchStr != "" {
				fmt.Fprint(&notes, researchStr)
				prefixLinefeed = "\n"

			}
		}
		// Available Vehicles and Trains from research
		availableFleetSize := ei.GetFleetSize(farm.GetCommonResearch())
		availableTrainLength := ei.GetTrainLength(farm.GetCommonResearch())
		vehicleCount := uint32(len(farm.GetVehicles()))
		trainLengths := farm.GetTrainLength()
		maxTrainLength := uint32(5)
		for _, length := range trainLengths {
			if length > maxTrainLength {
				maxTrainLength = length
			}
		}

		// Add max available vehicles and train length info
		switch selectedEggIndex {
		case 0: // Curiosity
			if availableFleetSize < 17 {
				fmt.Fprintf(&notes, "%s-# Max Available Vehicles: %d/17 %s\n", prefixLinefeed, availableFleetSize, VehicleArt)
			} else if availableFleetSize == 17 && maxTrainLength < 10 {
				fmt.Fprintf(&notes, "%s-# All 17 vehicles available %s\n-# Max Available Train Length: %d/10 %s\n", prefixLinefeed, VehicleArt, maxTrainLength, ei.GetBotEmojiMarkdown("tl"))
			} else if availableFleetSize == 17 && maxTrainLength == 10 {
				fmt.Fprintf(&notes, "%s-# All 17 vehicles and max 10 train length available %s %s\n", prefixLinefeed, VehicleArt, ei.GetBotEmojiMarkdown("tl"))
			}
		case 2: // Humility
			// Calculate the launched ships
			fmt.Fprintf(&notes, "%s", getVirtueLaunchedShips(backup))

		default: //  On other eggs show only if < currently available
			if vehicleCount < availableFleetSize {
				fmt.Fprintf(&notes, "%s-# Available Fleet Size: %d/17 %s\n", prefixLinefeed, availableFleetSize, VehicleArt)
			} else if availableFleetSize == 17 && maxTrainLength < availableTrainLength {
				fmt.Fprintf(&notes, "%s-# All 17 vehicles available %s\n-# Available Train Length: %d/10 %s\n", prefixLinefeed, VehicleArt, availableTrainLength, ei.GetBotEmojiMarkdown("tl"))
			}
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
	if notes.Len() > 0 {
		components = append(components, &discordgo.TextDisplay{
			Content: notes.String(),
		})
		components = append(components, &discordgo.Separator{
			Divider: &divider,
			Spacing: &spacing,
		})
	}
	components = append(components, &discordgo.TextDisplay{
		Content: rockets.String(),
	})
	components = append(components, &discordgo.TextDisplay{
		Content: footer.String(),
	})

	return components
}

func getVirtueLaunchedShips(backup *ei.Backup) string {
	var output strings.Builder

	afx := backup.GetArtifactsDb()
	missionInfo := afx.GetMissionInfos()      // in progress missions
	missionArchive := afx.GetMissionArchive() // completed missions

	virtue := backup.GetVirtue()

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

	maxFill := make([]float64, len(fuels))
	for i := range fuels {
		maxFill[i] = tankLimit * fuelLimits[i]
	}

	// Create ordered list of missions by start time descending
	mymissions := append(missionInfo, missionArchive...)
	slices.SortFunc(mymissions, func(a, b *ei.MissionInfo) int {
		sa := a.GetStartTimeDerived()
		sb := b.GetStartTimeDerived()
		if sa > sb {
			return -1 // a before b (descending)
		}
		if sa < sb {
			return 1 // a after b
		}
		return 0
	})

	// Track fuel fills to determine if tank limit reached
	// If so, we stop processing further missions
	tankLimitReached := false
	shipCounts := make(map[string]int32)

	var firstMissionStart, lastMissionStart float64 = -1, -1

	for _, mi := range mymissions {
		if mi.GetType() != ei.MissionInfo_VIRTUE {
			continue
		}

		missionStart := mi.GetStartTimeDerived()
		//missionEnd := uint32(missionStart) + uint32(mi.GetDurationSeconds())
		// Only want missions in the last month
		if uint32(missionStart) < uint32(time.Now().AddDate(0, -1, 0).Unix()) {
			continue
		}

		// Track first and last missionStart times
		if firstMissionStart == -1 || float64(missionStart) < firstMissionStart {
			firstMissionStart = missionStart
		}
		if lastMissionStart == -1 || missionStart > lastMissionStart {
			lastMissionStart = missionStart
		}

		for _, f := range mi.GetFuel() {
			fuelEgg := f.GetEgg() - ei.Egg_CURIOSITY
			if fuelEgg == 2 {
				continue // Skip Humility fuel as it isn't previously banked
			}
			fuelAmount := f.GetAmount()
			// Add this amount to the fuels and if over maxFill, mark as tankLimitReached
			if int(fuelEgg) >= 0 && int(fuelEgg) < len(fuels) {
				fuels[fuelEgg] += fuelAmount
				if fuels[fuelEgg] >= maxFill[fuelEgg] {
					tankLimitReached = true
				}
			}
		}

		ship := mi.GetShip()
		durationType := mi.GetDurationType()
		key := fmt.Sprintf("%d/%d", ship, durationType)
		shipCounts[key]++

		if tankLimitReached {
			// We've reached tank limit, stop processing further missions
			break
		}
	}

	if len(shipCounts) == 0 {
		return ""
	}

	// Print the mission start time range
	if firstMissionStart != -1 && lastMissionStart != -1 {
		fmt.Fprintf(&output, "Missions launched between %s - %s\n", bottools.WrapTimestamp(int64(firstMissionStart), bottools.TimestampRelativeTime), bottools.WrapTimestamp(int64(lastMissionStart), bottools.TimestampRelativeTime))
	}

	// Want to sort keys for consistent output, shipID & durationTypes
	var keys []string
	for k := range shipCounts {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	// Group by ship ID
	shipGroups := make(map[int][]string)
	for _, key := range keys {
		parts := strings.Split(key, "/")
		if len(parts) != 2 {
			continue
		}
		shipID, err1 := strconv.Atoi(parts[0])
		durationType, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			continue
		}
		count := shipCounts[key]
		if durationType < 0 || durationType > math.MaxInt32 {
			continue
		}
		durationStr := ei.DurationTypeNameAbbr[int32(durationType)]
		shipGroups[shipID] = append(shipGroups[shipID], fmt.Sprintf("%sx%d", durationStr, count))
	}

	// Sort ship IDs and output
	var shipIDs []int
	for shipID := range shipGroups {
		shipIDs = append(shipIDs, shipID)
	}
	slices.Sort(shipIDs)

	for _, shipID := range shipIDs {
		craft := ei.MissionArt.Ships[shipID]
		art := ei.GetBotEmojiMarkdown(craft.Art)
		durations := strings.Join(shipGroups[shipID], " / ")
		fmt.Fprintf(&output, "%s%s\n", art, durations)
	}

	return output.String()
}

func getShiftCost(shiftCount uint32, soulEggs float64) float64 {
	X := float64(soulEggs) * (0.02*math.Pow(float64(shiftCount)/120, 3) + 0.0001)
	C := math.Pow(10, 11) + 0.6*X + math.Pow(0.4*X, 0.9)

	return C
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
func getVehicleIconStrings(
	vehicles []uint32,
	trainLength []uint32,
	getBotEmojiMarkdown func(string) string) (
	string, string) {

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
