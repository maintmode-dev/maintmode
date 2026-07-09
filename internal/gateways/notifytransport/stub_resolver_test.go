package notifytransport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
)

// The stub resolver answers every name with the stub transport — the dev
// use_stub mode, selected once at wiring time in bootstrap. The live-path
// counterpart (the integration service implementing TransportResolver) is
// covered by that service's resolve tests plus its compile-time interface
// assertion.
func TestStubResolver_ReturnsStubForAnyName(t *testing.T) {
	t.Parallel()
	r := notifytransport.NewStubResolver()

	for _, name := range []entity.NotifyTransport{
		entity.NotifyTransportSlack,
		entity.NotifyTransportTelegram,
		entity.NotifyTransport("unknown-kind"),
	} {
		tr, err := r.Get(context.Background(), name)
		require.NoError(t, err)
		require.Equal(t, entity.NotifyTransportStub, tr.TransportID(), "stub resolver must short-circuit %q", name)
	}
}
