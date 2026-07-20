package bootstrap

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/storages/notifychannel"

	"github.com/ruko1202/maintmode/internal/storages/audit"
	"github.com/ruko1202/maintmode/internal/storages/blacklisttoken"
	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/datakey"
	"github.com/ruko1202/maintmode/internal/storages/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/storages/distributedlock"
	integrationstore "github.com/ruko1202/maintmode/internal/storages/integration"
	"github.com/ruko1202/maintmode/internal/storages/licensecache"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/notifytargets"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/userinvitations"
	users "github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Stores contains all storage layer dependencies for the single maintmode
// process. It carries both the core (maintenance/resource/notify) stores and
// the auth (user/token/invitation/audit) stores collapsed in from the former
// auth binary (RUK-194).
type Stores struct {
	TxManager             *dbtx.TxManager
	Maintenances          *maintenances.Store
	Resources             *resources.Store
	Conflicts             *conflicts.Store
	ConflictSnapshots     *conflictsnapshots.Store
	NotifyTargets         *notifytargets.Store
	DeferredNotifications *deferrednotifications.Store
	ChannelCatalog        *notifychannel.Store

	// Integration registry stores (RUK-196): the settings catalog and the wrapped
	// data-encryption keys that protect its secrets at rest.
	Integrations *integrationstore.Store
	DataKeys     *datakey.Store

	// LicenseCache holds the last successful Console heartbeat response
	//. Constructed unconditionally (it is just a table handle); only a
	// license-enabled process ever reads or writes it.
	LicenseCache *licensecache.Store

	// Auth-module stores (formerly AuthStores). TokenBlackList and Locker are
	// Redis-backed; the rest are Postgres-backed.
	Users           *users.Store
	UserIdentities  *useridentities.Store
	UserInvitations *userinvitations.Store
	RefreshToken    *refreshtoken.Store
	TokenBlackList  *blacklisttoken.Store
	Locker          *distributedlock.Store
	Audit           *audit.Store

	// taskStorage backs the single goque outbox shared by every module: messaging,
	// reminders, auto-cancel, invitation emails, audit writes and audit prune all
	// enqueue/drain through this one storage (one goque_task table per process).
	taskStorage goque.TaskStorage
}

// NewStores creates and initializes all storage layer dependencies. The Redis
// client backs the token blacklist and the distributed locker; everything else
// is Postgres-backed.
func NewStores(
	db *sqlx.DB,
	redisDB *redis.Client,
) (*Stores, error) {
	taskStorage, err := goque.NewStorage(db)
	if err != nil {
		return nil, fmt.Errorf("init goque storage: %w", err)
	}
	return &Stores{
		TxManager:             dbtx.NewTxManager(db),
		Maintenances:          maintenances.NewStore(db),
		Resources:             resources.NewStore(db),
		Conflicts:             conflicts.NewStore(db),
		ConflictSnapshots:     conflictsnapshots.NewStore(db),
		NotifyTargets:         notifytargets.NewStore(db),
		DeferredNotifications: deferrednotifications.NewStore(db),
		ChannelCatalog:        notifychannel.NewStore(db),

		Integrations: integrationstore.NewStore(db),
		DataKeys:     datakey.NewStore(db),
		LicenseCache: licensecache.NewStore(db),

		Users:           users.NewStore(db),
		UserIdentities:  useridentities.NewStore(db),
		UserInvitations: userinvitations.NewStore(db),
		RefreshToken:    refreshtoken.NewStore(db),
		TokenBlackList:  blacklisttoken.NewStore(redisDB),
		Locker:          distributedlock.NewStore(redisDB),
		Audit:           audit.NewStore(db),

		taskStorage: taskStorage,
	}, nil
}
