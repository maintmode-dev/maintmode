package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The JWT block carries no defaults and the config file is mounted separately
// from the image, so a binary deployed against a config predating these keys
// reads 0s for both -- and every refresh would then find the session already
// past its inactive limit. That is a silent, total sign-in outage, which is
// what this validator turns into a loud startup failure. Both directions are
// pinned: a validator that rejects an honest config is worse than none.
func TestValidate_SessionLifetimes(t *testing.T) {
	t.Parallel()

	cfg := func(access, inactive, maxLifetime time.Duration) *AppConfig {
		return &AppConfig{JWT: JWT{
			AccessTokenTTL:          access,
			SessionInactiveLifetime: inactive,
			SessionMaxLifetime:      maxLifetime,
		}}
	}

	const (
		access   = 15 * time.Minute
		inactive = 168 * time.Hour
		maxLife  = 720 * time.Hour
	)

	cases := []struct {
		name    string
		cfg     *AppConfig
		wantErr string
	}{
		{
			name: "the production shape is accepted",
			cfg:  cfg(access, inactive, maxLife),
		},
		{
			// One hard deadline that activity does not extend is a legitimate
			// policy, so equality must not be rejected.
			name: "equal limits are allowed",
			cfg:  cfg(access, maxLife, maxLife),
		},
		{
			// The case this exists for: the keys are absent.
			name:    "a missing inactive limit is rejected",
			cfg:     cfg(access, 0, maxLife),
			wantErr: "session_inactive_lifetime must be positive",
		},
		{
			name:    "a missing maximum is rejected",
			cfg:     cfg(access, inactive, 0),
			wantErr: "session_max_lifetime must be positive",
		},
		{
			// A session that dies before its first refresh is due looks like
			// random sign-outs, not like a misconfiguration.
			name:    "an inactive limit below the access TTL is rejected",
			cfg:     cfg(access, 10*time.Minute, maxLife),
			wantErr: "must be greater than access_token_ttl",
		},
		{
			// A maximum under the idle limit makes the idle limit unreachable.
			name:    "a maximum below the inactive limit is rejected",
			cfg:     cfg(access, inactive, time.Hour),
			wantErr: "must be at least session_inactive_lifetime",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.cfg.validateSessionLifetimes()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
