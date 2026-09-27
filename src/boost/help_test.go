package boost

import (
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

func TestGetSlashHelpCommand(t *testing.T) {
	cmd := GetSlashHelpCommand("help")
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	if cmd.Name != "help" {
		t.Errorf("got command name %q, want %q", cmd.Name, "help")
	}

	var foundAttributions bool
	for _, opt := range cmd.Options {
		if boolOpt, ok := opt.(dc.BoolOption); ok && boolOpt.Name == "attributions" {
			foundAttributions = true
			if boolOpt.Required {
				t.Error("expected attributions option to be optional (Required: false)")
			}
			break
		}
	}
	if !foundAttributions {
		t.Error("expected /help to have an optional 'attributions' bool option")
	}
}

func TestGetAttributions(t *testing.T) {
	embed := GetAttributions()
	if embed == nil {
		t.Fatal("expected non-nil embed")
	}

	if embed.Title != "Boost Bot Attributions" {
		t.Errorf("got title %q, want %q", embed.Title, "Boost Bot Attributions")
	}

	allContent := embed.Title + "\n" + embed.Description
	var communityFound, toolsFound, developersFound bool

	for _, field := range embed.Fields {
		allContent += "\n" + field.Name + "\n" + field.Value

		if strings.Contains(field.Name, "Egg Inc Community") || strings.Contains(field.Value, "Egg Inc Community") {
			communityFound = true
		}
		if strings.Contains(field.Value, "carpet-wasmegg") &&
			strings.Contains(field.Value, "staabmia") &&
			strings.Contains(field.Value, "Wonky Projects") {
			toolsFound = true
		}
		if strings.Contains(field.Value, "RAIYC") &&
			strings.Contains(field.Value, "jameswst") &&
			!strings.Contains(field.Value, "mutilis") &&
			strings.Contains(field.Value, "developers (not bots)") {
			developersFound = true
		}
	}

	if !communityFound {
		t.Error("expected embed to thank the Egg Inc Community")
	}
	if !toolsFound {
		t.Error("expected embed to thank carpet-wasmegg, staabmia, and Wonky Projects within the same section")
	}
	if !developersFound {
		t.Error("expected embed to thank developers (not bots) contributing to the BoostBot project")
	}
}
