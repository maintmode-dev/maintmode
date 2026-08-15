package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// The approve gate answers one question: did the thing the approver agreed to
// change? A neighbor's resource that our maintenance does not touch is not part
// of that thing — the two maintenances still collide over exactly the same
// resources, in the same window, as when the card was read.
//
// The read side deliberately reports the neighbor's FULL resource set, because a
// reader wants to know what that neighbor touches. Feeding that same set into the
// gate would make every unrelated edit anywhere in the window invalidate a
// pending approval. The fingerprint therefore intersects first: it hashes the
// overlap, which is what "the conflict" actually means.
//
// This mirrors the rule already stated on MarkKnownAtApproval — a neighbor whose
// resource set shifted is still the same neighbor — so the two mechanisms now
// agree instead of contradicting each other.
func conflictAt(id uuid.UUID, resources ...uuid.UUID) *ConflictWithResources {
	return &ConflictWithResources{
		Conflict: &Conflict{
			MaintenanceID: id,
			Title:         "neighbor",
			OverlapStart:  time.Date(2124, 1, 1, 10, 0, 0, 0, time.UTC),
			OverlapEnd:    time.Date(2124, 1, 1, 12, 0, 0, 0, time.UTC),
			Scope:         MaintenanceScopeResources,
		},
		Resources: resources,
	}
}

var neighborID = uuid.MustParse("40000000-0000-0000-0000-000000000004")

func TestConflictFingerprintFor_IgnoresResourcesWeDoNotHold(t *testing.T) {
	t.Parallel()

	own := []uuid.UUID{resID1}

	// The neighbor gains resID3, which our maintenance does not hold. The overlap
	// is still exactly {resID1}, so the approval the operator previewed is still
	// the approval they are giving.
	before := ConflictFingerprintFor([]*ConflictWithResources{
		conflictAt(neighborID, resID1, resID2),
	}, own)

	after := ConflictFingerprintFor([]*ConflictWithResources{
		conflictAt(neighborID, resID1, resID2, resID3),
	}, own)

	require.Equal(t, before, after,
		"a neighbor gaining a resource we do not hold must not invalidate the preview")
}

func TestConflictFingerprintFor_CatchesOverlapChanges(t *testing.T) {
	t.Parallel()

	own := []uuid.UUID{resID1, resID2}

	base := ConflictFingerprintFor([]*ConflictWithResources{
		conflictAt(neighborID, resID1, resID2),
	}, own)

	t.Run("shared resource dropped", func(t *testing.T) {
		t.Parallel()

		// The neighbor stops touching resID2. The collision genuinely shrank, so
		// the approver is now agreeing to something else.
		got := ConflictFingerprintFor([]*ConflictWithResources{
			conflictAt(neighborID, resID1),
		}, own)

		require.NotEqual(t, base, got,
			"losing a shared resource changes the conflict and must be caught")
	})

	t.Run("shared resource gained", func(t *testing.T) {
		t.Parallel()

		// Our maintenance holds resID3 too, and the neighbor now takes it: the
		// overlap grew, which is a bigger blast radius than was reviewed.
		ownWider := []uuid.UUID{resID1, resID2, resID3}

		narrow := ConflictFingerprintFor([]*ConflictWithResources{
			conflictAt(neighborID, resID1, resID2),
		}, ownWider)
		wider := ConflictFingerprintFor([]*ConflictWithResources{
			conflictAt(neighborID, resID1, resID2, resID3),
		}, ownWider)

		require.NotEqual(t, narrow, wider,
			"gaining a resource we DO hold widens the overlap and must be caught")
	})

	t.Run("neighbor disappears", func(t *testing.T) {
		t.Parallel()

		got := ConflictFingerprintFor(nil, own)
		require.NotEqual(t, base, got, "a vanished conflict must be caught")
	})
}

// A global-scope maintenance holds no resources, so it shares none with anyone.
// Every conflict's overlap is empty, and the fingerprint must rest on the set of
// neighbors and their windows alone — not silently collapse to one value for any
// two different neighbor sets.
func TestConflictFingerprintFor_GlobalSubjectStillDistinguishesNeighbors(t *testing.T) {
	t.Parallel()

	otherID := uuid.MustParse("50000000-0000-0000-0000-000000000005")

	one := ConflictFingerprintFor([]*ConflictWithResources{
		conflictAt(neighborID, resID1),
	}, nil)
	other := ConflictFingerprintFor([]*ConflictWithResources{
		conflictAt(otherID, resID1),
	}, nil)
	both := ConflictFingerprintFor([]*ConflictWithResources{
		conflictAt(neighborID, resID1),
		conflictAt(otherID, resID1),
	}, nil)

	require.NotEqual(t, one, other, "different neighbors must fingerprint differently")
	require.NotEqual(t, one, both, "an added neighbor must be caught")
}

// The fingerprint must not corrupt what it is given: the same slices are the live
// read-path result and the client's snapshot, and both are used after the gate
// runs (SaveSnapshot persists the snapshot verbatim).
func TestConflictFingerprintFor_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	conflict := conflictAt(neighborID, resID3, resID1, resID2)
	original := append([]uuid.UUID(nil), conflict.Resources...)

	ConflictFingerprintFor([]*ConflictWithResources{conflict}, []uuid.UUID{resID1})

	require.Equal(t, original, conflict.Resources,
		"the conflict's own resource slice must survive fingerprinting unchanged")
}
