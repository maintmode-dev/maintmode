package apperr

import "errors"

// ErrDataKeyNotFound is returned when a data_keys row is expected but absent —
// e.g. an UpdateWrap targeting an id that was deleted concurrently.
var ErrDataKeyNotFound = errors.New("data key not found")
