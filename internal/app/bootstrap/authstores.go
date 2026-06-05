package bootstrap

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/ruko1202/goque"

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
	// taskStorage backs the goque outbox: invitation emails enqueue through it
	// (atomically with the invitation insert) and the auth task processor drains
	// it. Shares the goque_task table with the maintmode binary.
	taskStorage goque.TaskStorage
}

func NewAuthStores(db *sqlx.DB, redisDB *redis.Client) (*AuthStores, error) {
	taskStorage, err := goque.NewStorage(db)
	if err != nil {
		return nil, fmt.Errorf("init goque storage: %w", err)
	}

	return &AuthStores{
		TxManager:       dbtx.NewTxManager(db),
		Users:           users.NewStore(db),
		UserIdentities:  useridentities.NewStore(db),
		UserInvitations: userinvitations.NewStore(db),
		RefreshToken:    refreshtoken.NewStore(db),
		TokenBlackList:  blacklisttoken.NewStore(redisDB),
		Locker:          distributedlock.NewStore(redisDB),
		Audit:           audit.NewStore(db),
		taskStorage:     taskStorage,
	}, nil
}
