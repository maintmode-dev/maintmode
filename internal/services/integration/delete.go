package integration

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// Delete removes an integration, taking a login provider's linked identities
// with it.
//
// The thing being protected is not the row but the binding between a provider
// NAME and the identities that carry it. user_identities.provider is a bare
// string with no foreign key, so a row left behind waits for the name to be
// created again against a different IdP and then vouches for it against an
// account that predates it -- which is why the identities go with the provider
// rather than staying.
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
		// A sign-in committing an identity after the cascade has run still
		// leaves a row behind, and no lock fixes that -- the sign-in does not
		// take one. What makes it harmless is the closed set: the name it lands
		// on can only be reused by the same two entries, and `google` cannot be
		// aimed elsewhere. RUK-303 removes the possibility entirely by making
		// the column a foreign key.
		unlinked, cascadeErr := s.unlinkIdentities(ctx, kind, name)
		if cascadeErr != nil {
			return cascadeErr
		}
		unlinkedCount = unlinked

		if delErr := s.store.Delete(ctx, kind, name); delErr != nil {
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

// unlinkIdentities removes the identities tied to a login provider's name and
// reports how many there were.
//
// A CASCADE, where this used to be a refusal. The refusal was unsatisfiable:
// it told an operator to unlink the accounts first, and the only unlink in the
// product is /me/providers/{provider}/disconnect -- self-service, performed by
// the account's owner, and refused outright when it would remove their last
// sign-in method. An admin deleting a provider could not clear the way except
// by editing the database.
//
// Leaving the rows is not an option either, and it is the one this codebase
// must reject hardest: provider is a bare string with no foreign key, so an
// orphaned row does not merely dangle. It waits for the same name to be created
// again against a different IdP, and then hands that IdP an account that
// predates it -- the takeover the whole closed set exists to prevent.
//
// So the delete takes the accounts with it. Deliberately not gated on a count:
// an admin removing a provider people sign in through has decided something,
// and there is no second question to ask them that they could answer. The
// number is logged by the caller, because nobody else will mention it: the API
// answers 204, and the people affected are not in this request.
//
// A nil identities store means the guard was never wired -- tests, and binaries
// with no auth storage. Nothing to cascade there.
func (s *Service) unlinkIdentities(ctx context.Context, kind, name string) (int64, error) {
	if s.identities == nil {
		return 0, nil
	}

	// Scoped to the login category, because user_identities.provider holds
	// system names with no category qualifier. Unscoped, deleting a Slack row
	// named "keycloak" would take the OIDC provider's identities with it --
	// TestDelete_DeliveryRowDoesNotCascade is what keeps that honest, since the
	// comparison reads as obviously right and fails silently when it is not.
	if kind != integrationkinds.CategoryLogin {
		return 0, nil
	}

	unlinked, err := s.identities.DeleteByProvider(ctx, entity.AuthMethod(name))
	if err != nil {
		return 0, fmt.Errorf("unlink accounts: %w", err)
	}

	return unlinked, nil
}
