package dc

import "testing"

func TestCustomIDRoundTrip(t *testing.T) {
	id := CustomID("rc_", "boost", "quiet-name")
	if id != "rc_#boost#quiet-name" {
		t.Fatalf("want rc_#boost#quiet-name, got %q", id)
	}
	parts := SplitCustomID(id)
	if len(parts) != 3 || parts[0] != "rc_" || parts[2] != "quiet-name" {
		t.Fatalf("split wrong: %v", parts)
	}
}

func TestCustomIDRejectsSeparatorInSegment(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("want panic when a segment contains the separator")
		}
	}()
	_ = CustomID("rc_", "bo#ost")
}

func TestCustomIDSinglePart(t *testing.T) {
	if got := CustomID("solo"); got != "solo" {
		t.Fatalf("want solo, got %q", got)
	}
}

func TestCustomIDNoParts(t *testing.T) {
	if got := CustomID(); got != "" {
		t.Fatalf("want empty string, got %q", got)
	}
}

func TestSplitCustomIDNoSeparator(t *testing.T) {
	parts := SplitCustomID("solo")
	if len(parts) != 1 || parts[0] != "solo" {
		t.Fatalf("split wrong: %v", parts)
	}
}

func TestSplitContractHash(t *testing.T) {
	cases := map[string]string{
		"rc_#boost#hash123":     "hash123",
		"cs_#features#top#h456": "h456",
		"solo":                  "solo",
		"prefix#":               "",
	}
	for id, want := range cases {
		if got := SplitContractHash(id); got != want {
			t.Errorf("SplitContractHash(%q) = %q, want %q", id, got, want)
		}
	}
}
