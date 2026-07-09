package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplySecret(t *testing.T) {
	t.Parallel()
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
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
			// Crypto.LocalKeys is the first config field whose secret refs live inside
			// a map, exercising the resolver's reflect.Map branch (which copies the
			// value out, resolves it, and writes it back via SetMapIndex). Two
			// entries so the map loop runs more than once. ActiveKEKURI holds no ref
			// and must survive resolution unchanged.
			Crypto: CryptoConfig{
				ActiveKEKURI: "local-kms://kek-2",
				LocalKeys: map[string]string{
					"local-kms://kek-1": "<secret:crypto/kek/kek-1>",
					"local-kms://kek-2": "<secret:crypto/kek/kek-2>",
				},
			},
		}

		secrets := secretStore{
			"db/dsn":                     "postgres://maintmode:strong-password@db.internal:5432/maintmode?sslmode=require",
			"redis/password":             "strong-redis-password",
			"oauth/google/client_id":     "google-client-id",
			"oauth/google/client_secret": "strong-google-client-secret",
			"oauth/google/profile_scope": "profile_scope",
			"jwt/issuer_private_key":     "1be2f1f68285c972b750b7718b00d5453f2c08f88c7894d1b9013f75a439de20",
			"jwt/issuer_kid":             "jwt-kid-1",
			"crypto/kek/kek-1":           "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			"crypto/kek/kek-2":           "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
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
			Crypto: CryptoConfig{
				ActiveKEKURI: "local-kms://kek-2",
				LocalKeys: map[string]string{
					"local-kms://kek-1": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
					"local-kms://kek-2": "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
				},
			},
		}, cfg)
	})

	t.Run("missing secret", func(t *testing.T) {
		t.Parallel()
		cfg := &AppConfig{
			DB: DB{
				DSN: "<secret:db/dsn>",
			},
		}

		err := cfg.applySecrets(secretStore{})
		require.Error(t, err)
		require.EqualError(t, err, "secret not found: db/dsn")
	})

	t.Run("missing map secret fails", func(t *testing.T) {
		t.Parallel()
		// A secret ref living inside a map value must fail-fast on startup just
		// like a struct-field ref does. This guards Task-1 acceptance ("missing
		// crypto/kek → fail-fast loader") through the reflect.Map branch, which
		// the struct-field "missing secret" case above does not reach.
		cfg := &AppConfig{
			Crypto: CryptoConfig{
				ActiveKEKURI: "local-kms://kek-1",
				LocalKeys: map[string]string{
					"local-kms://kek-1": "<secret:crypto/kek/kek-1>",
				},
			},
		}

		err := cfg.applySecrets(secretStore{})
		require.Error(t, err)
		require.EqualError(t, err, "secret not found: crypto/kek/kek-1")
	})

	t.Run("ignores non matching secret ref", func(t *testing.T) {
		t.Parallel()
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
