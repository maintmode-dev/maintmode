package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// TestUnlinkIdentity_RegistryBackedNoOp covers the idempotent half of the
// disconnect contract on the REGISTRY branch.
//
// The existing no-op test uses AuthMethodStub, which is built-in and returns
// before any lookup happens -- so it never exercises the path where the name
// has to be matched against the user's own identities. That path answers
// ErrProviderNotConnected when nothing matches, and the service must turn it
// into success: "you are not linked to this" and "you asked to stop being
// linked to this" describe the same desired state, and a client retrying a
// disconnect it already completed must not start seeing errors.
//
// Without this test, letting that error escape leaves the suite green.
func TestUnlinkIdentity_RegistryBackedNoOp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := initService(t)

	// Two identities, so the last-provider guard passes and cannot mask the
	// behavior under test.
	user := makeUser(ctx, t, srv)
	require.NoError(t, srv.LinkIdentity(ctx, user.ID, entity.AuthMethodGithub,
		claimsFor("gh-"+xuuid.NewString()+"@example.com")))

	// A real, registry-backed provider this user was never linked to.
	unlinkedName := entity.AuthMethod("unlinked-" + xuuid.NewString())
	unlinkedID := seedExtraProvider(ctx, t, unlinkedName)
	srv.WithLoginProviderResolver(newResolverWith(unlinkedName, unlinkedID))

	require.NoError(t, srv.UnlinkIdentity(ctx, user.ID, unlinkedName),
		"disconnecting a registry-backed provider the user never linked is already satisfied")

	providers, err := srv.ListConnectedProviders(ctx, user.ID)
	require.NoError(t, err)
	require.ElementsMatch(t,
		[]entity.AuthMethod{entity.AuthMethodGoogle, entity.AuthMethodGithub}, providers,
		"a no-op must remove nothing")
}

// TestIdentity_CannotOutliveItsProvider is the guarantee this whole change
// exists for, asserted directly.
//
// The orphan was the danger: a row left behind after its provider was deleted
// waited for the same NAME to be created again against a different IdP, and
// then vouched for that IdP against an account predating it. With the reference
// in the schema the state is not merely avoided by careful code -- it cannot be
// written down.
//
// Both halves are asserted, because either alone would pass against a weaker
// schema: the provider cannot be deleted out from under a live identity
// (RESTRICT), and an identity cannot be pointed at a provider that is already
// gone (the reference itself).
func TestIdentity_CannotOutliveItsProvider(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := initService(t)

	doomedName := entity.AuthMethod("doomed-" + xuuid.NewString())
	doomedID := seedExtraProvider(ctx, t, doomedName)

	user := makeUser(ctx, t, srv)
	srv.WithLoginProviderResolver(newResolverWith(doomedName, doomedID))
	require.NoError(t, srv.LinkIdentity(ctx, user.ID, doomedName,
		claimsFor(xuuid.NewString()+"@doomed-test.com")))

	_, err := db.ExecContext(ctx, `DELETE FROM integration_settings WHERE id = $1`, doomedID)
	require.ErrorContains(t, err, "user_identities_integration_id_fkey",
		"a provider with a live identity must not vanish and leave it behind")

	// And the reverse: once the provider is gone, nothing can point back at it.
	_, err = db.ExecContext(ctx,
		`DELETE FROM user_identities WHERE user_id = $1 AND integration_id = $2`, user.ID, doomedID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM integration_settings WHERE id = $1`, doomedID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO user_identities (user_id, subject, email, integration_id) VALUES ($1, $2, '', $3)`,
		user.ID, xuuid.NewString(), doomedID)
	require.ErrorContains(t, err, "user_identities_integration_id_fkey",
		"an identity must not be able to reference a provider that no longer exists")
}

// seedExtraProvider inserts one login row under a caller-chosen name, for tests
// that need a provider of their own to delete or leave unlinked.
func seedExtraProvider(ctx context.Context, t *testing.T, name entity.AuthMethod) uuid.UUID {
	t.Helper()

	var dekID uuid.UUID
	require.NoError(t, db.QueryRowxContext(ctx,
		`INSERT INTO data_keys (kek_id, encrypted_dek) VALUES ($1, $2) RETURNING id`,
		"unlink-test-kek", []byte("wrapped")).Scan(&dekID))

	var id uuid.UUID
	require.NoError(t, db.QueryRowxContext(ctx,
		`INSERT INTO integration_settings (kind, name, enabled, config, secrets, dek_id)
		 VALUES ('login', $1, true, '{}'::jsonb, '{}'::jsonb, $2) RETURNING id`,
		string(name), dekID).Scan(&id))

	return id
}

// newResolverWith resolves one extra provider on top of the suite's own.
func newResolverWith(name entity.AuthMethod, id uuid.UUID) *extraResolver {
	return &extraResolver{name: name, id: id}
}

type extraResolver struct {
	name entity.AuthMethod
	id   uuid.UUID
}

func (r *extraResolver) ResolveID(ctx context.Context, name entity.AuthMethod) (uuid.UUID, error) {
	if name == r.name {
		return r.id, nil
	}

	return loginProviders.ResolveID(ctx, name)
}
