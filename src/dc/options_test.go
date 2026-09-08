package dc

import "testing"

// OptionValues exists for handlers that interrupt themselves and resume on a
// later interaction: ask for a modal, then finish the original work once the
// modal comes back. The second interaction carries none of the first one's
// options, so the handler stashes a snapshot and reads from that.

func TestOptionValuesReadsEveryKind(t *testing.T) {
	options := commandEventFrom(t, `{
		"id": "100", "application_id": "200", "type": 2, "token": "tok", "version": 1,
		`+testChannel+`, `+testMember+`, `+testGuild+`,
		"data": {"id": "1", "name": "mixed", "type": 1, "options": [
			{"name": "text", "type": 3, "value": "hello"},
			{"name": "count", "type": 4, "value": 7},
			{"name": "ratio", "type": 10, "value": 1.5},
			{"name": "flag", "type": 5, "value": true},
			{"name": "who", "type": 6, "value": "777"},
			{"name": "file", "type": 11, "value": "888"}
		], "resolved": {
			"users": {"777": {"id": "777", "username": "sink", "discriminator": "0"}},
			"attachments": {"888": {"id": "888", "filename": "mint.csv", "size": 42,
				"url": "https://cdn.example/mint.csv", "proxy_url": "https://proxy.example/mint.csv"}}
		}}
	}`).Options()

	if got, ok := options.String("text"); !ok || got != "hello" {
		t.Errorf("String = %q, %v", got, ok)
	}
	if got, ok := options.Int("count"); !ok || got != 7 {
		t.Errorf("Int = %d, %v", got, ok)
	}
	if got, ok := options.Uint("count"); !ok || got != 7 {
		t.Errorf("Uint = %d, %v", got, ok)
	}
	if got, ok := options.Float("ratio"); !ok || got != 1.5 {
		t.Errorf("Float = %v, %v", got, ok)
	}
	if got, ok := options.Bool("flag"); !ok || !got {
		t.Errorf("Bool = %v, %v", got, ok)
	}
	if user, ok := options.User("who"); !ok || user.Username != "sink" {
		t.Errorf("User = %+v, %v", user, ok)
	}
	if attachment, ok := options.Attachment("file"); !ok || attachment.Filename != "mint.csv" {
		t.Errorf("Attachment = %+v, %v", attachment, ok)
	}
}

func TestOptionValuesReportsMissingOptions(t *testing.T) {
	options := commandEventFrom(t, commandPayload("contract", `[{"name": "text", "type": 3, "value": "x"}]`)).Options()

	if _, ok := options.String("absent"); ok {
		t.Error("a missing string reported as present")
	}
	if _, ok := options.Int("absent"); ok {
		t.Error("a missing int reported as present")
	}
	if _, ok := options.Bool("absent"); ok {
		t.Error("a missing bool reported as present")
	}
	if _, ok := options.User("absent"); ok {
		t.Error("a missing user reported as present")
	}
	// Reading an option as the wrong type is the same as it not being there.
	if _, ok := options.Int("text"); ok {
		t.Error("a string option read as an int")
	}
}

// The snapshot has to outlive the event it came from, because the handler
// reads it on a later interaction entirely.
func TestOptionValuesSurvivesTheEvent(t *testing.T) {
	options := commandEventFrom(t, commandPayload("rerun-eval", `[{"name": "mode", "type": 3, "value": "predictions"}]`)).Options()

	if got, ok := options.String("mode"); !ok || got != "predictions" {
		t.Errorf("String = %q, %v", got, ok)
	}
}

func TestOptionValuesTraversesSubcommands(t *testing.T) {
	options := commandEventFrom(t, commandPayload("admin", `[
		{"name": "contract", "type": 2, "options": [
			{"name": "reload", "type": 1, "options": [
				{"name": "force", "type": 5, "value": true}
			]}
		]}
	]`)).Options()

	if path := options.SubcommandPath(); len(path) != 2 || path[0] != "contract" || path[1] != "reload" {
		t.Fatalf("subcommand path = %v", path)
	}
	if name, ok := options.Subcommand(); !ok || name != "contract" {
		t.Errorf("Subcommand = %q, %v", name, ok)
	}
	if got, ok := options.Bool("contract-reload-force"); !ok || !got {
		t.Errorf("path-joined lookup failed: %v, %v", got, ok)
	}
}
