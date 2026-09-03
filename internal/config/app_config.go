package config

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"log"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/spf13/viper"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// HTTPServer represents HTTP server configuration with host and port settings.
type HTTPServer struct {
	Name string `mapstructure:"name"`
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	// RateLimiter caps the unauthenticated login/oauth and invitation surfaces,
	// keyed by IP. Its threshold is an anti-enumeration ceiling: the token in
	// the link is the only credential there, so it is deliberately low.
	RateLimiter RateLimiterConfig `mapstructure:"rate_limiter"`
	// UIRateLimiter caps the token-gated /ui/v1 screen routes, keyed by user.
	// It is a separate block rather than a reuse of RateLimiter because the two
	// answer different questions: the anti-enumeration ceiling above is set low
	// enough to be lethal to a screen group, whose routes are called in batches
	// on every page load. Same shape, different meaning.
	UIRateLimiter RateLimiterConfig `mapstructure:"ui_rate_limiter"`
	// Timeouts are the socket-level deadlines stamped onto the http.Server.
	// They are connection hygiene, not request budgets: the cheapest defense
	// against slowloris and against connections held open for free.
	Timeouts ServerTimeouts `mapstructure:"timeouts"`
}

// ServerTimeouts carries the four http.Server deadlines. A zero field means
// "use the default from TimeoutsOrDefault", NOT "no timeout": these exist to
// bound untrusted clients, so an omitted config key must not silently disarm
// them. Set a field negative to mean "no deadline" explicitly.
type ServerTimeouts struct {
	ReadHeader time.Duration `mapstructure:"read_header"`
	Read       time.Duration `mapstructure:"read"`
	Write      time.Duration `mapstructure:"write"`
	Idle       time.Duration `mapstructure:"idle"`
}

// Socket-level deadline defaults. Each is pinned to something observable in
// this deployment rather than picked for looking round; the couplings are why
// they must not be tuned in isolation.
const (
	// defaultReadHeaderTimeout is the one that closes slowloris on the header
	// phase: a client dribbling headers is cut off here.
	defaultReadHeaderTimeout = 10 * time.Second
	// defaultReadTimeout matches the 30s Echo already pre-sets on the server it
	// builds (its own gosec G112 default), so adopting these defaults changes
	// nothing about the read path. It is stated explicitly rather than inherited
	// so an Echo upgrade cannot move it silently.
	defaultReadTimeout = 30 * time.Second
	// defaultWriteTimeout bounds a slow response write. Nothing on the API
	// server streams and no middleware enforces a longer per-request budget, so
	// this is above every real handler. It must stay above any per-request
	// timeout added later — a shorter socket deadline severs a handler the
	// application was still willing to run.
	defaultWriteTimeout = 30 * time.Second
	// defaultIdleTimeout must exceed the polling interval of everything that
	// talks to these servers on a schedule, or each poll pays a fresh handshake
	// instead of reusing its connection. The binding constraints are Caddy's
	// health_interval (2s, deployment/caddy/Caddyfile) and the Prometheus and
	// Alloy scrape_interval (15s, monitoring/config). 75s clears all three with
	// room for a slower scrape config.
	defaultIdleTimeout = 75 * time.Second
)

// TimeoutsOrDefault fills unset fields with the defaults above, so existing
// config files keep working and gain the deadlines rather than opting out of
// them by omission.
//
// A negative value is preserved as an explicit "no deadline" and reaches the
// server as zero, which is how net/http spells unlimited. That is the escape
// hatch for a route that must outlive a deadline — see InfraServerTimeouts,
// where pprof needs exactly that.
func (t ServerTimeouts) TimeoutsOrDefault() ServerTimeouts {
	return ServerTimeouts{
		ReadHeader: xtime.OrDefaultDuration(t.ReadHeader, defaultReadHeaderTimeout),
		Read:       xtime.OrDefaultDuration(t.Read, defaultReadTimeout),
		Write:      xtime.OrDefaultDuration(t.Write, defaultWriteTimeout),
		Idle:       xtime.OrDefaultDuration(t.Idle, defaultIdleTimeout),
	}
}

// InfraServerTimeouts is TimeoutsOrDefault with the write deadline dropped.
//
// The infra server serves /debug/pprof, and a profile is a deliberately slow
// response: process_cpu blocks for the whole sampling window (30s for Alloy's
// scrape, longer for a human running `go tool pprof -seconds 60`) before the
// first byte is useful. Any fixed WriteTimeout cuts that off mid-profile and
// turns profiling into a truncated-body error, so this port gets none.
//
// Dropping it is affordable precisely here and nowhere else: this port is not
// public — it is bound for the proxy, the scrapers and operators — so the
// slow-write budget it gives away is not exposed to untrusted clients. The
// three read/idle deadlines still apply, so a slowloris on the header phase is
// still closed. An explicit positive write value in config still wins, for an
// operator who has decided their infra port needs the cap more than it needs
// long profiles.
func (t ServerTimeouts) InfraServerTimeouts() ServerTimeouts {
	return ServerTimeouts{
		ReadHeader: xtime.OrDefaultDuration(t.ReadHeader, defaultReadHeaderTimeout),
		Read:       xtime.OrDefaultDuration(t.Read, defaultReadTimeout),
		// No default to fall back to: an unset write deadline stays unset here,
		// which is the whole point of this method. Only an explicit positive
		// value from config survives.
		Write: xtime.OrDefaultDuration(t.Write, 0),
		Idle:  xtime.OrDefaultDuration(t.Idle, defaultIdleTimeout),
	}
}

