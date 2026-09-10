package httperrors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/apperr"
)

func ToAPIError(c *echo.Context, operation string, err error) error {
	var (
		statusCode int
		errResp    *ErrorResponse
	)

	switch {
	case err == nil:
		statusCode, errResp = http.StatusInternalServerError, NewErrorResponse(ErrInternalError, "unknown error")
	// maint domain errors
	case errors.Is(err, apperr.ErrMaintNotFound),
		errors.Is(err, apperr.ErrUserNotFound),
		errors.Is(err, apperr.ErrResourceNotFound),
		errors.Is(err, apperr.ErrForbiddenMaintStatusTransition),
		errors.Is(err, apperr.ErrConflictsChangedSincePreview),
		errors.Is(err, apperr.ErrMaintChangedSincePreview),
		errors.Is(err, apperr.ErrConcurrentModification),
		errors.Is(err, apperr.ErrResourceAlreadyExists),
		errors.Is(err, apperr.ErrNotifyChannelAlreadyExists),
		errors.Is(err, apperr.ErrNotifyChannelNotFound),
		errors.Is(err, apperr.ErrStepNotFound),
		errors.Is(err, apperr.ErrStepOrderViolation),
		errors.Is(err, apperr.ErrForbiddenStepStatusTransition),
		errors.Is(err, apperr.ErrMaintenanceHasUnfinishedSteps),
		errors.Is(err, apperr.ErrInvalidRole),
		errors.Is(err, apperr.ErrIntegrationNotFound),
		errors.Is(err, apperr.ErrIntegrationConflict):
		statusCode, errResp = mapError(err)
	// auth domain errors
	case errors.Is(err, apperr.ErrLockBusy),
		errors.Is(err, apperr.ErrTokenReuse),
		errors.Is(err, apperr.ErrRefreshTokenNotFound),
		errors.Is(err, apperr.ErrInvalidAccessToken),
		errors.Is(err, apperr.ErrInvalidRefreshToken),
		errors.Is(err, apperr.ErrSuspiciousActivity),
		errors.Is(err, apperr.ErrTokenExpired),
		errors.Is(err, apperr.ErrLogoutAlready),
		errors.Is(err, apperr.ErrUserBlocked),
		errors.Is(err, apperr.ErrUnsupportedProvider),
		errors.Is(err, apperr.ErrAuthUnavailable),
		errors.Is(err, apperr.ErrProviderAlreadyConnected),
		errors.Is(err, apperr.ErrProviderLinkedToAnotherUser),
		errors.Is(err, apperr.ErrCannotDisconnectLastProvider),
		errors.Is(err, apperr.ErrInvitationNotFound),
		errors.Is(err, apperr.ErrInvitationNotPending),
		errors.Is(err, apperr.ErrInvitationExpired),
		errors.Is(err, apperr.ErrUserAlreadyExists),
		errors.Is(err, apperr.ErrActivePendingExists),
		errors.Is(err, apperr.ErrInvalidCredentials):
		statusCode, errResp = mapAuthError(err)

	// Invitation accept failures: surface only the status code, never the
	// wrapped message — a token-link holder must not learn which precondition
	// failed. Checked before the generic ErrValidation case below (both wrap it).
	// 400 rather than 403, matching every neighboring outcome in this family
	// (ErrInvalidAccessToken, ErrEmailMismatch): the request carried a token
	// that cannot establish who is asking, which is a problem with what was
	// sent, not with what the caller may do. The message names the cause so an
	// operator reading a support ticket can tell it from a generic auth failure.
	case errors.Is(err, apperr.ErrEmailNotVerified):
		statusCode, errResp = http.StatusBadRequest,
			NewErrorResponse(ErrEmailNotVerified, "the identity provider reports this email as unverified")

	case errors.Is(err, apperr.ErrEmailMismatch):
		statusCode, errResp = http.StatusBadRequest, NewErrorResponse(ErrEmailMismatch, "")
	case errors.Is(err, apperr.ErrInvalidInvitation):
		statusCode, errResp = http.StatusBadRequest, NewErrorResponse(ErrInvitationInvalid, "")

	// license enforcement: stable 403 codes, checked before the
	// generic ErrForbidden case so the frontend can distinguish them.
	case errors.Is(err, apperr.ErrSeatsLimitExceeded):
		statusCode, errResp = http.StatusForbidden, NewErrorResponse(ErrSeatsLimitExceeded, err.Error())
	case errors.Is(err, apperr.ErrOrganizationSuspended):
		statusCode, errResp = http.StatusForbidden, NewErrorResponse(ErrOrganizationSuspended, err.Error())

	// A failed integration probe is an upstream failure, not ours: 502, and the
	// far end's own text passes through. Handled here rather than in mapError
	// because that helper is only reached from the allow-list above, and an
	// unlisted sentinel would fall to the default arm and lose the detail.
	case errors.Is(err, apperr.ErrIntegrationProbeFailed):
		statusCode, errResp = http.StatusBadGateway, NewErrorResponse(ErrIntegrationProbeFailed, err.Error())

	// signup policy: stable 403 code, checked before the generic ErrForbidden
	// case. The message is a fixed generic string (not err.Error()) so wrapped
	// context can never leak whether an invitation exists for the email.
	case errors.Is(err, apperr.ErrSignupDisabled):
		statusCode, errResp = http.StatusForbidden, NewErrorResponse(ErrSignupDisabled, "signup is disabled")

	// common errors. check after specific domain errors
	case errors.Is(err, apperr.ErrValidation):
		statusCode, errResp = http.StatusBadRequest, NewErrorResponse(ErrInvalidRequest, err.Error())
	case errors.Is(err, apperr.ErrForbidden):
		statusCode, errResp = http.StatusForbidden, NewErrorResponse(ErrForbidden, err.Error())
	case errors.Is(err, apperr.ErrMethodNotAllowedInProd):
		statusCode, errResp = http.StatusMethodNotAllowed, NewErrorResponse(ErrServiceUnavailable, err.Error())
	default:
		// For any other error, return internal server error with operation context
		statusCode, errResp = http.StatusInternalServerError, NewErrorResponse(ErrInternalError,
			fmt.Sprintf("%s failed", operation),
		)
	}

	return c.JSON(statusCode, errResp)
}

