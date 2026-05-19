package guildstate

import (
	"database/sql"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	db, _ := sql.Open("sqlite", ":memory:")
	_, _ = db.ExecContext(ctx, ddl)
	queries = New(db)
	guildstate = make(map[string]*GuildState)
	os.Exit(m.Run())
}

func TestSetAndGetGuildSettingString(t *testing.T) {
	SetGuildSettingString("guild-1", "admin_logs_channel", "123456789")

	got := GetGuildSettingString("guild-1", "admin_logs_channel")
	if got != "123456789" {
		t.Errorf("GetGuildSettingString() = %q, want %q", got, "123456789")
	}
}

func TestGetGuildSettingStringMissing(t *testing.T) {
	got := GetGuildSettingString("guild-missing", "no_such_key")
	if got != "" {
		t.Errorf("GetGuildSettingString() on missing key = %q, want empty string", got)
	}
}

func TestSetAndGetGuildSettingFlag(t *testing.T) {
	SetGuildSettingFlag("guild-2", "active-contracts-show-completed", true)

	if !GetGuildSettingFlag("guild-2", "active-contracts-show-completed") {
		t.Error("GetGuildSettingFlag() = false, want true")
	}

	SetGuildSettingFlag("guild-2", "active-contracts-show-completed", false)
	if GetGuildSettingFlag("guild-2", "active-contracts-show-completed") {
		t.Error("GetGuildSettingFlag() = true after setting false, want false")
	}
}

func TestCoordinatorAddAndCheck(t *testing.T) {
	if err := AddGuildCoordinator("guild-3", "user-a", "admin-1"); err != nil {
		t.Fatalf("AddGuildCoordinator() error: %v", err)
	}

	if !IsGuildCoordinator("guild-3", "user-a") {
		t.Error("IsGuildCoordinator() = false after add, want true")
	}
}

func TestCoordinatorNotPresent(t *testing.T) {
	if IsGuildCoordinator("guild-3", "user-nobody") {
		t.Error("IsGuildCoordinator() = true for unknown user, want false")
	}
}

func TestCoordinatorRemove(t *testing.T) {
	_ = AddGuildCoordinator("guild-4", "user-b", "admin-1")

	if err := RemoveGuildCoordinator("guild-4", "user-b"); err != nil {
		t.Fatalf("RemoveGuildCoordinator() error: %v", err)
	}

	if IsGuildCoordinator("guild-4", "user-b") {
		t.Error("IsGuildCoordinator() = true after remove, want false")
	}
}

func TestCoordinatorList(t *testing.T) {
	_ = AddGuildCoordinator("guild-5", "user-c", "admin-1")
	_ = AddGuildCoordinator("guild-5", "user-d", "admin-1")

	coords, err := GetCoordinatorList("guild-5")
	if err != nil {
		t.Fatalf("GetCoordinatorList() error: %v", err)
	}

	if len(coords) != 2 {
		t.Fatalf("GetCoordinatorList() returned %d, want 2", len(coords))
	}
}

func TestCoordinatorListEmpty(t *testing.T) {
	coords, err := GetCoordinatorList("guild-empty")
	if err != nil {
		t.Fatalf("GetCoordinatorList() error: %v", err)
	}

	if len(coords) != 0 {
		t.Errorf("GetCoordinatorList() returned %d, want 0", len(coords))
	}
}

func TestCoordinatorDuplicateInsert(t *testing.T) {
	_ = AddGuildCoordinator("guild-6", "user-e", "admin-1")

	if err := AddGuildCoordinator("guild-6", "user-e", "admin-2"); err == nil {
		t.Error("AddGuildCoordinator() expected error on duplicate, got nil")
	}
}
