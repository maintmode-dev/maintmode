package entity

// UserMention carries everything a notification needs to address one person, in
// one value.
//
// Name is inside deliberately. Rendering a mention always needs a fallback: the
// tag may be absent (the common, switched-off state), rejected by the render-time
// sanitizer, or withheld because the user is blocked. In every one of those cases
// the message degrades to the display name — and the renderer must be able to do
// that without a second resolve round trip, which for a background reminder task
// would mean another auth call on a path where auth may well be unavailable. Name
// therefore already carries its own degraded forms: UnknownUserName when auth
// could not resolve the user at all, and "<name> [blocked]" for a blocked one.
//
// This is not UserSummary and must not be merged into it. UserSummary is the
// public authorship projection returned by read APIs (maintenance author, resource
// author, channel author) and carries ID and Email; messenger handles must not
// leak through it. UserMention is the opposite shape: no ID, no Email, private to
// the notification pipeline, and it exists only to be rendered into a message
// body.
type UserMention struct {
	Name string
	// The handles as the user typed them; nil when not provided.
	TelegramTag *string
	SlackTag    *string
}
