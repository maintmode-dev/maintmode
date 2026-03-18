// Package config provides application configuration management.
// It includes HTTP server, database settings, and configuration initialization.
package config

import "github.com/ruko1202/maintmode/internal/config/buildmeta"

var appConfig *AppConfig

func init() {
	appConfig = initConfig()
}

// GetAppConfig returns the initialized application configuration.
func GetAppConfig() *AppConfig {
	return appConfig
}

// GetAppBuildMeta returns the initialized application build metadata.
func GetAppBuildMeta() *buildmeta.AppBuildMeta {
	return buildmeta.GetAppBuildMeta()
}
