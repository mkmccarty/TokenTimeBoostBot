package farm

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var totemBuffs = map[string]float64{
	"T1C": 2.0,
	"T2C": 3.0, "T2R": 8.0,
	"T3C": 20.0, "T3R": 40.0,
	"T4C": 50.0, "T4R": 100.0, "T4E": 150.0, "T4L": 200.0,
}

var necklaceBuffs = map[string]float64{
	"T1C": 1.10,
	"T2C": 1.25, "T2R": 1.35,
	"T3C": 1.50, "T3R": 1.60, "T3E": 1.75,
	"T4C": 2.00, "T4R": 2.25, "T4E": 2.50, "T4L": 3.00,
}

var ankhBuffs = map[string]float64{
	"T1C": 1.10,
	"T2C": 1.25, "T2R": 1.28,
	"T3C": 1.50, "T3R": 1.75, "T3L": 2.00,
	"T4C": 2.00, "T4R": 2.25, "T4L": 2.50,
}

var gussetBuffs = map[string]float64{
	"T1C": 1.05,
	"T2C": 1.10, "T2E": 1.12,
	"T3C": 1.15, "T3R": 1.16,
	"T4C": 1.20, "T4E": 1.22, "T4L": 1.25,
}

var metronomeBuffs = map[string]float64{
	"T1C": 1.05,
	"T2C": 1.10, "T2R": 1.12,
	"T3C": 1.15, "T3R": 1.17, "T3E": 1.20,
	"T4C": 1.25, "T4R": 1.27, "T4E": 1.30, "T4L": 1.35,
}

var compassBuffs = map[string]float64{
	"T1C": 1.05,
	"T2C": 1.10,
	"T3C": 1.20, "T3R": 1.22,
	"T4C": 1.30, "T4R": 1.35, "T4E": 1.40, "T4L": 1.50,
}

var tachyonStoneBuffs = []float64{1.02, 1.04, 1.05} // T2, T3, T4 (level 0, 1, 2)
var quantumStoneBuffs = []float64{1.02, 1.04, 1.05} // T2, T3, T4 (level 0, 1, 2)
var lunarStoneBuffs = []float64{1.20, 1.30, 1.40}   // T2, T3, T4 (level 0, 1, 2)
var shellStoneBuffs = []float64{1.05, 1.08, 1.10}   // T2, T3, T4 (level 0, 1, 2)
var clarityStoneBuffs = []float64{0.25, 0.50, 1.00} // T2, T3, T4 (level 0, 1, 2)

var broochBuffs = map[string]float64{
	"T1C": 0.10,
	"T2C": 0.25,
	"T3C": 0.50, "T3R": 0.60, "T3E": 0.70,
	"T4C": 1.00, "T4R": 1.25, "T4E": 1.50, "T4L": 2.00,
}

var medallionBuffs = map[string]float64{
	"T1C": 0.10,
	"T2C": 0.20, "T2R": 0.20,
	"T3C": 0.35, "T3E": 0.35,
	"T4C": 0.50, "T4R": 0.50, "T4E": 0.50, "T4L": 1.30,
}

var beakBuffs = map[string]float64{
	"T1C": 0.20,
	"T2C": 0.50,
	"T3C": 1.00, "T3R": 2.00,
	"T4C": 5.00, "T4R": 10.00, "T4L": 100.00,
}

// StoneCandidate represents an available stone in inventory.
type StoneCandidate struct {
	Spec       *ei.ArtifactSpec
	Multiplier float64
}

// StoneGroup represents a grouped count of stones of the same type and tier for display.
type StoneGroup struct {
	Spec        *ei.ArtifactSpec
	Count       int
	EffectDelta float64
	Emoji       string
	Label       string
}

// DeliveryResult contains the optimal delivery setup.
type DeliveryResult struct {
	OptimalArtifacts      []*ei.CompleteArtifact
	OptimalStones         []StoneGroup
	LayingRate            float64
	ShippingRate          float64
	DeliveryRate          float64
	CurrentLayingRate     float64
	CurrentShippingRate   float64
	CurrentDeliveryRate   float64
	IsAlreadyOptimal      bool
	GussetRequired        bool
	GussetRequiredName    string
	HabPopulation         float64
	BaseHabCapacity       float64
	ColleggtibleBuffs     ei.DimensionBuffs
	ColleggtiblesOverview ei.ColleggtiblesOverview
}

// EarningsResult contains the optimal earnings setup.
type EarningsResult struct {
	OptimalArtifacts      []*ei.CompleteArtifact
	OptimalStones         []StoneGroup
	TotalMultiplier       float64
	CurrentMultiplier     float64
	IsAlreadyOptimal      bool
	GussetRequired        bool
	GussetRequiredName    string
	HabPopulation         float64
	BaseHabCapacity       float64
	EventMultiplier       float64
	HasActiveEvent        bool
	EventMessage          string
	EventEndTime          time.Time
	ColleggtibleBuffs     ei.DimensionBuffs
	ColleggtiblesOverview ei.ColleggtiblesOverview
}

// DroneFarmingResult contains the optimal drone farming setup.
type DroneFarmingResult struct {
	OptimalArtifacts     []*ei.CompleteArtifact
	OptimalStones        []StoneGroup
	BroochMultiplier     float64
	MedallionMultiplier  float64
	GEDropChance         float64
	TotalMultiplier      float64
	CurrentBroochMult    float64
	CurrentMedallionMult float64
	CurrentGEDropChance  float64
	CurrentMultiplier    float64
	IsAlreadyOptimal     bool
	IsEnlightenment      bool
	EventMultiplier      float64
	HasActiveEvent       bool
	EventMessage         string
	EventEndTime         time.Time
}

// Farm executes the farm command with an encrypted or raw Egg Inc ID.
func Farm(e dc.InteractionEvent, options dc.OptionValues, eiID string, okayToSave bool) {
	eggIncID := eiID
	if len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		eggIncID = decryptEggIncID(eiID)
	}
	if eggIncID == "" || len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		if cmd, ok := e.(*dc.CommandEvent); ok {
			boost.RequestEggIncIDModal(cmd, "farm", options)
		}
		return
	}
	ExecuteFarm(e, options, eggIncID, okayToSave)
}

// ExecuteFarm executes the /farm display logic and responds to the interaction event.
func ExecuteFarm(e dc.InteractionEvent, options dc.OptionValues, eggIncID string, okayToSave bool) {
	effectiveUserID := farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID())
	ExecuteFarmTarget(e, options, eggIncID, okayToSave, effectiveUserID, "")
}

// ExecuteFarmTarget executes the /farm display logic for a specific target user ID with optional notice.
func ExecuteFarmTarget(e dc.InteractionEvent, options dc.OptionValues, eggIncID string, okayToSave bool, targetUserID string, notice string, actAsNotice ...string) {
	ephemeral := e.ChannelID() == "571836573243539476" // ACO- #bot-commands
	_ = e.Defer(ephemeral)

	backup, _ := ei.GetFirstContactFromAPINoCache(eggIncID, targetUserID, okayToSave)
	if backup == nil {
		_ = e.Followup(dc.Message{
			Content:   "Unable to retrieve game data for this Egg Inc ID. Please verify your ID and try again.",
			Ephemeral: true,
		})
		return
	}

	farmerstate.SetFarmerBackupDetails(targetUserID, backup)

	avatarURL := ""
	if targetUserID != e.UserID() {
		altDiscordID := farmerstate.FindAltDiscordID(targetUserID, e.UserID())
		if altDiscordID != "" {
			avatarURL = bottools.GetDiscordAvatarURL(e.Client(), e.GuildID(), altDiscordID)
		}
	} else {
		avatarURL = e.AvatarURL()
		if avatarURL == "" && e.Client() != nil && e.GuildID() != "" {
			avatarURL = bottools.GetDiscordAvatarURL(e.Client(), e.GuildID(), targetUserID)
		}
	}

	actAsText := ""
	if len(actAsNotice) > 0 {
		actAsText = actAsNotice[0]
	}

	farm := getActiveFarm(backup)
	if farm == nil {
		_ = e.Followup(dc.Message{
			Content:   "No active farm data found in Egg Inc API response.",
			Ephemeral: true,
		})
		return
	}

	subcmd := ""
	if sc, ok := options.Subcommand(); ok {
		subcmd = sc
	} else if cmdEvent, ok := e.(*dc.CommandEvent); ok {
		if sc, ok := cmdEvent.Subcommand(); ok {
			subcmd = sc
		}
	}

	var components []dc.LayoutComponent
	switch subcmd {
	case "delivery":
		components = printDeliverySetup(targetUserID, backup, farm, avatarURL, actAsText)
	case "earnings":
		components = printEarningsSetup(targetUserID, backup, farm, avatarURL, actAsText)
	case "drone-farming":
		components = printDroneFarmingSetup(targetUserID, backup, farm, avatarURL, actAsText)
	default:
		_ = e.Followup(dc.Message{
			Content:   farmHelpText(),
			Ephemeral: true,
		})
		return
	}

	if notice != "" {
		components = append([]dc.LayoutComponent{
			dc.TextDisplay{Content: notice},
		}, components...)
	}

	_ = e.Followup(dc.Message{
		Ephemeral:  ephemeral,
		Components: components,
	})
}

func getActiveFarm(backup *ei.Backup) *ei.Backup_Simulation {
	if backup == nil {
		return nil
	}
	if len(backup.GetFarms()) > 0 && backup.GetFarms()[0] != nil {
		return backup.GetFarms()[0]
	}
	return backup.GetSim()
}

func defaultFarmAvatarURL(farm *ei.Backup_Simulation) string {
	if farm != nil {
		eggType := farm.GetEggType()
		if eggType >= ei.Egg_CURIOSITY && eggType <= ei.Egg_KINDNESS {
			if u := ei.GetBotEmojiURL("egg_truth", 128); u != "" {
				return u
			}
		} else {
			eggName := strings.ToLower(eggType.String())
			if u := ei.GetBotEmojiURL("egg_"+eggName, 128); u != "" {
				return u
			}
		}
	}
	return "https://cdn.discordapp.com/emojis/1418022084205875210.webp?size=128"
}

func formatFarmTypeHeader(farm *ei.Backup_Simulation) (string, string) {
	if farm == nil {
		return "Unknown", ""
	}
	eggType := farm.GetEggType()
	c := cases.Title(language.Und)
	eggName := c.String(strings.ToLower(eggType.String()))
	eggEmote := ei.GetBotEmojiMarkdown("egg_" + strings.ToLower(eggType.String()))

	if eggType >= ei.Egg_CURIOSITY && eggType <= ei.Egg_KINDNESS {
		return fmt.Sprintf("Virtue (%s)", eggName), eggEmote
	}
	if eggType == ei.Egg_ENLIGHTENMENT {
		return "Enlightenment", eggEmote
	}
	return fmt.Sprintf("Home (%s)", eggName), eggEmote
}

func getArtifactSlotCount(spec *ei.ArtifactSpec) int {
	if spec == nil || isStone(spec.GetName()) {
		return 0
	}
	slots, err := ei.GetStones(spec.GetName(), spec.GetLevel(), spec.GetRarity())
	if err == nil && slots > 0 {
		return slots
	}

	// Fallback table when eiafx-data is uninitialized or returns 0
	switch spec.GetRarity() {
	case ei.ArtifactSpec_COMMON:
		return 0
	case ei.ArtifactSpec_RARE:
		return 1
	case ei.ArtifactSpec_EPIC:
		return 2
	case ei.ArtifactSpec_LEGENDARY:
		if spec.GetName() == ei.ArtifactSpec_INTERSTELLAR_COMPASS ||
			spec.GetName() == ei.ArtifactSpec_TACHYON_DEFLECTOR ||
			spec.GetName() == ei.ArtifactSpec_SHIP_IN_A_BOTTLE ||
			spec.GetName() == ei.ArtifactSpec_BEAK_OF_MIDAS {
			return 2
		}
		return 3
	}
	return 0
}

func formatArtifactIcons(artifacts []*ei.CompleteArtifact) string {
	var icons strings.Builder
	for _, a := range artifacts {
		if a == nil || a.GetSpec() == nil {
			continue
		}
		spec := a.GetSpec()
		lvl := int(spec.GetLevel())
		rar := int(spec.GetRarity())
		lvlStr := "T1"
		if lvl >= 0 && lvl < len(ei.ArtifactLevels) {
			lvlStr = ei.ArtifactLevels[lvl]
		}
		rarStr := "C"
		if rar >= 0 && rar < len(ei.ArtifactRarity) {
			rarStr = ei.ArtifactRarity[rar]
		}
		strType := lvlStr + rarStr
		icons.WriteString(ei.GetBotEmojiMarkdown(fmt.Sprintf("%s%s", ei.ShortArtifactName[int32(spec.GetName())], strType)))
	}
	return icons.String()
}

func formatArtifactName(spec *ei.ArtifactSpec) string {
	if spec == nil {
		return "Unknown"
	}
	lvl := int(spec.GetLevel())
	rar := int(spec.GetRarity())
	lvlStr := "T1"
	if lvl >= 0 && lvl < len(ei.ArtifactLevels) {
		lvlStr = ei.ArtifactLevels[lvl]
	}
	rarStr := "C"
	if rar >= 0 && rar < len(ei.ArtifactRarity) {
		rarStr = ei.ArtifactRarity[rar]
	}
	tier := lvlStr + rarStr
	name := spec.GetName().String()
	if short, ok := ei.ShortArtifactName[int32(spec.GetName())]; ok && short != "" {
		name = strings.TrimSuffix(short, "_")
	}
	name = strings.ReplaceAll(strings.ToLower(name), "_", " ")
	c := cases.Title(language.Und)
	name = c.String(name)
	return fmt.Sprintf("%s %s", tier, name)
}

func reslotArtifacts(artifacts []*ei.CompleteArtifact, stones []StoneCandidate) []*ei.CompleteArtifact {
	res := make([]*ei.CompleteArtifact, 0, len(artifacts))
	stoneIdx := 0
	for _, a := range artifacts {
		if a == nil || a.GetSpec() == nil {
			continue
		}
		slots := getArtifactSlotCount(a.GetSpec())
		assigned := make([]*ei.ArtifactSpec, 0, slots)
		for i := 0; i < slots && stoneIdx < len(stones); i++ {
			assigned = append(assigned, stones[stoneIdx].Spec)
			stoneIdx++
		}
		res = append(res, &ei.CompleteArtifact{
			Spec:   a.GetSpec(),
			Stones: assigned,
		})
	}
	return res
}

func isSameArtifactLoadout(current, optimal []*ei.CompleteArtifact) bool {
	if len(current) != len(optimal) {
		return false
	}
	currArtCounts := make(map[string]int)
	currStoneCounts := make(map[string]int)
	for _, a := range current {
		if a == nil || a.GetSpec() == nil {
			continue
		}
		s := a.GetSpec()
		key := fmt.Sprintf("%d-%d-%d", s.GetName(), s.GetLevel(), s.GetRarity())
		currArtCounts[key]++
		for _, st := range a.GetStones() {
			if st == nil {
				continue
			}
			stKey := fmt.Sprintf("%d-%d", st.GetName(), st.GetLevel())
			currStoneCounts[stKey]++
		}
	}

	optArtCounts := make(map[string]int)
	optStoneCounts := make(map[string]int)
	for _, a := range optimal {
		if a == nil || a.GetSpec() == nil {
			continue
		}
		s := a.GetSpec()
		key := fmt.Sprintf("%d-%d-%d", s.GetName(), s.GetLevel(), s.GetRarity())
		optArtCounts[key]++
		for _, st := range a.GetStones() {
			if st == nil {
				continue
			}
			stKey := fmt.Sprintf("%d-%d", st.GetName(), st.GetLevel())
			optStoneCounts[stKey]++
		}
	}

	if len(currArtCounts) != len(optArtCounts) || len(currStoneCounts) != len(optStoneCounts) {
		return false
	}
	for k, v := range currArtCounts {
		if optArtCounts[k] != v {
			return false
		}
	}
	for k, v := range currStoneCounts {
		if optStoneCounts[k] != v {
			return false
		}
	}
	return true
}

