package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate_S2SConfig(t *testing.T) {
	t.Parallel()

	const goodSecret = "0123456789abcdef0123456789abcdef"
	goodAPI := []string{"/api/v1/refresh"}

	cases := []struct {
		name    string
		env     Environment
		s2s     S2SConfig
		wantErr string
	}{
		{
			name:    "no client at all leaves every protected route refusing everyone",
			s2s:     nil,
			wantErr: "s2s: no client configured",
		},
		{
			name:    "a short secret",
			s2s:     S2SConfig{"maintmode-ui": {Secret: goodSecret[:31], AvailableAPI: goodAPI}},
			wantErr: "is too short: 31 characters",
		},
		{
			name: "the sample placeholder",
			s2s: S2SConfig{"maintmode-ui": {
				Secret: "replace-me-prod-value-s2s-maintmode-ui", AvailableAPI: goodAPI,
			}},
			wantErr: "is a placeholder",
		},
		{
			name:    "a trailing newline from a secret store",
			s2s:     S2SConfig{"maintmode-ui": {Secret: goodSecret + "\n", AvailableAPI: goodAPI}},
			wantErr: "leading or trailing whitespace",
		},
		{
			name: "the value published in this repository, in prod",
			env:  ProdEnvironment,
			s2s: S2SConfig{"maintmode-ui": {
				Secret: "local-bff-s2s-token-not-a-secret-0123456789", AvailableAPI: goodAPI,
			}},
			wantErr: "is the value published in this repository",
		},
		{
			name:    "a client allowed nowhere",
			s2s:     S2SConfig{"maintmode-ui": {Secret: goodSecret}},
			wantErr: "available_api is empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := &AppConfig{Environment: tc.env, S2S: tc.s2s}
			require.ErrorContains(t, cfg.validateS2SConfig(), tc.wantErr)
		})
	}

	// The dev, local and test stacks all run on the published value, and the
	// test stack is a "dev" environment, so only prod can refuse it.
	t.Run("the published value outside prod is accepted", func(t *testing.T) {
		t.Parallel()

		cfg := &AppConfig{Environment: DevEnvironment, S2S: S2SConfig{"maintmode-ui": {
			Secret: "local-bff-s2s-token-not-a-secret-0123456789", AvailableAPI: goodAPI,
		}}}
		require.NoError(t, cfg.validateS2SConfig())
	})

	t.Run("a well-formed client is accepted", func(t *testing.T) {
		t.Parallel()

		cfg := &AppConfig{S2S: S2SConfig{"maintmode-ui": {Secret: goodSecret, AvailableAPI: goodAPI}}}
		require.NoError(t, cfg.validateS2SConfig())
	})
}
