// Package config provides application configuration management.
// It includes HTTP server, database settings, and configuration initialization.
package config

var appConfig *AppConfig

func init() {
	appConfig = initConfig()
}

// GetAppConfig returns the initialized application configuration.
func GetAppConfig() *AppConfig {
	return appConfig
}
