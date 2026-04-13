package dispatch

import (
	"context"

	"github.com/slack-go/slack"
)

// implSearchMessages implements search.messages.
func implSearchMessages(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := slack.SearchParameters{
		Sort:          flagStr(flags, "sort"),
		SortDirection: flagStr(flags, "sort-direction"),
		Highlight:     flagBool(flags, "highlight"),
		Count:         flagInt(flags, "count"),
		Page:          flagInt(flags, "page"),
	}
	result, err := client.SearchMessagesContext(ctx, flagStr(flags, "query"), params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// implSearchFiles implements search.files.
func implSearchFiles(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := slack.SearchParameters{
		Sort:          flagStr(flags, "sort"),
		SortDirection: flagStr(flags, "sort-direction"),
		Highlight:     flagBool(flags, "highlight"),
		Count:         flagInt(flags, "count"),
		Page:          flagInt(flags, "page"),
	}
	result, err := client.SearchFilesContext(ctx, flagStr(flags, "query"), params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func init() {
	RegisterDispatch("search.messages", implSearchMessages)
	RegisterDispatch("search.files", implSearchFiles)
}
