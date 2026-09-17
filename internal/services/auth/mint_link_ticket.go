package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// MintLinkTicket parks a link intent and returns the opaque ticket naming it.
//
// This is where linking begins, and it begins HERE rather than at /start for a
// reason that is not stylistic: /start is entered by a top-level browser
// navigation, and this backend authenticates with a Bearer header only. A
// navigation cannot carry that header, so /start cannot learn who is asking.
// This endpoint can, because the frontend calls it with the token it already
// holds.
//
// The ticket is what carries that answer forward. It is an opaque secret, so the
// user id never travels through the provider's redirect, its logs, or a Referer.
func (s *Service) MintLinkTicket(
	ctx context.Context,
	userID uuid.UUID,
	providerSegment string,
) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.MintLinkTicket",
		xfield.String("provider", providerSegment))
	defer span.End()

	// The same allow-list /start applies, and applied here for the same reason:
	// a ticket for an instance whose dance the backend does not run would send
	// the browser to a /start that answers 400, after the frontend had already
	// been told the link was ready.
	provider, ok := s.authMethods.DanceProvider(providerSegment)
	if !ok {
		return "", fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, providerSegment)
	}

	ticket, err := newDanceSecret()
	if err != nil {
		return "", fmt.Errorf("mint link ticket: %w", err)
	}

	// Fail closed, exactly as the invitation handle does: returning a ticket that
	// was never stored would leave the browser holding one that redeems to
	// nothing, which reads to the user as "your link expired" rather than "our
	// store is down".
	if err := s.danceCodes.PutLinkTicket(ctx, ticket, entity.LinkIntent{
		UserID:   userID,
		Provider: provider,
	}); err != nil {
		return "", fmt.Errorf("store link ticket: %w", err)
	}

	return ticket, nil
}
