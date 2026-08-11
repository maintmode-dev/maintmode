package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// use_stub routes every notify delivery to the in-memory stub and is gated on
// IsDev() at wiring time. validate() turns a use_stub set outside a dev
// environment — a silent no-op that could mask real prod deliveries — into a
// loud startup failure.
func TestValidate_UseStubEnvironmentGate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		env     Environment
		useStub bool
		wantErr bool
	}{
		{name: "stub in local is allowed", env: LocalEnvironment, useStub: true},
		{name: "stub in dev is allowed", env: DevEnvironment, useStub: true},
		{name: "stub in performance_test is allowed", env: PerformanceTestEnvironment, useStub: true},
		{name: "stub in prod is rejected", env: ProdEnvironment, useStub: true, wantErr: true},
		{name: "no stub in prod is allowed", env: ProdEnvironment, useStub: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := &AppConfig{Environment: tc.env}
			cfg.NotifyTransport.UseStub = tc.useStub

			err := cfg.validateUseStubInDev()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// A negative retention would push the prune cutoff into the future and make
// every terminal invitation eligible for deletion. The service clamps it to a
// safe default, but a negative value in config is always an operator typo — so
// validate() rejects it loudly at startup rather than silently substituting.
func TestValidate_InvitationPruneRetention(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		retention time.Duration
		wantErr   bool
	}{
		{name: "positive retention is allowed", retention: 8760 * time.Hour},
		{name: "zero retention is allowed (service defaults it)", retention: 0},
		{name: "negative retention is rejected", retention: -time.Hour, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := &AppConfig{}
			cfg.TaskProcessor.InvitationPrune.Retention = tc.retention

			err := cfg.validateInvitationRetention()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// An empty valkey.addr must be a startup error, not a default. The key was
// renamed redis -> valkey and viper ignores the leftover `redis:` block, so a
// stale config leaves Valkey zero-valued; go-redis then substitutes
// localhost:6379 for the empty address, and the process boots "healthy" while
// pointed at the wrong store. The rate limiter silently stops being
// replica-shared and token blacklisting stops being seen across replicas — all
// without a single error in the logs.
func TestValidate_ValkeyConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{name: "a configured address is allowed", addr: "valkey:6379"},
		{name: "an empty address is rejected", addr: "", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := &AppConfig{Valkey: Valkey{Address: tc.addr}}

			err := cfg.validateValkeyConfig()
			if tc.wantErr {
				require.Error(t, err)
				// The message must name the rename: a stale `redis:` block is the
				// likely cause and is otherwise invisible to whoever reads the panic.
				require.Contains(t, err.Error(), "valkey.addr")
				require.Contains(t, err.Error(), "redis")
				return
			}
			require.NoError(t, err)
		})
	}
}

// The Prove-It test for the stale-key bug itself, at the layer where it
// originates. TestValidate_ValkeyConfig above pins the guard; this pins the
// mechanism the guard exists for.
//
// readConfig calls viper's Unmarshal with no ErrorUnused hook, so a config file
// still carrying the pre-rename `redis:` block is not a decode error — the key
// is simply ignored and Valkey is left at its zero value. Asserting the empty
// address here means the comment on validateValkeyConfig is a verified claim
// about viper's behavior rather than a plausible one, and it fails if a future
// viper default for valkey.addr ever papers over the missing key.
func TestReadConfig_StaleRedisKeyLeavesValkeyUnset(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	t.Run("the renamed key decodes", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(dir, "valkey.yaml")
		require.NoError(t, os.WriteFile(path, []byte("valkey:\n  addr: valkey:6379\n  db: 0\n"), 0o600))

		cfg, err := readConfig(path)
		require.NoError(t, err)
		require.Equal(t, "valkey:6379", cfg.Valkey.Address)
		require.NoError(t, cfg.validateValkeyConfig())
	})

	t.Run("a stale redis block is silently ignored and then rejected", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(dir, "redis.yaml")
		require.NoError(t, os.WriteFile(path, []byte("redis:\n  addr: redis:6379\n  db: 0\n"), 0o600))

		cfg, err := readConfig(path)
		// The decode itself must succeed: that silence is the bug, and it is
		// what makes the startup guard the only thing standing between a stale
		// config and a process talking to the wrong store.
		require.NoError(t, err)
		require.Empty(t, cfg.Valkey.Address, "unknown `redis:` key must not populate Valkey")
		require.Error(t, cfg.validateValkeyConfig())
	})
}

// The shipped deployment configs are never parsed by any other test, so a
// reverted key in one of them — prod being both the likeliest to be edited by
// hand and the least likely to be exercised — would otherwise surface only as a
// panic at deploy time. Parsing them here moves that failure to `make tloc`.
func TestReadConfig_ShippedDeploymentConfigsSetValkeyAddr(t *testing.T) {
	t.Parallel()

	for _, env := range []string{"local", "dev", "test", "prod"} {
		t.Run(env, func(t *testing.T) {
			t.Parallel()

			cfg, err := readConfig(filepath.Join("..", "..", "deployment", "maintmode", env, "app.config.yaml"))
			require.NoError(t, err)
			require.NotEmpty(t, cfg.Valkey.Address, "deployment/maintmode/%s/app.config.yaml must set valkey.addr", env)
			require.NoError(t, cfg.validateValkeyConfig())
		})
	}
}

// The two issuer fields fail in OPPOSITE directions when left unset, so both
// have to be startup errors rather than defaults: an empty JWTIssuer makes
// jwt.WithIssuer skip the check and accept ANY issuer, while an empty
// JWTIssuers makes validation.In match nothing and reject EVERY Google token.
// One is a silent auth bypass, the other a total login outage.
//
// This is not hypothetical — the Google verifier used to call jwt.WithIssuer
// with the singular field, which no environment sets, so that check was dead
// everywhere until it was removed.
func TestValidate_IssuerConfig(t *testing.T) {
	t.Parallel()

	// newValid returns a config that passes, so each case can break exactly one
	// field and prove that field is what the validator rejects.
	newValid := func() *AppConfig {
		cfg := &AppConfig{}
		cfg.JWTVerifier.JWTIssuer = "oauth-service"
		cfg.OauthProviders.Google.JWTVerify.JWTIssuers = []string{
			"accounts.google.com",
			"https://accounts.google.com",
		}
		return cfg
	}

	cases := []struct {
		name    string
		mutate  func(*AppConfig)
		wantErr string
	}{
		{
			name:   "deployed shape is accepted",
			mutate: func(*AppConfig) {},
		},
		{
			name:    "empty jwt_issuer is rejected — it would accept any issuer",
			mutate:  func(c *AppConfig) { c.JWTVerifier.JWTIssuer = "" },
			wantErr: "jwtverifier.jwt_issuer must be set",
		},
		{
			name:    "empty jwt_issuers is rejected — it would reject every token",
			mutate:  func(c *AppConfig) { c.OauthProviders.Google.JWTVerify.JWTIssuers = nil },
			wantErr: "jwt_issuers must list at least one issuer",
		},
		{
			name: "the two fields are independent, not interchangeable",
			// Setting Google's singular field does NOT satisfy the plural one:
			// that confusion is exactly what shipped a dead check before.
			mutate: func(c *AppConfig) {
				c.OauthProviders.Google.JWTVerify.JWTIssuers = nil
				c.OauthProviders.Google.JWTVerify.JWTIssuer = "accounts.google.com"
			},
			wantErr: "jwt_issuers must list at least one issuer",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := newValid()
			tc.mutate(cfg)

			err := cfg.validateIssuerConfig()
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
