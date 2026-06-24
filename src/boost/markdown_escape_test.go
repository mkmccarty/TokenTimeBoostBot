package boost

import "testing"

func TestEscapeDiscordMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "plain", input: "Alpha Bravo", want: "Alpha Bravo"},
		{name: "markdown chars", input: "*_~`|>#[]()", want: "\\*\\_\\~\\`\\|\\>\\#\\[\\]\\(\\)"},
		{name: "mixed name", input: "A*B_[C](D)#E", want: "A\\*B\\_\\[C\\]\\(D\\)\\#E"},
		{name: "backslash", input: `A\\B`, want: `A\\\\B`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeDiscordMarkdown(tc.input)
			if got != tc.want {
				t.Fatalf("escapeDiscordMarkdown() = %q, want %q", got, tc.want)
			}
		})
	}
}
