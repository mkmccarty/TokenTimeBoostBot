package dctest

import (
	"net/http"

	"github.com/disgoorg/disgo/rest"
)

// APIError builds the error a rejected Discord REST call produces, so a test
// can exercise dc's error classification without naming the underlying
// library. Pass a zero code for a response Discord sent no error body with.
func APIError(status, code int, message string) error {
	return &rest.Error{
		Response: &http.Response{StatusCode: status},
		Code:     rest.JSONErrorCode(code),
		Message:  message,
	}
}
