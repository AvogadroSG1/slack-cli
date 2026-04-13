package exitcode_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"

	"github.com/poconnor/slack-cli/internal/exitcode"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		// nil
		{
			name: "nil error returns OK",
			err:  nil,
			want: exitcode.OK,
		},

		// Auth errors (SlackErrorResponse)
		{
			name: "invalid_auth returns AuthError",
			err:  slack.SlackErrorResponse{Err: "invalid_auth"},
			want: exitcode.AuthError,
		},
		{
			name: "not_authed returns AuthError",
			err:  slack.SlackErrorResponse{Err: "not_authed"},
			want: exitcode.AuthError,
		},
		{
			name: "token_revoked returns AuthError",
			err:  slack.SlackErrorResponse{Err: "token_revoked"},
			want: exitcode.AuthError,
		},
		{
			name: "token_expired returns AuthError",
			err:  slack.SlackErrorResponse{Err: "token_expired"},
			want: exitcode.AuthError,
		},
		{
			name: "account_inactive returns AuthError",
			err:  slack.SlackErrorResponse{Err: "account_inactive"},
			want: exitcode.AuthError,
		},
		{
			name: "missing_scope returns AuthError",
			err:  slack.SlackErrorResponse{Err: "missing_scope"},
			want: exitcode.AuthError,
		},

		// Generic API errors (SlackErrorResponse)
		{
			name: "channel_not_found returns APIError",
			err:  slack.SlackErrorResponse{Err: "channel_not_found"},
			want: exitcode.APIError,
		},
		{
			name: "unknown slack error returns APIError",
			err:  slack.SlackErrorResponse{Err: "some_other_error"},
			want: exitcode.APIError,
		},

		// Rate limiting
		{
			name: "RateLimitedError returns APIError",
			err:  &slack.RateLimitedError{RetryAfter: 30 * time.Second},
			want: exitcode.APIError,
		},

		// HTTP status code errors
		{
			name: "StatusCodeError returns NetError",
			err:  slack.StatusCodeError{Code: 502, Status: "502 Bad Gateway"},
			want: exitcode.NetError,
		},

		// Context errors
		{
			name: "DeadlineExceeded returns NetError",
			err:  context.DeadlineExceeded,
			want: exitcode.NetError,
		},
		{
			name: "Canceled returns NetError",
			err:  context.Canceled,
			want: exitcode.NetError,
		},

		// Wrapped errors
		{
			name: "wrapped auth error returns AuthError",
			err:  fmt.Errorf("api call failed: %w", slack.SlackErrorResponse{Err: "invalid_auth"}),
			want: exitcode.AuthError,
		},
		{
			name: "wrapped API error returns APIError",
			err:  fmt.Errorf("api call failed: %w", slack.SlackErrorResponse{Err: "channel_not_found"}),
			want: exitcode.APIError,
		},
		{
			name: "wrapped RateLimitedError returns APIError",
			err:  fmt.Errorf("rate limited: %w", &slack.RateLimitedError{RetryAfter: 5 * time.Second}),
			want: exitcode.APIError,
		},
		{
			name: "wrapped StatusCodeError returns NetError",
			err:  fmt.Errorf("http error: %w", slack.StatusCodeError{Code: 503, Status: "503 Service Unavailable"}),
			want: exitcode.NetError,
		},
		{
			name: "wrapped DeadlineExceeded returns NetError",
			err:  fmt.Errorf("timed out: %w", context.DeadlineExceeded),
			want: exitcode.NetError,
		},
		{
			name: "wrapped Canceled returns NetError",
			err:  fmt.Errorf("cancelled: %w", context.Canceled),
			want: exitcode.NetError,
		},

		// Unknown errors
		{
			name: "unknown error returns NetError",
			err:  errors.New("something went wrong"),
			want: exitcode.NetError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exitcode.Classify(tt.err)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Classify(%v) mismatch (-want +got):\n%s", tt.err, diff)
			}
		})
	}
}
