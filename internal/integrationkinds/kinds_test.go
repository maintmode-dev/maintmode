package integrationkinds

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// cfg is a test helper: a raw JSON config body from a literal.
func cfg(s string) json.RawMessage { return json.RawMessage(s) }

func TestSlack_ParseValidateSecretKeys(t *testing.T) {
	t.Parallel()
	s := slack{}
	require.Equal(t, "slack", s.Kind())
	require.Equal(t, []string{"bot_token"}, s.SecretKeys())

	t.Run("parse valid", func(t *testing.T) {
		t.Parallel()
		got, err := s.Parse(
			cfg(`{"api_url":"https://slack.test","timeout":"10s"}`),
			map[string]string{"bot_token": "xoxb-1"},
		)
		require.NoError(t, err)
		settings, ok := got.(SlackSettings)
		require.True(t, ok, "Parse must return SlackSettings")
		require.Equal(t, "xoxb-1", settings.BotToken)
		require.Equal(t, "https://slack.test", settings.APIURL)
		require.Equal(t, "10s", settings.Timeout)
		require.NoError(t, s.Validate(got))
	})

	t.Run("missing secret rejected at validate", func(t *testing.T) {
		t.Parallel()
		// The secret is no longer required by Parse (it just merges the map);
		// Validate is where a missing/blank bot_token is rejected.
		got, err := s.Parse(cfg(`{}`), map[string]string{})
		require.NoError(t, err)
		require.ErrorContains(t, s.Validate(got), "bot_token")
	})

	t.Run("blank secret rejected at validate", func(t *testing.T) {
		t.Parallel()
		got, err := s.Parse(cfg(`{}`), map[string]string{"bot_token": ""})
		require.NoError(t, err)
		require.ErrorContains(t, s.Validate(got), "bot_token")
	})

	t.Run("wrong-typed field", func(t *testing.T) {
		t.Parallel()
		// api_url is a string field: a JSON number is a malformed body.
		_, err := s.Parse(cfg(`{"api_url":42}`), map[string]string{"bot_token": "x"})
		require.Error(t, err)
	})

	t.Run("bad duration rejected at validate", func(t *testing.T) {
		t.Parallel()
		got, err := s.Parse(cfg(`{"timeout":"nope"}`), map[string]string{"bot_token": "x"})
		require.NoError(t, err)
		require.Error(t, s.Validate(got))
	})

	t.Run("malformed api_url rejected at validate", func(t *testing.T) {
		t.Parallel()
		got, err := s.Parse(cfg(`{"api_url":"not a url"}`), map[string]string{"bot_token": "x"})
		require.NoError(t, err)
		require.ErrorContains(t, s.Validate(got), "api_url")
	})

	t.Run("internal http api_url accepted", func(t *testing.T) {
		t.Parallel()
		// Self-hosted proxies/gateways on plain http inside the perimeter are a
		// legitimate target; the check is syntactic, not a destination policy.
		got, err := s.Parse(cfg(`{"api_url":"http://slack-proxy:8081"}`), map[string]string{"bot_token": "x"})
		require.NoError(t, err)
		require.NoError(t, s.Validate(got))
	})
}

func TestTelegram_ParseValidateSecretKeys(t *testing.T) {
	t.Parallel()
	tg := Telegram
	require.Equal(t, "telegram", tg.Kind())
	require.Equal(t, []string{"bot_token"}, tg.SecretKeys())

	// Assert the concrete field mapping — the telegram-specific code Slack's tests
	// can't cover (a transposed or dropped assignment would slip past err==nil).
	got, err := tg.Parse(
		cfg(`{"api_url":"https://tg.test","timeout":"10s"}`),
		map[string]string{"bot_token": "123:abc"},
	)
	require.NoError(t, err)
	settings := got.(TelegramSettings)
	require.Equal(t, "123:abc", settings.BotToken)
	require.Equal(t, "https://tg.test", settings.APIURL)
	require.Equal(t, "10s", settings.Timeout)
	require.NoError(t, tg.Validate(got))

	got, err = tg.Parse(cfg(`{}`), map[string]string{})
	require.NoError(t, err)
	require.Error(t, tg.Validate(got), "missing bot_token must fail validation")
}

func TestEmail_ParseValidateSecretKeys(t *testing.T) {
	t.Parallel()
	e := Email
	require.Equal(t, "email", e.Kind())
	require.Equal(t, []string{"password"}, e.SecretKeys())

	t.Run("valid authenticated", func(t *testing.T) {
		t.Parallel()
		got, err := e.Parse(
			cfg(`{"host":"smtp.test","port":587,"from":"a@b.c","username":"u","tls_policy":"mandatory"}`),
			map[string]string{"password": "pw"},
		)
		require.NoError(t, err)
		settings := got.(EmailSettings)
		require.Equal(t, "smtp.test", settings.Host)
		require.Equal(t, 587, settings.Port)
		require.Equal(t, "pw", settings.Password)
		require.NoError(t, e.Validate(got))
	})

	t.Run("timeout string carried through", func(t *testing.T) {
		t.Parallel()
		got, err := e.Parse(
			cfg(`{"host":"smtp.test","from":"a@b.c","timeout":"20s"}`),
			map[string]string{},
		)
		require.NoError(t, err)
		require.Equal(t, "20s", got.(EmailSettings).Timeout)
		require.NoError(t, e.Validate(got))
	})

	t.Run("valid unauthenticated relay", func(t *testing.T) {
		t.Parallel()
		got, err := e.Parse(cfg(`{"host":"smtp.test","from":"a@b.c"}`), map[string]string{})
		require.NoError(t, err)
		require.NoError(t, e.Validate(got), "no username + no password is a valid open relay")
	})

	t.Run("missing host", func(t *testing.T) {
		t.Parallel()
		got, err := e.Parse(cfg(`{"from":"a@b.c"}`), map[string]string{})
		require.NoError(t, err)
		require.Error(t, e.Validate(got))
	})

	t.Run("missing from", func(t *testing.T) {
		t.Parallel()
		got, err := e.Parse(cfg(`{"host":"smtp.test"}`), map[string]string{})
		require.NoError(t, err)
		require.Error(t, e.Validate(got))
	})

	t.Run("half credential rejected", func(t *testing.T) {
		t.Parallel()
		// username without password.
		got, err := e.Parse(cfg(`{"host":"smtp.test","from":"a@b.c","username":"u"}`), map[string]string{})
		require.NoError(t, err)
		require.Error(t, e.Validate(got), "username without password must be rejected")
	})

	t.Run("non-integer port", func(t *testing.T) {
		t.Parallel()
		// port is an int field: a fractional JSON number is a malformed body.
		_, err := e.Parse(cfg(`{"port":587.5}`), map[string]string{})
		require.Error(t, err)
	})
}

func TestSecretKeys_ReturnFreshSliceNotSharedState(t *testing.T) {
	t.Parallel()
	// A caller mutating the returned slice must not corrupt the next call's result.
	first := Slack.SecretKeys()
	first[0] = "mutated"
	second := Slack.SecretKeys()
	require.Equal(t, []string{"bot_token"}, second, "SecretKeys must return a fresh slice each call")
}

func TestValidate_RejectsForeignSettingsType(t *testing.T) {
	t.Parallel()
	// Each kind's Validate must reject a Settings that isn't its own concrete type
	// (the within-kind contract of Settings=any), not panic.
	require.Error(t, Slack.Validate(TelegramSettings{BotToken: "x"}))
	require.Error(t, Email.Validate(SlackSettings{BotToken: "x"}))
}
