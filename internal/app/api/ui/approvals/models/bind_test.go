package approvalsmodels

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
)

// TestToAPIApprovalRow_UpdatedAt pins the nullability the screen reads as "never
// touched since it was created": a maintenance with no updated_at must not have
// created_at echoed back into that slot.
func TestToAPIApprovalRow_UpdatedAt(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 7, 20, 9, 15, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 7, 28, 14, 2, 0, 0, time.UTC)

	tests := []struct {
		name      string
		updatedAt *time.Time
		wantJSON  string
	}{
		{name: "never updated", updatedAt: nil, wantJSON: `null`},
		{name: "updated", updatedAt: &updatedAt, wantJSON: `"2026-07-28T14:02:00Z"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ToAPIApprovalRow(&calendardto.Maintenance{
				CreatedAt: createdAt,
				UpdatedAt: tt.updatedAt,
			}, nil)

			require.Equal(t, tt.updatedAt, got.UpdatedAt)

			raw, err := json.Marshal(got)
			require.NoError(t, err)
			require.JSONEq(t, tt.wantJSON, gjsonField(t, raw, "updated_at"))
		})
	}
}

// TestToAPIApprovalRows_EmptyIsNotNull pins the most common response in
// production. Checked on the raw JSON, not on the Go slice: null would break
// .map() on the FE, and only the serialized form proves it does not happen.
func TestToAPIApprovalRows_EmptyIsNotNull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		maints []*calendardto.Maintenance
	}{
		{name: "nil slice", maints: nil},
		{name: "empty slice", maints: []*calendardto.Maintenance{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rows := ToAPIApprovalRows(tt.maints, nil)
			require.NotNil(t, rows)

			raw, err := json.Marshal(&ListApprovalsResponse{Maintenances: rows})
			require.NoError(t, err)
			require.Contains(t, string(raw), `"maintenances":[]`)
		})
	}
}

// TestToAPIApprovalRow_Author covers both degradation steps of the authorship
// resolve: no summary at all serializes as null, and a summary the resolver
// could not fill in keeps its id under the "Unknown user" label.
func TestToAPIApprovalRow_Author(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()

	t.Run("no author resolves to null", func(t *testing.T) {
		t.Parallel()

		got := ToAPIApprovalRow(&calendardto.Maintenance{CreatedByUserID: authorID}, nil)
		require.Nil(t, got.CreatedBy)

		raw, err := json.Marshal(got)
		require.NoError(t, err)
		require.JSONEq(t, `null`, gjsonField(t, raw, "created_by"))
	})

	t.Run("degraded author keeps the unknown-user label", func(t *testing.T) {
		t.Parallel()

		got := ToAPIApprovalRows(
			[]*calendardto.Maintenance{{CreatedByUserID: authorID}},
			map[uuid.UUID]*entity.UserSummary{authorID: entity.NewUnknownUserSummary(authorID)},
		)

		require.Len(t, got, 1)
		require.Equal(t, &UserSummary{ID: authorID, DisplayName: entity.UnknownUserName}, got[0].CreatedBy)
	})
}

// TestToAPIApprovalRow_OpenEndedPeriod pins the documented behavior: an
// open-ended maintenance yields zero-time, not null, exactly like the calendar
// event. It is written down so a well-meant "fix" to null cannot pass silently —
// the FE reads zero-time as "no end set".
func TestToAPIApprovalRow_OpenEndedPeriod(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	got := ToAPIApprovalRow(&calendardto.Maintenance{
		PlannedPeriod: entity.NewOpenEndedPeriod(start),
	}, nil)

	require.Equal(t, start, got.Start)
	require.True(t, got.End.IsZero(), "an open-ended period must map to zero-time, not null")

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, `"0001-01-01T00:00:00Z"`, gjsonField(t, raw, "end"))
}

// TestToAPIApprovalRow_CarriesRow pins the plain field mapping of a fully
// populated row so a silently dropped column shows up here.
func TestToAPIApprovalRow_CarriesRow(t *testing.T) {
	t.Parallel()

	var (
		maintID   = uuid.New()
		authorID  = uuid.New()
		start     = time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
		end       = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
		createdAt = time.Date(2026, 7, 20, 9, 15, 0, 0, time.UTC)
	)

	got := ToAPIApprovalRow(&calendardto.Maintenance{
		ID:              maintID,
		Title:           "Cluster upgrade",
		PlannedPeriod:   entity.NewPeriod(start, end),
		Scope:           entity.MaintenanceScopeResources,
		Impact:          entity.MaintenanceImpactPartial,
		CreatedAt:       createdAt,
		CreatedByUserID: authorID,
	}, &entity.UserSummary{ID: authorID, Name: "Ann", Email: "ann@example.com"})

	require.Equal(t, &ApprovalRow{
		ID:        maintID,
		Title:     "Cluster upgrade",
		Start:     start,
		End:       end,
		Scope:     "resource",
		Impact:    "partial_outage",
		CreatedBy: &UserSummary{ID: authorID, DisplayName: "Ann", Email: "ann@example.com"},
		CreatedAt: createdAt,
	}, got)
}

// TestApprovalUserIDs pins the input of the handler's single ResolveMany call:
// author ids in page order, and nothing to resolve for an empty page.
func TestApprovalUserIDs(t *testing.T) {
	t.Parallel()

	first, second := uuid.New(), uuid.New()

	require.Equal(t, []uuid.UUID{first, second}, ApprovalUserIDs([]*calendardto.Maintenance{
		{CreatedByUserID: first},
		{CreatedByUserID: second},
	}), "ids must keep page order — ResolveMany results are matched back by id")
	require.Empty(t, ApprovalUserIDs(nil))
}

// gjsonField pulls one field out of a marshaled row so the assertions can be
// made against the wire form rather than the Go value.
func gjsonField(t *testing.T, raw []byte, field string) string {
	t.Helper()

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))

	value, ok := fields[field]
	require.Truef(t, ok, "field %q missing from the serialized row", field)

	return string(value)
}
