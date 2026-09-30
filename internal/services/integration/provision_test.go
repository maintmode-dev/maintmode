package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
)

// None of the tests in this file are parallel, and that is load-bearing:
// Provision releases EVERY provisioned login row its config does not declare,
// so two of them running at once would release each other's rows. Serial
// top-level tests never overlap the package's parallel ones.
//
// The same reach extends past this package: run against a database where a
// dev app has provisioned a login provider, these tests release it until that
// app restarts.

// declared is a valid declared entry for the fixture's preset login name:
// facts, enabled and settings side by side, with the secret already moved out
// of the settings the way the config loader leaves it.
func declared(clientID string) config.LoginProvider {
	return config.LoginProvider{
		LoginFacts: config.LoginFacts{DisplayName: "Test IdP", IssuerURL: testPresetIssuer},
		ManagedEntry: config.ManagedEntry{
			ManagedBy: config.ManagedByConfig,
			Enabled:   lo.ToPtr(true),
			Settings: map[string]any{
				"client_id":    clientID,
				"redirect_uri": "https://app.example/auth/callback",
				"jwtverifier":  map[string]any{"allowed_hosted_domains": []any{"example.com"}},
			},
			Secrets: map[string]string{"client_secret": "from-the-secrets-file"},
		},
	}
}

// storedRow reads a row's raw columns, bypassing the service's masking.
type storedRow struct {
	ID          uuid.UUID `db:"id"`
	Enabled     bool      `db:"enabled"`
	Provisioned bool      `db:"provisioned"`
	Secrets     string    `db:"secrets"`
	Config      string    `db:"config"`
	UpdatedAt   time.Time `db:"updated_at"`
	UpdatedBy   *string   `db:"updated_by_user_id"`
}

func readRow(ctx context.Context, t *testing.T, kind, name string) (storedRow, bool) {
	t.Helper()

	var row storedRow
	err := db.GetContext(ctx, &row, `
		SELECT id, enabled, provisioned, secrets::text AS secrets, config::text AS config, updated_at, updated_by_user_id
		FROM integration_settings WHERE kind = $1 AND name = $2`, kind, name)
	if err != nil {
		return storedRow{}, false
	}

	return row, true
}

func TestProvision_InsertsADeclaredProvider(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	svc, _ := newServiceFor(t, kinds, nil, config.LoginProviders{kinds.oidc: declared("cfg-client")})

	require.NoError(t, svc.Provision(ctx))

	row, found := readRow(ctx, t, kinds.login, kinds.oidc)
	require.True(t, found)
	require.True(t, row.Provisioned)
	require.True(t, row.Enabled)
	require.JSONEq(t, `{}`, row.Secrets, "the secret is never written to the database")
	require.Contains(t, row.Config, testPresetIssuer, "the facts come from the declared entry")
}

