package config

import (
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
