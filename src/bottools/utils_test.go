package bottools

import (
	"testing"
)

func TestSanitizeStringDuration(t *testing.T) {

	if SanitizeStringDuration("1h30m15s") != "1h30m15s" {
		t.Errorf("SanitizeStringDuration() = %v, want %v", SanitizeStringDuration("1h30m15s"), "1h30m15s")
	}

	if SanitizeStringDuration("1h 30m 15s") != "1h30m15s" {
		t.Errorf("SanitizeStringDuration() = %v, want %v", SanitizeStringDuration("1h 30m 15s"), "1h30m15s")
	}

	if SanitizeStringDuration("1h 15s 30m") != "1h30m15s" {
		t.Errorf("SanitizeStringDuration() = %v, want %v", SanitizeStringDuration("1h 15s 30m"), "1h30m15s")
	}

	if SanitizeStringDuration("1h30m15s, 2h34s") != "1h30m15s" {
		t.Errorf("SanitizeStringDuration() = %v, want %v", SanitizeStringDuration("1h30m15s, 2h34s"), "1h30m15s")
	}

	if SanitizeStringDuration("5d1h30m15s, 2h34s") != "5d1h30m15s" {
		t.Errorf("SanitizeStringDuration() = %v, want %v", SanitizeStringDuration("5d1h30m15s, 2h34s"), "5d1h30m15s")
	}
}
