package virtue

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const virtueAllPageSize = 15

// HandleVirtueDismiss removes the action row buttons from the virtue display.
func HandleVirtueDismiss(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		return
	}
	targetUserID := parts[1]
	callerID := e.UserID()
	altController := farmerstate.GetMiscSettingString(targetUserID, "AltController")
	if callerID != targetUserID && (altController == "" || callerID != altController) {
		_ = e.Respond(dc.Message{
			Ephemeral: true,
			Content:   "Only the original caller can use this button.",
		})
		return
	}
	removeVirtueTimer(e.ChannelID(), e.MessageID())
	_ = e.Update(dc.Message{
		Components: e.MessageComponentsWithoutActionRows(),
	})
}

// HandleVirtueRefresh re-fetches the user's farm backup and updates the virtue display.
func HandleVirtueRefresh(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 4 {
		return
	}
	targetUserID := parts[1]
	simEggInt, _ := strconv.ParseInt(parts[2], 10, 32)
	if simEggInt < math.MinInt32 || simEggInt > math.MaxInt32 {
		simEggInt = -1
	}
	simulatedEgg := ei.Egg(simEggInt)
	targetTE, _ := strconv.ParseUint(parts[3], 10, 32)

	callerID := e.UserID()
	altController := farmerstate.GetMiscSettingString(targetUserID, "AltController")
	if callerID != targetUserID && (altController == "" || callerID != altController) {
		_ = e.Respond(dc.Message{
			Ephemeral: true,
			Content:   "Only the original caller can use this button.",
		})
		return
	}

	eiID := farmerstate.GetMiscSettingString(targetUserID, "encrypted_ei_id")
	if eiID == "" {
		_ = e.Respond(dc.Message{
			Ephemeral: true,
			Content:   "Could not find Egg Inc ID for this user.",
		})
		return
	}
	eggIncID := decryptEggIncID(eiID)

	backup, _ := ei.GetFirstContactFromAPINoCache(eggIncID, targetUserID, false)
	if backup == nil {
		_ = e.Respond(dc.Message{
			Ephemeral: true,
			Content:   "Could not load farm backup.",
		})
		return
	}

	compact := false
	if savedCompact := farmerstate.GetMiscSettingString(callerID, "virtueCompactMode"); savedCompact != "" {
		compact, _ = strconv.ParseBool(savedCompact)
	}

	components := printVirtue(targetUserID, backup, simulatedEgg, targetTE, compact, "")
	_ = e.Update(dc.Message{
		Components: components,
	})
}

// HandleVirtueShowAll handles the "Show All TE" button and its ephemeral pagination buttons.
func HandleVirtueShowAll(e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 5 {
		return
	}
	targetUserID := parts[1]
	simEggInt, _ := strconv.ParseInt(parts[2], 10, 32)
	if simEggInt < math.MinInt32 || simEggInt > math.MaxInt32 {
		simEggInt = -1
	}
	simulatedEgg := ei.Egg(simEggInt)
	targetTE, _ := strconv.ParseUint(parts[3], 10, 32)
	page, _ := strconv.Atoi(parts[4])
	isNav := len(parts) >= 6 && parts[5] == "nav"

	callerID := e.UserID()
	altController := farmerstate.GetMiscSettingString(targetUserID, "AltController")
	if callerID != targetUserID && (altController == "" || callerID != altController) {
		_ = e.Respond(dc.Message{
			Ephemeral: true,
			Content:   "Only the original caller can use this button.",
		})
		return
	}

	eiID := farmerstate.GetMiscSettingString(targetUserID, "encrypted_ei_id")
	if eiID == "" {
		_ = e.Respond(dc.Message{
			Ephemeral: true,
			Content:   "Could not find Egg Inc ID for this user.",
		})
		return
	}
	eggIncID := decryptEggIncID(eiID)

	backup, _ := ei.GetFirstContactFromAPINoCache(eggIncID, targetUserID, false)
	if backup == nil {
		_ = e.Respond(dc.Message{
			Ephemeral: true,
			Content:   "Could not load farm backup.",
		})
		return
	}

	content, curPage, totalPages := buildVirtueAllTEPage(backup, simulatedEgg, targetTE, page)
	var comps []dc.LayoutComponent
	comps = append(comps, dc.TextDisplay{Content: content})

	if totalPages > 1 {
		prevPage := max(curPage-1, 0)
		nextPage := min(curPage+1, totalPages-1)
		comps = append(comps, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{
					Label:    "Prev",
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("virtue_all#%s#%d#%d#%d#nav", targetUserID, int(simulatedEgg), targetTE, prevPage),
					Disabled: curPage == 0,
				},
				dc.Button{
					Label:    fmt.Sprintf("Page %d/%d", curPage+1, totalPages),
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("virtue_all#%s#%d#%d#%d#noop", targetUserID, int(simulatedEgg), targetTE, curPage),
					Disabled: true,
				},
				dc.Button{
					Label:    "Next",
					Style:    dc.ButtonSecondary,
					CustomID: fmt.Sprintf("virtue_all#%s#%d#%d#%d#nav", targetUserID, int(simulatedEgg), targetTE, nextPage),
					Disabled: curPage >= totalPages-1,
				},
			},
		})
	}

	msg := dc.Message{
		Ephemeral:  true,
		Components: comps,
	}
	if isNav {
		_ = e.Update(msg)
	} else {
		_ = e.Respond(msg)
	}
}

