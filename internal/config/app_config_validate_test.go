package config

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// IsProd() and IsDev() are exact string comparisons, so any value outside the
// known set leaves both false — a third state in which no IsProd() gate fires
// and every future one silently opens. The point of the validator is that such
// a value can never reach a gate, so the table asserts on the near-misses an
// operator actually types.
func TestValidate_Environment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		env     Environment
		wantErr bool
	}{
		{name: "prod is known", env: ProdEnvironment},
		{name: "dev is known", env: DevEnvironment},
		{name: "local is known", env: LocalEnvironment},
		{name: "performance_test is known", env: PerformanceTestEnvironment},
		{name: "empty is rejected", env: "", wantErr: true},
		{name: "capitalized Prod is rejected", env: "Prod", wantErr: true},
		{name: "upper-case PROD is rejected", env: "PROD", wantErr: true},
		{name: "the word production is rejected", env: "production", wantErr: true},
		// A trailing space survives YAML quoting and is invisible in review.
		{name: "trailing whitespace is rejected", env: "prod ", wantErr: true},
		{name: "an unrelated value is rejected", env: "staging", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := (&AppConfig{Environment: tc.env}).validateEnvironment()
			if tc.wantErr {
				require.ErrorContains(t, err, "unknown environment")
				// The message must list the accepted values: this fires at
				// startup before the logger exists, so the raw text is all the
				// operator gets.
				require.ErrorContains(t, err, "performance_test")
				return
			}
			require.NoError(t, err)
		})
	}
}

// The validators never run before a merge — the binary only starts in the
// api-test job, which is push-to-main only — so this is the single check that
// an invalid environment in a shipped config is caught by `make tloc`.
func TestReadConfig_ShippedDeploymentConfigsHaveKnownEnvironment(t *testing.T) {
	t.Parallel()

	for _, env := range []string{"local", "dev", "test", "prod"} {
		t.Run(env, func(t *testing.T) {
			t.Parallel()

			cfg, err := readConfig(filepath.Join("..", "..", "deployment", "maintmode", env, "app.config.yaml"))
			require.NoError(t, err)
			require.NoError(
				t,
				cfg.validateEnvironment(),
				"deployment/maintmode/%s/app.config.yaml has an environment the binary would refuse to start on", env,
			)
		})
	}
}

// A validator that is never called is worth nothing, and a table test over the
// pure function cannot tell the difference: deleting both calls from initConfig
// leaves every other test in this package green. These assert the wiring, which
// is the only thing that makes an operator's typo stop the process.
//
// initConfig panics via log.Panicf rather than returning an error, so the panic
// IS the observable behavior — recovering from it needs no signature change.
func TestInitConfig_RunsTheNewValidators(t *testing.T) {
	// Not parallel: initConfig reads process environment variables.

	// panicMessage runs initConfig against a config dir and returns the panic
	// text, failing if it did not panic at all.
	panicMessage := func(t *testing.T, dir string) (msg string) {
		t.Helper()
		t.Setenv("MAINTMODE_"+configDirEnv, dir)

		defer func() {
			r := recover()
			require.NotNil(t, r, "initConfig accepted an invalid config instead of panicking")
			msg = fmt.Sprint(r)
		}()

		initConfig("maintmode")
		return ""
	}

	// newConfigDir copies the local stand's config and secrets into a temp dir,
	// applying mutate to the config YAML first.
	newConfigDir := func(t *testing.T, mutate func(string) string) string {
		t.Helper()

		src := filepath.Join("..", "..", "deployment", "maintmode", "local")
		cfgBody, err := os.ReadFile(filepath.Join(src, defaultConfigFile))
		require.NoError(t, err)
		secretsBody, err := os.ReadFile(filepath.Join(src, defaultSecretsFile))
		require.NoError(t, err, "local secrets are required; run `make secrets`")

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, defaultConfigFile), []byte(mutate(string(cfgBody))), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, defaultSecretsFile), secretsBody, 0o600))
		return dir
	}

	t.Run("an unknown environment stops startup", func(t *testing.T) {
		dir := newConfigDir(t, func(body string) string {
			return strings.Replace(body, "environment: local", "environment: staging", 1)
		})

		require.Contains(t, panicMessage(t, dir), "unknown environment")
	})

	t.Run("a placeholder signing key stops startup", func(t *testing.T) {
		// 0xAA repeated: BitLen is 256, so only the repeated-byte check rejects
		// it. Using it here proves that specific check is reachable from startup.
		dir := newConfigDir(t, func(body string) string { return body })

		secretsPath := filepath.Join(dir, defaultSecretsFile)
		secrets, err := os.ReadFile(secretsPath)
		require.NoError(t, err)

		placeholder := hex.EncodeToString(bytes.Repeat([]byte{0xaa}, 32))
		patched := regexp.MustCompile(`(?m)^"jwt/issuer_private_key":.*$`).
			ReplaceAllString(string(secrets), `"jwt/issuer_private_key": "`+placeholder+`"`)
		require.NotEqual(t, string(secrets), patched, "jwt/issuer_private_key key not found in local secrets")
		require.NoError(t, os.WriteFile(secretsPath, []byte(patched), 0o600))

		require.Contains(t, panicMessage(t, dir), "placeholder")
	})
}
