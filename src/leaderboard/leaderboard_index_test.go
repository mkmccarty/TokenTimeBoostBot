package leaderboard

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
)

func TestBuildChannelIndexComponents(t *testing.T) {
	guildID := "test_guild_idx_build"
	chID := "test_chan_idx_1"
	otherChID := "test_chan_idx_2"
	postedAt := time.Unix(1795273413, 0)

	configs := []LBConfig{
		{
			LBType:     "group_cs_stats",
			GuildID:    guildID,
			ChannelID:  chID,
			MessageIDs: []string{"msg_cs_1", ""}, // Second member has no message ID and should be skipped
		},
		{
			LBType:     LBDrones,
			GuildID:    guildID,
			ChannelID:  chID,
			MessageIDs: []string{"msg_drones_1"},
		},
		{
			LBType:     LBPrestiges,
			GuildID:    guildID,
			ChannelID:  otherChID,
			MessageIDs: []string{"msg_other_chan"},
		},
	}

	components := buildChannelIndexComponents(guildID, chID, configs, postedAt)
	if len(components) != 1 {
		t.Fatalf("expected 1 TextDisplay component, got %d", len(components))
	}

	td, ok := components[0].(dc.TextDisplay)
	if !ok {
		t.Fatalf("expected dc.TextDisplay component, got %T", components[0])
	}

	expectedTimestamp := bottools.WrapTimestamp(postedAt.Unix(), bottools.TimestampShortDateTime)
	if !strings.Contains(td.Content, expectedTimestamp) {
		t.Fatalf("expected index title to include timestamp %q, got:\n%s", expectedTimestamp, td.Content)
	}
	if !strings.Contains(td.Content, fmt.Sprintf("[Contract Score](https://discord.com/channels/%s/%s/msg_cs_1)", guildID, chID)) {
		t.Fatalf("expected Contract Score link in index, got:\n%s", td.Content)
	}
	if strings.Contains(td.Content, "Weekly CS") {
		t.Fatalf("did not expect Weekly CS with empty message ID in index, got:\n%s", td.Content)
	}
	if !strings.Contains(td.Content, fmt.Sprintf("[Drones](https://discord.com/channels/%s/%s/msg_drones_1)", guildID, chID)) {
		t.Fatalf("expected Drones link in index, got:\n%s", td.Content)
	}
	if strings.Contains(td.Content, "msg_other_chan") {
		t.Fatalf("did not expect message from another channel in index, got:\n%s", td.Content)
	}

	empty := buildChannelIndexComponents(guildID, "empty_chan", configs, postedAt)
	if empty != nil {
		t.Fatalf("expected nil components for channel with no configs, got %v", empty)
	}
}

func TestBuildChannelIndexComponents_SplitsLongIndex(t *testing.T) {
	guildID := "test_guild_idx_long"
	chID := "test_chan_idx_long"
	postedAt := time.Unix(1795273413, 0)

	var configs []LBConfig
	for i, def := range AllLeaderboards {
		configs = append(configs, LBConfig{
			LBType:     def.Key,
			GuildID:    guildID,
			ChannelID:  chID,
			MessageIDs: []string{fmt.Sprintf("10000000000000000%02d", i)},
		})
	}

	components := buildChannelIndexComponents(guildID, chID, configs, postedAt)
	if len(components) < 2 {
		t.Fatalf("expected long index to split across multiple TextDisplay components, got %d", len(components))
	}
	for i, comp := range components {
		td, ok := comp.(dc.TextDisplay)
		if !ok {
			t.Fatalf("component %d: expected dc.TextDisplay, got %T", i, comp)
		}
		if len(td.Content) > discordMessageCharLimit {
			t.Fatalf("component %d length %d exceeds discordMessageCharLimit %d", i, len(td.Content), discordMessageCharLimit)
		}
	}
}

