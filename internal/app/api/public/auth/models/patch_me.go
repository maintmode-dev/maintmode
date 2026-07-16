package apiauthmodels

// UpdateMeRequest is the body of PATCH /api/v1/me.
//
// Timezone carries the user's preferred IANA timezone (e.g. "Asia/Nicosia").
// null (or an omitted/empty value) resets the preference to auto-detect. An
// invalid IANA identifier is rejected with HTTP 400.
type UpdateMeRequest struct {
	Timezone *string `json:"timezone"`
}
