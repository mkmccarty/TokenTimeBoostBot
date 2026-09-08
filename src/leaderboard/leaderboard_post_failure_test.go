package leaderboard

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

// Discord answers Unknown Message with HTTP 404, the same status a missing
// channel produces. The message case must win, or an orphaned post takes the
// guild's whole config down with it.
func TestClassifyEditFailureUnknownMessageBeforeChannel(t *testing.T) {
	err := dctest.APIError(404, dc.ErrCodeUnknownMessage, "Unknown Message")
	if got := classifyEditFailure(err); got != editFailureRepost {
		t.Fatalf("want editFailureRepost, got %v", got)
	}
}

func TestClassifyEditFailureUnknownChannel(t *testing.T) {
	err := dctest.APIError(404, dc.ErrCodeUnknownChannel, "Unknown Channel")
	if got := classifyEditFailure(err); got != editFailureChannelGone {
		t.Fatalf("want editFailureChannelGone, got %v", got)
	}
}

// A 404 with no error code says nothing about a message, so it is still read
// as a dead channel.
func TestClassifyEditFailureBare404(t *testing.T) {
	err := dctest.APIError(404, 0, "")
	if got := classifyEditFailure(err); got != editFailureChannelGone {
		t.Fatalf("want editFailureChannelGone, got %v", got)
	}
}

func TestClassifyEditFailureTransient(t *testing.T) {
	rateLimited := dctest.APIError(429, 0, "You are being rate limited.")
	if got := classifyEditFailure(rateLimited); got != editFailureRetryLater {
		t.Fatalf("want editFailureRetryLater, got %v", got)
	}
	if got := classifyEditFailure(errors.New("connection reset")); got != editFailureRetryLater {
		t.Fatalf("want editFailureRetryLater, got %v", got)
	}
}

func TestClassifyEditFailureWrapped(t *testing.T) {
	wrapped := fmt.Errorf("editing leaderboard: %w", dctest.APIError(404, dc.ErrCodeUnknownMessage, "Unknown Message"))
	if got := classifyEditFailure(wrapped); got != editFailureRepost {
		t.Fatalf("want editFailureRepost, got %v", got)
	}
}
