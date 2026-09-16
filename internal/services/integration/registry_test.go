package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

func TestNewRegistry_RegistersRealSystems(t *testing.T) {
	t.Parallel()
	// The production delivery systems register without a duplicate/empty-name
	// error. The login entries are covered separately, in login_kinds_test.go:
	// they are two entries over one implementation, which is a different
	// property than "each system registers".
	reg, err := NewRegistry(integrationkinds.Slack, integrationkinds.Telegram, integrationkinds.Email)
	require.NoError(t, err)
	for _, name := range []string{"slack", "telegram", "email"} {
		got, err := reg.get(name)
		require.NoError(t, err)
		require.Equal(t, name, got.Name())
	}
}

// fakeIntegration is a minimal Integration for registry tests. The
// parse/validate methods are unused here; only the name matters.
type fakeIntegration struct {
	name string
}

func (f fakeIntegration) Name() string         { return f.name }
func (fakeIntegration) Category() string       { return integrationkinds.CategoryNotify }
func (f fakeIntegration) SecretKeys() []string { return nil }

func (f fakeIntegration) Parse(json.RawMessage, map[string]string) (integrationkinds.Settings, error) {
	return nil, nil
}
func (f fakeIntegration) Validate(integrationkinds.Settings) error { return nil }

func TestNewRegistry_LookupByName(t *testing.T) {
	t.Parallel()
	reg, err := NewRegistry(fakeIntegration{name: "slack"}, fakeIntegration{name: "telegram"})
	require.NoError(t, err)

	got, err := reg.get("slack")
	require.NoError(t, err)
	require.Equal(t, "slack", got.Name())

	got, err = reg.get("telegram")
	require.NoError(t, err)
	require.Equal(t, "telegram", got.Name())
}

func TestNewRegistry_UnknownKind(t *testing.T) {
	t.Parallel()
	reg, err := NewRegistry(fakeIntegration{name: "slack"})
	require.NoError(t, err)

	_, err = reg.get("nonexistent")
	require.ErrorIs(t, err, apperr.ErrUnknownIntegrationKind)
}

func TestNewRegistry_UnknownKindIsValidationError(t *testing.T) {
	t.Parallel()
	// ErrUnknownIntegrationKind wraps ErrValidation so the API maps it to 400.
	// Assert the wrap chain directly — the ErrorIs on the leaf sentinel above
	// would still pass if someone changed the sentinel to a bare errors.New.
	reg, err := NewRegistry(fakeIntegration{name: "slack"})
	require.NoError(t, err)

	_, err = reg.get("nonexistent")
	require.ErrorIs(t, err, apperr.ErrValidation)
}

func TestNewRegistry_RegistersEverySystem(t *testing.T) {
	t.Parallel()
	// Every registered system must be retrievable — a regression that drops or
	// shadows a middle entry would slip past a two-entry lookup test. Fake names
	// rather than the real five, because the property under test is the map, not
	// the product's set.
	want := []string{"alpha", "beta", "gamma", "delta"}

	entries := make([]integrationkinds.Integration, 0, len(want))
	for _, name := range want {
		entries = append(entries, fakeIntegration{name: name})
	}

	reg, err := NewRegistry(entries...)
	require.NoError(t, err)

	for _, name := range want {
		got, err := reg.get(name)
		require.NoError(t, err)
		require.Equal(t, name, got.Name())
	}
}

func TestNewRegistry_DuplicateNameFailsFast(t *testing.T) {
	t.Parallel()
	_, err := NewRegistry(fakeIntegration{name: "slack"}, fakeIntegration{name: "slack"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate name")
}

func TestNewRegistry_EmptyNameFailsFast(t *testing.T) {
	t.Parallel()
	_, err := NewRegistry(fakeIntegration{name: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty name")
}

func TestNewRegistry_Empty(t *testing.T) {
	t.Parallel()
	// An empty registry is valid (no integrations configured yet); every lookup
	// simply reports the kind as unknown.
	reg, err := NewRegistry()
	require.NoError(t, err)

	_, err = reg.get("slack")
	require.ErrorIs(t, err, apperr.ErrUnknownIntegrationKind)
}
