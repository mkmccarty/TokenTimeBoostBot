package boost

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"google.golang.org/protobuf/proto"
)

func TestRenderCoopStatusStones_DepartedNoMissingColleggtibles(t *testing.T) {
	// Setup CustomEggMap so that maximum colleggtibles are > 1.0
	ei.CustomEggMap = make(map[string]*ei.EggIncCustomEgg)
	ei.CustomEggMap["test-hab"] = &ei.EggIncCustomEgg{
		Dimension:      ei.GameModifier_HAB_CAPACITY,
		DimensionValue: []float64{1.05},
	}
	ei.CustomEggMap["test-elr"] = &ei.EggIncCustomEgg{
		Dimension:      ei.GameModifier_EGG_LAYING_RATE,
		DimensionValue: []float64{1.05},
	}
	ei.CustomEggMap["test-sr"] = &ei.EggIncCustomEgg{
		Dimension:      ei.GameModifier_SHIPPING_CAPACITY,
		DimensionValue: []float64{1.05},
	}
	ei.SetColleggtibleValues()

	contractID := "test-stones-contract"

	// Register test contract
	grades := make([]ei.ContractGrade, 6)
	grades[int(ei.Contract_GRADE_AAA)] = ei.ContractGrade{
		ModifierELR:     1.0,
		ModifierSR:      1.0,
		ModifierHabCap:  1.0,
		TargetAmount:    []float64{1e15},
		LengthInSeconds: 86400,
	}

	if ei.EggIncContractsAll == nil {
		ei.EggIncContractsAll = make(map[string]ei.EggIncContract)
	}
	ei.EggIncContractsAll[contractID] = ei.EggIncContract{
		ID:              contractID,
		MinutesPerToken: 60,
		ModifierELR:     1.0,
		ModifierSR:      1.0,
		ModifierHabCap:  1.0,
		Grade:           grades,
	}

	// Build a mock ContractCoopStatusResponse with an active player and a departed player.
	coopStatus := &ei.ContractCoopStatusResponse{
		ContractIdentifier: proto.String(contractID),
		CoopIdentifier:     proto.String("test-coop"),
		ResponseStatus:     ei.ContractCoopStatusResponse_NO_ERROR.Enum(),
		Grade:              ei.Contract_GRADE_AAA.Enum(),
		Contributors: []*ei.ContractCoopStatusResponse_ContributionInfo{
			{
				UserId:             proto.String("EI1111111111111111"),
				UserName:           proto.String("ActivePlayer"),
				ContributionAmount: proto.Float64(1e12),
				ColleggtibleInfo:   &ei.PlayerColleggtibleInfo{},
				FarmInfo:           &ei.PlayerFarmInfo{},
				ProductionParams:   &ei.FarmProductionParams{},
			},
			{
				UserId:             proto.String("EI2222222222222222"),
				UserName:           proto.String("[departed]"),
				ContributionAmount: proto.Float64(5e11),
				FarmInfo:           &ei.PlayerFarmInfo{},
				ProductionParams:   &ei.FarmProductionParams{},
			},
		},
	}

	for _, details := range []bool{false, true} {
		result, _, _ := renderCoopStatusStones("chan-1", contractID, coopStatus, details, "", false, "")

		lines := strings.Split(result, "\n")
		var activeLine, departedLine string
		for _, line := range lines {
			if strings.HasPrefix(line, "`") && !strings.HasPrefix(line, "```") {
				if strings.Contains(line, "ActivePlayer") {
					activeLine = line
				}
				if strings.Contains(line, "[departed]") {
					departedLine = line
				}
			}
		}

		if activeLine == "" {
			t.Fatalf("details=%v: expected ActivePlayer row in output, got:\n%s", details, result)
		}
		if departedLine == "" {
			t.Fatalf("details=%v: expected [departed] row in output, got:\n%s", details, result)
		}

		// Active player with no colleggtibles should show all missing colleggtible icons
		colleggtibleIcons := []string{"🛖", "📦", "🚚"}
		for _, icon := range colleggtibleIcons {
			if !strings.Contains(activeLine, icon) {
				t.Errorf("details=%v: expected active player row to show missing colleggtible icon %q, got: %s", details, icon, activeLine)
			}
		}

		// Departed player must NOT show any missing colleggtible icons on their row
		for _, icon := range colleggtibleIcons {
			if strings.Contains(departedLine, icon) {
				t.Errorf("details=%v: departed player row must not contain missing colleggtible icon %q, got: %s", details, icon, departedLine)
			}
		}
	}
}
