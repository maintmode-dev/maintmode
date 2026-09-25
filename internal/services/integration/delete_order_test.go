package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// TestDelete_RunsTheCascadeBeforeRemovingTheRow pins the statement order inside
// Service.Delete, against a REAL identities store.
//
// The order stopped being a style choice when the column became a foreign key.
// PostgreSQL checks ON DELETE RESTRICT immediately rather than deferring to
// commit, so removing the provider row before its identities fails on children
// that the very next statement would have deleted. Every delete of a provider
// with linked accounts -- the case an admin actually hits -- would 500.
//
// The fake identities store cannot show this: it has no database, so it cannot
// refuse anything. Only a real store puts the foreign key in the path, which is
// why this test is wired differently from the rest of the suite.
//
// Proven by mutation: swapping the two statements in Service.Delete must turn
// this red. It was written after observing that the existing delete tests stay
// green under exactly that swap.
func TestDelete_RunsTheCascadeBeforeRemovingTheRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initServiceWithRealIdentities(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.oidc,
		Enabled: lo.ToPtr(true),
		Config:  presetConfig("corp-client"),
		Secrets: json.RawMessage(`{"client_secret":"corp-secret"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	created, err := svc.GetByKindName(ctx, kinds.login, kinds.oidc)
	require.NoError(t, err)

	// A real identity, so the foreign key has something to refuse.
	identities := useridentities.NewStore(db)
	_, err = identities.Create(ctx, buildIdentity(ctx, t, created.ID))
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, kinds.login, kinds.oidc, actor),
		"the cascade must clear the identities before the row goes, or RESTRICT refuses the delete")
}

// buildIdentity makes an identity owned by a freshly seeded user, referencing
// one provider row.
func buildIdentity(ctx context.Context, t *testing.T, providerID uuid.UUID) *entity.UserIdentity {
	t.Helper()

	var userID uuid.UUID
	err := db.QueryRowxContext(ctx,
		`INSERT INTO users (email, name, roles) VALUES ($1, $2, '{guest}') RETURNING id`,
		xuuid.NewString()+"@delete-order-test.com", "Delete Order Test").Scan(&userID)
	require.NoError(t, err)

	identity := &entity.UserIdentity{
		UserID:  userID,
		Subject: xuuid.NewString(),
		Email:   xuuid.NewString() + "@delete-order-test.com",
	}
	entity.SignInByIntegration(providerID).Apply(identity)

	return identity
}
