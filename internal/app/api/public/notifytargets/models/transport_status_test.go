package apimodels

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestIntegrationIndex_StatusFor pins the kind→enabled → TransportStatus
// mapping: enabled → ok, present-but-disabled → disabled, absent →
// not_configured (RUK-198).
func TestIntegrationIndex_StatusFor(t *testing.T) {
	t.Parallel()

	index := NewIntegrationIndex([]*entity.MaskedIntegration{
		{Kind: string(entity.NotifyTransportSlack), Enabled: true},
		{Kind: string(entity.NotifyTransportTelegram), Enabled: false},
	})

	tests := []struct {
		name      string
		transport entity.NotifyTransport
		want      TransportStatus
	}{
		{name: "enabled integration", transport: entity.NotifyTransportSlack, want: TransportStatusOK},
		{name: "disabled integration", transport: entity.NotifyTransportTelegram, want: TransportStatusDisabled},
		{name: "missing integration", transport: entity.NotifyTransportEmail, want: TransportStatusNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, index.StatusFor(tt.transport))
		})
	}
}

// TestIntegrationIndex_Empty covers the empty-registry case: every transport
// reads as not_configured.
func TestIntegrationIndex_Empty(t *testing.T) {
	t.Parallel()

	index := NewIntegrationIndex(nil)
	require.Equal(t, TransportStatusNotConfigured, index.StatusFor(entity.NotifyTransportSlack))
}
