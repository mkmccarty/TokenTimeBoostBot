package events

import (
	"testing"
	"time"

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

func TestFindLatestEventStartedToday(t *testing.T) {
	loc := time.FixedZone("PT", -8*60*60)
	now := time.Date(2026, time.May, 10, 12, 0, 0, 0, loc)

	tests := []struct {
		name          string
		events        []ei.EggEvent
		wantFound     bool
		wantEventType string
	}{
		{
			name: "returns false when no events started today",
			events: []ei.EggEvent{
				{EventType: "gift-boost", StartTime: time.Date(2026, time.May, 9, 9, 0, 0, 0, loc)},
				{EventType: "earnings-boost", StartTime: time.Date(2026, time.May, 11, 9, 0, 0, 0, loc)},
			},
			wantFound: false,
		},
		{
			name: "returns latest event among events started today",
			events: []ei.EggEvent{
				{EventType: "gift-boost", StartTime: time.Date(2026, time.May, 10, 8, 0, 0, 0, loc)},
				{EventType: "earnings-boost", StartTime: time.Date(2026, time.May, 10, 9, 0, 0, 0, loc)},
				{EventType: "research-sale", StartTime: time.Date(2026, time.May, 10, 7, 0, 0, 0, loc)},
			},
			wantFound:     true,
			wantEventType: "earnings-boost",
		},
		{
			name: "ignores zero start time",
			events: []ei.EggEvent{
				{EventType: "gift-boost", StartTime: time.Time{}},
				{EventType: "research-sale", StartTime: time.Date(2026, time.May, 10, 6, 30, 0, 0, loc)},
			},
			wantFound:     true,
			wantEventType: "research-sale",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := findLatestEventStartedToday(tc.events, now, loc)
			if found != tc.wantFound {
				t.Fatalf("findLatestEventStartedToday() found = %v, want %v", found, tc.wantFound)
			}
			if !tc.wantFound {
				return
			}
			if got.EventType != tc.wantEventType {
				t.Fatalf("findLatestEventStartedToday() event = %q, want %q", got.EventType, tc.wantEventType)
			}
		})
	}
}
