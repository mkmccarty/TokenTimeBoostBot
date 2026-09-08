package dc

import "github.com/disgoorg/disgo/discord"

// This file renders the facade's modal types into disgo's. It is the disgo
// half of modal.go, which still renders the discordgo half until the
// interaction events stop speaking discordgo.

func (s TextInputStyle) disgoStyle() discord.TextInputStyle {
	if s == TextInputStyleParagraph {
		return discord.TextInputStyleParagraph
	}
	return discord.TextInputStyleShort
}

func (t TextInput) disgoTextInput() discord.TextInputComponent {
	input := discord.TextInputComponent{
		CustomID:    t.CustomID,
		Style:       t.Style.disgoStyle(),
		Placeholder: t.Placeholder,
		Value:       t.Value,
		Required:    t.Required,
		MaxLength:   t.MaxLength,
	}
	// disgo types MinLength as a pointer, where the facade uses 0 for "no
	// minimum" — the same meaning discordgo's plain int carried.
	if t.MinLength > 0 {
		minLength := t.MinLength
		input.MinLength = &minLength
	}
	return input
}

// toDisgo renders the modal as the payload disgo submits.
//
// The shape differs from the discordgo rendering, and not by choice.
// discordgo builds Discord's original modal: an action row per field, with the
// field's label on the text input itself. disgo's TextInputComponent has no
// Label at all, because Discord moved a modal field's label out to a wrapping
// label component. Each input is therefore wrapped in a discord.LabelComponent
// carrying the label, rather than placed in an action row. Both render as a
// labelled field to the user.
func (m Modal) toDisgo() discord.ModalCreate {
	modal := discord.ModalCreate{
		CustomID: m.CustomID,
		Title:    m.Title,
	}
	for _, input := range m.Inputs {
		modal.Components = append(modal.Components, discord.LabelComponent{
			Label:     input.Label,
			Component: input.disgoTextInput(),
		})
	}
	return modal
}
