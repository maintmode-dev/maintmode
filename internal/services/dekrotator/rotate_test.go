package dekrotator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mock_dekrotator "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/dekrotator"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
)

// The rotation tests share the data_keys table with other suites on the shared
// dev DB and do NOT wipe it. Isolation is by key identity instead: every test
// mints unique KEK URIs (uniqueKEKURI), so foreign rows are always "unknown
// KEK" to the test's keyring — skipped by rotation, invisible to `rewrapped`,
// and untouched. Assertions therefore check exact counts for the test's own
// rows, but only lower bounds (or nothing) for table-wide totals like
// `skipped`, which foreign rows legitimately inflate.

func TestRotator_ReWrapsAllDEKsUnderNewKEK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1, kek2 := uniqueKEKURI("kek-1"), uniqueKEKURI("kek-2")

	// Seed DEKs under kek-1, then rotate to a keyring whose active is kek-2 but
	// which still holds kek-1 so the old DEKs unwrap.
	old := keyringWith(t, kek1, kek1)
	a, dekA := seedDEK(ctx, t, old, "rot-a")
	b, dekB := seedDEK(ctx, t, old, "rot-b")

	rotated := keyringWith(t, kek2, kek1, kek2)
	rot := NewRotator(txManager, store, rotated)

	n, _, err := rot.Rotate(ctx)
	require.NoError(t, err)
	// Exact count works without wiping the table: rewrapped only counts rows
	// under a KEK this keyring knows, and the unique kek-1 exists on exactly the
	// two rows seeded above. (skipped is not asserted — foreign rows inflate it.)
	require.Equal(t, 2, n, "both seeded DEKs must be re-wrapped")

	// Both rows now carry the active KEK id and still unwrap to the same DEK.
	gotA := findByID(ctx, t, a.ID)
	require.Equal(t, kek2, gotA.KEKID)
	unwrappedA, err := rotated.UnwrapDEK(gotA.EncryptedDEK, gotA.KEKID)
	require.NoError(t, err)
	require.NoError(t, secrets.EquivalentDEKs(dekA, unwrappedA), "DEK must be unchanged by rotation")

	gotB := findByID(ctx, t, b.ID)
	require.Equal(t, kek2, gotB.KEKID)
	unwrappedB, err := rotated.UnwrapDEK(gotB.EncryptedDEK, gotB.KEKID)
	require.NoError(t, err)
	require.NoError(t, secrets.EquivalentDEKs(dekB, unwrappedB))

	// Post-condition (ship-audit T2): no row anywhere references the retired KEK
	// after a successful rotation. Scanning the whole table stays valid without a
	// wipe because kek1 is unique to this run — only the two rows above ever
	// carried it.
	all, err := store.ListForUpdate(ctx)
	require.NoError(t, err)
	for _, dk := range all {
		require.NotEqual(t, kek1, dk.KEKID, "no row may reference the retired KEK after rotation")
	}
}

func TestRotator_SkipsRowsAlreadyOnActiveKEK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1, kek2 := uniqueKEKURI("kek-1"), uniqueKEKURI("kek-2")

	// One row on the retired KEK, one already wrapped under the active KEK.
	full := keyringWith(t, kek2, kek1, kek2)
	retired, _ := seedDEK(ctx, t, keyringWith(t, kek1, kek1), "retired")
	active, _ := seedDEK(ctx, t, full, "active")
	activeBefore := findByID(ctx, t, active.ID)

	n, _, err := NewRotator(txManager, store, full).Rotate(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n, "only the retired-KEK row is re-wrapped; the active one is skipped")

	// The already-active row is left byte-for-byte untouched (not re-wrapped).
	activeAfter := findByID(ctx, t, active.ID)
	require.Equal(t, activeBefore.EncryptedDEK, activeAfter.EncryptedDEK)
	require.Equal(t, kek2, activeAfter.KEKID)

	// The retired row moved to the active KEK.
	require.Equal(t, kek2, findByID(ctx, t, retired.ID).KEKID)
}

func TestRotator_SecretSealedBeforeRotationStillReadable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1, kek2 := uniqueKEKURI("kek-1"), uniqueKEKURI("kek-2")

	old := keyringWith(t, kek1, kek1)
	row, dek := seedDEK(ctx, t, old, "secret")

	// Seal a secret with the DEK before rotation. The AAD binds the envelope to a
	// (kind, key) slot; rotation only re-wraps the DEK, so the DEK-sealed secret
	// (and its AAD) is untouched and must still open afterward.
	cipher := secrets.NewAESCipher()
	aad := secrets.SecretAAD("slack", "bot_token")
	sealed, err := cipher.Encrypt(dek, []byte("xoxb-token"), aad)
	require.NoError(t, err)

	rotated := keyringWith(t, kek2, kek1, kek2)
	_, _, err = NewRotator(txManager, store, rotated).Rotate(ctx)
	require.NoError(t, err)

	// The DEK plaintext is unchanged, so unwrapping the rotated row yields the
	// same DEK, which still opens the pre-rotation secret.
	got := findByID(ctx, t, row.ID)
	dekAfter, err := rotated.UnwrapDEK(got.EncryptedDEK, got.KEKID)
	require.NoError(t, err)

	plaintext, err := cipher.Decrypt(dekAfter, sealed, aad)
	require.NoError(t, err)
	require.Equal(t, []byte("xoxb-token"), plaintext)
}

