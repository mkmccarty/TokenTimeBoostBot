package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

func getArtifactsPageFromContent(content string) string {
	if strings.Contains(content, "Set: IHR") {
		return "ihr"
	}
	if strings.Contains(content, "Set: Colleggtibles") {
		return "collegg"
	}
	return "delivery"
}

const (
	colleggCategoryLay   = "collegg-lay"
	colleggCategoryShip  = "collegg-ship"
	colleggCategoryIHR   = "collegg-ihr"
	colleggCategoryOther = "collegg-other"
)

func normalizeColleggtibleName(name string) string {
	n := strings.ToUpper(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, " ", "")
	n = strings.ReplaceAll(n, "-", "")
	return n
}

func customEggNameLookup() map[string]string {
	lookup := make(map[string]string, len(ei.CustomEggMap))
	for _, egg := range ei.CustomEggMap {
		if egg == nil || egg.Name == "" {
			continue
		}
		lookup[normalizeColleggtibleName(egg.Name)] = egg.Name
	}
	return lookup
}

func getSelectedColleggtiblesFromStored(stored string) map[string]bool {
	selected := make(map[string]bool)
	if stored == "" {
		return selected
	}
	lookup := customEggNameLookup()
	for _, raw := range strings.Split(stored, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if canonical, ok := lookup[normalizeColleggtibleName(name)]; ok {
			selected[canonical] = true
		}
	}
	return selected
}