func buildVirtueAllTEPage(backup *ei.Backup, simulatedEgg ei.Egg, targetTE uint64, page int) (string, int, int) {
	if len(backup.GetFarms()) == 0 {
		return "No farm data available.", 0, 1
	}
	farm := backup.GetFarms()[0]
	virtue := backup.GetVirtue()
	if farm == nil || virtue == nil {
		return "No virtue data available.", 0, 1
	}

	eggType := farm.GetEggType()
	isVirtueEgg := eggType >= ei.Egg_CURIOSITY && eggType <= ei.Egg_KINDNESS
	if !isVirtueEgg {
		return "Your home farm isn't currently producing Eggs of Virtue.", 0, 1
	}

	virtueEggNames := []string{"Curiosity", "Integrity", "Humility", "Resilience", "Kindness"}
	colBuffs := ei.GetColleggtibleBuffs(backup.GetContracts())
	artifactBuffs := ei.GetArtifactBuffs(nil)
	shippingRate := ei.GetShippingRateFromBackup(farm, backup.GetGame()) * artifactBuffs.SR * colBuffs.SR
	eggLayingRate, habPop, habCap := ei.GetEggLayingRateFromBackup(farm, backup.GetGame(), colBuffs.Hab)
	elrPerChicken := 0.0
	if habPop > 0 {
		elrPerChicken = eggLayingRate / habPop
	} else {
		elrPerChicken = (1.0 / 30.0) * ei.GetCommonResearchLayRate(farm.GetCommonResearch()) * ei.GetEpicResearchLayRate(backup.GetGame().GetEpicResearch()) * 3600.0
	}
	eggLayingRate *= artifactBuffs.ELR * colBuffs.ELR
	elrPerChicken *= artifactBuffs.ELR * colBuffs.ELR
	var allEov uint32
	for _, eov := range virtue.GetEovEarned() {
		allEov += eov
	}
	_, _, _, offlineRate := ei.GetInternalHatcheryFromBackup(farm.GetCommonResearch(), backup.GetGame(), artifactBuffs.IHR*colBuffs.IHR, allEov)
	fuelRate := 0.0

	lastSaved := time.Unix(int64(farm.GetLastStepTime()), 0)
	elapsed := max(time.Since(lastSaved).Seconds(), 0)

	maxTE := uint32(98)
	if targetTE > 0 && targetTE < 98 {
		maxTE = uint32(min(targetTE, 98))
	}

	var lines []string

	appendEggLines := func(eggIdx int, startTE uint32, endTE uint32, delivered float64, pop, elr, fuel, timeOffset float64) {
		emote := ei.GetBotEmojiMarkdown("egg_" + strings.ToLower(virtueEggNames[eggIdx]))
		for te := startTE; te <= endTE; te++ {
			target := ei.TruthEggThresholdByIndex(te)
			if target == math.Inf(1) {
				break
			}
			rem := ei.TimeToDeliverEggs(pop, habCap, offlineRate, elr-fuel, shippingRate, target-delivered)
			if rem == -1.0 {
				lines = append(lines, fmt.Sprintf("%d%s %s in more than a year",
					te, emote, ei.FormatEIValue(target, map[string]any{"decimals": 4, "trim": true, "max_length": 5})))
			} else {
				totalSec := max(rem+timeOffset, 0)
				eta := time.Now().Add(time.Duration(int64(totalSec)) * time.Second).Unix()
				lines = append(lines, fmt.Sprintf("%d%s %s <t:%d:d> <t:%d:t> %s",
					te, emote,
					ei.FormatEIValue(target, map[string]any{"decimals": 4, "trim": true, "max_length": 5}),
					eta, eta,
					bottools.FmtDuration((time.Duration(int64(totalSec))*time.Second).Round(time.Minute))))
			}
		}
	}

	curIdx := int(eggType - ei.Egg_CURIOSITY)
	curDelivered := virtue.GetEggsDelivered()[curIdx]
	curEarned := ei.CountTruthEggTiersPassed(curDelivered)

	if simulatedEgg != -1 && targetTE != 0 {
		// Current egg up to targetTE (capped at 98)
		if curEarned < maxTE {
			appendEggLines(curIdx, curEarned+1, maxTE, curDelivered, habPop, eggLayingRate, fuelRate, -elapsed)
		}
		// Offset to reach targetTE on current egg
		curTarget := ei.TruthEggThresholdByIndex(maxTE)
		remOffset := ei.TimeToDeliverEggs(habPop, habCap, offlineRate, eggLayingRate-fuelRate, shippingRate, curTarget-curDelivered)
		offsetSec := 0.0
		if remOffset != -1.0 {
			offsetSec = max(remOffset-elapsed, 0)
		}

		// Simulated egg up to targetTE (capped at 98)
		simIdx := int(simulatedEgg - ei.Egg_CURIOSITY)
		if simIdx >= 0 && simIdx < len(virtueEggNames) {
			simDelivered := virtue.GetEggsDelivered()[simIdx]
			simEarned := ei.CountTruthEggTiersPassed(simDelivered)
			if simEarned < maxTE {
				appendEggLines(simIdx, simEarned+1, maxTE, simDelivered, 1, elrPerChicken, 0, offsetSec)
			}
		}
	} else {
		selIdx := curIdx
		timeOff := -elapsed
		pop := habPop
		elr := eggLayingRate
		fuel := fuelRate
		if simulatedEgg != -1 {
			selIdx = int(simulatedEgg - ei.Egg_CURIOSITY)
			timeOff = 0
			pop = 1
			elr = elrPerChicken
			fuel = 0
		}
		if selIdx >= 0 && selIdx < len(virtueEggNames) {
			selDelivered := virtue.GetEggsDelivered()[selIdx]
			selEarned := ei.CountTruthEggTiersPassed(selDelivered)
			if selEarned < 98 {
				appendEggLines(selIdx, selEarned+1, 98, selDelivered, pop, elr, fuel, timeOff)
			}
		}
	}

	if len(lines) == 0 {
		return "No remaining Truth Egg thresholds to display.", 0, 1
	}

	totalPages := (len(lines) + virtueAllPageSize - 1) / virtueAllPageSize
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	baseSize := len(lines) / totalPages
	remainder := len(lines) % totalPages
	start := page*baseSize + min(page, remainder)
	pageSize := baseSize
	if page < remainder {
		pageSize++
	}
	end := min(start+pageSize, len(lines))

	return "## All TE Deliveries\n" + strings.Join(lines[start:end], "\n"), page, totalPages
}
