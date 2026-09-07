package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
)

// TestDanceStateTTL pins the fallback, which is the branch that matters.
//
// Config blocks carry no viper defaults, so an absent or half-filled auth block
// arrives as a bare Go zero — and a zero TTL would sign every state as already
// expired, refusing every callback on a stand nobody thought they had
// misconfigured. Falling back rather than installing that is the whole point of
// having a resolver instead of reading the field directly.
func TestDanceStateTTL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		configured time.Duration
		want       time.Duration
	}{
		"configured value wins":   {configured: 3 * time.Minute, want: 3 * time.Minute},
		"unset falls back":        {configured: 0, want: defaultDanceStateTTL},
		"negative falls back too": {configured: -time.Minute, want: defaultDanceStateTTL},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, danceStateTTL(config.Auth{OAuthDanceStateTTL: tt.configured}))
		})
	}
}
