package watch

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func setupContractCompletionTestEnv(t *testing.T) {
	t.Helper()

	origFile := contractCompletionWatchesFile
	contractCompletionWatchesFile = filepath.Join(t.TempDir(), "contract_completion_watches.json")

	origFetch := fetchContractCoopStatus
	origFetchUncached := fetchContractCoopStatusUncached

	t.Cleanup(func() {
		contractCompletionMu.Lock()
		for k, tm := range contractCompletionTimers {
			tm.Stop()
			delete(contractCompletionTimers, k)
		}
		contractCompletionMu.Unlock()

		contractCompletionWatchesFile = origFile
		fetchContractCoopStatus = origFetch
		fetchContractCoopStatusUncached = origFetchUncached
	})
}

func TestContractCompletionPromptFlowAndClear(t *testing.T) {
	setupContractCompletionTestEnv(t)

	userID := "100000000000000201"
	now := time.Now()
	toggleEv := dctest.ComponentButtonEventWithUser("watch-ccomp-toggle#test-contract#test-coop", userID)
	ackedBeforeFetch := false
	fetchContractCoopStatus = func(contractID, coopID, eeidOverride string) (*ei.ContractCoopStatusResponse, time.Time, string, error) {
		if toggleEv.LastResponse != nil && toggleEv.LastResponse.Ephemeral {
			ackedBeforeFetch = true
		}
		return makeTestCoopStatus(nil, 12*3600, false), now, "", nil
	}

	// 1. Toggle immediately acks ephemerally, then updates with offset menu (5 options) + 2 quiet menus + Confirm/Cancel
	HandleContractCompletionToggle(toggleEv)
	if !ackedBeforeFetch {
		t.Fatalf("expected immediate ephemeral ack before fetchContractCoopStatus was called")
	}
	if toggleEv.LastResponse == nil || !toggleEv.LastResponse.Ephemeral {
		t.Fatalf("expected ephemeral response from HandleContractCompletionToggle")
	}
	if len(toggleEv.LastResponse.Components) != 5 {
		t.Fatalf("expected 5 layout components, got %d", len(toggleEv.LastResponse.Components))
	}

	offsetRow := toggleEv.LastResponse.Components[1].(dc.ActionRow)
	offsetMenu := offsetRow.Components[0].(dc.SelectMenu)
	if len(offsetMenu.Options) != 5 {
		t.Fatalf("expected 5 offset options (5m, 10m, 15m, 30m, 1h), got %d", len(offsetMenu.Options))
	}
	wantVals := []string{"300", "600", "900", "1800", "3600"}
	for i, w := range wantVals {
		if offsetMenu.Options[i].Value != w {
			t.Errorf("offset option %d value = %q, want %q", i, offsetMenu.Options[i].Value, w)
		}
	}

	// 2. Select 15m offset (900s)
	offsetEv := dctest.ComponentSelectEventWithUser(offsetMenu.CustomID, userID, "900")
	HandleContractCompletionSelect(offsetEv)
	if offsetEv.LastResponse == nil {
		t.Fatalf("expected update after selecting offset")
	}

	// 3. Confirm
	btnRow := toggleEv.LastResponse.Components[4].(dc.ActionRow)
	confirmBtn := btnRow.Components[0].(dc.Button)
	confirmEv := dctest.ComponentButtonEventWithUser(confirmBtn.CustomID, userID)

	client := dctest.New()
	HandleContractCompletionConfirm(client, confirmEv)
	if confirmEv.LastResponse == nil {
		t.Fatalf("expected update after confirm")
	}
	lastTD := confirmEv.LastResponse.Components[len(confirmEv.LastResponse.Components)-1].(dc.TextDisplay)
	if !strings.Contains(lastTD.Content, "**15m** before completion") {
		t.Errorf("expected confirmation text to mention 15m offset, got: %s", lastTD.Content)
	}

	// 4. Verify persisted watch
	contractCompletionMu.Lock()
	watches := loadContractCompletionWatchesLocked()
	contractCompletionMu.Unlock()
	if len(watches) != 1 || watches[0].OffsetSeconds != 900 {
		t.Fatalf("unexpected persisted watches: %+v", watches)
	}

	// 5. Clicking Completion Watch again prompts to clear the existing watch
	toggleAgainEv := dctest.ComponentButtonEventWithUser("watch-ccomp-toggle#test-contract#test-coop", userID)
	HandleContractCompletionToggle(toggleAgainEv)
	if toggleAgainEv.LastResponse == nil || len(toggleAgainEv.LastResponse.Components) != 2 {
		t.Fatalf("expected clear prompt with 2 components, got %+v", toggleAgainEv.LastResponse)
	}
	clearRow := toggleAgainEv.LastResponse.Components[1].(dc.ActionRow)
	clearBtn := clearRow.Components[0].(dc.Button)
	clearEv := dctest.ComponentButtonEventWithUser(clearBtn.CustomID, userID)
	HandleContractCompletionClear(clearEv)

	contractCompletionMu.Lock()
	watchesAfterClear := loadContractCompletionWatchesLocked()
	contractCompletionMu.Unlock()
	if len(watchesAfterClear) != 0 {
		t.Fatalf("expected watches to be cleared, got %+v", watchesAfterClear)
	}
}

