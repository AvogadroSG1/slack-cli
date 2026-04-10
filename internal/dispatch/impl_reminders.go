package dispatch

import (
	"context"

	"github.com/slack-go/slack"
)

// implRemindersDelete implements reminders.delete.
func implRemindersDelete(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	err := client.DeleteReminderContext(ctx, flagStr(flags, "reminder"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implRemindersList implements reminders.list.
func implRemindersList(ctx context.Context, client *slack.Client, _ map[string]any) (any, error) {
	reminders, err := client.ListRemindersContext(ctx)
	if err != nil {
		return nil, err
	}
	return reminders, nil
}

func init() {
	RegisterDispatch("reminders.delete", implRemindersDelete)
	RegisterDispatch("reminders.list", implRemindersList)
}
