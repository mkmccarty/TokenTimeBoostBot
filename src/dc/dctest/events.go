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

// ComponentSelectEvent builds a dc.ComponentEvent for a select menu interaction
// with the given customID and selected values.
func ComponentSelectEvent(customID string, values ...string) *dc.ComponentEvent {
	return ComponentSelectEventWithUser(customID, "4", values...)
}

// ComponentSelectEventWithUser builds a dc.ComponentEvent for a select menu interaction
// with the given customID, userID and selected values.
func ComponentSelectEventWithUser(customID, userID string, values ...string) *dc.ComponentEvent {
	encodedValues := make([]string, 0, len(values))
	for _, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			panic(fmt.Sprintf("dctest: encoding value %q: %v", v, err))
		}
		encodedValues = append(encodedValues, string(b))
	}

	payload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 3,
		"token": "test-token",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"message": {"id": "700", "channel_id": "3", "content": "", "timestamp": "2026-01-01T00:00:00Z",
			"author": {"id": "1", "username": "bot", "discriminator": "0"}},
		"data": {"custom_id": %q, "component_type": 3, "values": [%s]}
	}`, userID, customID, strings.Join(encodedValues, ","))

	event, err := dc.NewComponentEventFromPayload([]byte(payload))
	if err != nil {
		panic(fmt.Sprintf("dctest: building a component event: %v", err))
	}
	return event
}

// ComponentButtonEvent builds a dc.ComponentEvent for a button click interaction
// with the given customID.
func ComponentButtonEvent(customID string) *dc.ComponentEvent {
	return ComponentButtonEventWithUser(customID, "4")
}

// ComponentButtonEventWithUser builds a dc.ComponentEvent for a button click interaction
// with the given customID and userID.
func ComponentButtonEventWithUser(customID, userID string) *dc.ComponentEvent {
	payload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 3,
		"token": "test-token",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"message": {"id": "700", "channel_id": "3", "content": "", "timestamp": "2026-01-01T00:00:00Z",
			"author": {"id": "1", "username": "bot", "discriminator": "0"}},
		"data": {"custom_id": %q, "component_type": 2}
	}`, userID, customID)

	event, err := dc.NewComponentEventFromPayload([]byte(payload))
	if err != nil {
		panic(fmt.Sprintf("dctest: building a button event: %v", err))
	}
	return event
}

// AutocompleteEvent builds a dc.AutocompleteEvent for an autocomplete interaction.
func AutocompleteEvent(commandName, optionName, optionValue string) *dc.AutocompleteEvent {
	return AutocompleteEventWithUser(commandName, optionName, optionValue, "4")
}

// AutocompleteEventWithUser builds a dc.AutocompleteEvent for an autocomplete interaction with a specific user.
func AutocompleteEventWithUser(commandName, optionName, optionValue, userID string) *dc.AutocompleteEvent {
	payload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 4,
		"token": "test-token",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {
			"id": "5",
			"name": %q,
			"type": 1,
			"options": [{"name": %q, "type": 3, "value": %q, "focused": true}]
		}
	}`, userID, commandName, optionName, optionValue)

	event, err := dc.NewAutocompleteEventFromPayload([]byte(payload))
	if err != nil {
		panic(fmt.Sprintf("dctest: building an autocomplete event: %v", err))
	}
	return event
}

// SubcommandAutocompleteEvent builds a dc.AutocompleteEvent for a subcommand option.
func SubcommandAutocompleteEvent(commandName, subcommandName, optionName, optionValue, userID string) *dc.AutocompleteEvent {
	payload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 4,
		"token": "test-token",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {
			"id": "5",
			"name": %q,
			"type": 1,
			"options": [{
				"name": %q,
				"type": 1,
				"options": [{"name": %q, "type": 3, "value": %q, "focused": true}]
			}]
		}
	}`, userID, commandName, subcommandName, optionName, optionValue)

	event, err := dc.NewAutocompleteEventFromPayload([]byte(payload))
	if err != nil {
		panic(fmt.Sprintf("dctest: building a subcommand autocomplete event: %v", err))
	}
	return event
}

// SubcommandEventWithUser builds a dc.CommandEvent with a subcommand.
func SubcommandEventWithUser(name, subcommand, userID string, options ...StringOption) *dc.CommandEvent {
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
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {
			"id": "5",
			"name": %q,
			"type": 1,
			"options": [{
				"name": %q,
				"type": 1,
				"options": [%s]
			}]
		}
	}`, userID, name, subcommand, strings.Join(encoded, ","))

	event, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		panic(fmt.Sprintf("dctest: building a subcommand command event: %v", err))
	}
	return event
}

// ModalSubmitEventWithUser builds a dc.ModalEvent for testing modal submit handlers.
func ModalSubmitEventWithUser(customID, userID, inputID, value string) *dc.ModalEvent {
	payload := fmt.Sprintf(`{
		"id": "1",
		"application_id": "2",
		"type": 5,
		"token": "test-token",
		"version": 1,
		"channel": {"id": "3", "type": 0},
		"user": {"id": %q, "username": "tester", "discriminator": "0"},
		"data": {
			"custom_id": %q,
			"components": [
				{"type": 18, "label": "Field", "component": {"type": 4, "custom_id": %q, "value": %q}}
			]
		}
	}`, userID, customID, inputID, value)

	event, err := dc.NewModalEventFromPayload([]byte(payload))
	if err != nil {
		panic(fmt.Sprintf("dctest: building a modal submit event: %v", err))
	}
	return event
}
