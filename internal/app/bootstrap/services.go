package bootstrap

import (
	"context"
	"fmt"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
	"github.com/ruko1202/maintmode/internal/services/auditor"
	"github.com/ruko1202/maintmode/internal/services/auditpublisher"
	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/authz"
	"github.com/ruko1202/maintmode/internal/services/calendar"
	conflictsSvr "github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/services/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/services/invitation"
	"github.com/ruko1202/maintmode/internal/services/jwtverifier"
	maintSrv "github.com/ruko1202/maintmode/internal/services/maint"
	"github.com/ruko1202/maintmode/internal/services/maintnotify"
	"github.com/ruko1202/maintmode/internal/services/messaging/scheduler"
	messagesender "github.com/ruko1202/maintmode/internal/services/messaging/sender"
	"github.com/ruko1202/maintmode/internal/services/notifytargets"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider/googleoauth"
	resourcesSrv "github.com/ruko1202/maintmode/internal/services/resources"
	statecodec "github.com/ruko1202/maintmode/internal/services/state_codec"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/user"
	"github.com/ruko1202/maintmode/internal/services/userpicker"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
)

// Services contains all service layer dependencies for the single maintmode
// process: the core maintenance/resource/notify services and the auth
// (user/token/invitation/audit) services collapsed in from the former auth
// binary (RUK-194).
type Services struct {
	// Core services.
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

	// Auth-module services (formerly AuthServices).
	Auth       *auth.Service
	Token      *token.Service
	User       *user.Service
	Invitation *invitation.Service
	Audit      *auditor.Auditor
	StateCodec *statecodec.Service
	// AuditPublisher enqueues audit events to the durable goque outbox; the
	// audit-write processor drains them after commit. There are no in-process
	// goroutines to drain, so no Stop is needed on shutdown — the goque runtime
	// owns the drain lifecycle.
	AuditPublisher *auditpublisher.Publisher

	// TokenChecker is the active-token checker wired into the API server's
	// active-token middleware (the real *auth.Service).
	TokenChecker middlewares.ActiveTokenChecker
}

func NewServices(ctx context.Context,
	cfg *config.AppConfig,
	stores *Stores,
	gateways *Gateways,
) (*Services, error) {
	// queue is the single goque task-queue manager for this process. The scheduler
	// wraps it for delivery/reminder enqueue; the audit publisher (RUK-182)
	// enqueues audit.write tasks through it directly. Both the audit publisher and
	// the messaging scheduler share this one queue manager (one connection per
	// queue).
	queue := goque.NewTaskQueueManager(stores.taskStorage)

	// auditPublisher enqueues audit events to the durable outbox. The audit-write
	// processor (processors.go) drains them after commit. Built first because the
	// user and auth services below depend on it.
	auditPublisher := auditpublisher.New(queue)

	// tokenSrv is built before userSrv: blocking a user revokes their refresh
	// tokens, so the user service depends on the token service.
	tokenSrv := token.NewService(
		stores.TxManager,
		stores.RefreshToken,
		cfg.JWT.GeneratePrivateKey(),
		cfg.JWT.Issuer,
		cfg.JWT.Kid,
	)

	userSrv := user.NewService(
		cfg.Environment,
		stores.TxManager,
		stores.Users,
		stores.UserIdentities,
		auditPublisher,
		tokenSrv,
	)

	// authorizer is the single RBAC authorizer shared by the core (scenario
	// middleware) and the auth admin routes.
	authorizer, err := authz.NewCasbinAuthorizer(cfg.RBAC)
	if err != nil {
		return nil, fmt.Errorf("failed to init casbin authorizer: %w", err)
	}

	stateCodec := statecodec.NewService(
		[]byte(cfg.JWT.OAuthStateSigningKey),
		cfg.JWT.OAuthStateTTL,
	)

	oauthProviderList, err := initOAuthProviders(ctx, &cfg.OauthProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to init oauth providers: %w", err)
	}
	oauthProviders := oauthprovider.NewOAuthProviders(cfg, oauthProviderList)

	authSrv := auth.NewService(
		&cfg.JWT,
		stores.TxManager,
		userSrv,
		stores.Locker,
		stores.TokenBlackList,
		oauthProviders,
		tokenSrv,
		auditPublisher,
	)

	// Auditor is both read-side (api/public/audit reads logs through it) and
	// write-side (the audit-write goque processor writes the log after commit).
	auditorSrv := auditor.NewAuditor(
		stores.Audit,
	)

	invitationSrv := invitation.NewService(
		cfg,
		stores.TxManager,
		stores.UserInvitations,
		userSrv,
		authSrv,
		oauthProviders,
		messagesender.NewService(
			gateways.NotifyTransportRegistry,
			scheduler.NewService(queue),
		),
	)

	core, err := newCoreServices(ctx, cfg, stores, gateways, queue)
	if err != nil {
		return nil, err
	}

	return &Services{
		Maint: maintSrv.NewService(
			stores.TxManager,
			stores.Maintenances,
			stores.Resources,
			core.notifyTargets,
			core.conflicts,
			core.notifier,
			core.deferred,
			userSrv,
			auditPublisher,
		),
		Conflicts: core.conflicts,
		Calendar: calendar.NewService(
			stores.Maintenances,
			stores.Resources,
			stores.NotifyTargets,
			core.conflicts,
		),
		Resources: resourcesSrv.NewService(
			stores.TxManager,
			stores.Resources,
		),
		RBAC:          authorizer,
		JWTVerifier:   core.jwtVerifier,
		NotifyTargets: core.notifyTargets,
		Notifier:      core.notifier,
		UserPicker:    userpicker.NewService(userSrv),
		UserSummary:   usersummary.NewService(userSrv),

		Auth:           authSrv,
		Token:          tokenSrv,
		User:           userSrv,
		Invitation:     invitationSrv,
		Audit:          auditorSrv,
		StateCodec:     stateCodec,
		AuditPublisher: auditPublisher,
		TokenChecker:   authSrv,
	}, nil
}

