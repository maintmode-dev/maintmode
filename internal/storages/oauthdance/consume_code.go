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

// ConsumeCode redeems a one-time opaque code, returning nil when there is
// nothing to redeem. Checking the entry's binding is the caller's job; the code
// is spent either way, so a wrong guess at the binding burns it.
//
// A nil pair covers unknown, expired and already-redeemed alike, and the caller
// answers all three with one identical 401: telling them apart would tell an
// attacker which of their guesses was structurally right. The audit trail is
// where the distinction is kept.
//
// GETDEL, not GET-then-DEL. The single round trip is what makes the code truly
// single-use: with two calls, N concurrent redemptions all read the same live
// value before any of them deletes it, and every one of them gets a token pair.
func (s *Store) ConsumeCode(ctx context.Context, code string) (*entity.DanceCode, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.OAuthDance.ConsumeCode")
	defer span.End()

	encoded, err := s.db.GetDel(ctx, codeKey(code)).Result()
	if err != nil {
		if errors.Is(err, valkeylib.Nil) {
			return nil, nil //nolint:nilnil // "no code to redeem" is not an error condition here; see the doc comment.
		}

		return nil, fmt.Errorf("consume dance code: %w", err)
	}

	entry := new(entity.DanceCode)
	if err := json.Unmarshal([]byte(encoded), entry); err != nil {
		return nil, fmt.Errorf("unmarshal dance code: %w", err)
	}

	return entry, nil
}