func isDiscordSnowflake(value string) bool {
	if len(value) < 15 || len(value) > 21 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func formatArtifactTarget(userID string) string {
	if mentionID, ok := parseMentionUserID(userID); ok {
		ign := strings.TrimSpace(farmerstate.GetMiscSettingString(mentionID, "ei_ign"))
		if ign != "" {
			return fmt.Sprintf("<@%s> (%s)", mentionID, ei.NormalizePlayerNameForDisplay(ign))
		}
		return "<@" + mentionID + ">"
	}

	if isDiscordSnowflake(userID) {
		ign := strings.TrimSpace(farmerstate.GetMiscSettingString(userID, "ei_ign"))
		if ign != "" {
			return fmt.Sprintf("<@%s> (%s)", userID, ei.NormalizePlayerNameForDisplay(ign))
		}
		return "<@" + userID + ">"
	}

	ign := strings.TrimSpace(farmerstate.GetMiscSettingString(userID, "ei_ign"))
	if ign != "" {
		return fmt.Sprintf("%s (%s)", userID, ei.NormalizePlayerNameForDisplay(ign))
	}
	return userID
}

func getColleggtibleCategory(egg *ei.EggIncCustomEgg) string {
	if egg == nil {
		return colleggCategoryOther
	}
	switch egg.Dimension {
	case ei.GameModifier_EGG_LAYING_RATE, ei.GameModifier_HAB_CAPACITY:
		return colleggCategoryLay
	case ei.GameModifier_SHIPPING_CAPACITY:
		return colleggCategoryShip
	case ei.GameModifier_INTERNAL_HATCHERY_RATE:
		return colleggCategoryIHR
	default:
		return colleggCategoryOther
	}
}

func updateColleggtibleCategorySelection(userID string, category string, selectedValues []string) {
	current := getSelectedColleggtiblesFromStored(farmerstate.GetMiscSettingString(userID, "collegg"))

	validInCategory := make(map[string]bool)
	for _, egg := range ei.CustomEggMap {
		if egg == nil || egg.Name == "" {
			continue
		}
		if getColleggtibleCategory(egg) == category {
			validInCategory[egg.Name] = true
		}
	}

	// Remove prior selections from this category only.
	for name := range validInCategory {
		delete(current, name)
	}

	for _, val := range selectedValues {
		name := strings.TrimSpace(val)
		if name == "" {
			continue
		}
		if validInCategory[name] {
			current[name] = true
		}
	}

	names := make([]string, 0, len(current))
	for name := range current {
		names = append(names, name)
	}
	sort.Strings(names)
	farmerstate.SetMiscSettingString(userID, "collegg", strings.Join(names, ","))
}

func populateColleggtiblesFromBackup(userID string, backup *ei.Backup) ([]string, bool) {
	previous := getSelectedColleggtiblesFromStored(farmerstate.GetMiscSettingString(userID, "collegg"))

	if backup == nil || backup.GetContracts() == nil {
		return nil, false
	}

	owned := make(map[string]bool)
	contracts := append(backup.GetContracts().GetArchive(), backup.GetContracts().GetContracts()...)
	for _, c := range contracts {
		if c == nil {
			continue
		}
		contractID := c.GetContractIdentifier()
		if contractID == "" && c.GetContract() != nil {
			contractID = c.GetContract().GetIdentifier()
		} else if contractID == "" && c.GetEvaluation() != nil {
			contractID = c.GetEvaluation().GetContractIdentifier()
		}
		eggID := ""
		if c.GetContract() != nil {
			eggID = c.GetContract().GetCustomEggId()
		} else if contractID != "" {
			if contractInfo, ok := ei.GetEggIncContract(contractID); ok {
				if contractInfo.Egg == int32(ei.Egg_CUSTOM_EGG) {
					eggID = contractInfo.EggName
				}
			}
		}
		if eggID == "" {
			continue
		}
		// Tier 0 colleggtible ownership starts at 10M farm size.
		if c.GetMaxFarmSizeReached() < 1e7 {
			continue
		}
		if egg, ok := ei.CustomEggMap[eggID]; ok && egg != nil && egg.Name != "" {
			owned[egg.Name] = true
		}
	}

	if len(owned) == 0 {
		farmerstate.SetMiscSettingString(userID, "collegg", "")
		return nil, len(previous) != 0
	}

	names := make([]string, 0, len(owned))
	for name := range owned {
		names = append(names, name)
	}
	sort.Strings(names)
	farmerstate.SetMiscSettingString(userID, "collegg", strings.Join(names, ","))

	if len(previous) != len(owned) {
		return names, true
	}
	for name := range owned {
		if !previous[name] {
			return names, true
		}
	}

	return names, false
}

func displayArtifactQuality(val string) string {
	if strings.TrimSpace(val) == "" {
		return "None"
	}
	return val
}

func populateArtifactsFromBackup(client dc.Client, userID string) (string, string, error) {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	if eiID == "" && !isDiscordSnowflake(userID) {
		if discordID, err := farmerstate.GetDiscordUserIDFromEiIgnExact(userID); err == nil && discordID != "" {
			eiID = farmerstate.GetMiscSettingString(discordID, "encrypted_ei_id")
		}
	}
	if eiID == "" {
		msg := "No saved Egg Inc ID found. Run /register first."
		return msg, msg, nil
	}

	backup, _ := ei.GetFirstContactFromAPI(eiID, userID, true)
	if backup == nil || backup.GetArtifactsDb() == nil {
		msg := "Unable to fetch backup artifacts right now."
		return msg, msg, nil
	}

	best := ei.GetBestCoopArtifactsFromInventory(backup.GetArtifactsDb().GetInventoryItems())

	type artifactSlot struct {
		key   string
		label string
		value string
	}

	slots := []artifactSlot{
		{key: "defl", label: "Deflector", value: best["defl"]},
		{key: "metr", label: "Metronome", value: best["metr"]},
		{key: "comp", label: "Compass", value: best["comp"]},
		{key: "guss", label: "Gusset", value: best["guss"]},
		{key: "chalice", label: "Chalice", value: best["chalice"]},
		{key: "monocle", label: "Monocle", value: best["monocle"]},
		{key: "siab", label: "SIAB", value: best["siab"]},
	}
	if slots[6].value == "" {
		slots[6].value = best["SIAB"]
	}
	slots = append(slots, artifactSlot{key: "defl-ihr", label: "IHR Deflector", value: best["defl"]})

	changedArtifactDetails := make([]string, 0)
	changedArtifactCount := 0
	discoveredArtifactCount := 0
	discoveredArtifactDetails := make([]string, 0)

	// Always refresh all supported keys so stale values don't linger.
	for _, slot := range slots {
		oldVal := farmerstate.GetMiscSettingString(userID, slot.key)
		newVal := slot.value
		if slot.key == "defl-ihr" && strings.HasSuffix(oldVal, "_L") {
			baseOldDefl := strings.TrimSuffix(oldVal, "_L")
			if baseOldDefl == newVal {
				newVal = oldVal
			}
		}
		if (slot.key == "siab" || slot.key == "SIAB") && oldVal != "" {
			oldArt := ei.GetArtifactByKey("SIAB-" + oldVal)
			if oldArt != nil && oldArt.Stones >= 3 {
				newVal = oldVal
			}
		}
		if strings.TrimSpace(newVal) != "" {
			discoveredArtifactCount++
			discoveredArtifactDetails = append(discoveredArtifactDetails,
				fmt.Sprintf("%s: %s", slot.label, displayArtifactQuality(newVal)))
		}

		if oldVal != newVal {
			changedArtifactCount++
			changedArtifactDetails = append(changedArtifactDetails,
				fmt.Sprintf("%s: %s -> %s", slot.label, displayArtifactQuality(oldVal), displayArtifactQuality(newVal)))
		}

		farmerstate.SetMiscSettingString(userID, slot.key, newVal)
	}

	colleggtibles, colleggtiblesChanged := populateColleggtiblesFromBackup(userID, backup)

	updatedContracts := 0
	ContractsMutex.RLock()
	defer ContractsMutex.RUnlock()
	for _, contract := range Contracts {
		if contract == nil || contract.State == ContractStateCompleted || contract.State == ContractStateArchive {
			continue
		}

		if !UserInContract(contract, userID) {
			continue
		}

		updated := false
		contract.mutex.Lock()
		if b := contract.Boosters[userID]; b != nil {
			b.ArtifactSet = getUserArtifacts(userID, nil)
			rate, logStr := CalculateIHRRateFromDB(userID)
			b.IHRRate = rate
			b.IHRCalcLog = logStr
			updatedContracts++
			updated = true
		}
		contract.mutex.Unlock()

		if !updated {
			continue
		}

		refreshBoostListMessage(client, contract, false)
		saveData(contract.ContractHash)
	}

	status := fmt.Sprintf("Backup loaded (%s).", time.Now().Format("15:04:05"))
	if updatedContracts > 0 {
		status = fmt.Sprintf("Backup loaded and updated %d running contract(s) (%s).", updatedContracts, time.Now().Format("15:04:05"))
	}

	var summary strings.Builder
	summary.WriteString("## Backup Load Summary\n")
	fmt.Fprintf(&summary, "Discovered artifact slots: %d/8\n", discoveredArtifactCount)
	if len(discoveredArtifactDetails) > 0 {
		summary.WriteString(strings.Join(discoveredArtifactDetails, "\n"))
		summary.WriteString("\n")
	}
	fmt.Fprintf(&summary, "\nChanged artifact slots: %d\n", changedArtifactCount)
	if len(changedArtifactDetails) > 0 {
		summary.WriteString(strings.Join(changedArtifactDetails, "\n"))
		summary.WriteString("\n")
	}
	fmt.Fprintf(&summary, "\nColleggtibles discovered: %d\n", len(colleggtibles))
	fmt.Fprintf(&summary, "Colleggtibles changed: %t\n", colleggtiblesChanged)
	fmt.Fprintf(&summary, "Running contracts refreshed: %d", updatedContracts)

	return status, summary.String(), nil
}

func getArtifactsComponents(userID string, channelID string, contractOnly bool, page string, backupButtonLabel string) (string, []dc.LayoutComponent) {
	minValues := 0
	minV := 0
	if page == "" {
		page = "delivery"
	}
	page = strings.ToLower(page)
	if page != "delivery" && page != "ihr" && page != "collegg" {
		page = "delivery"
	}

	// is this channelID a thread
	as := getUserArtifacts(userID, nil)

	ihrRate, _ := CalculateIHRRateFromDB(userID)
	ihrStr := ei.FormatEIValue(ihrRate, map[string]any{"decimals": 2, "trim": true})

	elrDisplay := fmt.Sprintf("ELR: %1.3f", as.LayRate)
	srDisplay := fmt.Sprintf("SR: %2.3f", as.ShipRate)
	ihrDisplay := fmt.Sprintf("IHR: %s", ihrStr)

	switch page {
	case "ihr":
		ihrDisplay = "**" + ihrDisplay + "**"
	case "delivery":
		elrDisplay = "**" + elrDisplay + "**"
	}

	var builder strings.Builder
	targetDisplay := formatArtifactTarget(userID)
	if !contractOnly {
		fmt.Fprintf(&builder, "Select your global coop artifacts %s\n%s  %s", targetDisplay, elrDisplay, ihrDisplay)
	} else {
		fmt.Fprintf(&builder, "Adjust your coop artifact overrides for this contract %s\n %s  %s  %s", targetDisplay, elrDisplay, srDisplay, ihrDisplay)
	}

	// These are the global settings
	deflector := ""
	metronome := ""
	compass := ""
	gusset := ""
	ihrDeflector := ""
	chalice := ""
	monocle := ""
	siab := ""
	coll := ""

	temp := "PERM"
	if contractOnly {
		temp = "TEMP"
		contract := FindContract(channelID)
		if contract != nil {
			if UserInContract(contract, userID) {
				for a := range contract.Boosters[userID].ArtifactSet.Artifacts {
					if contract.Boosters[userID].ArtifactSet.Artifacts[a].Type == "IHR Deflector" {
						ihrDeflector = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
						continue
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Deflector") {
						deflector = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Metronome") {
						metronome = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Compass") {
						compass = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Gusset") {
						gusset = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Chalice") {
						chalice = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Monocle") {
						monocle = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "SIAB") {
						siab = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
				}
			}
		} else {
			return "No contract exists in this channel", nil
		}
	} else {
		deflector = farmerstate.GetMiscSettingString(userID, "defl")
		metronome = farmerstate.GetMiscSettingString(userID, "metr")
		compass = farmerstate.GetMiscSettingString(userID, "comp")
		gusset = farmerstate.GetMiscSettingString(userID, "guss")
		ihrDeflector = farmerstate.GetMiscSettingString(userID, "defl-ihr")
		chalice = farmerstate.GetMiscSettingString(userID, "chalice")
		monocle = farmerstate.GetMiscSettingString(userID, "monocle")
		siab = farmerstate.GetMiscSettingString(userID, "siab")
		if siab == "" {
			siab = farmerstate.GetMiscSettingString(userID, "SIAB")
		}
		coll = farmerstate.GetMiscSettingString(userID, "collegg")

		// Need to perform a conversion on what's in coll.
		// CarbonFiber,Chocolate,Easter,Firework,Pumpkin,Waterballoon,Lithium
		coll = strings.ToUpper(coll)
		coll = strings.ReplaceAll(coll, "CARBONFIBER", "CARBON FIBER")
		coll = strings.ReplaceAll(coll, "FLAMERETARDANT", "FLAME RETARDANT")
	}

	component := []dc.LayoutComponent{
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    "as_#DEFL#" + userID + "#" + temp,
					Placeholder: "Select your Deflector...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []dc.SelectOption{
						{
							Label:       "Deflector T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     deflector == "T4L",
							Emoji:       ei.GetBotComponentEmoji("defl_T4L")},
						{
							Label:       "Deflector T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     deflector == "T4E",
							Emoji:       ei.GetBotComponentEmoji("defl_T4E"),
						},
						{
							Label:       "Deflector T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     deflector == "T4R",
							Emoji:       ei.GetBotComponentEmoji("defl_T4R"),
						},
						{
							Label:       "Deflector T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     deflector == "T4C",
							Emoji:       ei.GetBotComponentEmoji("defl_T4C"),
						},
						{
							Label:       "Deflector T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     deflector == "T3R",
							Emoji:       ei.GetBotComponentEmoji("defl_T3R"),
						},
						{
							Label:       "Deflector T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     deflector == "T3C",
							Emoji:       ei.GetBotComponentEmoji("defl_T3C"),
						},
						{
							Label:       "None",
							Description: "No Deflector equipped",
							Value:       "NONE",
							Default:     deflector == "NONE" || deflector == "",
						},
					},
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    "as_#METR#" + userID + "#" + temp,
					Placeholder: "Select your Metronome...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []dc.SelectOption{
						{
							Label:       "Metronome T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     metronome == "T4L",
							Emoji:       ei.GetBotComponentEmoji("metr_T4L"),
						},
						{
							Label:       "Metronome T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     metronome == "T4E",
							Emoji:       ei.GetBotComponentEmoji("metr_T4E"),
						},
						{
							Label:       "Metronome T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     metronome == "T4R",
							Emoji:       ei.GetBotComponentEmoji("metr_T4R"),
						},
						{
							Label:       "Metronome T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     metronome == "T4C",
							Emoji:       ei.GetBotComponentEmoji("metr_T4C"),
						},
						{
							Label:       "Metronome T3E",
							Description: "Epic",
							Value:       "T3E",
							Default:     metronome == "T3E",
							Emoji:       ei.GetBotComponentEmoji("metr_T3E"),
						},
						{
							Label:       "Metronome T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     metronome == "T3R",
							Emoji:       ei.GetBotComponentEmoji("metr_T3R"),
						},
						{
							Label:       "Metronome T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     metronome == "T3C",
							Emoji:       ei.GetBotComponentEmoji("metr_T3C"),
						},
						{
							Label:       "None",
							Description: "No Metronome equipped",
							Value:       "NONE",
							Default:     metronome == "NONE" || metronome == "",
						},
					},
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    "as_#COMP#" + userID + "#" + temp,
					Placeholder: "Select your Compass...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []dc.SelectOption{
						{
							Label:       "Compass T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     compass == "T4L",
							Emoji:       ei.GetBotComponentEmoji("comp_T4L"),
						},
						{
							Label:       "Compass T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     compass == "T4E",
							Emoji:       ei.GetBotComponentEmoji("comp_T4E"),
						},
						{
							Label:       "Compass T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     compass == "T4R",
							Emoji:       ei.GetBotComponentEmoji("comp_T4R"),
						},
						{
							Label:       "Compass T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     compass == "T4C",
							Emoji:       ei.GetBotComponentEmoji("comp_T4C"),
						},
						{
							Label:       "Compass T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     compass == "T3R",
							Emoji:       ei.GetBotComponentEmoji("comp_T3R"),
						},
						{
							Label:       "Compass T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     compass == "T3C",
							Emoji:       ei.GetBotComponentEmoji("comp_T3C"),
						},
						{
							Label:       "None",
							Description: "No Compass equipped",
							Value:       "NONE",
							Default:     compass == "NONE" || compass == "",
						},
					},
				},
			},
		},
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.SelectMenu{
					CustomID:    "as_#GUSS#" + userID + "#" + temp,
					Placeholder: "Select your Gusset...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []dc.SelectOption{
						{
							Label:       "Gusset T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     gusset == "T4L",
							Emoji:       ei.GetBotComponentEmoji("gusset_T4L"),
						},
						{
							Label:       "Gusset T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     gusset == "T4E",
							Emoji:       ei.GetBotComponentEmoji("gusset_T4E"),
						},
						{
							Label:       "Gusset T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     gusset == "T4C",
							Emoji:       ei.GetBotComponentEmoji("gusset_T4C"),
						},
						{
							Label:       "Gusset T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     gusset == "T3R",
							Emoji:       ei.GetBotComponentEmoji("gusset_T3R"),
						},
						{
							Label:       "Gusset T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     gusset == "T3C",
							Emoji:       ei.GetBotComponentEmoji("gusset_T3C"),
						},
						{
							Label:       "Gusset T2E",
							Description: "Epic",
							Value:       "T2E",
							Default:     gusset == "T2E",
							Emoji:       ei.GetBotComponentEmoji("gusset_T2E"),
						},
						{
							Label:       "None",
							Description: "No Gusset equipped",
							Value:       "NONE",
							Default:     gusset == "NONE" || gusset == "",
						},
					},
				},
			},
		},
	}

	if !contractOnly && page == "ihr" {
		component = []dc.LayoutComponent{
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.SelectMenu{
						CustomID:    "as_#DEFL-IHR#" + userID + "#" + temp,
						Placeholder: "Select your IHR Deflector...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options: []dc.SelectOption{
							{Label: "IHR Deflector T4L w/Life", Description: "Legendary (w/Life stones)", Value: "T4L_L", Default: ihrDeflector == "T4L_L", Emoji: ei.GetBotComponentEmoji("defl_T4L")},
							{Label: "IHR Deflector T4L", Description: "Legendary", Value: "T4L", Default: ihrDeflector == "T4L", Emoji: ei.GetBotComponentEmoji("defl_T4L")},
							{Label: "IHR Deflector T4E w/Life", Description: "Epic (w/Life stones)", Value: "T4E_L", Default: ihrDeflector == "T4E_L", Emoji: ei.GetBotComponentEmoji("defl_T4E")},
							{Label: "IHR Deflector T4E", Description: "Epic", Value: "T4E", Default: ihrDeflector == "T4E", Emoji: ei.GetBotComponentEmoji("defl_T4E")},
							{Label: "IHR Deflector T4R w/Life", Description: "Rare (w/Life stones)", Value: "T4R_L", Default: ihrDeflector == "T4R_L", Emoji: ei.GetBotComponentEmoji("defl_T4R")},
							{Label: "IHR Deflector T4R", Description: "Rare", Value: "T4R", Default: ihrDeflector == "T4R", Emoji: ei.GetBotComponentEmoji("defl_T4R")},
							{Label: "IHR Deflector T4C", Description: "Common", Value: "T4C", Default: ihrDeflector == "T4C", Emoji: ei.GetBotComponentEmoji("defl_T4C")},
							{Label: "IHR Deflector T3R w/Life", Description: "Rare (w/Life stones)", Value: "T3R_L", Default: ihrDeflector == "T3R_L", Emoji: ei.GetBotComponentEmoji("defl_T3R")},
							{Label: "IHR Deflector T3R", Description: "Rare", Value: "T3R", Default: ihrDeflector == "T3R", Emoji: ei.GetBotComponentEmoji("defl_T3R")},
							{Label: "IHR Deflector T3C", Description: "Common", Value: "T3C", Default: ihrDeflector == "T3C", Emoji: ei.GetBotComponentEmoji("defl_T3C")},
							{Label: "None", Description: "No IHR Deflector equipped", Value: "NONE", Default: ihrDeflector == "NONE" || ihrDeflector == ""},
						},
					},
				},
			},
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.SelectMenu{
						CustomID:    "as_#CHALICE#" + userID + "#" + temp,
						Placeholder: "Select your Chalice...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options: []dc.SelectOption{
							{Label: "Chalice T4L", Description: "Legendary", Value: "T4L", Default: chalice == "T4L", Emoji: ei.GetBotComponentEmoji("chalice_T4L")},
							{Label: "Chalice T4E", Description: "Epic", Value: "T4E", Default: chalice == "T4E", Emoji: ei.GetBotComponentEmoji("chalice_T4E")},
							{Label: "Chalice T4C", Description: "Common", Value: "T4C", Default: chalice == "T4C", Emoji: ei.GetBotComponentEmoji("chalice_T4C")},
							{Label: "Chalice T3E", Description: "Epic", Value: "T3E", Default: chalice == "T3E", Emoji: ei.GetBotComponentEmoji("chalice_T3E")},
							{Label: "Chalice T3R", Description: "Rare", Value: "T3R", Default: chalice == "T3R", Emoji: ei.GetBotComponentEmoji("chalice_T3R")},
							{Label: "Chalice T3C", Description: "Common", Value: "T3C", Default: chalice == "T3C", Emoji: ei.GetBotComponentEmoji("chalice_T3C")},
							{Label: "Chalice T2E", Description: "Epic", Value: "T2E", Default: chalice == "T2E", Emoji: ei.GetBotComponentEmoji("chalice_T2E")},
							{Label: "Chalice T2C", Description: "Common", Value: "T2C", Default: chalice == "T2C", Emoji: ei.GetBotComponentEmoji("chalice_T2C")},
							{Label: "Chalice T1C", Description: "Common", Value: "T1C", Default: chalice == "T1C", Emoji: ei.GetBotComponentEmoji("chalice_T1C")},
							{Label: "None", Description: "No Chalice equipped", Value: "NONE", Default: chalice == "NONE" || chalice == ""},
						},
					},
				},
			},
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.SelectMenu{
						CustomID:    "as_#MONOCLE#" + userID + "#" + temp,
						Placeholder: "Select your Monocle...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options: []dc.SelectOption{
							{Label: "Monocle T4L", Description: "Legendary", Value: "T4L", Default: monocle == "T4L", Emoji: ei.GetBotComponentEmoji("monocle_T4L")},
							{Label: "Monocle T4E", Description: "Epic", Value: "T4E", Default: monocle == "T4E", Emoji: ei.GetBotComponentEmoji("monocle_T4E")},
							{Label: "Monocle T4C", Description: "Common", Value: "T4C", Default: monocle == "T4C", Emoji: ei.GetBotComponentEmoji("monocle_T4C")},
							{Label: "Monocle T3C", Description: "Common", Value: "T3C", Default: monocle == "T3C", Emoji: ei.GetBotComponentEmoji("monocle_T3C")},
							{Label: "Monocle T2C", Description: "Common", Value: "T2C", Default: monocle == "T2C", Emoji: ei.GetBotComponentEmoji("monocle_T2C")},
							{Label: "Monocle T1C", Description: "Common", Value: "T1C", Default: monocle == "T1C", Emoji: ei.GetBotComponentEmoji("monocle_T1C")},
							{Label: "None", Description: "No Monocle equipped", Value: "NONE", Default: monocle == "NONE" || monocle == ""},
						},
					},
				},
			},
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.SelectMenu{
						CustomID:    "as_#SIAB#" + userID + "#" + temp,
						Placeholder: "Select your Ship In A Bottle...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options: []dc.SelectOption{
							{Label: "SIAB T4L", Description: "Legendary", Value: "T4L", Default: siab == "T4L", Emoji: ei.GetBotComponentEmoji("SIAB_T4L")},
							{Label: "SIAB T4E", Description: "Epic", Value: "T4E", Default: siab == "T4E", Emoji: ei.GetBotComponentEmoji("SIAB_T4E")},
							{Label: "SIAB T4R", Description: "Rare", Value: "T4R", Default: siab == "T4R", Emoji: ei.GetBotComponentEmoji("SIAB_T4R")},
							{Label: "SIAB T4C", Description: "Common", Value: "T4C", Default: siab == "T4C", Emoji: ei.GetBotComponentEmoji("SIAB_T4C")},
							{Label: "SIAB T3R", Description: "Rare", Value: "T3R", Default: siab == "T3R", Emoji: ei.GetBotComponentEmoji("SIAB_T3R")},
							{Label: "SIAB T3C", Description: "Common", Value: "T3C", Default: siab == "T3C", Emoji: ei.GetBotComponentEmoji("SIAB_T3C")},
							{Label: "SIAB T2C", Description: "Common", Value: "T2C", Default: siab == "T2C", Emoji: ei.GetBotComponentEmoji("SIAB_T2C")},
							{Label: "SIAB T1C", Description: "Common", Value: "T1C", Default: siab == "T1C", Emoji: ei.GetBotComponentEmoji("SIAB_T1C")},
							{Label: "3 Slot Artifact", Description: "Generic 3 Stone Slot Artifact", Value: "3S", Default: siab == "3S"},
							{Label: "2 Slot Artifact", Description: "Generic 2 Stone Slot Artifact", Value: "2S", Default: siab == "2S"},
							{Label: "1 Slot Artifact", Description: "Generic 1 Stone Slot Artifact", Value: "1S", Default: siab == "1S"},
							{Label: "None", Description: "No SIAB equipped", Value: "NONE", Default: siab == "NONE" || siab == ""},
						},
					},
				},
			},
		}
	}

	if !contractOnly && page == "collegg" {
		selectedColleggtibles := getSelectedColleggtiblesFromStored(coll)
		selectedLayCount := 0
		selectedShipCount := 0
		selectedIHRCount := 0
		selectedOtherCount := 0
		keys := make([]string, 0, len(ei.CustomEggMap))
		for k := range ei.CustomEggMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		layOptions := make([]dc.SelectOption, 0)
		shipOptions := make([]dc.SelectOption, 0)
		ihrOptions := make([]dc.SelectOption, 0)
		otherOptions := make([]dc.SelectOption, 0)
		for _, k := range keys {
			egg := ei.CustomEggMap[k]
			if egg == nil || egg.Name == "" {
				continue
			}
			if selectedColleggtibles[egg.Name] {
				switch getColleggtibleCategory(egg) {
				case colleggCategoryLay:
					selectedLayCount++
				case colleggCategoryShip:
					selectedShipCount++
				case colleggCategoryIHR:
					selectedIHRCount++
				default:
					selectedOtherCount++
				}
			}
			opt := dc.SelectOption{
				Label:       egg.Name,
				Description: egg.Description,
				Value:       egg.Name,
				Default:     selectedColleggtibles[egg.Name],
				Emoji:       ei.GetBotComponentEmoji("egg_" + egg.ID),
			}
			switch getColleggtibleCategory(egg) {
			case colleggCategoryLay:
				layOptions = append(layOptions, opt)
			case colleggCategoryShip:
				shipOptions = append(shipOptions, opt)
			case colleggCategoryIHR:
				ihrOptions = append(ihrOptions, opt)
			default:
				otherOptions = append(otherOptions, opt)
			}
		}

		component = []dc.LayoutComponent{}
		if len(layOptions) > 0 {
			fmt.Fprintf(&builder, "\nLay Rate colleggtibles: %d selected", selectedLayCount)
		}
		if len(shipOptions) > 0 {
			fmt.Fprintf(&builder, "\nShipping Rate colleggtibles: %d selected", selectedShipCount)
		}
		if len(ihrOptions) > 0 {
			fmt.Fprintf(&builder, "\nInternal Hatchery Rate colleggtibles: %d selected", selectedIHRCount)
		}
		if len(otherOptions) > 0 {
			fmt.Fprintf(&builder, "\nOther colleggtibles: %d selected", selectedOtherCount)
		}

		if len(layOptions) > 0 {
			component = append(component, dc.ActionRow{Components: []dc.InteractiveComponent{dc.SelectMenu{
				CustomID:    "as_#COLLEGG-LAY#" + userID + "#" + temp,
				Placeholder: "Select Lay Rate colleggtibles",
				MinValues:   &minV,
				MaxValues:   len(layOptions),
				Options:     layOptions,
			}}})
		}
		if len(shipOptions) > 0 {
			component = append(component, dc.ActionRow{Components: []dc.InteractiveComponent{dc.SelectMenu{
				CustomID:    "as_#COLLEGG-SHIP#" + userID + "#" + temp,
				Placeholder: "Select Shipping Rate colleggtibles",
				MinValues:   &minV,
				MaxValues:   len(shipOptions),
				Options:     shipOptions,
			}}})
		}
		if len(ihrOptions) > 0 {
			component = append(component, dc.ActionRow{Components: []dc.InteractiveComponent{dc.SelectMenu{
				CustomID:    "as_#COLLEGG-IHR#" + userID + "#" + temp,
				Placeholder: "Select Internal Hatchery Rate colleggtibles",
				MinValues:   &minV,
				MaxValues:   len(ihrOptions),
				Options:     ihrOptions,
			}}})
		}
		if len(otherOptions) > 0 {
			component = append(component, dc.ActionRow{Components: []dc.InteractiveComponent{dc.SelectMenu{
				CustomID:    "as_#COLLEGG-OTHER#" + userID + "#" + temp,
				Placeholder: "Select Other colleggtibles",
				MinValues:   &minV,
				MaxValues:   len(otherOptions),
				Options:     otherOptions,
			}}})
		}
	}

	if !contractOnly {
		deliveryStyle := dc.ButtonSecondary
		ihrStyle := dc.ButtonSecondary
		colleggStyle := dc.ButtonSecondary
		eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		if eiID == "" && !isDiscordSnowflake(userID) {
			if discordID, err := farmerstate.GetDiscordUserIDFromEiIgnExact(userID); err == nil && discordID != "" {
				eiID = farmerstate.GetMiscSettingString(discordID, "encrypted_ei_id")
			}
		}
		hasBackup := eiID != ""
		switch page {
		case "delivery":
			deliveryStyle = dc.ButtonPrimary
		case "ihr":
			ihrStyle = dc.ButtonPrimary
		default:
			colleggStyle = dc.ButtonPrimary
		}

		navButtons := []dc.InteractiveComponent{
			dc.Button{
				Label:    "Delivery Set",
				Style:    deliveryStyle,
				CustomID: "as_#PAGEDEL#" + userID + "#" + temp,
			},
			dc.Button{
				Label:    "IHR Set",
				Style:    ihrStyle,
				CustomID: "as_#PAGEIHR#" + userID + "#" + temp,
			},
			dc.Button{
				Label:    "Colleggtibles",
				Style:    colleggStyle,
				CustomID: "as_#PAGECOL#" + userID + "#" + temp,
			},
		}

		if hasBackup {
			label := backupButtonLabel
			if label == "" {
				label = "Load from Backup"
			}
			navButtons = append(navButtons, dc.Button{
				Label:    label,
				Style:    dc.ButtonSuccess,
				CustomID: "as_#POPBACKUP#" + userID + "#" + temp,
			})
		}

		component = append(component, dc.ActionRow{Components: navButtons})
	}

	return builder.String(), component
}