func TestContractCompletionNotifyDMAndCleanup(t *testing.T) {
	setupContractCompletionTestEnv(t)

	userID := "100000000000000202"
	now := time.Now()
	rec := contractCompletionWatch{
		UserID:        userID,
		NotifyUserID:  userID,
		ChannelID:     "chan-1",
		ContractID:    "test-contract",
		CoopID:        "test-coop",
		OffsetSeconds: 1800,
		NextCheckAt:   now.Unix(),
	}
	upsertContractCompletionWatch(rec)

	client := dctest.New()
	contractCompletionMu.Lock()
	contractCompletionClient = client
	contractCompletionMu.Unlock()

	fetchContractCoopStatusUncached = func(contractID, coopID, eeidOverride string) (*ei.ContractCoopStatusResponse, time.Time, string, error) {
		return makeTestCoopStatus(nil, 900, false), now, "", nil // 15m left, inside 30m offset
	}

	checkContractCompletionWatch(contractCompletionKey(userID, "test-contract", "test-coop"))

	var sentDM bool
	for _, call := range client.Calls {
		if call.Method == "SendMessage" {
			sentDM = true
			if len(call.Args) < 2 || !strings.Contains(call.Args[1], "estimated to complete") {
				t.Errorf("unexpected DM content in call args: %+v", call.Args)
			}
			if len(call.Args) < 2 || !strings.Contains(call.Args[1], "<#chan-1>") {
				t.Errorf("expected DM content to reference <#chan-1>, got: %+v", call.Args)
			}
		}
	}
	if !sentDM {
		t.Fatalf("expected SendMessage DM call, got calls: %+v", client.Calls)
	}

	contractCompletionMu.Lock()
	remaining := loadContractCompletionWatchesLocked()
	contractCompletionMu.Unlock()
	if len(remaining) != 0 {
		t.Fatalf("expected watch to be removed after notifying, got %+v", remaining)
	}
}

func TestSendWatchNotificationDM_HasDismissButton(t *testing.T) {
	client := dctest.New()
	if err := sendWatchNotificationDM(client, "user-1", "chan-fallback", "test notification"); err != nil {
		t.Fatalf("sendWatchNotificationDM failed: %v", err)
	}
	if len(client.SentMessages) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(client.SentMessages))
	}
	msg := client.SentMessages[0]
	if !msg.ComponentsV1 {
		t.Errorf("expected ComponentsV1 to be true")
	}
	if len(msg.Components) != 1 {
		t.Fatalf("expected 1 layout component, got %d", len(msg.Components))
	}
	row, ok := msg.Components[0].(dc.ActionRow)
	if !ok || len(row.Components) != 1 {
		t.Fatalf("expected ActionRow with 1 button, got %#v", msg.Components[0])
	}
	btn, ok := row.Components[0].(dc.Button)
	if !ok {
		t.Fatalf("expected dc.Button, got %#v", row.Components[0])
	}
	if btn.CustomID != "watch-dismiss" || btn.Label != "Dismiss" {
		t.Errorf("unexpected dismiss button: %+v", btn)
	}
}
