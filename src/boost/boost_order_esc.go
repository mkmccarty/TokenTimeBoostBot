package boost

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	ESCRoleSIAB    = 1
	ESCRoleGusset  = 2
	ESCRoleQuant   = 3
	ESCRoleRegular = 4
	ESCRoleAlt     = 5
)

// getESCRolePriority determines the booster's role priority bucket:
// 1: SIAB
// 2: Gusset
// 3: Quant
// 4: Regular Main
// 5: Alt
func getESCRolePriority(b *Booster) int {
	if b == nil {
		return ESCRoleAlt
	}
	// Alts always go to the alt tier
	if b.AltController != "" {
		return ESCRoleAlt
	}

	// 1. SIAB
	if hasBoosterArtifact(b, "SIAB") || (farmerstate.GetMiscSettingString(b.UserID, "siab") != "" && farmerstate.GetMiscSettingString(b.UserID, "siab") != "NONE") {
		return ESCRoleSIAB
	}

	// 2. Gusset
	if hasBoosterArtifact(b, "Gusset") || (farmerstate.GetMiscSettingString(b.UserID, "guss") != "" && farmerstate.GetMiscSettingString(b.UserID, "guss") != "NONE") {
		return ESCRoleGusset
	}

	// 3. Quant
	if isBoosterQuant(b) {
		return ESCRoleQuant
	}

	// 4. Regular
	return ESCRoleRegular
}

func hasBoosterArtifact(b *Booster, artType string) bool {
	if b == nil {
		return false
	}
	for _, a := range b.ArtifactSet.Artifacts {
		if a.Type == artType && a.Quality != "NONE" && a.Quality != "" {
			return true
		}
	}
	return false
}

func isBoosterQuant(b *Booster) bool {
	if b == nil {
		return false
	}
	if farmerstate.GetMiscSettingFlag(b.UserID, "quant") || farmerstate.GetMiscSettingString(b.UserID, "quant") != "" {
		return true
	}
	stoneSetting := strings.ToLower(farmerstate.GetMiscSettingString(b.UserID, "stone"))
	if strings.Contains(stoneSetting, "quant") {
		return true
	}
	for _, a := range b.ArtifactSet.Artifacts {
		if a.Type == "Compass" || strings.Contains(strings.ToLower(a.Type), "quant") {
			return true
		}
	}
	return false
}

// getESCDeflectorScore returns deflector priority score based on run type:
// GG runs: T4L (4) > T4E (3) > T4R (2) > T3R (1) > Other (0)
// Standard runs: 2-slot (3) > 1-slot (2) > T3R (1) > Other (0)
func getESCDeflectorScore(b *Booster, isGG bool) int {
	if b == nil {
		return 0
	}
	quality := ""
	for _, a := range b.ArtifactSet.Artifacts {
		if a.Type == "Deflector" || a.Type == "IHR Deflector" {
			quality = a.Quality
			break
		}
	}
	if quality == "" {
		quality = farmerstate.GetMiscSettingString(b.UserID, "defl")
	}
	if quality == "" {
		quality = farmerstate.GetMiscSettingString(b.UserID, "defl-ihr")
	}

	quality = strings.ToUpper(strings.TrimSpace(quality))
	if strings.HasSuffix(quality, "_L") {
		quality = strings.TrimSuffix(quality, "_L")
	}

	if isGG {
		switch quality {
		case "T4L":
			return 4
		case "T4E":
			return 3
		case "T4R":
			return 2
		case "T3R":
			return 1
		default:
			return 0
		}
	} else {
		// Standard runs: 2-slot (T4L, T4E) > 1-slot (T4R) > T3R > 0-slot / Other
		switch quality {
		case "T4L", "T4E":
			return 3
		case "T4R":
			return 2
		case "T3R":
			return 1
		default:
			return 0
		}
	}
}