// SlashArtifactsCommand creates a new slash command for setting Egg, Inc name
func SlashArtifactsCommand(cmd string) *dc.Command {
	command := anywhereCommand(cmd, "Indicate best contract artifacts you have.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "alternate",
			Description:  "Select a linked alternate account",
			Autocomplete: true,
		},
	}
	return &command
}

// HandleArtifactAltAutoComplete suggests the alternates linked to the caller.
func HandleArtifactAltAutoComplete(e *dc.AutocompleteEvent) {
	userID := e.UserID()
	alts := farmerstate.GetAltControllerByMiscString("AltController", userID)

	choices := make([]dc.Choice[string], 0, len(alts))
	for _, alt := range alts {
		ign := strings.TrimSpace(farmerstate.GetMiscSettingString(alt, "ei_ign"))
		displayName := alt
		if isDiscordSnowflake(alt) {
			if ign != "" {
				displayName = ei.NormalizePlayerNameForDisplay(ign)
			}
		} else {
			if ign != "" {
				displayName = fmt.Sprintf("%s (%s)", alt, ei.NormalizePlayerNameForDisplay(ign))
			}
		}
		choices = append(choices, dc.Choice[string]{
			Name:  displayName,
			Value: alt,
		})
	}

	_ = e.RespondChoices(choices)
}

