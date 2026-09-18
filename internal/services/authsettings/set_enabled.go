package authsettings

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// SetEnabled sets one method's flag to the requested value.
//
// SetEnabled rather than Toggle: the caller states the target state, so a
// retried request converges on it instead of flipping back. The state is
// idempotent; the audit record is not, and that split is intended -- a second
// PATCH leaves a second row, which is what "who did what, when" requires.
func (s *Service) SetEnabled(
	ctx context.Context,
	cmd *entity.SetAuthMethodEnabledCmd,
) (*entity.AuthMethodSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AuthSettings.SetEnabled")
	defer span.End()

	// The actor is obligatory, not optional. Every caller is an authenticated
	// admin today, so this cannot fire -- but the alternative to checking is a
	// nil dereference in the audit record and the log line below, which would
	// turn a wiring mistake into a panic on a request that otherwise succeeded.
	if cmd.Actor == nil {
		return nil, fmt.Errorf("%w: setting a sign-in method requires an actor", apperr.ErrValidation)
	}

	if !cmd.Method.IsValid() {
		return nil, fmt.Errorf("%w: %q", apperr.ErrAuthMethodNotFound, cmd.Method)
	}

	var updated *entity.AuthMethodSetting

	// A transaction around a single-row write, which is not redundant: the read
	// resolves the row's id and the write addresses it, so without one a row
	// deleted in between turns a successful-looking call into a silent no-op.
	//
	// No row locking, and the read is of ONE row rather than the table. Both
	// used to be otherwise, for a guard that refused to let an admin turn off
	// the last enabled method: it counted the other rows, so it had to read them
	// and lock them. That guard is gone -- an instance with every method off is
	// a configuration an admin may choose, recoverable through break-glass and
	// through the admin's own still-valid session -- and with it the only reason
	// this ever touched a row it was not writing.
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		target, err := s.store.GetByMethod(ctx, cmd.Method)
		if err != nil {
			return err
		}

		target.Enabled = cmd.Enabled
		target.UpdatedByUserID = &cmd.Actor.ID

		updated, err = s.store.Update(ctx, target)
		if err != nil {
			return fmt.Errorf("update auth setting: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Published after the transaction commits, never inside it: the outbox write
	// belongs to a committed change, and a row audited for a change that rolled
	// back would be worse than a missing one.
	if err = s.publishAudit(ctx, audit.AuthMethodToggled{
		Actor:   cmd.Actor,
		Method:  cmd.Method,
		Enabled: cmd.Enabled,
	}); err != nil {
		// The change is committed; failing the request now would tell the admin
		// their toggle did not happen when it did. Logged loudly instead -- an
		// unaudited security change is a real gap, and this is the line that
		// reports it.
		xlog.Error(ctx, "auth method toggle was not audited",
			xfield.String("method", string(cmd.Method)),
			xfield.Error(err),
		)
	}

	// Logged synchronously and in the container log, not only in the audit
	// trail. The audit trail is asynchronous and needs an admin session to
	// read, and the operator who most needs this line is the one correlating
	// "sign-ins started failing at 14:32" against a change -- which is exactly
	// the moment nobody can sign in to read the audit log.
	xlog.Info(ctx, "auth method toggled",
		xfield.String("method", string(cmd.Method)),
		xfield.Bool("enabled", cmd.Enabled),
		xfield.String("actor", cmd.Actor.Email),
	)

	return updated, nil
}
