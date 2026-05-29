package dbtx

import (
	"errors"

	"github.com/lib/pq"
)

// ErrPGUniqueViolation is the PostgreSQL SQLSTATE for unique_violation.
// See https://www.postgresql.org/docs/current/errcodes-appendix.html.
const ErrPGUniqueViolation = pq.ErrorCode("23505")

func ErrorIs(err error, target pq.ErrorCode) bool {
	var pqErr *pq.Error

	return errors.As(err, &pqErr) && pqErr.Code == target
}
