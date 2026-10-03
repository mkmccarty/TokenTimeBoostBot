package eb

import (
	"encoding/base64"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
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
	if len(cmd.Options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(cmd.Options))
	}
	strOpt, ok := cmd.Options[0].(dc.StringOption)
	if !ok {
		t.Fatalf("expected option 0 to be dc.StringOption, got %T", cmd.Options[0])
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
	boolOpt, ok := cmd.Options[1].(dc.BoolOption)
	if !ok {
		t.Fatalf("expected option 1 to be dc.BoolOption, got %T", cmd.Options[1])
	}
	if boolOpt.Name != "show-avatar" {
		t.Errorf("option Name = %q; want show-avatar", boolOpt.Name)
	}
	if boolOpt.Required {
		t.Errorf("expected show-avatar option to not be required")
	}
	altOpt, ok := cmd.Options[2].(dc.StringOption)
	if !ok {
		t.Fatalf("expected option 2 to be dc.StringOption, got %T", cmd.Options[2])
	}
	if altOpt.Name != "alt" {
		t.Errorf("option Name = %q; want alt", altOpt.Name)
	}
	if altOpt.Required {
		t.Errorf("expected alt option to not be required")
	}
	if !altOpt.Autocomplete {
		t.Errorf("expected alt option to have Autocomplete = true")
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

	if embed.Title != "FarmerBob" {
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

	if embed.Title != "BadgeFarmer" {
		t.Errorf("unexpected embed title: %q", embed.Title)
	}

	// Badges should be at the start of the description
	expectedBadgeRow := "<:badge_nah:111> <:badge_good_job:222>"
	if !strings.HasPrefix(embed.Description, expectedBadgeRow) {
		t.Errorf("expected embed description to start with %q, got:\n%s", expectedBadgeRow, embed.Description)
	}
}

func TestBuildEbEmbedStandardPermit(t *testing.T) {
	makerPro := ei.NewBackupMaker("EI1234567890123456", "ProFarmer")
	makerPro.SetPermitLevel(1)
	backupPro := makerPro.GetBackup()

	makerStd := ei.NewBackupMaker("EI1234567890123456", "StdFarmer")
	makerStd.SetPermitLevel(0)
	backupStd := makerStd.GetBackup()

	// Pro permit player with 4 artifact slots should have higher or equal Dressed EB than standard permit with 2 slots
	proDressedEB := ei.GetDressedEarningsBonus(backupPro, 0)
	stdDressedEB := ei.GetDressedEarningsBonus(backupStd, 0)

	if stdDressedEB > proDressedEB {
		t.Errorf("standard permit player Dressed EB (%e) should not exceed pro permit player Dressed EB (%e)", stdDressedEB, proDressedEB)
	}

	// Verify BuildEbEmbed works properly for standard permit
	embedStd := BuildEbEmbed(backupStd, FarmHomeAndVirtue, "user-std")
	homeIcon, virtueIcon := determineFarmIcons(backupStd)
	if !strings.Contains(embedStd.Description, fmt.Sprintf("### %s Home Farm", homeIcon)) {
		t.Errorf("missing Home Farm section for standard permit player")
	}
	if !strings.Contains(embedStd.Description, fmt.Sprintf("### %s Virtue Farm", virtueIcon)) {
		t.Errorf("missing Virtue Farm section for standard permit player")
	}

	// Standard permit max CTE should use 2 slots
	stdCTEResult := ei.CalculateMaxClothedTEWithSlotHint(backupStd, 0)
	expectedCTEStr := fmt.Sprintf("**CTE**: %.0f", stdCTEResult.ClothedTE)
	if !strings.Contains(embedStd.Description, expectedCTEStr) {
		t.Errorf("expected standard permit embed to contain %q, got %s", expectedCTEStr, embedStd.Description)
	}
}

func TestBuildEbEmbed_PermitEmojis(t *testing.T) {
	oldMap := ei.EmoteMap
	ei.EmoteMap = make(map[string]ei.Emotes)
	ei.EmoteMap["pro_permit"] = ei.Emotes{Name: "pro_permit", ID: "3001"}
	ei.EmoteMap["free_permit"] = ei.Emotes{Name: "free_permit", ID: "3002"}
	defer func() {
		ei.EmoteMap = oldMap
	}()

	makerPro := ei.NewBackupMaker("EI1234567890123456", "ProFarmer")
	makerPro.SetPermitLevel(1)
	embedPro := BuildEbEmbed(makerPro.GetBackup(), FarmHomeAndVirtue, "user-pro")
	if !strings.HasPrefix(embedPro.Description, "<:pro_permit:3001>") {
		t.Errorf("expected embedPro description to start with pro_permit emoji, got:\n%s", embedPro.Description)
	}

	makerStd := ei.NewBackupMaker("EI1234567890123456", "StdFarmer")
	makerStd.SetPermitLevel(0)
	embedStd := BuildEbEmbed(makerStd.GetBackup(), FarmHomeAndVirtue, "user-std")
	if !strings.HasPrefix(embedStd.Description, "<:free_permit:3002>") {
		t.Errorf("expected embedStd description to start with free_permit emoji, got:\n%s", embedStd.Description)
	}
}

func TestBuildEbComponents_WithAvatar(t *testing.T) {
	maker := ei.NewBackupMaker("EI1234567890123456", "FarmerBob")
	backup := maker.GetBackup()
	avatarURL := "https://cdn.discordapp.com/avatars/123/abc.png"

	components := BuildEbComponents(backup, FarmHomeAndVirtue, "user-123", avatarURL)
	if len(components) != 1 {
		t.Fatalf("expected 1 layout component, got %d", len(components))
	}

	container, ok := components[0].(dc.Container)
	if !ok {
		t.Fatalf("expected dc.Container, got %T", components[0])
	}

	if container.AccentColor == 0 {
		t.Errorf("expected non-zero container accent color")
	}

	if len(container.Components) < 3 {
		t.Fatalf("expected at least 3 container sub-components, got %d", len(container.Components))
	}

	// First component should be a Section with the Thumbnail accessory
	section, ok := container.Components[0].(dc.Section)
	if !ok {
		t.Fatalf("expected first sub-component to be dc.Section, got %T", container.Components[0])
	}

	if len(section.Components) != 1 {
		t.Fatalf("expected 1 TextDisplay in section, got %d", len(section.Components))
	}
	if !strings.Contains(section.Components[0].Content, "## FarmerBob") {
		t.Errorf("expected section content to contain title, got: %s", section.Components[0].Content)
	}

	thumb, ok := section.Accessory.(dc.Thumbnail)
	if !ok {
		t.Fatalf("expected section accessory to be dc.Thumbnail, got %T", section.Accessory)
	}
	if thumb.URL != avatarURL {
		t.Errorf("expected thumbnail URL %q, got %q", avatarURL, thumb.URL)
	}

	// Second component should be a Separator
	separator, ok := container.Components[1].(dc.Separator)
	if !ok {
		t.Fatalf("expected second sub-component to be dc.Separator, got %T", container.Components[1])
	}
	if !separator.Divider {
		t.Errorf("expected separator to have Divider = true")
	}

	// Third component should be a TextDisplay with farm details
	farmDisplay, ok := container.Components[2].(dc.TextDisplay)
	if !ok {
		t.Fatalf("expected third sub-component to be dc.TextDisplay, got %T", container.Components[2])
	}
	if !strings.Contains(farmDisplay.Content, "Home Farm") {
		t.Errorf("expected farm display to contain Home Farm, got: %s", farmDisplay.Content)
	}
}

func TestBuildEbComponents_WithoutAvatar(t *testing.T) {
	maker := ei.NewBackupMaker("EI1234567890123456", "FarmerBob")
	backup := maker.GetBackup()

	components := BuildEbComponents(backup, FarmHomeAndVirtue, "user-123", "")
	if len(components) != 1 {
		t.Fatalf("expected 1 layout component, got %d", len(components))
	}

	container, ok := components[0].(dc.Container)
	if !ok {
		t.Fatalf("expected dc.Container, got %T", components[0])
	}

	// When no avatar is provided, the first component should be a TextDisplay instead of Section
	headerDisplay, ok := container.Components[0].(dc.TextDisplay)
	if !ok {
		t.Fatalf("expected first sub-component to be dc.TextDisplay when no avatar, got %T", container.Components[0])
	}
	if !strings.Contains(headerDisplay.Content, "## FarmerBob") {
		t.Errorf("expected header content to contain title, got: %s", headerDisplay.Content)
	}
}

func TestEbStickyShowAvatar(t *testing.T) {
	testUser := "user-test-avatar"
	// Verify default is false
	defaultVal := farmerstate.GetMiscSettingFlag(testUser, StickySettingEbShowAvatar)
	if defaultVal {
		t.Errorf("expected default show-avatar to be false, got true")
	}

	// Set to true and verify it persists
	farmerstate.SetMiscSettingFlag(testUser, StickySettingEbShowAvatar, true)
	if !farmerstate.GetMiscSettingFlag(testUser, StickySettingEbShowAvatar) {
		t.Errorf("expected show-avatar to be true after setting")
	}

	// Set back to false and verify it persists
	farmerstate.SetMiscSettingFlag(testUser, StickySettingEbShowAvatar, false)
	if farmerstate.GetMiscSettingFlag(testUser, StickySettingEbShowAvatar) {
		t.Errorf("expected show-avatar to be false after setting")
	}
}

func makeTestEncryptedEID(t *testing.T, plainEID string) string {
	t.Helper()
	if config.Key == "" {
		key, err := config.GenerateKey()
		if err != nil {
			t.Fatalf("failed to generate config key: %v", err)
		}
		config.Key = base64.StdEncoding.EncodeToString(key)
	}
	keyBytes, err := base64.StdEncoding.DecodeString(config.Key)
	if err != nil {
		t.Fatalf("failed to decode config.Key: %v", err)
	}
	enc, err := config.EncryptAndCombine(keyBytes, []byte(plainEID))
	if err != nil {
		t.Fatalf("failed to encrypt test eid: %v", err)
	}
	return base64.StdEncoding.EncodeToString(enc)
}

func TestGetUserAltsWithSavedEID(t *testing.T) {
	parentUser := "user-parent-1"
	altWithEID := "alt-user-has-eid"
	altNoEID := "alt-user-no-eid"
	altSnowflake := "123456789012345678"

	// Dummy synthetic IDs adhering to no_player_ids rule
	dummyEID1 := "EI1111111111111111"
	dummyEID2 := "EI2222222222222222"

	encEID1 := makeTestEncryptedEID(t, dummyEID1)
	encEID2 := makeTestEncryptedEID(t, dummyEID2)

	farmerstate.SetMiscSettingString(altWithEID, "AltController", parentUser)
	farmerstate.SetMiscSettingString(altWithEID, "encrypted_ei_id", encEID1)
	farmerstate.SetMiscSettingString(altWithEID, "ei_ign", "AltOne")

	farmerstate.SetMiscSettingString(altNoEID, "AltController", parentUser)
	farmerstate.SetMiscSettingString(altNoEID, "encrypted_ei_id", "")
	farmerstate.SetMiscSettingString(altNoEID, "ei_ign", "AltNoEID")

	farmerstate.SetMiscSettingString(altSnowflake, "AltController", parentUser)
	farmerstate.SetMiscSettingString(altSnowflake, "encrypted_ei_id", encEID2)
	farmerstate.SetMiscSettingString(altSnowflake, "ei_ign", "SnowflakeFarmer")

	alts := GetUserAltsWithSavedEID(parentUser)
	if len(alts) != 2 {
		t.Fatalf("expected 2 alts with saved EID, got %d", len(alts))
	}

	foundNamed := false
	foundSnowflake := false
	for _, a := range alts {
		if a.ID == altWithEID {
			foundNamed = true
			if a.IGN != "AltOne" {
				t.Errorf("expected IGN AltOne, got %q", a.IGN)
			}
			if a.EggIncID != dummyEID1 {
				t.Errorf("expected EggIncID %q, got %q", dummyEID1, a.EggIncID)
			}
			if a.DisplayName != "alt-user-has-eid (AltOne)" {
				t.Errorf("expected display name %q, got %q", "alt-user-has-eid (AltOne)", a.DisplayName)
			}
		}
		if a.ID == altSnowflake {
			foundSnowflake = true
			if a.DisplayName != "SnowflakeFarmer" {
				t.Errorf("expected snowflake display name to use IGN %q, got %q", "SnowflakeFarmer", a.DisplayName)
			}
		}
	}

	if !foundNamed || !foundSnowflake {
		t.Errorf("expected to find both alts, got foundNamed=%v, foundSnowflake=%v", foundNamed, foundSnowflake)
	}

	// User with no alts
	emptyAlts := GetUserAltsWithSavedEID("user-with-no-alts-at-all")
	if len(emptyAlts) != 0 {
		t.Errorf("expected 0 alts for user without alts, got %d", len(emptyAlts))
	}
}

func TestResolveAltSelection(t *testing.T) {
	parentUser := "user-parent-2"
	alt1 := "alt-child-1"
	dummyEID := "EI3333333333333333"
	encEID := makeTestEncryptedEID(t, dummyEID)

	farmerstate.SetMiscSettingString(alt1, "AltController", parentUser)
	farmerstate.SetMiscSettingString(alt1, "encrypted_ei_id", encEID)
	farmerstate.SetMiscSettingString(alt1, "ei_ign", "LittleFarmer")

	// 1. alt parameter empty -> no-op
	targetID, eid, notice := resolveAltSelection(parentUser, "")
	if targetID != parentUser || eid != "" || notice != "" {
		t.Errorf("empty alt: got (%q, %q, %q); want (%q, \"\", \"\")", targetID, eid, notice, parentUser)
	}

	// 2. User with no registered alts used alt parameter
	noAltUser := "user-no-alts"
	targetID, eid, notice = resolveAltSelection(noAltUser, "some-alt")
	if targetID != noAltUser || eid != "" {
		t.Errorf("user with no alts: got targetID %q, eid %q", targetID, eid)
	}
	expectedNoAltNotice := "You have no registered alternate accounts. Showing your EB instead."
	if notice != expectedNoAltNotice {
		t.Errorf("notice = %q; want %q", notice, expectedNoAltNotice)
	}

	// 3. User with alts, but none with saved EID
	userWithAltsNoEID := "user-alts-no-eid"
	altNoEID := "alt-no-eid-child"
	farmerstate.SetMiscSettingString(altNoEID, "AltController", userWithAltsNoEID)
	targetID, eid, notice = resolveAltSelection(userWithAltsNoEID, altNoEID)
	if targetID != userWithAltsNoEID || eid != "" {
		t.Errorf("user with alts without EID: got targetID %q, eid %q", targetID, eid)
	}
	expectedNoEIDNotice := "You have no registered alternate accounts with a saved Egg Inc ID. Showing your EB instead."
	if notice != expectedNoEIDNotice {
		t.Errorf("notice = %q; want %q", notice, expectedNoEIDNotice)
	}

	// 4. Valid alt selection by ID
	targetID, eid, notice = resolveAltSelection(parentUser, alt1)
	if targetID != alt1 || eid != dummyEID || notice != "" {
		t.Errorf("valid alt by ID: got (%q, %q, %q); want (%q, %q, \"\")", targetID, eid, notice, alt1, dummyEID)
	}

	// 5. Valid alt selection by IGN (case-insensitive)
	targetID, eid, notice = resolveAltSelection(parentUser, "littlefarmer")
	if targetID != alt1 || eid != dummyEID || notice != "" {
		t.Errorf("valid alt by IGN: got (%q, %q, %q); want (%q, %q, \"\")", targetID, eid, notice, alt1, dummyEID)
	}

	// 6. Unknown alt selection when user has alts with saved EID
	targetID, eid, notice = resolveAltSelection(parentUser, "unknown-alt")
	if targetID != parentUser || eid != "" {
		t.Errorf("unknown alt: got targetID %q, eid %q", targetID, eid)
	}
	if !strings.Contains(notice, "not found with a saved Egg Inc ID") {
		t.Errorf("unexpected notice for unknown alt: %q", notice)
	}
}

func TestHandleEbAutocomplete(t *testing.T) {
	parentUser := "4" // dctest.AutocompleteEvent uses user ID "4"
	altA := "alt-alpha"
	altB := "alt-beta"
	dummyEID1 := "EI4444444444444444"
	dummyEID2 := "EI5555555555555555"

	enc1 := makeTestEncryptedEID(t, dummyEID1)
	enc2 := makeTestEncryptedEID(t, dummyEID2)

	farmerstate.SetMiscSettingString(altA, "AltController", parentUser)
	farmerstate.SetMiscSettingString(altA, "encrypted_ei_id", enc1)
	farmerstate.SetMiscSettingString(altA, "ei_ign", "AlphaFarm")

	farmerstate.SetMiscSettingString(altB, "AltController", parentUser)
	farmerstate.SetMiscSettingString(altB, "encrypted_ei_id", enc2)
	farmerstate.SetMiscSettingString(altB, "ei_ign", "BetaFarm")

	// Autocomplete for focused option "alt" with empty search
	eventAll := dctest.AutocompleteEvent("eb", "alt", "")
	HandleEbAutocomplete(eventAll)

	// Verify choices were returned
	choices := eventAll.LastChoices
	if len(choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(choices))
	}

	// Autocomplete with filter "alpha"
	eventFilter := dctest.AutocompleteEvent("eb", "alt", "alpha")
	HandleEbAutocomplete(eventFilter)
	filteredChoices := eventFilter.LastChoices
	if len(filteredChoices) != 1 {
		t.Fatalf("expected 1 choice for 'alpha', got %d", len(filteredChoices))
	}
	if filteredChoices[0].Value != altA {
		t.Errorf("expected choice value %q, got %q", altA, filteredChoices[0].Value)
	}

	// Autocomplete for user with no alts
	farmerstate.SetMiscSettingString(altA, "AltController", "other-user")
	farmerstate.SetMiscSettingString(altB, "AltController", "other-user")
	eventEmpty := dctest.AutocompleteEvent("eb", "alt", "")
	HandleEbAutocomplete(eventEmpty)
	if len(eventEmpty.LastChoices) != 0 {
		t.Errorf("expected 0 choices for user without alts, got %d", len(eventEmpty.LastChoices))
	}
}

func TestFindAltDiscordID(t *testing.T) {
	// 1. Direct snowflake ID
	snowflakeID := "123456789012345678"
	if got := findAltDiscordID(snowflakeID); got != snowflakeID {
		t.Errorf("findAltDiscordID(%q) = %q; want %q", snowflakeID, got, snowflakeID)
	}

	// 2. Alt with discord_id misc setting
	altWithSetting := "alt-with-discord-setting"
	targetSnowflake := "987654321098765432"
	farmerstate.SetMiscSettingString(altWithSetting, "discord_id", targetSnowflake)
	if got := findAltDiscordID(altWithSetting); got != targetSnowflake {
		t.Errorf("findAltDiscordID(%q) = %q; want %q", altWithSetting, got, targetSnowflake)
	}

	// 3. Alt with IGN matching a Discord user in farmerstate
	altNamed := "MyAltAccount"
	userSnowflake := "112233445566778899"
	farmerstate.SetMiscSettingString(userSnowflake, "ei_ign", "MyAltIGN")
	farmerstate.SetMiscSettingString(altNamed, "ei_ign", "MyAltIGN")
	if got := findAltDiscordID(altNamed); got != userSnowflake {
		t.Errorf("findAltDiscordID(%q) = %q; want %q", altNamed, got, userSnowflake)
	}

	// 4. Alt with no Discord ID
	altNoDiscord := "alt-no-discord-id"
	if got := findAltDiscordID(altNoDiscord); got != "" {
		t.Errorf("findAltDiscordID(%q) = %q; want empty string", altNoDiscord, got)
	}
}

func TestGetDiscordAvatarURL(t *testing.T) {
	fakeClient := &dctest.FakeClient{
		Users: map[string]*dc.User{
			"user-with-avatar": {
				ID:        "user-with-avatar",
				AvatarURL: "https://cdn.discordapp.com/avatars/user-with-avatar/avatar.png",
			},
			"user-no-avatar": {
				ID: "user-no-avatar",
			},
		},
		Members: map[string]*dc.Member{
			"guild-1:member-with-avatar": {
				UserID:    "member-with-avatar",
				AvatarURL: "https://cdn.discordapp.com/guilds/guild-1/users/member-with-avatar/avatar.png",
			},
		},
	}

	// 1. Nil client
	if got := getDiscordAvatarURL(nil, "guild-1", "user-with-avatar"); got != "" {
		t.Errorf("expected empty string for nil client, got %q", got)
	}

	// 2. Empty discord ID
	if got := getDiscordAvatarURL(fakeClient, "guild-1", ""); got != "" {
		t.Errorf("expected empty string for empty discord ID, got %q", got)
	}

	// 3. Guild member avatar found
	wantGuildAvatar := "https://cdn.discordapp.com/guilds/guild-1/users/member-with-avatar/avatar.png"
	if got := getDiscordAvatarURL(fakeClient, "guild-1", "member-with-avatar"); got != wantGuildAvatar {
		t.Errorf("got %q; want %q", got, wantGuildAvatar)
	}

	// 4. Fallback to user avatar when member not in guild
	wantUserAvatar := "https://cdn.discordapp.com/avatars/user-with-avatar/avatar.png"
	if got := getDiscordAvatarURL(fakeClient, "other-guild", "user-with-avatar"); got != wantUserAvatar {
		t.Errorf("got %q; want %q", got, wantUserAvatar)
	}

	// 5. User with no avatar
	if got := getDiscordAvatarURL(fakeClient, "", "user-no-avatar"); got != "" {
		t.Errorf("expected empty string for user without avatar, got %q", got)
	}
}

func TestBuildEb_UltraBadge(t *testing.T) {
	oldMap := ei.EmoteMap
	ei.EmoteMap = make(map[string]ei.Emotes)
	ei.EmoteMap["ultra"] = ei.Emotes{Name: "ultra", ID: "4001"}
	ei.EmoteMap["pro_permit"] = ei.Emotes{Name: "pro_permit", ID: "3001"}
	ei.EmoteMap["free_permit"] = ei.Emotes{Name: "free_permit", ID: "3002"}
	ei.EmoteMap["badge_nah"] = ei.Emotes{Name: "badge_nah", ID: "111"}
	defer func() {
		ei.EmoteMap = oldMap
	}()

	// 1. Ultra active with Pro Permit
	makerUltra := ei.NewBackupMaker("EI1234567890123456", "UltraFarmer")
	makerUltra.SetPermitLevel(1)
	makerUltra.SetSubscriptionStatus(ei.UserSubscriptionInfo_ACTIVE)
	embedUltra := BuildEbEmbed(makerUltra.GetBackup(), FarmHomeAndVirtue, "user-ultra-pro")
	expectedPrefix := "<:ultra:4001> <:pro_permit:3001>"
	if !strings.HasPrefix(embedUltra.Description, expectedPrefix) {
		t.Errorf("expected Ultra embed to start with %q, got:\n%s", expectedPrefix, embedUltra.Description)
	}

	// Also verify Components header has ultra badge left of permit icon
	compsUltra := BuildEbComponents(makerUltra.GetBackup(), FarmHomeAndVirtue, "user-ultra-pro", "")
	if len(compsUltra) > 0 {
		container := compsUltra[0].(dc.Container)
		headerDisplay := container.Components[0].(dc.TextDisplay)
		if !strings.Contains(headerDisplay.Content, expectedPrefix) {
			t.Errorf("expected Components header to contain %q, got:\n%s", expectedPrefix, headerDisplay.Content)
		}
	}

	// 2. Ultra expired with Pro Permit (should NOT have ultra badge)
	makerExpired := ei.NewBackupMaker("EI1234567890123456", "ExpiredFarmer")
	makerExpired.SetPermitLevel(1)
	makerExpired.SetSubscriptionStatus(ei.UserSubscriptionInfo_EXPIRED)
	embedExpired := BuildEbEmbed(makerExpired.GetBackup(), FarmHomeAndVirtue, "user-expired")
	if strings.Contains(embedExpired.Description, "<:ultra:4001>") {
		t.Errorf("expected expired ultra to not have ultra emoji, got:\n%s", embedExpired.Description)
	}
	if !strings.HasPrefix(embedExpired.Description, "<:pro_permit:3001>") {
		t.Errorf("expected expired ultra embed to start with pro_permit emoji, got:\n%s", embedExpired.Description)
	}

	// 3. Ultra active with Standard Permit
	makerStdUltra := ei.NewBackupMaker("EI1234567890123456", "StdUltraFarmer")
	makerStdUltra.SetPermitLevel(0)
	makerStdUltra.SetSubscriptionStatus(ei.UserSubscriptionInfo_ACTIVE)
	embedStdUltra := BuildEbEmbed(makerStdUltra.GetBackup(), FarmHomeAndVirtue, "user-std-ultra")
	expectedStdPrefix := "<:ultra:4001> <:free_permit:3002>"
	if !strings.HasPrefix(embedStdUltra.Description, expectedStdPrefix) {
		t.Errorf("expected Std Ultra embed to start with %q, got:\n%s", expectedStdPrefix, embedStdUltra.Description)
	}

	// 4. Ultra active with Badges (ultra -> permit -> badges)
	makerWithBadges := ei.NewBackupMaker("EI1234567890123456", "BadgeUltraFarmer")
	makerWithBadges.SetPermitLevel(1)
	makerWithBadges.SetSubscriptionStatus(ei.UserSubscriptionInfo_ACTIVE)
	farmsize := make([]uint64, 19)
	farmsize[18] = 21000000000
	backupWithBadges := makerWithBadges.GetBackup()
	backupWithBadges.Game.MaxFarmSizeReached = farmsize
	embedWithBadges := BuildEbEmbed(backupWithBadges, FarmHome, "user-badges-ultra")
	expectedBadgePrefix := "<:ultra:4001> <:pro_permit:3001> <:badge_nah:111>"
	if !strings.HasPrefix(embedWithBadges.Description, expectedBadgePrefix) {
		t.Errorf("expected Ultra embed with badges to start with %q, got:\n%s", expectedBadgePrefix, embedWithBadges.Description)
	}

	// 5. Fallback: nil SubInfo in backup, but farmerstate.IsUltra is true
	makerNilSub := ei.NewBackupMaker("EI1234567890123456", "FarmerstateUltra")
	makerNilSub.SetPermitLevel(1)
	backupNilSub := makerNilSub.GetBackup()
	backupNilSub.SubInfo = nil
	farmerstate.SetUltra("user-farmerstate-ultra")
	embedNilSub := BuildEbEmbed(backupNilSub, FarmHomeAndVirtue, "user-farmerstate-ultra")
	if !strings.HasPrefix(embedNilSub.Description, expectedPrefix) {
		t.Errorf("expected fallback ultra embed to start with %q, got:\n%s", expectedPrefix, embedNilSub.Description)
	}
}
