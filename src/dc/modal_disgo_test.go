package dc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/disgoorg/disgo/discord"
)

// realModal is the shape this bot actually submits: the Egg Inc ID form, which
// is the only modal with more than one field.
func realModal() Modal {
	return Modal{
		CustomID: "m_eggid#register",
		Title:    "BoostBot needs your Egg Inc ID",
		Inputs: []TextInput{
			{
				CustomID:    "egginc-id",
				Label:       "Egg Inc ID (EI+16 digits)",
				Style:       TextInputStyleShort,
				Placeholder: "EI0000000000000000",
				MaxLength:   18,
				Required:    true,
			},
			{
				CustomID:    "confirm",
				Label:       "Save or Forget this ID after this session?",
				Style:       TextInputStyleShort,
				Placeholder: "save or forget",
				Value:       "save",
				MaxLength:   6,
				Required:    true,
			},
		},
	}
}

func TestModalToDisgo(t *testing.T) {
	modal := realModal().toDisgo()

	if modal.CustomID != "m_eggid#register" {
		t.Errorf("custom id = %q", modal.CustomID)
	}
	if modal.Title != "BoostBot needs your Egg Inc ID" {
		t.Errorf("title = %q", modal.Title)
	}
	if len(modal.Components) != 2 {
		t.Fatalf("want 2 fields, got %d", len(modal.Components))
	}

	label, ok := modal.Components[0].(discord.LabelComponent)
	if !ok {
		t.Fatalf("field 0 is %T, want discord.LabelComponent", modal.Components[0])
	}
	if label.Label != "Egg Inc ID (EI+16 digits)" {
		t.Errorf("label = %q", label.Label)
	}
	input, ok := label.Component.(discord.TextInputComponent)
	if !ok {
		t.Fatalf("field 0 holds %T, want discord.TextInputComponent", label.Component)
	}
	if input.CustomID != "egginc-id" {
		t.Errorf("input custom id = %q", input.CustomID)
	}
	if input.Style != discord.TextInputStyleShort {
		t.Errorf("style = %d, want %d", input.Style, discord.TextInputStyleShort)
	}
	if input.Placeholder != "EI0000000000000000" || input.MaxLength != 18 || !input.Required {
		t.Errorf("input fields wrong: %+v", input)
	}
	if input.MinLength != nil {
		t.Errorf("min length should be unset, got %d", *input.MinLength)
	}

	second := modal.Components[1].(discord.LabelComponent).Component.(discord.TextInputComponent)
	if second.Value != "save" {
		t.Errorf("prefilled value = %q, want save", second.Value)
	}
}

// A modal field's label has to survive the format change, or every field in
// every form would render blank.
func TestModalToDisgoKeepsEveryLabel(t *testing.T) {
	encoded, err := json.Marshal(realModal().toDisgo())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, want := range []string{
		"Egg Inc ID (EI+16 digits)",
		"Save or Forget this ID after this session?",
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("label %q missing from the payload: %s", want, encoded)
		}
	}
}

func TestTextInputToDisgoStylesAndLimits(t *testing.T) {
	paragraph := TextInput{CustomID: "notes", Style: TextInputStyleParagraph, MinLength: 5, MaxLength: 200}.disgoTextInput()
	if paragraph.Style != discord.TextInputStyleParagraph {
		t.Errorf("style = %d, want %d", paragraph.Style, discord.TextInputStyleParagraph)
	}
	if paragraph.MinLength == nil || *paragraph.MinLength != 5 {
		t.Errorf("min length = %v, want 5", paragraph.MinLength)
	}
	if paragraph.MaxLength != 200 {
		t.Errorf("max length = %d, want 200", paragraph.MaxLength)
	}

	// disgo numbers the styles from 1, so a missing conversion would turn a
	// short field into a paragraph and an unset one into nothing at all.
	if int(TextInputStyleShort) == int(discord.TextInputStyleShort) {
		t.Error("the facade and disgo text input styles happen to line up; this test no longer proves anything")
	}
}
