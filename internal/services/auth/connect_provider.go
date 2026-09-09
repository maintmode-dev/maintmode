package auth

import (
	"context"
	"fmt"
	"slices"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// ConnectProvider verifies an upstream provider ID token and links the
// resulting identity to the authenticated user. This is the BFF-owned flow:
// the frontend completes the OAuth dance and posts the ID token here.
func (s *Service) ConnectProvider(ctx context.Context, cmd *entity.ConnectProviderCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.ConnectProvider",
		xfield.String("provider", cmd.Provider),
	)
	defer span.End()

	provider, ok := s.authMethods.Parse(cmd.Provider)
	if !ok {
		return fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, cmd.Provider)
	}

	authMethod, err := s.authMethods.Get(ctx, provider)
	if err != nil {
		return fmt.Errorf("get oauth provider: %w", err)
	}

	claims, err := authMethod.Authenticate(ctx, cmd.IDToken)
	if err != nil {
		xlog.Error(ctx, "verify id token failed", xfield.Error(err))
		return err
	}

	if err := s.usersSrv.LinkIdentity(ctx, cmd.UserID, provider, claims); err != nil {
		return fmt.Errorf("link identity: %w", err)
	}

	return nil
}

// DisconnectProvider unlinks a provider identity from the authenticated user.
// It refuses to remove the user's only sign-in method.
//
// A provider the user has not linked is already in the requested state, so this
// returns success without touching the database. The membership check is against
// what the USER has linked, not against the registry: unlinking is how a user
// gets out of a provider that has been removed from configuration, and refusing
// on "unknown provider" would strand exactly the identities most in need of
// removal.
//
// The early return is not merely a shortcut. Without it, UnlinkIdentity takes a
// FOR UPDATE lock on the user row and hits the last-provider guard BEFORE it
// reaches the delete, so an arbitrary path segment would open a transaction and
// lock a row per request, and a user with one identity would be told they cannot
// disconnect their last provider -- about a provider that never existed.
func (s *Service) DisconnectProvider(ctx context.Context, cmd *entity.DisconnectProviderCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.DisconnectProvider",
		xfield.String("provider", cmd.Provider),
	)
	defer span.End()

	linked, err := s.usersSrv.ListConnectedProviders(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("list connected providers: %w", err)
	}

	if !slices.Contains(linked, entity.AuthMethod(cmd.Provider)) {
		return nil
	}

	if err := s.usersSrv.UnlinkIdentity(ctx, cmd.UserID, entity.AuthMethod(cmd.Provider)); err != nil {
		return fmt.Errorf("unlink identity: %w", err)
	}

	return nil
}
