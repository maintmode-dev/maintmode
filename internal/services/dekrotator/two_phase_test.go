package dekrotator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/pkg/secrets"
)

// These tests exercise the operational TWO-PHASE KEK rotation procedure end to
// end at the rotator level, as opposed to rotate_test.go which pins a single
// re-wrap. Each test walks the config through the states an operator actually
// deploys — phase A (add the new KEK, keep the old active), phase B (flip the
// active KEK), and the rollbacks — and asserts secrets stay readable at every
// step. The dangerous "rollback past phase A" case is asserted explicitly: it
// is the one path the config comments warn about but nothing else covers.
//
// Isolation follows the same rule as rotate_test.go: every test mints unique KEK
// URIs (uniqueKEKURI), so foreign rows on the shared dev DB sit under KEKs these
// keyrings do not know and are skipped, never re-wrapped.

// sealSecret encrypts a plaintext secret with dek under a fixed AAD and returns
// the envelope. Rotation only re-wraps the DEK, so the same envelope must open
// with the DEK recovered after any rotation step.
func sealSecret(t *testing.T, dek []byte, plaintext string) (envelope, aad []byte) {
	t.Helper()
	aad = secrets.SecretAAD("slack", "bot_token")
	envelope, err := secrets.NewAESCipher().Encrypt(dek, []byte(plaintext), aad)
	require.NoError(t, err)
	return envelope, aad
}

// TestTwoPhase_PhaseA_AddsNewKEK_ActiveStaysOld models the first deploy of a
// rotation: the new KEK is present in local_keys but active_kek_uri still points
// at the old one. Nothing is re-wrapped yet, and the pre-existing secret stays
// readable. This is the safe, reversible half of the procedure.
func TestTwoPhase_PhaseA_AddsNewKEK_ActiveStaysOld(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1, kek2 := uniqueKEKURI("kek-1"), uniqueKEKURI("kek-2")

	// Secret sealed under a DEK wrapped by kek-1, before rotation begins.
	old := keyringWith(t, kek1, kek1)
	row, dek := seedDEK(ctx, t, old, "phaseA")
	envelope, aad := sealSecret(t, dek, "xoxb-phaseA")

	// Phase A keyring: active is STILL kek-1, but kek-2 is now known.
	phaseA := keyringWith(t, kek1, kek1, kek2)
	require.Equal(t, kek1, phaseA.ActiveKEKID(), "phase A must keep the old KEK active")

	n, _, err := NewRotator(txManager, store, phaseA).Rotate(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, n, "phase A re-wraps nothing: the row is already on the active KEK")

	// The row is untouched and the secret still opens under kek-1.
	got := findByID(ctx, t, row.ID)
	require.Equal(t, kek1, got.KEKID)
	dekAfter, err := phaseA.UnwrapDEK(got.EncryptedDEK, got.KEKID)
	require.NoError(t, err)
	plaintext, err := secrets.NewAESCipher().Decrypt(dekAfter, envelope, aad)
	require.NoError(t, err)
	require.Equal(t, "xoxb-phaseA", string(plaintext))
}

// TestTwoPhase_PhaseB_FlipReWrapsOntoNew models the second deploy: active_kek_uri
// flips to the new KEK while both keys remain in local_keys. Boot re-wraps every
// DEK onto the new KEK, and the secret sealed before rotation still opens.
func TestTwoPhase_PhaseB_FlipReWrapsOntoNew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1, kek2 := uniqueKEKURI("kek-1"), uniqueKEKURI("kek-2")

	old := keyringWith(t, kek1, kek1)
	row, dek := seedDEK(ctx, t, old, "phaseB")
	envelope, aad := sealSecret(t, dek, "xoxb-phaseB")

	// Phase B keyring: active flipped to kek-2, both keys still present.
	phaseB := keyringWith(t, kek2, kek1, kek2)
	require.Equal(t, kek2, phaseB.ActiveKEKID())

	n, _, err := NewRotator(txManager, store, phaseB).Rotate(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n, "phase B re-wraps the seeded DEK onto the new active KEK")

	// The row now carries the new KEK and the pre-rotation secret still opens.
	got := findByID(ctx, t, row.ID)
	require.Equal(t, kek2, got.KEKID)
	dekAfter, err := phaseB.UnwrapDEK(got.EncryptedDEK, got.KEKID)
	require.NoError(t, err)
	plaintext, err := secrets.NewAESCipher().Decrypt(dekAfter, envelope, aad)
	require.NoError(t, err)
	require.Equal(t, "xoxb-phaseB", string(plaintext))
}

