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
	if len(cmd.Options) != 1 {
		t.Fatalf("cmd.Options length = %d, want 1", len(cmd.Options))
	}
	opt, ok := cmd.Options[0].(dc.StringOption)
	if !ok {
		t.Fatalf("expected StringOption, got %T", cmd.Options[0])
	}
	if opt.Name != "task" || !opt.Required || !opt.Autocomplete {
		t.Errorf("Option mismatch: name=%q, required=%v, autocomplete=%v", opt.Name, opt.Required, opt.Autocomplete)
	}
}
