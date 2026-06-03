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
func (s *Service) LinkIdentity(ctx context.Context, userID uuid.UUID, provider entity.OAuthProvider, claims *entity.OAuthIDTokenClaims) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.LinkIdentity",
		xfield.String("provider", string(provider)),
	)
	defer span.End()

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		// Reject if this provider subject is already linked anywhere.
		bySubject, err := s.identitiesStore.GetByProviderSubject(ctx, provider, claims.Subject)
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
		_, err = s.identitiesStore.GetByUserAndProvider(ctx, userID, provider)
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
		_, err = s.identitiesStore.Create(ctx, &entity.UserIdentity{
			UserID:   userID,
			Provider: provider,
			Subject:  claims.Subject,
			Email:    claims.Email,
		})
		if err != nil {
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
// the last remaining identity (ErrCannotDisconnectLastProvider). Disconnecting a
// provider the user is not linked to is a no-op success (idempotent).
func (s *Service) UnlinkIdentity(ctx context.Context, userID uuid.UUID, provider entity.OAuthProvider) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.UnlinkIdentity",
		xfield.String("provider", string(provider)),
	)
	defer span.End()

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		// Lock the user row so concurrent disconnects serialize; otherwise two
		// disconnects could both observe count > 1 and both delete, locking the
		// user out. With one identity per (user, provider), the row count equals
		// the number of connected providers, so the guard below is exact.
		if _, err := s.usersStore.GetForUpdateByID(ctx, userID); err != nil {
			return fmt.Errorf("lock user: %w", err)
		}

		count, err := s.identitiesStore.CountByUserID(ctx, userID)
		if err != nil {
			return fmt.Errorf("count identities: %w", err)
		}
		if count <= 1 {
			return apperr.ErrCannotDisconnectLastProvider
		}

		if err := s.identitiesStore.DeleteByUserAndProvider(ctx, userID, provider); err != nil {
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

// ListConnectedProviders returns the providers linked to userID, ordered by the
// identity creation time.
func (s *Service) ListConnectedProviders(ctx context.Context, userID uuid.UUID) ([]entity.OAuthProvider, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.ListConnectedProviders")
	defer span.End()

	providers, err := s.identitiesStore.ListProvidersByUserID(ctx, userID)
	if err != nil {
		xlog.Error(ctx, "failed to list providers", xfield.Error(err))
		return nil, fmt.Errorf("list providers: %w", err)
	}

	return providers, nil
}
