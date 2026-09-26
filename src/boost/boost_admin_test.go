package boost

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func TestGetVersionAndRevisionInfo(t *testing.T) {
	runningVer, runningRev, _, diskVer, diskRev, _, _ := getVersionAndRevisionInfo()

	if runningVer == "" {
		t.Errorf("expected runningVer to not be empty")
	}
	t.Logf("runningVer=%q, runningRev=%q, diskVer=%q, diskRev=%q", runningVer, runningRev, diskVer, diskRev)
}

func TestLdflagsVersionRegex(t *testing.T) {
	tests := []struct {
		ldflags string
		want    string
	}{
		{`-s -w -X main.Version=v7.0-203-ge42cb8920-dirty`, `v7.0-203-ge42cb8920-dirty`},
		{`-X main.Version=v7.0-100`, `v7.0-100`},
		{`-X 'main.Version=v7.0-99'`, `v7.0-99`},
		{`-X "main.Version=v7.0-98"`, `v7.0-98`},
		{`-X Version=v1.2.3`, `v1.2.3`},
	}

	for _, tt := range tests {
		m := ldflagsVersionRegex.FindStringSubmatch(tt.ldflags)
		if len(m) < 2 || m[1] != tt.want {
			t.Errorf("ldflagsVersionRegex(%q) = %v; want %q", tt.ldflags, m, tt.want)
		}
	}
}

func TestAdminExitNoticeCycle(t *testing.T) {
	defer os.Remove(adminExitNoticeFile)
	_ = os.Remove(adminExitNoticeFile)

	exitTime := time.Now()
	exitTimestamp := bottools.WrapTimestamp(exitTime.Unix(), bottools.TimestampLongTime)
	initialText := fmt.Sprintf("Bot is exiting gracefully for a restart... %s", exitTimestamp)

	notice := AdminExitNotice{
		ApplicationID:    "123456",
		InteractionToken: "sample-token",
		MessageID:        "987654",
		ChannelID:        "111222",
		GuildID:          "333444",
		UserID:           "555666",
		ExitTime:         exitTime,
		InitialText:      initialText,
	}

	saveAdminExitNotice(notice)

	if _, err := os.Stat(adminExitNoticeFile); os.IsNotExist(err) {
		t.Fatalf("expected admin exit notice file to be created")
	}

	fakeClient := dctest.New()
	CheckAndNotifyAdminExitRestart(fakeClient)

	// Verify notice file was removed after processing
	if _, err := os.Stat(adminExitNoticeFile); !os.IsNotExist(err) {
		t.Errorf("expected admin exit notice file to be removed after restart notification")
	}

	// Verify FakeClient recorded the edit call
	calls := fakeClient.CallsTo("EditInteractionResponse")
	if len(calls) == 0 {
		t.Fatalf("expected EditInteractionResponse to be called on restart")
	}

	call := calls[0]
	if call.Args[0] != "123456" || call.Args[1] != "sample-token" {
		t.Errorf("unexpected args to EditInteractionResponse: %v", call.Args)
	}

	updatedText := call.Args[2]
	if !strings.HasPrefix(updatedText, initialText) {
		t.Errorf("expected updated text to start with %q, got %q", initialText, updatedText)
	}
	if !strings.Contains(updatedText, "Bot process started") {
		t.Errorf("expected updated text to contain 'Bot process started', got %q", updatedText)
	}
	if !strings.Contains(updatedText, "Bot is online and ready") {
		t.Errorf("expected updated text to contain 'Bot is online and ready', got %q", updatedText)
	}
}

func TestAdminExitNoticeExpired(t *testing.T) {
	defer os.Remove(adminExitNoticeFile)
	_ = os.Remove(adminExitNoticeFile)

	notice := AdminExitNotice{
		ApplicationID:    "123456",
		InteractionToken: "sample-token",
		MessageID:        "987654",
		ChannelID:        "111222",
		ExitTime:         time.Now().Add(-20 * time.Minute), // expired
		InitialText:      "Bot is exiting gracefully for a restart...",
	}

	saveAdminExitNotice(notice)

	fakeClient := dctest.New()
	CheckAndNotifyAdminExitRestart(fakeClient)

	// Verify no edit call was made for expired notice
	calls := fakeClient.CallsTo("EditInteractionResponse")
	if len(calls) != 0 {
		t.Errorf("expected no EditInteractionResponse calls for expired notice, got %d", len(calls))
	}
}
