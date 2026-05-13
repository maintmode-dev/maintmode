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
	ErrForbiddenStatusTransition    ErrorCode = "forbidden status"
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