// The take-over is why the flag lives on the existing row rather than in a
// table of its own: the identities point at the id, and the id survives.
func TestProvision_TakeOverKeepsTheRowAndItsIdentities(t *testing.T) {
	ctx := context.Background()
	adminSvc, kinds, _ := initServiceWithRealIdentities(t)
	_, err := adminSvc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind: kinds.login, Name: kinds.oidc, Enabled: lo.ToPtr(true),
		Config:  presetConfig("admin-client"),
		Secrets: json.RawMessage(`{"client_secret":"admin-typed"}`),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	before, _ := readRow(ctx, t, kinds.login, kinds.oidc)

	identities := useridentities.NewStore(db)
	identity, err := identities.Create(ctx, buildIdentity(ctx, t, before.ID))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM user_identities WHERE id = $1`, identity.ID) })

	svc, _ := newServiceFor(t, kinds, identities,
		config.LoginProviders{kinds.oidc: declared("cfg-client")})
	require.NoError(t, svc.Provision(ctx))

	after, _ := readRow(ctx, t, kinds.login, kinds.oidc)
	require.Equal(t, before.ID, after.ID)
	require.True(t, after.Provisioned)
	require.JSONEq(t, `{}`, after.Secrets, "the admin-entered secret is wiped")
	require.Contains(t, after.Config, "cfg-client")
	require.Nil(t, after.UpdatedBy)

	var linkedTo uuid.UUID
	require.NoError(t, db.GetContext(ctx, &linkedTo,
		`SELECT integration_id FROM user_identities WHERE id = $1`, identity.ID))
	require.Equal(t, before.ID, linkedTo, "the identity still points at the same provider")
}

func TestProvision_ReleasesAProviderDroppedFromConfig(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	first, _ := newServiceFor(t, kinds, nil, config.LoginProviders{kinds.oidc: declared("cfg-client")})
	require.NoError(t, first.Provision(ctx))

	next, mocks := newServiceFor(t, kinds, nil, config.LoginProviders{})
	require.NoError(t, next.Provision(ctx))

	row, found := readRow(ctx, t, kinds.login, kinds.oidc)
	require.True(t, found, "a released row stays")
	require.False(t, row.Provisioned)
	require.False(t, row.Enabled)
	require.Contains(t, row.Config, "cfg-client", "config is left as it was")
	require.Equal(t, uuid.Nil, mocks.identities.unlinkedFrom, "identities are untouched")
}

// Release reaches only the names this service's registry holds. Another run's
// renamed kinds -- or a stand's real rows on the same database -- are not this
// config's to release, even though they are provisioned and undeclared here.
func TestProvision_ReleaseLeavesUnregisteredNamesAlone(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	owner, _ := newServiceFor(t, kinds, nil, config.LoginProviders{kinds.oidc: declared("cfg-client")})
	require.NoError(t, owner.Provision(ctx))

	_, otherKinds, _ := initService(t)
	stranger, _ := newServiceFor(t, otherKinds, nil, config.LoginProviders{})
	require.NoError(t, stranger.Provision(ctx))

	row, found := readRow(ctx, t, kinds.login, kinds.oidc)
	require.True(t, found)
	require.True(t, row.Provisioned, "a name outside the registry is not released")
	require.True(t, row.Enabled)
}

func TestProvision_InvalidEntryChangesNothing(t *testing.T) {
	cases := []struct {
		name string
		decl func(kinds testKinds) config.LoginProviders
	}{
		{
			name: "unknown name",
			decl: func(kinds testKinds) config.LoginProviders {
				return config.LoginProviders{
					kinds.oidc: declared("cfg-client"), "no-such-provider": declared("x"),
				}
			},
		},
		{
			// The config file is committed. Which field is a secret is the
			// provider kind's to say, and that one must be a reference.
			name: "literal client_secret",
			decl: func(kinds testKinds) config.LoginProviders {
				bad := declared("cfg-client")
				bad.Secrets = nil
				bad.Settings["client_secret"] = "from-the-secrets-file"
				return config.LoginProviders{kinds.oidc: bad}
			},
		},
		{
			// A pinned-off entry never reaches Validate, and its config is
			// still written into the row.
			name: "literal client_secret on a pinned-off entry",
			decl: func(kinds testKinds) config.LoginProviders {
				bad := declared("cfg-client")
				bad.Enabled = lo.ToPtr(false)
				bad.Secrets = nil
				bad.Settings["client_secret"] = "from-the-secrets-file"
				return config.LoginProviders{kinds.oidc: bad}
			},
		},
		{
			name: "missing client_secret",
			decl: func(kinds testKinds) config.LoginProviders {
				bad := declared("cfg-client")
				bad.Secrets = nil
				return config.LoginProviders{kinds.oidc: bad}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			_, kinds, _ := initService(t)
			svc, _ := newServiceFor(t, kinds, nil, tc.decl(kinds))

			err := svc.Provision(ctx)

			require.ErrorIs(t, err, apperr.ErrValidation)
			require.NotContains(t, err.Error(), "from-the-secrets-file")
			_, found := readRow(ctx, t, kinds.login, kinds.oidc)
			require.False(t, found, "nothing is written when any entry is invalid")
		})
	}
}
