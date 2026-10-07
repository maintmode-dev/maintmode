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

// ConsumeLinkCode redeems a one-time link code, returning nil when there is
// nothing to redeem. GETDEL for the same reason as ConsumeCode: one round trip
// is what makes it single-use under concurrent redemptions.
func (s *Store) ConsumeLinkCode(ctx context.Context, code string) (*entity.PendingLink, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.OAuthDance.ConsumeLinkCode")
	defer span.End()

	raw, err := s.db.GetDel(ctx, linkCodeKey(code)).Result()
	if err != nil {
		if errors.Is(err, valkeylib.Nil) {
			return nil, nil //nolint:nilnil // a clean miss is not a fault; the caller answers it like a bad code.
		}

		return nil, fmt.Errorf("consume link code: %w", err)
	}

	link := new(entity.PendingLink)
	if err := json.Unmarshal([]byte(raw), link); err != nil {
		return nil, fmt.Errorf("parse pending link: %w", err)
	}

	return link, nil
}
