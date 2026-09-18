// Package models holds the wire shapes for the auth settings API.
package models

import "time"

// AuthMethodSetting is one built-in sign-in method's flag as the admin UI sees
// it.
//
// Authorship is deliberately absent. The column is stored, but "who last
// changed this" is already answered by the audit trail, where the actor is
// mandatory -- resolving it here too would be the same question answered twice,
// and it would pull a user lookup into an endpoint that otherwise touches one
// table.
type AuthMethodSetting struct {
	Method    string    `json:"method"    example:"email_otp"`
	Enabled   bool      `json:"enabled"   example:"true"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AuthMethodSettingsResponse lists every built-in method.
//
// An object rather than a bare array, matching the sign-in listing: a top-level
// array cannot grow a sibling field later without breaking every client.
type AuthMethodSettingsResponse struct {
	Methods []AuthMethodSetting `json:"methods"`
}

// SetAuthMethodEnabledRequest asks for one method's flag to be set.
//
// The target STATE, not a flip. A toggle that inverted whatever it found would
// turn a duplicate submit -- a double-click, a retried request -- into a silent
// re-enable of a method the admin had just closed.
type SetAuthMethodEnabledRequest struct {
	Enabled bool `json:"enabled" example:"false"`
}
