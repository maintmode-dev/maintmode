package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The validator is exercised directly rather than through LoadAppConfig: the
// initConfig chain reports a validation failure with log.Panicf, which would
// take the test binary down instead of failing an assertion.
func TestValidateOTPRetention(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		retention time.Duration
		wantErr   bool
	}{
		{
			name:      "negative is rejected",
			retention: -time.Hour,
			wantErr:   true,
		},
		{
			// Zero means "unset": the service applies its own default, so it must
			// not be treated as a misconfiguration.
			name:      "zero is accepted as unset",
			retention: 0,
			wantErr:   false,
		},
		{
			name:      "shipped default is accepted",
			retention: 24 * time.Hour,
			wantErr:   false,
		},
		{
			// A retention far below the code lifetime is a poor forensic window
			// but not a safety problem: a row is only eligible once expires_at is
			// already in the past, so no live code can be reached. Accepting it
			// keeps the validator a mirror of its invitation sibling.
			name:      "small positive is accepted",
			retention: time.Minute,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &AppConfig{}
			cfg.TaskProcessor.OTPPrune.Retention = tt.retention

			err := cfg.validateOTPRetention()
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "task_processor.otp_prune.retention")
				return
			}
			require.NoError(t, err)
		})
	}
}