// mapError maps domain errors to HTTP responses
// Returns the HTTP status code and ErrorResponse for the given error
func mapError(err error) (int, *ErrorResponse) {
	if err == nil {
		return http.StatusInternalServerError, NewErrorResponse(ErrInternalError, "unknown error")
	}

	// Check for specific domain errors
	switch {
	case errors.Is(err, apperr.ErrMaintNotFound),
		errors.Is(err, apperr.ErrUserNotFound),
		errors.Is(err, apperr.ErrResourceNotFound),
		errors.Is(err, apperr.ErrNotifyChannelNotFound),
		errors.Is(err, apperr.ErrIntegrationNotFound):
		return http.StatusNotFound, NewErrorResponse(ErrNotFound, err.Error())

	case errors.Is(err, apperr.ErrIntegrationConflict):
		return http.StatusConflict, NewErrorResponse(ErrConflict, err.Error())

	case errors.Is(err, apperr.ErrForbiddenMaintStatusTransition):
		return http.StatusConflict, NewErrorResponse(ErrForbiddenStatusTransition, err.Error())

	case errors.Is(err, apperr.ErrConflictsChangedSincePreview):
		return http.StatusConflict, NewErrorResponse(ErrConflictsChangedSincePreview, err.Error())

	case errors.Is(err, apperr.ErrMaintChangedSincePreview):
		return http.StatusConflict, NewErrorResponse(ErrMaintChangedSincePreview, err.Error())

	case errors.Is(err, apperr.ErrConcurrentModification):
		return http.StatusConflict, NewErrorResponse(ErrConcurrentModification, err.Error())

	case errors.Is(err, apperr.ErrResourceAlreadyExists):
		return http.StatusConflict, NewErrorResponse(ErrResourceAlreadyExists, err.Error())

	case errors.Is(err, apperr.ErrNotifyChannelAlreadyExists):
		return http.StatusConflict, NewErrorResponse(ErrNotifyChannelAlreadyExists, err.Error())

	case errors.Is(err, apperr.ErrStepNotFound):
		return http.StatusNotFound, NewErrorResponse(ErrNotFound, err.Error())

	case errors.Is(err, apperr.ErrMaintenanceHasUnfinishedSteps):
		return http.StatusConflict, NewErrorResponse(ErrStepsNotTerminal, err.Error())

	case errors.Is(err, apperr.ErrStepOrderViolation),
		errors.Is(err, apperr.ErrForbiddenStepStatusTransition):
		return http.StatusConflict, NewErrorResponse(ErrForbiddenStatusTransition, err.Error())

	case errors.Is(err, apperr.ErrInvalidRole):
		return http.StatusBadRequest, NewErrorResponse(ErrInvalidRequest, err.Error())

	default:
		// For any other error, return internal server error
		return http.StatusInternalServerError, NewErrorResponse(ErrInternalError, "internal server error")
	}
}

