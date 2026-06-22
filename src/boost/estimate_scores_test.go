package boost

import (
	"strings"
	"testing"
)

func TestRenderScoreTableANSINormalizesPUAPlayerNames(t *testing.T) {
	rows := []srRow{{
		name: "Chipmunk \ue10c",
		cxp:  123,
		btv:  456,
	}}

	out := RenderScoreTableANSI(rows, false, false)
	if !strings.Contains(out, "Chipmunk 👽") {
		t.Fatalf("expected rendered table to contain normalized emoji name, got %q", out)
	}
	if strings.Contains(out, "\ue10c") {
		t.Fatalf("expected rendered table to not contain raw PUA character, got %q", out)
	}
}