// TestTwoPhase_RollbackBtoA_SelfHeals proves the config comment's claim that a
// rollback from phase B to phase A is self-healing: the phase-A config still
// knows the new KEK, so booting it simply re-wraps every DEK back onto its
// (old) active KEK. No error, no data loss.
func TestTwoPhase_RollbackBtoA_SelfHeals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1, kek2 := uniqueKEKURI("kek-1"), uniqueKEKURI("kek-2")

	old := keyringWith(t, kek1, kek1)
	row, dek := seedDEK(ctx, t, old, "rollbackBA")
	envelope, aad := sealSecret(t, dek, "xoxb-rollbackBA")

	// Phase B: flip to kek-2, re-wrap everything onto it.
	phaseB := keyringWith(t, kek2, kek1, kek2)
	_, _, err := NewRotator(txManager, store, phaseB).Rotate(ctx)
	require.NoError(t, err)
	require.Equal(t, kek2, findByID(ctx, t, row.ID).KEKID)

	// Rollback to phase A: active is kek-1 again, but kek-2 is still known, so the
	// row wrapped under kek-2 can be unwrapped and re-wrapped back onto kek-1.
	phaseA := keyringWith(t, kek1, kek1, kek2)
	n, _, err := NewRotator(txManager, store, phaseA).Rotate(ctx)
	require.NoError(t, err, "rollback B->A must self-heal, not fail")
	require.Equal(t, 1, n, "the kek-2 row is re-wrapped back onto kek-1")

	// Row is back on kek-1 and the secret still opens.
	got := findByID(ctx, t, row.ID)
	require.Equal(t, kek1, got.KEKID)
	dekAfter, err := phaseA.UnwrapDEK(got.EncryptedDEK, got.KEKID)
	require.NoError(t, err)
	plaintext, err := secrets.NewAESCipher().Decrypt(dekAfter, envelope, aad)
	require.NoError(t, err)
	require.Equal(t, "xoxb-rollbackBA", string(plaintext))
}

// TestTwoPhase_RollbackPastA_BootWarnsButUnwrapFails pins the catastrophic case
// the config comments warn about (app.config.yaml crypto section, rotate.go's
// doc): once phase B has booted (DEKs wrapped under the new KEK), rolling back
// to a config that has NEVER seen the new KEK — one that predates phase A — does
// NOT fail boot. Rotation skips the now-unknown-KEK row with a warning and the
// process starts. But the secret is sealed: any runtime unwrap hard-errors,
// because the KEK that wrapped the DEK is gone from config.
//
// This is the single most dangerous rotation mistake, and this test is its
// regression anchor: if someone ever makes boot fail-fast on skipped>0, or
// silences the runtime unwrap error, one of the two halves below breaks.
func TestTwoPhase_RollbackPastA_BootWarnsButUnwrapFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kek1, kek2 := uniqueKEKURI("kek-1"), uniqueKEKURI("kek-2")

	// Reach the post-phase-B state: the DEK is wrapped under kek-2.
	old := keyringWith(t, kek1, kek1)
	row, _ := seedDEK(ctx, t, old, "rollbackPastA")
	phaseB := keyringWith(t, kek2, kek1, kek2)
	_, _, err := NewRotator(txManager, store, phaseB).Rotate(ctx)
	require.NoError(t, err)
	require.Equal(t, kek2, findByID(ctx, t, row.ID).KEKID)

	before := findByID(ctx, t, row.ID)

	// Catastrophic rollback: a config that only ever knew kek-1. kek-2 is gone.
	preA := keyringWith(t, kek1, kek1)

	// Boot half: rotation must NOT fail. The kek-2 row is unknown to this keyring,
	// so it is skipped (counted in skipped), and the process boots.
	n, skipped, err := NewRotator(txManager, store, preA).Rotate(ctx)
	require.NoError(t, err, "boot must not fail on a row under a now-unknown KEK")
	require.Equal(t, 0, n, "nothing is re-wrapped: the kek-2 row cannot be unwrapped")
	require.GreaterOrEqual(t, skipped, 1, "the kek-2 row is skipped (unknown KEK)")

	// The skipped row is left byte-for-byte untouched — its envelope is not
	// corrupted, it is simply unreadable without kek-2.
	after := findByID(ctx, t, row.ID)
	require.Equal(t, before.KEKID, after.KEKID)
	require.Equal(t, before.EncryptedDEK, after.EncryptedDEK)

	// Runtime half: any attempt to actually USE the secret hard-errors, because
	// the KEK that sealed the DEK is no longer in config. This is the loud failure
	// the design promises instead of silent corruption.
	_, err = preA.UnwrapDEK(after.EncryptedDEK, after.KEKID)
	require.Error(t, err, "unwrapping a DEK under a dropped KEK must hard-error at runtime")
	require.ErrorContains(t, err, "unknown kek")
}
