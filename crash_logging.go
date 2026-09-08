package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
	"github.com/mkmccarty/TokenTimeBoostBot/src/version"
)

const (
	crashLogFileName    = "ttbb-data/crash.log"
	maxCrashAlertLength = 1900
)

func writeCrashLog(scope string, recovered any, metadata map[string]string) {
	if err := os.MkdirAll("ttbb-data", 0755); err != nil {
		log.Printf("panic recovered but could not create crashlog directory: %v", err)
	}

	md := make(map[string]string, len(metadata)+8)
	for k, v := range metadata {
		md[k] = sanitizeCrashValue(v)
	}
	if hostname, err := os.Hostname(); err == nil {
		md["host"] = hostname
	}
	md["scope"] = scope
	md["pid"] = strconv.Itoa(os.Getpid())
	md["cwd"] = mustGetwd()
	md["release"] = version.Release
	md["build"] = Version
	md["go_version"] = runtime.Version()
	md["goos"] = runtime.GOOS
	md["goarch"] = runtime.GOARCH

	keys := make([]string, 0, len(md))
	for k := range md {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	panicType := fmt.Sprintf("%T", recovered)
	panicValue := sanitizeCrashValue(fmt.Sprintf("%v", recovered))
	stack := debug.Stack()
	var b strings.Builder
	now := time.Now()
	b.WriteString("\n=== panic-report-v1 ===\n")
	_, _ = fmt.Fprintf(&b, "timestamp_rfc3339=%s\n", now.Format(time.RFC3339))
	_, _ = fmt.Fprintf(&b, "timestamp_unix=%d\n", now.Unix())
	_, _ = fmt.Fprintf(&b, "panic_type=%s\n", panicType)
	_, _ = fmt.Fprintf(&b, "panic_value=%s\n", panicValue)
	b.WriteString("runtime:\n")
	_, _ = fmt.Fprintf(&b, "go_version=%s\n", runtime.Version())
	_, _ = fmt.Fprintf(&b, "goos=%s\n", runtime.GOOS)
	_, _ = fmt.Fprintf(&b, "goarch=%s\n", runtime.GOARCH)
	_, _ = fmt.Fprintf(&b, "num_cpu=%d\n", runtime.NumCPU())
	_, _ = fmt.Fprintf(&b, "num_goroutine=%d\n", runtime.NumGoroutine())
	b.WriteString("process:\n")
	_, _ = fmt.Fprintf(&b, "pid=%d\n", os.Getpid())
	_, _ = fmt.Fprintf(&b, "ppid=%d\n", os.Getppid())
	_, _ = fmt.Fprintf(&b, "executable=%s\n", mustExecutablePath())
	_, _ = fmt.Fprintf(&b, "cwd=%s\n", mustGetwd())
	_, _ = fmt.Fprintf(&b, "scope=%s\n", scope)
	_, _ = fmt.Fprintf(&b, "release=%s\n", version.Release)
	_, _ = fmt.Fprintf(&b, "build=%s\n", Version)
	b.WriteString("metadata:\n")
	for _, k := range keys {
		_, _ = fmt.Fprintf(&b, "%s=%s\n", k, md[k])
	}
	b.WriteString("stack:\n")
	b.Write(stack)
	b.WriteString("llm_triage_prompt:\n")
	b.WriteString("Please analyze this panic report and produce: probable root cause, exact file/function likely responsible, and smallest safe code change to prevent recurrence. If nil pointer is possible, identify which value is likely nil and where to guard it.\n")
	b.WriteString("\n")

	message := b.String()

	f, err := os.OpenFile(crashLogFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("panic recovered but could not open %s: %v", crashLogFileName, err)
		log.Print(message)
		return
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			log.Printf("panic recovered but could not close %s: %v", crashLogFileName, cerr)
		}
	}()

	if _, err := f.WriteString(message); err != nil {
		log.Printf("panic recovered but could not write crashlog: %v", err)
	}

	log.Printf("Panic captured in %s; details written to %s", scope, crashLogFileName)
}

func sanitizeCrashValue(v string) string {
	v = strings.ReplaceAll(v, "\n", "\\n")
	v = strings.ReplaceAll(v, "\r", "")
	if len(v) > 2048 {
		return v[:2048] + "...<truncated>"
	}
	return v
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return wd
}

func mustExecutablePath() string {
	execPath, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	return execPath
}

func recoverPanic(scope string, exitCode int, metadata map[string]string) {
	if r := recover(); r != nil {
		handlePanic(scope, r, metadata)
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}
}

