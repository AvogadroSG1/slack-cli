package override

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/validate"
)

// targetKind identifies what an argument resolved to.
type targetKind int

const (
	targetChannel targetKind = iota
	targetUser
	targetUsergroup
)

// usergroupIDRe matches Slack usergroup (subteam) IDs.
var usergroupIDRe = regexp.MustCompile(`^S[A-Z0-9]{8,}$`)

// cacheMissHint wraps a cache lookup error with an actionable suggestion.
func cacheMissHint(err error) error {
	return fmt.Errorf("%w (if this name should exist, run \"slack-cli cache warm\" to refresh the cache)", err)
}

// resolveChannelArg accepts "#name", a bare name, or a raw C/G/D… ID and
// returns a channel ID. Raw IDs pass through without touching the cache.
func resolveChannelArg(arg string) (string, error) {
	name := strings.TrimPrefix(arg, "#")
	if !strings.HasPrefix(arg, "#") && validate.ChannelID(arg) == nil {
		return arg, nil
	}
	lock, err := cache.AcquireShared()
	if err != nil {
		return "", err
	}
	defer lock.Close()
	id, err := cache.ResolveChannel(name)
	if err != nil {
		return "", cacheMissHint(err)
	}
	return id, nil
}

// resolveUserArg accepts "@name", a bare name, or a raw U/W… ID and returns
// a user ID. Raw IDs pass through without touching the cache.
func resolveUserArg(arg string) (string, error) {
	name := strings.TrimPrefix(arg, "@")
	if !strings.HasPrefix(arg, "@") && validate.UserID(arg) == nil {
		return arg, nil
	}
	lock, err := cache.AcquireShared()
	if err != nil {
		return "", err
	}
	defer lock.Close()
	id, err := cache.ResolveUser(name, "id")
	if err != nil {
		return "", cacheMissHint(err)
	}
	return id, nil
}

// resolveTarget auto-detects what arg refers to: a '#' prefix or channel ID
// forces channel, an '@' prefix or user ID forces user, a usergroup ID
// forces usergroup. Bare names try the channel cache first, then users,
// then usergroups.
func resolveTarget(arg string) (id string, kind targetKind, err error) {
	switch {
	case strings.HasPrefix(arg, "#"):
		id, err = resolveChannelArg(arg)
		return id, targetChannel, err
	case strings.HasPrefix(arg, "@"):
		id, err = resolveUserArg(arg)
		return id, targetUser, err
	case validate.ChannelID(arg) == nil:
		return arg, targetChannel, nil
	case validate.UserID(arg) == nil:
		return arg, targetUser, nil
	case usergroupIDRe.MatchString(arg):
		return arg, targetUsergroup, nil
	}

	lock, err := cache.AcquireShared()
	if err != nil {
		return "", targetChannel, err
	}
	defer lock.Close()

	if id, err := cache.ResolveChannel(arg); err == nil {
		return id, targetChannel, nil
	}
	if id, err := cache.ResolveUser(arg, "id"); err == nil {
		return id, targetUser, nil
	}
	if id, err := cache.ResolveUsergroup(arg, "id"); err == nil {
		return id, targetUsergroup, nil
	}
	return "", targetChannel, cacheMissHint(fmt.Errorf("no channel, user, or usergroup named %q", arg))
}
