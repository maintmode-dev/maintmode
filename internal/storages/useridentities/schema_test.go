package useridentities

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// TestCheckConstraint_ExactlyOneTarget proves the schema refuses an identity
// that names no method and one that names two.
//
// Both are unrepresentable through entity.SignInMethodRef, which sets one column
// and clears the other, so the rows are inserted with raw SQL. That is the
// point: the CHECK protects the table from a future writer that does not go
// through that helper, and a test reaching the constraint only through it would
// be testing the helper instead.
func TestCheckConstraint_ExactlyOneTarget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := seedUser(ctx, t)
	providerID := seedProvider(ctx, t)

	t.Run("neither column set is refused", func(t *testing.T) {
		t.Parallel()

		_, err := db.ExecContext(ctx,
			`INSERT INTO user_identities (user_id, subject, email) VALUES ($1, $2, '')`,
			userID, xuuid.NewString())
		require.ErrorContains(t, err, "user_identities_provider_target_chk",
			"an identity that names no method is unattributable")
	})

	t.Run("both columns set is refused", func(t *testing.T) {
		t.Parallel()

		_, err := db.ExecContext(ctx,
			`INSERT INTO user_identities (user_id, subject, email, integration_id, builtin_method)
			 VALUES ($1, $2, '', $3, 'bootstrap')`,
			userID, xuuid.NewString(), providerID)
		require.ErrorContains(t, err, "user_identities_provider_target_chk",
			"two methods on one identity would let the branches disagree about who vouched")
	})
}

// TestUniqueness_BySubject proves one subject identifies one account, on BOTH
// branches.
//
// The built-in half is the one that matters here. Under a nullable column the
// original full index over (provider, subject) would have kept existing while
// silently ceasing to hold -- NULL is never equal to NULL in PostgreSQL -- so
// two identical break-glass identities would have become insertable with the
// index still listed and every test still green.
func TestUniqueness_BySubject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("registry-backed", func(t *testing.T) {
		t.Parallel()
		providerID := seedProvider(ctx, t)
		method := entity.SignInByIntegration(providerID)

		first := identity(seedUser(ctx, t), method)
		_, err := store.Create(ctx, first)
		require.NoError(t, err)

		// Another user, same provider, same subject: the subject is the
		// provider's own key for a person, so this is the same person twice.
		second := identity(seedUser(ctx, t), method)
		second.Subject = first.Subject

		_, err = store.Create(ctx, second)
		require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)
	})

	t.Run("builtin", func(t *testing.T) {
		t.Parallel()
		method := entity.SignInByBuiltin(entity.AuthMethodBootstrap)

		first := identity(seedUser(ctx, t), method)
		_, err := store.Create(ctx, first)
		require.NoError(t, err)

		second := identity(seedUser(ctx, t), method)
		second.Subject = first.Subject

		_, err = store.Create(ctx, second)
		require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected,
			"a nullable column with the old full index would have admitted this")
	})
}

// TestUniqueness_ByUserAndMethod proves a user holds at most one identity per
// method, on both branches. UnlinkIdentity's last-provider guard counts rows
// per user and is exact only while this holds.
func TestUniqueness_ByUserAndMethod(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("registry-backed", func(t *testing.T) {
		t.Parallel()
		userID := seedUser(ctx, t)
		method := entity.SignInByIntegration(seedProvider(ctx, t))

		_, err := store.Create(ctx, identity(userID, method))
		require.NoError(t, err)

		// Same user and provider under a DIFFERENT subject: a second account at
		// the same provider, which the count-based guard must never see.
		_, err = store.Create(ctx, identity(userID, method))
		require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)
	})

	t.Run("builtin", func(t *testing.T) {
		t.Parallel()
		userID := seedUser(ctx, t)
		method := entity.SignInByBuiltin(entity.AuthMethodBootstrap)

		_, err := store.Create(ctx, identity(userID, method))
		require.NoError(t, err)

		_, err = store.Create(ctx, identity(userID, method))
		require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)
	})
}