// handlePanic records an already-recovered panic. dc.Bot recovers panics
// raised inside interaction handlers itself and hands the value to its OnPanic
// hook, so that path cannot go through recoverPanic's own recover().
func handlePanic(scope string, recovered any, metadata map[string]string) {
	writeCrashLog(scope, recovered, metadata)
	sendCrashAlert(scope, recovered, metadata)
}

func sendCrashAlert(scope string, recovered any, metadata map[string]string) {
	defer func() {
		if rr := recover(); rr != nil {
			log.Printf("Failed while sending crash alert: %v", rr)
		}
	}()

	client := bot.Client()
	if client == nil {
		return
	}

	content := formatCrashAlert(scope, recovered, metadata)

	channelID := getCrashAlertChannelID(metadata)
	if channelID != "" {
		if _, err := client.SendMessage(channelID, dc.Message{Content: content}); err != nil {
			log.Printf("Failed to send crash alert to channel %s: %v", channelID, err)
		}
	}

	adminUserID := strings.TrimSpace(config.AdminUserID)
	if adminUserID == "" {
		return
	}

	dm, err := client.CreateUserChannel(adminUserID)
	if err != nil {
		log.Printf("Failed to create admin DM channel for crash alert (user=%s): %v", adminUserID, err)
		return
	}

	if _, err := client.SendMessage(dm.ID, dc.Message{Content: content}); err != nil {
		log.Printf("Failed to send crash alert DM to admin user %s: %v", adminUserID, err)
	}
}

func getCrashAlertChannelID(metadata map[string]string) string {
	if metadata != nil {
		if channelID := strings.TrimSpace(metadata["crash_alert_channel"]); channelID != "" {
			return channelID
		}
	}

	if channelID := strings.TrimSpace(guildstate.GetGuildSettingString("DEFAULT", "crash_alert_channel")); channelID != "" {
		return channelID
	}

	if channelID := strings.TrimSpace(guildstate.GetGuildSettingString("DEFAULT", "admin_logs_channel")); channelID != "" {
		return channelID
	}

	return ""
}

func formatCrashAlert(scope string, recovered any, metadata map[string]string) string {
	panicType := fmt.Sprintf("%T", recovered)
	panicValue := sanitizeCrashValue(fmt.Sprintf("%v", recovered))

	lines := []string{
		"TTBB crash recovered",
		fmt.Sprintf("scope=%s", scope),
		fmt.Sprintf("panic_type=%s", panicType),
		fmt.Sprintf("panic_value=%s", panicValue),
		fmt.Sprintf("build=%s", Version),
		fmt.Sprintf("release=%s", version.Release),
		fmt.Sprintf("log=%s", crashLogFileName),
	}

	keys := []string{
		"interaction_type",
		"command_name",
		"custom_id",
		"guild_id",
		"channel_id",
		"user_id",
		"message_id",
		"job",
		"heartbeat_path",
	}

	for _, k := range keys {
		if metadata == nil {
			break
		}
		if v := strings.TrimSpace(metadata[k]); v != "" {
			lines = append(lines, fmt.Sprintf("%s=%s", k, sanitizeCrashValue(v)))
		}
	}

	body := strings.Join(lines, "\n")
	message := "```\n" + body + "\n```"

	if len(message) > maxCrashAlertLength {
		trimmed := message[:maxCrashAlertLength-len("...\n```")] + "...\n```"
		return trimmed
	}

	return message
}

func handleCrash() {
	recoverPanic("main", 2, nil)
}

func safeGo(scope string, fn func()) {
	safeGoMeta(scope, nil, fn)
}

func safeGoMeta(scope string, metadata map[string]string, fn func()) {
	go func() {
		defer recoverPanic(scope, 0, metadata)
		fn()
	}()
}

// withSessionHints adds the bot's own identity to crash metadata, which is
// what tells two bots logging to the same place apart.
func withSessionHints(md map[string]string) map[string]string {
	if md == nil {
		md = map[string]string{}
	}

	md["discord_intents"] = strconv.Itoa(int(gatewayIntents))
	if userID := bot.UserID(); userID != "" {
		md["bot_user_id"] = userID
		if client := bot.Client(); client != nil {
			md["bot_username"] = client.BotUsername()
		}
	}

	return md
}

// interactionCrashMetadata describes the interaction a handler panicked on.
func interactionCrashMetadata(meta dc.InteractionMeta) map[string]string {
	md := map[string]string{
		"interaction_type": string(meta.Kind),
		"guild_id":         meta.GuildID,
		"channel_id":       meta.ChannelID,
		"user_id":          meta.UserID,
	}

	switch meta.Kind {
	case dc.KindCommand, dc.KindAutocomplete:
		md["command_name"] = meta.Name
	case dc.KindModal, dc.KindComponent:
		md["custom_id"] = meta.CustomID
	}

	return withSessionHints(md)
}
