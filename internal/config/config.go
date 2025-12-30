// Package config provides application configuration management.
// It includes HTTP server, database settings, and configuration initialization.
package config

// AppName is the application name used throughout the system.
const (
	AppName = "maintmode"
)

var appConfig *AppConfig

func init() {
	appConfig = initConfig()
}

// GetAppConfig returns the initialized application configuration.
func GetAppConfig() *AppConfig {
	return appConfig
}
