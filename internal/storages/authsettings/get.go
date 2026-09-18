package authsettings

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// GetByMethod reads one method's row.
//
// A missing row is ErrAuthMethodNotFound and carries the name, which covers two
// different faults with one answer on purpose: a method outside the closed set
// was asked for, or a seeded row went missing. Neither has a safe default --
// guessing "enabled" reopens a path an admin closed, guessing "disabled" takes
// one away -- so both are reported rather than resolved here.
func (s *Store) GetByMethod(
	ctx context.Context, method entity.AuthMethodName,
) (*entity.AuthMethodSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthSettings.GetByMethod")
	defer span.End()

	stmt := table.AuthSettings.
		SELECT(table.AuthSettings.AllColumns).
		WHERE(table.AuthSettings.Method.EQ(postgres.String(string(method))))

	dbModel := new(model.AuthSettings)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), dbModel); err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, fmt.Errorf("%w: %q", apperr.ErrAuthMethodNotFound, method)
		}

		return nil, err
	}

	return fromDB(dbModel), nil
}