type escSortItem struct {
	userID       string
	rolePriority int
	deflScore    int
	elr          float64
	ihr          float64
	tokensWanted int
	te           int
}

// sortESCRemaining sorts the provided booster userIDs according to ESC rules.
func sortESCRemaining(contract *Contract, unselected []string, isGG bool) []string {
	if len(unselected) <= 1 {
		return append([]string(nil), unselected...)
	}

	items := make([]escSortItem, len(unselected))
	for i, userID := range unselected {
		b := contract.Boosters[userID]
		rolePrio := getESCRolePriority(b)
		deflScore := getESCDeflectorScore(b, isGG)
		elrVal := 0.0
		ihrVal := 0.0
		tokensW := 0
		teVal := 0

		if b != nil {
			elrVal = b.ArtifactSet.LayRate
			ihrVal = b.IHRRate
			tokensW = b.TokensWanted
			teVal = b.TECount

			if !isGG {
				// Standard runs: Fuzzy IHR (6% random bonus)
				randomBonusMax := ihrVal * 0.06
				randomOffset := (rand.Float64()*2 - 1) * randomBonusMax
				sortIHR := ihrVal + randomOffset
				b.FuzzyOffset = randomOffset
				if b.IHRCalcLog == "" {
					b.IHRCalcLog = fmt.Sprintf("IHR Rate = %0.2f", ihrVal)
				} else if idx := strings.Index(b.IHRCalcLog, ", Fuzzy ("); idx != -1 {
					b.IHRCalcLog = b.IHRCalcLog[:idx]
				}
				b.IHRCalcLog = fmt.Sprintf("%s, Fuzzy (Max=%0.2f, Offset=%0.2f, Sorted=%0.2f)", b.IHRCalcLog, randomBonusMax, randomOffset, sortIHR)
				ihrVal = sortIHR
			}
		}

		items[i] = escSortItem{
			userID:       userID,
			rolePriority: rolePrio,
			deflScore:    deflScore,
			elr:          elrVal,
			ihr:          ihrVal,
			tokensWanted: tokensW,
			te:           teVal,
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		// 1. First sort: Role priority (SIAB -> Gusset -> Quant -> Regular -> Alt)
		if items[i].rolePriority != items[j].rolePriority {
			return items[i].rolePriority < items[j].rolePriority
		}

		if isGG {
			// In GG runs: Def sort can be skipped if equivalent ELR
			equivELR := math.Abs(items[i].elr-items[j].elr) < 1e-4
			if !equivELR && items[i].deflScore != items[j].deflScore {
				return items[i].deflScore > items[j].deflScore
			}

			// Boost token plan priority: IHR multi is primary sort
			if items[i].ihr != items[j].ihr {
				return items[i].ihr > items[j].ihr
			}
			if items[i].tokensWanted != items[j].tokensWanted {
				return items[i].tokensWanted < items[j].tokensWanted
			}
			if items[i].te != items[j].te {
				return items[i].te > items[j].te
			}
		} else {
			// Standard runs: Deflector (2-slot > 1-slot > T3R)
			if items[i].deflScore != items[j].deflScore {
				return items[i].deflScore > items[j].deflScore
			}
			// Fuzzy IHR
			if items[i].ihr != items[j].ihr {
				return items[i].ihr > items[j].ihr
			}
			if items[i].tokensWanted != items[j].tokensWanted {
				return items[i].tokensWanted < items[j].tokensWanted
			}
			if items[i].te != items[j].te {
				return items[i].te > items[j].te
			}
		}

		return items[i].userID < items[j].userID
	})

	sorted := make([]string, len(items))
	for i, item := range items {
		sorted[i] = item.userID
	}
	return sorted
}

