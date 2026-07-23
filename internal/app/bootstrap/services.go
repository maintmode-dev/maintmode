package bootstrap

import (
	"context"
	"fmt"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/services/dekrotator"

	"github.com/ruko1202/maintmode/internal/config"
	licensegw "github.com/ruko1202/maintmode/internal/gateways/license"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
	"github.com/ruko1202/maintmode/internal/services/auditor"
	"github.com/ruko1202/maintmode/internal/services/auditpublisher"
	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/authz"
	"github.com/ruko1202/maintmode/internal/services/calendar"
	conflictsSvr "github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/services/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/services/integration"
	"github.com/ruko1202/maintmode/internal/services/invitation"
	"github.com/ruko1202/maintmode/internal/services/jwtverifier"
	licensesvc "github.com/ruko1202/maintmode/internal/services/license"
	maintSrv "github.com/ruko1202/maintmode/internal/services/maint"
	"github.com/ruko1202/maintmode/internal/services/maintnotify"
	"github.com/ruko1202/maintmode/internal/services/messaging/scheduler"
	messagesender "github.com/ruko1202/maintmode/internal/services/messaging/sender"
	"github.com/ruko1202/maintmode/internal/services/notifytargets"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider/googleoauth"
	resourcesSrv "github.com/ruko1202/maintmode/internal/services/resources"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/transportresolver"
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
	Integration   *integration.Service
	// TransportResolver is the runtime delivery seam: it resolves a live transport
	// per send from the integration registry (dev keeps the stub). The async
	// send processor uses it in place of the static config-built registry.
	TransportResolver notifytransport.TransportResolver

	// Auth-module services (formerly AuthServices).
	Auth       *auth.Service
	Token      *token.Service
	User       *user.Service
	Invitation *invitation.Service
	Audit      *auditor.Auditor
	// AuditPublisher enqueues audit events to the durable goque outbox; the
	// audit-write processor drains them after commit. There are no in-process
	// goroutines to drain, so no Stop is needed on shutdown — the goque runtime
	// owns the drain lifecycle.
	AuditPublisher *auditpublisher.Publisher

	// TokenChecker is the active-token checker wired into the API server's
	// active-token middleware (the real *auth.Service).
	TokenChecker  middlewares.ActiveTokenChecker
	MessageSender *messagesender.Service

	// License is the enforcement surface (block-gate source, cache reload).
	// Never nil: the real license service in SaaS mode, Noop on self-hosted —
	// consumers never branch on "is the license configured".
	License licensesvc.Enforcement
	// licenseHeartbeat is the concrete license service driving the heartbeat
	// processor; nil on self-hosted, where the processor is not registered.
	licenseHeartbeat *licensesvc.Service
}

func NewServices(ctx context.Context,
	cfg *config.AppConfig,
	stores *Stores,
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

	// License enforcement surface (the suspend-gate license source): the real
	// client in SaaS mode, Noop otherwise.
	enforcement, heartbeatSrv := newLicenseService(cfg, stores)

	tokenSrv, userSrv := newTokenAndUserServices(cfg, stores, auditPublisher, enforcement)

	// authorizer is the single RBAC authorizer shared by the core (scenario
	// middleware) and the auth admin routes.
	authorizer, err := authz.NewCasbinAuthorizer(cfg.RBAC)
	if err != nil {
		return nil, fmt.Errorf("failed to init casbin authorizer: %w", err)
	}

	oauthProviders, err := initOAuthProviders(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init oauth providers: %w", err)
	}

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
	auditorSrv := auditor.NewAuditor(stores.Audit)

	integrationSrv, err := newIntegrationService(ctx, cfg, stores, auditPublisher)
	if err != nil {
		return nil, err
	}

	transportResolver := initTransportResolver(cfg, integrationSrv)

	messageSender := messagesender.NewService(transportResolver, scheduler.NewService(queue))

	invitationSrv := invitation.NewService(
		cfg,
		stores.TxManager,
		stores.UserInvitations,
		userSrv,
		authSrv,
		oauthProviders,
		messageSender,
		enforcement,
	)

	core, err := newCoreServices(ctx, cfg, stores, queue, transportResolver)
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
		RBAC:              authorizer,
		JWTVerifier:       core.jwtVerifier,
		NotifyTargets:     core.notifyTargets,
		Notifier:          core.notifier,
		UserPicker:        userpicker.NewService(userSrv),
		UserSummary:       usersummary.NewService(userSrv),
		Integration:       integrationSrv,
		TransportResolver: transportResolver,

		Auth:             authSrv,
		Token:            tokenSrv,
		User:             userSrv,
		Invitation:       invitationSrv,
		Audit:            auditorSrv,
		AuditPublisher:   auditPublisher,
		TokenChecker:     authSrv,
		MessageSender:    messageSender,
		License:          enforcement,
		licenseHeartbeat: heartbeatSrv,
	}, nil
}

