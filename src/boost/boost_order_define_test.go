package boost

import (
	"os"
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func TestCustomOrderPersistence(t *testing.T) {
	testUserID := "test_user_define_isolated"
	farmerstate.SetMiscSettingString(testUserID, "saved_custom_orders", "")

	// Initial check should be empty
	orders := GetUserCustomOrders(testUserID)
	if len(orders) != 0 {
		t.Fatalf("expected 0 initial orders, got %d", len(orders))
	}

	t1 := CustomBoostOrderTemplate{
		Name:  "Test Order Alpha",
		Lines: []string{"<DEFL_EFFORT[50]>", "<IHR[6%]>", ">TOKENS"},
	}

	SaveUserCustomOrder(testUserID, t1)

	orders = GetUserCustomOrders(testUserID)
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}

	found := FindCustomOrderTemplate(testUserID, "user:Test Order Alpha")
	if found == nil {
		t.Fatalf("expected to find custom order by 'user:' prefix")
	}
	if len(found.Lines) != 3 || found.Lines[0] != "<DEFL_EFFORT[50]>" {
		t.Errorf("unexpected lines in found order: %+v", found.Lines)
	}

	// Test update existing order name
	t1Updated := CustomBoostOrderTemplate{
		Name:  "Test Order Alpha",
		Lines: []string{"<IHR", ">TOKENS"},
	}
	SaveUserCustomOrder(testUserID, t1Updated)
	orders = GetUserCustomOrders(testUserID)
	if len(orders) != 1 {
		t.Errorf("expected count to stay 1 after update, got %d", len(orders))
	}
	found = FindCustomOrderTemplate(testUserID, "Test Order Alpha")
	if found == nil || len(found.Lines) != 2 || found.Lines[0] != "<IHR" {
		t.Errorf("expected updated lines, got %+v", found)
	}

	// Test Global publish with isolated state
	origGlobals := farmerstate.GetMiscSettingString("GLOBAL", "global_custom_orders")
	defer farmerstate.SetMiscSettingString("GLOBAL", "global_custom_orders", origGlobals)
	farmerstate.SetMiscSettingString("GLOBAL", "global_custom_orders", "")

	globalBefore := len(GetGlobalCustomOrders())
	globalName := "Global Order 1"
	PublishGlobalCustomOrder(CustomBoostOrderTemplate{
		Name:  globalName,
		Lines: []string{"<DEFL_EFFORT[40]>", "<IHR"},
	})
	globalAfter := len(GetGlobalCustomOrders())
	if globalAfter != globalBefore+1 {
		t.Errorf("expected global orders to increase by 1, before=%d, after=%d", globalBefore, globalAfter)
	}

	foundGlobal := FindCustomOrderTemplate(testUserID, "global:"+globalName)
	if foundGlobal == nil || !foundGlobal.IsGlobal {
		t.Fatalf("expected to find global order: %+v", foundGlobal)
	}

	// Test Deletion
	if !DeleteUserCustomOrder(testUserID, "Test Order Alpha") {
		t.Errorf("expected DeleteUserCustomOrder to return true")
	}
	if FindCustomOrderTemplate(testUserID, "Test Order Alpha") != nil {
		t.Errorf("expected deleted user order to not be found")
	}
	if DeleteUserCustomOrder(testUserID, "Nonexistent Order") {
		t.Errorf("expected DeleteUserCustomOrder to return false for nonexistent order")
	}

	if !DeleteGlobalCustomOrder(globalName) {
		t.Errorf("expected DeleteGlobalCustomOrder to return true")
	}
	if FindCustomOrderTemplate(testUserID, "global:"+globalName) != nil {
		t.Errorf("expected deleted global order to not be found")
	}
	if len(GetGlobalCustomOrders()) != globalBefore {
		t.Errorf("expected global orders to return to original count %d, got %d", globalBefore, len(GetGlobalCustomOrders()))
	}
}

func TestBuildDefineCustomOrderMessage(t *testing.T) {
	tmpl := CustomBoostOrderTemplate{
		Name:  "Preview Test",
		Lines: []string{"<DEFL_EFFORT[50]>", "<IHR[6%]>", ">TOKENS", "<TE"},
	}

	findButton := func(msg dc.Message, suffix string) *dc.Button {
		for _, comp := range msg.Components {
			if ar, ok := comp.(dc.ActionRow); ok {
				for _, ic := range ar.Components {
					if btn, ok := ic.(dc.Button); ok {
						if strings.HasSuffix(btn.CustomID, suffix) {
							return &btn
						}
					}
				}
			}
		}
		return nil
	}

	// 1. With sample contract (nil contract passed) - SELECT should always be disabled, no DELETE button in preview
	msgSample := BuildDefineCustomOrderMessage(nil, tmpl, "test-session-uuid", "status message", false)
	if len(msgSample.Components) == 0 {
		t.Fatalf("expected components in sample message")
	}
	if len(msgSample.Files) == 0 {
		t.Errorf("expected rendered image file in sample message")
	}
	btnSampleSelect := findButton(msgSample, "#select")
	if btnSampleSelect == nil {
		t.Fatalf("expected SELECT button in sample message")
	}
	if !btnSampleSelect.Disabled {
		t.Errorf("expected SELECT button to be disabled in sample preview, got enabled")
	}
	btnSampleDelete := findButton(msgSample, "#delete")
	if btnSampleDelete != nil {
		t.Errorf("expected no DELETE button in modify preview message")
	}

	msgSampleSaved := BuildDefineCustomOrderMessage(nil, tmpl, "test-session-uuid", "", true)
	btnSampleSavedSelect := findButton(msgSampleSaved, "#select")
	if btnSampleSavedSelect == nil || !btnSampleSavedSelect.Disabled {
		t.Errorf("expected SELECT button to remain disabled in sample preview even if saved")
	}

	// 2. With real contract
	contract := &Contract{
		ContractID:   "test-define-contract",
		CoopID:       "coop-1",
		ContractHash: "hash-123",
		Boosters:     make(map[string]*Booster),
		Order:        []string{"u1", "u2"},
	}
	contract.Boosters["u1"] = &Booster{
		UserID:  "u1",
		Nick:    "Farmer 1",
		IHRRate: 5e9,
	}
	contract.Boosters["u2"] = &Booster{
		UserID:  "u2",
		Nick:    "Farmer 2",
		IHRRate: 10e9,
	}

	// Unsaved custom order on contract: SELECT must be disabled
	msgContractUnsaved := BuildDefineCustomOrderMessage(contract, tmpl, "test-session-uuid", "", false)
	btnContractUnsavedSelect := findButton(msgContractUnsaved, "#select")
	if btnContractUnsavedSelect == nil {
		t.Fatalf("expected SELECT button in contract message")
	}
	if !btnContractUnsavedSelect.Disabled {
		t.Errorf("expected SELECT button to be disabled for unsaved order on contract")
	}

	// Saved custom order on contract: SELECT must be active (enabled)
	msgContractSaved := BuildDefineCustomOrderMessage(contract, tmpl, "test-session-uuid", "", true)
	btnContractSavedSelect := findButton(msgContractSaved, "#select")
	if btnContractSavedSelect == nil {
		t.Fatalf("expected SELECT button in contract message")
	}
	if btnContractSavedSelect.Disabled {
		t.Errorf("expected SELECT button to be enabled for saved order on contract")
	}
}