func getStandardEquippedArtifacts(backup *ei.Backup) []*ei.CompleteArtifact {
	if backup == nil || backup.GetArtifactsDb() == nil {
		return nil
	}
	db := backup.GetArtifactsDb()
	inventory := db.GetInventoryItems()
	itemsByID := make(map[uint64]*ei.CompleteArtifact, len(inventory))
	for _, it := range inventory {
		if it != nil && it.GetArtifact() != nil {
			itemsByID[it.GetItemId()] = it.GetArtifact()
		}
	}
	var slots []*ei.ActiveArtifactSlot
	if len(db.GetActiveArtifactSets()) > 0 && db.GetActiveArtifactSets()[0] != nil {
		slots = db.GetActiveArtifactSets()[0].GetSlots()
	} else if len(db.GetActiveArtifacts_DEPRECATED()) > 0 {
		slots = db.GetActiveArtifacts_DEPRECATED()
	}
	var active []*ei.CompleteArtifact
	for _, s := range slots {
		if s != nil && s.GetOccupied() {
			if a := itemsByID[s.GetItemId()]; a != nil {
				active = append(active, a)
			}
		}
	}
	return active
}

func getEquippedArtifacts(backup *ei.Backup, farm *ei.Backup_Simulation) []*ei.CompleteArtifact {
	if farm != nil && farm.GetEggType() >= ei.Egg_CURIOSITY && farm.GetEggType() <= ei.Egg_KINDNESS {
		return ei.GetActiveVirtueArtifacts(backup)
	}
	return getStandardEquippedArtifacts(backup)
}

func getInventoryItems(backup *ei.Backup, farm *ei.Backup_Simulation) []*ei.ArtifactInventoryItem {
	if backup == nil || backup.GetArtifactsDb() == nil {
		return nil
	}
	if farm != nil && farm.GetEggType() >= ei.Egg_CURIOSITY && farm.GetEggType() <= ei.Egg_KINDNESS {
		if vdb := backup.GetArtifactsDb().GetVirtueAfxDb(); vdb != nil {
			return vdb.GetInventoryItems()
		}
		return nil
	}
	return backup.GetArtifactsDb().GetInventoryItems()
}

// OptimizeEarnings finds the optimal loadout for offline earnings.
func OptimizeEarnings(backup *ei.Backup, farm *ei.Backup_Simulation) *EarningsResult {
	res := &EarningsResult{
		TotalMultiplier:   1.0,
		CurrentMultiplier: 1.0,
		EventMultiplier:   1.0,
	}
	if backup == nil || farm == nil {
		return res
	}

	isUltra := backup != nil && backup.GetSubInfo() != nil && backup.GetSubInfo().GetStatus() == ei.UserSubscriptionInfo_ACTIVE
	eventInfo := getEarningsEventInfo(isUltra)
	res.EventMultiplier = eventInfo.Multiplier
	res.HasActiveEvent = eventInfo.HasActiveEvent
	res.EventMessage = eventInfo.Event.Message
	res.EventEndTime = eventInfo.Event.EndTime

	maxSlots := ei.GetMaxEquippedArtifacts(backup)
	overview := ei.GetColleggtiblesOverview(backup.GetContracts(), "")
	colBuffs := overview.Buffs
	res.ColleggtibleBuffs = colBuffs
	res.ColleggtiblesOverview = overview

	_, habPop, baseHabCap := ei.GetEggLayingRateFromBackup(farm, backup.GetGame(), colBuffs.Hab)
	res.HabPopulation = habPop
	res.BaseHabCapacity = baseHabCap

	// Gusset requirement check
	gussetRequired := habPop > baseHabCap
	res.GussetRequired = gussetRequired

	inventory := getInventoryItems(backup, farm)
	hosts, stones := collectEarningsCandidates(inventory)

	// Filter gussets that can hold habPop if required
	var qualifyingGusset *ei.CompleteArtifact
	if gussetRequired {
		bestGussetSlots := -1
		for _, h := range hosts {
			if h.GetSpec().GetName() == ei.ArtifactSpec_ORNATE_GUSSET {
				tierStr := ei.ArtifactLevels[h.GetSpec().GetLevel()] + ei.ArtifactRarity[h.GetSpec().GetRarity()]
				if gBuff, ok := gussetBuffs[tierStr]; ok && baseHabCap*gBuff >= habPop {
					slots := getArtifactSlotCount(h.GetSpec())
					if slots > bestGussetSlots {
						bestGussetSlots = slots
						qualifyingGusset = h
					}
				}
			}
		}
		if qualifyingGusset != nil {
			gEmoji := getArtifactEmojiMarkdown(qualifyingGusset.GetSpec())
			res.GussetRequiredName = fmt.Sprintf("%s %s", gEmoji, formatArtifactName(qualifyingGusset.GetSpec()))
		}
	}

	// Stone products
	sort.SliceStable(stones, func(i, j int) bool {
		return stones[i].Multiplier > stones[j].Multiplier
	})
	stoneProducts := make([]float64, len(stones)+1)
	stoneProducts[0] = 1.0
	for i := 1; i <= len(stones); i++ {
		stoneProducts[i] = stoneProducts[i-1] * stones[i-1].Multiplier
	}

	bestMultiplier := 1.0
	var bestLoadout []*ei.CompleteArtifact

	var dfs func(idx int, current []*ei.CompleteArtifact, usedFamilies map[ei.ArtifactSpec_Name]bool)
	dfs = func(idx int, current []*ei.CompleteArtifact, usedFamilies map[ei.ArtifactSpec_Name]bool) {
		if len(current) > 0 {
			if gussetRequired {
				hasGusset := false
				for _, a := range current {
					if a.GetSpec().GetName() == ei.ArtifactSpec_ORNATE_GUSSET {
						hasGusset = true
						break
					}
				}
				if !hasGusset && len(current) == maxSlots {
					return
				}
			}

			baseMult := 1.0
			totalSlots := 0
			for _, a := range current {
				spec := a.GetSpec()
				tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
				switch spec.GetName() {
				case ei.ArtifactSpec_LUNAR_TOTEM:
					if b, ok := totemBuffs[tierStr]; ok {
						baseMult *= b
					}
				case ei.ArtifactSpec_DEMETERS_NECKLACE:
					if b, ok := necklaceBuffs[tierStr]; ok {
						baseMult *= b
					}
				case ei.ArtifactSpec_TUNGSTEN_ANKH:
					if b, ok := ankhBuffs[tierStr]; ok {
						baseMult *= b
					}
				}
				totalSlots += getArtifactSlotCount(spec)
			}
			slotsToUse := min(totalSlots, len(stones))
			totalMult := baseMult * stoneProducts[slotsToUse]
			if totalMult > bestMultiplier {
				bestMultiplier = totalMult
				bestLoadout = append([]*ei.CompleteArtifact(nil), current...)
			}
		}

		if idx >= len(hosts) || len(current) >= maxSlots {
			return
		}

		for i := idx; i < len(hosts); i++ {
			fam := hosts[i].GetSpec().GetName()
			if usedFamilies[fam] {
				continue
			}
			usedFamilies[fam] = true
			dfs(i+1, append(current, hosts[i]), usedFamilies)
			delete(usedFamilies, fam)
		}
	}

	used := make(map[ei.ArtifactSpec_Name]bool)
	if gussetRequired && qualifyingGusset != nil {
		used[ei.ArtifactSpec_ORNATE_GUSSET] = true
		dfs(0, []*ei.CompleteArtifact{qualifyingGusset}, used)
	} else {
		dfs(0, nil, used)
	}

	colEarnings := colBuffs.Earnings
	if colEarnings <= 0 {
		colEarnings = 1.0
	}
	colAway := colBuffs.AwayEarnings
	if colAway <= 0 {
		colAway = 1.0
	}
	colleggMult := colEarnings * colAway

	res.TotalMultiplier = bestMultiplier * colleggMult

	// Reslot the bestLoadout with top stones
	totalStoneSlots := 0
	for _, a := range bestLoadout {
		totalStoneSlots += getArtifactSlotCount(a.GetSpec())
	}
	usedStoneCount := min(totalStoneSlots, len(stones))
	res.OptimalArtifacts = reslotArtifacts(bestLoadout, stones[:usedStoneCount])
	res.OptimalStones = groupStones(stones[:usedStoneCount])

	// Calculate current multiplier from currently equipped artifacts
	currentArtifacts := getEquippedArtifacts(backup, farm)
	currBase := 1.0
	currStoneMult := 1.0
	for _, a := range currentArtifacts {
		if a == nil || a.GetSpec() == nil {
			continue
		}
		spec := a.GetSpec()
		tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
		switch spec.GetName() {
		case ei.ArtifactSpec_LUNAR_TOTEM:
			if b, ok := totemBuffs[tierStr]; ok {
				currBase *= b
			}
		case ei.ArtifactSpec_DEMETERS_NECKLACE:
			if b, ok := necklaceBuffs[tierStr]; ok {
				currBase *= b
			}
		case ei.ArtifactSpec_TUNGSTEN_ANKH:
			if b, ok := ankhBuffs[tierStr]; ok {
				currBase *= b
			}
		}
		for _, st := range a.GetStones() {
			if st == nil {
				continue
			}
			lvl := int(st.GetLevel())
			switch st.GetName() {
			case ei.ArtifactSpec_LUNAR_STONE:
				if lvl >= 0 && lvl < len(lunarStoneBuffs) {
					currStoneMult *= lunarStoneBuffs[lvl]
				}
			case ei.ArtifactSpec_SHELL_STONE:
				if lvl >= 0 && lvl < len(shellStoneBuffs) {
					currStoneMult *= shellStoneBuffs[lvl]
				}
			}
		}
	}
	res.CurrentMultiplier = currBase * currStoneMult * colleggMult
	diff := math.Abs(res.TotalMultiplier - res.CurrentMultiplier)
	res.IsAlreadyOptimal = diff < 1e-4 || (res.TotalMultiplier > 0 && diff/res.TotalMultiplier < 1e-4) || res.CurrentMultiplier >= res.TotalMultiplier
	if isSameArtifactLoadout(currentArtifacts, res.OptimalArtifacts) {
		res.OptimalArtifacts = currentArtifacts
		res.IsAlreadyOptimal = true
	}

	return res
}

func collectEarningsCandidates(inventory []*ei.ArtifactInventoryItem) ([]*ei.CompleteArtifact, []StoneCandidate) {
	bestByFamily := make(map[ei.ArtifactSpec_Name]*ei.CompleteArtifact)
	var stones []StoneCandidate

	for _, item := range inventory {
		if item == nil || item.GetArtifact() == nil {
			continue
		}
		art := item.GetArtifact()
		spec := art.GetSpec()
		if spec == nil {
			continue
		}

		if isStone(spec.GetName()) {
			mult := getEarningsStoneMultiplier(spec)
			if mult > 1.0 {
				qty := int(item.GetQuantity())
				if qty < 1 {
					qty = 1
				}
				for i := 0; i < qty; i++ {
					stones = append(stones, StoneCandidate{Spec: spec, Multiplier: mult})
				}
			}
			continue
		}

		for _, st := range art.GetStones() {
			if st != nil {
				mult := getEarningsStoneMultiplier(st)
				if mult > 1.0 {
					stones = append(stones, StoneCandidate{Spec: st, Multiplier: mult})
				}
			}
		}

		slots := getArtifactSlotCount(spec)
		baseBuff := getEarningsArtifactBuff(spec)
		if baseBuff <= 1.0 && slots == 0 {
			continue
		}

		curBest, exists := bestByFamily[spec.GetName()]
		if !exists || earningsScore(spec, slots) > earningsScore(curBest.GetSpec(), getArtifactSlotCount(curBest.GetSpec())) {
			bestByFamily[spec.GetName()] = art
		}
	}

	hosts := make([]*ei.CompleteArtifact, 0, len(bestByFamily))
	for _, a := range bestByFamily {
		hosts = append(hosts, a)
	}

	sort.SliceStable(hosts, func(i, j int) bool {
		si := hosts[i].GetSpec()
		sj := hosts[j].GetSpec()
		bi := getEarningsArtifactBuff(si) > 1.0
		bj := getEarningsArtifactBuff(sj) > 1.0
		if bi != bj {
			return bi
		}
		return getArtifactSlotCount(si) > getArtifactSlotCount(sj)
	})

	return hosts, stones
}

func earningsScore(spec *ei.ArtifactSpec, slots int) float64 {
	base := getEarningsArtifactBuff(spec)
	return base * math.Pow(1.4, float64(slots))
}

func getEarningsArtifactBuff(spec *ei.ArtifactSpec) float64 {
	tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
	switch spec.GetName() {
	case ei.ArtifactSpec_LUNAR_TOTEM:
		if b, ok := totemBuffs[tierStr]; ok {
			return b
		}
	case ei.ArtifactSpec_DEMETERS_NECKLACE:
		if b, ok := necklaceBuffs[tierStr]; ok {
			return b
		}
	case ei.ArtifactSpec_TUNGSTEN_ANKH:
		if b, ok := ankhBuffs[tierStr]; ok {
			return b
		}
	case ei.ArtifactSpec_ORNATE_GUSSET:
		if b, ok := gussetBuffs[tierStr]; ok {
			return b
		}
	}
	return 1.0
}

func getEarningsStoneMultiplier(spec *ei.ArtifactSpec) float64 {
	lvl := int(spec.GetLevel())
	switch spec.GetName() {
	case ei.ArtifactSpec_LUNAR_STONE:
		if lvl >= 0 && lvl < len(lunarStoneBuffs) {
			return lunarStoneBuffs[lvl]
		}
	case ei.ArtifactSpec_SHELL_STONE:
		if lvl >= 0 && lvl < len(shellStoneBuffs) {
			return shellStoneBuffs[lvl]
		}
	}
	return 1.0
}

func isStone(name ei.ArtifactSpec_Name) bool {
	switch name {
	case ei.ArtifactSpec_TACHYON_STONE,
		ei.ArtifactSpec_DILITHIUM_STONE,
		ei.ArtifactSpec_SHELL_STONE,
		ei.ArtifactSpec_LUNAR_STONE,
		ei.ArtifactSpec_SOUL_STONE,
		ei.ArtifactSpec_PROPHECY_STONE,
		ei.ArtifactSpec_QUANTUM_STONE,
		ei.ArtifactSpec_TERRA_STONE,
		ei.ArtifactSpec_LIFE_STONE,
		ei.ArtifactSpec_CLARITY_STONE,
		ei.ArtifactSpec_TACHYON_STONE_FRAGMENT,
		ei.ArtifactSpec_DILITHIUM_STONE_FRAGMENT,
		ei.ArtifactSpec_SHELL_STONE_FRAGMENT,
		ei.ArtifactSpec_LUNAR_STONE_FRAGMENT,
		ei.ArtifactSpec_SOUL_STONE_FRAGMENT,
		ei.ArtifactSpec_PROPHECY_STONE_FRAGMENT,
		ei.ArtifactSpec_QUANTUM_STONE_FRAGMENT,
		ei.ArtifactSpec_TERRA_STONE_FRAGMENT,
		ei.ArtifactSpec_LIFE_STONE_FRAGMENT,
		ei.ArtifactSpec_CLARITY_STONE_FRAGMENT:
		return true
	default:
		return false
	}
}

