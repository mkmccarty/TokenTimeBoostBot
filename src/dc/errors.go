package dc

import (
	"errors"
	"fmt"

	"github.com/disgoorg/disgo/rest"
)

// Discord JSON error codes the bot reacts to. Discord returns these in the
// body of a failed REST call, alongside the HTTP status.
const (
	// ErrCodeUnknownChannel means the channel no longer exists.
	ErrCodeUnknownChannel = 10003
	// ErrCodeUnknownMessage means the message no longer exists.
	ErrCodeUnknownMessage = 10008
	// ErrCodeMissingAccess means the bot cannot see the channel at all.
	ErrCodeMissingAccess = 50001
	// ErrCodeMissingPermissions means the bot can see the channel but is not
	// allowed the action it attempted.
	ErrCodeMissingPermissions = 50013
	// ErrCodeThreadArchived means the thread is archived and must be unarchived
	// before messages can be sent to it.
	ErrCodeThreadArchived = 50083
)

// APIError is a rejected Discord REST call. Code is Discord's own error code
// from the response body and is zero when Discord sent no parsable body.
type APIError struct {
	StatusCode int
	Code       int
	Message    string
}

// Error implements error.
func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("discord: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("discord: HTTP %d (%d): %s", e.StatusCode, e.Code, e.Message)
}

// AsAPIError reports whether err came from a rejected Discord REST call, and
// if so translates it. It unwraps, so a wrapped error still matches.
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	var restErr *rest.Error
	if !errors.As(err, &restErr) {
		return nil, false
	}
	return apiErrorFrom(restErr), true
}

// apiErrorFrom converts a disgo REST failure into the facade's error.
func apiErrorFrom(err *rest.Error) *APIError {
	out := &APIError{Code: int(err.Code), Message: err.Message}
	if err.Response != nil {
		out.StatusCode = err.Response.StatusCode
	}
	return out
}

// wrapAPIError translates a rejected REST call into the facade's error type,
// so callers match on APIError rather than on whichever library made the call.
// Anything else passes through untouched.
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}
	var restErr *rest.Error
	if errors.As(err, &restErr) {
		return apiErrorFrom(restErr)
	}
	return err
}

// ErrWrongInteractionResponse is returned when an interaction is answered in a
// way Discord does not allow for its kind, such as opening a modal in reply to
// a modal submission.
var ErrWrongInteractionResponse = errors.New("dc: this interaction cannot be answered that way")

// IsUnknownMessage reports whether err is Discord refusing a call because the
// message is gone — deleted, or posted by a bot whose messages were purged.
func IsUnknownMessage(err error) bool {
	apiErr, ok := AsAPIError(err)
	return ok && apiErr.Code == ErrCodeUnknownMessage
}

// IsUnknownChannel reports whether err is Discord refusing a call because the
// channel is gone. A bare 404 counts: Discord does not always include an error
// code, and for a channel-scoped call there is nothing else it can mean.
func IsUnknownChannel(err error) bool {
	apiErr, ok := AsAPIError(err)
	if !ok {
		return false
	}
	return apiErr.Code == ErrCodeUnknownChannel || apiErr.StatusCode == 404
}

// IsThreadArchived reports whether err is Discord refusing a message send
// because the target thread is archived.
func IsThreadArchived(err error) bool {
	apiErr, ok := AsAPIError(err)
	return ok && apiErr.Code == ErrCodeThreadArchived
}
