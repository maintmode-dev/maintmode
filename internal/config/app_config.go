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
	OAuthStateSigningKey            string        `mapstructure:"oauth_state_signing_key"`
	OAuthStateTTL                   time.Duration `mapstructure:"oauth_state_ttl"`
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

type SlackConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BotToken string `mapstructure:"bot_token"`
	// APIURL overrides the default Slack API base URL. Leave empty in
	// production; set for integration tests against an httptest mock,
	// for egress through a corporate proxy, or for a self-hosted Slack
	// API endpoint.
	APIURL  string        `mapstructure:"api_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type TelegramConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BotToken string `mapstructure:"bot_token"`
	// APIURL overrides the default Telegram Bot API base URL. Leave
	// empty in production; set for integration tests, corporate
	// proxies, or a self-hosted telegram-bot-api server.
	APIURL  string        `mapstructure:"api_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type NotifyTransportConfig struct {
	UseStub  bool           `mapstructure:"use_stub"`
	Slack    SlackConfig    `mapstructure:"slack"`
	Telegram TelegramConfig `mapstructure:"telegram"`
}

type TaskProcessorConfig struct {
	Messaging TaskProcessorMessagingConfig `mapstructure:"messaging"`
}

type TaskProcessorMessagingConfig struct {
	Workers     int   `mapstructure:"workers"`
	MaxAttempts int32 `mapstructure:"max_attempts"`
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

type S2S struct {
	Secret       string   `mapstructure:"secret"`
	AvailableAPI []string `mapstructure:"available_api"`
}

// S2SConfig is a map of S2S services.
// The key is the service name and the value is the service configuration.
type S2SConfig = map[string]S2S

type ExternalService struct {
	Secret   string        `mapstructure:"secret"`
	Protocol string        `mapstructure:"protocol"`
	Host     string        `mapstructure:"host"`
	Port     int           `mapstructure:"port"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

func (e ExternalService) GetURL() string {
	switch e.Protocol {
	case "http", "https":
		return fmt.Sprintf("%s://%s:%d", e.Protocol, e.Host, e.Port)
	case "grpc", "grpcs":
		return fmt.Sprintf("%s:%d", e.Host, e.Port)
	default:
		return fmt.Sprintf("%s://%s:%d", e.Protocol, e.Host, e.Port)
	}
}

// AppConfig holds the complete application configuration including servers and database.
type AppConfig struct {
	App              App                        `mapstructure:"app"`
	Environment      Environment                `mapstructure:"environment"`
	JWTVerifier      JWTVerifierConfig          `mapstructure:"jwtverifier"`
	RBAC             RbacConfig                 `mapstructure:"rbac"`
	InfraServer      HTTPServer                 `mapstructure:"infra_server"`
	APIServer        HTTPServer                 `mapstructure:"api_server"`
	Shutdown         Shutdown                   `mapstructure:"shutdown"`
	Tracer           Tracer                     `mapstructure:"tracer"`
	Logger           LoggerConfig               `mapstructure:"logger"`
	DB               DB                         `mapstructure:"db"`
	Redis            Redis                      `mapstructure:"redis"`
	OauthProviders   OauthProviders             `mapstructure:"oauth_providers"`
	JWT              JWT                        `mapstructure:"jwt"`
	S2SConfig        S2SConfig                  `mapstructure:"s2s"`
	ExternalServices map[string]ExternalService `mapstructure:"external_services"`
	NotifyTransport  NotifyTransportConfig      `mapstructure:"notify_transport"`
	TaskProcessor    TaskProcessorConfig        `mapstructure:"taskprocessor"`
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

	return cfg
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
