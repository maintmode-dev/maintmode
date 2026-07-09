package integration_test

import (
	"encoding/json"
	"testing"

	integrationsvc "github.com/ruko1202/maintmode/internal/services/integration"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

func TestNewRegistry_RegistersRealKinds(t *testing.T) {
	t.Parallel()
	// The production kinds register without a duplicate/empty-kind error — the
	// same set bootstrap registers at startup.
	reg, err := integrationsvc.NewRegistry(integrationkinds.Slack, integrationkinds.Telegram, integrationkinds.Email)
	require.NoError(t, err)
	for _, kind := range []string{"slack", "telegram", "email"} {
		got, err := reg.Get(kind)
		require.NoError(t, err)
		require.Equal(t, kind, got.Kind())
	}
}

// fakeIntegration is a minimal Integration for registry tests. The
// parse/validate methods are unused here; only Kind matters.
type fakeIntegration struct {
	kind string
}

func (f fakeIntegration) Kind() string         { return f.kind }
func (f fakeIntegration) SecretKeys() []string { return nil }

func (f fakeIntegration) Parse(json.RawMessage, map[string]string) (integrationkinds.Settings, error) {
	return nil, nil
}
func (f fakeIntegration) Validate(integrationkinds.Settings) error { return nil }

func TestNewRegistry_LookupByKind(t *testing.T) {
	t.Parallel()
	reg, err := integrationsvc.NewRegistry(fakeIntegration{kind: "slack"}, fakeIntegration{kind: "telegram"})
	require.NoError(t, err)

	got, err := reg.Get("slack")
	require.NoError(t, err)
	require.Equal(t, "slack", got.Kind())

	got, err = reg.Get("telegram")
	require.NoError(t, err)
	require.Equal(t, "telegram", got.Kind())
}

func TestNewRegistry_UnknownKind(t *testing.T) {
	t.Parallel()
	reg, err := integrationsvc.NewRegistry(fakeIntegration{kind: "slack"})
	require.NoError(t, err)

	_, err = reg.Get("jira")
	require.ErrorIs(t, err, apperr.ErrUnknownIntegrationKind)
}

func TestNewRegistry_UnknownKindIsValidationError(t *testing.T) {
	t.Parallel()
	// ErrUnknownIntegrationKind wraps ErrValidation so the API maps it to 400.
	// Assert the wrap chain directly — the ErrorIs on the leaf sentinel above
	// would still pass if someone changed the sentinel to a bare errors.New.
	reg, err := integrationsvc.NewRegistry(fakeIntegration{kind: "slack"})
	require.NoError(t, err)

	_, err = reg.Get("jira")
	require.ErrorIs(t, err, apperr.ErrValidation)
}

func TestNewRegistry_RegistersAllKinds(t *testing.T) {
	t.Parallel()
	// Every registered kind must be retrievable — a regression that drops or
	// shadows a middle kind would slip past a 2-kind lookup test.
	wantKinds := []string{"slack", "telegram", "smtp", "webhook"}
	reg, err := integrationsvc.NewRegistry(
		fakeIntegration{kind: "slack"},
		fakeIntegration{kind: "telegram"},
		fakeIntegration{kind: "smtp"},
		fakeIntegration{kind: "webhook"},
	)
	require.NoError(t, err)

	for _, kind := range wantKinds {
		got, err := reg.Get(kind)
		require.NoError(t, err)
		require.Equal(t, kind, got.Kind())
	}
}

func TestNewRegistry_DuplicateKindFailsFast(t *testing.T) {
	t.Parallel()
	_, err := integrationsvc.NewRegistry(fakeIntegration{kind: "slack"}, fakeIntegration{kind: "slack"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate kind")
}

func TestNewRegistry_EmptyKindFailsFast(t *testing.T) {
	t.Parallel()
	_, err := integrationsvc.NewRegistry(fakeIntegration{kind: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty kind")
}

func TestNewRegistry_Empty(t *testing.T) {
	t.Parallel()
	// An empty registry is valid (no integrations configured yet); every lookup
	// simply reports the kind as unknown.
	reg, err := integrationsvc.NewRegistry()
	require.NoError(t, err)

	_, err = reg.Get("slack")
	require.ErrorIs(t, err, apperr.ErrUnknownIntegrationKind)
}
