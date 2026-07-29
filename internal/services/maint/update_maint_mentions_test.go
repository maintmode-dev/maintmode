package maint

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// makeUserIDs returns n distinct mentionable user ids. Ids are uniquified per
// run because the suite runs -count 2 against a shared database.
func makeUserIDs(n int) []uuid.UUID {
	ids := make([]uuid.UUID, 0, n)
	for range n {
		ids = append(ids, xuuid.New())
	}

	return ids
}

// mentionInputs wraps ids into the command shape the service takes.
func mentionInputs(ids ...uuid.UUID) []*entity.MentionInput {
	return lo.Map(ids, func(id uuid.UUID, _ int) *entity.MentionInput {
		return &entity.MentionInput{UserID: id}
	})
}

// TestUpdateMaintMentionsTriState pins the clearing contract: a nil set leaves
// mentions alone, an empty set clears them, and a non-empty set replaces them.
// Assertions read the persisted rows back rather than trusting a "replace was
// called" signal — the empty case must actually delete the previous mentions,
// not rewrite them.
func TestUpdateMaintMentionsTriState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	service, mocks := initService(t)
	// Every id in these tests is a valid, unblocked user; eligibility is not
	// what this test is about.
	mocks.expectAnyApproverEligible()

	// seedMaint creates a draft that already carries two mentions.
	seedMaint := func(t *testing.T) (*entity.Maintenance, []uuid.UUID) {
		t.Helper()

		start := xtime.UTCNow().Add(uniqueFutureOffset())
		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
			entity.NewPeriod(start, start.Add(time.Hour)))

		existing := makeUserIDs(2)
		require.NoError(t, service.maintStore.AddMentions(ctx, maint.ID, existing))

		return maint, existing
	}

	t.Run("nil leaves mentions unchanged", func(t *testing.T) {
		t.Parallel()

		maint, existing := seedMaint(t)

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:  maint.ID,
			Title:    lo.ToPtr("Updated title"),
			Mentions: nil,
			Actor:    actor(),
		})
		require.NoError(t, err)

		got, err := service.maintStore.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, existing, got, "a nil set must not touch the existing mentions")
	})

	t.Run("empty set clears mentions", func(t *testing.T) {
		t.Parallel()

		maint, _ := seedMaint(t)

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:  maint.ID,
			Title:    lo.ToPtr("Updated title"),
			Mentions: lo.ToPtr([]*entity.MentionInput{}),
			Actor:    actor(),
		})
		require.NoError(t, err)

		got, err := service.maintStore.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Empty(t, got, "an empty set must delete the mentions, not rewrite them")
	})

	t.Run("non-empty set replaces mentions", func(t *testing.T) {
		t.Parallel()

		maint, existing := seedMaint(t)
		replacement := makeUserIDs(2)

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:  maint.ID,
			Title:    lo.ToPtr("Updated title"),
			Mentions: lo.ToPtr(mentionInputs(replacement...)),
			Actor:    actor(),
		})
		require.NoError(t, err)

		got, err := service.maintStore.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, replacement, got)
		require.NotContains(t, got, existing[0], "mentions should be replaced, not kept")
	})

	// Pins the staging gate on its own. The replace block persists whatever this
	// function stages, so if the two gates disagree an empty set would carry the
	// previous mentions into the replace and rewrite them instead of clearing.
	// The end-to-end assertions above cannot see that today only because
	// GetMaintForUpdate never hydrates this field; this test does not depend on
	// that.
	t.Run("staging gate mirrors the replace gate", func(t *testing.T) {
		t.Parallel()

		existing := makeUserIDs(1)

		t.Run("empty set stages an empty collection", func(t *testing.T) {
			t.Parallel()

			maint := &entity.Maintenance{Mentions: existing}
			applyValuesFromUpdateCmd(maint, nil, &entity.UpdateMaintenanceCmd{
				Mentions: lo.ToPtr([]*entity.MentionInput{}),
			})

			require.Empty(t, maint.Mentions)
		})

		t.Run("nil keeps the existing collection", func(t *testing.T) {
			t.Parallel()

			maint := &entity.Maintenance{Mentions: existing}
			applyValuesFromUpdateCmd(maint, nil, &entity.UpdateMaintenanceCmd{
				Mentions: nil,
			})

			require.Len(t, maint.Mentions, 1)
		})

		t.Run("non-empty set stages the supplied ids", func(t *testing.T) {
			t.Parallel()

			replacement := makeUserIDs(2)
			maint := &entity.Maintenance{Mentions: existing}
			applyValuesFromUpdateCmd(maint, nil, &entity.UpdateMaintenanceCmd{
				Mentions: lo.ToPtr(mentionInputs(replacement...)),
			})

			require.Equal(t, replacement, maint.Mentions)
		})
	})

	t.Run("replacement is audited", func(t *testing.T) {
		t.Parallel()

		maint, _ := seedMaint(t)
		before := len(mocks.audit.all())

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:  maint.ID,
			Mentions: lo.ToPtr(mentionInputs(xuuid.New())),
			Actor:    actor(),
		})
		require.NoError(t, err)

		require.True(t, auditedMentionChange(mocks.audit.all()[before:], maint.ID),
			"replacing mentions must record an audit change")
	})

	t.Run("clearing is audited", func(t *testing.T) {
		t.Parallel()

		maint, _ := seedMaint(t)
		before := len(mocks.audit.all())

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:  maint.ID,
			Mentions: lo.ToPtr([]*entity.MentionInput{}),
			Actor:    actor(),
		})
		require.NoError(t, err)

		// Clearing hard-deletes rows, so the audit entry is the only trace.
		require.True(t, auditedMentionChange(mocks.audit.all()[before:], maint.ID),
			"clearing mentions must record an audit change")
	})

	// The negative half of the audit gate: an edit that did not supply mentions
	// must not claim it changed them, or every unrelated draft edit would read as
	// a mention change in the audit trail.
	t.Run("nil is not audited as a mention change", func(t *testing.T) {
		t.Parallel()

		maint, _ := seedMaint(t)
		before := len(mocks.audit.all())

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:  maint.ID,
			Title:    lo.ToPtr("Updated title without mentions"),
			Mentions: nil,
			Actor:    actor(),
		})
		require.NoError(t, err)

		require.False(t, auditedMentionChange(mocks.audit.all()[before:], maint.ID),
			"an edit that left mentions alone must not record a mention change")
	})
}

