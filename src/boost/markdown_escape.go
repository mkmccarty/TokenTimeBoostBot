package boost

import "strings"

var discordMarkdownEscaper = strings.NewReplacer(
	"\\", "\\\\",
	"*", "\\*",
	"_", "\\_",
	"~", "\\~",
	"`", "\\`",
	"|", "\\|",
	">", "\\>",
	"#", "\\#",
	"[", "\\[",
	"]", "\\]",
	"(", "\\(",
	")", "\\)",
)

func escapeDiscordMarkdown(s string) string {
	if s == "" {
		return ""
	}
	return discordMarkdownEscaper.Replace(s)
}