// A DEK sealed under a KEK the rotation keyring does not hold is skipped, not
// re-wrapped and not an error: rotation only migrates DEKs it can unwrap. This is
// what keeps a startup rotation from aborting on an orphaned/foreign row (e.g. a
// leftover on a shared dev DB) instead of failing the whole boot.
func TestRotator_SkipsRowsUnderUnknownKEK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1, kek2, orphanKEK := uniqueKEKURI("kek-1"), uniqueKEKURI("kek-2"), uniqueKEKURI("orphan-kek")

	// Seed one row whose KEK the rotation keyring will NOT know, and one it will.
	orphan, _ := seedDEK(ctx, t, keyringWith(t, orphanKEK, orphanKEK), "orphan")
	orphanBefore := findByID(ctx, t, orphan.ID)
	live, _ := seedDEK(ctx, t, keyringWith(t, kek1, kek1), "live")

	// Rotation keyring holds kek-1 and kek-2 but not orphan-kek.
	rotated := keyringWith(t, kek2, kek1, kek2)
	n, skipped, err := NewRotator(txManager, store, rotated).Rotate(ctx)
	require.NoError(t, err, "an unknown-KEK row must not fail rotation")
	require.Equal(t, 1, n, "only the known-KEK row is re-wrapped")
	// At least the orphan is skipped; foreign rows from other suites/runs also
	// land in skipped, so only a lower bound holds on the shared table.
	require.GreaterOrEqual(t, skipped, 1, "the unknown-KEK row is skipped")

	// The orphan row is left byte-for-byte untouched under its original KEK.
	orphanAfter := findByID(ctx, t, orphan.ID)
	require.Equal(t, orphanBefore.KEKID, orphanAfter.KEKID)
	require.Equal(t, orphanBefore.EncryptedDEK, orphanAfter.EncryptedDEK)

	// The live row moved onto the active KEK.
	require.Equal(t, kek2, findByID(ctx, t, live.ID).KEKID)
}

func TestRotator_UnwrapFailureMidwayRollsBackFully(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1 := uniqueKEKURI("kek-1")

	old := keyringWith(t, kek1, kek1)
	row, _ := seedDEK(ctx, t, old, "rollback")
	before := findByID(ctx, t, row.ID)

	// A keyring whose active id is not in its key set would fail fast at
	// construction, so inject a mock that fails on wrap to force a mid-rotation
	// error, and assert the row is untouched afterwards. Knows()==true means the
	// mock claims every row on the shared table, so the wrap failure fires on the
	// first non-active row it meets — whichever it is, the whole tx must roll back.
	failing := failingKeyring(gomock.NewController(t), errors.New("boom"))
	_, _, err := NewRotator(txManager, store, failing).Rotate(ctx)
	require.Error(t, err)

	after := findByID(ctx, t, row.ID)
	require.Equal(t, before.KEKID, after.KEKID, "kek_id must be unchanged after rollback")
	require.Equal(t, before.EncryptedDEK, after.EncryptedDEK, "envelope must be unchanged after rollback")
}

func TestRotator_PersistedRowRolledBackWhenLaterRowFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1, kek2 := uniqueKEKURI("kek-1"), uniqueKEKURI("kek-2")

	// Two rows on the retired KEK. Row 1 re-wraps and persists successfully; row 2
	// fails on wrap. The whole transaction must roll back — including row 1's
	// already-executed UPDATE — proving atomicity across rows, not just per-row.
	// The delegate keyring knows only this test's unique KEKs, so foreign rows are
	// skipped before WrapDEK and cannot consume the failAt budget.
	old := keyringWith(t, kek1, kek1)
	r1, _ := seedDEK(ctx, t, old, "atomic-1")
	r2, _ := seedDEK(ctx, t, old, "atomic-2")
	before1 := findByID(ctx, t, r1.ID)
	before2 := findByID(ctx, t, r2.ID)

	// Real re-wrap for the first WrapDEK call, hard failure on the second.
	failOnSecond := &failOnNthWrap{
		delegate: keyringWith(t, kek2, kek1, kek2),
		failAt:   2,
	}
	n, _, err := NewRotator(txManager, store, failOnSecond).Rotate(ctx)
	require.Error(t, err)
	require.Equal(t, 0, n, "rollback reports zero re-wrapped")

	after1 := findByID(ctx, t, r1.ID)
	after2 := findByID(ctx, t, r2.ID)
	require.Equal(t, before1.KEKID, after1.KEKID, "row 1's persisted re-wrap must roll back")
	require.Equal(t, before1.EncryptedDEK, after1.EncryptedDEK, "row 1 envelope must roll back")
	require.Equal(t, before2.KEKID, after2.KEKID, "row 2 must be untouched")
	require.Equal(t, before2.EncryptedDEK, after2.EncryptedDEK)
}

