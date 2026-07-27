package apimodels

import "encoding/json"

// Field names of UpdateUserTagsRequest as they appear on the wire. Used as the
// keys of the presence set, so the JSON tags and these constants must stay in
// step.
//
// These are deliberately this package's own constants, not borrowed from the
// PATCH /me request or from the audit diff: there they name a self-service body
// and an audit field respectively. The strings coincide today; the meanings do
// not, and a shared constant would tie three contracts to one edit.
const (
	FieldTelegramTag = "telegram_tag"
	FieldSlackTag    = "slack_tag"
)

// UpdateUserTagsRequest is the body of PATCH /api/v1/users/{id}, an admin's edit
// of another user's messenger handles. It is a true patch: only the keys
// actually present in the JSON object are applied.
//
// A pointer alone cannot express that, because null and "absent" both decode to
// nil while meaning opposite things — null clears the handle, absent leaves it
// alone. Presence is therefore tracked separately, in a set built by
// UnmarshalJSON, and queried through Has.
//
// Timezone is deliberately not part of this contract. It only affects how the
// owner sees their own interface, so an admin editing it would break someone's
// display while leaving notification delivery — the thing the admin is
// accountable for — untouched. Reusing the self-service request type here would
// have accepted the key and silently dropped it.
//
// Both tags carry messenger handles, stored exactly as entered; null, empty or
// whitespace clears them. Validation lives in the entity layer and the user
// service, not here.
type UpdateUserTagsRequest struct {
	TelegramTag *string `json:"telegram_tag"`
	SlackTag    *string `json:"slack_tag"`

	// present holds the keys that were physically in the request body. It is
	// unexported and untagged so it stays out of the generated OpenAPI schema,
	// which must expose exactly the two properties above.
	present map[string]bool
}

// UnmarshalJSON decodes the request and records which keys were present.
func (r *UpdateUserTagsRequest) UnmarshalJSON(data []byte) error {
	// The shadow type is mandatory: unmarshalling into *UpdateUserTagsRequest
	// here would call this method again, forever.
	type alias UpdateUserTagsRequest
	if err := json.Unmarshal(data, (*alias)(r)); err != nil {
		return err
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}

	r.present = make(map[string]bool, len(keys))
	for key := range keys {
		r.present[key] = true
	}

	return nil
}

// Has reports whether the named field was present in the request body.
//
// A nil presence set answers false for every field, which is exactly right for
// the two ways it can stay nil: a body of "{}" (unmarshalled, no keys) and a
// request with no body at all. Echo returns from BindBody before deserializing
// when Content-Length is 0, so UnmarshalJSON never runs in the second case —
// the two requests take different paths and must produce the same outcome:
// nothing is touched.
func (r *UpdateUserTagsRequest) Has(field string) bool {
	return r.present[field]
}
