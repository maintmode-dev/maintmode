package testdbutils

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// LoginProviders resolves the login provider names a test suite signs in with,
// standing in for user.LoginProviderResolver.
//
// It exists because user_identities references integration_settings now: a
// sign-in through `google` needs a `google` row to point at. That is the
// change's real consequence rather than a test artifact -- a stand where nobody
// configured the provider cannot sign anyone in through it either.
//
// It refuses any name it was not seeded with, exactly as the real resolver
// refuses a name with no row. Silently answering some other id would link the
// account to the wrong provider.
type LoginProviders struct {
	byName map[entity.AuthMethod]uuid.UUID
}

// ResolveID implements the resolver the user service depends on.
func (p *LoginProviders) ResolveID(_ context.Context, name entity.AuthMethod) (uuid.UUID, error) {
	id, ok := p.byName[name]
	if !ok {
		return uuid.Nil, apperr.ErrIntegrationNotFound
	}

	return id, nil
}

// ID returns the seeded row id for one name, for assertions that need to
// address an identity directly.
func (p *LoginProviders) ID(name entity.AuthMethod) uuid.UUID {
	return p.byName[name]
}

// SeedLoginProviders inserts one registry row per name and returns a resolver
// over them.
//
// Written with raw SQL rather than through the integration service: that
// service belongs to another module, and importing its store from an auth-side
// test is forbidden (see .golangci.yaml, module-auth-stores). What a test needs
// here is a parent row, not the registry's behavior.
//
// Rows are keyed by the fixed names the suites sign in with and upserted, so a
// second -count run reuses them instead of colliding on UNIQUE (kind, name).
// Built-in methods must NOT be passed: they never reach a resolver, and a row
// for one would assert a relationship that does not exist.
func SeedLoginProviders(
	ctx context.Context, db *sqlx.DB, kekID string, names ...entity.AuthMethod,
) (*LoginProviders, error) {
	var dekID uuid.UUID
	if err := db.QueryRowxContext(ctx,
		`INSERT INTO data_keys (kek_id, encrypted_dek) VALUES ($1, $2) RETURNING id`,
		kekID, []byte("wrapped"),
	).Scan(&dekID); err != nil {
		return nil, err
	}

	byName := make(map[entity.AuthMethod]uuid.UUID, len(names))

	for _, name := range names {
		var id uuid.UUID
		if err := db.QueryRowxContext(ctx,
			`INSERT INTO integration_settings (kind, name, enabled, config, secrets, dek_id)
			 VALUES ('login', $1, true, '{}'::jsonb, '{}'::jsonb, $2)
			 ON CONFLICT (kind, name) DO UPDATE SET updated_at = now()
			 RETURNING id`,
			string(name), dekID,
		).Scan(&id); err != nil {
			return nil, err
		}

		byName[name] = id
	}

	return &LoginProviders{byName: byName}, nil
}

// MustSeedLoginProviders is SeedLoginProviders for TestMain, where there is no
// *testing.T to fail and a suite whose providers are missing can only produce
// misleading failures in every sign-in test that follows.
func MustSeedLoginProviders(
	ctx context.Context, db *sqlx.DB, kekID string, names ...entity.AuthMethod,
) *LoginProviders {
	providers, err := SeedLoginProviders(ctx, db, kekID, names...)
	if err != nil {
		panic("seed login providers: " + err.Error())
	}

	return providers
}
