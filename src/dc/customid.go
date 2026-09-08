package dc

import "strings"

// customIDSeparator is the delimiter every CustomID in this bot is built
// from: "prefix#arg#...#contractHash". A component's handler prefix and its
// trailing contract hash are both segments split on this character, so it
// must never appear inside a segment.
const customIDSeparator = "#"

// CustomID joins parts into the bot's "prefix#arg#...#contractHash"
// convention for button, select menu and modal CustomIDs. It panics if any
// part contains the separator: that convention is load-bearing across every
// message the bot has already posted, and a silently malformed ID is worse
// than a crash caught in tests.
func CustomID(parts ...string) string {
	for _, part := range parts {
		if strings.Contains(part, customIDSeparator) {
			panic("dc.CustomID: segment contains " + customIDSeparator + ": " + part)
		}
	}
	return strings.Join(parts, customIDSeparator)
}

// SplitCustomID reverses CustomID, returning the segments in order.
func SplitCustomID(id string) []string {
	return strings.Split(id, customIDSeparator)
}
