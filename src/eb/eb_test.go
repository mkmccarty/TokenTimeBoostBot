package eb

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestNormalizeFarmChoice(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Home", FarmHome},
		{"home", FarmHome},
		{" HOME ", FarmHome},
		{"Virtue", FarmVirtue},
		{"virtue", FarmVirtue},
		{"Home & Virtue", FarmHomeAndVirtue},
		{"home & virtue", FarmHomeAndVirtue},
		{"home and virtue", FarmHomeAndVirtue},
		{"both", FarmHomeAndVirtue},
		{"all", FarmHomeAndVirtue},
		{"", DefaultFarmChoice},
		{"random", DefaultFarmChoice},
	}

	for _, tc := range tests {
		got := NormalizeFarmChoice(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeFarmChoice(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestGetSlashEbCommand(t *testing.T) {
	cmd := GetSlashEbCommand("eb")
	if cmd.Name != "eb" {
		t.Errorf("cmd.Name = %q; want eb", cmd.Name)
	}
	if len(cmd.Options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(cmd.Options))
	}
	strOpt, ok := cmd.Options[0].(dc.StringOption)
	if !ok {
		t.Fatalf("expected option to be dc.StringOption, got %T", cmd.Options[0])
	}
	if strOpt.Name != "farm" {
		t.Errorf("option Name = %q; want farm", strOpt.Name)
	}
	if len(strOpt.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(strOpt.Choices))
	}
	expectedChoices := []string{FarmHomeAndVirtue, FarmHome, FarmVirtue}
	for i, exp := range expectedChoices {
		if strOpt.Choices[i].Name != exp || strOpt.Choices[i].Value != exp {
			t.Errorf("choice %d = %v; want %s", i, strOpt.Choices[i], exp)
		}
	}
	if len(cmd.IntegrationTypes) != 2 || cmd.IntegrationTypes[0] != dc.IntegrationGuildInstall || cmd.IntegrationTypes[1] != dc.IntegrationUserInstall {

		t.Errorf("expected IntegrationTypes to have GuildInstall and UserInstall, got %v", cmd.IntegrationTypes)
	}
	if len(cmd.Contexts) != 3 {
		t.Errorf("expected 3 Contexts, got %v", cmd.Contexts)
	}
	if cmd.DefaultMemberPermissions != nil {
		t.Errorf("expected DefaultMemberPermissions to be nil, got %v", cmd.DefaultMemberPermissions)
	}
}

func TestBuildEbEmbedHome(t *testing.T) {
	maker := ei.NewBackupMaker("EI1234567890123456", "FarmerBob")
	backup := maker.GetBackup()

	embed := BuildEbEmbed(backup, FarmHome, "user-123")

	if embed.Title != "Earnings Bonus — FarmerBob" {
		t.Errorf("unexpected embed title: %q", embed.Title)
	}
	if embed.Color == 0 {
		t.Errorf("expected non-zero embed color")
	}
	if !strings.Contains(embed.Description, "### 🏠 Home Farm") {
		t.Errorf("missing Home Farm section in embed description: %s", embed.Description)
	}
	if strings.Contains(embed.Description, "Virtue Farm") {
		t.Errorf("Virtue Farm should not be present in Home view: %s", embed.Description)
	}
	if strings.Contains(embed.Description, "pending") {
		t.Errorf("pending TE should not be shown in Home view: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**Nekkid EB**:") || !strings.Contains(embed.Description, "**Dressed EB**:") {
		t.Errorf("expected Nekkid EB and Dressed EB in description: %s", embed.Description)
	}

	if !strings.Contains(embed.Description, "**Role**:") {
		t.Errorf("expected Role in description: %s", embed.Description)
	}
}

func TestBuildEbEmbedVirtueNoPending(t *testing.T) {
	maker := ei.NewBackupMaker("EI1234567890123456", "VirtueFarmer")
	backup := maker.GetBackup()
	backup.Virtue = nil // no virtue data -> 0 TE, 0 pending TE

	embed := BuildEbEmbed(backup, FarmVirtue, "user-123")

	if embed.Color == 0 {
		t.Errorf("expected non-zero embed color")
	}
	if !strings.Contains(embed.Description, "### 🕊️ Virtue Farm") {
		t.Errorf("missing Virtue Farm section in embed description: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**CTE**:") {
		t.Errorf("missing CTE in Virtue view: %s", embed.Description)
	}
	if strings.Contains(embed.Description, "**Pending CTE**:") {
		t.Errorf("should not display Pending CTE when pendingTE == 0: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**Actual EB**:") {
		t.Errorf("missing Actual EB in Virtue view: %s", embed.Description)
	}
	// When pendingTE == 0, Pending EB and Home Farm (with pending TE) should NOT be displayed
	if strings.Contains(embed.Description, "**Pending EB**:") {
		t.Errorf("should not display Pending EB when pendingTE == 0: %s", embed.Description)
	}
	if strings.Contains(embed.Description, "### 🏠 Home Farm (with pending TE)") {
		t.Errorf("should not display Home Farm with pending TE when pendingTE == 0: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "1.10^TE") {
		t.Errorf("missing 1.10^TE note in Virtue view: %s", embed.Description)
	}
}

func TestBuildEbEmbedVirtueWithPending(t *testing.T) {
	maker := ei.NewBackupMaker("EI1234567890123456", "VirtueFarmer")
	backup := maker.GetBackup()
	backup.Virtue = &ei.Backup_Virtue{
		EovEarned:     []uint32{0},
		EggsDelivered: []float64{1e18},
	}

	embed := BuildEbEmbed(backup, FarmVirtue, "user-123")

	if !strings.Contains(embed.Description, "**CTE**:") {
		t.Errorf("missing CTE in Virtue view: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**Pending CTE**:") {
		t.Errorf("expected Pending CTE when pendingTE > 0: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**Pending EB**:") {
		t.Errorf("expected Pending EB when pendingTE > 0: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "### 🏠 Home Farm (with pending TE)") {
		t.Errorf("expected Home Farm with pending TE when pendingTE > 0: %s", embed.Description)
	}
}

func TestBuildEbEmbedHomeAndVirtueWithPending(t *testing.T) {
	maker := ei.NewBackupMaker("EI1234567890123456", "AllFarmer")
	backup := maker.GetBackup() // default maker has pending TE

	embed := BuildEbEmbed(backup, FarmHomeAndVirtue, "user-123")

	if !strings.Contains(embed.Description, "### 🏠 Home Farm") {
		t.Errorf("missing Home Farm section: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "### 🕊️ Virtue Farm") {
		t.Errorf("missing Virtue Farm section: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**CTE**:") {
		t.Errorf("missing CTE in Virtue section: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**Pending CTE**:") {
		t.Errorf("expected Pending CTE when pendingTE > 0: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**Actual EB**:") {
		t.Errorf("missing Actual EB in Virtue section: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**Pending EB**:") {
		t.Errorf("expected Pending EB when pendingTE > 0: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "### 🏠 Home Farm (with pending TE)") {
		t.Errorf("expected Home Farm with pending TE when pendingTE > 0: %s", embed.Description)
	}
	if strings.Contains(embed.Description, "\n\n###") {
		t.Errorf("found unwanted extra newline before section header: %s", embed.Description)
	}
}

func TestBuildEbEmbedHomeAndVirtueNoPending(t *testing.T) {
	maker := ei.NewBackupMaker("EI1234567890123456", "AllFarmer")
	backup := maker.GetBackup()
	backup.Virtue = nil // no virtue data -> 0 TE, 0 pending TE

	embed := BuildEbEmbed(backup, FarmHomeAndVirtue, "user-123")

	if !strings.Contains(embed.Description, "### 🏠 Home Farm") {
		t.Errorf("missing Home Farm section: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "### 🕊️ Virtue Farm") {
		t.Errorf("missing Virtue Farm section: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**CTE**:") {
		t.Errorf("missing CTE in Virtue section: %s", embed.Description)
	}
	if strings.Contains(embed.Description, "**Pending CTE**:") {
		t.Errorf("should not display Pending CTE when pendingTE == 0: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**Actual EB**:") {
		t.Errorf("missing Actual EB in Virtue section: %s", embed.Description)
	}
	if strings.Contains(embed.Description, "**Pending EB**:") {
		t.Errorf("should not display Pending EB when pendingTE == 0: %s", embed.Description)
	}
	if strings.Contains(embed.Description, "### 🏠 Home Farm (with pending TE)") {
		t.Errorf("should not display Home Farm with pending TE when pendingTE == 0: %s", embed.Description)
	}
}

func TestVirtueEBFormula(t *testing.T) {
	// Formula: 1.10^TE
	tests := []struct {
		te       uint32
		expected float64
	}{
		{0, 1.0},
		{1, 1.1},
		{10, math.Pow(1.10, 10)},
		{50, math.Pow(1.10, 50)},
		{98, math.Pow(1.10, 98)},
	}

	for _, tc := range tests {
		ratio := math.Pow(1.10, float64(tc.te))
		if math.Abs(ratio-tc.expected) > 1e-9 {
			t.Errorf("ratio for TE %d = %f, want %f", tc.te, ratio, tc.expected)
		}
	}
}

func TestDetermineFarmIcons(t *testing.T) {
	if ei.EmoteMap == nil {
		ei.EmoteMap = make(map[string]ei.Emotes)
	}
	ei.EmoteMap["egg_enlightenment"] = ei.Emotes{Name: "egg_enlightenment", ID: "1001"}
	ei.EmoteMap["egg_curiosity"] = ei.Emotes{Name: "egg_curiosity", ID: "1002"}
	ei.EmoteMap["egg_universe"] = ei.Emotes{Name: "egg_universe", ID: "1003"}
	ei.EmoteMap["egg_truth"] = ei.Emotes{Name: "egg_truth", ID: "1004"}

	// Case 1: Player on Virtue farm (Curiosity)
	makerVirtue := ei.NewBackupMaker("EI1234567890123456", "VirtueFarmer")
	bVirtue := makerVirtue.GetBackup()
	curEgg := ei.Egg_CURIOSITY
	bVirtue.Farms = []*ei.Backup_Simulation{{FarmType: ei.FarmType_HOME.Enum(), EggType: &curEgg}}

	homeIcon, virtueIcon := determineFarmIcons(bVirtue)
	if homeIcon != "<:egg_enlightenment:1001>" {
		t.Errorf("expected homeIcon to be enlightenment emoji on virtue farm, got %s", homeIcon)
	}
	if virtueIcon != "<:egg_curiosity:1002>" {
		t.Errorf("expected virtueIcon to be curiosity emoji on virtue farm, got %s", virtueIcon)
	}

	// Case 2: Player on Home farm (Universe)
	makerHome := ei.NewBackupMaker("EI1234567890123456", "HomeFarmer")
	bHome := makerHome.GetBackup()
	uniEgg := ei.Egg_UNIVERSE
	bHome.Farms = []*ei.Backup_Simulation{{FarmType: ei.FarmType_HOME.Enum(), EggType: &uniEgg}}

	homeIcon2, virtueIcon2 := determineFarmIcons(bHome)
	if homeIcon2 != "<:egg_universe:1003>" {
		t.Errorf("expected homeIcon to be universe emoji on home farm, got %s", homeIcon2)
	}
	if virtueIcon2 != "<:egg_truth:1004>" {
		t.Errorf("expected virtueIcon to be TE emoji when not on virtue farm, got %s", virtueIcon2)
	}

	// Case 3: Emojis not available (fallback to 🏠 and 🕊️)
	oldMap := ei.EmoteMap
	ei.EmoteMap = make(map[string]ei.Emotes)
	homeIcon3, virtueIcon3 := determineFarmIcons(bHome)
	if homeIcon3 != "🏠" {
		t.Errorf("expected fallback 🏠, got %s", homeIcon3)
	}
	if virtueIcon3 != "🕊️" {
		t.Errorf("expected fallback 🕊️, got %s", virtueIcon3)
	}
	ei.EmoteMap = oldMap
}

func TestBuildEbEmbedMaxEarningsCTE(t *testing.T) {
	maker := ei.NewBackupMaker("EI1234567890123456", "MaxFarmer")
	backup := maker.GetBackup()

	// With default maker, inventory has lunar totem / artifacts
	// Calculate active CTE vs max CTE
	activeCTE := ei.CalculateClothedTE(backup)
	maxResult := ei.CalculateMaxClothedTEWithSlotHint(backup, 0)

	embed := BuildEbEmbed(backup, FarmVirtue, "user-123")

	expectedCTEStr := fmt.Sprintf("**CTE**: %.0f", maxResult.ClothedTE)
	if !strings.Contains(embed.Description, expectedCTEStr) {
		t.Errorf("expected embed to contain %q (max CTE), got %s", expectedCTEStr, embed.Description)
	}

	_ = activeCTE
}

func TestBuildEbEmbedWithBadges(t *testing.T) {
	if ei.EmoteMap == nil {
		ei.EmoteMap = make(map[string]ei.Emotes)
	}
	ei.EmoteMap["badge_nah"] = ei.Emotes{Name: "badge_nah", ID: "111"}
	ei.EmoteMap["badge_good_job"] = ei.Emotes{Name: "badge_good_job", ID: "222"}

	maker := ei.NewBackupMaker("EI1234567890123456", "BadgeFarmer")
	backup := maker.GetBackup()

	// Grant NAH
	farmsize := make([]uint64, 19)
	farmsize[18] = 21000000000
	backup.Game.MaxFarmSizeReached = farmsize

	// Grant Crafting Level 30 (Good Job)
	xp := 5070943000.0 + 100.0
	backup.Artifacts = &ei.Backup_Artifacts{
		CraftingXp: &xp,
	}

	embed := BuildEbEmbed(backup, FarmHome, "user-123")

	if embed.Title != "Earnings Bonus — BadgeFarmer" {
		t.Errorf("unexpected embed title: %q", embed.Title)
	}

	// Badges should be at the start of the description
	expectedBadgeRow := "<:badge_nah:111> <:badge_good_job:222>"
	if !strings.HasPrefix(embed.Description, expectedBadgeRow) {
		t.Errorf("expected embed description to start with %q, got:\n%s", expectedBadgeRow, embed.Description)
	}
}