// TestUpdateMaintValidatesMentions covers gates 6 and 7 — the size cap and the
// per-element rules on the update path. The create path has its own tests, but
// these are separate call sites: deleting the whole mention validation block from
// validateUpdate left every other test in the suite green, because the tri-state
// tests only ever supply well-formed input and the API layer checks neither
// the nil uuid, so a malformed element would otherwise reach the store.
func TestUpdateMaintValidatesMentions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	service, mocks := initService(t)
	mocks.expectAnyApproverEligible()

	seedMaint := func(t *testing.T) *entity.Maintenance {
		t.Helper()

		start := xtime.UTCNow().Add(uniqueFutureOffset())

		return testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
			entity.NewPeriod(start, start.Add(time.Hour)))
	}

	tests := []struct {
		name     string
		mentions []*entity.MentionInput
	}{
		{
			name:     "rejects the nil user id",
			mentions: mentionInputs(uuid.Nil),
		},
		{
			name:     "rejects one over the maximum",
			mentions: mentionInputs(makeUserIDs(maxMentions + 1)...),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			maint := seedMaint(t)

			err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
				MaintID:  maint.ID,
				Mentions: lo.ToPtr(tt.mentions),
				Actor:    actor(),
			})
			require.Error(t, err, "invalid mentions must be rejected on update, not persisted")

			// The rejection must happen before anything is written.
			got, storeErr := service.maintStore.GetMaintMentions(ctx, maint.ID)
			require.NoError(t, storeErr)
			require.Empty(t, got, "a rejected update must not persist any mention")
		})
	}

	t.Run("accepts exactly the maximum", func(t *testing.T) {
		t.Parallel()

		maint := seedMaint(t)
		mentioned := makeUserIDs(maxMentions)

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:  maint.ID,
			Mentions: lo.ToPtr(mentionInputs(mentioned...)),
			Actor:    actor(),
		})
		require.NoError(t, err, "the cap is inclusive: exactly maxMentions must be accepted")

		got, err := service.maintStore.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, mentioned, got)
	})
}

// auditedMentionChange reports whether the actions contain a maintenance-updated
// entry for maintID carrying a "mentions" field change.
func auditedMentionChange(actions []audit.Action, maintID uuid.UUID) bool {
	for _, action := range actions {
		updated, ok := action.(audit.MaintUpdated)
		if !ok || updated.Maint.ID != maintID {
			continue
		}
		for _, change := range updated.Changes {
			if change.Field == "mentions" {
				return true
			}
		}
	}

	return false
}
