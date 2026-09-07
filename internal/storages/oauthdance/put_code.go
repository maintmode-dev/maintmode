package oauthdance

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// PutCode parks a minted token pair under the hash of the one-time opaque code
// the browser carries to the frontend.
//
// The pair is encoded from the ENTITY, deliberately, and never from the API
// response DTO. entity.TokenPair.SessionID is documented as never appearing in
// the API response, so a DTO round trip would drop it silently — and SessionID
// is what ties a login to its audit record.
func (s *Store) PutCode(ctx context.Context, code string, pair *entity.TokenPair) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.OAuthDance.PutCode")
	defer span.End()

	// The pair genuinely is credential material: that is what a one-time code
	// redeems. It is held for 60 seconds under a hashed key, which is the whole
	// design, so the marshal-a-secret warning has nothing to add here.
	//nolint:gosec // G117: storing the token pair IS the purpose of this store
	encoded, err := json.Marshal(pair)
	if err != nil {
		return fmt.Errorf("marshal token pair: %w", err)
	}

	if err := s.db.Set(ctx, codeKey(code), encoded, s.codeTTL).Err(); err != nil {
		return fmt.Errorf("put dance code: %w", err)
	}

	return nil
}
