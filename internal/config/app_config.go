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

// AppConfig holds the complete application configuration including servers and database.
type AppConfig struct {
	Environment Environment
	InfraServer HTTPServer
	APIServer   HTTPServer
	DB          DB
}

func (c *AppConfig) IsDevEnvironment() bool {
	return c.Environment.IsDev()
}

func initConfig() *AppConfig {
	viper.SetDefault("DB_DSN", "not set")
	viper.SetDefault("DB_DRIVER", "postgres")
	viper.SetDefault("DB_MAX_OPEN_CONN", 25)
	viper.SetDefault("DB_MAX_IDLE_CONNS", 5)
	viper.SetDefault("DB_CONNECTION_MAX_IDLE_TIME", "5m")
	viper.SetDefault("DB_CONNECTIONS_MAX_LIFETIME", "1m")
	viper.AutomaticEnv()

	return &AppConfig{
		Environment: Environment(viper.GetString("ENVIRONMENT")),
		APIServer: HTTPServer{
			Name: "api",
			Host: "localhost",
			Port: 8000,
		},
		InfraServer: HTTPServer{
			Name: "status",
			Host: "localhost",
			Port: 8001,
		},
		DB: DB{
			DSN:             viper.GetString("DB_DSN"),
			Driver:          viper.GetString("DB_DRIVER"),
			MaxIdleConn:     viper.GetInt("DB_MAX_IDLE_CONNS"),
			MaxOpenConn:     viper.GetInt("DB_MAX_OPEN_CONNS"),
			ConnMaxIdleTime: viper.GetDuration("DB_CONNECTION_MAX_IDLE_TIME"),
			ConnMaxLifetime: viper.GetDuration("DB_CONNECTIONS_MAX_LIFETIME"),
		},
	}
}
