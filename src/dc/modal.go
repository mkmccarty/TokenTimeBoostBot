package dc

// TextInputStyle is the shape of a modal text input: a single line or a
// multi-line paragraph box.
type TextInputStyle int

// Text input styles a modal's TextInput can take.
const (
	TextInputStyleShort TextInputStyle = iota
	TextInputStyleParagraph
)

// TextInput is one text field on a Modal.
type TextInput struct {
	CustomID    string
	Label       string
	Style       TextInputStyle
	Placeholder string
	Value       string
	Required    bool
	MinLength   int
	MaxLength   int
}

// Modal is a form Discord presents to the user in response to an
// interaction, submitted back as a ModalEvent. CustomID follows this bot's
// "prefix#arg#...#contractHash" convention.
type Modal struct {
	CustomID string
	Title    string
	Inputs   []TextInput
}
