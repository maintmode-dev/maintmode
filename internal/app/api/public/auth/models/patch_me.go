package apiauthmodels

import "encoding/json"

// Field names of UpdateMeRequest as they appear on the wire. Used as the keys of
// the presence set, so the JSON tags and these constants must stay in step.
const (
	FieldTimezone    = "timezone"
	FieldTelegramTag = "telegram_tag"
	FieldSlackTag    = "slack_tag"
)

// UpdateMeRequest is the body of PATCH /api/v1/me. It is a true patch: only the
// keys actually present in the JSON object are applied.
//
// A pointer alone cannot express that, because null and "absent" both decode to
// nil while meaning opposite things — null resets the preference to NULL, absent
// leaves it alone. Presence is therefore tracked separately, in a set built by
// UnmarshalJSON, and queried through Has.
//
// Timezone carries an IANA identifier (e.g. "Asia/Nicosia"); null, empty or
// whitespace resets it to browser auto-detect. TelegramTag and SlackTag carry
// messenger handles, stored exactly as entered; null or empty clears them.
// Validation lives in the entity layer and the user service, not here.
type UpdateMeRequest struct {
	Timezone    *string `json:"timezone"`
	TelegramTag *string `json:"telegram_tag"`
	SlackTag    *string `json:"slack_tag"`

	// present holds the keys that were physically in the request body. It is
	// unexported and untagged so it stays out of the generated OpenAPI schema,
	// which must expose exactly the three properties above.
	present map[string]bool
}

// UnmarshalJSON decodes the request and records which keys were present.
func (r *UpdateMeRequest) UnmarshalJSON(data []byte) error {
	// The shadow type is mandatory: unmarshalling into *UpdateMeRequest here
	// would call this method again, forever.
	type alias UpdateMeRequest
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
func (r *UpdateMeRequest) Has(field string) bool {
	return r.present[field]
}