// TestForeignKey_RestrictsProviderDelete proves the database refuses to remove
// a provider that still has identities.
//
// This is the guarantee that replaces the old free-text column: the cascade
// lives in Go, and RESTRICT is what turns a cascade that stops running into a
// failed delete instead of an orphaned row waiting for its name to be reused.
func TestForeignKey_RestrictsProviderDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	providerID := seedProvider(ctx, t)

	_, err := store.Create(ctx, identity(seedUser(ctx, t), entity.SignInByIntegration(providerID)))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `DELETE FROM integration_settings WHERE id = $1`, providerID)
	require.ErrorContains(t, err, "user_identities_integration_id_fkey",
		"a provider with live identities must not vanish without the cascade running first")

	// And once the identities go, the provider can.
	removed, err := store.DeleteByIntegrationID(ctx, providerID)
	require.NoError(t, err)
	require.EqualValues(t, 1, removed)

	_, err = db.ExecContext(ctx, `DELETE FROM integration_settings WHERE id = $1`, providerID)
	require.NoError(t, err)
}

// TestListMethods_ReportsRegistryProvidersOnly proves the read path reports
// provider NAMES, sorted, and leaves break-glass out.
//
// Break-glass is the deployment's way in, not a method an account links, and
// these lists are what the UI renders as connected sign-in methods -- so a user
// holding it must not see it there.
//
// Two registry providers, because the order is the contract that matters:
// entity.PrimaryAuthMethod takes the first element, and with one provider there
// is nothing to sort. The comparison is exact, not ElementsMatch, for the same
// reason.
func TestListMethods_ReportsRegistryProvidersOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := seedUser(ctx, t)

	// The first provider seeded gets the name that sorts LAST. Ids are
	// time-ordered, and the join may return rows in id order, so names that
	// followed the same order would arrive sorted and hide a sort that does
	// nothing.
	suffix := xuuid.NewString()
	late := entity.AuthMethod("z-provider-" + suffix)
	early := entity.AuthMethod("a-provider-" + suffix)
	for _, name := range []entity.AuthMethod{late, early} {
		providerID := seedNamedProvider(ctx, t, string(name))
		_, err := store.Create(ctx, identity(userID, entity.SignInByIntegration(providerID)))
		require.NoError(t, err)
	}
	want := []entity.AuthMethod{early, late}

	_, err := store.Create(ctx, identity(userID, entity.SignInByBuiltin(entity.AuthMethodBootstrap)))
	require.NoError(t, err)

	methods, err := store.ListMethodsByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, want, methods,
		"registry providers only, sorted: break-glass is not a connected method")

	// The batch read shares the query and the mapping, and must agree exactly.
	byUser, err := store.ListMethodsByUserIDs(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	require.Equal(t, methods, byUser[userID])
}

// TestBreakGlass_IsOneIdentityPerInstance pins the invariant the production
// constant rests on.
//
// bootstrapauth issues the same subject every time -- entity.BootstrapSubject,
// because there is no upstream provider to issue one -- so a second break-glass
// identity would have to reuse that pair. The index refuses it, and that refusal
// is what makes "one instance, one break-glass account" true rather than merely
// intended.
//
// Asserted here rather than in the service tests, which deliberately give each
// case its own subject: they share a database and would otherwise all resolve
// the first case's user. That parameterization is also why none of them can
// prove this.
func TestBreakGlass_IsOneIdentityPerInstance(t *testing.T) {
	ctx := context.Background()
	method := entity.SignInByBuiltin(entity.AuthMethodBootstrap)

	// NOT parallel and not uniquified: this test owns the real constant for its
	// duration. A parallel case using the same pair would race with it.
	_, err := db.ExecContext(ctx,
		`DELETE FROM user_identities WHERE builtin_method = 'bootstrap' AND subject = $1`,
		entity.BootstrapSubject)
	require.NoError(t, err)

	first := identity(seedUser(ctx, t), method)
	first.Subject = entity.BootstrapSubject
	_, err = store.Create(ctx, first)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, cleanupErr := db.ExecContext(ctx,
			`DELETE FROM user_identities WHERE builtin_method = 'bootstrap' AND subject = $1`,
			entity.BootstrapSubject)
		require.NoError(t, cleanupErr)
	})

	// A second break-glass account, for a different person. In production this
	// is the only shape it could take: the subject is not the caller's to vary.
	second := identity(seedUser(ctx, t), method)
	second.Subject = entity.BootstrapSubject

	_, err = store.Create(ctx, second)
	require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected,
		"an instance admits exactly one break-glass identity")
}
