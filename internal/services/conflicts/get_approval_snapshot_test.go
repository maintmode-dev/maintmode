package conflicts

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// The deliberate asymmetry in how a snapshot read failure is handled, pinned
// from the side that must NOT degrade.
//
// MarkApprovalState swallows the same failure on the live and factual branches,
// and that is right there: the conflict set is the content, the snapshot only
// annotates it, so losing the annotation beats losing the card. On the snapshot
// branch the snapshot IS the content — degrading would render a maintenance
// that had known conflicts as one that had none, which on an incident-review
// screen is the worst possible failure: a confident, wrong, clean slate.
//
// Without this test, changing the error return to `nil, nil` passes everything
// else in the suite.
func TestGetApprovalSnapshotPropagatesReadFailure(t *testing.T) {
	t.Parallel()

	s, mocks := initService(t)
	readErr := errors.New("snapshot table unreadable")

	mocks.snapshots.EXPECT().
		GetSnapshots(gomock.Any(), maintID).
		Return(nil, readErr)

	snapshot, err := s.GetApprovalSnapshot(context.Background(), maintID)

	require.ErrorIs(t, err, readErr,
		"the snapshot is this branch's entire content, so a read failure must "+
			"surface rather than render an empty conflict list")
	require.Nil(t, snapshot)
}