// newTokenAndUserServices builds the token service first, then the user
// service on top of it: blocking a user revokes their refresh tokens, so the
// user service depends on the token service.
func newTokenAndUserServices(
	cfg *config.AppConfig,
	stores *Stores,
	auditPublisher *auditpublisher.Publisher,
	seatGuard user.SeatGuard,
) (tokenSvc *token.Service, userSvc *user.Service) {
	tokenSrv := token.NewService(
		stores.TxManager,
		stores.RefreshToken,
		cfg.JWT.GeneratePrivateKey(),
		cfg.JWT.Issuer,
		cfg.JWT.Kid,
	)

	userSrv := user.NewService(
		stores.TxManager,
		stores.Users,
		stores.UserIdentities,
		auditPublisher,
		tokenSrv,
		seatGuard,
		cfg.Auth.AllowOpenSignup,
	)

	return tokenSrv, userSrv
}

// newLicenseService builds the license enforcement: the real service (backed
// by the Console heartbeat gateway) when the license is configured, Noop
// otherwise — the process always starts, consumers always hold a working
// guard, and nothing is enforced without a config. Enforcement reads the
// license_cache row, seeded with a default active license by the migration and
// refreshed by the cron heartbeat, so startup never blocks on Console. The
// second return is the concrete service for the heartbeat processor — nil on
// self-hosted, where the processor is not registered.
func newLicenseService(
	cfg *config.AppConfig,
	stores *Stores,
) (licensesvc.Enforcement, *licensesvc.Service) {
	if !cfg.License.Enabled() {
		return licensesvc.NewNoop(), nil
	}

	// No synchronous heartbeat at startup: the license is always read from the
	// license_cache row, which the migration seeds with a default active license
	// so the block gate has something to read from the first request. The cron
	// heartbeat processor refreshes that row on its schedule, so startup never
	// blocks on Console being reachable, and a restart of a blocked org closes
	// the gate immediately from the persisted row.
	licenseSrv := licensesvc.NewService(
		stores.Users,
		stores.UserInvitations,
		stores.Audit,
		stores.LicenseCache,
		licensegw.NewClient(cfg.License),
		config.GetAppBuildMeta().Version,
		cfg.License.CacheReloadInterval,
	)

	return licenseSrv, licenseSrv
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
// services. It depends only on stores, config, the transport resolver and the
// shared goque queue manager — not on the auth-module services.
func newCoreServices(
	ctx context.Context,
	cfg *config.AppConfig,
	stores *Stores,
	queue goque.TaskQueueManager,
	transportResolver notifytransport.TransportResolver,
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

	messageSender := messagesender.NewService(transportResolver, taskScheduler)

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

func initOAuthProviders(ctx context.Context, cfg *config.AppConfig) (*oauthprovider.Providers, error) {
	google, err := googleoauth.NewProvider(ctx, &cfg.OauthProviders.Google)
	if err != nil {
		return nil, fmt.Errorf("init google oauth provider: %w", err)
	}

	return oauthprovider.NewOAuthProviders(cfg, []oauthprovider.OAuthProvider{
		google,
	}), nil
}

// newIntegrationService builds the DB-backed integration registry service on a
// fail-fast keyring, re-wrapping stored DEKs onto the active KEK first. The
// delivery-side resolver over it is wired separately (initTransportResolver).
func newIntegrationService(
	ctx context.Context,
	cfg *config.AppConfig,
	stores *Stores,
	auditPublisher *auditpublisher.Publisher,
) (*integration.Service, error) {
	// Build the keyring once (fail-fast — a missing/weak KEK aborts startup) and
	// reuse it for both the startup re-wrap and the service.
	keyring, err := secrets.NewLocalKeyring(cfg.Crypto.ActiveKEKURI, cfg.Crypto.LocalKeys)
	if err != nil {
		return nil, fmt.Errorf("init integration keyring: %w", err)
	}

	// Re-wrap stored DEKs onto the active KEK before anything reads them, so a KEK
	// rotation applied via redeploy takes effect with zero manual steps.
	if err := rotateDataKeys(ctx, stores, keyring); err != nil {
		return nil, err
	}

	registry, err := integration.NewRegistry(
		integrationkinds.Slack,
		integrationkinds.Telegram,
		integrationkinds.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("build integration registry: %w", err)
	}

	return integration.NewService(
		stores.TxManager,
		stores.Integrations,
		stores.DataKeys,
		registry,
		keyring,
		secrets.NewAESCipher(),
		auditPublisher,
	), nil
}

func initTransportResolver(cfg *config.AppConfig, integrationSrv *integration.Service) notifytransport.TransportResolver {
	if cfg.Environment.IsDev() && cfg.NotifyTransport.UseStub {
		return notifytransport.NewStubResolver()
	}

	// Grafana-shape split: the registry (integration service) stores configs and
	// secrets and knows nothing about transports; the transport resolver owns the
	// kind->builder mapping and the delivery cache. The registry's post-commit
	// onChange hook drives the resolver's cache invalidation. Dev use_stub swaps
	// in the stub implementation once here — no per-delivery branch.
	liveResolver := transportresolver.New(integrationSrv, transportresolver.Builders())
	integrationSrv.SetOnChange(liveResolver.Invalidate)

	return liveResolver
}

// rotateDataKeys re-wraps every stored DEK under the active KEK at startup, so
// a KEK rotation ships as config changes only — an operator never touches the DB
// by hand. It is a fast no-op when every DEK already sits on the active KEK.
//
// Rotation MUST be two-phase (add the new KEK to local_keys first, flip
// active_kek_uri in a separate later deploy — see config.CryptoConfig): after
// the flip boots, every DEK is wrapped by the new KEK, and a rollback to a
// config that predates the key would skip-and-Warn here, boot, and then fail
// every secret unwrap at runtime. Rolling back only as far as the phase-A
// config is safe by construction: it knows the new KEK, so this same re-wrap
// simply rotates everything back onto its active KEK. A
// rotation error is returned so startup fails fast — consistent with the keyring's
// fail-fast and the atomic, all-or-nothing rotation transaction (no partial
// re-wrap can persist). A row under an unknown KEK URI is skipped, not an error
// (orphaned/foreign rows on a shared dev DB must not block boot) — but skipped>0
// can also mean a retired KEK was dropped from config too early, permanently
// orphaning a live secret, so it is surfaced as a Warn for alerting rather than
// buried in the rotator's Info summary.
func rotateDataKeys(ctx context.Context, stores *Stores, keyring *secrets.Keyring) error {
	rotator := dekrotator.NewRotator(stores.TxManager, stores.DataKeys, keyring)
	rewrapped, skipped, err := rotator.Rotate(ctx)
	if err != nil {
		return fmt.Errorf("rotate data keys onto active KEK: %w", err)
	}
	if skipped > 0 {
		xlog.Warn(ctx, "kek rotation skipped deks under unknown keks; live secrets sealed by them are unreadable",
			xfield.Int("skipped", skipped),
			xfield.Int("rewrapped", rewrapped),
		)
	}
	return nil
}
