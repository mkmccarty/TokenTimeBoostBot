package boost

import "testing"

func TestParseMentionUserID(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantID string
		wantOK bool
	}{
		{name: "standard mention", input: "<@1234567890123456789>", wantID: "1234567890123456789", wantOK: true},
		{name: "nickname mention", input: "<@!1234567890123456789>", wantID: "1234567890123456789", wantOK: true},
		{name: "nickname mention with extra text", input: "<@!1234567890123456789> (fill)", wantID: "1234567890123456789", wantOK: true},
		{name: "nickname mention with prefix", input: "prefix <@!1234567890123456789>", wantID: "1234567890123456789", wantOK: true},
		{name: "double nickname mention", input: "<@1234567890123456789> <@67890123456789012345>", wantID: "1234567890123456789", wantOK: true},
		{name: "escaped mention", input: `\u003c@1234567890123456789\u003e`, wantID: "1234567890123456789", wantOK: true},
		{name: "escaped nickname mention", input: `\u003c@!1234567890123456789\u003e`, wantID: "1234567890123456789", wantOK: true},
		{name: "plain user id", input: "1234567890123456789", wantID: "", wantOK: false},
		{name: "guest name", input: "guest-farmer", wantID: "", wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotOK := parseMentionUserID(tc.input)
			if gotOK != tc.wantOK {
				t.Fatalf("parseMentionUserID() ok = %v, want %v", gotOK, tc.wantOK)
			}
			if gotID != tc.wantID {
				t.Fatalf("parseMentionUserID() id = %q, want %q", gotID, tc.wantID)
			}
		})
	}
}

func TestNormalizeUserIDInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "mention", input: "<@1234567890123456789>", want: "1234567890123456789"},
		{name: "escaped mention", input: `\u003c@1234567890123456789\u003e`, want: "1234567890123456789"},
		{name: "plain id", input: "1234567890123456789", want: "1234567890123456789"},
		{name: "guest", input: "guest-farmer", want: "guest-farmer"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeUserIDInput(tc.input)
			if got != tc.want {
				t.Fatalf("normalizeUserIDInput() = %q, want %q", got, tc.want)
			}
		})
	}
}
