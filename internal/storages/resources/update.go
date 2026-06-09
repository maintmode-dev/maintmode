package resources

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Update persists the given resource and returns the stored row. The caller is
// expected to pass a fully merged entity (read-modify-write); Update sets
// updated_at to now and writes all mutable columns.
func (s *Store) Update(ctx context.Context, r *entity.ResourceDetails) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Resources.Update")
	defer span.End()

	r.UpdatedAt = lo.ToPtr(xtime.UTCNow())

	resource := toDBResource(r)

	// Write only the editable columns. Status is owned by the archive/unarchive
	// endpoints, so a full MutableColumns write would risk clobbering a
	// concurrent archive (lost update). CreatedAt / CreatedByUserID are likewise
	// left untouched so the author survives edits.
	stmt := table.Resources.
		UPDATE(
			table.Resources.Name,
			table.Resources.Description,
			table.Resources.ExternalID,
			table.Resources.UpdatedAt,
			table.Resources.UpdatedByUserID,
		).
		MODEL(resource).
		WHERE(table.Resources.ID.EQ(postgres.UUID(r.ID))).
		RETURNING(table.Resources.AllColumns)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), resource)
	if err != nil {
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrResourceAlreadyExists
		}
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrResourceNotFound
		}
		return nil, err
	}

	return fromDBResource(resource), nil
}
