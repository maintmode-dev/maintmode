package token

import (
	"crypto/ecdsa"
	"time"

	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Service handles JWT issuance and verification using ES256.
type Service struct {
	txManager   *dbtx.TxManager
	tokensStore *refreshtoken.Store
	privateKey  *ecdsa.PrivateKey
	kid         string
	issuer      string
	getNowF     func() time.Time
}

func NewService(
	txManager *dbtx.TxManager,
	refreshTokenStore *refreshtoken.Store,
	privateKey *ecdsa.PrivateKey,
	issuer, kid string,
) *Service {
	return &Service{
		txManager:   txManager,
		tokensStore: refreshTokenStore,
		privateKey:  privateKey,
		kid:         kid,
		issuer:      issuer,
		getNowF:     xtime.UTCNow,
	}
}
