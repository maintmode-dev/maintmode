package httperrors

import (
	"bytes"
	"encoding/json"
)

type ErrorCode string

var (
	ErrUnauthorized ErrorCode = "unauthorized"
	ErrLockBusy     ErrorCode = "lock is already held"
	ErrForbidden    ErrorCode = "forbidden"

	ErrNotFound                     ErrorCode = "not found"
	ErrInternalError                ErrorCode = "internal error"
	ErrServiceUnavailable           ErrorCode = "service unavailable"
	ErrInvalidRequest               ErrorCode = "invalid request"
	ErrConflictsChangedSincePreview ErrorCode = "conflicts changed since preview"
	ErrMaintChangedSincePreview     ErrorCode = "maintenance changed since preview"
	ErrResourceAlreadyExists        ErrorCode = "resource already exists"
	ErrNotifyChannelAlreadyExists   ErrorCode = "notify channel already exists"
	ErrForbiddenStatusTransition    ErrorCode = "forbidden status"
	ErrStepsNotTerminal             ErrorCode = "steps not terminal"
	ErrConflict                     ErrorCode = "conflict"
	ErrConcurrentModification       ErrorCode = "concurrent modification"

	// Invitation accept codes. The accept endpoint must not leak which
	// precondition failed (privacy), so these are returned with no message.
	ErrInvitationInvalid ErrorCode = "invalid"
	ErrEmailMismatch     ErrorCode = "email_mismatch"

	// License enforcement codes. Stable machine-readable contract for
	// the frontend: organization_suspended drives the full-screen suspended page
	//, seats_limit_exceeded surfaces on invite/activate over the cap.
	ErrSeatsLimitExceeded    ErrorCode = "seats_limit_exceeded"
	ErrOrganizationSuspended ErrorCode = "organization_suspended"

	// ErrSignupDisabled is the stable machine-readable code for an OAuth login
	// of an unknown user rejected by the signup policy (invite-only, no open
	// signup). Returned with a generic message only — the response must not
	// reveal whether an invitation exists for the email.
	ErrSignupDisabled ErrorCode = "signup_disabled"

	// ErrOTPSessionMismatch is the ONE failure of the one-time-code verify
	// endpoint that does not collapse into the generic unauthorized answer.
	//
	// The exception is safe because provoking it requires already holding a live
	// code, so it says nothing about whether an account exists. It is worth
	// having because the legitimate cause is a user who closed the tab while the
	// mail was in flight: told "invalid code" they would retype a correct code
	// against a nonce that no longer exists, forever. Told to request a new one,
	// they recover in a step.
	ErrOTPSessionMismatch ErrorCode = "otp_session_mismatch"
)

type ErrorResponse struct {
	Code    string `json:"code" example:"error code"`
	Message string `json:"message" example:"error message"`
}

func NewErrorResponse(code ErrorCode, message string) *ErrorResponse {
	return &ErrorResponse{
		Code:    string(code),
		Message: message,
	}
}

func (e *ErrorResponse) JSON() string {
	buf := new(bytes.Buffer)
	_ = json.NewEncoder(buf).Encode(e)

	return buf.String()
}
