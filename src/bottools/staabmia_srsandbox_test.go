package bottools

import (
	"bytes"
	"image/png"
	"math"
	"testing"
)

func TestRunSRSandboxSimulationBasicCoop(t *testing.T) {
	cfg := SRSandboxConfig{
		ContractName:      "Test Speedrun",
		CRTRun:            true,
		TokenToggle:       true,
		GG:                false,
		EarlySwap:         false,
		CxpToggle:         true,
		JoinDelaySeconds:  20,
		MinPerTokenGift:   13,
		DurationSeconds:   7 * 86400,
		TargetEggAmount:   500e15,
		TokenTimerMinutes: 60,
		ModifierType:      SRSandboxModHabSpace,
		ModifierMult:      1.0,
		BTVTarget:         1.875,
		Players: []SRSandboxPlayerInput{
			{Name: "Player 0", Tokens: 5, TE: 50, Colleggtible: true, Creator: true},
			{Name: "Player 1", Tokens: 5, TE: 50, Colleggtible: true},
			{Name: "Player 2", Tokens: 5, TE: 50, Colleggtible: true},
			{Name: "Player 3", Tokens: 5, TE: 50, Colleggtible: true, Sink: true},
		},
	}

	res, tableData, imgBytes, err := GenerateSRSandboxResultsImage(cfg)
	if err != nil {
		t.Fatalf("GenerateSRSandboxResultsImage failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected contract to succeed, got Success=false")
	}
	if res.CompletionTime <= 0 || res.CompletionTime >= cfg.DurationSeconds {
		t.Fatalf("unexpected completion time: %v", res.CompletionTime)
	}
	if res.TotalDeflector != 80 {
		t.Fatalf("expected total deflector 80%%, got %.1f%%", res.TotalDeflector)
	}
	// Exact parity assertions against https://srsandbox-staabmia.netlify.app/scripts.js
	if got := srFormatRateQHr(res.CoopRate); got != "60.190185" {
		t.Errorf("CoopRate = %s, want 60.190185", got)
	}
	if got := FormatSRSandboxDuration(res.CompletionTime); got != "9hrs, 9min, 7s" {
		t.Errorf("CompletionTime = %q, want %q", got, "9hrs, 9min, 7s")
	}
	if res.MaxCS != 91105 {
		t.Errorf("MaxCS = %.0f, want 91105", res.MaxCS)
	}
	if got := srFormatRounded(res.MeanCS, 2); got != "90835.67" {
		t.Errorf("MeanCS = %s, want 90835.67", got)
	}
	if res.UnusedDeflector != 0 {
		t.Errorf("UnusedDeflector = %.0f, want 0", res.UnusedDeflector)
	}
	if len(res.Players) != 4 {
		t.Fatalf("expected 4 player results, got %d", len(res.Players))
	}
	wantCS := []float64{91105, 90837, 90565, 90368}
	wantTTB := []string{"16min, 15s", "32min, 30s", "48min, 45s", "1hr, 20s"}
	wantBT := []string{"17min, 1s", "16min, 37s", "16min, 14s", "15min, 57s"}
	wantTW := []string{"0.663873", "0.666722", "0.669573", "0.671604"}
	for i, p := range res.Players {
		if p.CS != wantCS[i] {
			t.Errorf("player %d CS = %.0f, want %.0f", i, p.CS, wantCS[i])
		}
		if got := FormatSRSandboxDuration(p.TimeToBoost); got != wantTTB[i] {
			t.Errorf("player %d TimeToBoost = %q, want %q", i, got, wantTTB[i])
		}
		if got := FormatSRSandboxDuration(p.BoostingTime); got != wantBT[i] {
			t.Errorf("player %d BoostingTime = %q, want %q", i, got, wantBT[i])
		}
		if got := srFormatRounded(p.Teamwork, 6); got != wantTW[i] {
			t.Errorf("player %d Teamwork = %s, want %s", i, got, wantTW[i])
		}
		if got := srFormatRateQHr(p.LayRate); got != "15.047546" {
			t.Errorf("player %d LayRate = %s, want 15.047546", i, got)
		}
		if got := srFormatRateQHr(p.ShipRate); got != "15.087062" {
			t.Errorf("player %d ShipRate = %s, want 15.087062", i, got)
		}
		if p.NumTach != 5 || p.NumQuant != 5 {
			t.Errorf("player %d stones = %dT/%dQ, want 5T/5Q", i, p.NumTach, p.NumQuant)
		}
	}
	if len(tableData.CoopRows) != 1 || len(tableData.PlayerRows) != 4 {
		t.Fatalf("unexpected table row counts: coop=%d players=%d", len(tableData.CoopRows), len(tableData.PlayerRows))
	}
	if tableData.FooterNote == "" {
		t.Errorf("expected FooterNote citing staabmia site")
	}

	img, err := png.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		t.Fatalf("failed to decode rendered PNG: %v", err)
	}
	if img.Bounds().Dx() < 400 || img.Bounds().Dy() < 150 {
		t.Fatalf("unexpected PNG dimensions: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestRunSRSandboxSimulationWithSIAB(t *testing.T) {
	// Player 0 equips T4L SIAB in Metro slot (index 11) and swaps mid-contract.
	cfg := SRSandboxConfig{
		ContractName:      "SIAB Test",
		CRTRun:            true,
		TokenToggle:       true,
		CxpToggle:         true,
		JoinDelaySeconds:  20,
		MinPerTokenGift:   13,
		DurationSeconds:   7 * 86400,
		TargetEggAmount:   1.5e18,
		TokenTimerMinutes: 60,
		ModifierType:      SRSandboxModHabSpace,
		ModifierMult:      1.0,
		BTVTarget:         1.875,
		Players: []SRSandboxPlayerInput{
			{Name: "Player 0", Tokens: 6, TE: 50, Colleggtible: true, Creator: true, MetroIdx: 11},
			{Name: "Player 1", Tokens: 6, TE: 50, Colleggtible: true},
			{Name: "Player 2", Tokens: 6, TE: 50, Colleggtible: true},
			{Name: "Player 3", Tokens: 6, TE: 50, Colleggtible: true},
		},
	}

	res, tableData, imgBytes, err := GenerateSRSandboxResultsImage(cfg)
	if err != nil {
		t.Fatalf("GenerateSRSandboxResultsImage failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected SIAB contract to succeed")
	}
	if !res.SIABActive {
		t.Fatalf("expected SIABActive=true")
	}
	if tableData.SIABNote == "" {
		t.Fatalf("expected non-empty SIABNote")
	}
	if _, err := png.Decode(bytes.NewReader(imgBytes)); err != nil {
		t.Fatalf("failed to decode SIAB PNG: %v", err)
	}
}

func TestRunSRSandboxSimulationFailure(t *testing.T) {
	cfg := SRSandboxConfig{
		ContractName:      "Impossible",
		CRTRun:            true,
		TokenToggle:       true,
		CxpToggle:         true,
		JoinDelaySeconds:  20,
		MinPerTokenGift:   13,
		DurationSeconds:   3600, // 1 hour
		TargetEggAmount:   1e20, // way too high
		TokenTimerMinutes: 60,
		ModifierType:      SRSandboxModHabSpace,
		ModifierMult:      1.0,
		BTVTarget:         1.875,
		Players: []SRSandboxPlayerInput{
			{Name: "Player 0", Tokens: 5, TE: 50, Colleggtible: true, Creator: true},
		},
	}

	res, tableData, imgBytes, err := GenerateSRSandboxResultsImage(cfg)
	if err != nil {
		t.Fatalf("GenerateSRSandboxResultsImage failed: %v", err)
	}
	if res.Success {
		t.Fatalf("expected contract to fail")
	}
	if tableData.Success {
		t.Fatalf("expected tableData.Success=false")
	}
	if _, err := png.Decode(bytes.NewReader(imgBytes)); err != nil {
		t.Fatalf("failed to decode failure PNG: %v", err)
	}
}

func TestFormatSRSandboxHelpers(t *testing.T) {
	if got := FormatSRSandboxDuration(90061); got != "1day, 1hr, 1min, 1s" {
		t.Errorf("FormatSRSandboxDuration(90061) = %q, want %q", got, "1day, 1hr, 1min, 1s")
	}
	if got := FormatSRSandboxDuration(180122); got != "2days, 2hrs, 2min, 2s" {
		t.Errorf("FormatSRSandboxDuration(180122) = %q, want %q", got, "2days, 2hrs, 2min, 2s")
	}
	if got := FormatSRSandboxComma(14883750000); got != "14,883,750,000" {
		t.Errorf("FormatSRSandboxComma(14883750000) = %q, want %q", got, "14,883,750,000")
	}
	if got := FormatSRSandboxComma(9999); got != "9999" {
		t.Errorf("FormatSRSandboxComma(9999) = %q, want %q", got, "9999")
	}
	if got := srCalcSinkCR(4, 7); got != 21 {
		t.Errorf("srCalcSinkCR(4, 7) = %d, want 21", got)
	}
	if got := math.Round(srGetTeamwork(1.875, 4, 7, 20, 10, true)*1e6) / 1e6; got != 0.756579 {
		t.Errorf("srGetTeamwork new2p0 = %v, want 0.756579", got)
	}
	if got := UnescapeDiscordMarkdown(`Is\_a\_plant\*`); got != `Is_a_plant*` {
		t.Errorf("UnescapeDiscordMarkdown = %q, want %q", got, `Is_a_plant*`)
	}
}
