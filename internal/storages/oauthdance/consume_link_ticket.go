package oauthdance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	valkeylib "github.com/redis/go-redis/v9"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ConsumeLinkTicket redeems a ticket for the intent behind it, returning nil
// when there is nothing to redeem.
//
// GETDEL, for the reason ConsumeCode and ConsumeInvitationHandle document: the
// single round trip is what makes the ticket genuinely one-shot. This is the
// ONLY spend in the link flow -- /start peeks -- so it is what stops a captured
// cookie from linking twice, or from linking at all after the first completion.
//
// A nil intent with no error is a clean miss, and the caller must not read it as
// "this was not a link". Once the browser presented a link cookie, a miss is a
// refusal; treating it as an absent ticket would sign the person in as whichever
// provider account they just authenticated with.
func (s *Store) ConsumeLinkTicket(ctx context.Context, ticket string) (*entity.LinkIntent, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.OAuthDance.ConsumeLinkTicket")
	defer span.End()

	raw, err := s.db.GetDel(ctx, linkKey(ticket)).Result()
	if err != nil {
		if errors.Is(err, valkeylib.Nil) {
			return nil, nil //nolint:nilnil // a clean miss is not a fault here; see the doc comment.
		}

		return nil, fmt.Errorf("consume link ticket: %w", err)
	}

	return decodeLinkIntent(raw)
}

// decodeLinkIntent parses a stored intent.
//
// Only this service writes these values, so an unparseable one means the entry
// was corrupted or tampered with. Refusing loudly is right for the same reason
// the invitation handle refuses: the user id here decides whose account gains a
// sign-in method.
func decodeLinkIntent(raw string) (*entity.LinkIntent, error) {
	var intent entity.LinkIntent
	if err := json.Unmarshal([]byte(raw), &intent); err != nil {
		return nil, fmt.Errorf("parse link intent: %w", err)
	}

	return &intent, nil
}