func TestPostChannelIndex_Actions(t *testing.T) {
	origDelay := rateLimitDelay
	rateLimitDelay = 0
	t.Cleanup(func() { rateLimitDelay = origDelay })

	guildID := "test_guild_idx_actions"
	chID := "test_chan_idx_actions"

	t.Cleanup(func() {
		_ = DeleteGuildLBConfig(guildID, LBDrones)
		_ = guildstate.DeleteGuildState(guildID)
	})
	_ = guildstate.DeleteGuildState(guildID)

	if err := UpsertGuildLBConfig(LBConfig{
		LBType:     LBDrones,
		GuildID:    guildID,
		ChannelID:  chID,
		MessageIDs: []string{"lb_msg_1"},
	}); err != nil {
		t.Fatalf("UpsertGuildLBConfig: %v", err)
	}

	postedAt := time.Unix(1795273413, 0)
	client := dctest.New()
	client.NextMessageID = "idx"

	// 1. Initial update with no prior index message: sends a new index message.
	postChannelIndex(client, guildID, chID, "update", postedAt)
	if len(client.SentMessages) != 1 {
		t.Fatalf("expected 1 sent message on initial update, got %d", len(client.SentMessages))
	}
	storedID := guildstate.GetGuildSettingString(guildID, channelIndexSettingKey(chID))
	if storedID != "idx-1" {
		t.Fatalf("expected stored index ID %q, got %q", "idx-1", storedID)
	}

	// 2. Subsequent update: edits the existing index message in place.
	postChannelIndex(client, guildID, chID, "update", postedAt.Add(time.Minute))
	if len(client.SentMessages) != 1 {
		t.Fatalf("expected no new sent messages on update, got %d", len(client.SentMessages))
	}
	if len(client.EditedMessages) != 1 {
		t.Fatalf("expected 1 edited message on update, got %d", len(client.EditedMessages))
	}
	if got := client.CallsTo("EditMessage")[0].Args[1]; got != "idx-1" {
		t.Fatalf("expected EditMessage on %q, got %v", "idx-1", got)
	}

	// 3. Update when the existing index message was deleted: reposts a new index message.
	client.EditErr = dctest.APIError(404, dc.ErrCodeUnknownMessage, "Unknown Message")
	postChannelIndex(client, guildID, chID, "update", postedAt.Add(2*time.Minute))
	client.EditErr = nil
	if len(client.SentMessages) != 2 {
		t.Fatalf("expected reposted message on Unknown Message edit failure, got %d sent", len(client.SentMessages))
	}
	repostedID := guildstate.GetGuildSettingString(guildID, channelIndexSettingKey(chID))
	if repostedID == "" || repostedID == storedID {
		t.Fatalf("expected new stored index ID after repost, got %q (was %q)", repostedID, storedID)
	}

	// 4. Bump: sends a new index message and deletes the old one.
	postChannelIndex(client, guildID, chID, "bump", postedAt.Add(3*time.Minute))
	if len(client.SentMessages) != 3 {
		t.Fatalf("expected new sent message on bump, got %d sent", len(client.SentMessages))
	}
	bumpedID := guildstate.GetGuildSettingString(guildID, channelIndexSettingKey(chID))
	if bumpedID == "" || bumpedID == repostedID {
		t.Fatalf("expected new stored index ID after bump, got %q (was %q)", bumpedID, repostedID)
	}
	deletes := client.CallsTo("DeleteMessage")
	if len(deletes) != 1 || deletes[0].Args[1] != repostedID {
		t.Fatalf("expected DeleteMessage for old index %q on bump, got %+v", repostedID, deletes)
	}

	// 5. New: sends a new index message and preserves the old one.
	postChannelIndex(client, guildID, chID, "new", postedAt.Add(4*time.Minute))
	if len(client.SentMessages) != 4 {
		t.Fatalf("expected new sent message on new action, got %d sent", len(client.SentMessages))
	}
	newID := guildstate.GetGuildSettingString(guildID, channelIndexSettingKey(chID))
	if newID == "" || newID == bumpedID {
		t.Fatalf("expected new stored index ID after new, got %q (was %q)", newID, bumpedID)
	}
	if len(client.CallsTo("DeleteMessage")) != 1 {
		t.Fatalf("expected no additional DeleteMessage on new action, got %d", len(client.CallsTo("DeleteMessage")))
	}
}
