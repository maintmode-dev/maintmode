package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ConnectProvider verifies an upstream provider ID token and links the
// resulting identity to the authenticated user. This is the BFF-owned flow:
// the frontend completes the OAuth dance and posts the ID token here.
func (s *Service) ConnectProvider(ctx context.Context, cmd *entity.ConnectProviderCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.ConnectProvider",
		xfield.String("provider", string(cmd.Provider)),
	)
	defer span.End()

	oauthProvider, err := s.oauthProviders.Get(ctx, cmd.Provider)
	if err != nil {
		return fmt.Errorf("get oauth provider: %w", err)
	}

	claims, err := oauthProvider.VerifyToken(ctx, cmd.IDToken)
	if err != nil {
		xlog.Error(ctx, "verify id token failed", xfield.Error(err))
		return err
	}

	if err := s.usersSrv.LinkIdentity(ctx, cmd.UserID, cmd.Provider, claims); err != nil {
		return fmt.Errorf("link identity: %w", err)
	}

	return nil
}

// DisconnectProvider unlinks a provider identity from the authenticated user.
// It refuses to remove the user's only sign-in method.
func (s *Service) DisconnectProvider(ctx context.Context, cmd *entity.DisconnectProviderCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.DisconnectProvider",
		xfield.String("provider", string(cmd.Provider)),
	)
	defer span.End()

	if err := s.usersSrv.UnlinkIdentity(ctx, cmd.UserID, cmd.Provider); err != nil {
		return fmt.Errorf("unlink identity: %w", err)
	}

	return nil
}
