package integration

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Delete removes an integration, taking a login provider's linked identities
// with it.
//
// The identities go first and the row second, inside one transaction, and that
// order is now load-bearing rather than incidental: user_identities.integration_id
// references this row ON DELETE RESTRICT, and PostgreSQL checks RESTRICT
// immediately rather than deferring to commit. Removing the row first would
// fail on its own children even though the very next statement would have
// deleted them.
//
// RESTRICT does not reach the admin. By the time the row is deleted its
// identities are gone, so this answers 204 whether the provider carried one
// account or a hundred thousand -- the unsatisfiable refusal that once told an
// operator to unlink accounts only the account OWNER can unlink is not coming
// back. What RESTRICT catches is us: a cascade that stops running turns into a
// failing test instead of an orphaned row.
//
// Toggling off is deliberately not guarded, and unlike delete it is not
// destructive: it locks those accounts out just as thoroughly, but it is
// reversible in one click and changes no binding.
func (s *Service) Delete(ctx context.Context, kind, name string, actor *entity.User) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.Delete",
		xfield.String("kind", kind),
		xfield.String("name", name),
	)
	defer span.End()

	var unlinkedCount int64

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		// No lock: nothing here reads a value and then writes based on it. The
		// cascade DELETEs, which needs no prior count to be correct, and the
		// row delete follows it in the same transaction.
		//
		// A sign-in committing an identity after the cascade has run no longer
		// leaves a row behind. The insert takes a FOR KEY SHARE lock on this
		// row, so the two serialize: either the sign-in commits first and this
		// delete fails on the child it did not see, or this commits first and
		// the sign-in's insert fails against a parent that is gone. Both answers
		// are correct; neither is an orphan. The first surfaces to the admin as
		// a conflict they clear by repeating the delete.
		//
		// The row is read INSIDE the transaction because the cascade needs its
		// id, not its name. Reading it outside would reintroduce by the back
		// door exactly what the foreign key removes: a window in which the id
		// belongs to a row that has since been replaced under the same name.
		existing, getErr := s.store.GetForUpdateByKindName(ctx, kind, name)
		if getErr != nil {
			xlog.Error(ctx, "failed to load integration for delete", xfield.Error(getErr))

			return getErr
		}

		unlinked, cascadeErr := s.unlinkIdentities(ctx, kind, existing.ID)
		if cascadeErr != nil {
			return cascadeErr
		}
		unlinkedCount = unlinked

		if delErr := s.store.Delete(ctx, kind, name); delErr != nil {
			// A foreign-key violation here means a sign-in committed an identity
			// between the cascade above and this statement -- the one window the
			// cascade cannot cover, since the sign-in takes no lock this
			// transaction could wait on. RESTRICT refuses rather than orphaning
			// it, which is correct, but the admin did nothing wrong: repeating
			// the delete cascades whatever arrived and succeeds. Translated so
			// they are told that instead of reading a 500.
			if dbtx.ErrorIs(delErr, dbtx.ErrPGForeignKeyViolation) {
				xlog.Warn(ctx, "provider delete raced a sign-in; retry is safe",
					xfield.String("name", name))

				return apperr.ErrIntegrationInUse
			}

			xlog.Error(ctx, "failed to delete integration", xfield.Error(delErr))

			return delErr
		}

		s.publishAudit(ctx, audit.IntegrationDeleted{Actor: actor, Kind: kind, Name: name})

		return nil
	})
	if err != nil {
		return err
	}

	if unlinkedCount > 0 {
		// Said out loud, because nothing else will say it: the API answers 204,
		// and the people whose sign-in just stopped working are not in this
		// request. The audit entry carries the provider; this carries the cost.
		xlog.Warn(ctx, "login provider deleted with linked accounts",
			xfield.String("name", name), xfield.Int64("unlinked_identities", unlinkedCount))
	}

	// Post-commit, like every other mutation here: see notifyChanged on why
	// invalidating inside the tx can repopulate from a pre-commit read.
	s.notifyChanged(kind, name)

	return nil
}

// unlinkIdentities removes the identities tied to a login provider and reports
// how many there were.
//
// A CASCADE, where this used to be a refusal. The refusal was unsatisfiable:
// it told an operator to unlink the accounts first, and the only unlink in the
// product is /me/providers/{provider}/disconnect -- self-service, performed by
// the account's owner, and refused outright when it would remove their last
// sign-in method. An admin deleting a provider could not clear the way except
// by editing the database.
//
// It stays in Go rather than becoming ON DELETE CASCADE deliberately. A
// schema-level cascade would make the destructive half of a provider delete
// invisible at this call site, and would keep working silently if this function
// were ever removed -- so no test could observe the loss. RESTRICT plus an
// explicit delete fails loudly instead.
//
// Deliberately not gated on a count: an admin removing a provider people sign
// in through has decided something, and there is no second question to ask them
// that they could answer. The number is logged by the caller, because nobody
// else will mention it: the API answers 204, and the people affected are not in
// this request.
func (s *Service) unlinkIdentities(ctx context.Context, kind string, integrationID uuid.UUID) (int64, error) {
	// Scoped to the login category. Only a login row can have identities
	// pointing at it, so for anything else there is nothing to cascade and the
	// FK has nothing to refuse -- TestDelete_DeliveryRowDoesNotCascade is what
	// keeps that honest, since the comparison reads as obviously right and fails
	// silently when it is not.
	if kind != integrationkinds.CategoryLogin {
		return 0, nil
	}

	// No nil guard on the store. Under ON DELETE RESTRICT a skipped cascade does
	// not quietly leave rows behind -- it makes the delete below fail on the
	// children it did not remove. A binary wired for login providers without an
	// identities store is misconfigured, and a nil dereference here says so at
	// once rather than surfacing later as an unexplained foreign-key error.
	unlinked, err := s.identities.DeleteByIntegrationID(ctx, integrationID)
	if err != nil {
		return 0, fmt.Errorf("unlink accounts: %w", err)
	}

	return unlinked, nil
}
