package bootstrap

import (
	"context"
	"fmt"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/authz"
	"github.com/ruko1202/maintmode/internal/services/calendar"
	conflictsSvr "github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/services/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/services/jwtverifier"
	maintSrv "github.com/ruko1202/maintmode/internal/services/maint"
	"github.com/ruko1202/maintmode/internal/services/maintnotify"
	"github.com/ruko1202/maintmode/internal/services/messaging/scheduler"
	messagesender "github.com/ruko1202/maintmode/internal/services/messaging/sender"
	"github.com/ruko1202/maintmode/internal/services/notifytargets"
	resourcesSrv "github.com/ruko1202/maintmode/internal/services/resources"
	"github.com/ruko1202/maintmode/internal/services/userpicker"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
)

// Services contains all service layer dependencies
type Services struct {
	Maint         *maintSrv.Service
	Conflicts     *conflictsSvr.Service
	Calendar      *calendar.Service
	Resources     *resourcesSrv.Service
	RBAC          *authz.CasbinAuthorizer
	JWTVerifier   *jwtverifier.Service
	NotifyTargets *notifytargets.Service
	Notifier      *maintnotify.Service
	UserPicker    *userpicker.Service
	UserSummary   *usersummary.Service
}

func NewServices(ctx context.Context,
	cfg *config.AppConfig,
	stores *Stores,
	gateways *Gateways,
) (*Services, error) {
	conflictsService := conflictsSvr.NewService(
		stores.Conflicts,
		stores.ConflictSnapshots,
	)

	jwtVerifier, err := jwtverifier.NewService(ctx, cfg.JWTVerifier)
	if err != nil {
		return nil, fmt.Errorf("failed to init jwt verifier: %w", err)
	}

	authorizer, err := authz.NewCasbinAuthorizer(cfg.RBAC)
	if err != nil {
		return nil, fmt.Errorf("failed to init casbin authorizer: %w", err)
	}

	notifyTargets := notifytargets.NewService(
		stores.TxManager,
		stores.ChannelCatalog,
		stores.NotifyTargets,
	)

	// scheduler owns all goque enqueue/cancel plumbing. The message sender
	// (delivery) and deferred reminders both schedule through it.
	taskScheduler := scheduler.NewService(goque.NewTaskQueueManager(stores.taskStorage))

	messageSender := messagesender.NewService(gateways.NotifyTransportRegistry, taskScheduler)

	notifier, err := maintnotify.NewNotifier(cfg, messageSender, stores.NotifyTargets)
	if err != nil {
		return nil, fmt.Errorf("failed to init maintnotify: %w", err)
	}

	deferred := deferrednotifications.NewService(
		stores.TxManager,
		stores.DeferredNotifications,
		taskScheduler,
	)

	return &Services{
		Maint: maintSrv.NewService(
			stores.TxManager,
			stores.Maintenances,
			stores.Resources,
			notifyTargets,
			conflictsService,
			notifier,
			deferred,
		),
		Conflicts: conflictsService,
		Calendar: calendar.NewService(
			stores.Maintenances,
			stores.Resources,
			conflictsService,
		),
		Resources: resourcesSrv.NewService(
			stores.TxManager,
			stores.Resources,
		),
		RBAC:          authorizer,
		JWTVerifier:   jwtVerifier,
		NotifyTargets: notifyTargets,
		Notifier:      notifier,
		UserPicker:    userpicker.NewService(gateways.Auth),
		UserSummary:   usersummary.NewService(gateways.Auth),
	}, nil
}
