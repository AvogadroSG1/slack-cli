package dispatch

import (
	"context"

	"github.com/slack-go/slack"
)

// implFilesList implements files.list.
func implFilesList(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := slack.GetFilesParameters{
		User:    flagStr(flags, "user"),
		Channel: flagStr(flags, "channel"),
		TeamID:  flagStr(flags, "team-id"),
		Types:   flagStr(flags, "types"),
		Count:   flagInt(flags, "count"),
		Page:    flagInt(flags, "page"),
	}
	files, paging, err := client.GetFilesContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"files":  files,
		"paging": paging,
	}, nil
}

// implFilesInfo implements files.info.
func implFilesInfo(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	file, comments, paging, err := client.GetFileInfoContext(ctx, flagStr(flags, "file"), flagInt(flags, "count"), flagInt(flags, "page"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"file":     file,
		"comments": comments,
		"paging":   paging,
	}, nil
}

// implFilesDelete implements files.delete.
func implFilesDelete(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	err := client.DeleteFileContext(ctx, flagStr(flags, "file"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implFilesSharedPublicURL implements files.sharedPublicURL.
func implFilesSharedPublicURL(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	file, comments, paging, err := client.ShareFilePublicURLContext(ctx, flagStr(flags, "file"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"file":     file,
		"comments": comments,
		"paging":   paging,
	}, nil
}

// implFilesRevokePublicURL implements files.revokePublicURL.
func implFilesRevokePublicURL(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	file, err := client.RevokeFilePublicURLContext(ctx, flagStr(flags, "file"))
	if err != nil {
		return nil, err
	}
	return file, nil
}

func init() {
	RegisterDispatch("files.list", implFilesList)
	RegisterDispatch("files.info", implFilesInfo)
	RegisterDispatch("files.delete", implFilesDelete)
	RegisterDispatch("files.sharedPublicURL", implFilesSharedPublicURL)
	RegisterDispatch("files.revokePublicURL", implFilesRevokePublicURL)
}
