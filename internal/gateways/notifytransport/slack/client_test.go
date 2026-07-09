package slack

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// New must stay resilient when the bot token is absent (e.g. the integration is
// enabled in the DB but not yet fully provisioned): it returns a usable, non-nil
// client with the right transport id and defers the "no token" failure to send
// time rather than panicking or failing at construction.
func TestNew_EmptyToken_StaysResilient(t *testing.T) {
	t.Parallel()

	c := New(Params{})
	require.NotNil(t, c, "empty token must still yield a non-nil client")
	require.Equal(t, entity.NotifyTransportSlack, c.TransportID())
}

// Params carries a secret (BotToken). Lock in that it defines no Stringer or JSON
// marshaler so it can never be accidentally logged or serialized in full — the
// same at-rest guarantee the type comment asserts, enforced against future edits.
func TestParams_HasNoSecretLeakingSerializer(t *testing.T) {
	t.Parallel()

	var p any = Params{BotToken: "xoxb-secret"}
	_, isStringer := p.(fmt.Stringer)
	require.False(t, isStringer, "Params must not implement fmt.Stringer (would risk logging the token)")
	_, isMarshaler := p.(json.Marshaler)
	require.False(t, isMarshaler, "Params must not implement json.Marshaler (would risk serializing the token)")
}
