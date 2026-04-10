package dispatch

import (
	"context"
	"fmt"

	"github.com/poconnor/slack-cli/internal/registry"
	"github.com/slack-go/slack"
)

// Paginate fetches results from a paginated Slack API method. When fetchAll
// is false it delegates to Execute and returns immediately. When fetchAll is
// true it follows cursor-based pagination until the cursor is empty or the
// effective limit is reached. Context cancellation between pages causes an
// early return with a partial result map.
func Paginate(
	ctx context.Context,
	client *slack.Client,
	method registry.MethodDef,
	flags map[string]any,
	fetchAll bool,
	limit int,
	maxResults int,
) (any, error) {
	if !fetchAll {
		return Execute(ctx, client, method.APIMethod, flags)
	}

	effectiveLimit := maxResults
	if limit > 0 && limit < maxResults {
		effectiveLimit = limit
	}

	var allResults []any
	cursor := ""

	for {
		// Set the cursor for this page (empty string on the first call is
		// fine — the API ignores it).
		if cursor != "" {
			flags[method.CursorParam] = cursor
		}

		raw, err := Execute(ctx, client, method.APIMethod, flags)
		if err != nil {
			return nil, fmt.Errorf("paginate %s: %w", method.APIMethod, err)
		}

		page, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("paginate %s: expected map[string]any, got %T", method.APIMethod, raw)
		}

		// Extract items from the response key.
		items, _ := page[method.ResponseKey].([]any)
		allResults = append(allResults, items...)

		// Extract the next cursor.
		cursor, _ = page[method.CursorField].(string)

		// Stop when we have enough results or no more pages.
		if len(allResults) >= effectiveLimit || cursor == "" {
			break
		}

		// Check for context cancellation between pages.
		select {
		case <-ctx.Done():
			return map[string]any{
				"results":     allResults,
				"partial":     true,
				"next_cursor": cursor,
				"reason":      "interrupted",
			}, nil
		default:
		}
	}

	// Trim results to the effective limit if we overshot.
	if len(allResults) > effectiveLimit {
		allResults = allResults[:effectiveLimit]
	}

	return map[string]any{
		"results": allResults,
	}, nil
}
