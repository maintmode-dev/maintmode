package token

import (
	"context"
	"crypto/ecdsa"
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// RevokedSessions records sessions whose access tokens must stop working
// before they expire. Satisfied by *blacklisttoken.Store.
type RevokedSessions interface {
	AddSessions(ctx context.Context, expiration time.Duration, sessionIDs ...uuid.UUID) error
}

// Service handles JWT issuance and verification using ES256.
type Service struct {
	txManager       *dbtx.TxManager
	tokensStore     *refreshtoken.Store
	revokedSessions RevokedSessions
	// revokedSessionTTL is how long a revoked session stays marked: the longest
	// an access token minted for it can still be valid.
	revokedSessionTTL time.Duration
	privateKey        *ecdsa.PrivateKey
	kid               string
	issuer            string
	getNowF           func() time.Time
}

// revokedSessionSkew pads the mark past the access-token TTL. It covers clock
// skew between replicas and an access token minted for the session by a
// refresh that raced the revocation, whose exp lands slightly after the
// revocation's own now+TTL.
const revokedSessionSkew = time.Minute

// NewService builds the token service. cfg.AccessTokenTTL is the configured
// (longest) access-token lifetime; a revoked session is remembered that long.
// privateKey is cfg's signing key, parsed by the caller so a bad key fails
// startup there.
func NewService(
	txManager *dbtx.TxManager,
	refreshTokenStore *refreshtoken.Store,
	revokedSessions RevokedSessions,
	cfg *config.JWT,
	privateKey *ecdsa.PrivateKey,
) *Service {
	return &Service{
		txManager:         txManager,
		tokensStore:       refreshTokenStore,
		revokedSessions:   revokedSessions,
		revokedSessionTTL: cfg.AccessTokenTTL + revokedSessionSkew,
		privateKey:        privateKey,
		kid:               cfg.Kid,
		issuer:            cfg.Issuer,
		getNowF:           xtime.UTCNow,
	}
}
