package oauthdance

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// PutLinkTicket parks a link intent behind an opaque ticket.
//
// JSON rather than the bare-value encoding the invitation handle uses, because
// there are two fields and both are checked: a positional or delimiter-joined
// form would have to define what a delimiter inside a value means, and the
// provider name comes from configuration.
func (s *Store) PutLinkTicket(ctx context.Context, ticket string, intent entity.LinkIntent) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.OAuthDance.PutLinkTicket")
	defer span.End()

	payload, err := json.Marshal(intent)
	if err != nil {
		return fmt.Errorf("marshal link intent: %w", err)
	}

	if err := s.db.Set(ctx, linkKey(ticket), payload, s.handleTTL).Err(); err != nil {
		return fmt.Errorf("store link ticket: %w", err)
	}

	return nil
}
