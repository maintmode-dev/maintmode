package bootstrap

import (
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/storages/audit"

	"github.com/ruko1202/maintmode/internal/storages/distributedlock"

	"github.com/ruko1202/maintmode/internal/storages/blacklisttoken"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/userinvitations"
	users "github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

type AuthStores struct {
	TxManager       *dbtx.TxManager
	Users           *users.Store
	UserIdentities  *useridentities.Store
	UserInvitations *userinvitations.Store
	RefreshToken    *refreshtoken.Store
	TokenBlackList  *blacklisttoken.Store
	Locker          *distributedlock.Store
	Audit           *audit.Store
}

func NewAuthStores(db *sqlx.DB, redisDB *redis.Client) *AuthStores {
	return &AuthStores{
		TxManager:       dbtx.NewTxManager(db),
		Users:           users.NewStore(db),
		UserIdentities:  useridentities.NewStore(db),
		UserInvitations: userinvitations.NewStore(db),
		RefreshToken:    refreshtoken.NewStore(db),
		TokenBlackList:  blacklisttoken.NewStore(redisDB),
		Locker:          distributedlock.NewStore(redisDB),
		Audit:           audit.NewStore(db),
	}
}
