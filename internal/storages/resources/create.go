package resources

import (
	"context"
	"errors"

	"github.com/lib/pq"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// pgUniqueViolation is the PostgreSQL SQLSTATE for unique_violation.
// See https://www.postgresql.org/docs/current/errcodes-appendix.html.
const pgUniqueViolation = pq.ErrorCode("23505")

func (s *Store) Create(ctx context.Context, r *entity.ResourceDetails) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Resources.Create")
	defer span.End()

	resource := toDBResource(r)

	stmt := table.Resources.
		INSERT(table.Resources.MutableColumns.
			Except(table.Resources.CreatedAt),
		).
		MODEL(resource).
		RETURNING(table.Resources.AllColumns)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), resource)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pgUniqueViolation {
			return nil, apperr.ErrResourceAlreadyExists
		}
		return nil, err
	}
	return fromDBResource(resource), nil
}
