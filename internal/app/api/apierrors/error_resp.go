package apierrors

type ErrorCode string

var (
	ErrUnauthorized ErrorCode = "unauthorized"
	ErrLockBusy     ErrorCode = "lock is already held"

	ErrNotFound                     ErrorCode = "not found"
	ErrInternalError                ErrorCode = "internal error"
	ErrInvalidRequest               ErrorCode = "invalid request"
	ErrConflictsChangedSincePreview ErrorCode = "conflicts changed since preview"
	ErrMaintChangedSincePreview     ErrorCode = "maintenance changed since preview"
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
