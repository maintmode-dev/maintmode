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

// TestIntegrationID_IsRequired proves the schema refuses an identity that
// names no provider. entity.UserIdentity cannot express one, so the row is
// inserted with raw SQL: the constraint protects the table from a writer that
// does not go through it.
func TestIntegrationID_IsRequired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO user_identities (user_id, subject, email) VALUES ($1, $2, '')`,
		seedUser(ctx, t), xuuid.NewString())
	require.ErrorContains(t, err, "integration_id",
		"an identity that names no provider is unattributable")
}

// TestUniqueness_BySubject proves one subject identifies one account.
func TestUniqueness_BySubject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	providerID := seedProvider(ctx, t)

	first := identity(seedUser(ctx, t), providerID)
	_, err := store.Create(ctx, first)
	require.NoError(t, err)

	// Another user, same provider, same subject: the subject is the
	// provider's own key for a person, so this is the same person twice.
	second := identity(seedUser(ctx, t), providerID)
	second.Subject = first.Subject

	_, err = store.Create(ctx, second)
	require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)
}

// TestUniqueness_ByUserAndMethod proves a user holds at most one identity per
// method. UnlinkIdentity's last-provider guard counts rows per user and is
// exact only while this holds.
func TestUniqueness_ByUserAndMethod(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	userID := seedUser(ctx, t)
	providerID := seedProvider(ctx, t)

	_, err := store.Create(ctx, identity(userID, providerID))
	require.NoError(t, err)

	// Same user and provider under a DIFFERENT subject: a second account at
	// the same provider, which the count-based guard must never see.
	_, err = store.Create(ctx, identity(userID, providerID))
	require.ErrorIs(t, err, apperr.ErrProviderAlreadyConnected)
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

	_, err := store.Create(ctx, identity(seedUser(ctx, t), providerID))
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
// provider NAMES, sorted.
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
		_, err := store.Create(ctx, identity(userID, providerID))
		require.NoError(t, err)
	}
	want := []entity.AuthMethod{early, late}

	methods, err := store.ListMethodsByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, want, methods, "registry providers, sorted by name")

	// The batch read shares the query and the mapping, and must agree exactly.
	byUser, err := store.ListMethodsByUserIDs(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	require.Equal(t, methods, byUser[userID])
}
