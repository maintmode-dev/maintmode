package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// LinkIdentity attaches a provider identity (provider + subject) to userID.
// It rejects identities already linked to this user (ErrProviderAlreadyConnected)
// or to a different user (ErrProviderLinkedToAnotherUser).
func (s *Service) LinkIdentity(ctx context.Context, userID uuid.UUID, provider entity.AuthMethod, claims *entity.OAuthIDTokenClaims) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.LinkIdentity",
		xfield.String("provider", string(provider)),
	)
	defer span.End()

	method, err := s.methodRef(ctx, provider)
	if err != nil {
		xlog.Error(ctx, "failed to resolve provider", xfield.Error(err))

		return err
	}

	err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		// The account is re-resolved INSIDE the transaction, not before it. A
		// dance-driven link presents a ticket that may be a full dance-state TTL
		// old, so the account can have been blocked in the meantime; as two
		// calls, a user blocked between them would still be linked.
		//
		// GetForUpdateByID answers ErrUserNotFound for a deleted account but says
		// nothing about blocking -- UnlinkIdentity calls it purely as a lock and
		// discards the row -- so the IsBlocked check has to be explicit.
		//
		// This also applies to the BFF connect path, which shares this method,
		// and that is intended: refusing a blocked account a NEW permanent way in
		// is correct on both. Putting the check only in the dance branch would
		// leave one path able to do what the other refuses.
		owner, err := s.usersStore.GetForUpdateByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("lock user: %w", err)
		}

		if owner.IsBlocked() {
			return apperr.ErrUserBlocked
		}

		// Reject if this provider subject is already linked anywhere.
		bySubject, err := s.identitiesStore.GetByMethodSubject(ctx, method, claims.Subject)
		switch {
		case err == nil && bySubject.UserID == userID:
			return apperr.ErrProviderAlreadyConnected
		case err == nil:
			return apperr.ErrProviderLinkedToAnotherUser
		case errors.Is(err, apperr.ErrProviderNotConnected):
			// subject not linked yet — continue
		default:
			return fmt.Errorf("get identity by subject: %w", err)
		}

		// Reject if this user already has an identity for this provider (under a
		// different subject). One identity per (user, provider) keeps the
		// disconnect lockout guard sound.
		_, err = s.identitiesStore.GetByUserAndMethod(ctx, userID, method)
		switch {
		case err == nil:
			return apperr.ErrProviderAlreadyConnected
		case errors.Is(err, apperr.ErrProviderNotConnected):
			// provider not linked for this user yet — proceed
		default:
			return fmt.Errorf("get identity by user and provider: %w", err)
		}

		// On a concurrent connect that races past the checks above, Create
		// surfaces ErrProviderAlreadyConnected from the unique index — that's
		// already the 409 we want, so no special handling is needed here.
		identity := &entity.UserIdentity{
			UserID:  userID,
			Subject: claims.Subject,
			Email:   claims.Email,
		}
		method.Apply(identity)

		if _, err = s.identitiesStore.Create(ctx, identity); err != nil {
			return fmt.Errorf("create identity: %w", err)
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to link identity", xfield.Error(err))
		return err
	}

	return nil
}

// UnlinkIdentity removes the provider identity from userID. It refuses to remove
// the last remaining identity (ErrCannotDisconnectLastProvider) and refuses
// built-in methods outright (ErrCannotDisconnectBuiltinMethod). Disconnecting a
// provider the user is not linked to is a no-op success (idempotent).
func (s *Service) UnlinkIdentity(ctx context.Context, userID uuid.UUID, provider entity.AuthMethod) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.UnlinkIdentity",
		xfield.String("provider", string(provider)),
	)
	defer span.End()

	// Break-glass is not the account's to detach: it is configured on the
	// deployment, revoked by emptying the instance secret, and written again by
	// the next break-glass sign-in. Removing the row would revoke nothing and
	// mislead whoever asked.
	//
	// Refused HERE rather than only at the API boundary, because this method is
	// the one that guarantees it. A guard on the endpoint alone leaves every
	// other caller -- including a future one -- able to do what the product does
	// not allow.
	if provider.IsBuiltin() {
		return apperr.ErrCannotDisconnectBuiltinMethod
	}

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		// Lock the user row so concurrent disconnects serialize; otherwise two
		// disconnects could both observe count > 1 and both delete, locking the
		// user out. With one identity per (user, registry row) -- the partial
		// unique index -- the count equals the number of connected providers, so
		// the guard below is exact. Break-glass is not in it: see
		// CountProvidersByUserID.
		if _, err := s.usersStore.GetForUpdateByID(ctx, userID); err != nil {
			return fmt.Errorf("lock user: %w", err)
		}

		count, err := s.identitiesStore.CountProvidersByUserID(ctx, userID)
		if err != nil {
			return fmt.Errorf("count identities: %w", err)
		}
		if count <= 1 {
			return apperr.ErrCannotDisconnectLastProvider
		}

		// Addressed by the provider's NAME, joined to the registry row inside the
		// statement. Deleting an identity the user does not hold removes nothing
		// and reports no error, which is the idempotence /disconnect promises.
		//
		// Built-in methods never arrive here: DisconnectProvider refuses them
		// before this runs, because break-glass belongs to the deployment rather
		// than to the account and detaching it would revoke nothing.
		if err := s.identitiesStore.DeleteUserIdentityByProviderName(ctx, userID, provider); err != nil {
			return fmt.Errorf("delete identity: %w", err)
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to unlink identity", xfield.Error(err))
		return err
	}

	return nil
}

// ListConnectedProviders returns the providers linked to userID, ordered by
// provider ASC.
func (s *Service) ListConnectedProviders(ctx context.Context, userID uuid.UUID) ([]entity.AuthMethod, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.ListConnectedProviders")
	defer span.End()

	providers, err := s.identitiesStore.ListMethodsByUserID(ctx, userID)
	if err != nil {
		xlog.Error(ctx, "failed to list providers", xfield.Error(err))
		return nil, fmt.Errorf("list providers: %w", err)
	}

	return providers, nil
}