type RateLimiterConfig struct {
	RequestsPerMinute int           `mapstructure:"requests_per_minute"`
	Burst             int           `mapstructure:"burst"`
	ExpiresIn         time.Duration `mapstructure:"expires_in"`
	Timeout           time.Duration `mapstructure:"timeout"`
}

// Shutdown controls graceful-shutdown behavior. On SIGTERM the process
// first marks itself not-ready (Readiness → 503) and waits DrainTimeout
// before closing the HTTP server, giving the reverse proxy time to eject
// this replica from its load-balancing pool. This is the in-process half
// of zero-downtime rolling deploys (the deploy script is the other half).
type Shutdown struct {
	DrainTimeout time.Duration `mapstructure:"drain_timeout"`
}

// DrainTimeoutOrDefault returns the configured drain delay, falling back to
// a safe default when unset so existing config files keep working.
func (s Shutdown) DrainTimeoutOrDefault() time.Duration {
	if s.DrainTimeout <= 0 {
		return defaultDrainTimeout
	}
	return s.DrainTimeout
}

// defaultDrainTimeout must comfortably exceed the proxy's active health-check
// interval so the replica is ejected from the pool before the server stops.
const defaultDrainTimeout = 6 * time.Second

// DB represents database connection configuration including pool settings.
type DB struct {
	DSN             string        `mapstructure:"dsn"`
	Driver          string        `mapstructure:"driver"`
	MaxOpenConn     int           `mapstructure:"max_open_conns"`
	MaxIdleConn     int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"connection_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"connection_max_idle_time"`
}

