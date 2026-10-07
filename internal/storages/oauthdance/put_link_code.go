package oauthdance

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// PutLinkCode parks a pending link under the hash of the one-time code the
// browser carries to the frontend. It lives as long as a sign-in code: it covers
// the same hop, from the redirect to the BFF redeeming it.
func (s *Store) PutLinkCode(ctx context.Context, code string, link entity.PendingLink) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.OAuthDance.PutLinkCode")
	defer span.End()

	payload, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("marshal pending link: %w", err)
	}

	if err := s.db.Set(ctx, linkCodeKey(code), payload, s.codeTTL).Err(); err != nil {
		return fmt.Errorf("put link code: %w", err)
	}

	return nil
}
