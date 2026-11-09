package events

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestHasExpectedActiveContracts(t *testing.T) {
	tests := []struct {
		name      string
		contracts []ei.EggIncContract
		want      bool
	}{
		{
			name: "returns false below threshold",
			contracts: []ei.EggIncContract{
				{ID: "c1"},
				{ID: "c2"},
				{ID: "c3"},
				{ID: "c4"},
				{ID: "c5"},
			},
			want: false,
		},
		{
			name: "ignores predicted contracts",
			contracts: []ei.EggIncContract{
				{ID: "c1"},
				{ID: "c2"},
				{ID: "c3"},
				{ID: "c4"},
				{ID: "c5"},
				{ID: "predicted-1", Predicted: true},
				{ID: "predicted-2", Predicted: true},
			},
			want: false,
		},
		{
			name: "returns true at threshold",
			contracts: []ei.EggIncContract{
				{ID: "c1"},
				{ID: "c2"},
				{ID: "c3"},
				{ID: "c4"},
				{ID: "c5"},
				{ID: "c6"},
				{ID: "predicted-1", Predicted: true},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasExpectedActiveContracts(tc.contracts); got != tc.want {
				t.Fatalf("hasExpectedActiveContracts() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFindEventsStartedToday(t *testing.T) {
	loc := time.FixedZone("PT", -8*60*60)
	now := time.Date(2026, time.May, 10, 12, 0, 0, 0, loc)

	tests := []struct {
		name               string
		events             []ei.EggEvent
		wantCount          int
		wantLatestEventTyp string
	}{
		{
			name: "returns false when no events started today",
			events: []ei.EggEvent{
				{EventType: "gift-boost", StartTime: time.Date(2026, time.May, 9, 9, 0, 0, 0, loc)},
				{EventType: "earnings-boost", StartTime: time.Date(2026, time.May, 11, 9, 0, 0, 0, loc)},
			},
			wantCount: 0,
		},
		{
			name: "returns all events started today",
			events: []ei.EggEvent{
				{EventType: "gift-boost", StartTime: time.Date(2026, time.May, 10, 8, 0, 0, 0, loc)},
				{EventType: "earnings-boost", StartTime: time.Date(2026, time.May, 10, 9, 0, 0, 0, loc)},
				{EventType: "research-sale", StartTime: time.Date(2026, time.May, 10, 7, 0, 0, 0, loc)},
			},
			wantCount:          3,
			wantLatestEventTyp: "earnings-boost",
		},
		{
			name: "ignores zero start time",
			events: []ei.EggEvent{
				{EventType: "gift-boost", StartTime: time.Time{}},
				{EventType: "research-sale", StartTime: time.Date(2026, time.May, 10, 6, 30, 0, 0, loc)},
			},
			wantCount:          1,
			wantLatestEventTyp: "research-sale",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findEventsStartedToday(tc.events, now, loc)
			if len(got) != tc.wantCount {
				t.Fatalf("findEventsStartedToday() count = %d, want %d", len(got), tc.wantCount)
			}
			if tc.wantCount == 0 {
				return
			}
			latest := got[len(got)-1]
			if latest.EventType != tc.wantLatestEventTyp {
				t.Fatalf("findEventsStartedToday() latest event = %q, want %q", latest.EventType, tc.wantLatestEventTyp)
			}
		})
	}
}

func TestFindOngoingEvents(t *testing.T) {
	loc := time.FixedZone("PT", -8*60*60)
	now := time.Date(2026, time.May, 10, 12, 0, 0, 0, loc)

	events := []ei.EggEvent{
		{
			ID:        "ended",
			EventType: "gift-boost",
			StartTime: time.Date(2026, time.May, 10, 8, 0, 0, 0, loc),
			EndTime:   time.Date(2026, time.May, 10, 11, 0, 0, 0, loc),
		},
		{
			ID:        "active-late-end",
			EventType: "earnings-boost",
			StartTime: time.Date(2026, time.May, 10, 9, 0, 0, 0, loc),
			EndTime:   time.Date(2026, time.May, 10, 16, 0, 0, 0, loc),
		},
		{
			ID:        "not-started",
			EventType: "research-sale",
			StartTime: time.Date(2026, time.May, 10, 13, 0, 0, 0, loc),
			EndTime:   time.Date(2026, time.May, 10, 18, 0, 0, 0, loc),
		},
		{
			ID:        "active-earlier-end",
			EventType: "drone-frenzy",
			StartTime: time.Date(2026, time.May, 9, 23, 0, 0, 0, loc),
			EndTime:   time.Date(2026, time.May, 10, 13, 0, 0, 0, loc),
		},
	}

	got := findOngoingEvents(events, now)
	if len(got) != 2 {
		t.Fatalf("findOngoingEvents() count = %d, want 2", len(got))
	}

	if got[0].ID != "active-earlier-end" {
		t.Fatalf("findOngoingEvents() first ID = %q, want %q", got[0].ID, "active-earlier-end")
	}
	if got[1].ID != "active-late-end" {
		t.Fatalf("findOngoingEvents() second ID = %q, want %q", got[1].ID, "active-late-end")
	}
}

func TestPrepareEggMessageForChannel_WithEmbedLinks(t *testing.T) {
	egg := ei.EggIncCustomEgg{
		ID:                   "test-copper",
		Name:                 "COPPER",
		Value:                10,
		DimensionName:        "Hab Cost",
		DimensionValueString: []string{"95%", "90%", "85%", "80%"},
		IconURL:              "https://example.com/copper.png",
	}

	origMessage := dc.Message{
		Content: "New Custom Egg Detected: COPPER",
		Embeds: []dc.Embed{{
			Title:       egg.Name,
			Description: "95%,90%,85%,80% Hab Cost\nValue: 10",
			Thumbnail:   egg.IconURL,
		}},
	}

	// Permission with EmbedLinks (e.g. Administrator or EmbedLinks bit set)
	perms := dc.Permissions(1 << 14) // permissionEmbedLinks
	msg := prepareEggMessageForChannel(origMessage, egg, perms)

	if len(msg.Embeds) != 1 {
		t.Fatalf("expected 1 embed when EmbedLinks is true, got %d", len(msg.Embeds))
	}
	if msg.Content != origMessage.Content {
		t.Errorf("content = %q, want %q", msg.Content, origMessage.Content)
	}
	if len(msg.Files) != 0 {
		t.Errorf("expected no attached files when EmbedLinks is true, got %d", len(msg.Files))
	}
}

func TestPrepareEggMessageForChannel_WithoutEmbedLinks(t *testing.T) {
	tempDir := t.TempDir()
	origBannerPath := config.BannerPath
	config.BannerPath = tempDir
	defer func() { config.BannerPath = origBannerPath }()

	eggImageContent := []byte("fake-png-image")
	eggFile := filepath.Join(tempDir, "egg_testcopper.png")
	if err := os.WriteFile(eggFile, eggImageContent, 0644); err != nil {
		t.Fatalf("failed to write test egg image: %v", err)
	}

	egg := ei.EggIncCustomEgg{
		ID:                   "test-copper",
		Name:                 "COPPER",
		Value:                10,
		DimensionName:        "Hab Cost",
		DimensionValueString: []string{"95%", "90%", "85%", "80%"},
		IconURL:              "https://example.com/copper.png",
	}

	origMessage := dc.Message{
		Content: "New Custom Egg Detected: COPPER",
		Embeds: []dc.Embed{{
			Title:       egg.Name,
			Description: "95%,90%,85%,80% Hab Cost\nValue: 10",
			Thumbnail:   egg.IconURL,
		}},
	}

	// Permission without EmbedLinks but with AttachFiles
	perms := dc.Permissions(1<<11 | 1<<15) // SendMessages + AttachFiles
	msg := prepareEggMessageForChannel(origMessage, egg, perms)

	if len(msg.Embeds) != 0 {
		t.Fatalf("expected 0 embeds when EmbedLinks is false, got %d", len(msg.Embeds))
	}
	if !strings.Contains(msg.Content, "New Custom Egg Detected: COPPER") {
		t.Errorf("content missing title: %s", msg.Content)
	}
	if !strings.Contains(msg.Content, "95%,90%,85%,80% Hab Cost") {
		t.Errorf("content missing dimension: %s", msg.Content)
	}
	if len(msg.Files) != 1 {
		t.Fatalf("expected 1 file attached, got %d", len(msg.Files))
	}
	if msg.Files[0].Name != "egg_testcopper.png" {
		t.Errorf("filename = %q, want egg_testcopper.png", msg.Files[0].Name)
	}
}
