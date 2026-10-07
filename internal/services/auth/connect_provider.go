package auth

import (
	"context"
	"fmt"
	"slices"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// DisconnectProvider unlinks a provider identity from the authenticated user.
// It refuses to remove the user's last provider unless a built-in method would
// still let them in (see hasBuiltinSignIn).
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

	// Asked only for the last provider, the one case it decides: with another
	// provider linked the user keeps a way in regardless, and a flag that
	// cannot be read must not fail a disconnect that never needed it.
	keepsSignIn := false
	if len(linked) <= 1 {
		keepsSignIn, err = s.hasBuiltinSignIn(ctx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("check built-in sign-in: %w", err)
		}
	}

	if err := s.usersSrv.UnlinkIdentity(ctx, cmd.UserID, entity.AuthMethod(cmd.Provider), keepsSignIn); err != nil {
		return fmt.Errorf("unlink identity: %w", err)
	}

	return nil
}
