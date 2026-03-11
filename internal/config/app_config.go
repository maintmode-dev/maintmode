package config

import (
	"fmt"
	"time"

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
	DSN             string
	Driver          string
	MaxOpenConn     int
	MaxIdleConn     int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type Tracer struct {
	CollectorHost string
	CollectorPort int32
}

// AppConfig holds the complete application configuration including servers and database.
type AppConfig struct {
	Environment Environment
	InfraServer HTTPServer
	APIServer   HTTPServer
	DB          DB
	Tracer      Tracer
}

func (c *AppConfig) IsDevEnvironment() bool {
	return c.Environment.IsDev()
}

func initConfig() *AppConfig {
	viper.SetDefault("DB_DSN", "not set")
	viper.SetDefault("DB_DRIVER", "postgres")
	viper.SetDefault("DB_MAX_OPEN_CONNS", 25)
	viper.SetDefault("DB_MAX_IDLE_CONNS", 5)
	viper.SetDefault("DB_CONNECTION_MAX_IDLE_TIME", "5m")
	viper.SetDefault("DB_CONNECTION_MAX_LIFETIME", "1m")
	viper.SetDefault("APP_HOST", "0.0.0.0")
	viper.SetDefault("APP_PUBLIC_API_PORT", "8000")
	viper.SetDefault("APP_INFRA_API_PORT", "8001")
	viper.SetDefault("APP_TRACER_COLLECTOR_HOST", "0.0.0.0")
	viper.SetDefault("APP_TRACER_COLLECTOR_PORT", "4317")
	viper.AutomaticEnv()

	return &AppConfig{
		Environment: Environment(viper.GetString("ENVIRONMENT")),
		APIServer: HTTPServer{
			Name: "api",
			Host: viper.GetString("APP_HOST"),
			Port: viper.GetInt("APP_PUBLIC_API_PORT"),
		},
		InfraServer: HTTPServer{
			Name: "status",
			Host: viper.GetString("APP_HOST"),
			Port: viper.GetInt("APP_INFRA_API_PORT"),
		},
		DB: DB{
			DSN:             viper.GetString("DB_DSN"),
			Driver:          viper.GetString("DB_DRIVER"),
			MaxIdleConn:     viper.GetInt("DB_MAX_IDLE_CONNS"),
			MaxOpenConn:     viper.GetInt("DB_MAX_OPEN_CONNS"),
			ConnMaxIdleTime: viper.GetDuration("DB_CONNECTION_MAX_IDLE_TIME"),
			ConnMaxLifetime: viper.GetDuration("DB_CONNECTION_MAX_LIFETIME"),
		},
		Tracer: Tracer{
			CollectorHost: viper.GetString("APP_TRACER_COLLECTOR_HOST"),
			CollectorPort: viper.GetInt32("APP_TRACER_COLLECTOR_PORT"),
		},
	}
}
