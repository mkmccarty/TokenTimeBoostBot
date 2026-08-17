package boost

import (
	"reflect"
	"testing"
)

func TestDeduplicateContributors(t *testing.T) {
	tests := []struct {
		name         string
		resolvedList []resolvedContributor
		storedIgns   map[string]string
		wantList     []resolvedContributor
		wantMissing  []string
	}{
		{
			name: "No duplicates",
			resolvedList: []resolvedContributor{
				{name: "Alice", discordID: "userA", eiID: "EI1"},
				{name: "Bob", discordID: "userB", eiID: "EI2"},
			},
			storedIgns: map[string]string{
				"userA": "Alice",
				"userB": "Bob",
			},
			wantList: []resolvedContributor{
				{name: "Alice", discordID: "userA", eiID: "EI1"},
				{name: "Bob", discordID: "userB", eiID: "EI2"},
			},
			wantMissing: nil,
		},
		{
			name: "Duplicate EID - one exact case-insensitive match",
			resolvedList: []resolvedContributor{
				{name: "ClydeEg", discordID: "userClyde", eiID: "EI_SHARED"},
				{name: "MadSheep", discordID: "userSheep", eiID: "EI_SHARED"},
			},
			storedIgns: map[string]string{
				"userClyde": "ClydeEg",
				"userSheep": "SomeOtherName", // Sheep doesn't match their stored ign
			},
			wantList: []resolvedContributor{
				{name: "ClydeEg", discordID: "userClyde", eiID: "EI_SHARED"},
			},
			wantMissing: []string{"MadSheep"},
		},
		{
			name: "Duplicate EID - case-insensitive match tie breaker (case-sensitive)",
			resolvedList: []resolvedContributor{
				{name: "clydeeg", discordID: "userClyde1", eiID: "EI_SHARED"},
				{name: "ClydeEg", discordID: "userClyde2", eiID: "EI_SHARED"},
			},
			storedIgns: map[string]string{
				"userClyde1": "ClydeEg", // matches case-insensitively but not case-sensitively
				"userClyde2": "ClydeEg", // matches case-sensitively
			},
			wantList: []resolvedContributor{
				{name: "ClydeEg", discordID: "userClyde2", eiID: "EI_SHARED"},
			},
			wantMissing: []string{"clydeeg"},
		},
		{
			name: "Duplicate EID - no matches",
			resolvedList: []resolvedContributor{
				{name: "Alice", discordID: "userA", eiID: "EI_SHARED"},
				{name: "Bob", discordID: "userB", eiID: "EI_SHARED"},
			},
			storedIgns: map[string]string{
				"userA": "Charlie",
				"userB": "David",
			},
			wantList:    nil,
			wantMissing: []string{"Alice", "Bob"}, // Alice is processed first in map order or similar, let's check sorting/order. Wait, map order is non-deterministic, but both should be missing regardless of order!
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			getStoredIgn := func(discordID string) string {
				return tc.storedIgns[discordID]
			}
			gotList, gotMissing := deduplicateContributors(tc.resolvedList, getStoredIgn)

			// Helper to check if elements match regardless of order since maps are non-deterministic.
			// However, for wantList/wantMissing, let's normalize or compare using helper maps.
			gotListMap := make(map[string]resolvedContributor)
			for _, r := range gotList {
				gotListMap[r.name] = r
			}
			wantListMap := make(map[string]resolvedContributor)
			for _, r := range tc.wantList {
				wantListMap[r.name] = r
			}

			if !reflect.DeepEqual(gotListMap, wantListMap) {
				t.Errorf("deduplicateContributors list = %v, want %v", gotList, tc.wantList)
			}

			gotMissingMap := make(map[string]bool)
			for _, m := range gotMissing {
				gotMissingMap[m] = true
			}
			wantMissingMap := make(map[string]bool)
			for _, m := range tc.wantMissing {
				wantMissingMap[m] = true
			}
			if !reflect.DeepEqual(gotMissingMap, wantMissingMap) {
				t.Errorf("deduplicateContributors missing = %v, want %v", gotMissing, tc.wantMissing)
			}
		})
	}
}
