package boost

import (
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
}

func TestBuildDefineCustomOrderMessage(t *testing.T) {
	tmpl := CustomBoostOrderTemplate{
		Name:  "Preview Test",
		Lines: []string{"<DEFL_EFFORT[50]>", "<IHR[6%]>", ">TOKENS", "<TE"},
	}

	// 1. With sample contract (nil contract passed)
	msgSample := BuildDefineCustomOrderMessage(nil, tmpl, "test-session-uuid", "status message")
	if len(msgSample.Components) == 0 {
		t.Fatalf("expected components in sample message")
	}
	if len(msgSample.Files) == 0 {
		t.Errorf("expected rendered image file in sample message")
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

	msgContract := BuildDefineCustomOrderMessage(contract, tmpl, "test-session-uuid", "")
	if len(msgContract.Components) == 0 {
		t.Fatalf("expected components in contract message")
	}
	if len(msgContract.Files) == 0 {
		t.Errorf("expected rendered image file in contract message")
	}
}

func TestSlashDefineCustomOrderCommand(t *testing.T) {
	cmd := GetSlashDefineCustomOrderCommand("define-custom-order")
	if cmd == nil {
		t.Fatalf("expected non-nil command")
	}
	if cmd.Name != "define-custom-order" {
		t.Errorf("expected command name 'define-custom-order', got %s", cmd.Name)
	}
	if len(cmd.Options) == 0 {
		t.Fatalf("expected 'order' option in command")
	}
	opt, ok := cmd.Options[0].(dc.StringOption)
	if !ok {
		t.Fatalf("expected Option[0] to be dc.StringOption, got %T", cmd.Options[0])
	}
	if opt.Name != "order" || !opt.Autocomplete {
		t.Errorf("expected autocomplete enabled for order option, got Name=%s, Autocomplete=%v", opt.Name, opt.Autocomplete)
	}
}

func init() {
	// Initialize farmerstate test dir if needed
	farmerstate.SetMiscSettingString("test_user_define_init", "key", "val")
}
