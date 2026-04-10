package dispatch

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
)

// dispatchGetUserInfoImpl implements users.info.
func dispatchGetUserInfoImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	user := flagStr(flags, "user")
	if user == "" {
		return nil, fmt.Errorf("user flag is required")
	}

	u, err := client.GetUserInfoContext(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("users.info: %v", err)
	}

	return u, nil
}

// dispatchGetUsersImpl implements users.list.
func dispatchGetUsersImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	users, err := client.GetUsersContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("users.list: %v", err)
	}

	return users, nil
}

// dispatchGetUserPresenceImpl implements users.getPresence.
func dispatchGetUserPresenceImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	user := flagStr(flags, "user")
	if user == "" {
		return nil, fmt.Errorf("user flag is required")
	}

	presence, err := client.GetUserPresenceContext(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("users.getPresence: %v", err)
	}

	return presence, nil
}

// dispatchSetUserPresenceImpl implements users.setPresence.
func dispatchSetUserPresenceImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	presence := flagStr(flags, "presence")
	if presence == "" {
		return nil, fmt.Errorf("presence flag is required")
	}
	if presence != "auto" && presence != "away" {
		return nil, fmt.Errorf("presence must be %q or %q, got %q", "auto", "away", presence)
	}

	if err := client.SetUserPresenceContext(ctx, presence); err != nil {
		return nil, fmt.Errorf("users.setPresence: %v", err)
	}

	return map[string]string{"ok": "true"}, nil
}

// dispatchGetUserProfileImpl implements users.profile.get.
func dispatchGetUserProfileImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := &slack.GetUserProfileParameters{
		UserID:        flagStr(flags, "user"),
		IncludeLabels: flagBool(flags, "include-labels"),
	}

	profile, err := client.GetUserProfileContext(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("users.profile.get: %v", err)
	}

	return profile, nil
}

// dispatchGetUserByEmailImpl implements users.lookupByEmail.
func dispatchGetUserByEmailImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	email := flagStr(flags, "email")
	if email == "" {
		return nil, fmt.Errorf("email flag is required")
	}

	u, err := client.GetUserByEmailContext(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("users.lookupByEmail: %v", err)
	}

	return u, nil
}

func init() {
	RegisterDispatch("users.info", dispatchGetUserInfoImpl)
	RegisterDispatch("users.list", dispatchGetUsersImpl)
	RegisterDispatch("users.getPresence", dispatchGetUserPresenceImpl)
	RegisterDispatch("users.setPresence", dispatchSetUserPresenceImpl)
	RegisterDispatch("users.profile.get", dispatchGetUserProfileImpl)
	RegisterDispatch("users.lookupByEmail", dispatchGetUserByEmailImpl)
}
