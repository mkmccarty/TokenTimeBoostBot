package dctest

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// StringOption is one string option on a synthesized command interaction.
type StringOption struct {
	Name  string
	Value string
}

// CommandEvent builds a dc.CommandEvent for a slash command invoked with the
// given string options. It exists so a test can exercise option parsing
// without naming the Discord library itself.
//
// The event is built from Discord's own interaction payload, because that is
// the only way to construct one: the library keeps an interaction's identity
// fields unexported and fills them in while decoding. The event carries no
// gateway, so it can read options but cannot respond.
func CommandEvent(name string, options ...StringOption) *dc.CommandEvent {
	encoded := make([]string, 0, len(options))
	for _, opt := range options {
		value, err := json.Marshal(opt.Value)
		if err != nil {
			panic(fmt.Sprintf("dctest: encoding option %q: %v", opt.Name, err))
		}
		encoded = append(encoded, fmt.Sprintf(`{"name":%q,"type":3,"value":%s}`, opt.Name, value))
	}

	payload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 2,
		"token": "test-token",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": "4", "username": "tester", "discriminator": "0"},
		"data": {"id": "5", "name": %q, "type": 1, "options": [%s]}
	}`, name, strings.Join(encoded, ","))

	event, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		panic(fmt.Sprintf("dctest: building a command event: %v", err))
	}
	return event
}