// OptimizeDelivery finds the optimal loadout for delivery rate.
func OptimizeDelivery(backup *ei.Backup, farm *ei.Backup_Simulation) *DeliveryResult {
	res := &DeliveryResult{}
	if backup == nil || farm == nil {
		return res
	}

	maxSlots := ei.GetMaxEquippedArtifacts(backup)
	overview := ei.GetColleggtiblesOverview(backup.GetContracts(), "")
	colBuffs := overview.Buffs

	baseEggLayingRate, habPop, baseHabCap := ei.GetEggLayingRateFromBackup(farm, backup.GetGame(), colBuffs.Hab)
	baseShippingRate := ei.GetShippingRateFromBackup(farm, backup.GetGame())
	res.HabPopulation = habPop
	res.BaseHabCapacity = baseHabCap
	res.ColleggtibleBuffs = colBuffs
	res.ColleggtiblesOverview = overview

	if habPop == 0 {
		userLayRate := (1.0 / 30.0) * ei.GetCommonResearchLayRate(farm.GetCommonResearch()) * ei.GetEpicResearchLayRate(backup.GetGame().GetEpicResearch()) * 3600.0
		baseEggLayingRate = userLayRate * baseHabCap
		habPop = baseHabCap
	}

	gussetRequired := habPop > baseHabCap
	res.GussetRequired = gussetRequired

	inventory := getInventoryItems(backup, farm)
	hosts, tachStones, quantStones := collectDeliveryCandidates(inventory)

	// Filter gussets that can hold habPop if required
	var qualifyingGusset *ei.CompleteArtifact
	if gussetRequired {
		bestGussetSlots := -1
		for _, h := range hosts {
			if h.GetSpec().GetName() == ei.ArtifactSpec_ORNATE_GUSSET {
				tierStr := ei.ArtifactLevels[h.GetSpec().GetLevel()] + ei.ArtifactRarity[h.GetSpec().GetRarity()]
				if gBuff, ok := gussetBuffs[tierStr]; ok && baseHabCap*gBuff >= habPop {
					slots := getArtifactSlotCount(h.GetSpec())
					if slots > bestGussetSlots {
						bestGussetSlots = slots
						qualifyingGusset = h
					}
				}
			}
		}
		if qualifyingGusset != nil {
			gEmoji := getArtifactEmojiMarkdown(qualifyingGusset.GetSpec())
			res.GussetRequiredName = fmt.Sprintf("%s %s", gEmoji, formatArtifactName(qualifyingGusset.GetSpec()))
		}
	}

	// Sort stones descending
	sort.SliceStable(tachStones, func(i, j int) bool {
		return tachStones[i].Multiplier > tachStones[j].Multiplier
	})
	sort.SliceStable(quantStones, func(i, j int) bool {
		return quantStones[i].Multiplier > quantStones[j].Multiplier
	})

	tachProducts := make([]float64, len(tachStones)+1)
	tachProducts[0] = 1.0
	for i := 1; i <= len(tachStones); i++ {
		tachProducts[i] = tachProducts[i-1] * tachStones[i-1].Multiplier
	}

	quantProducts := make([]float64, len(quantStones)+1)
	quantProducts[0] = 1.0
	for i := 1; i <= len(quantStones); i++ {
		quantProducts[i] = quantProducts[i-1] * quantStones[i-1].Multiplier
	}

	bestDelivery := 0.0
	bestLaying := 0.0
	bestShipping := 0.0
	var bestLoadout []*ei.CompleteArtifact
	bestTachCount := 0
	bestQuantCount := 0

	var dfs func(idx int, current []*ei.CompleteArtifact, usedFamilies map[ei.ArtifactSpec_Name]bool)
	dfs = func(idx int, current []*ei.CompleteArtifact, usedFamilies map[ei.ArtifactSpec_Name]bool) {
		if len(current) > 0 {
			if gussetRequired {
				hasGusset := false
				for _, a := range current {
					if a.GetSpec().GetName() == ei.ArtifactSpec_ORNATE_GUSSET {
						hasGusset = true
						break
					}
				}
				if !hasGusset && len(current) == maxSlots {
					return
				}
			}

			metronomeMult := 1.0
			compassMult := 1.0
			gussetMult := 1.0
			totalSlots := 0

			for _, a := range current {
				spec := a.GetSpec()
				tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
				switch spec.GetName() {
				case ei.ArtifactSpec_QUANTUM_METRONOME:
					if b, ok := metronomeBuffs[tierStr]; ok {
						metronomeMult *= b
					}
				case ei.ArtifactSpec_INTERSTELLAR_COMPASS:
					if b, ok := compassBuffs[tierStr]; ok {
						compassMult *= b
					}
				case ei.ArtifactSpec_ORNATE_GUSSET:
					if b, ok := gussetBuffs[tierStr]; ok {
						gussetMult *= b
					}
				}
				totalSlots += getArtifactSlotCount(spec)
			}

			baseLaying := baseEggLayingRate * metronomeMult * colBuffs.ELR
			if habPop == 0 {
				baseLaying *= gussetMult
			}
			baseShipping := baseShippingRate * compassMult * colBuffs.SR

			maxTach := min(totalSlots, len(tachStones))
			for t := 0; t <= maxTach; t++ {
				q := min(totalSlots-t, len(quantStones))
				lay := baseLaying * tachProducts[t]
				ship := baseShipping * quantProducts[q]
				delivery := math.Min(lay, ship)

				if delivery > bestDelivery || (delivery > 0 && math.Abs(delivery-bestDelivery) < 1e-9 && (len(current) > len(bestLoadout) || (t+q) > (bestTachCount+bestQuantCount))) {
					bestDelivery = delivery
					bestLaying = lay
					bestShipping = ship
					bestLoadout = append([]*ei.CompleteArtifact(nil), current...)
					bestTachCount = t
					bestQuantCount = q
				}
			}
		}

		if idx >= len(hosts) || len(current) >= maxSlots {
			return
		}

		for i := idx; i < len(hosts); i++ {
			fam := hosts[i].GetSpec().GetName()
			if usedFamilies[fam] {
				continue
			}
			usedFamilies[fam] = true
			dfs(i+1, append(current, hosts[i]), usedFamilies)
			delete(usedFamilies, fam)
		}
	}

	used := make(map[ei.ArtifactSpec_Name]bool)
	if gussetRequired && qualifyingGusset != nil {
		used[ei.ArtifactSpec_ORNATE_GUSSET] = true
		dfs(0, []*ei.CompleteArtifact{qualifyingGusset}, used)
	} else {
		dfs(0, nil, used)
	}

	// Reslot the bestLoadout with top stones
	chosenStones := make([]StoneCandidate, 0, bestTachCount+bestQuantCount)
	chosenStones = append(chosenStones, tachStones[:bestTachCount]...)
	chosenStones = append(chosenStones, quantStones[:bestQuantCount]...)

	res.OptimalArtifacts = reslotArtifacts(bestLoadout, chosenStones)
	res.LayingRate = bestLaying
	res.ShippingRate = bestShipping
	res.DeliveryRate = bestDelivery
	res.OptimalStones = groupStones(chosenStones)

	// Calculate current rates from currently equipped artifacts
	currentArtifacts := getEquippedArtifacts(backup, farm)
	currBuffs := ei.GetArtifactBuffs(currentArtifacts)
	res.CurrentLayingRate = baseEggLayingRate * currBuffs.ELR * colBuffs.ELR
	if habPop == 0 {
		res.CurrentLayingRate *= currBuffs.Hab
	}
	res.CurrentShippingRate = baseShippingRate * currBuffs.SR * colBuffs.SR
	res.CurrentDeliveryRate = math.Min(res.CurrentLayingRate, res.CurrentShippingRate)

	diff := math.Abs(res.DeliveryRate - res.CurrentDeliveryRate)
	res.IsAlreadyOptimal = diff < 1e-4 || (res.DeliveryRate > 0 && diff/res.DeliveryRate < 1e-4) || res.CurrentDeliveryRate >= res.DeliveryRate
	if isSameArtifactLoadout(currentArtifacts, res.OptimalArtifacts) {
		res.OptimalArtifacts = currentArtifacts
		res.IsAlreadyOptimal = true
	}

	return res
}

func collectDeliveryCandidates(inventory []*ei.ArtifactInventoryItem) ([]*ei.CompleteArtifact, []StoneCandidate, []StoneCandidate) {
	bestByFamily := make(map[ei.ArtifactSpec_Name]*ei.CompleteArtifact)
	var tachStones []StoneCandidate
	var quantStones []StoneCandidate

	for _, item := range inventory {
		if item == nil || item.GetArtifact() == nil {
			continue
		}
		art := item.GetArtifact()
		spec := art.GetSpec()
		if spec == nil {
			continue
		}

		if isStone(spec.GetName()) {
			lvl := int(spec.GetLevel())
			qty := int(item.GetQuantity())
			if qty < 1 {
				qty = 1
			}
			if spec.GetName() == ei.ArtifactSpec_TACHYON_STONE && lvl >= 0 && lvl < len(tachyonStoneBuffs) {
				for i := 0; i < qty; i++ {
					tachStones = append(tachStones, StoneCandidate{Spec: spec, Multiplier: tachyonStoneBuffs[lvl]})
				}
			} else if spec.GetName() == ei.ArtifactSpec_QUANTUM_STONE && lvl >= 0 && lvl < len(quantumStoneBuffs) {
				for i := 0; i < qty; i++ {
					quantStones = append(quantStones, StoneCandidate{Spec: spec, Multiplier: quantumStoneBuffs[lvl]})
				}
			}
			continue
		}

		for _, st := range art.GetStones() {
			if st != nil {
				lvl := int(st.GetLevel())
				if st.GetName() == ei.ArtifactSpec_TACHYON_STONE && lvl >= 0 && lvl < len(tachyonStoneBuffs) {
					tachStones = append(tachStones, StoneCandidate{Spec: st, Multiplier: tachyonStoneBuffs[lvl]})
				} else if st.GetName() == ei.ArtifactSpec_QUANTUM_STONE && lvl >= 0 && lvl < len(quantumStoneBuffs) {
					quantStones = append(quantStones, StoneCandidate{Spec: st, Multiplier: quantumStoneBuffs[lvl]})
				}
			}
		}

		slots := getArtifactSlotCount(spec)
		isDirectBuff := spec.GetName() == ei.ArtifactSpec_QUANTUM_METRONOME ||
			spec.GetName() == ei.ArtifactSpec_INTERSTELLAR_COMPASS ||
			spec.GetName() == ei.ArtifactSpec_ORNATE_GUSSET

		if !isDirectBuff && slots == 0 {
			continue
		}

		curBest, exists := bestByFamily[spec.GetName()]
		if !exists || deliveryScore(spec, slots) > deliveryScore(curBest.GetSpec(), getArtifactSlotCount(curBest.GetSpec())) {
			bestByFamily[spec.GetName()] = art
		}
	}

	hosts := make([]*ei.CompleteArtifact, 0, len(bestByFamily))
	for _, a := range bestByFamily {
		hosts = append(hosts, a)
	}

	sort.SliceStable(hosts, func(i, j int) bool {
		si := hosts[i].GetSpec()
		sj := hosts[j].GetSpec()
		priI := deliveryPriority(si.GetName())
		priJ := deliveryPriority(sj.GetName())
		if priI != priJ {
			return priI < priJ
		}
		return getArtifactSlotCount(si) > getArtifactSlotCount(sj)
	})

	return hosts, tachStones, quantStones
}

func deliveryPriority(name ei.ArtifactSpec_Name) int {
	switch name {
	case ei.ArtifactSpec_QUANTUM_METRONOME:
		return 1
	case ei.ArtifactSpec_INTERSTELLAR_COMPASS:
		return 2
	case ei.ArtifactSpec_ORNATE_GUSSET:
		return 3
	default:
		return 4
	}
}

func deliveryScore(spec *ei.ArtifactSpec, slots int) float64 {
	score := 1.0
	tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
	switch spec.GetName() {
	case ei.ArtifactSpec_QUANTUM_METRONOME:
		if b, ok := metronomeBuffs[tierStr]; ok {
			score *= b
		}
	case ei.ArtifactSpec_INTERSTELLAR_COMPASS:
		if b, ok := compassBuffs[tierStr]; ok {
			score *= b
		}
	case ei.ArtifactSpec_ORNATE_GUSSET:
		if b, ok := gussetBuffs[tierStr]; ok {
			score *= b
		}
	}
	return score * math.Pow(1.05, float64(slots))
}

func groupStones(stones []StoneCandidate) []StoneGroup {
	type key struct {
		name  ei.ArtifactSpec_Name
		level ei.ArtifactSpec_Level
	}
	counts := make(map[key]int)
	specMap := make(map[key]*ei.ArtifactSpec)

	for _, s := range stones {
		if s.Spec == nil {
			continue
		}
		k := key{name: s.Spec.GetName(), level: s.Spec.GetLevel()}
		counts[k]++
		specMap[k] = s.Spec
	}

	var groups []StoneGroup
	for k, cnt := range counts {
		spec := specMap[k]
		tier := int(spec.GetLevel()) + 2
		delta := 0.0
		lvl := int(spec.GetLevel())
		nameStr := ""

		switch spec.GetName() {
		case ei.ArtifactSpec_TACHYON_STONE:
			nameStr = "Tachyon Stone"
			if lvl >= 0 && lvl < len(tachyonStoneBuffs) {
				delta = (tachyonStoneBuffs[lvl] - 1.0) * 100
			}
		case ei.ArtifactSpec_QUANTUM_STONE:
			nameStr = "Quantum Stone"
			if lvl >= 0 && lvl < len(quantumStoneBuffs) {
				delta = (quantumStoneBuffs[lvl] - 1.0) * 100
			}
		case ei.ArtifactSpec_LUNAR_STONE:
			nameStr = "Lunar Stone"
			if lvl >= 0 && lvl < len(lunarStoneBuffs) {
				delta = (lunarStoneBuffs[lvl] - 1.0) * 100
			}
		case ei.ArtifactSpec_SHELL_STONE:
			nameStr = "Shell Stone"
			if lvl >= 0 && lvl < len(shellStoneBuffs) {
				delta = (shellStoneBuffs[lvl] - 1.0) * 100
			}
		case ei.ArtifactSpec_CLARITY_STONE:
			nameStr = "Clarity Stone"
			if lvl >= 0 && lvl < len(clarityStoneBuffs) {
				delta = clarityStoneBuffs[lvl] * 100
			}
		}

		plural := "s"
		if cnt == 1 {
			plural = ""
		}
		label := fmt.Sprintf("%d T%d %s%s (+%d%% each)", cnt, tier, nameStr, plural, int(math.Round(delta)))
		groups = append(groups, StoneGroup{
			Spec:        spec,
			Count:       cnt,
			EffectDelta: delta,
			Emoji:       getStoneEmojiMarkdown(spec.GetName(), spec.GetLevel()),
			Label:       label,
		})
	}

	sort.SliceStable(groups, func(i, j int) bool {
		si := groups[i].Spec
		sj := groups[j].Spec
		if si.GetName() != sj.GetName() {
			return stoneOrder(si.GetName()) < stoneOrder(sj.GetName())
		}
		return si.GetLevel() > sj.GetLevel()
	})

	return groups
}

