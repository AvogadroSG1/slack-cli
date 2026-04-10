// Package main provides the code generator that introspects the slack-go/slack
// SDK and emits registry and dispatch source files.
package main

// methodMapping maps Go method base names (without the "Context" suffix) to
// their corresponding Slack API method names. Only methods present in this
// table are included in the generated output; all others are silently skipped.
// This is intentional: we add API surface incrementally rather than exposing
// every SDK method at once.
var methodMapping = map[string]string{
	// auth
	"AuthTest": "auth.test",

	// chat
	"PostMessage":     "chat.postMessage",
	"PostEphemeral":   "chat.postEphemeral",
	"UpdateMessage":   "chat.update",
	"DeleteMessage":   "chat.delete",
	"GetPermalink":    "chat.getPermalink",
	"ScheduleMessage": "chat.scheduleMessage",

	// conversations
	"GetConversations":         "conversations.list",
	"GetConversationHistory":   "conversations.history",
	"GetConversationInfo":      "conversations.info",
	"GetConversationReplies":   "conversations.replies",
	"CreateConversation":       "conversations.create",
	"ArchiveConversation":      "conversations.archive",
	"UnArchiveConversation":    "conversations.unarchive",
	"InviteUsersToConversation": "conversations.invite",
	"KickUserFromConversation": "conversations.kick",
	"LeaveConversation":        "conversations.leave",
	"JoinConversation":         "conversations.join",
	"SetPurposeOfConversation": "conversations.setPurpose",
	"SetTopicOfConversation":   "conversations.setTopic",
	"OpenConversation":         "conversations.open",
	"CloseConversation":        "conversations.close",
	"RenameConversation":       "conversations.rename",
	"MarkConversation":         "conversations.mark",
	"GetUsersInConversation":   "conversations.members",

	// users
	"GetUserInfo":        "users.info",
	"GetUsers":           "users.list",
	"GetUserPresence":    "users.getPresence",
	"SetUserPresence":    "users.setPresence",
	"GetUserProfile":     "users.profile.get",
	"LookupUserByEmail":  "users.lookupByEmail",

	// reactions
	"AddReaction":    "reactions.add",
	"RemoveReaction": "reactions.remove",
	"GetReactions":   "reactions.get",
	"ListReactions":  "reactions.list",

	// search
	"SearchMessages": "search.messages",
	"SearchFiles":    "search.files",

	// files
	"GetFiles":             "files.list",
	"GetFileInfo":          "files.info",
	"DeleteFile":           "files.delete",
	"ShareFilePublicURL":   "files.sharedPublicURL",
	"RevokeFilePublicURL":  "files.revokePublicURL",

	// pins
	"AddPin":    "pins.add",
	"RemovePin": "pins.remove",
	"ListPins":  "pins.list",

	// stars
	"AddStar":    "stars.add",
	"RemoveStar": "stars.remove",
	"ListStars":  "stars.list",

	// team
	"GetTeamInfo": "team.info",

	// emoji
	"GetEmoji": "emoji.list",

	// usergroups
	"CreateUserGroup":        "usergroups.create",
	"DisableUserGroup":       "usergroups.disable",
	"EnableUserGroup":        "usergroups.enable",
	"GetUserGroups":          "usergroups.list",
	"UpdateUserGroup":        "usergroups.update",
	"GetUserGroupMembers":    "usergroups.users.list",
	"UpdateUserGroupMembers": "usergroups.users.update",

	// views
	"OpenView":    "views.open",
	"PushView":    "views.push",
	"UpdateView":  "views.update",
	"PublishView": "views.publish",

	// dialog
	"OpenDialog": "dialog.open",

	// dnd
	"EndDND":         "dnd.endDnd",
	"EndSnooze":      "dnd.endSnooze",
	"GetDNDInfo":     "dnd.info",
	"SetSnooze":      "dnd.setSnooze",
	"GetDNDTeamInfo": "dnd.teamInfo",

	// bookmarks
	"AddBookmark":    "bookmarks.add",
	"EditBookmark":   "bookmarks.edit",
	"ListBookmarks":  "bookmarks.list",
	"RemoveBookmark": "bookmarks.remove",

	// bots
	"GetBotInfo": "bots.info",

	// reminders
	"AddReminder":      "reminders.add",
	"DeleteReminder":   "reminders.delete",
	"ListReminders":    "reminders.list",
	"CompleteReminder": "reminders.complete",
}