// GenerateESCOrderReport builds a detailed breakdown of the ESC boost order calculations.
func GenerateESCOrderReport(contract *Contract) string {
	if contract == nil {
		return "Contract not found."
	}
	contract.mutex.Lock()
	defer contract.mutex.Unlock()

	isGG := contract.BoostOrder == ContractOrderESCGG
	runTypeName := "Standard / Leggacy / First Run"
	if isGG {
		runTypeName = "GG Run"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## 🪐 ESC Boost Order Calculations\n")
	fmt.Fprintf(&sb, "**Contract:** `%s` | **Coop:** `%s` | **Mode:** %s\n", contract.ContractID, contract.CoopID, runTypeName)
	fmt.Fprintf(&sb, "-# Hierarchy: (1) SIAB > Gusset > Quant > Main > Alts | (2) Deflector | (3) IHR / Token Plan\n\n")

	orderList := contract.Order
	if len(contract.OriginalOrder) > 0 {
		orderList = contract.OriginalOrder
	}
	sorted := sortESCRemaining(contract, orderList, isGG)

	fmt.Fprintf(&sb, "```\n")
	fmt.Fprintf(&sb, "%-3s %-16s %-8s %-10s %-7s %-8s %-4s %-4s\n", "#", "Player", "Role", "Deflector", "ELR", "IHR", "Ask", "TE")
	fmt.Fprintf(&sb, "%-3s %-16s %-8s %-10s %-7s %-8s %-4s %-4s\n", "---", "----------------", "--------", "----------", "-------", "--------", "----", "----")

	for idx, userID := range sorted {
		b := contract.Boosters[userID]
		if b == nil {
			continue
		}
		name := b.Nick
		if name == "" {
			name = b.UserName
		}
		if name == "" {
			name = b.GlobalName
		}
		if name == "" {
			name = userID
		}
		if len(name) > 16 {
			name = name[:16]
		}

		roleStr := "Main"
		switch getESCRolePriority(b) {
		case ESCRoleSIAB:
			roleStr = "SIAB"
		case ESCRoleGusset:
			roleStr = "Gusset"
		case ESCRoleQuant:
			roleStr = "Quant"
		case ESCRoleAlt:
			roleStr = "Alt"
		}

		deflQuality := ""
		for _, a := range b.ArtifactSet.Artifacts {
			if a.Type == "Deflector" || a.Type == "IHR Deflector" {
				deflQuality = a.Quality
				break
			}
		}
		if deflQuality == "" {
			deflQuality = farmerstate.GetMiscSettingString(b.UserID, "defl")
		}
		if deflQuality == "" {
			deflQuality = farmerstate.GetMiscSettingString(b.UserID, "defl-ihr")
		}
		deflQuality = strings.TrimSpace(deflQuality)
		if deflQuality == "" || deflQuality == "NONE" {
			deflQuality = "-"
		}

		ihrMult := fmt.Sprintf("%0.2fx", b.IHRRate/DefaultLeggyIHR)
		elrStr := fmt.Sprintf("%0.2f", b.ArtifactSet.LayRate)
		askStr := fmt.Sprintf("%d", b.TokensWanted)
		teStr := fmt.Sprintf("%d", b.TECount)

		fmt.Fprintf(&sb, "%-3d %-16s %-8s %-10s %-7s %-8s %-4s %-4s\n", idx+1, name, roleStr, deflQuality, elrStr, ihrMult, askStr, teStr)
	}
	fmt.Fprintf(&sb, "```\n")

	return sb.String()
}

// sendESCOrderCalculationReport sends the ESC order calculations report to the channel.
func sendESCOrderCalculationReport(client dc.Client, channelID string, contract *Contract) {
	report := GenerateESCOrderReport(contract)
	if len(report) > 1950 {
		_, _ = client.SendMessage(channelID, dc.Message{
			Content: "## 🪐 ESC Boost Order Calculations",
			Files: []dc.File{{
				Name:        "esc_order_calculations.txt",
				ContentType: "text/plain",
				Reader:      strings.NewReader(report),
			}},
		})
	} else {
		_, _ = client.SendMessage(channelID, dc.Message{Content: report})
	}
}

