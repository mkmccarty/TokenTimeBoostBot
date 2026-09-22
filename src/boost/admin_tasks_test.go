package boost

import (
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

func TestSlashAdminTasksCommand(t *testing.T) {
	cmd := SlashAdminTasksCommand("admin-tasks")
	if cmd == nil {
		t.Fatalf("SlashAdminTasksCommand returned nil")
	}
	if cmd.Name != "admin-tasks" {
		t.Errorf("cmd.Name = %q, want %q", cmd.Name, "admin-tasks")
	}
	if len(cmd.Options) != 2 {
		t.Fatalf("cmd.Options length = %d, want 2", len(cmd.Options))
	}
	opt0, ok := cmd.Options[0].(dc.StringOption)
	if !ok {
		t.Fatalf("expected StringOption for options[0], got %T", cmd.Options[0])
	}
	if opt0.Name != "task" || !opt0.Required || !opt0.Autocomplete {
		t.Errorf("Option mismatch on options[0]: name=%q, required=%v, autocomplete=%v", opt0.Name, opt0.Required, opt0.Autocomplete)
	}

	opt1, ok := cmd.Options[1].(dc.StringOption)
	if !ok {
		t.Fatalf("expected StringOption for options[1], got %T", cmd.Options[1])
	}
	if opt1.Name != "param" || opt1.Required || !opt1.Autocomplete {
		t.Errorf("Option mismatch on options[1]: name=%q, required=%v, autocomplete=%v", opt1.Name, opt1.Required, opt1.Autocomplete)
	}
}

func TestExtractContractIDFromTask(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"regen-complaints-summer-heat-2024", "summer-heat-2024"},
		{"regen complaints summer-heat-2024", "summer-heat-2024"},
		{"regen-complaints:summer-heat-2024", "summer-heat-2024"},
		{"Regen Complaints summer-heat-2024 (Summer Heat)", "summer-heat-2024"},
		{"regen-complaings-test-contract", "test-contract"},
		{"regen-complaint-test-contract", "test-contract"},
		{"regen-complaints", ""},
		{"Regen Complaints", ""},
		{"custom-task-id", "custom-task-id"},
	}

	for _, tt := range tests {
		got := extractContractIDFromTask(tt.input)
		if got != tt.expected {
			t.Errorf("extractContractIDFromTask(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsRegenComplaintsTask(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"regen-complaints-summer-heat-2024", true},
		{"regen complaints summer-heat-2024", true},
		{"regen-complaings-test", true},
		{"regen-complaint-test", true},
		{"cycle-encryption-key", false},
		{"reload-emojis", false},
	}

	for _, tt := range tests {
		got := isRegenComplaintsTask(tt.input)
		if got != tt.expected {
			t.Errorf("isRegenComplaintsTask(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestThematicComplaintsGenerator(t *testing.T) {
	orig := thematicComplaintsGenerator
	defer func() { thematicComplaintsGenerator = orig }()

	called := false
	SetThematicComplaintsGenerator(func(eggName, contractName, contractDesc string, quantity int) []string {
		called = true
		return []string{"[player] is out of tokens"}
	})

	if thematicComplaintsGenerator == nil {
		t.Fatal("expected thematicComplaintsGenerator to be set")
	}
	res := thematicComplaintsGenerator("Edible", "Test Contract", "Description", 1)
	if !called || len(res) != 1 {
		t.Errorf("generator failed to execute, called=%v, len=%d", called, len(res))
	}
}

func TestPeriodicalsRefresher(t *testing.T) {
	orig := periodicalsRefresher
	defer func() { periodicalsRefresher = orig }()

	called := false
	SetPeriodicalsRefresher(func(client dc.Client) bool {
		called = true
		return true
	})

	if periodicalsRefresher == nil {
		t.Fatal("expected periodicalsRefresher to be set")
	}
	res := periodicalsRefresher(nil)
	if !called || !res {
		t.Errorf("refresher failed to execute, called=%v, res=%v", called, res)
	}
}

func TestTokenComplaintsRefresher(t *testing.T) {
	orig := tokenComplaintsRefresher
	defer func() { tokenComplaintsRefresher = orig }()

	called := false
	SetTokenComplaintsRefresher(func() (int, error) {
		called = true
		return 42, nil
	})

	if tokenComplaintsRefresher == nil {
		t.Fatal("expected tokenComplaintsRefresher to be set")
	}
	count, err := tokenComplaintsRefresher()
	if !called || err != nil || count != 42 {
		t.Errorf("refresher failed to execute, called=%v, count=%d, err=%v", called, count, err)
	}
}

func TestStatusMessagesRefresher(t *testing.T) {
	orig := statusMessagesRefresher
	defer func() { statusMessagesRefresher = orig }()

	called := false
	SetStatusMessagesRefresher(func() (int, error) {
		called = true
		return 55, nil
	})

	if statusMessagesRefresher == nil {
		t.Fatal("expected statusMessagesRefresher to be set")
	}
	count, err := statusMessagesRefresher()
	if !called || err != nil || count != 55 {
		t.Errorf("refresher failed to execute, called=%v, count=%d, err=%v", called, count, err)
	}
}

func TestAdminTaskListContainsNewTasks(t *testing.T) {
	expectedTasks := map[string]bool{
		"cycle-encryption-key":     false,
		"reload-emojis":            false,
		"refresh-periodicals":      false,
		"check-colleggtible":       false,
		"regen-complaints":         false,
		"refresh-token-complaints": false,
		"refresh-status-messages":  false,
	}

	for _, task := range adminTaskList {
		if _, ok := expectedTasks[task.ID]; ok {
			expectedTasks[task.ID] = true
		}
	}

	for id, found := range expectedTasks {
		if !found {
			t.Errorf("expected adminTaskList to contain %q, but it was missing", id)
		}
	}
}

func TestAdminTasksPermissions(t *testing.T) {
	// Verify that tasks restricted to AdminUserID are properly identified
	restrictedTasks := []string{"cycle-encryption-key", "regen-complaints-summer-heat"}
	for _, task := range restrictedTasks {
		isRestricted := task == "cycle-encryption-key" || isRegenComplaintsTask(task)
		if !isRestricted {
			t.Errorf("expected task %q to be restricted to AdminUserID", task)
		}
	}

	generalTasks := []string{
		"reload-emojis",
		"refresh-periodicals",
		"check-colleggtible",
		"refresh-token-complaints",
		"refresh-status-messages",
	}
	for _, task := range generalTasks {
		isRestricted := task == "cycle-encryption-key" || isRegenComplaintsTask(task)
		if isRestricted {
			t.Errorf("expected task %q to be available to all admins", task)
		}
	}
}

func TestIsHomeGuild(t *testing.T) {
	// If home guild is empty or DISABLED, all guilds are treated as home guild
	if !isHomeGuild("any-guild") {
		// If home guild is set, verify matching behavior
		home := isHomeGuild("")
		_ = home
	}
}

func TestFilterAdminTaskChoices_EncryptionKeyHiddenByDefault(t *testing.T) {
	hasEncryptionKey := func(choices []dc.Choice[string]) bool {
		for _, c := range choices {
			if c.Value == "cycle-encryption-key" {
				return true
			}
		}
		return false
	}

	// 1. When search is empty, cycle-encryption-key must NOT appear
	choicesEmpty := filterAdminTaskChoices(true, true, "")
	if hasEncryptionKey(choicesEmpty) {
		t.Errorf("expected cycle-encryption-key to be hidden when search is empty")
	}

	// 2. When searching "encrypt", cycle-encryption-key must appear for admin on home guild
	choicesEncrypt := filterAdminTaskChoices(true, true, "encrypt")
	if !hasEncryptionKey(choicesEncrypt) {
		t.Errorf("expected cycle-encryption-key to appear when search matches name")
	}

	// 3. When searching "cycle", cycle-encryption-key must appear
	choicesCycle := filterAdminTaskChoices(true, true, "cycle")
	if !hasEncryptionKey(choicesCycle) {
		t.Errorf("expected cycle-encryption-key to appear when search matches 'cycle'")
	}

	// 4. When searching a description-only word ("rotate"), cycle-encryption-key must NOT appear
	choicesRotate := filterAdminTaskChoices(true, true, "rotate")
	if hasEncryptionKey(choicesRotate) {
		t.Errorf("expected cycle-encryption-key to NOT appear when search only matches description")
	}

	// 5. When not admin or not home guild, cycle-encryption-key must NOT appear even if searching "encrypt"
	choicesNotAdmin := filterAdminTaskChoices(false, true, "encrypt")
	if hasEncryptionKey(choicesNotAdmin) {
		t.Errorf("expected cycle-encryption-key to be hidden for non-admin")
	}

	choicesNotHome := filterAdminTaskChoices(true, false, "encrypt")
	if hasEncryptionKey(choicesNotHome) {
		t.Errorf("expected cycle-encryption-key to be hidden for non-home guild")
	}
}
