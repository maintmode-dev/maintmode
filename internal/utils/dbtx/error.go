package dbtx

import (
	"errors"

	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

// ErrPGUniqueViolation is the PostgreSQL SQLSTATE for unique_violation.
// See https://www.postgresql.org/docs/current/errcodes-appendix.html.
const ErrPGUniqueViolation = pqerror.Code("23505")

func ErrorIs(err error, target pqerror.Code) bool {
	var pqErr *pq.Error

	return errors.As(err, &pqErr) && pqErr.Code == target
}
