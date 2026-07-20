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
		errors.Is(err, apperr.ErrInvalidOAuthState),
		errors.Is(err, apperr.ErrOAuthStateExpired),
		errors.Is(err, apperr.ErrOAuthStateTampered),
		errors.Is(err, apperr.ErrAuthUnavailable),
		errors.Is(err, apperr.ErrProviderAlreadyConnected),
		errors.Is(err, apperr.ErrProviderLinkedToAnotherUser),
		errors.Is(err, apperr.ErrCannotDisconnectLastProvider),
		errors.Is(err, apperr.ErrInvitationNotFound),
		errors.Is(err, apperr.ErrInvitationNotPending),
		errors.Is(err, apperr.ErrInvitationExpired),
		errors.Is(err, apperr.ErrUserAlreadyExists),
		errors.Is(err, apperr.ErrActivePendingExists):
		statusCode, errResp = mapAuthError(err)

	// Invitation accept failures: surface only the status code, never the
	// wrapped message — a token-link holder must not learn which precondition
	// failed. Checked before the generic ErrValidation case below (both wrap it).
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
		errors.Is(err, apperr.ErrSuspiciousActivity):
		return http.StatusUnauthorized, NewErrorResponse(ErrUnauthorized, err.Error())
	case errors.Is(err, apperr.ErrLockBusy):
		return http.StatusTooManyRequests, NewErrorResponse(ErrLockBusy, err.Error())
	case errors.Is(err, apperr.ErrAuthUnavailable):
		return http.StatusServiceUnavailable, NewErrorResponse(ErrServiceUnavailable, err.Error())

	case errors.Is(err, apperr.ErrUnsupportedProvider),
		errors.Is(err, apperr.ErrInvalidOAuthState),
		errors.Is(err, apperr.ErrOAuthStateExpired),
		errors.Is(err, apperr.ErrOAuthStateTampered),
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
