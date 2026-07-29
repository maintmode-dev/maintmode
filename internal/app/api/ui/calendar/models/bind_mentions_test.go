package uimodels

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
)

// TestToAPIMaintenanceView_Mentions pins the mention mapping of the UI
// read-view: ids keep their order, names come from the resolved summaries, and
// an unresolved id still appears under the resolver's label instead of being
// dropped.
func TestToAPIMaintenanceView_Mentions(t *testing.T) {
	t.Parallel()

	named := uuid.New()
	unresolved := uuid.New()

	got := ToAPIMaintenanceView(
		&calendardto.Maintenance{Mentions: []uuid.UUID{named, unresolved}},
		map[uuid.UUID]*entity.UserSummary{named: {ID: named, Name: "Ivan Petrov"}},
	)

	require.Equal(t, []*MentionView{
		{UserID: named, DisplayName: "Ivan Petrov"},
		{UserID: unresolved, DisplayName: entity.UnknownUserName},
	}, got.Mentions, "mentions must keep their order and never drop an unresolved user")
}

// TestToAPIMaintenanceView_MentionsEmpty pins the contract the FE relies on: an
// empty mention list serializes as [], never null.
func TestToAPIMaintenanceView_MentionsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mentions []uuid.UUID
	}{
		{name: "nil slice", mentions: nil},
		{name: "empty slice", mentions: []uuid.UUID{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ToAPIMaintenanceView(&calendardto.Maintenance{Mentions: tt.mentions}, nil)

			require.NotNil(t, got.Mentions)
			require.Empty(t, got.Mentions)

			raw, err := json.Marshal(got)
			require.NoError(t, err)
			require.Contains(t, string(raw), `"mentions":[]`,
				"an empty mention list must serialize as [], not null")
		})
	}
}

// TestMaintenanceViewMentionsCarryNoMessengerTag is a negative assertion on the
// serialized shape: the card names who was mentioned, never whether they have a
// messenger configured. This view is readable by guests, so the flag and the tag
// values must not leak here even though the picker exposes the flag to editors.
func TestMaintenanceViewMentionsCarryNoMessengerTag(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	got := ToAPIMaintenanceView(
		&calendardto.Maintenance{Mentions: []uuid.UUID{userID}},
		map[uuid.UUID]*entity.UserSummary{userID: {ID: userID, Name: "Ivan Petrov"}},
	)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "has_messenger_tag")
	require.NotContains(t, string(raw), "telegram_tag")
	require.NotContains(t, string(raw), "slack_tag")

	var decoded struct {
		Mentions []map[string]any `json:"mentions"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Len(t, decoded.Mentions, 1)
	require.Len(t, decoded.Mentions[0], 2,
		"a mention must expose exactly user_id and display_name")
}
