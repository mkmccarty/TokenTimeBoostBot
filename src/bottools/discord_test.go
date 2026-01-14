package bottools

import (
	"fmt"
	"testing"
	"time"
)

func TestIsValidDiscordID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{
			name:     "valid Discord ID",
			id:       "123456789012345678",
			expected: true,
		},
		{
			name:     "too short (16 digits)",
			id:       "1234567890123456",
			expected: false,
		},
		{
			name:     "too long (21 digits)",
			id:       "123456789012345678901",
			expected: false,
		},
		{
			name:     "non-numeric characters",
			id:       "12345678901234567a",
			expected: false,
		},
		{
			name:     "empty string",
			id:       "",
			expected: false,
		},
		{
			name:     "short name",
			id:       "j2",
			expected: false,
		},
		{
			name:     "valid length but invalid timestamp (before Discord epoch)",
			id:       "1000000000000000",
			expected: false,
		},
		{
			name:     "valid length but timestamp too far in future",
			id:       "9999999999999999999",
			expected: false,
		},
		{
			name:     "minimum valid length (17 digits)",
			id:       "81384788765712384",
			expected: true,
		},
		{
			name:     "maximum valid length (20 digits)",
			id:       "99999999999999999999",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidDiscordID(tt.id)
			if result != tt.expected {
				t.Errorf("IsValidDiscordID(%q) = %v, want %v", tt.id, result, tt.expected)
			}
		})
	}
}

func TestIsValidDiscordIDWithRecentTimestamp(t *testing.T) {
	// Create a snowflake ID with current timestamp
	const discordEpoch int64 = 1420070400000
	now := time.Now().UnixMilli()
	snowflake := (now - discordEpoch) << 22

	id := fmt.Sprintf("%d", snowflake)
	result := IsValidDiscordID(id)

	if !result {
		t.Errorf("IsValidDiscordID with recent timestamp should be valid, got ID: %s", id)
	}
}
