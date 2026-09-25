package dbtx

import (
	"errors"

	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

// ErrPGUniqueViolation is the PostgreSQL SQLSTATE for unique_violation.
// See https://www.postgresql.org/docs/current/errcodes-appendix.html.
const ErrPGUniqueViolation = pqerror.Code("23505")

// ErrPGSerializationFailure is the PostgreSQL SQLSTATE for
// serialization_failure, returned by SERIALIZABLE/REPEATABLE READ transactions
// that lost a write conflict. Such transactions are safe to retry from scratch.
const ErrPGSerializationFailure = pqerror.Code("40001")

// ErrPGForeignKeyViolation is the PostgreSQL SQLSTATE for
// foreign_key_violation. It reaches the delete path when a row is removed while
// something still references it -- under ON DELETE RESTRICT, which is checked
// immediately rather than deferred to commit.
const ErrPGForeignKeyViolation = pqerror.Code("23503")

func ErrorIs(err error, target pqerror.Code) bool {
	var pqErr *pq.Error

	return errors.As(err, &pqErr) && pqErr.Code == target
}
