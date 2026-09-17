package oauthdance

import (
	"context"
	"errors"
	"fmt"

	valkeylib "github.com/redis/go-redis/v9"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// PeekLinkTicket reads a link intent WITHOUT spending it, returning nil when
// there is nothing to read.
//
// The read-only twin of ConsumeLinkTicket exists because /start has to validate
// a ticket -- does it exist, was it minted for this provider -- before sending
// the browser to the provider. Spending it there would leave the callback, where
// the link actually happens, with nothing to redeem.
//
// A nil intent with no error means a clean miss: unknown, expired or already
// spent. That is deliberately DISTINCT from an error, and callers must keep the
// two apart: reading a store failure as "no ticket" is what would turn a Valkey
// outage into a sign-in the user never asked for.
func (s *Store) PeekLinkTicket(ctx context.Context, ticket string) (*entity.LinkIntent, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.OAuthDance.PeekLinkTicket")
	defer span.End()

	raw, err := s.db.Get(ctx, linkKey(ticket)).Result()
	if err != nil {
		if errors.Is(err, valkeylib.Nil) {
			return nil, nil //nolint:nilnil // a clean miss is not a fault here; see the doc comment.
		}

		return nil, fmt.Errorf("peek link ticket: %w", err)
	}

	return decodeLinkIntent(raw)
}