// mapAuthError maps domain errors to HTTP responses
// Returns the HTTP status code and ErrorResponse for the given error
func mapAuthError(err error) (int, *ErrorResponse) {
	if err == nil {
		return http.StatusInternalServerError, NewErrorResponse(ErrInternalError, "unknown error")
	}

	// Check for specific domain errors
	switch {
	case errors.Is(err, apperr.ErrTokenReuse),
		errors.Is(err, apperr.ErrInvalidAccessToken),
		errors.Is(err, apperr.ErrInvalidRefreshToken),
		errors.Is(err, apperr.ErrRefreshTokenNotFound),
		errors.Is(err, apperr.ErrTokenExpired),
		errors.Is(err, apperr.ErrLogoutAlready),
		errors.Is(err, apperr.ErrUserBlocked),
		errors.Is(err, apperr.ErrSuspiciousActivity),
		// A rejected credential is a 401. Without this arm it fell through to
		// the default and answered 500 -- POST /me/password with the wrong
		// current password returned an internal error, which a client cannot
		// classify and which reads as a server fault rather than a refusal.
		//
		// The login routes never noticed: they answer failures themselves and
		// deliberately bypass this mapper to keep every rejection identical.
		errors.Is(err, apperr.ErrInvalidCredentials):
		return http.StatusUnauthorized, NewErrorResponse(ErrUnauthorized, err.Error())
	case errors.Is(err, apperr.ErrLockBusy):
		return http.StatusTooManyRequests, NewErrorResponse(ErrLockBusy, err.Error())
	case errors.Is(err, apperr.ErrAuthUnavailable):
		return http.StatusServiceUnavailable, NewErrorResponse(ErrServiceUnavailable, err.Error())

	case errors.Is(err, apperr.ErrUnsupportedProvider),
		errors.Is(err, apperr.ErrCannotDisconnectLastProvider):
		return http.StatusBadRequest, NewErrorResponse(ErrInvalidRequest, err.Error())

	case errors.Is(err, apperr.ErrProviderAlreadyConnected),
		errors.Is(err, apperr.ErrProviderLinkedToAnotherUser):
		return http.StatusConflict, NewErrorResponse(ErrConflict, err.Error())

	case errors.Is(err, apperr.ErrInvitationNotFound):
		return http.StatusNotFound, NewErrorResponse(ErrNotFound, err.Error())

	case errors.Is(err, apperr.ErrUserAlreadyExists),
		errors.Is(err, apperr.ErrActivePendingExists),
		errors.Is(err, apperr.ErrInvitationNotPending),
		errors.Is(err, apperr.ErrInvitationExpired):
		return http.StatusConflict, NewErrorResponse(ErrConflict, err.Error())

	default:
		// For any other error, return internal server error
		return http.StatusInternalServerError, NewErrorResponse(ErrInternalError, "internal server error")
	}
}
