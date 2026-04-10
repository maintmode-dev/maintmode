// Package config provides application configuration management.
// It includes HTTP server, database settings, and configuration initialization.
package config

import "github.com/ruko1202/maintmode/internal/config/buildmeta"

// LoadAppConfig returns the initialized application configuration.
func LoadAppConfig() *AppConfig {
	return initConfig(buildmeta.MaintModeAppName)
}

// LoadAuthAppConfig returns the initialized application configuration.
func LoadAuthAppConfig() *AppConfig {
	return initConfig(buildmeta.AuthAppName)
}

// GetAppBuildMeta returns the initialized application build metadata.
func GetAppBuildMeta() *buildmeta.AppBuildMeta {
	return buildmeta.GetAppBuildMeta()
}

// GetAuthAppBuildMeta returns the initialized application build metadata.
func GetAuthAppBuildMeta() *buildmeta.AppBuildMeta {
	return buildmeta.GetAuthAppBuildMeta()
}
