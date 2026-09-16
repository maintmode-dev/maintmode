package entity_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestNotifyTransport_IsValid pins what IsValid answers for, which is narrower
// than "is this a transport the product can deliver on".
//
// email is the interesting case and the reason this test exists. It is a real
// delivery transport -- invitations and one-time codes go out over it -- but it
// is not a channel anyone may subscribe to: transport_channel_id is free text,
// so a channel created on it delivers to whatever address the creator typed.
// IsValid guards exactly one thing, channel creation, so email must not pass it.
func TestNotifyTransport_IsValid(t *testing.T) {
	t.Parallel()

	tests := map[entity.NotifyTransport]bool{
		entity.NotifyTransportSlack:    true,
		entity.NotifyTransportTelegram: true,
		// Delivers system mail, but is never a user-subscribable channel.
		entity.NotifyTransportEmail: false,
		// The stub is wired by configuration, not chosen by an operator.
		entity.NotifyTransportStub: false,
		"":                         false,
		"smtp":                     false,
		"Slack":                    false,
	}

	for transport, want := range tests {
		require.Equalf(t, want, transport.IsValid(), "IsValid(%q)", transport)
	}
}
