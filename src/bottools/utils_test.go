package bottools

import (
	"testing"
	"time"
)

func TestFmtDuration(t *testing.T) {
	now := time.Now()

	later := now.Add(1 * time.Hour)
	if FmtDuration(later.Sub(now)) != "1h0m" {
		t.Errorf("FmtDuration() = %v, want %v", FmtDuration(later.Sub(now)), "1h0m")
	}

	later = now.Add(1 * time.Hour).Add(15 * time.Minute)
	if FmtDuration(later.Sub(now)) != "1h15m" {
		t.Errorf("FmtDuration() = %v, want %v", FmtDuration(later.Sub(now)), "1h15m")
	}

	later = now.Add(72520 * time.Minute)
	if FmtDuration(later.Sub(now)) != "50d8h40m" {
		t.Errorf("FmtDuration() = %v, want %v", FmtDuration(later.Sub(now)), "50d8h40m")
	}
}

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

func FuzzSanitizeStringDuration(f *testing.F) {
	f.Add("1h30m15s")
	f.Add("1h 30m 15s")
	f.Add("1h 15s 30m")
	f.Add("1h30m15s, 2h34s")
	f.Add("5d1h30m15s, 2h34s")
	f.Fuzz(func(t *testing.T, s string) {
		result := SanitizeStringDuration(s)
		if len(result) > 0 {
			t.Logf("SanitizeStringDuration(%s) = %s", s, result)
		}
	})

}
