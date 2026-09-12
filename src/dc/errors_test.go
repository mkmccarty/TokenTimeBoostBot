package dc

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/disgoorg/disgo/rest"
)

func restError(status, code int, message string) error {
	return &rest.Error{
		Response: &http.Response{StatusCode: status},
		Code:     rest.JSONErrorCode(code),
		Message:  message,
	}
}

func TestAsAPIError(t *testing.T) {
	got, ok := AsAPIError(restError(404, ErrCodeUnknownMessage, "Unknown Message"))
	if !ok {
		t.Fatal("expected a Discord API error")
	}
	if got.StatusCode != 404 || got.Code != ErrCodeUnknownMessage || got.Message != "Unknown Message" {
		t.Fatalf("wrong translation: %+v", got)
	}
}

func TestAsAPIErrorWrapped(t *testing.T) {
	wrapped := fmt.Errorf("posting leaderboard: %w", restError(404, ErrCodeUnknownChannel, "Unknown Channel"))
	got, ok := AsAPIError(wrapped)
	if !ok {
		t.Fatal("expected errors.As to unwrap to a Discord API error")
	}
	if got.Code != ErrCodeUnknownChannel {
		t.Fatalf("want code %d, got %d", ErrCodeUnknownChannel, got.Code)
	}
}

func TestAsAPIErrorOther(t *testing.T) {
	if _, ok := AsAPIError(errors.New("connection reset")); ok {
		t.Fatal("a plain error is not a Discord API error")
	}
	if _, ok := AsAPIError(nil); ok {
		t.Fatal("nil is not a Discord API error")
	}
}

// A REST failure can arrive without a parsed body, in which case only the HTTP
// status is known.
func TestAsAPIErrorNoBody(t *testing.T) {
	err := &rest.Error{Response: &http.Response{StatusCode: 500}}
	got, ok := AsAPIError(err)
	if !ok {
		t.Fatal("expected a Discord API error")
	}
	if got.StatusCode != 500 || got.Code != 0 {
		t.Fatalf("wrong translation: %+v", got)
	}
}

func TestIsUnknownMessage(t *testing.T) {
	if !IsUnknownMessage(restError(404, ErrCodeUnknownMessage, "Unknown Message")) {
		t.Fatal("expected unknown message")
	}
	if IsUnknownMessage(restError(404, ErrCodeUnknownChannel, "Unknown Channel")) {
		t.Fatal("unknown channel is not unknown message")
	}
	if IsUnknownMessage(errors.New("nope")) {
		t.Fatal("a plain error is not unknown message")
	}
}

func TestIsUnknownChannel(t *testing.T) {
	if !IsUnknownChannel(restError(400, ErrCodeUnknownChannel, "Unknown Channel")) {
		t.Fatal("expected unknown channel by code")
	}
	// A bare 404 also means the channel is gone.
	if !IsUnknownChannel(&rest.Error{Response: &http.Response{StatusCode: 404}}) {
		t.Fatal("expected unknown channel by status")
	}
	if IsUnknownChannel(restError(403, 50013, "Missing Permissions")) {
		t.Fatal("missing permissions is not unknown channel")
	}
}

func TestIsThreadArchived(t *testing.T) {
	if !IsThreadArchived(restError(400, ErrCodeThreadArchived, "Thread is archived")) {
		t.Fatal("expected thread is archived")
	}
	if IsThreadArchived(restError(404, ErrCodeUnknownChannel, "Unknown Channel")) {
		t.Fatal("unknown channel is not thread archived")
	}
	if IsThreadArchived(errors.New("nope")) {
		t.Fatal("a plain error is not thread archived")
	}
}