// resolveArtifactTargetUserID picks whose artifacts the command is about: the
// caller, or an alternate they control. The last return is true when the
// requested alternate is not one of theirs.
func resolveArtifactTargetUserID(e *dc.CommandEvent) (string, string, bool) {
	requesterID := e.UserID()

	opt, ok := e.OptString("alternate")
	if !ok {
		return requesterID, requesterID, false
	}

	candidate := strings.TrimSpace(opt)
	if candidate == "" || candidate == requesterID {
		return requesterID, requesterID, false
	}

	for _, alt := range farmerstate.GetAltControllerByMiscString("AltController", requesterID) {
		if alt == candidate {
			return requesterID, candidate, false
		}
	}

	return requesterID, requesterID, true
}

// HandleArtifactCommand shows the artifact picker for the caller or one of
// their alternates.
func HandleArtifactCommand(e *dc.CommandEvent) {
	_, targetUserID, invalidAlt := resolveArtifactTargetUserID(e)
	if invalidAlt {
		_ = e.Respond(dc.Message{
			Content:   "The selected alternate is not linked to your account.",
			Ephemeral: true,
		})
		return
	}

	userID := targetUserID

	contractOnly := false

	str, comp := getArtifactsComponents(userID, e.ChannelID(), contractOnly, "delivery", "")

	err := e.Respond(dc.Message{
		Content:      str,
		Components:   comp,
		Ephemeral:    true,
		ComponentsV1: true,
	})
	if err != nil {
		log.Println("InteractionRespond: ", err)
	}

}

