package version

import (
	"strings"
	"testing"
)

func deriveVersionMetadata(ver string) (release, gitHash string) {
	if idx := strings.Index(ver, "-"); idx != -1 {
		release = ver[:idx]
	} else {
		release = ver
	}

	if idx := strings.LastIndex(ver, "-g"); idx != -1 {
		hash := ver[idx+2:]
		if dirtyIdx := strings.Index(hash, "-dirty"); dirtyIdx != -1 {
			hash = hash[:dirtyIdx]
		}
		gitHash = hash
	}
	return release, gitHash
}

func TestDeriveVersionMetadata(t *testing.T) {
	tests := []struct {
		input       string
		wantRelease string
		wantGitHash string
	}{
		{
			input:       "v2026.42-3-g1a2b3c4",
			wantRelease: "v2026.42",
			wantGitHash: "1a2b3c4",
		},
		{
			input:       "v2026.43-0-gefa60477d-dirty",
			wantRelease: "v2026.43",
			wantGitHash: "efa60477d",
		},
		{
			input:       "v2026.42",
			wantRelease: "v2026.42",
			wantGitHash: "",
		},
		{
			input:       "development",
			wantRelease: "development",
			wantGitHash: "",
		},
	}

	for _, tt := range tests {
		rel, hash := deriveVersionMetadata(tt.input)
		if rel != tt.wantRelease {
			t.Errorf("deriveVersionMetadata(%q) release = %q, want %q", tt.input, rel, tt.wantRelease)
		}
		if hash != tt.wantGitHash {
			t.Errorf("deriveVersionMetadata(%q) gitHash = %q, want %q", tt.input, hash, tt.wantGitHash)
		}
	}
}
