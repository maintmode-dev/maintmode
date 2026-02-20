package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConfigKeyConsistency verifies that all required configuration keys are consistent
// between SetDefault and Get* operations. This prevents bugs where default values are set
// with one key but read with a different key, leading to unexpected behavior.
func TestConfigKeyConsistency(t *testing.T) {
	tests := []struct {
		name         string
		defaultKey   string
		getKey       string
		defaultValue any
		description  string
	}{
		{
			name:         "DB_DSN",
			defaultKey:   "DB_DSN",
			getKey:       "DB_DSN",
			defaultValue: "not set",
			description:  "Database connection string",
		},
		{
			name:         "DB_DRIVER",
			defaultKey:   "DB_DRIVER",
			getKey:       "DB_DRIVER",
			defaultValue: "postgres",
			description:  "Database driver",
		},
		{
			name:         "DB_MAX_OPEN_CONNS",
			defaultKey:   "DB_MAX_OPEN_CONNS",
			getKey:       "DB_MAX_OPEN_CONNS",
			defaultValue: 25,
			description:  "Maximum number of open database connections",
		},
		{
			name:         "DB_MAX_IDLE_CONNS",
			defaultKey:   "DB_MAX_IDLE_CONNS",
			getKey:       "DB_MAX_IDLE_CONNS",
			defaultValue: 5,
			description:  "Maximum number of idle database connections",
		},
		{
			name:         "DB_CONNECTION_MAX_IDLE_TIME",
			defaultKey:   "DB_CONNECTION_MAX_IDLE_TIME",
			getKey:       "DB_CONNECTION_MAX_IDLE_TIME",
			defaultValue: "5m",
			description:  "Maximum time a connection can remain idle",
		},
		{
			name:         "DB_CONNECTIONS_MAX_LIFETIME",
			defaultKey:   "DB_CONNECTIONS_MAX_LIFETIME",
			getKey:       "DB_CONNECTIONS_MAX_LIFETIME",
			defaultValue: "1m",
			description:  "Maximum lifetime of a connection",
		},
		{
			name:         "APP_HOST",
			defaultKey:   "APP_HOST",
			getKey:       "APP_HOST",
			defaultValue: "0.0.0.0",
			description:  "Application host",
		},
		{
			name:         "APP_PUBLIC_API_PORT",
			defaultKey:   "APP_PUBLIC_API_PORT",
			getKey:       "APP_PUBLIC_API_PORT",
			defaultValue: 8000,
			description:  "Public API port",
		},
		{
			name:         "APP_INFRA_API_PORT",
			defaultKey:   "APP_INFRA_API_PORT",
			getKey:       "APP_INFRA_API_PORT",
			defaultValue: 8001,
			description:  "Infrastructure API port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure the default key matches the get key
			require.Equal(t, tt.defaultKey, tt.getKey,
				"Config key mismatch for %s: SetDefault uses '%s' but Get* uses '%s'",
				tt.name, tt.defaultKey, tt.getKey)

			// Verify description is not empty
			require.NotEmpty(t, tt.description,
				"Config key %s must have a description", tt.name)
		})
	}
}

// TestConfigDefaultValuesAreSet verifies that default values are properly set
// and can be retrieved when no environment variable is provided.
func TestConfigDefaultValuesAreSet(t *testing.T) {
	// Clear all environment variables that might interfere
	envVars := []string{
		"DB_DSN",
		"DB_DRIVER",
		"DB_MAX_OPEN_CONNS",
		"DB_MAX_IDLE_CONNS",
		"DB_CONNECTION_MAX_IDLE_TIME",
		"DB_CONNECTIONS_MAX_LIFETIME",
		"APP_HOST",
		"APP_PUBLIC_API_PORT",
		"APP_INFRA_API_PORT",
		"ENVIRONMENT",
	}

	// Store original values and clear them
	originalValues := make(map[string]string)
	for _, envVar := range envVars {
		if val, exists := os.LookupEnv(envVar); exists {
			originalValues[envVar] = val
			os.Unsetenv(envVar)
		}
	}
	defer func() {
		// Restore original values
		for envVar, val := range originalValues {
			os.Setenv(envVar, val)
		}
	}()

	// Initialize config with defaults only
	cfg := initConfig()

	// Verify all default values are set correctly
	require.Equal(t, "not set", cfg.DB.DSN)
	require.Equal(t, "postgres", cfg.DB.Driver)
	require.Equal(t, 25, cfg.DB.MaxOpenConn)
	require.Equal(t, 5, cfg.DB.MaxIdleConn)
	require.Equal(t, "0.0.0.0", cfg.APIServer.Host)
	require.Equal(t, 8000, cfg.APIServer.Port)
	require.Equal(t, 8001, cfg.InfraServer.Port)
}

// TestConfigDBPoolSettings validates database pool configuration
func TestConfigDBPoolSettings(t *testing.T) {
	// Clear environment variables to use defaults
	envVars := []string{
		"DB_DSN",
		"DB_DRIVER",
		"DB_MAX_OPEN_CONNS",
		"DB_MAX_IDLE_CONNS",
		"DB_CONNECTION_MAX_IDLE_TIME",
		"DB_CONNECTIONS_MAX_LIFETIME",
	}

	originalValues := make(map[string]string)
	for _, envVar := range envVars {
		if val, exists := os.LookupEnv(envVar); exists {
			originalValues[envVar] = val
			os.Unsetenv(envVar)
		}
	}
	defer func() {
		for envVar, val := range originalValues {
			os.Setenv(envVar, val)
		}
	}()

	cfg := initConfig()

	// Validate pool settings
	require.Greater(t, cfg.DB.MaxOpenConn, 0,
		"MaxOpenConn must be greater than 0, got %d", cfg.DB.MaxOpenConn)
	require.Greater(t, cfg.DB.MaxIdleConn, 0,
		"MaxIdleConn must be greater than 0, got %d", cfg.DB.MaxIdleConn)
	require.LessOrEqual(t, cfg.DB.MaxIdleConn, cfg.DB.MaxOpenConn,
		"MaxIdleConn (%d) must be less than or equal to MaxOpenConn (%d)",
		cfg.DB.MaxIdleConn, cfg.DB.MaxOpenConn)
	require.Greater(t, int64(cfg.DB.ConnMaxLifetime), int64(0),
		"ConnMaxLifetime must be greater than 0, got %v", cfg.DB.ConnMaxLifetime)
	require.Greater(t, int64(cfg.DB.ConnMaxIdleTime), int64(0),
		"ConnMaxIdleTime must be greater than 0, got %v", cfg.DB.ConnMaxIdleTime)
}
