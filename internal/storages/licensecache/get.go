package licensecache

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// Get reads the cached license. The migration seeds a default active row, so an
// empty table is not expected in normal operation; it maps to
// ErrLicenseCacheEmpty, which the service treats as fail-open (non-blocking).
func (s *Store) Get(ctx context.Context) (*entity.License, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.LicenseCache.Get")
	defer span.End()

	stmt := table.LicenseCache.
		SELECT(table.LicenseCache.AllColumns)

	row := new(model.LicenseCache)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), row); err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrLicenseCacheEmpty
		}
		return nil, err
	}
	return fromDB(row), nil
}