type Valkey struct {
	Address  string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// GoogleOauthProvider configures ID-token verification only.
//
// There is deliberately no client_secret, redirect_url or scopes: the BFF
// (maintmode-ui, NextAuth) owns the authorization-code exchange with Google
// and posts us the id_token. Those three configure a token-endpoint round
// trip this service never makes. We verify the token offline against Google's
// JWKS, and the only thing we need from the OAuth client is ClientID, as the
// expected audience. Do not reintroduce a secret here — it would be an unused
// copy of a credential that only the BFF needs.
type GoogleOauthProvider struct {
	ClientID  string            `mapstructure:"client_id"`
	JWTVerify JWTVerifierConfig `mapstructure:"jwtverifier"`
}

// OauthProviders has no `stub` section: the stub short-circuits verification in
// dev and reads nothing from config, so there is no StubOauthProvider type.
// UseStub (gated on IsDev) is the only stub-related knob.
type OauthProviders struct {
	UseStub bool                `mapstructure:"use_stub"`
	Google  GoogleOauthProvider `mapstructure:"google"`
}

type JWT struct {
	PrivateKey                      string        `mapstructure:"issuer_private_key"`
	Issuer                          string        `mapstructure:"issuer_name"`
	Kid                             string        `mapstructure:"issuer_kid"`
	AccessTokenTTL                  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL                 time.Duration `mapstructure:"refresh_token_ttl"`
	RefreshTokenGracePeriod         time.Duration `mapstructure:"refresh_token_grace_period"`
	RefreshTokenrDistributedLockTTL time.Duration `mapstructure:"refresh_token_distributed_lock_ttl"`
}

// ParsePrivateKey is the error-returning form of the signing-key parse. Callers
// outside the bootstrap path — where a config typo must surface as an error
// rather than as a stack trace from deep inside service wiring — use this one.
func (j JWT) ParsePrivateKey() (*ecdsa.PrivateKey, error) {
	privateKey, err := hex.DecodeString(j.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return key, nil
}

// PublicKey derives just the public half, so a token verifier can be handed the
// key it needs without the private half leaking into its package.
func (j JWT) PublicKey() (*ecdsa.PublicKey, error) {
	key, err := j.ParsePrivateKey()
	if err != nil {
		return nil, err
	}

	return &key.PublicKey, nil
}

// GeneratePrivateKey stays panicking because the bootstrap path that builds the
// token service has nowhere to return an error to: an unusable signing key means
// the process must not come up at all. It is a thin wrapper so the parsing logic
// lives in exactly one place.
func (j JWT) GeneratePrivateKey() *ecdsa.PrivateKey {
	key, err := j.ParsePrivateKey()
	if err != nil {
		xlog.Panic(context.Background(), "failed to parse private key", xfield.Error(err))
	}

	return key
}

type Tracer struct {
	CollectorHost string `mapstructure:"collector_host"`
	CollectorPort int32  `mapstructure:"collector_port"`
}

type LoggerConfig struct {
	Level string `mapstructure:"level"`
}

type App struct {
	FrontendURL string `mapstructure:"frontend_url"`
	// InvitationTTL is how long a user invitation link stays valid. Zero falls
	// back to a 7-day default at wiring time.
	InvitationTTL time.Duration `mapstructure:"invitation_ttl"`
}

// Auth holds the authentication signup policy.
type Auth struct {
	// AllowOpenSignup lets an unknown, uninvited user self-register as guest on
	// OAuth login. Default false: once the first admin exists, login of an
	// unknown user without an invitation is rejected (invite-only). Read at
	// startup, per-replica — a config rollout may briefly diverge across
	// replicas. Bootstrap correctness does not depend on it: the first-admin
	// decision is first-login-wins and takes no advisory lock, resting instead
	// on the operational model that the operator logs in before any other
	// traffic reaches the instance (see GetOrCreateByAuthInfo).
	AllowOpenSignup bool `mapstructure:"allow_open_signup"`
}

// NotifyTransportConfig holds process-level notify-delivery toggles. Per-transport
// credentials (Slack/Telegram/SMTP) live in the DB-backed integration registry,
// not here; the only remaining knob is the dev stub short-circuit.
type NotifyTransportConfig struct {
	// UseStub, in a dev environment, routes every delivery to the stub transport
	// instead of the real DB-resolved one — no external calls in local dev.
	UseStub bool `mapstructure:"use_stub"`
}

type TaskProcessorConfig struct {
	Messaging        TaskProcessorMessagingConfig        `mapstructure:"messaging"`
	MaintAutoCancel  TaskProcessorMaintAutoCancelConfig  `mapstructure:"maint_auto_cancel"`
	AuditPrune       TaskProcessorAuditPruneConfig       `mapstructure:"audit_prune"`
	InvitationRotate TaskProcessorInvitationRotateConfig `mapstructure:"invitation_rotate"`
	InvitationPrune  TaskProcessorInvitationPruneConfig  `mapstructure:"invitation_prune"`
}

// CryptoConfig addresses the master keys (KEKs) that wrap the data-encryption
// keys protecting integration secrets at rest. A KEK is addressed by URI; the
// scheme routes it to a provider: local-kms://<id> is served from LocalKeys,
// and a cloud KMS deployment points ActiveKEKURI at the provider's URI
// (gcp-kms://…, aws-kms://…) instead — the config contract does not change.
// Key values come from the secrets file — never inline. The keyring fails fast
// on an empty/unsupported/weak KEK so the binary refuses to start rather than
// silently protecting secrets with an unusable key.
type CryptoConfig struct {
	// ActiveKEKURI addresses the KEK that wraps new DEKs. It is also stamped
	// onto data_keys.kek_id.
	ActiveKEKURI string `mapstructure:"active_kek_uri"`
	// LocalKeys maps local-kms:// URI -> hex-encoded 32-byte key for the local
	// KEK provider. Holds the active KEK plus any retired-but-still-referenced
	// KEKs needed to unwrap existing DEKs. Empty when the KEK lives in a cloud
	// KMS.
	//
	// Rotation is TWO-PHASE, because startup re-wraps data_keys onto
	// ActiveKEKURI (see bootstrap rotateDataKeys) and a rollback boots an older
	// config against the re-wrapped rows:
	//   phase A: add the new KEK to LocalKeys, keep ActiveKEKURI on the old one;
	//   phase B: flip ActiveKEKURI to the new KEK (boot re-wraps all DEKs).
	// Rollback from B to A is self-healing (A knows the new KEK, its boot
	// re-wraps everything back); rollback PAST phase A — to a config that has
	// never seen the new KEK — leaves every DEK unreadable until the key is
	// added. Invariant: every KEK referenced by data_keys.kek_id must be present
	// in the LocalKeys of ANY config a deploy may roll back to. Remove a retired
	// KEK only after phase B is deployed everywhere and a rollback across the
	// flip is no longer possible.
	LocalKeys map[string]string `mapstructure:"local_keys"`
}

// LicenseConfig wires the instance to the closed MaintMode Console license
// server, which exists only for the paid seat-based SaaS offering. Self-hosted
// deployments leave the whole block empty — the sample configs under
// deployment/ ship no license section — so Enabled() reports false, the
// heartbeat processor is never registered, the license client is never built,
// no seat cap applies and nothing is sent anywhere. With both fields set, the
// instance heartbeats Console, caches the returned license in its own DB and
// enforces its status: a blocked license rejects every mutating business
// request (the block gate).
type LicenseConfig struct {
	// URL is the Console base URL; the heartbeat goes to
	// {URL}/cloud/v1/instances/heartbeat.
	URL string `mapstructure:"url"`
	// InstanceToken authenticates this instance (Authorization: Bearer). Comes
	// from the secrets file via <secret:...> — never inline in the config.
	InstanceToken string `mapstructure:"instance_token"`
	// CronSpec is the 5-field schedule of the heartbeat producer job. Zero falls
	// back to "* * * * *" (~60s, the contract cadence) at wiring time.
	CronSpec string `mapstructure:"cron_spec"`
	// HTTPTimeout bounds one heartbeat request. Zero falls back to 10s at wiring
	// time. There is no retry: the next cron tick is the retry, and a failed tick
	// just keeps the cached license (warning-level log).
	HTTPTimeout         time.Duration `mapstructure:"http_timeout"`
	CacheReloadInterval time.Duration `mapstructure:"cache_reload_interval"`
}

// Enabled reports whether the SaaS license mode is on. Both knobs are required,
// so a half-set block leaves the feature entirely off rather than half-armed:
// an instance with a url but no token cannot authenticate a heartbeat, and one
// with a token but no url has nowhere to send it.
func (c LicenseConfig) Enabled() bool {
	return c.URL != "" && c.InstanceToken != ""
}

type TaskProcessorMessagingConfig struct {
	Workers     int   `mapstructure:"workers"`
	MaxAttempts int32 `mapstructure:"max_attempts"`
	// FetchTick is how often a processor polls the queue for new tasks. Zero
	// means "leave goque's default" (30s), which is what every deployed stand
	// wants: the queue is durable, so a slow poll costs latency, not delivery.
	//
	// The API suite is the exception. It asserts on rows an outbox processor
	// writes after the request returns, so its budget has to cover a full poll
	// cycle — at 30s that made the audit assertions a race against the phase
	// offset of the tick, and a loaded shared database pushed drain latency
	// past two and a half minutes. Polling faster there removes the wait
	// instead of widening the timeout that was papering over it.
	FetchTick time.Duration `mapstructure:"fetch_tick"`
}

// TaskProcessorMaintAutoCancelConfig tunes the auto-cancel sweep of overdue
// not-started maintenances (see services/maint.Service.CancelUnStarted).
type TaskProcessorMaintAutoCancelConfig struct {
	// CronSpec is the 5-field schedule for the producer job (e.g. "* * * * *").
	CronSpec string `mapstructure:"cron_spec"`
	// Threshold is the grace window after a maintenance's planned start before it
	// is auto-canceled.
	Threshold time.Duration `mapstructure:"threshold"`
	// BatchLimit bounds how many overdue maintenances one sweep cancels.
	BatchLimit int64 `mapstructure:"batch_limit"`
}

// TaskProcessorAuditPruneConfig tunes the audit-log retention sweep (see
// services/auditor.Auditor.Prune). Owned by the auth binary, which owns the audit
// store.
type TaskProcessorAuditPruneConfig struct {
	// CronSpec is the 5-field schedule for the producer job (e.g. "0 3 * * *" =
	// daily at 03:00). The task is day-bucketed, so firing more often than daily
	// just dedupes to one prune per day.
	CronSpec string `mapstructure:"cron_spec"`
	// Retention is the age threshold: audit rows whose created_at is older than
	// now-Retention are deleted (e.g. 8760h = 365 days).
	Retention time.Duration `mapstructure:"retention"`
	// BatchLimit bounds how many rows one DELETE statement removes; the sweep loops
	// batches until the table is drained for the cutoff.
	BatchLimit int64 `mapstructure:"batch_limit"`
}

// TaskProcessorInvitationRotateConfig tunes the invitation-rotation sweep that
// flips pending invitations past their expiry to the persisted 'expired' status
// (see services/invitation.Service.Rotate).
type TaskProcessorInvitationRotateConfig struct {
	// CronSpec is the 5-field schedule for the producer job (e.g. "0 3 * * *" =
	// daily at 03:00). The task is day-bucketed, so firing more often than daily
	// just dedupes to one rotation per day.
	CronSpec string `mapstructure:"cron_spec"`
	// BatchLimit bounds how many rows one UPDATE statement flips; the sweep loops
	// batches until nothing is left past the expiry boundary.
	BatchLimit int64 `mapstructure:"batch_limit"`
}

// TaskProcessorInvitationPruneConfig tunes the invitation retention sweep that
// deletes terminal invitations older than the window (see
// services/invitation.Service.Prune).
type TaskProcessorInvitationPruneConfig struct {
	// CronSpec is the 5-field schedule for the producer job (e.g. "0 3 * * *" =
	// daily at 03:00). The task is day-bucketed, so firing more often than daily
	// just dedupes to one prune per day.
	CronSpec string `mapstructure:"cron_spec"`
	// Retention is the age threshold: terminal invitations whose created_at is
	// older than now-Retention are deleted (e.g. 8760h = 365 days).
	Retention time.Duration `mapstructure:"retention"`
	// BatchLimit bounds how many rows one DELETE statement removes; the sweep loops
	// batches until the table is drained for the cutoff.
	BatchLimit int64 `mapstructure:"batch_limit"`
}

type JWTVerifierConfig struct {
	// This struct is shared by two verifiers that use DIFFERENT issuer fields.
	// Set the one your consumer reads; validateIssuerConfig enforces both at
	// startup so a missing value cannot silently change behavior.
	//
	// JWTIssuer (singular) — read by internal/services/jwtverifier for our OWN
	// access tokens, via jwt.WithIssuer. Exactly one issuer.
	JWTIssuer string `mapstructure:"jwt_issuer"`
	// JWTIssuers (plural) — read by the googleoauth provider for Google ID
	// tokens, via validation.In. A list because Google mints both
	// "accounts.google.com" and "https://accounts.google.com" and either is
	// valid, which is why jwt.WithIssuer cannot express it.
	JWTIssuers []string `mapstructure:"jwt_issuers"`
	JWKSURL    string   `mapstructure:"jwks_url"`
	// AllowedHostedDomains, when non-empty, restricts ID tokens to those
	// whose `hd` claim matches one of the listed domains.
	AllowedHostedDomains      []string      `mapstructure:"allowed_hosted_domains"`
	JWKSRefreshInterval       time.Duration `mapstructure:"jwks_refresh_interval"`
	JWKSHTTPTimeout           time.Duration `mapstructure:"jwks_http_timeout"`
	JWTLeeway                 time.Duration `mapstructure:"jwt_leeway"`
	JWKSUnknownKIDRefreshRate time.Duration `mapstructure:"jwks_unknown_kid_refresh_rate"`
	JWKSUnknownKIDWaitMax     time.Duration `mapstructure:"jwks_unknown_kid_wait_max"`
}

type RbacConfig struct {
	Adapter string `mapstructure:"adapter"`
	// Model and Policy are file names (not full paths) read from YAML.
	// Their directory comes from <APP>_AUTHZ_DIR env.
	Model  string `mapstructure:"model"`
	Policy string `mapstructure:"policy"`
	// ModelPath and PolicyPath are derived at startup as
	// path.Join(AUTHZ_DIR, Model|Policy) and are not read from YAML.
	ModelPath  string `mapstructure:"-"`
	PolicyPath string `mapstructure:"-"`
	// PolicyData is the inline policy used by tests with the in-memory
	// adapter (instead of reading from disk).
	PolicyData string `mapstructure:"-"`
}

// AppConfig holds the complete application configuration including servers and database.
type AppConfig struct {
	App             App                   `mapstructure:"app"`
	Auth            Auth                  `mapstructure:"auth"`
	Environment     Environment           `mapstructure:"environment"`
	JWTVerifier     JWTVerifierConfig     `mapstructure:"jwtverifier"`
	RBAC            RbacConfig            `mapstructure:"rbac"`
	InfraServer     HTTPServer            `mapstructure:"infra_server"`
	APIServer       HTTPServer            `mapstructure:"api_server"`
	Shutdown        Shutdown              `mapstructure:"shutdown"`
	Tracer          Tracer                `mapstructure:"tracer"`
	Logger          LoggerConfig          `mapstructure:"logger"`
	DB              DB                    `mapstructure:"db"`
	Valkey          Valkey                `mapstructure:"valkey"`
	OauthProviders  OauthProviders        `mapstructure:"oauth_providers"`
	JWT             JWT                   `mapstructure:"jwt"`
	NotifyTransport NotifyTransportConfig `mapstructure:"notify_transport"`
	TaskProcessor   TaskProcessorConfig   `mapstructure:"task_processor"`
	Crypto          CryptoConfig          `mapstructure:"crypto"`
	License         LicenseConfig         `mapstructure:"license"`
	Bootstrap       BootstrapConfig       `mapstructure:"bootstrap"`
}

// BootstrapConfig configures the break-glass admin sign-in — the emergency
// login that breaks the "to configure a provider you must sign in, to sign in
// you must configure a provider" loop.
//
// Password is the only secret here and it never reaches the database: it comes
// from the secrets file (key "bootstrap/password", resolved by applySecrets)
// and from nowhere else. There is deliberately no environment-variable
// override: an env var is readable by anything that can inspect the process or
// the compose file, which is a wider blast radius than the secrets file this
// deployment already treats as the one place credentials live.
//
// An empty value is not a misconfiguration, it is the signal to generate a
// random one at startup and log it once — see bootstrapauth.ResolvePassword.
// The local/dev/test samples ship exactly that, so validateBootstrapConfig must
// keep accepting an empty password. The KEY, however, must be present in the
// secrets file: the resolver hard-fails on a missing one.
//
// Email determines the identity of the break-glass admin. It deliberately comes
// from configuration rather than the request body: whoever controls the
// deployment decides who the admin is, not whoever guessed the password, and a
// later sign-in resolves to the same user instead of creating a second one.
type BootstrapConfig struct {
	Email    string `mapstructure:"email"`
	Password string `mapstructure:"password"`
}

const (
	// CONFIG_DIR is the directory holding both the app config and the
	// secrets file. CONFIG_FILE / SECRETS_FILE name them within that
	// directory. Split into two env vars (instead of a single
	// APP_CONFIG_PATH) so callers can keep file names as defaults and
	// only override the directory per environment.
	configDirEnv       = "CONFIG_DIR"
	configFileEnv      = "CONFIG_FILE"
	secretsFileEnv     = "SECRETS_FILE" //nolint:gosec
	defaultConfigFile  = "app.config.yaml"
	defaultSecretsFile = "app.secrets.yaml" //nolint:gosec // file name, not a credential

	// AUTHZ_DIR is the directory holding RBAC model + policy files.
	// File names themselves come from rbac.model / rbac.policy in YAML.
	authzDirEnv = "AUTHZ_DIR"

	// secretPlaceholderMarker is the substring every shipped sample secret uses
	// for a value an operator must replace ("replace-me-prod-value"). Matching on
	// it turns a forgotten line into a startup error instead of a live
	// credential that is published in this repository.
	secretPlaceholderMarker = "replace-me"

	// minBootstrapPasswordLen follows NIST 800-63B: length is the requirement
	// that matters, character-class rules are not. It applies ONLY to a
	// configured password — an empty one means "generate at startup" and a
	// generated one is specified by entropy instead.
	minBootstrapPasswordLen = 12
)

func initConfig(appName string) *AppConfig {
	reader := viper.New()
	reader.SetEnvPrefix(appName)
	reader.AutomaticEnv()
	reader.SetDefault(configDirEnv, ".")
	reader.SetDefault(configFileEnv, defaultConfigFile)
	reader.SetDefault(secretsFileEnv, defaultSecretsFile)
	reader.SetDefault(authzDirEnv, ".")

	configDir := reader.GetString(configDirEnv)
	authzDir := reader.GetString(authzDirEnv)
	configPath := path.Join(configDir, reader.GetString(configFileEnv))
	secretsPath := path.Join(configDir, reader.GetString(secretsFileEnv))

	cfg, err := readConfig(configPath)
	if err != nil {
		log.Panicf("failed to read app '%s' config: %s", appName, err)
	}
	cfg.RBAC.ModelPath = path.Join(authzDir, cfg.RBAC.Model)
	cfg.RBAC.PolicyPath = path.Join(authzDir, cfg.RBAC.Policy)

	secrets, err := readSecrets(secretsPath)
	if err != nil {
		log.Panicf("failed to load secrets for service %s: %s", appName, err)
	}

	if err := cfg.applySecrets(secrets); err != nil {
		log.Panicf("failed to apply secrets for service %s: %s", appName, err)
	}

	// Environment first: the gates the other validators reason about are all
	// keyed off it, so an unknown value must stop the process before any of
	// them draws a conclusion from a silently-false IsProd().
	if err := cfg.validateEnvironment(); err != nil {
		log.Panicf("invalid config for service %s: %s", appName, err)
	}

	if err := cfg.validateUseStubInDev(); err != nil {
		log.Panicf("invalid config for service %s: %s", appName, err)
	}

	if err := cfg.validateJWTKey(); err != nil {
		log.Panicf("invalid config for service %s: %s", appName, err)
	}

	if err := cfg.validateIssuerConfig(); err != nil {
		log.Panicf("invalid config for service %s: %s", appName, err)
	}

	if err := cfg.validateInvitationRetention(); err != nil {
		log.Panicf("invalid config for service %s: %s", appName, err)
	}

	if err := cfg.validateValkeyConfig(); err != nil {
		log.Panicf("invalid config for service %s: %s", appName, err)
	}

	if err := cfg.validateBootstrapConfig(); err != nil {
		log.Panicf("invalid config for service %s: %s", appName, err)
	}

	return cfg
}

// validateBootstrapConfig rejects a configured break-glass password too short to
// be worth having. It deliberately does NOT reject an empty one: emptiness is
// the documented "generate a random password at startup" signal, and every
// shipped app.config.yaml carries it — a validator that treated empty as "too
// short" would panic every deployment at boot, which is precisely the outage
// this endpoint exists to prevent.
//
// A generated password is not checked here at all; it is specified by entropy
// (see bootstrapauth.ResolvePassword) and never passes through this function.
func (c *AppConfig) validateBootstrapConfig() error {
	// Email is checked FIRST, before the empty-password early return: a
	// generated password is the common case, and the identity still has to be
	// sound there. It is not decoration — linkBootstrapToExistingUser resolves
	// an account by this address and grants it admin, so an empty or
	// placeholder value is an admin grant pointed at the wrong row (or, when
	// empty, at a user created with an empty email that permanently occupies
	// the NOT NULL UNIQUE slot).
	if c.Bootstrap.Email == "" {
		return fmt.Errorf("bootstrap.email is required: it decides which account the break-glass login grants admin to")
	}

	if strings.Contains(c.Bootstrap.Email, secretPlaceholderMarker) {
		return fmt.Errorf(
			"bootstrap.email is a placeholder (%q): set the address of the operator who should hold break-glass access",
			c.Bootstrap.Email,
		)
	}

	if c.Bootstrap.Password == "" {
		return nil
	}

	if len(c.Bootstrap.Password) < minBootstrapPasswordLen {
		return fmt.Errorf(
			"bootstrap.password is too short: %d characters, expected at least %d "+
				"(leave it empty to have one generated at startup)",
			len(c.Bootstrap.Password), minBootstrapPasswordLen,
		)
	}

	// The sample secrets ship a "replace-me-prod-value" placeholder, and it is
	// long enough to clear the length floor above — so without this check an
	// operator who copies the prod sample and misses this one line boots a
	// production instance whose break-glass admin password is a string published
	// in this repository. validateJWTKey rejects its own placeholder for exactly
	// this reason.
	if strings.Contains(c.Bootstrap.Password, secretPlaceholderMarker) {
		return fmt.Errorf(
			"bootstrap.password is a placeholder (%q): set a real password in the secrets file, "+
				"or leave it empty to have one generated at startup",
			c.Bootstrap.Password,
		)
	}

	return nil
}

// validateEnvironment rejects any value outside the known set. IsProd() and
// IsDev() are exact string comparisons, so "Prod" or "production" would leave
// both false — a third state in which no IsProd() gate fires and every future
// one silently opens. Fail at startup instead. It panics-via-caller
// (initConfig) so the misconfiguration surfaces before any gate is consulted.
func (c *AppConfig) validateEnvironment() error {
	switch c.Environment {
	case ProdEnvironment, DevEnvironment, LocalEnvironment, PerformanceTestEnvironment:
		return nil
	default:
		return fmt.Errorf(
			"unknown environment %q: expected one of prod, dev, local, performance_test",
			c.Environment,
		)
	}
}

const (
	// minSigningKeyBits is the floor below which a P-256 scalar is a placeholder
	// rather than a generated key. See validateJWTKey for why it is not higher.
	minSigningKeyBits = 128
	// signingKeyBytes is the fixed width of a P-256 raw scalar.
	signingKeyBytes = 32
)

// validateJWTKey rejects a signing key that is obviously a placeholder rather
// than a generated one. It is a placeholder detector, NOT a measure of entropy:
// a key of 0xDEADBEEF repeated eight times passes both checks, and catching
// that is not a config validator's job.
//
// The two checks are orthogonal — neither subsumes the other:
//
//   - an implausibly small scalar (d=1, d=2) is caught by the bit-length floor.
//     128 bits is chosen so a genuine CSPRNG key is falsely rejected with
//     probability 2^127/n ≈ 1.5e-39. A higher floor is not free: at 250 bits it
//     would be 2^249/n ≈ 7.8e-3, rejecting roughly one honest key in 128 —
//     surfacing at a prod key rotation, never on CI;
//   - a key of one repeated byte (0x01×32, 0xAA×32) has a bit length of 249 and
//     256 respectively, so NO threshold catches it. It needs its own check.
//
// It panics-via-caller (initConfig) so the misconfiguration surfaces at startup.
func (c *AppConfig) validateJWTKey() error {
	key, err := c.JWT.ParsePrivateKey()
	if err != nil {
		return fmt.Errorf("jwt.issuer_private_key is unusable: %w", err)
	}

	if key.D.BitLen() < minSigningKeyBits {
		return fmt.Errorf(
			"jwt.issuer_private_key is a placeholder: scalar has %d bits, expected at least %d",
			key.D.BitLen(), minSigningKeyBits,
		)
	}

	raw := make([]byte, signingKeyBytes)
	key.D.FillBytes(raw)
	if bytes.Count(raw, []byte{raw[0]}) == signingKeyBytes {
		return fmt.Errorf(
			"jwt.issuer_private_key is a placeholder: all %d bytes are 0x%02x",
			signingKeyBytes, raw[0],
		)
	}

	return nil
}

// validateUseStubInDev enforces the cross-field invariant that would otherwise
// fail silently at runtime: use_stub routes every notify delivery to the
// in-memory stub and is gated on IsDev() at wiring time, so setting it in a
// non-dev environment is a silent no-op that a developer might expect to take
// effect. Reject it loudly instead of ignoring it, so a misplaced dev flag
// never masks real prod deliveries. It panics-via-caller (initConfig) so the
// misconfiguration surfaces at startup.
//
// Both use_stub flags are checked here, and the OAuth one matters most. The
// stub provider accepts ANY token and mints a random identity (see
// stuboauth/verify.go), so if its IsDev() gate at wiring time were ever
// loosened, a stray oauth_providers.use_stub: true carried into a non-dev
// config would turn every login into an unauthenticated one. dev's own
// app.config.yaml ships use_stub: true, which is exactly the file someone
// copies when seeding a new environment. Defense in depth: the runtime gate in
// authmethod.NewAuthMethods still stands, and this makes the config that
// would rely on it refuse to boot.
func (c *AppConfig) validateUseStubInDev() error {
	if c.Environment.IsDev() {
		return nil
	}

	if c.NotifyTransport.UseStub {
		return fmt.Errorf(
			"notify_transport.use_stub is only valid in a dev environment, got environment %q",
			c.Environment,
		)
	}

	if c.OauthProviders.UseStub {
		return fmt.Errorf(
			"oauth_providers.use_stub is only valid in a dev environment, got environment %q: "+
				"the stub provider accepts any token and mints a random identity",
			c.Environment,
		)
	}

	return nil
}

// validateValkeyConfig rejects an empty valkey.addr at startup.
//
// The key was renamed redis -> valkey, and viper unmarshals without
// ErrorUnused: a config file still carrying the old `redis:` block is not an
// error, it simply leaves Valkey as the Go zero value. That would be survivable
// if an empty address then failed to connect, but go-redis substitutes its own
// default instead (options.go: `if opt.Addr == "" { opt.Addr = "localhost:6379" }`),
// so the ping succeeds against whatever happens to answer on the local port and
// the process boots "healthy" while pointed at the wrong store — or, on a host
// running an unrelated instance, quietly shares it.
//
// The subsystems behind this client are the ones where that is least visible:
// the rate limiter silently stops being replica-shared, and blacklisted tokens
// and distributed locks stop being seen by the other replicas. The error names
// the rename because a stale `redis:` block is the overwhelmingly likely cause
// and is otherwise invisible. It panics-via-caller (initConfig) so the
// misconfiguration surfaces at startup.
func (c *AppConfig) validateValkeyConfig() error {
	if c.Valkey.Address == "" {
		return fmt.Errorf(
			"valkey.addr must not be empty: check the config uses the `valkey:` key " +
				"(renamed from `redis:`), otherwise the client silently defaults to localhost:6379",
		)
	}
	return nil
}

// validateInvitationRetention rejects a negative invitation-prune retention at
// startup. A negative retention would push the prune cutoff into the future and
// make every terminal invitation eligible for deletion; the service defensively
// clamps it, but a negative value in config is always an operator typo, so
// surface it loudly here rather than silently substituting a default. Zero is
// allowed — it means "unset" and the service applies its default. It
// panics-via-caller (initConfig) so the misconfiguration surfaces at startup.
func (c *AppConfig) validateInvitationRetention() error {
	if c.TaskProcessor.InvitationPrune.Retention < 0 {
		return fmt.Errorf(
			"task_processor.invitation_prune.retention must not be negative, got %s",
			c.TaskProcessor.InvitationPrune.Retention,
		)
	}
	return nil
}

// validateIssuerConfig rejects a verifier config that would silently stop
// checking the token issuer.
//
// JWTVerifierConfig carries two issuer fields for two consumers, and they fail
// in OPPOSITE directions when unset — which is why an unset one has to be an
// error rather than a default:
//
//   - jwtverifier (our own access tokens) reads the singular JWTIssuer and
//     passes it to jwt.WithIssuer. The library skips the check entirely on an
//     empty string, so a missing jwt_issuer FAILS OPEN: any issuer is accepted.
//   - googleoauth (Google ID tokens) reads the plural JWTIssuers and passes it
//     to validation.In. An empty list matches nothing, so a missing
//     jwt_issuers FAILS CLOSED: every token is rejected.
//
// One is a silent security hole, the other a total outage. Neither should be
// reachable by deleting a config line, and this is not hypothetical: the Google
// verifier previously also called jwt.WithIssuer with the singular field, which
// no config sets, so that check was dead in every environment until it was
// removed.
func (c *AppConfig) validateIssuerConfig() error {
	if c.JWTVerifier.JWTIssuer == "" {
		return fmt.Errorf(
			"jwtverifier.jwt_issuer must be set: an empty expected issuer makes jwt.WithIssuer skip the check, accepting any issuer",
		)
	}
	if len(c.OauthProviders.Google.JWTVerify.JWTIssuers) == 0 {
		return fmt.Errorf(
			"oauth_providers.google.jwtverifier.jwt_issuers must list at least one issuer: an empty allowlist rejects every Google ID token",
		)
	}
	return nil
}

func readConfig(filePath string) (*AppConfig, error) {
	reader := viper.New()
	reader.SetConfigFile(filePath)

	if err := reader.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file %s", filePath)
	}

	cfg := new(AppConfig)
	if err := reader.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", filePath, err)
	}

	return cfg, nil
}

func (c *AppConfig) applySecrets(secrets secretStore) error {
	resolver := secretResolver{secrets: secrets}

	return resolver.resolveValue(reflect.ValueOf(c))
}
