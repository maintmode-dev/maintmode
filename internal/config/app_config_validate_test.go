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
