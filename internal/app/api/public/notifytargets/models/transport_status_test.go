package apimodels

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// TestStatusFromResolve pins the resolve-outcome → TransportStatus mapping:
// nil → ok, ErrIntegrationNotConfigured → not_configured (checked
// BEFORE disabled — it wraps it), ErrIntegrationUnreadable → unreadable,
// ErrIntegrationDisabled → disabled, anything else → unclassified (the caller
// must 500, never invent a status).
func TestStatusFromResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		want   TransportStatus
		wantOK bool
	}{
		{name: "resolved", err: nil, want: TransportStatusOK, wantOK: true},
		{name: "not configured (wraps disabled)",
			err:  fmt.Errorf("%w: %q", apperr.ErrIntegrationNotConfigured, "slack"),
			want: TransportStatusNotConfigured, wantOK: true},
		{name: "disabled", err: apperr.ErrIntegrationDisabled,
			want: TransportStatusDisabled, wantOK: true},
		{name: "no builder is disabled",
			err:  fmt.Errorf("%w: transport %q has no builder", apperr.ErrIntegrationDisabled, "smtp"),
			want: TransportStatusDisabled, wantOK: true},
		{name: "unreadable (rolled-back KEK)",
			err:  fmt.Errorf("%w: unwrap dek: unknown kek", apperr.ErrIntegrationUnreadable),
			want: TransportStatusUnreadable, wantOK: true},
		{name: "storage outage is unclassified", err: errors.New("pq: connection refused"),
			want: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := StatusFromResolve(tt.err)
			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestTransportStatusIndex_StatusFor pins the index lookup and its safe
// default: a transport that was never resolved reads as not_configured, never
// a false ok.
func TestTransportStatusIndex_StatusFor(t *testing.T) {
	t.Parallel()

	index := TransportStatusIndex{
		entity.NotifyTransportSlack:    TransportStatusOK,
		entity.NotifyTransportTelegram: TransportStatusUnreadable,
	}
	require.Equal(t, TransportStatusOK, index.StatusFor(entity.NotifyTransportSlack))
	require.Equal(t, TransportStatusUnreadable, index.StatusFor(entity.NotifyTransportTelegram))
	require.Equal(t, TransportStatusNotConfigured, index.StatusFor(entity.NotifyTransportEmail),
		"an unresolved transport must degrade to not_configured, not ok")
}
