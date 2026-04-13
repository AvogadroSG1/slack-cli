// Package exitcode defines CLI exit codes and classifies errors from the
// Slack API into the appropriate code.
package exitcode

import (
	"context"
	"errors"

	"github.com/slack-go/slack"
)

// Exit code constants returned by the CLI process.
const (
	OK         = 0
	APIError   = 1
	AuthError  = 2
	InputError = 3
	NetError   = 4
)

// authErrors lists Slack API error strings that indicate an authentication
// or authorisation problem.
var authErrors = map[string]bool{
	"invalid_auth":     true,
	"not_authed":       true,
	"token_revoked":    true,
	"token_expired":    true,
	"account_inactive": true,
	"missing_scope":    true,
}

// Classify maps err to the appropriate exit code. A nil error returns OK.
// Wrapped errors are unwrapped via errors.As and errors.Is.
func Classify(err error) int {
	if err == nil {
		return OK
	}

	// Check for SlackErrorResponse (auth vs generic API errors).
	var slackErr slack.SlackErrorResponse
	if errors.As(err, &slackErr) {
		if authErrors[slackErr.Err] {
			return AuthError
		}
		return APIError
	}

	// Check for rate limiting.
	var rateErr *slack.RateLimitedError
	if errors.As(err, &rateErr) {
		return APIError
	}

	// Check for HTTP status code errors.
	var statusErr slack.StatusCodeError
	if errors.As(err, &statusErr) {
		return NetError
	}

	// Check for context errors.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return NetError
	}

	// Everything else is treated as a network-level failure.
	return NetError
}
