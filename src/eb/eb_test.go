package eb

import (
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
	if !strings.Contains(embed.Description, "**Naked EB**:") || !strings.Contains(embed.Description, "**Dressed EB**:") {
		t.Errorf("expected Naked EB and Dressed EB in description: %s", embed.Description)
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
	if !strings.Contains(embed.Description, "**Actual EB**:") {
		t.Errorf("missing Actual EB in Virtue section: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "**Pending EB**:") {
		t.Errorf("expected Pending EB when pendingTE > 0: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "### 🏠 Home Farm (with pending TE)") {
		t.Errorf("expected Home Farm with pending TE when pendingTE > 0: %s", embed.Description)
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
