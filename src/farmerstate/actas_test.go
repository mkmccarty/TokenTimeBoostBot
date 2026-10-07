package farmerstate

import (
	"testing"
	"time"
)

func TestActAsPendingRegistration(t *testing.T) {
	mainID := "userMain1"
	altID := "userAlt1"
	code := "12345678"

	StorePendingActAsRegistration(mainID, altID, code, 5*time.Minute)

	reg, ok := GetPendingActAsRegistration(mainID)
	if !ok {
		t.Fatalf("expected pending registration to be found")
	}
	if reg.MainUserID != mainID || reg.AltUserID != altID || reg.Code != code {
		t.Errorf("got %+v, mismatch", reg)
	}

	ClearPendingActAsRegistration(mainID)
	_, ok = GetPendingActAsRegistration(mainID)
	if ok {
		t.Errorf("expected pending registration to be cleared")
	}

	// Test expiration
	StorePendingActAsRegistration(mainID, altID, code, -1*time.Minute)
	_, ok = GetPendingActAsRegistration(mainID)
	if ok {
		t.Errorf("expected expired registration not to be found")
	}
}

func TestActAsLinksAndSwitches(t *testing.T) {
	mainID := "userMain2"
	altID := "userAlt2"

	if IsActAsLinked(mainID, altID) {
		t.Errorf("expected not linked yet")
	}

	err := AddActAsLink(mainID, altID)
	if err != nil {
		t.Fatalf("AddActAsLink failed: %v", err)
	}

	if !IsActAsLinked(mainID, altID) {
		t.Errorf("expected to be linked")
	}

	links := GetActAsLinks(mainID)
	if len(links) != 1 || links[0] != altID {
		t.Errorf("got links %v, want [%s]", links, altID)
	}

	chan1 := "channel-1"
	chan2 := "channel-2"
	ephemeralMsgID := "msg-12345"

	// Effective user ID before switch in chan1
	if got := GetEffectiveUserID(mainID, chan1); got != mainID {
		t.Errorf("GetEffectiveUserID() = %s, want %s", got, mainID)
	}

	// Set switch in chan1
	expiresAt := time.Now().Add(1 * time.Hour)
	err = SetActAsSwitch(mainID, chan1, altID, expiresAt)
	if err != nil {
		t.Fatalf("SetActAsSwitch failed: %v", err)
	}

	// Effective user ID in chan1 during active switch
	if got := GetEffectiveUserID(mainID, chan1); got != altID {
		t.Errorf("GetEffectiveUserID(chan1) = %s, want %s", got, altID)
	}

	// Channel isolation: chan2 should NOT be switched
	if got := GetEffectiveUserID(mainID, chan2); got != mainID {
		t.Errorf("GetEffectiveUserID(chan2) = %s, want %s", got, mainID)
	}

	eff, exp, isActAs := GetEffectiveUserIDAndExpiry(mainID, chan1)
	if !isActAs || eff != altID || exp.Unix() != expiresAt.Unix() {
		t.Errorf("got eff=%s exp=%v isActAs=%v; want eff=%s isActAs=true", eff, exp, isActAs, altID)
	}

	// Move switch to chan2 (e.g. when spawning a thread)
	err = MoveActAsSwitch(mainID, chan1, chan2)
	if err != nil {
		t.Fatalf("MoveActAsSwitch failed: %v", err)
	}
	if got := GetEffectiveUserID(mainID, chan1); got != mainID {
		t.Errorf("after move, chan1 should be back to main, got %s", got)
	}
	if got := GetEffectiveUserID(mainID, chan2); got != altID {
		t.Errorf("after move, chan2 should have active switch, got %s", got)
	}

	// Register ephemeral message ID launched under switch to altID
	RegisterEphemeralMessageSwitch(ephemeralMsgID, mainID, altID)

	// Even if chan2 switch is cleared, message ID remembers the switch
	_ = ClearActAsSwitch(mainID, chan2)
	if got := GetEffectiveUserID(mainID, chan2); got != mainID {
		t.Errorf("after ClearActAsSwitch, chan2 should be mainID, got %s", got)
	}
	if got := GetEffectiveUserID(mainID, ephemeralMsgID, chan2); got != altID {
		t.Errorf("ephemeral message ID should remember switch, got %s, want %s", got, altID)
	}

	// Switch expiry test
	expiredTime := time.Now().Add(-1 * time.Minute)
	_ = SetActAsSwitch(mainID, chan1, altID, expiredTime)
	if got := GetEffectiveUserID(mainID, chan1); got != mainID {
		t.Errorf("after expired switch, GetEffectiveUserID() = %s, want %s", got, mainID)
	}

	// Re-activate and test ClearActAsSwitch
	_ = SetActAsSwitch(mainID, chan1, altID, time.Now().Add(1*time.Hour))
	err = ClearActAsSwitch(mainID, chan1)
	if err != nil {
		t.Fatalf("ClearActAsSwitch failed: %v", err)
	}
	if got := GetEffectiveUserID(mainID, chan1); got != mainID {
		t.Errorf("after ClearActAsSwitch, GetEffectiveUserID() = %s, want %s", got, mainID)
	}

	// Re-activate and test RemoveActAsLink removes link and clears switches across all channels
	_ = SetActAsSwitch(mainID, chan1, altID, time.Now().Add(1*time.Hour))
	_ = SetActAsSwitch(mainID, chan2, altID, time.Now().Add(1*time.Hour))
	err = RemoveActAsLink(mainID, altID)
	if err != nil {
		t.Fatalf("RemoveActAsLink failed: %v", err)
	}
	if IsActAsLinked(mainID, altID) {
		t.Errorf("expected link to be removed")
	}
	if got := GetEffectiveUserID(mainID, chan1); got != mainID {
		t.Errorf("after RemoveActAsLink, GetEffectiveUserID(chan1) = %s, want %s", got, mainID)
	}
	if got := GetEffectiveUserID(mainID, chan2); got != mainID {
		t.Errorf("after RemoveActAsLink, GetEffectiveUserID(chan2) = %s, want %s", got, mainID)
	}
}