func stoneOrder(name ei.ArtifactSpec_Name) int {
	switch name {
	case ei.ArtifactSpec_LUNAR_STONE:
		return 1
	case ei.ArtifactSpec_TACHYON_STONE:
		return 2
	case ei.ArtifactSpec_QUANTUM_STONE:
		return 3
	case ei.ArtifactSpec_SHELL_STONE:
		return 4
	default:
		return 5
	}
}

func getStoneEmojiMarkdown(name ei.ArtifactSpec_Name, level ei.ArtifactSpec_Level) string {
	tier := int(level) + 2
	prefix := ""
	switch name {
	case ei.ArtifactSpec_TACHYON_STONE:
		prefix = "st_t"
	case ei.ArtifactSpec_QUANTUM_STONE:
		prefix = "st_q"
	case ei.ArtifactSpec_LUNAR_STONE:
		prefix = "st_lu"
	case ei.ArtifactSpec_SHELL_STONE:
		prefix = "st_sh"
	case ei.ArtifactSpec_LIFE_STONE:
		prefix = "st_li"
	case ei.ArtifactSpec_DILITHIUM_STONE:
		prefix = "st_di"
	case ei.ArtifactSpec_SOUL_STONE:
		prefix = "st_so"
	case ei.ArtifactSpec_PROPHECY_STONE:
		prefix = "st_pr"
	case ei.ArtifactSpec_CLARITY_STONE:
		prefix = "st_cl"
	case ei.ArtifactSpec_TERRA_STONE:
		prefix = "st_te"
	}
	if prefix != "" {
		emojiKey := fmt.Sprintf("%s%d", prefix, tier)
		if md, ok := ei.GetBotEmojiMarkdownIfExists(emojiKey); ok {
			return md
		}
		return ei.GetBotEmojiMarkdown(emojiKey)
	}
	return "💎"
}

func getArtifactEmojiMarkdown(spec *ei.ArtifactSpec) string {
	if spec == nil {
		return "✨"
	}
	lvl := int(spec.GetLevel())
	rar := int(spec.GetRarity())
	lvlStr := "T1"
	if lvl >= 0 && lvl < len(ei.ArtifactLevels) {
		lvlStr = ei.ArtifactLevels[lvl]
	}
	rarStr := "C"
	if rar >= 0 && rar < len(ei.ArtifactRarity) {
		rarStr = ei.ArtifactRarity[rar]
	}
	emojiName := fmt.Sprintf("%s%s%s", ei.ShortArtifactName[int32(spec.GetName())], lvlStr, rarStr)
	if md, ok := ei.GetBotEmojiMarkdownIfExists(emojiName); ok {
		return md
	}
	return ei.GetBotEmojiMarkdown(emojiName)
}

func formatMultiplier(val float64) string {
	rounded := int64(math.Round(val))
	in := strconv.FormatInt(rounded, 10)
	var out []byte
	n := len(in)
	for i := 0; i < n; i++ {
		if i > 0 && (n-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, in[i])
	}
	return string(out)
}

func formatBuffModifier(v float64) string {
	if v == 1.0 {
		return ""
	}
	if v < 2.0 {
		pct := math.Round((v - 1.0) * 100)
		if pct > 0 {
			return fmt.Sprintf("+%v%%", pct)
		}
		return fmt.Sprintf("%v%%", pct)
	}
	pct := math.Round((v - 1.0) * 100)
	return fmt.Sprintf("+%v%% (%.2gx)", pct, v)
}

func formatArtifactWithStonesDisplay(a *ei.CompleteArtifact) string {
	if a == nil || a.GetSpec() == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(getArtifactEmojiMarkdown(a.GetSpec()))
	for _, st := range a.GetStones() {
		if st != nil {
			b.WriteString(getStoneEmojiMarkdown(st.GetName(), st.GetLevel()))
		}
	}
	return b.String()
}

