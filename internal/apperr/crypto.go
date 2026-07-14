package apperr

import "errors"

var (
	// ErrDataKeyNotFound is returned when a data_keys row is expected but absent —
	// e.g. an UpdateWrap targeting an id that was deleted concurrently.
	ErrDataKeyNotFound = errors.New("data key not found")
	ErrUnwrapDEK       = errors.New("failed to unwrap DEK")
)
