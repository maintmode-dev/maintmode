package apierrors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ruko1202/maintmode/internal/apperr"
)

// ToAPIErrResponse maps service-level errors to HTTP responses with operation context
// Used when the error is related to a specific service operation
func ToAPIErrResponse(operation string, err error) (int, *ErrorResponse) {
	if err == nil {
		return http.StatusInternalServerError, NewErrorResponse(ErrInternalError, "unknown error")
	}

	// Check for specific domain errors first
	switch {
	//maint errors
	case errors.Is(err, apperr.ErrMaintNotFound),
		errors.Is(err, apperr.ErrResourceNotFound),
		errors.Is(err, apperr.ErrForbiddenMaintStatusTransition),
		errors.Is(err, apperr.ErrConflictsChangedSincePreview),
		errors.Is(err, apperr.ErrMaintChangedSincePreview),
		errors.Is(err, apperr.ErrResourceAlreadyExists),
		errors.Is(err, apperr.ErrStepNotFound),
		errors.Is(err, apperr.ErrValidation):
		return mapError(err)
	// auth errors
	case errors.Is(err, apperr.ErrLockBusy),
		errors.Is(err, apperr.ErrTokenReuse),
		errors.Is(err, apperr.ErrRefreshTokenNotFound),
		errors.Is(err, apperr.ErrInvalidAccessTokenToken),
		errors.Is(err, apperr.ErrInvalidRefreshTokenToken),
		errors.Is(err, apperr.ErrSuspiciousActivity),
		errors.Is(err, apperr.ErrTokenExpired),
		errors.Is(err, apperr.ErrLogoutAlready),
		errors.Is(err, apperr.ErrUnsupportedProvider),
		errors.Is(err, apperr.ErrInvalidOAuthState):
		return mapAuthError(err)

	default:
		// For any other error, return internal server error with operation context
		return http.StatusInternalServerError, NewErrorResponse(ErrInternalError, fmt.Sprintf("%s failed", operation))
	}
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
		errors.Is(err, apperr.ErrResourceNotFound):
		return http.StatusNotFound, NewErrorResponse(ErrNotFound, err.Error())

	case errors.Is(err, apperr.ErrForbiddenMaintStatusTransition):
		return http.StatusConflict, NewErrorResponse(ErrForbiddenStatusTransition, err.Error())

	case errors.Is(err, apperr.ErrConflictsChangedSincePreview):
		return http.StatusConflict, NewErrorResponse(ErrConflictsChangedSincePreview, err.Error())

	case errors.Is(err, apperr.ErrMaintChangedSincePreview):
		return http.StatusConflict, NewErrorResponse(ErrMaintChangedSincePreview, err.Error())

	case errors.Is(err, apperr.ErrResourceAlreadyExists):
		return http.StatusConflict, NewErrorResponse(ErrMaintChangedSincePreview, err.Error())

	case errors.Is(err, apperr.ErrStepNotFound):
		return http.StatusNotFound, NewErrorResponse(ErrNotFound, err.Error())

	case errors.Is(err, apperr.ErrStepOrderViolation),
		errors.Is(err, apperr.ErrForbiddenStepStatusTransition),
		errors.Is(err, apperr.ErrMaintenanceHasUnfinishedSteps):
		return http.StatusConflict, NewErrorResponse(ErrForbiddenStatusTransition, err.Error())

	case errors.Is(err, apperr.ErrValidation):
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
		errors.Is(err, apperr.ErrInvalidAccessTokenToken),
		errors.Is(err, apperr.ErrInvalidRefreshTokenToken),
		errors.Is(err, apperr.ErrRefreshTokenNotFound),
		errors.Is(err, apperr.ErrTokenExpired),
		errors.Is(err, apperr.ErrLogoutAlready),
		errors.Is(err, apperr.ErrSuspiciousActivity):
		return http.StatusUnauthorized, NewErrorResponse(ErrUnauthorized, err.Error())
	case errors.Is(err, apperr.ErrLockBusy):
		return http.StatusTooManyRequests, NewErrorResponse(ErrLockBusy, err.Error())

	case errors.Is(err, apperr.ErrValidation),
		errors.Is(err, apperr.ErrUnsupportedProvider),
		errors.Is(err, apperr.ErrInvalidOAuthState):
		return http.StatusBadRequest, NewErrorResponse(ErrInvalidRequest, err.Error())

	default:
		// For any other error, return internal server error
		return http.StatusInternalServerError, NewErrorResponse(ErrInternalError, "internal server error")
	}
}