// coreServices groups the core (non-auth) domain services so NewServices can
// build them in one step. The auth-module services (token/user/auth/...) are
// built inline in NewServices because the returned Services struct wires them
// into several fields directly.
type coreServices struct {
	conflicts     *conflictsSvr.Service
	jwtVerifier   *jwtverifier.Service
	notifyTargets *notifytargets.Service
	notifier      *maintnotify.Service
	deferred      *deferrednotifications.Service
}

// newCoreServices builds the core (maintenance/calendar/resource/notify) domain
// services. It depends only on stores, config, gateways and the shared goque
// queue manager — not on the auth-module services.
func newCoreServices(
	ctx context.Context,
	cfg *config.AppConfig,
	stores *Stores,
	gateways *Gateways,
	queue goque.TaskQueueManager,
) (*coreServices, error) {
	conflictsService := conflictsSvr.NewService(
		stores.Conflicts,
		stores.ConflictSnapshots,
	)

	jwtVerifier, err := jwtverifier.NewService(ctx, cfg.JWTVerifier)
	if err != nil {
		return nil, fmt.Errorf("failed to init jwt verifier: %w", err)
	}

	notifyTargets := notifytargets.NewService(
		stores.TxManager,
		stores.ChannelCatalog,
		stores.NotifyTargets,
	)

	// scheduler owns all goque enqueue/cancel plumbing. The message sender
	// (delivery) and deferred reminders both schedule through it.
	taskScheduler := scheduler.NewService(queue)

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

	return &coreServices{
		conflicts:     conflictsService,
		jwtVerifier:   jwtVerifier,
		notifyTargets: notifyTargets,
		notifier:      notifier,
		deferred:      deferred,
	}, nil
}

func initOAuthProviders(ctx context.Context, cfg *config.OauthProviders) ([]oauthprovider.OAuthProvider, error) {
	google, err := googleoauth.NewProvider(ctx, &cfg.Google)
	if err != nil {
		return nil, fmt.Errorf("init google oauth provider: %w", err)
	}

	return []oauthprovider.OAuthProvider{
		google,
	}, nil
}
