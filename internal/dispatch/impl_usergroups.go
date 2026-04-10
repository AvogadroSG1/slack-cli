package dispatch

import (
	"context"

	"github.com/slack-go/slack"
)

// implUsergroupsCreate implements usergroups.create.
func implUsergroupsCreate(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	ug := slack.UserGroup{
		Name:        flagStr(flags, "name"),
		Description: flagStr(flags, "description"),
		Handle:      flagStr(flags, "handle"),
	}
	result, err := client.CreateUserGroupContext(ctx, ug)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// implUsergroupsDisable implements usergroups.disable.
func implUsergroupsDisable(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	result, err := client.DisableUserGroupContext(ctx, flagStr(flags, "usergroup"))
	if err != nil {
		return nil, err
	}
	return result, nil
}

// implUsergroupsEnable implements usergroups.enable.
func implUsergroupsEnable(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	result, err := client.EnableUserGroupContext(ctx, flagStr(flags, "usergroup"))
	if err != nil {
		return nil, err
	}
	return result, nil
}

// implUsergroupsList implements usergroups.list.
func implUsergroupsList(ctx context.Context, client *slack.Client, _ map[string]any) (any, error) {
	groups, err := client.GetUserGroupsContext(ctx)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// implUsergroupsUpdate implements usergroups.update.
func implUsergroupsUpdate(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	result, err := client.UpdateUserGroupContext(ctx, flagStr(flags, "usergroup"))
	if err != nil {
		return nil, err
	}
	return result, nil
}

// implUsergroupsUsersList implements usergroups.users.list.
func implUsergroupsUsersList(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	members, err := client.GetUserGroupMembersContext(ctx, flagStr(flags, "usergroup"))
	if err != nil {
		return nil, err
	}
	return members, nil
}

// implUsergroupsUsersUpdate implements usergroups.users.update.
func implUsergroupsUsersUpdate(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	result, err := client.UpdateUserGroupMembersContext(ctx, flagStr(flags, "usergroup"), flagStr(flags, "members"))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func init() {
	RegisterDispatch("usergroups.create", implUsergroupsCreate)
	RegisterDispatch("usergroups.disable", implUsergroupsDisable)
	RegisterDispatch("usergroups.enable", implUsergroupsEnable)
	RegisterDispatch("usergroups.list", implUsergroupsList)
	RegisterDispatch("usergroups.update", implUsergroupsUpdate)
	RegisterDispatch("usergroups.users.list", implUsergroupsUsersList)
	RegisterDispatch("usergroups.users.update", implUsergroupsUsersUpdate)
}
