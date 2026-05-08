package events

import (
	"testing"

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