// HandleArtifactReactions handles all the button reactions for a contract settings.
//
// It still takes a raw session because updateFarmerInContracts and
// refreshBoostListMessage are not on the facade yet.
func HandleArtifactReactions(client dc.Client, e *dc.ComponentEvent) {
	// cs_#Name # cs_#ID # HASH
	reaction := strings.Split(e.CustomID(), "#")
	cmd := strings.ToLower(reaction[1])
	userID := reaction[len(reaction)-2]
	//override := reaction[len(reaction)-1]

	_ = e.DeferUpdate()

	values := e.Values()

	setValue := len(values) != 0
	page := getArtifactsPageFromContent(e.MessageContent())
	switch cmd {
	case "pagedel":
		page = "delivery"
	case "pageihr", "defl-ihr", "chalice", "monocle", "siab":
		page = "ihr"
	case "pagecol", "collegg", "collegg-lay", "collegg-ship", "collegg-ihr", "collegg-other":
		page = "collegg"
	}

	//if override == "PERM" {
	statusPrefix := ""
	backupSummary := ""
	switch cmd {
	case "popbackup":
		loadingStr, loadingComp := getArtifactsComponents(userID, e.ChannelID(), false, page, "Loading Backup...")
		_ = e.EditResponse(dc.Message{
			Content:      loadingStr,
			Components:   loadingComp,
			ComponentsV1: true,
		})
		status, summary, err := populateArtifactsFromBackup(client, userID)
		if err != nil {
			log.Printf("populateArtifactsFromBackup: %v", err)
		}
		statusPrefix = status
		backupSummary = summary
	case "defl", "metr", "comp", "guss", "defl-ihr", "chalice", "monocle", "siab":
		if setValue {
			farmerstate.SetMiscSettingString(userID, cmd, values[0])
		} else {
			farmerstate.SetMiscSettingString(userID, cmd, "") // Clear the value
		}
		updateFarmerInContracts(client, userID, "artifacts", 0)
	case "collegg", "collegg-lay", "collegg-ship", "collegg-ihr", "collegg-other":
		if cmd == "collegg" {
			farmerstate.SetMiscSettingString(userID, "collegg", strings.Join(values, ","))
		} else {
			updateColleggtibleCategorySelection(userID, cmd, values)
		}
		updateFarmerInContracts(client, userID, "artifacts", 0)
	}

	// Redraw the artifact list
	str, comp := getArtifactsComponents(userID, e.ChannelID(), false, page, "Load from Backup")
	if statusPrefix != "" {
		str = statusPrefix + "\n" + str
	}

	err := e.EditResponse(dc.Message{
		Content:      str,
		Components:   comp,
		ComponentsV1: true,
	})
	if err != nil {
		log.Println("InteractionResponseEdit: ", err)
	}

	if backupSummary != "" {
		_ = e.Followup(dc.Message{
			Content:   backupSummary,
			Ephemeral: true,
		})
	}

	//} else {
	contract := FindContract(e.ChannelID())
	if contract != nil {
		if UserInContract(contract, userID) {
			if cmd != "defl" && cmd != "metr" && cmd != "comp" && cmd != "guss" && cmd != "defl-ihr" && cmd != "chalice" && cmd != "monocle" && cmd != "siab" {
				goto done
			}
			// User in this contract
			currentSet := contract.Boosters[userID].ArtifactSet

			var prefix string
			switch cmd {
			case "defl":
				prefix = "D-"
			case "metr":
				prefix = "M-"
			case "comp":
				prefix = "C-"
			case "guss":
				prefix = "G-"
			case "defl-ihr":
				prefix = "ID-"
			case "chalice":
				prefix = "CH-"
			case "monocle":
				prefix = "MO-"
			case "siab":
				prefix = "SIAB-"
			}
			var newArtifact *ei.Artifact
			if len(values) == 0 {
				newArtifact = ei.GetArtifactByKey(prefix + "NONE")
			} else {
				val := strings.TrimSuffix(values[0], "_L")
				newArtifact = ei.GetArtifactByKey(prefix + val)
			}

			// Check if artifact was found in map
			if newArtifact != nil {
				// Check if the artifact already exists in the current set
				exists := false
				for i, artifact := range currentSet.Artifacts {
					if artifact.Type == newArtifact.Type {
						exists = true
						if setValue {
							currentSet.Artifacts[i] = *newArtifact
						} else {
							// Removing this artifact
							currentSet.Artifacts = append(currentSet.Artifacts[:i], currentSet.Artifacts[i+1:]...)
						}
						break
					}
				}
				// If the artifact doesn't exist, add it to the current set
				if !exists {
					currentSet.Artifacts = append(currentSet.Artifacts, *newArtifact)
				}

				contract.Boosters[userID].ArtifactSet = getUserArtifacts(userID, &currentSet)

				refreshBoostListMessage(client, contract, false)
				saveData(contract.ContractHash)
			}

		}
	}

done:
	//}
}
