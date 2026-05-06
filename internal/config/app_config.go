package config

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/spf13/viper"
)

// HTTPServer represents HTTP server configuration with host and port settings.
type HTTPServer struct {
	Name string
	Host string
	Port int
}

// BuildHostPort returns the server address in host:port format.
func (a *HTTPServer) BuildHostPort() string {
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
}

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
	ClientID          string   `mapstructure:"client_id"`
	ClientSecret      string   `mapstructure:"client_secret"`
	RedirectURL       string   `mapstructure:"redirect_url"`
	GoogleUserInfoURL string   `mapstructure:"google_userinfo_url"`
	Scopes            []string `mapstructure:"scopes"`
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
}

type JWTVerifierConfig struct {
	JWTIssuer                 string        `mapstructure:"jwt_issuer"`
	JWKSURL                   string        `mapstructure:"jwks_url"`
	JWKSRefreshInterval       time.Duration `mapstructure:"jwks_refresh_interval"`
	JWKSHTTPTimeout           time.Duration `mapstructure:"jwks_http_timeout"`
	JWTLeeway                 time.Duration `mapstructure:"jwt_leeway"`
	JWKSUnknownKIDRefreshRate time.Duration `mapstructure:"jwks_unknown_kid_refresh_rate"`
	JWKSUnknownKIDWaitMax     time.Duration `mapstructure:"jwks_unknown_kid_wait_max"`
}

type RbacConfig struct {
	Adapter    string `mapstructure:"adapter"`
	ModelPath  string `mapstructure:"model_path"`
	PolicyPath string `mapstructure:"policy_path"`
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
	Secret   string `mapstructure:"secret"`
	Protocol string `mapstructure:"protocol"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
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
	Tracer           Tracer                     `mapstructure:"tracer"`
	Logger           LoggerConfig               `mapstructure:"logger"`
	DB               DB                         `mapstructure:"db"`
	Redis            Redis                      `mapstructure:"redis"`
	OauthProviders   OauthProviders             `mapstructure:"oauth_providers"`
	JWT              JWT                        `mapstructure:"jwt"`
	S2SConfig        S2SConfig                  `mapstructure:"s2s"`
	ExternalServices map[string]ExternalService `mapstructure:"external_services"`
}

const (
	configPathEnv = "APP_CONFIG_PATH"
	secretPathEnv = "APP_SECRETS_PATH" //nolint:gosec
)

func initConfig(appName string) *AppConfig {
	reader := viper.New()
	reader.SetEnvPrefix(appName)
	reader.AutomaticEnv()
	reader.SetDefault(configPathEnv, "app.config.yaml")
	reader.SetDefault(secretPathEnv, "app.secret.yaml")

	cfg, err := readConfig(reader.GetString(configPathEnv))
	if err != nil {
		log.Panicf("failed to read app '%s' config: %s", appName, err)
	}

	secrets, err := readSecrets(reader.GetString(secretPathEnv))
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
		return nil, fmt.Errorf("failed to unmarshal config file %s", filePath)
	}

	return cfg, nil
}

func (c *AppConfig) applySecrets(secrets secretStore) error {
	resolver := secretResolver{secrets: secrets}

	return resolver.resolveValue(reflect.ValueOf(c))
}
