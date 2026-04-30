package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplySecret(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		cfg := &AppConfig{
			App: App{
				FrontendURL: "http://localhost:9000",
			},
			Environment: DevEnvironment,
			DB: DB{
				DSN: "<secret:db/dsn>",
			},
			Redis: Redis{
				Address:  "redis:address",
				Password: "<secret:redis/password>",
				DB:       0,
			},
			OauthProviders: OauthProviders{
				Google: GoogleOauthProvider{
					ClientID:     "<secret:oauth/google/client_id>",
					ClientSecret: "<secret:oauth/google/client_secret>",
					Scopes: []string{
						"openid",
						"email",
						"<secret:oauth/google/profile_scope>",
					},
				},
			},
			JWT: JWT{
				PrivateKey: "<secret:jwt/issuer_private_key>",
				Kid:        "<secret:jwt/issuer_kid>",
			},
			S2SConfig: S2SConfig{
				"maintmode": {
					Secret: "<secret:s2s/maintmode/secret>",
				},
			},
			ExternalServices: map[string]ExternalService{
				"auth": {
					Secret: "<secret:external_services/auth/secret>",
					Host:   "auth",
				},
			},
		}

		secrets := secretStore{
			"db/dsn":                        "postgres://maintmode:strong-password@db.internal:5432/maintmode?sslmode=require",
			"redis/password":                "strong-redis-password",
			"oauth/google/client_id":        "google-client-id",
			"oauth/google/client_secret":    "strong-google-client-secret",
			"oauth/google/profile_scope":    "profile_scope",
			"jwt/issuer_private_key":        "1be2f1f68285c972b750b7718b00d5453f2c08f88c7894d1b9013f75a439de20",
			"jwt/issuer_kid":                "jwt-kid-1",
			"s2s/maintmode/secret":          "s2s-maintmode-secret",
			"external_services/auth/secret": "external_services-auth-secret",
		}

		err := cfg.applySecrets(secrets)
		require.NoError(t, err)

		require.Equal(t, &AppConfig{
			App: App{
				FrontendURL: "http://localhost:9000",
			},
			Environment: DevEnvironment,
			DB: DB{
				DSN: "postgres://maintmode:strong-password@db.internal:5432/maintmode?sslmode=require",
			},
			Redis: Redis{
				Address:  "redis:address",
				Password: "strong-redis-password",
				DB:       0,
			},
			OauthProviders: OauthProviders{
				Google: GoogleOauthProvider{
					ClientID:     "google-client-id",
					ClientSecret: "strong-google-client-secret",
					Scopes: []string{
						"openid",
						"email",
						"profile_scope",
					},
				},
			},
			JWT: JWT{
				PrivateKey: "1be2f1f68285c972b750b7718b00d5453f2c08f88c7894d1b9013f75a439de20",
				Kid:        "jwt-kid-1",
			},
			S2SConfig: S2SConfig{
				"maintmode": {
					Secret: "s2s-maintmode-secret",
				},
			},
			ExternalServices: map[string]ExternalService{
				"auth": {
					Secret: "external_services-auth-secret",
					Host:   "auth",
				},
			},
		}, cfg)
	})

	t.Run("missing secret", func(t *testing.T) {
		cfg := &AppConfig{
			DB: DB{
				DSN: "<secret:db/dsn>",
			},
		}

		err := cfg.applySecrets(secretStore{})
		require.Error(t, err)
		require.EqualError(t, err, "secret not found: db/dsn")
	})

	t.Run("ignores non matching secret ref", func(t *testing.T) {
		cfg := &AppConfig{
			DB: DB{
				DSN: "postgres://<secret:db/dsn>",
			},
		}

		err := cfg.applySecrets(secretStore{})
		require.NoError(t, err)
		require.Equal(t, "postgres://<secret:db/dsn>", cfg.DB.DSN)
	})
}
