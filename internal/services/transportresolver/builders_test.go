package transportresolver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	emailtransport "github.com/ruko1202/maintmode/internal/gateways/notifytransport/email"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/services/transportresolver"
)

func TestBuilders_ConstructWorkingClients(t *testing.T) {
	t.Parallel()
	b := transportresolver.Builders()

	slackTr, err := b["slack"](integrationkinds.SlackSettings{BotToken: "xoxb-1", APIURL: "https://slack.test"})
	require.NoError(t, err)
	require.Equal(t, entity.NotifyTransportSlack, slackTr.TransportID())

	tgTr, err := b["telegram"](integrationkinds.TelegramSettings{BotToken: "123:abc", APIURL: "https://tg.test"})
	require.NoError(t, err)
	require.Equal(t, entity.NotifyTransportTelegram, tgTr.TransportID())

	emailTr, err := b["email"](integrationkinds.EmailSettings{
		Host: "smtp.test", From: "a@b.c", TLSPolicy: integrationkinds.TLSPolicyNone, Timeout: "5s",
	})
	require.NoError(t, err)
	require.Equal(t, entity.NotifyTransportEmail, emailTr.TransportID())
}

func TestBuilders_RejectForeignSettings(t *testing.T) {
	t.Parallel()
	b := transportresolver.Builders()

	_, err := b["slack"](integrationkinds.TelegramSettings{BotToken: "x"})
	require.Error(t, err, "a foreign Settings type must be rejected, not panic")

	_, err = b["telegram"](integrationkinds.SlackSettings{BotToken: "x"})
	require.Error(t, err)

	_, err = b["email"](integrationkinds.SlackSettings{BotToken: "x"})
	require.Error(t, err)
}

func TestBuilders_EmailInvalidConfigErrors(t *testing.T) {
	t.Parallel()
	// Missing From makes the underlying email client construction fail (not panic).
	_, err := transportresolver.Builders()["email"](integrationkinds.EmailSettings{Host: "smtp.test"})
	require.Error(t, err)
}

// The kind owns the admin-facing TLS vocabulary; the transport owns its
// interpretation. This test pins the two string sets together — the resolver is
// the one module that legitimately imports both halves.
func TestBuilders_EmailTLSVocabularyMatchesTransport(t *testing.T) {
	t.Parallel()
	require.Equal(t, emailtransport.TLSPolicyNone, integrationkinds.TLSPolicyNone)
	require.Equal(t, emailtransport.TLSPolicyOpportunistic, integrationkinds.TLSPolicyOpportunistic)
	require.Equal(t, emailtransport.TLSPolicyTLSMandatory, integrationkinds.TLSPolicyMandatory)
}

// A delivery-capable kind takes its name FROM entity.NotifyTransport by
// construction (kindSlack = string(entity.NotifyTransportSlack), ... in
// internal/integrationkinds), so the two string domains cannot drift. What still
// needs pinning is the WIRING: every delivery-capable kind must have a builder
// registered under exactly its transport, and no builder may hide under a name
// outside that set — either mistake degrades delivery to a silent best-effort
// drop (see Service.Get).
func TestBuilders_AlignWithIntegrationKinds(t *testing.T) {
	t.Parallel()
	b := transportresolver.Builders()

	deliveryKinds := []integrationkinds.Integration{integrationkinds.Slack, integrationkinds.Telegram, integrationkinds.Email}
	require.Len(t, b, len(deliveryKinds), "every builder must correspond to a delivery-capable kind")
	for _, in := range deliveryKinds {
		require.Contains(t, b, entity.NotifyTransport(in.Kind()), "kind %q has no builder", in.Kind())
	}
}