func TestSlashCustomBoostOrderCommand(t *testing.T) {
	cmd := GetSlashCustomBoostOrderCommand("custom-boost-order")
	if cmd == nil {
		t.Fatalf("expected non-nil command")
	}
	if cmd.Name != "custom-boost-order" {
		t.Errorf("expected command name 'custom-boost-order', got %s", cmd.Name)
	}
	if len(cmd.Options) != 3 {
		t.Fatalf("expected 3 subcommands in command, got %d", len(cmd.Options))
	}
	sub1, ok := cmd.Options[0].(dc.SubCommand)
	if !ok || sub1.Name != "craft" {
		t.Fatalf("expected Option[0] to be dc.SubCommand 'craft', got %T, val=%+v", cmd.Options[0], cmd.Options[0])
	}
	if len(sub1.Options) == 0 {
		t.Fatalf("expected 'order' option under 'craft' subcommand")
	}
	opt1, ok := sub1.Options[0].(dc.StringOption)
	if !ok || opt1.Name != "order" || !opt1.Autocomplete {
		t.Errorf("expected autocomplete enabled for order option in craft, got Name=%s, Autocomplete=%v", opt1.Name, opt1.Autocomplete)
	}

	sub2, ok := cmd.Options[1].(dc.SubCommand)
	if !ok || sub2.Name != "delete" {
		t.Fatalf("expected Option[1] to be dc.SubCommand 'delete', got %T, val=%+v", cmd.Options[1], cmd.Options[1])
	}
	if len(sub2.Options) == 0 {
		t.Fatalf("expected 'order' option under 'delete' subcommand")
	}
	opt2, ok := sub2.Options[0].(dc.StringOption)
	if !ok || opt2.Name != "order" || !opt2.Required || !opt2.Autocomplete {
		t.Errorf("expected required autocomplete order option in delete, got %+v", opt2)
	}

	sub3, ok := cmd.Options[2].(dc.SubCommand)
	if !ok || sub3.Name != "help" {
		t.Fatalf("expected Option[2] to be dc.SubCommand 'help', got %T, val=%+v", cmd.Options[2], cmd.Options[2])
	}

	docData := GetCustomBoostOrderDoc()
	if len(docData) == 0 {
		t.Errorf("expected non-empty doc data from GetCustomBoostOrderDoc")
	}
}

func TestSaveCustomOrderSuggestedName(t *testing.T) {
	tmplEmptyName := CustomBoostOrderTemplate{
		Name:  "",
		Lines: []string{"DEFL_EFFORT", "IHR"},
	}
	suggested := SuggestCustomOrderName(tmplEmptyName.Lines)
	if suggested != "Deflector Effort & IHR" {
		t.Errorf("expected 'Deflector Effort & IHR', got %q", suggested)
	}

	tmplCustomOrder := CustomBoostOrderTemplate{
		Name:  "Custom Order",
		Lines: []string{"T4L_ACTUATOR", "CRAFT(T4_ACTUATOR)"},
	}
	suggested2 := SuggestCustomOrderName(tmplCustomOrder.Lines)
	if suggested2 != "T4L Actuator & T4 Actuator Crafts" {
		t.Errorf("expected 'T4L Actuator & T4 Actuator Crafts', got %q", suggested2)
	}

	tmplExistingName := CustomBoostOrderTemplate{
		Name:  "My Custom Order",
		Lines: []string{"DEFL_EFFORT"},
	}
	nameVal := tmplExistingName.Name
	if strings.TrimSpace(nameVal) == "" || strings.EqualFold(nameVal, "Custom Order") {
		nameVal = SuggestCustomOrderName(tmplExistingName.Lines)
	}
	if nameVal != "My Custom Order" {
		t.Errorf("expected existing name to be preserved, got %q", nameVal)
	}
}

func init() {
	// Initialize farmerstate test dir if needed
	farmerstate.SetMiscSettingString("test_user_define_init", "key", "val")
	if data, err := os.ReadFile("../../doc/CustomBoostOrder.md"); err == nil {
		SetCustomBoostOrderDoc(data)
	}
}