func formatSingleArtifactBuffs(a *ei.CompleteArtifact, isDelivery bool) string {
	if a == nil || a.GetSpec() == nil {
		return ""
	}
	buffs := ei.GetArtifactBuffs([]*ei.CompleteArtifact{a})
	var parts []string

	if isDelivery {
		if buffs.ELR != 1 {
			parts = append(parts, fmt.Sprintf("%s ELR", formatBuffModifier(buffs.ELR)))
		}
		if buffs.SR != 1 {
			parts = append(parts, fmt.Sprintf("%s SR", formatBuffModifier(buffs.SR)))
		}
		if buffs.Hab != 1 {
			parts = append(parts, fmt.Sprintf("%s Hab", formatBuffModifier(buffs.Hab)))
		}
	} else {
		spec := a.GetSpec()
		tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
		if spec.GetName() == ei.ArtifactSpec_LUNAR_TOTEM {
			if mult, ok := totemBuffs[tierStr]; ok {
				parts = append(parts, fmt.Sprintf("%gx Away", mult))
			}
		} else if buffs.AwayEarnings != 1 {
			parts = append(parts, fmt.Sprintf("%s Away", formatBuffModifier(buffs.AwayEarnings)))
		}

		if buffs.Earnings != 1 {
			parts = append(parts, fmt.Sprintf("%s Value", formatBuffModifier(buffs.Earnings)))
		}
		if buffs.Hab != 1 {
			parts = append(parts, fmt.Sprintf("%s Hab", formatBuffModifier(buffs.Hab)))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

func formatAnyArtifactBuffs(a *ei.CompleteArtifact) string {
	if a == nil || a.GetSpec() == nil {
		return ""
	}
	buffs := ei.GetArtifactBuffs([]*ei.CompleteArtifact{a})
	var parts []string
	if buffs.ELR != 1 {
		parts = append(parts, fmt.Sprintf("%s ELR", formatBuffModifier(buffs.ELR)))
	}
	if buffs.SR != 1 {
		parts = append(parts, fmt.Sprintf("%s SR", formatBuffModifier(buffs.SR)))
	}
	if buffs.Hab != 1 {
		parts = append(parts, fmt.Sprintf("%s Hab", formatBuffModifier(buffs.Hab)))
	}
	if buffs.IHR != 1 {
		parts = append(parts, fmt.Sprintf("%s IHR", formatBuffModifier(buffs.IHR)))
	}
	spec := a.GetSpec()
	tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
	if spec.GetName() == ei.ArtifactSpec_LUNAR_TOTEM {
		if mult, ok := totemBuffs[tierStr]; ok {
			parts = append(parts, fmt.Sprintf("%gx Away", mult))
		}
	} else if buffs.AwayEarnings != 1 {
		parts = append(parts, fmt.Sprintf("%s Away", formatBuffModifier(buffs.AwayEarnings)))
	}
	if buffs.Earnings != 1 {
		parts = append(parts, fmt.Sprintf("%s Value", formatBuffModifier(buffs.Earnings)))
	}
	switch spec.GetName() {
	case ei.ArtifactSpec_AURELIAN_BROOCH:
		if b, ok := broochBuffs[tierStr]; ok {
			parts = append(parts, fmt.Sprintf("+%.0f%% Rewards", b*100))
		}
	case ei.ArtifactSpec_NEODYMIUM_MEDALLION:
		if b, ok := medallionBuffs[tierStr]; ok {
			parts = append(parts, fmt.Sprintf("+%.0f%% Frequency", b*100))
		}
	case ei.ArtifactSpec_BEAK_OF_MIDAS:
		if b, ok := beakBuffs[tierStr]; ok {
			ch := math.Min(1.0, 0.30*(1.0+b))
			if ch >= 0.999 {
				parts = append(parts, "100% GE Chance")
			} else {
				parts = append(parts, fmt.Sprintf("%.0f%% GE Chance", ch*100))
			}
		}
	}
	if len(parts) == 0 {
		slots := getArtifactSlotCount(spec)
		if slots > 0 {
			parts = append(parts, fmt.Sprintf("%d slots", slots))
		}
	}
	return strings.Join(parts, " · ")
}

func formatEquippedArtifactsSection(currentArtifacts []*ei.CompleteArtifact, isDelivery bool, layingRate, shippingRate, deliveryRate, currentMult float64) string {
	if len(currentArtifacts) == 0 {
		return "**Currently Equipped:** None\n\n"
	}
	var sb strings.Builder
	sb.WriteString("**Currently Equipped:**\n")
	for _, a := range currentArtifacts {
		icons := formatArtifactWithStonesDisplay(a)
		buffStr := formatSingleArtifactBuffs(a, isDelivery)
		if buffStr == "" {
			buffStr = formatAnyArtifactBuffs(a)
		}
		if buffStr != "" {
			fmt.Fprintf(&sb, "%s %s\n", icons, buffStr)
		} else {
			fmt.Fprintf(&sb, "%s\n", icons)
		}
	}
	if isDelivery {
		fmt.Fprintf(&sb, "-# 🐔 Laying: %s/hr · 🚚 Shipping: %s/hr · 📦 Delivery: **%s/hr**\n\n",
			ei.FormatEIValue(layingRate, map[string]any{"decimals": 2, "trim": true}),
			ei.FormatEIValue(shippingRate, map[string]any{"decimals": 2, "trim": true}),
			ei.FormatEIValue(deliveryRate, map[string]any{"decimals": 2, "trim": true}),
		)
	} else {
		fmt.Fprintf(&sb, "-# Total Multiplier: **%sx**\n\n", formatMultiplier(currentMult))
	}
	return sb.String()
}

func formatEquippedEarningsArtifactsSection(currentArtifacts []*ei.CompleteArtifact, currentMult, eventMult float64, hasActiveEvent bool, eventName string) string {
	if len(currentArtifacts) == 0 {
		return "**Currently Equipped:** None\n\n"
	}
	var sb strings.Builder
	sb.WriteString("**Currently Equipped:**\n")
	for _, a := range currentArtifacts {
		icons := formatArtifactWithStonesDisplay(a)
		buffStr := formatSingleArtifactBuffs(a, false)
		if buffStr == "" {
			buffStr = formatAnyArtifactBuffs(a)
		}
		if buffStr != "" {
			fmt.Fprintf(&sb, "%s %s\n", icons, buffStr)
		} else {
			fmt.Fprintf(&sb, "%s\n", icons)
		}
	}
	if hasActiveEvent && eventMult > 1.0 {
		fmt.Fprintf(&sb, "-# Total Multiplier: **%sx** (**%sx** with %s)\n\n",
			formatMultiplier(currentMult), formatMultiplier(currentMult*eventMult), eventName)
	} else {
		fmt.Fprintf(&sb, "-# Total Multiplier: **%sx**\n\n", formatMultiplier(currentMult))
	}
	return sb.String()
}

func buildEarningsFormula(artifacts []*ei.CompleteArtifact, totalMultiplier, eventMult float64, hasActiveEvent bool, colBuffs ei.DimensionBuffs, colOverview ei.ColleggtiblesOverview) string {
	var terms []string
	for _, a := range artifacts {
		if a == nil || a.GetSpec() == nil {
			continue
		}
		spec := a.GetSpec()
		tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
		emoji := getArtifactEmojiMarkdown(spec)
		switch spec.GetName() {
		case ei.ArtifactSpec_LUNAR_TOTEM:
			if b, ok := totemBuffs[tierStr]; ok {
				terms = append(terms, fmt.Sprintf("%s %.0fx", emoji, b))
			}
		case ei.ArtifactSpec_DEMETERS_NECKLACE:
			if b, ok := necklaceBuffs[tierStr]; ok {
				terms = append(terms, fmt.Sprintf("%s %.2fx", emoji, b))
			}
		case ei.ArtifactSpec_TUNGSTEN_ANKH:
			if b, ok := ankhBuffs[tierStr]; ok {
				terms = append(terms, fmt.Sprintf("%s %.2fx", emoji, b))
			}
		}
	}

	stoneMult := 1.0
	stoneCount := 0
	var stoneSpec *ei.ArtifactSpec
	for _, a := range artifacts {
		for _, st := range a.GetStones() {
			if st == nil {
				continue
			}
			lvl := int(st.GetLevel())
			switch st.GetName() {
			case ei.ArtifactSpec_LUNAR_STONE:
				if lvl >= 0 && lvl < len(lunarStoneBuffs) {
					stoneMult *= lunarStoneBuffs[lvl]
					stoneCount++
					stoneSpec = st
				}
			case ei.ArtifactSpec_SHELL_STONE:
				if lvl >= 0 && lvl < len(shellStoneBuffs) {
					stoneMult *= shellStoneBuffs[lvl]
					stoneCount++
					stoneSpec = st
				}
			}
		}
	}
	if stoneCount > 0 && stoneSpec != nil {
		stoneEmoji := getStoneEmojiMarkdown(stoneSpec.GetName(), stoneSpec.GetLevel())
		terms = append(terms, fmt.Sprintf("%s %d× (%.2fx)", stoneEmoji, stoneCount, stoneMult))
	}

	// Colleggtibles contributing to earnings
	colEarnTerms := getColleggtibleFormulaTerms(colOverview, ei.GameModifier_EARNINGS, ei.GameModifier_AWAY_EARNINGS)
	if len(colEarnTerms) > 0 {
		terms = append(terms, colEarnTerms...)
	} else {
		if colBuffs.Earnings > 1.0001 {
			terms = append(terms, fmt.Sprintf("%s %.2fx", ei.GetBotEmojiMarkdown("collegg"), colBuffs.Earnings))
		}
		if colBuffs.AwayEarnings > 1.0001 {
			terms = append(terms, fmt.Sprintf("%s %.2fx", ei.GetBotEmojiMarkdown("collegg"), colBuffs.AwayEarnings))
		}
	}

	if len(terms) == 0 {
		if hasActiveEvent && eventMult > 1.0 {
			return fmt.Sprintf("**Total Multiplier**: %sx (× %.0fx Event = **%sx**)", formatMultiplier(totalMultiplier), eventMult, formatMultiplier(totalMultiplier*eventMult))
		}
		return fmt.Sprintf("**Total Multiplier**: %sx", formatMultiplier(totalMultiplier))
	}
	baseFormula := fmt.Sprintf("%s = **%sx**", strings.Join(terms, " × "), formatMultiplier(totalMultiplier))
	if hasActiveEvent && eventMult > 1.0 {
		return fmt.Sprintf("%s (× %.0fx Event = **%sx**)", baseFormula, eventMult, formatMultiplier(totalMultiplier*eventMult))
	}
	return baseFormula
}

func getColleggtibleFormulaTerms(overview ei.ColleggtiblesOverview, dims ...ei.GameModifier_GameDimension) []string {
	var terms []string
	dimSet := make(map[ei.GameModifier_GameDimension]bool, len(dims))
	for _, d := range dims {
		dimSet[d] = true
	}

	for _, d := range overview.Dimensions {
		if !dimSet[d.Dimension] {
			continue
		}
		for _, egg := range d.Colleggtibles {
			if egg.Tier >= 0 && egg.CurrentValue > 1.0001 {
				emoji := ei.FindEggEmoji(egg.EggID)
				terms = append(terms, fmt.Sprintf("%s %.2fx", emoji, egg.CurrentValue))
			}
		}
	}
	return terms
}

func buildDeliveryFormula(artifacts []*ei.CompleteArtifact, deliveryRate float64, colBuffs ei.DimensionBuffs, colOverview ei.ColleggtiblesOverview) string {
	metronomeMult := 1.0
	compassMult := 1.0
	gussetMult := 1.0
	tachCount := 0
	tachMult := 1.0
	quantCount := 0
	quantMult := 1.0

	var metrEmoji, compEmoji, gussetEmoji, tachEmoji, quantEmoji string

	for _, a := range artifacts {
		if a == nil || a.GetSpec() == nil {
			continue
		}
		spec := a.GetSpec()
		tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
		switch spec.GetName() {
		case ei.ArtifactSpec_QUANTUM_METRONOME:
			if b, ok := metronomeBuffs[tierStr]; ok {
				metronomeMult = b
				metrEmoji = getArtifactEmojiMarkdown(spec)
			}
		case ei.ArtifactSpec_INTERSTELLAR_COMPASS:
			if b, ok := compassBuffs[tierStr]; ok {
				compassMult = b
				compEmoji = getArtifactEmojiMarkdown(spec)
			}
		case ei.ArtifactSpec_ORNATE_GUSSET:
			if b, ok := gussetBuffs[tierStr]; ok {
				gussetMult = b
				gussetEmoji = getArtifactEmojiMarkdown(spec)
			}
		}

		for _, st := range a.GetStones() {
			if st == nil {
				continue
			}
			lvl := int(st.GetLevel())
			switch st.GetName() {
			case ei.ArtifactSpec_TACHYON_STONE:
				if lvl >= 0 && lvl < len(tachyonStoneBuffs) {
					tachMult *= tachyonStoneBuffs[lvl]
					tachCount++
					if tachEmoji == "" {
						tachEmoji = getStoneEmojiMarkdown(st.GetName(), st.GetLevel())
					}
				}
			case ei.ArtifactSpec_QUANTUM_STONE:
				if lvl >= 0 && lvl < len(quantumStoneBuffs) {
					quantMult *= quantumStoneBuffs[lvl]
					quantCount++
					if quantEmoji == "" {
						quantEmoji = getStoneEmojiMarkdown(st.GetName(), st.GetLevel())
					}
				}
			}
		}
	}

	elrCollegg := colBuffs.ELR
	if elrCollegg <= 0 {
		elrCollegg = 1.0
	}
	srCollegg := colBuffs.SR
	if srCollegg <= 0 {
		srCollegg = 1.0
	}
	habCollegg := colBuffs.Hab
	if habCollegg <= 0 {
		habCollegg = 1.0
	}

	totalELR := metronomeMult * tachMult * elrCollegg
	totalSR := compassMult * quantMult * srCollegg
	totalHab := gussetMult * habCollegg

	// ELR details
	var elrTerms []string
	if metronomeMult != 1 {
		elrTerms = append(elrTerms, fmt.Sprintf("%s %.2fx", metrEmoji, metronomeMult))
	}
	if tachCount > 0 {
		elrTerms = append(elrTerms, fmt.Sprintf("%s %d× %.2fx", tachEmoji, tachCount, tachMult))
	}
	colElrTerms := getColleggtibleFormulaTerms(colOverview, ei.GameModifier_EGG_LAYING_RATE)
	if len(colElrTerms) > 0 {
		elrTerms = append(elrTerms, colElrTerms...)
	} else if elrCollegg > 1.0001 {
		elrTerms = append(elrTerms, fmt.Sprintf("%s %.2fx", ei.GetBotEmojiMarkdown("collegg"), elrCollegg))
	}

	var elrStr string
	if totalELR > 1.0001 || len(elrTerms) > 0 {
		elrDetails := ""
		if len(elrTerms) > 0 {
			elrDetails = fmt.Sprintf(" (%s)", strings.Join(elrTerms, " × "))
		}
		elrStr = fmt.Sprintf("🐔 Laying: **%s**%s", ei.FormatModifierValue(totalELR), elrDetails)
	}

	// Hab details (coupled with Laying)
	var habTerms []string
	if gussetMult != 1 {
		habTerms = append(habTerms, fmt.Sprintf("%s %.2fx", gussetEmoji, gussetMult))
	}
	colHabTerms := getColleggtibleFormulaTerms(colOverview, ei.GameModifier_HAB_CAPACITY)
	if len(colHabTerms) > 0 {
		habTerms = append(habTerms, colHabTerms...)
	} else if habCollegg > 1.0001 {
		habTerms = append(habTerms, fmt.Sprintf("%s %.2fx", ei.GetBotEmojiMarkdown("collegg"), habCollegg))
	}

	var habStr string
	if totalHab > 1.0001 || len(habTerms) > 0 {
		habDetails := ""
		if len(habTerms) > 0 {
			habDetails = fmt.Sprintf(" (%s)", strings.Join(habTerms, " × "))
		}
		habStr = fmt.Sprintf("🏠 Habs: **%s**%s", ei.FormatModifierValue(totalHab), habDetails)
	}

	// Couple Laying and Habs together with multiplication
	var layingStr string
	if elrStr != "" && habStr != "" {
		layingStr = fmt.Sprintf("%s × %s", elrStr, habStr)
	} else if elrStr != "" {
		layingStr = elrStr
	} else if habStr != "" {
		layingStr = habStr
	}

	// SR details
	var srTerms []string
	if compassMult != 1 {
		srTerms = append(srTerms, fmt.Sprintf("%s %.2fx", compEmoji, compassMult))
	}
	if quantCount > 0 {
		srTerms = append(srTerms, fmt.Sprintf("%s %d× %.2fx", quantEmoji, quantCount, quantMult))
	}
	colSrTerms := getColleggtibleFormulaTerms(colOverview, ei.GameModifier_SHIPPING_CAPACITY)
	if len(colSrTerms) > 0 {
		srTerms = append(srTerms, colSrTerms...)
	} else if srCollegg > 1.0001 {
		srTerms = append(srTerms, fmt.Sprintf("%s %.2fx", ei.GetBotEmojiMarkdown("collegg"), srCollegg))
	}

	var srStr string
	if totalSR > 1.0001 || len(srTerms) > 0 {
		srDetails := ""
		if len(srTerms) > 0 {
			srDetails = fmt.Sprintf(" (%s)", strings.Join(srTerms, " × "))
		}
		srStr = fmt.Sprintf("🚚 Shipping: **%s**%s", ei.FormatModifierValue(totalSR), srDetails)
	}

	var parts []string
	if layingStr != "" {
		parts = append(parts, layingStr)
	}
	if srStr != "" {
		parts = append(parts, srStr)
	}

	return strings.Join(parts, "\n")
}

func formatContributingColleggtibles(backup *ei.Backup, targetUserID string, isDelivery bool) string {
	storedCollegg := farmerstate.GetMiscSettingString(targetUserID, "collegg")
	overview := ei.GetColleggtiblesOverview(backup.GetContracts(), storedCollegg)

	var sb strings.Builder
	hasAny := false

	if isDelivery {
		dimMap := map[ei.GameModifier_GameDimension]string{
			ei.GameModifier_SHIPPING_CAPACITY: "🚚 Shipping",
			ei.GameModifier_EGG_LAYING_RATE:   "🐔 Laying",
			ei.GameModifier_HAB_CAPACITY:      "🏠 Habs",
		}
		dimOrder := []ei.GameModifier_GameDimension{
			ei.GameModifier_SHIPPING_CAPACITY,
			ei.GameModifier_EGG_LAYING_RATE,
			ei.GameModifier_HAB_CAPACITY,
		}

		for _, d := range dimOrder {
			for _, dim := range overview.Dimensions {
				if dim.Dimension != d {
					continue
				}
				var eggItems []string
				for _, egg := range dim.Colleggtibles {
					if egg.Tier >= 0 && egg.CurrentValue > 1.0 {
						emoji := ei.FindEggEmoji(egg.EggID)
						eggItems = append(eggItems, fmt.Sprintf("%s %s", emoji, egg.CurrentValueString))
					}
				}
				if len(eggItems) > 0 {
					if !hasAny {
						fmt.Fprintf(&sb, "%s **Colleggtibles Contributing to Delivery:**\n", ei.GetBotEmojiMarkdown("collegg"))
						hasAny = true
					}
					fmt.Fprintf(&sb, "-# %s (%s): %s\n", dimMap[d], dim.CurrentString, strings.Join(eggItems, " "))
				}
			}
		}
	} else {
		dimMap := map[ei.GameModifier_GameDimension]string{
			ei.GameModifier_EARNINGS:      "💎 Earnings",
			ei.GameModifier_AWAY_EARNINGS: "💤 Away Earnings",
		}
		dimOrder := []ei.GameModifier_GameDimension{
			ei.GameModifier_EARNINGS,
			ei.GameModifier_AWAY_EARNINGS,
		}

		for _, d := range dimOrder {
			for _, dim := range overview.Dimensions {
				if dim.Dimension != d {
					continue
				}
				var eggItems []string
				for _, egg := range dim.Colleggtibles {
					if egg.Tier >= 0 && egg.CurrentValue > 1.0 {
						emoji := ei.FindEggEmoji(egg.EggID)
						eggItems = append(eggItems, fmt.Sprintf("%s %s", emoji, egg.CurrentValueString))
					}
				}
				if len(eggItems) > 0 {
					if !hasAny {
						fmt.Fprintf(&sb, "%s **Colleggtibles Contributing to Earnings:**\n", ei.GetBotEmojiMarkdown("collegg"))
						hasAny = true
					}
					fmt.Fprintf(&sb, "-# %s (%s): %s\n", dimMap[d], dim.CurrentString, strings.Join(eggItems, " "))
				}
			}
		}
	}

	return sb.String()
}

func printEarningsSetup(userID string, backup *ei.Backup, farm *ei.Backup_Simulation, avatarURL string, actAsNotice string) []dc.LayoutComponent {
	farmTypeStr, eggEmote := formatFarmTypeHeader(farm)
	displayName := ei.NormalizePlayerNameForDisplay(backup.GetUserName())
	isVirtue := farm != nil && farm.GetEggType() >= ei.Egg_CURIOSITY && farm.GetEggType() <= ei.Egg_KINDNESS

	header := strings.Builder{}
	artifactsStr := strings.Builder{}
	footer := strings.Builder{}

	fmt.Fprint(&header, "# Optimal Earnings Setup\n")
	fmt.Fprintf(&header, "**__%s__**\n", displayName)
	fmt.Fprintf(&header, "**Farm Type**: %s %s\n", eggEmote, farmTypeStr)

	currentArtifacts := getEquippedArtifacts(backup, farm)
	currIcons := formatArtifactIcons(currentArtifacts)

	var optArtifacts []*ei.CompleteArtifact
	var optIcons string
	var isOptimal bool
	var totalMultiplier float64
	var currentMultiplier float64
	var gussetNote string

	if isVirtue {
		cte := ei.CalculateClothedTE(backup)
		maxCTEResult := ei.CalculateMaxClothedTEWithSlotHint(backup, len(currentArtifacts))
		maxCTE := maxCTEResult.ClothedTE
		cteDelta := maxCTE - cte

		optArtifacts = maxCTEResult.Artifacts
		optIcons = formatArtifactIcons(optArtifacts)
		isOptimal = currIcons == optIcons || cteDelta <= 0 || isSameArtifactLoadout(currentArtifacts, optArtifacts)
		totalMultiplier = maxCTE
		currentMultiplier = cte

		if isOptimal {
			artifactsStr.WriteString("**Currently Equipped:**\n")
			for _, a := range currentArtifacts {
				icons := formatArtifactWithStonesDisplay(a)
				buffStr := formatSingleArtifactBuffs(a, false)
				if buffStr == "" {
					buffStr = formatAnyArtifactBuffs(a)
				}
				if buffStr != "" {
					fmt.Fprintf(&artifactsStr, "%s %s\n", icons, buffStr)
				} else {
					fmt.Fprintf(&artifactsStr, "%s\n", icons)
				}
			}
			fmt.Fprintf(&artifactsStr, "Formula: %s\n", buildEarningsFormula(currentArtifacts, totalMultiplier, 1.0, false, ei.DimensionBuffs{}, ei.ColleggtiblesOverview{}))
			fmt.Fprintf(&artifactsStr, "**Max CTE**: **%.0f**\n\n", maxCTE)
			artifactsStr.WriteString("✅ **Your current setup is already optimal!**\n\n")
		} else {
			// Currently equipped section
			artifactsStr.WriteString(formatEquippedArtifactsSection(currentArtifacts, false, 0, 0, 0, currentMultiplier))

			// Optimal setup section
			artifactsStr.WriteString("**Optimal Setup:**\n")
			for _, a := range optArtifacts {
				icons := formatArtifactWithStonesDisplay(a)
				buffStr := formatSingleArtifactBuffs(a, false)
				if buffStr == "" {
					buffStr = formatAnyArtifactBuffs(a)
				}
				if buffStr != "" {
					fmt.Fprintf(&artifactsStr, "%s %s\n", icons, buffStr)
				} else {
					fmt.Fprintf(&artifactsStr, "%s\n", icons)
				}
			}
			fmt.Fprintf(&artifactsStr, "Formula: %s\n", buildEarningsFormula(optArtifacts, totalMultiplier, 1.0, false, ei.DimensionBuffs{}, ei.ColleggtiblesOverview{}))
			fmt.Fprintf(&artifactsStr, "**Max CTE**: **%.0f** (Current: %.0f)\n\n", maxCTE, cte)
		}
	} else {
		opt := OptimizeEarnings(backup, farm)
		storedCollegg := farmerstate.GetMiscSettingString(userID, "collegg")
		if storedCollegg != "" {
			overview := ei.GetColleggtiblesOverview(backup.GetContracts(), storedCollegg)
			oldColleggMult := 1.0
			if opt.ColleggtibleBuffs.Earnings > 0 {
				oldColleggMult *= opt.ColleggtibleBuffs.Earnings
			}
			if opt.ColleggtibleBuffs.AwayEarnings > 0 {
				oldColleggMult *= opt.ColleggtibleBuffs.AwayEarnings
			}

			opt.ColleggtibleBuffs = overview.Buffs
			opt.ColleggtiblesOverview = overview

			newColleggMult := 1.0
			if opt.ColleggtibleBuffs.Earnings > 0 {
				newColleggMult *= opt.ColleggtibleBuffs.Earnings
			}
			if opt.ColleggtibleBuffs.AwayEarnings > 0 {
				newColleggMult *= opt.ColleggtibleBuffs.AwayEarnings
			}

			if oldColleggMult > 0 {
				opt.TotalMultiplier = (opt.TotalMultiplier / oldColleggMult) * newColleggMult
				opt.CurrentMultiplier = (opt.CurrentMultiplier / oldColleggMult) * newColleggMult
			}
		}

		optArtifacts = opt.OptimalArtifacts
		optIcons = formatArtifactIcons(optArtifacts)
		isOptimal = opt.IsAlreadyOptimal
		totalMultiplier = opt.TotalMultiplier
		currentMultiplier = opt.CurrentMultiplier

		eventLabel := opt.EventMessage
		if eventLabel == "" {
			eventLabel = fmt.Sprintf("%.0fx Event", opt.EventMultiplier)
		}

		if opt.GussetRequired {
			gussetNote = fmt.Sprintf("-# ⚠️ **Gusset Required:** Current population (%s) exceeds base hab capacity (%s) — loadout includes %s.\n",
				ei.FormatEIValue(opt.HabPopulation, map[string]any{"decimals": 2, "trim": true}),
				ei.FormatEIValue(opt.BaseHabCapacity, map[string]any{"decimals": 2, "trim": true}),
				opt.GussetRequiredName,
			)
		}

		if isOptimal {
			artifactsStr.WriteString("**Currently Equipped:**\n")
			for _, a := range currentArtifacts {
				icons := formatArtifactWithStonesDisplay(a)
				buffStr := formatSingleArtifactBuffs(a, false)
				if buffStr == "" {
					buffStr = formatAnyArtifactBuffs(a)
				}
				if buffStr != "" {
					fmt.Fprintf(&artifactsStr, "%s %s\n", icons, buffStr)
				} else {
					fmt.Fprintf(&artifactsStr, "%s\n", icons)
				}
			}
			fmt.Fprintf(&artifactsStr, "Formula: %s\n", buildEarningsFormula(currentArtifacts, totalMultiplier, opt.EventMultiplier, opt.HasActiveEvent, opt.ColleggtibleBuffs, opt.ColleggtiblesOverview))
			if opt.HasActiveEvent && opt.EventMultiplier > 1.0 {
				fmt.Fprintf(&artifactsStr, "**Optimal Multiplier**: **%sx** (**%sx** with %s)\n\n",
					formatMultiplier(totalMultiplier), formatMultiplier(totalMultiplier*opt.EventMultiplier), eventLabel)
			} else {
				fmt.Fprintf(&artifactsStr, "**Optimal Multiplier**: **%sx**\n\n", formatMultiplier(totalMultiplier))
			}
			artifactsStr.WriteString("✅ **Your current setup is already optimal!**\n\n")
		} else {
			// Currently equipped section
			artifactsStr.WriteString(formatEquippedEarningsArtifactsSection(currentArtifacts, currentMultiplier, opt.EventMultiplier, opt.HasActiveEvent, eventLabel))

			// Optimal setup section
			artifactsStr.WriteString("**Optimal Setup:**\n")
			for _, a := range optArtifacts {
				icons := formatArtifactWithStonesDisplay(a)
				buffStr := formatSingleArtifactBuffs(a, false)
				if buffStr == "" {
					buffStr = formatAnyArtifactBuffs(a)
				}
				if buffStr != "" {
					fmt.Fprintf(&artifactsStr, "%s %s\n", icons, buffStr)
				} else {
					fmt.Fprintf(&artifactsStr, "%s\n", icons)
				}
			}
			fmt.Fprintf(&artifactsStr, "Formula: %s\n", buildEarningsFormula(optArtifacts, totalMultiplier, opt.EventMultiplier, opt.HasActiveEvent, opt.ColleggtibleBuffs, opt.ColleggtiblesOverview))
			if opt.HasActiveEvent && opt.EventMultiplier > 1.0 {
				fmt.Fprintf(&artifactsStr, "**Optimal Multiplier**: **%sx** (**%sx** with %s)\n\n",
					formatMultiplier(totalMultiplier), formatMultiplier(totalMultiplier*opt.EventMultiplier), eventLabel)
			} else {
				fmt.Fprintf(&artifactsStr, "**Optimal Multiplier**: **%sx**\n\n", formatMultiplier(totalMultiplier))
			}

			if currIcons != "" && optIcons != "" {
				diff := totalMultiplier - currentMultiplier
				if opt.HasActiveEvent && opt.EventMultiplier > 1.0 {
					fmt.Fprintf(&artifactsStr, "%s ➜ %s  **+%sx Earnings** (**+%sx** with %s)\n\n",
						currIcons, optIcons, formatMultiplier(diff), formatMultiplier(diff*opt.EventMultiplier), eventLabel)
				} else {
					fmt.Fprintf(&artifactsStr, "%s ➜ %s  **+%sx Earnings**\n\n",
						currIcons, optIcons, formatMultiplier(diff))
				}
			}
		}

		if opt.HasActiveEvent && opt.EventMultiplier > 1.0 {
			msg := opt.EventMessage
			if msg == "" {
				msg = fmt.Sprintf("%.0fx Earnings Event", opt.EventMultiplier)
			}
			fmt.Fprintf(&artifactsStr, "-# 🎉 **Active Periodicals Event:** %s amplifies earnings by **%.0fx** (ends <t:%d:R>).\n",
				msg, opt.EventMultiplier, opt.EventEndTime.Unix())
		}
	}

	// Contributing Colleggtibles section
	colleggStr := formatContributingColleggtibles(backup, userID, false)
	if colleggStr != "" {
		artifactsStr.WriteString(colleggStr)
		artifactsStr.WriteString("\n")
	}

	if gussetNote != "" {
		artifactsStr.WriteString(gussetNote)
	}

	// Footer
	now := time.Now()
	syncTime := time.Unix(int64(farm.GetLastStepTime()), 0)
	if farm.GetLastStepTime() == 0 {
		syncTime = time.Unix(int64(backup.GetApproxTime()), 0)
	}
	syncStr := fmt.Sprintf("Sync:%s", bottools.WrapTimestamp(syncTime.Unix(), bottools.TimestampShortTime))
	if now.Sub(syncTime) > 15*time.Minute {
		syncStr = fmt.Sprintf("⚠️ %s ⚠️", syncStr)
	}
	fmt.Fprintf(&footer, "-# Report:%s  %s\n",
		bottools.WrapTimestamp(now.Unix(), bottools.TimestampShortTime),
		syncStr)
	if actAsNotice != "" {
		fmt.Fprintf(&footer, "-# %s\n", actAsNotice)
	}

	avatar := avatarURL
	if avatar == "" {
		avatar = defaultFarmAvatarURL(farm)
	}

	var components []dc.LayoutComponent
	components = append(components, dc.Section{
		Components: []dc.TextDisplay{
			{Content: header.String()},
		},
		Accessory: dc.Thumbnail{
			URL: avatar,
		},
	})
	components = append(components, dc.Separator{
		Divider: true,
		Spacing: dc.SeparatorSpacingSmall,
	})
	components = append(components, dc.TextDisplay{
		Content: artifactsStr.String(),
	})
	components = append(components, dc.Separator{
		Divider: true,
		Spacing: dc.SeparatorSpacingSmall,
	})
	components = append(components, dc.TextDisplay{
		Content: footer.String(),
	})

	return components
}

func printDeliverySetup(userID string, backup *ei.Backup, farm *ei.Backup_Simulation, avatarURL string, actAsNotice string) []dc.LayoutComponent {
	farmTypeStr, eggEmote := formatFarmTypeHeader(farm)
	displayName := ei.NormalizePlayerNameForDisplay(backup.GetUserName())

	header := strings.Builder{}
	artifactsStr := strings.Builder{}
	footer := strings.Builder{}

	fmt.Fprint(&header, "# Optimal Production Setup\n")
	fmt.Fprintf(&header, "**__%s__**\n", displayName)
	fmt.Fprintf(&header, "**Farm Type**: %s %s\n\n", eggEmote, farmTypeStr)

	opt := OptimizeDelivery(backup, farm)
	storedCollegg := farmerstate.GetMiscSettingString(userID, "collegg")
	if storedCollegg != "" {
		overview := ei.GetColleggtiblesOverview(backup.GetContracts(), storedCollegg)
		opt.ColleggtibleBuffs = overview.Buffs
		opt.ColleggtiblesOverview = overview
	}
	currentArtifacts := getEquippedArtifacts(backup, farm)
	currIcons := formatArtifactIcons(currentArtifacts)
	optIcons := formatArtifactIcons(opt.OptimalArtifacts)

	if opt.IsAlreadyOptimal {
		artifactsStr.WriteString("**Currently Equipped:**\n")
		for _, a := range currentArtifacts {
			icons := formatArtifactWithStonesDisplay(a)
			buffStr := formatSingleArtifactBuffs(a, true)
			if buffStr == "" {
				buffStr = formatAnyArtifactBuffs(a)
			}
			if buffStr != "" {
				fmt.Fprintf(&artifactsStr, "%s %s\n", icons, buffStr)
			} else {
				fmt.Fprintf(&artifactsStr, "%s\n", icons)
			}
		}
		formulaStr := buildDeliveryFormula(currentArtifacts, opt.CurrentDeliveryRate, opt.ColleggtibleBuffs, opt.ColleggtiblesOverview)
		if formulaStr != "" {
			fmt.Fprintf(&artifactsStr, "Formulas:\n%s\n", formulaStr)
		}
		fmt.Fprintf(&artifactsStr, "📦 Delivery: **%s/hr** (🐔 Laying: %s/hr · 🚚 Shipping: %s/hr)\n\n",
			ei.FormatEIValue(opt.CurrentDeliveryRate, map[string]any{"decimals": 2, "trim": true}),
			ei.FormatEIValue(opt.CurrentLayingRate, map[string]any{"decimals": 2, "trim": true}),
			ei.FormatEIValue(opt.CurrentShippingRate, map[string]any{"decimals": 2, "trim": true}),
		)
		artifactsStr.WriteString("✅ **Your current setup is already optimal!**\n\n")
	} else {
		// Currently equipped section
		artifactsStr.WriteString(formatEquippedArtifactsSection(currentArtifacts, true, opt.CurrentLayingRate, opt.CurrentShippingRate, opt.CurrentDeliveryRate, 0))

		// Optimal setup section
		artifactsStr.WriteString("**Optimal Setup:**\n")
		for _, a := range opt.OptimalArtifacts {
			icons := formatArtifactWithStonesDisplay(a)
			buffStr := formatSingleArtifactBuffs(a, true)
			if buffStr == "" {
				buffStr = formatAnyArtifactBuffs(a)
			}
			if buffStr != "" {
				fmt.Fprintf(&artifactsStr, "%s %s\n", icons, buffStr)
			} else {
				fmt.Fprintf(&artifactsStr, "%s\n", icons)
			}
		}
		formulaStr := buildDeliveryFormula(opt.OptimalArtifacts, opt.DeliveryRate, opt.ColleggtibleBuffs, opt.ColleggtiblesOverview)
		if formulaStr != "" {
			fmt.Fprintf(&artifactsStr, "Formulas:\n%s\n", formulaStr)
		}
		fmt.Fprintf(&artifactsStr, "📦 Delivery: **%s/hr** (🐔 Laying: %s/hr · 🚚 Shipping: %s/hr)\n\n",
			ei.FormatEIValue(opt.DeliveryRate, map[string]any{"decimals": 2, "trim": true}),
			ei.FormatEIValue(opt.LayingRate, map[string]any{"decimals": 2, "trim": true}),
			ei.FormatEIValue(opt.ShippingRate, map[string]any{"decimals": 2, "trim": true}),
		)

		// Transition
		if currIcons != "" && optIcons != "" {
			delta := opt.DeliveryRate - opt.CurrentDeliveryRate
			fmt.Fprintf(&artifactsStr, "%s ➜ %s  **+%s/hr Delivery**\n\n", currIcons, optIcons, ei.FormatEIValue(delta, map[string]any{"decimals": 2, "trim": true}))
		}
	}

	// Contributing Colleggtibles section
	colleggStr := formatContributingColleggtibles(backup, userID, true)
	if colleggStr != "" {
		artifactsStr.WriteString(colleggStr)
		artifactsStr.WriteString("\n")
	}

	if opt.GussetRequired {
		fmt.Fprintf(&artifactsStr, "-# ⚠️ **Gusset Required:** Current population (%s) exceeds base hab capacity (%s) — loadout includes %s.\n",
			ei.FormatEIValue(opt.HabPopulation, map[string]any{"decimals": 2, "trim": true}),
			ei.FormatEIValue(opt.BaseHabCapacity, map[string]any{"decimals": 2, "trim": true}),
			opt.GussetRequiredName,
		)
	}

	// Footer
	now := time.Now()
	syncTime := time.Unix(int64(farm.GetLastStepTime()), 0)
	if farm.GetLastStepTime() == 0 {
		syncTime = time.Unix(int64(backup.GetApproxTime()), 0)
	}
	syncStr := fmt.Sprintf("Sync:%s", bottools.WrapTimestamp(syncTime.Unix(), bottools.TimestampShortTime))
	if now.Sub(syncTime) > 15*time.Minute {
		syncStr = fmt.Sprintf("⚠️ %s ⚠️", syncStr)
	}
	fmt.Fprintf(&footer, "-# Report:%s  %s\n",
		bottools.WrapTimestamp(now.Unix(), bottools.TimestampShortTime),
		syncStr)
	if actAsNotice != "" {
		fmt.Fprintf(&footer, "-# %s\n", actAsNotice)
	}

	avatar := avatarURL
	if avatar == "" {
		avatar = defaultFarmAvatarURL(farm)
	}

	var components []dc.LayoutComponent
	components = append(components, dc.Section{
		Components: []dc.TextDisplay{
			{Content: header.String()},
		},
		Accessory: dc.Thumbnail{
			URL: avatar,
		},
	})
	components = append(components, dc.Separator{
		Divider: true,
		Spacing: dc.SeparatorSpacingSmall,
	})
	components = append(components, dc.TextDisplay{
		Content: artifactsStr.String(),
	})
	components = append(components, dc.Separator{
		Divider: true,
		Spacing: dc.SeparatorSpacingSmall,
	})
	components = append(components, dc.TextDisplay{
		Content: footer.String(),
	})

	return components
}

func formatDroneBonus(mult float64) string {
	pct := math.Round((mult - 1.0) * 100)
	if pct >= 0 {
		return fmt.Sprintf("+%d%%", int(pct))
	}
	return fmt.Sprintf("%d%%", int(pct))
}

func getGoldenEggEmojiMarkdown() string {
	if md, ok := ei.GetBotEmojiMarkdownIfExists("icon_golden_egg"); ok {
		return md
	}
	if md, ok := ei.GetBotEmojiMarkdownIfExists("golden_egg"); ok {
		return md
	}
	return "🥚"
}

func formatDroneGEChance(chance float64) string {
	if chance >= 0.999 {
		return "100%"
	}
	return fmt.Sprintf("%.0f%%", chance*100)
}

func buildDroneFarmingFormula(broochMult, medallionMult, geChance, totalMult, eventMult float64, hasActiveEvent bool) string {
	bBonus := formatDroneBonus(broochMult)
	mBonus := formatDroneBonus(medallionMult)
	geStr := formatDroneGEChance(geChance)
	geMult := geChance / 0.30
	geEmoji := getGoldenEggEmojiMarkdown()

	baseFormula := fmt.Sprintf("🎁 Rewards: **%s** (%.2fx) × ⏱️ Frequency: **%s** (%.2fx) × %s GE Chance: **%s** (%.2fx) = **%.1fx**",
		bBonus, broochMult,
		mBonus, medallionMult,
		geEmoji, geStr, geMult,
		totalMult,
	)
	if hasActiveEvent && eventMult > 1.0 {
		return fmt.Sprintf("%s (× %.0fx Event = **%.1fx**)", baseFormula, eventMult, totalMult*eventMult)
	}
	return baseFormula
}

func formatDroneArtifactBuff(a *ei.CompleteArtifact) string {
	if a == nil || a.GetSpec() == nil {
		return ""
	}
	spec := a.GetSpec()
	tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]

	clarity := 0.0
	for _, st := range a.GetStones() {
		if st != nil && st.GetName() == ei.ArtifactSpec_CLARITY_STONE {
			lvl := int(st.GetLevel())
			if lvl >= 0 && lvl < len(clarityStoneBuffs) {
				clarity += clarityStoneBuffs[lvl]
			}
		}
	}

	switch spec.GetName() {
	case ei.ArtifactSpec_AURELIAN_BROOCH:
		base := broochBuffs[tierStr]
		eff := base * clarity
		if clarity > 0 {
			return fmt.Sprintf("+%.0f%% Rewards", eff*100)
		}
		return "+0% Rewards (Needs Clarity)"

	case ei.ArtifactSpec_NEODYMIUM_MEDALLION:
		base := medallionBuffs[tierStr]
		eff := base * clarity
		if clarity > 0 {
			return fmt.Sprintf("+%.0f%% Frequency", eff*100)
		}
		return "+0% Frequency (Needs Clarity)"

	case ei.ArtifactSpec_BEAK_OF_MIDAS:
		base := beakBuffs[tierStr]
		if clarity > 0 {
			ch := math.Min(1.0, 0.30*(1.0+base*clarity))
			if ch >= 0.999 {
				return "100% GE Chance"
			}
			return fmt.Sprintf("%.0f%% GE Chance", ch*100)
		}
		return "30% GE Chance (Needs Clarity)"

	default:
		return formatAnyArtifactBuffs(a)
	}
}

func formatEquippedDroneArtifactsSection(currentArtifacts []*ei.CompleteArtifact, broochMult, medallionMult, geChance, totalMult, eventMult float64, hasActiveEvent bool, eventName string) string {
	if len(currentArtifacts) == 0 {
		return "**Currently Equipped:** None\n\n"
	}
	var sb strings.Builder
	sb.WriteString("**Currently Equipped:**\n")
	for _, a := range currentArtifacts {
		icons := formatArtifactWithStonesDisplay(a)
		buffStr := formatDroneArtifactBuff(a)
		if buffStr != "" {
			fmt.Fprintf(&sb, "%s %s\n", icons, buffStr)
		} else {
			fmt.Fprintf(&sb, "%s\n", icons)
		}
	}
	geEmoji := getGoldenEggEmojiMarkdown()
	fmt.Fprintf(&sb, "-# 🎁 Rewards: %s (%.2fx) · ⏱️ Frequency: %s (%.2fx) · %s GE Chance: %s (%.2fx)\n",
		formatDroneBonus(broochMult), broochMult,
		formatDroneBonus(medallionMult), medallionMult,
		geEmoji, formatDroneGEChance(geChance), geChance/0.30,
	)
	if hasActiveEvent && eventMult > 1.0 {
		fmt.Fprintf(&sb, "%s Total GE Multiplier: **%.1fx** (**%.1fx** with %s)\n\n", geEmoji, totalMult, totalMult*eventMult, eventName)
	} else {
		fmt.Fprintf(&sb, "%s Total GE Multiplier: **%.1fx**\n\n", geEmoji, totalMult)
	}
	return sb.String()
}

// EarningsEventInfo holds active periodical earnings event information.
type EarningsEventInfo struct {
	HasActiveEvent bool
	Event          ei.EggEvent
	Multiplier     float64
	EventName      string
}

func getEarningsEventInfo(isUltra bool) EarningsEventInfo {
	ei.EventMutex.Lock()
	defer ei.EventMutex.Unlock()

	now := time.Now().UTC()
	var activeEvent ei.EggEvent
	hasActive := false

	// Check currently active events
	for _, e := range ei.EggIncEvents {
		if strings.Contains(strings.ToLower(e.EventType), "earnings") || strings.Contains(strings.ToLower(e.ID), "earnings") {
			if e.Ultra && !isUltra {
				continue
			}
			if e.StartTime.Before(now) && e.EndTime.After(now) && e.Multiplier > 1.0 {
				if !hasActive || e.Multiplier > activeEvent.Multiplier || (e.Multiplier == activeEvent.Multiplier && e.Ultra) {
					activeEvent = e
					hasActive = true
				}
			}
		}
	}

	if hasActive {
		name := fmt.Sprintf("%.0fx Event", activeEvent.Multiplier)
		if activeEvent.Ultra {
			name = fmt.Sprintf("%.0fx Ultra Event", activeEvent.Multiplier)
		}
		return EarningsEventInfo{
			HasActiveEvent: true,
			Event:          activeEvent,
			Multiplier:     activeEvent.Multiplier,
			EventName:      name,
		}
	}

	return EarningsEventInfo{
		HasActiveEvent: false,
		Multiplier:     1.0,
		EventName:      "",
	}
}

// DroneEventInfo holds active or recent periodical drone event information.
type DroneEventInfo struct {
	HasActiveEvent bool
	Event          ei.EggEvent
	Multiplier     float64
	EventName      string
}

func getDroneEventInfo(isUltra bool) DroneEventInfo {
	ei.EventMutex.Lock()
	defer ei.EventMutex.Unlock()

	now := time.Now().UTC()
	var activeEvent ei.EggEvent
	hasActive := false

	// Check currently active events
	for _, e := range ei.EggIncEvents {
		if strings.Contains(strings.ToLower(e.EventType), "drone") || strings.Contains(strings.ToLower(e.ID), "drone") {
			if e.Ultra && !isUltra {
				continue
			}
			if e.StartTime.Before(now) && e.EndTime.After(now) {
				if !hasActive || e.Multiplier > activeEvent.Multiplier || (e.Multiplier == activeEvent.Multiplier && e.Ultra) {
					activeEvent = e
					hasActive = true
				}
			}
		}
	}

	if hasActive {
		name := fmt.Sprintf("%.0fx Event", activeEvent.Multiplier)
		if activeEvent.Ultra {
			name = fmt.Sprintf("%.0fx Ultra Event", activeEvent.Multiplier)
		}
		return DroneEventInfo{
			HasActiveEvent: true,
			Event:          activeEvent,
			Multiplier:     activeEvent.Multiplier,
			EventName:      name,
		}
	}

	return DroneEventInfo{
		HasActiveEvent: false,
		Multiplier:     1.0,
		EventName:      "",
	}
}

func collectDroneCandidates(inventory []*ei.ArtifactInventoryItem) (bestBrooch, bestMedallion, bestBeak *ei.CompleteArtifact, clarityStones []StoneCandidate) {
	for _, item := range inventory {
		if item == nil || item.GetArtifact() == nil {
			continue
		}
		art := item.GetArtifact()
		spec := art.GetSpec()
		if spec == nil {
			continue
		}

		if isStone(spec.GetName()) {
			if spec.GetName() == ei.ArtifactSpec_CLARITY_STONE {
				lvl := int(spec.GetLevel())
				if lvl >= 0 && lvl < len(clarityStoneBuffs) {
					qty := int(item.GetQuantity())
					if qty < 1 {
						qty = 1
					}
					for i := 0; i < qty; i++ {
						clarityStones = append(clarityStones, StoneCandidate{
							Spec:       spec,
							Multiplier: clarityStoneBuffs[lvl],
						})
					}
				}
			}
			continue
		}

		// Extract any clarity stones slotted into inventory/equipped artifacts
		for _, st := range art.GetStones() {
			if st != nil && st.GetName() == ei.ArtifactSpec_CLARITY_STONE {
				lvl := int(st.GetLevel())
				if lvl >= 0 && lvl < len(clarityStoneBuffs) {
					clarityStones = append(clarityStones, StoneCandidate{
						Spec:       st,
						Multiplier: clarityStoneBuffs[lvl],
					})
				}
			}
		}

		if spec.GetName() != ei.ArtifactSpec_AURELIAN_BROOCH &&
			spec.GetName() != ei.ArtifactSpec_NEODYMIUM_MEDALLION &&
			spec.GetName() != ei.ArtifactSpec_BEAK_OF_MIDAS {
			continue
		}

		tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
		slots := getArtifactSlotCount(spec)

		switch spec.GetName() {
		case ei.ArtifactSpec_AURELIAN_BROOCH:
			base := broochBuffs[tierStr]
			score := base * (1.0 + 0.5*float64(slots))
			if bestBrooch == nil {
				bestBrooch = art
			} else {
				curSpec := bestBrooch.GetSpec()
				curTier := ei.ArtifactLevels[curSpec.GetLevel()] + ei.ArtifactRarity[curSpec.GetRarity()]
				curScore := broochBuffs[curTier] * (1.0 + 0.5*float64(getArtifactSlotCount(curSpec)))
				if score > curScore {
					bestBrooch = art
				}
			}
		case ei.ArtifactSpec_NEODYMIUM_MEDALLION:
			base := medallionBuffs[tierStr]
			score := base * (1.0 + 0.5*float64(slots))
			if bestMedallion == nil {
				bestMedallion = art
			} else {
				curSpec := bestMedallion.GetSpec()
				curTier := ei.ArtifactLevels[curSpec.GetLevel()] + ei.ArtifactRarity[curSpec.GetRarity()]
				curScore := medallionBuffs[curTier] * (1.0 + 0.5*float64(getArtifactSlotCount(curSpec)))
				if score > curScore {
					bestMedallion = art
				}
			}
		case ei.ArtifactSpec_BEAK_OF_MIDAS:
			score := beakBuffs[tierStr]
			if slots > 0 {
				score += 1000.0
				score += float64(slots) * 10.0
			}
			if bestBeak == nil {
				bestBeak = art
			} else {
				curSpec := bestBeak.GetSpec()
				curTier := ei.ArtifactLevels[curSpec.GetLevel()] + ei.ArtifactRarity[curSpec.GetRarity()]
				curSlots := getArtifactSlotCount(curSpec)
				curScore := beakBuffs[curTier]
				if curSlots > 0 {
					curScore += 1000.0
					curScore += float64(curSlots) * 10.0
				}
				if score > curScore {
					bestBeak = art
				}
			}
		}
	}

	sort.SliceStable(clarityStones, func(i, j int) bool {
		return clarityStones[i].Multiplier > clarityStones[j].Multiplier
	})

	return bestBrooch, bestMedallion, bestBeak, clarityStones
}

func optimizeStoneDistributionForHosts(hosts []*ei.CompleteArtifact, clarityStones []StoneCandidate) *DroneFarmingResult {
	res := &DroneFarmingResult{
		BroochMultiplier:    1.0,
		MedallionMultiplier: 1.0,
		GEDropChance:        0.30,
		TotalMultiplier:     1.0,
	}
	if len(hosts) == 0 {
		return res
	}

	hostSlots := make([]int, len(hosts))
	totalSlots := 0
	for i, h := range hosts {
		hostSlots[i] = getArtifactSlotCount(h.GetSpec())
		totalSlots += hostSlots[i]
	}

	numStonesToConsider := totalSlots
	if len(clarityStones) < numStonesToConsider {
		numStonesToConsider = len(clarityStones)
	}

	assigned := make([][]*ei.ArtifactSpec, len(hosts))
	for i := range assigned {
		assigned[i] = make([]*ei.ArtifactSpec, 0, hostSlots[i])
	}

	bestAssigned := make([][]*ei.ArtifactSpec, len(hosts))
	for i := range bestAssigned {
		bestAssigned[i] = make([]*ei.ArtifactSpec, 0, hostSlots[i])
	}
	bestTotal := -1.0
	var bestBMult, bestMMult, bestGEChance float64

	var dfs func(stoneIdx int)
	dfs = func(stoneIdx int) {
		if stoneIdx == numStonesToConsider {
			bMult := 1.0
			mMult := 1.0
			geChance := 0.30

			for i, h := range hosts {
				spec := h.GetSpec()
				tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]
				cSum := 0.0
				for _, st := range assigned[i] {
					lvl := int(st.GetLevel())
					if lvl >= 0 && lvl < len(clarityStoneBuffs) {
						cSum += clarityStoneBuffs[lvl]
					}
				}

				switch spec.GetName() {
				case ei.ArtifactSpec_AURELIAN_BROOCH:
					bMult = 1.0 + broochBuffs[tierStr]*cSum
				case ei.ArtifactSpec_NEODYMIUM_MEDALLION:
					mMult = 1.0 + medallionBuffs[tierStr]*cSum
				case ei.ArtifactSpec_BEAK_OF_MIDAS:
					if cSum > 0 {
						geChance = math.Min(1.0, 0.30*(1.0+beakBuffs[tierStr]*cSum))
					}
				}
			}

			total := bMult * mMult * (geChance / 0.30)
			if total > bestTotal {
				bestTotal = total
				bestBMult = bMult
				bestMMult = mMult
				bestGEChance = geChance
				for i := range assigned {
					bestAssigned[i] = append([]*ei.ArtifactSpec(nil), assigned[i]...)
				}
			}
			return
		}

		st := clarityStones[stoneIdx].Spec

		// Try assigning to each host that has an open slot
		for i, h := range hosts {
			if len(assigned[i]) < hostSlots[i] {
				// If host is Beak and already guarantees 100% GE, don't assign additional stones to it
				if h.GetSpec().GetName() == ei.ArtifactSpec_BEAK_OF_MIDAS {
					tierStr := ei.ArtifactLevels[h.GetSpec().GetLevel()] + ei.ArtifactRarity[h.GetSpec().GetRarity()]
					curCSum := 0.0
					for _, s := range assigned[i] {
						lvl := int(s.GetLevel())
						if lvl >= 0 && lvl < len(clarityStoneBuffs) {
							curCSum += clarityStoneBuffs[lvl]
						}
					}
					if curCSum > 0 && math.Min(1.0, 0.30*(1.0+beakBuffs[tierStr]*curCSum)) >= 0.999 {
						continue
					}
				}

				assigned[i] = append(assigned[i], st)
				dfs(stoneIdx + 1)
				assigned[i] = assigned[i][:len(assigned[i])-1]
			}
		}

		// Also try leaving stone unassigned
		dfs(stoneIdx + 1)
	}

	dfs(0)

	var optArtifacts []*ei.CompleteArtifact
	var usedStones []StoneCandidate
	for i, h := range hosts {
		optArtifacts = append(optArtifacts, &ei.CompleteArtifact{
			Spec:   h.GetSpec(),
			Stones: bestAssigned[i],
		})
		for _, st := range bestAssigned[i] {
			lvl := int(st.GetLevel())
			mult := 0.0
			if lvl >= 0 && lvl < len(clarityStoneBuffs) {
				mult = clarityStoneBuffs[lvl]
			}
			usedStones = append(usedStones, StoneCandidate{Spec: st, Multiplier: mult})
		}
	}

	res.OptimalArtifacts = optArtifacts
	res.OptimalStones = groupStones(usedStones)
	res.BroochMultiplier = bestBMult
	res.MedallionMultiplier = bestMMult
	res.GEDropChance = bestGEChance
	res.TotalMultiplier = bestTotal

	return res
}

func isAlreadyOptimalDroneLoadout(
	currentArtifacts []*ei.CompleteArtifact,
	optArtifacts []*ei.CompleteArtifact,
	curBroochMult, optBroochMult float64,
	curMedMult, optMedMult float64,
	curGEChance, optGEChance float64,
	curTotalMult, optTotalMult float64,
) bool {
	if optTotalMult <= 0 {
		return true
	}

	diff := optTotalMult - curTotalMult
	if diff <= 1e-4 || (diff/optTotalMult) < 1e-4 {
		return true
	}

	// Component-wise check:
	// A loadout is optimal if each drone component (Rewards, Frequency, GE Chance)
	// meets or exceeds optimal.
	// For GE Chance, once 100% (>= 0.999) is achieved, any extra clarity or other stones on Beak are redundant.
	geChanceMatches := (curGEChance >= optGEChance-1e-4) || (curGEChance >= 0.999 && optGEChance >= 0.999)
	broochMatches := curBroochMult >= optBroochMult-1e-4
	medallionMatches := curMedMult >= optMedMult-1e-4

	return geChanceMatches && broochMatches && medallionMatches
}

// OptimizeDroneFarming finds the optimal drone farming loadout for Enlightenment farm.
func OptimizeDroneFarming(backup *ei.Backup, farm *ei.Backup_Simulation) *DroneFarmingResult {
	res := &DroneFarmingResult{
		BroochMultiplier:     1.0,
		MedallionMultiplier:  1.0,
		GEDropChance:         0.30,
		TotalMultiplier:      1.0,
		CurrentBroochMult:    1.0,
		CurrentMedallionMult: 1.0,
		CurrentGEDropChance:  0.30,
		CurrentMultiplier:    1.0,
	}
	if backup == nil || farm == nil {
		return res
	}

	isEnlightenment := farm.GetEggType() == ei.Egg_ENLIGHTENMENT
	res.IsEnlightenment = isEnlightenment

	isUltra := backup != nil && backup.GetSubInfo() != nil && backup.GetSubInfo().GetStatus() == ei.UserSubscriptionInfo_ACTIVE
	eventInfo := getDroneEventInfo(isUltra)
	res.EventMultiplier = eventInfo.Multiplier
	res.HasActiveEvent = eventInfo.HasActiveEvent
	res.EventMessage = eventInfo.Event.Message
	res.EventEndTime = eventInfo.Event.EndTime

	// Calculate equipped drone stats with Clarity Stones (intended for Enlightenment farm)
	currentArtifacts := getEquippedArtifacts(backup, farm)
	equippedBroochMult := 1.0
	equippedMedallionMult := 1.0
	equippedGEDropChance := 0.30

	for _, a := range currentArtifacts {
		if a == nil || a.GetSpec() == nil {
			continue
		}
		spec := a.GetSpec()
		tierStr := ei.ArtifactLevels[spec.GetLevel()] + ei.ArtifactRarity[spec.GetRarity()]

		clarity := 0.0
		for _, st := range a.GetStones() {
			if st != nil && st.GetName() == ei.ArtifactSpec_CLARITY_STONE {
				lvl := int(st.GetLevel())
				if lvl >= 0 && lvl < len(clarityStoneBuffs) {
					clarity += clarityStoneBuffs[lvl]
				}
			}
		}

		switch spec.GetName() {
		case ei.ArtifactSpec_AURELIAN_BROOCH:
			base := broochBuffs[tierStr]
			equippedBroochMult = 1.0 + base*clarity
		case ei.ArtifactSpec_NEODYMIUM_MEDALLION:
			base := medallionBuffs[tierStr]
			equippedMedallionMult = 1.0 + base*clarity
		case ei.ArtifactSpec_BEAK_OF_MIDAS:
			base := beakBuffs[tierStr]
			if clarity > 0 {
				equippedGEDropChance = math.Min(1.0, 0.30*(1.0+base*clarity))
			}
		}
	}
	equippedMultiplier := equippedBroochMult * equippedMedallionMult * (equippedGEDropChance / 0.30)

	// Current farm multiplier: clarity stones only activate when on the Enlightenment farm.
	if isEnlightenment {
		res.CurrentBroochMult = equippedBroochMult
		res.CurrentMedallionMult = equippedMedallionMult
		res.CurrentGEDropChance = equippedGEDropChance
		res.CurrentMultiplier = equippedMultiplier
	} else {
		res.CurrentBroochMult = 1.0
		res.CurrentMedallionMult = 1.0
		res.CurrentGEDropChance = 0.30
		res.CurrentMultiplier = 1.0
	}

	// Collect candidate artifacts and clarity stones from standard inventory
	var inventory []*ei.ArtifactInventoryItem
	if backup.GetArtifactsDb() != nil {
		inventory = backup.GetArtifactsDb().GetInventoryItems()
	}
	bestBrooch, bestMedallion, bestBeak, clarityStones := collectDroneCandidates(inventory)

	var candidateHosts []*ei.CompleteArtifact
	if bestBrooch != nil {
		candidateHosts = append(candidateHosts, bestBrooch)
	}
	if bestMedallion != nil {
		candidateHosts = append(candidateHosts, bestMedallion)
	}
	if bestBeak != nil {
		candidateHosts = append(candidateHosts, bestBeak)
	}

	maxSlots := ei.GetMaxEquippedArtifacts(backup)
	if maxSlots < len(candidateHosts) && maxSlots > 0 {
		var bestSubRes *DroneFarmingResult
		var bestSubTotal float64 = -1

		var generateSubsets func(start int, current []*ei.CompleteArtifact)
		generateSubsets = func(start int, current []*ei.CompleteArtifact) {
			if len(current) == maxSlots {
				subRes := optimizeStoneDistributionForHosts(current, clarityStones)
				if subRes.TotalMultiplier > bestSubTotal {
					bestSubTotal = subRes.TotalMultiplier
					bestSubRes = subRes
				}
				return
			}
			for i := start; i < len(candidateHosts); i++ {
				generateSubsets(i+1, append(current, candidateHosts[i]))
			}
		}
		generateSubsets(0, nil)

		if bestSubRes != nil {
			res.OptimalArtifacts = bestSubRes.OptimalArtifacts
			res.OptimalStones = bestSubRes.OptimalStones
			res.BroochMultiplier = bestSubRes.BroochMultiplier
			res.MedallionMultiplier = bestSubRes.MedallionMultiplier
			res.GEDropChance = bestSubRes.GEDropChance
			res.TotalMultiplier = bestSubRes.TotalMultiplier
		}
	} else if len(candidateHosts) > 0 {
		subRes := optimizeStoneDistributionForHosts(candidateHosts, clarityStones)
		res.OptimalArtifacts = subRes.OptimalArtifacts
		res.OptimalStones = subRes.OptimalStones
		res.BroochMultiplier = subRes.BroochMultiplier
		res.MedallionMultiplier = subRes.MedallionMultiplier
		res.GEDropChance = subRes.GEDropChance
		res.TotalMultiplier = subRes.TotalMultiplier
	} else {
		res.OptimalArtifacts = nil
		res.BroochMultiplier = 1.0
		res.MedallionMultiplier = 1.0
		res.GEDropChance = 0.30
		res.TotalMultiplier = 1.0
	}

	res.IsAlreadyOptimal = isAlreadyOptimalDroneLoadout(
		currentArtifacts,
		res.OptimalArtifacts,
		equippedBroochMult,
		res.BroochMultiplier,
		equippedMedallionMult,
		res.MedallionMultiplier,
		equippedGEDropChance,
		res.GEDropChance,
		equippedMultiplier,
		res.TotalMultiplier,
	)
	if res.IsAlreadyOptimal {
		res.OptimalArtifacts = currentArtifacts
		res.BroochMultiplier = equippedBroochMult
		res.MedallionMultiplier = equippedMedallionMult
		res.GEDropChance = equippedGEDropChance
		res.TotalMultiplier = equippedMultiplier
	}

	return res
}

func printDroneFarmingSetup(userID string, backup *ei.Backup, farm *ei.Backup_Simulation, avatarURL string, actAsNotice string) []dc.LayoutComponent {
	farmTypeStr, eggEmote := formatFarmTypeHeader(farm)
	displayName := ei.NormalizePlayerNameForDisplay(backup.GetUserName())
	isEnlightenment := farm != nil && farm.GetEggType() == ei.Egg_ENLIGHTENMENT

	header := strings.Builder{}
	artifactsStr := strings.Builder{}
	footer := strings.Builder{}

	fmt.Fprint(&header, "# Optimal Drone Farming Setup\n")
	fmt.Fprintf(&header, "**__%s__**\n", displayName)
	enlightenmentEmote := ei.GetBotEmojiMarkdown("egg_enlightenment")
	if isEnlightenment {
		fmt.Fprintf(&header, "**Farm Type**: %s Enlightenment\n", enlightenmentEmote)
	} else {
		fmt.Fprintf(&header, "**Farm Type**: %s Enlightenment *(Current: %s %s)*\n", enlightenmentEmote, eggEmote, farmTypeStr)
	}

	currentArtifacts := getEquippedArtifacts(backup, farm)
	currIcons := formatArtifactIcons(currentArtifacts)

	opt := OptimizeDroneFarming(backup, farm)
	optArtifacts := opt.OptimalArtifacts
	optIcons := formatArtifactIcons(optArtifacts)

	currentPrefix := eggEmote
	if currentPrefix != "" {
		currentPrefix += " "
	} else {
		currentPrefix = "Current: "
	}

	if opt.IsAlreadyOptimal {
		artifactsStr.WriteString("**Currently Equipped:**\n")
		for _, a := range currentArtifacts {
			icons := formatArtifactWithStonesDisplay(a)
			buffStr := formatDroneArtifactBuff(a)
			if buffStr != "" {
				fmt.Fprintf(&artifactsStr, "%s %s\n", icons, buffStr)
			} else {
				fmt.Fprintf(&artifactsStr, "%s\n", icons)
			}
		}
		bMult := opt.BroochMultiplier
		mMult := opt.MedallionMultiplier
		geChance := opt.GEDropChance
		totMult := opt.TotalMultiplier
		if opt.CurrentMultiplier >= opt.TotalMultiplier-1e-4 {
			bMult = opt.CurrentBroochMult
			mMult = opt.CurrentMedallionMult
			geChance = opt.CurrentGEDropChance
			totMult = opt.CurrentMultiplier
		}
		eventLabel := opt.EventMessage
		if eventLabel == "" {
			eventLabel = fmt.Sprintf("%.0fx Event", opt.EventMultiplier)
		}
		geEmoji := getGoldenEggEmojiMarkdown()
		fmt.Fprintf(&artifactsStr, "Formula: %s\n", buildDroneFarmingFormula(bMult, mMult, geChance, totMult, opt.EventMultiplier, opt.HasActiveEvent))
		if opt.HasActiveEvent && opt.EventMultiplier > 1.0 {
			fmt.Fprintf(&artifactsStr, "%s **Optimal %s GE Multiplier**: **%.1fx** (**%.1fx** with %s) (%s%.1fx · %.1fx with event)\n\n",
				geEmoji, enlightenmentEmote, totMult, totMult*opt.EventMultiplier, eventLabel,
				currentPrefix, opt.CurrentMultiplier, opt.CurrentMultiplier*opt.EventMultiplier)
		} else {
			fmt.Fprintf(&artifactsStr, "%s **Optimal %s GE Multiplier**: **%.1fx** (%s%.1fx)\n\n",
				geEmoji, enlightenmentEmote, totMult, currentPrefix, opt.CurrentMultiplier)
		}
		artifactsStr.WriteString("✅ **Your current setup is already optimal!**\n\n")
	} else {
		eventLabel := opt.EventMessage
		if eventLabel == "" {
			eventLabel = fmt.Sprintf("%.0fx Event", opt.EventMultiplier)
		}

		// Currently equipped section
		artifactsStr.WriteString(formatEquippedDroneArtifactsSection(currentArtifacts, opt.CurrentBroochMult, opt.CurrentMedallionMult, opt.CurrentGEDropChance, opt.CurrentMultiplier, opt.EventMultiplier, opt.HasActiveEvent, eventLabel))

		// Optimal setup section
		artifactsStr.WriteString("**Optimal Setup:**\n")
		for _, a := range optArtifacts {
			icons := formatArtifactWithStonesDisplay(a)
			buffStr := formatDroneArtifactBuff(a)
			if buffStr != "" {
				fmt.Fprintf(&artifactsStr, "%s %s\n", icons, buffStr)
			} else {
				fmt.Fprintf(&artifactsStr, "%s\n", icons)
			}
		}
		geEmoji := getGoldenEggEmojiMarkdown()
		fmt.Fprintf(&artifactsStr, "Formula: %s\n", buildDroneFarmingFormula(opt.BroochMultiplier, opt.MedallionMultiplier, opt.GEDropChance, opt.TotalMultiplier, opt.EventMultiplier, opt.HasActiveEvent))
		if opt.HasActiveEvent && opt.EventMultiplier > 1.0 {
			fmt.Fprintf(&artifactsStr, "%s **Optimal %s GE Multiplier**: **%.1fx** (**%.1fx** with %s) (%s%.1fx · %.1fx with event)\n\n",
				geEmoji, enlightenmentEmote, opt.TotalMultiplier, opt.TotalMultiplier*opt.EventMultiplier, eventLabel,
				currentPrefix, opt.CurrentMultiplier, opt.CurrentMultiplier*opt.EventMultiplier)
		} else {
			fmt.Fprintf(&artifactsStr, "%s **Optimal %s GE Multiplier**: **%.1fx** (%s%.1fx)\n\n",
				geEmoji, enlightenmentEmote, opt.TotalMultiplier, currentPrefix, opt.CurrentMultiplier)
		}

		if currIcons != "" && optIcons != "" {
			diff := opt.TotalMultiplier - opt.CurrentMultiplier
			if diff > 0 {
				if opt.HasActiveEvent && opt.EventMultiplier > 1.0 {
					fmt.Fprintf(&artifactsStr, "%s ➜ %s  **+%.1fx %s Yield** (**+%.1fx** with %s)\n\n", currIcons, optIcons, diff, geEmoji, diff*opt.EventMultiplier, eventLabel)
				} else {
					fmt.Fprintf(&artifactsStr, "%s ➜ %s  **+%.1fx %s Yield**\n\n", currIcons, optIcons, diff, geEmoji)
				}
			}
		}
	}

	artifactsStr.WriteString("-# 💡 Drone farming calculations are based on being on the **Enlightenment Farm** with Clarity Stones.\n")
	if !isEnlightenment {
		fmt.Fprintf(&artifactsStr, "-# ⚠️ **Switch to Enlightenment Egg:** You are currently on %s %s — switch to the **Enlightenment Farm** to activate Clarity Stones for drone farming.\n", eggEmote, farmTypeStr)
	}
	if opt.HasActiveEvent && opt.EventMultiplier > 1.0 {
		fmt.Fprintf(&artifactsStr, "-# 🎉 **Active Periodicals Event:** %s amplifies drone rewards by **%.0fx** (ends <t:%d:R>).\n", opt.EventMessage, opt.EventMultiplier, opt.EventEndTime.Unix())
	}
	maxSlots := ei.GetMaxEquippedArtifacts(backup)
	if maxSlots >= 3 {
		artifactsStr.WriteString("-# 💡 Only Brooch, Medallion, and Beak boost drone Golden Egg drops. The 4th slot can be used for any utility.\n")
	}

	// Footer
	now := time.Now()
	syncTime := time.Unix(int64(farm.GetLastStepTime()), 0)
	if farm.GetLastStepTime() == 0 {
		syncTime = time.Unix(int64(backup.GetApproxTime()), 0)
	}
	syncStr := fmt.Sprintf("Sync:%s", bottools.WrapTimestamp(syncTime.Unix(), bottools.TimestampShortTime))
	if now.Sub(syncTime) > 15*time.Minute {
		syncStr = fmt.Sprintf("⚠️ %s ⚠️", syncStr)
	}
	fmt.Fprintf(&footer, "-# Report:%s  %s\n",
		bottools.WrapTimestamp(now.Unix(), bottools.TimestampShortTime),
		syncStr)
	if actAsNotice != "" {
		fmt.Fprintf(&footer, "-# %s\n", actAsNotice)
	}

	avatar := avatarURL
	if avatar == "" {
		avatar = defaultFarmAvatarURL(farm)
	}

	var components []dc.LayoutComponent
	components = append(components, dc.Section{
		Components: []dc.TextDisplay{
			{Content: header.String()},
		},
		Accessory: dc.Thumbnail{
			URL: avatar,
		},
	})
	components = append(components, dc.Separator{
		Divider: true,
		Spacing: dc.SeparatorSpacingSmall,
	})
	components = append(components, dc.TextDisplay{
		Content: artifactsStr.String(),
	})
	components = append(components, dc.Separator{
		Divider: true,
		Spacing: dc.SeparatorSpacingSmall,
	})
	components = append(components, dc.TextDisplay{
		Content: footer.String(),
	})

	return components
}



