package boost

import (
	"regexp"
	"strings"
)

var mentionEscapeReplacer = strings.NewReplacer(
	`\\u003c`, "<",
	`\\u003e`, ">",
	`\\u003C`, "<",
	`\\u003E`, ">",
	`\u003c`, "<",
	`\u003e`, ">",
	`\u003C`, "<",
	`\u003E`, ">",
)

var userMentionRegex = regexp.MustCompile(`^<@!?(\d+)>$`)

func normalizeMentionSyntax(value string) string {
	return mentionEscapeReplacer.Replace(strings.TrimSpace(value))
}

func parseMentionUserID(value string) (string, bool) {
	normalized := normalizeMentionSyntax(value)
	matches := userMentionRegex.FindStringSubmatch(normalized)
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}

func normalizeUserIDInput(value string) string {
	if userID, ok := parseMentionUserID(value); ok {
		return userID
	}
	return normalizeMentionSyntax(value)
}
