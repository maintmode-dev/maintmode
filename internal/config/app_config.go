package config

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"log"
	"path"
	"reflect"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/spf13/viper"
)

// HTTPServer represents HTTP server configuration with host and port settings.
type HTTPServer struct {
	Name        string            `mapstructure:"name"`
	Host        string            `mapstructure:"host"`
	Port        int               `mapstructure:"port"`
	RateLimiter RateLimiterConfig `mapstructure:"rate_limiter"`
}

// BuildHostPort returns the server address in host:port format.
func (a *HTTPServer) BuildHostPort() string {
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
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

type Redis struct {
	Address  string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type GoogleOauthProvider struct {
	ClientID string `mapstructure:"client_id"`
	// ClientSecret is deprecated for production: the BFF-owned OAuth flow
	ClientSecret      string            `mapstructure:"client_secret"`
	RedirectURL       string            `mapstructure:"redirect_url"`
	GoogleUserInfoURL string            `mapstructure:"google_userinfo_url"`
	Scopes            []string          `mapstructure:"scopes"`
	JWTVerify         JWTVerifierConfig `mapstructure:"jwtverifier"`
}

type StubOauthProvider struct {
	RedirectURL string `mapstructure:"redirect_url"`
}

type OauthProviders struct {
	UseStub bool                `mapstructure:"use_stub"`
	Google  GoogleOauthProvider `mapstructure:"google"`
	Stub    StubOauthProvider   `mapstructure:"stub"`
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

func (j JWT) GeneratePrivateKey() *ecdsa.PrivateKey {
	privateKey, err := hex.DecodeString(j.PrivateKey)
	if err != nil {
		xlog.Panic(context.Background(), "failed to decode private key", xfield.Error(err))
	}
	key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), privateKey)
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
	// replicas. Bootstrap correctness does not depend on it (advisory lock in
	// the shared DB).
	AllowOpenSignup bool `mapstructure:"allow_open_signup"`
}

// NotifyTransportConfig holds process-level notify-delivery toggles. Per-transport
// credentials (Slack/Telegram/SMTP) moved to the DB-backed integration registry
// in RUK-196; the only remaining knob is the dev stub short-circuit.
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
// server (SaaS mode). Self-hosted deployments leave the whole block
// empty: Enabled() reports false, the license client never starts and no
// limits apply. With both fields set, the instance heartbeats Console, caches
// the returned license in its own DB and enforces its status: a blocked license
// rejects every mutating business request (the block gate).
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

// Enabled reports whether the SaaS license mode is on. Both knobs are required;
// a half-set block is rejected at startup by validateLicense.
func (c LicenseConfig) Enabled() bool {
	return c.URL != "" && c.InstanceToken != ""
}

type TaskProcessorMessagingConfig struct {
	Workers     int   `mapstructure:"workers"`
	MaxAttempts int32 `mapstructure:"max_attempts"`
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
	JWTIssuer  string   `mapstructure:"jwt_issuer"`  // JWTIssuer is the expected issuer of the JWT.
	JWTIssuers []string `mapstructure:"jwt_issuers"` // JWTIssuers is a list of expected issuers of the JWT.
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
	Redis           Redis                 `mapstructure:"redis"`
	OauthProviders  OauthProviders        `mapstructure:"oauth_providers"`
	JWT             JWT                   `mapstructure:"jwt"`
	NotifyTransport NotifyTransportConfig `mapstructure:"notify_transport"`
	TaskProcessor   TaskProcessorConfig   `mapstructure:"task_processor"`
	Crypto          CryptoConfig          `mapstructure:"crypto"`
	License         LicenseConfig         `mapstructure:"license"`
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

	if err := cfg.validateUseStubInDev(); err != nil {
		log.Panicf("invalid config for service %s: %s", appName, err)
	}

	if err := cfg.validateInvitationRetention(); err != nil {
		log.Panicf("invalid config for service %s: %s", appName, err)
	}

	return cfg
}

// validateUseStubInDev enforces the cross-field invariant that would otherwise
// fail silently at runtime: use_stub routes every notify delivery to the
// in-memory stub and is gated on IsDev() at wiring time, so setting it in a
// non-dev environment is a silent no-op that a developer might expect to take
// effect. Reject it loudly instead of ignoring it, so a misplaced dev flag
// never masks real prod deliveries. It panics-via-caller (initConfig) so the
// misconfiguration surfaces at startup.
func (c *AppConfig) validateUseStubInDev() error {
	if !c.Environment.IsDev() && c.NotifyTransport.UseStub {
		return fmt.Errorf(
			"notify_transport.use_stub is only valid in a dev environment, got environment %q",
			c.Environment,
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
