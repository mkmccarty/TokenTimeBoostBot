package ei

import (
	"encoding/base64"
	"regexp"
	"testing"

	"google.golang.org/protobuf/proto"
)

const sampleLeaderboardBodyBase64 = "CgASQDFlYzVjOTY5ODA2ZTdhY2Q4YjZkYWUxODAxM2U5NDY4NTI4YTVlZjJiZmNkNmFlYmMwZGI2NzZlZTgzZTc5NjAgAA=="

func TestLeaderboardExampleBodyParsing(t *testing.T) {
	rawBytes, err := base64.StdEncoding.DecodeString(sampleLeaderboardBodyBase64)
	if err != nil {
		t.Fatalf("failed to decode sample response body from base64: %v", err)
	}

	t.Run("direct leaderboard decode", func(t *testing.T) {
		resp := &LeaderboardResponse{}
		if err := proto.Unmarshal(rawBytes, resp); err != nil {
			t.Fatalf("direct LeaderboardResponse unmarshal failed: %v", err)
		}

		if resp.GetScope() != "" || len(resp.GetTopEntries()) != 0 || resp.GetCount() != 0 {
			t.Fatalf("expected empty leaderboard when decoding sample directly, got scope=%q entries=%d count=%d", resp.GetScope(), len(resp.GetTopEntries()), resp.GetCount())
		}
	})

	t.Run("authenticated envelope decode", func(t *testing.T) {
		auth := &AuthenticatedMessage{}
		if err := proto.Unmarshal(rawBytes, auth); err != nil {
			t.Fatalf("AuthenticatedMessage unmarshal failed: %v", err)
		}

		if len(auth.GetMessage()) != 0 {
			t.Fatalf("expected empty inner message in sample, got %d bytes", len(auth.GetMessage()))
		}
		if auth.GetCode() == "" {
			t.Fatal("expected auth code in sample envelope, got empty code")
		}
		if ok, _ := regexp.MatchString("^[a-f0-9]{64}$", auth.GetCode()); !ok {
			t.Fatalf("expected 64-char lowercase hex auth code, got %q", auth.GetCode())
		}
		if auth.GetCompressed() {
			t.Fatal("expected sample envelope to be uncompressed")
		}
	})
}
