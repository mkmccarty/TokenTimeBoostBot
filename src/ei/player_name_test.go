package ei

import "testing"

func TestNormalizePlayerNameForDisplay(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "replaces apple private-use alien",
			in:   "Chipmunk \ue10c",
			want: "Chipmunk 👽",
		},
		{
			name: "leaves normal text unchanged",
			in:   "Plain Name",
			want: "Plain Name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizePlayerNameForDisplay(tc.in); got != tc.want {
				t.Fatalf("NormalizePlayerNameForDisplay(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