func TestRotator_VerifyCatchesWrapOfDifferentKeyMaterial(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1, kek2 := uniqueKEKURI("kek-1"), uniqueKEKURI("kek-2")

	old := keyringWith(t, kek1, kek1)
	row, _ := seedDEK(ctx, t, old, "verify-guard")
	before := findByID(ctx, t, row.ID)

	// A wrap that silently swaps in different key material must be caught by the
	// functional verify (probe under the original DEK, open under the unwrapped
	// re-wrap) before the stored envelope is overwritten.
	swapping := &swappingKeyring{delegate: keyringWith(t, kek2, kek1, kek2)}
	_, _, err := NewRotator(txManager, store, swapping).Rotate(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "verify mismatch")

	after := findByID(ctx, t, row.ID)
	require.Equal(t, before.KEKID, after.KEKID, "row must be untouched after verify failure")
	require.Equal(t, before.EncryptedDEK, after.EncryptedDEK)
}

// swappingKeyring delegates everything but wraps a FRESH DEK instead of the one
// it was given — simulating a wrap that does not preserve key material.
type swappingKeyring struct {
	delegate Keyring
}

func (s *swappingKeyring) ActiveKEKID() string     { return s.delegate.ActiveKEKID() }
func (s *swappingKeyring) Knows(kekID string) bool { return s.delegate.Knows(kekID) }

func (s *swappingKeyring) UnwrapDEK(wrapped []byte, kekID string) ([]byte, error) {
	return s.delegate.UnwrapDEK(wrapped, kekID)
}

func (s *swappingKeyring) WrapDEK(_ []byte) (wrapped []byte, kekID string, err error) {
	other, err := secrets.GenerateDEK()
	if err != nil {
		return nil, "", err
	}
	return s.delegate.WrapDEK(other)
}

// failingKeyring implements the rotation keyring but fails wrap to exercise the
// rollback path.
// failingKeyring builds a mock Keyring that unwraps successfully but fails every
// wrap with wrapErr, driving rotation into its rollback path. Knows()==true so
// every row is attempted (never skipped) and UnwrapDEK echoes its input so the
// flow reaches the wrap failure. All expectations are AnyTimes: the count of
// foreign rows on the shared table is not fixed.
func failingKeyring(ctrl *gomock.Controller, wrapErr error) *mock_dekrotator.MockKeyring {
	kr := mock_dekrotator.NewMockKeyring(ctrl)
	kr.EXPECT().ActiveKEKID().Return(uniqueKEKURI("kek-2")).AnyTimes()
	kr.EXPECT().Knows(gomock.Any()).Return(true).AnyTimes()
	kr.EXPECT().UnwrapDEK(gomock.Any(), gomock.Any()).
		DoAndReturn(func(wrapped []byte, _ string) ([]byte, error) { return wrapped, nil }).AnyTimes()
	kr.EXPECT().WrapDEK(gomock.Any()).Return(nil, "", wrapErr).AnyTimes()
	return kr
}

// failOnNthWrap delegates to a real keyring but fails the failAt-th WrapDEK call,
// so rotation persists earlier rows and then aborts, exercising cross-row rollback.
type failOnNthWrap struct {
	delegate Keyring
	failAt   int
	calls    int
}

func (f *failOnNthWrap) ActiveKEKID() string { return f.delegate.ActiveKEKID() }

func (f *failOnNthWrap) Knows(kekID string) bool { return f.delegate.Knows(kekID) }

func (f *failOnNthWrap) UnwrapDEK(wrapped []byte, kekID string) ([]byte, error) {
	return f.delegate.UnwrapDEK(wrapped, kekID)
}

func (f *failOnNthWrap) WrapDEK(dek []byte) (wrapped []byte, kekID string, err error) {
	f.calls++
	if f.calls == f.failAt {
		return nil, "", errors.New("boom on nth wrap")
	}
	return f.delegate.WrapDEK(dek)
}