func TestActAsForeverSwitch(t *testing.T) {
	mainID := "userMainForever"
	altID := "userAltForever"
	chan1 := "channel-forever-1"
	chan2 := "channel-forever-2"

	_ = AddActAsLink(mainID, altID)
	defer func() {
		_ = ClearAllActAsSwitches(mainID)
		_ = RemoveActAsLink(mainID, altID)
	}()

	// Set temporary switch in chan1
	err := SetActAsSwitch(mainID, chan1, altID, time.Now().Add(15*time.Minute))
	if err != nil {
		t.Fatalf("SetActAsSwitch failed: %v", err)
	}

	_, exp1, active1 := GetActAsSwitch(mainID, chan1)
	if !active1 || IsActAsForever(exp1) {
		t.Errorf("expected 15m switch not to be forever")
	}

	// Move to chan2 with forever expiry (e.g. when spawned by /contract)
	err = MoveActAsSwitchWithExpiry(mainID, chan1, chan2, ActAsForeverExpiry)
	if err != nil {
		t.Fatalf("MoveActAsSwitchWithExpiry failed: %v", err)
	}

	// chan1 should now be cleared
	if got := GetEffectiveUserID(mainID, chan1); got != mainID {
		t.Errorf("chan1 should be mainID after move, got %s", got)
	}

	// chan2 should be altID and forever
	altGot, exp2, active2 := GetActAsSwitch(mainID, chan2)
	if !active2 || altGot != altID {
		t.Fatalf("expected chan2 to have active switch for %s, got active=%v alt=%s", altID, active2, altGot)
	}
	if !IsActAsForever(exp2) {
		t.Errorf("expected chan2 switch to be forever, got %v", exp2)
	}

	eff, expEff, isActAs := GetEffectiveUserIDAndExpiry(mainID, chan2)
	if !isActAs || eff != altID || !IsActAsForever(expEff) {
		t.Errorf("GetEffectiveUserIDAndExpiry mismatch: isActAs=%v eff=%s exp=%v", isActAs, eff, expEff)
	}
}
