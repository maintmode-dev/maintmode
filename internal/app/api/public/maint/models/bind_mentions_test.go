package apimodels

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestUpdateDraftMaintRequestMentionsWire pins the JSON decoding half of the
// tri-state contract, which the DTO->cmd mapping depends on. Absent and null
// must both arrive as nil ("unchanged") and an empty array as a non-nil empty
// slice ("clear"). null is asserted explicitly because it reaches nil by a
// different mechanism than an absent field: an absent field leaves the
// destination untouched, whereas null actively resets it. They coincide only
// because binding always decodes into a zero-valued request struct — so the
// field is seeded with a non-nil value here to prove null really clears it
// rather than merely being skipped.
func TestUpdateDraftMaintRequestMentionsWire(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	for _, tc := range []struct {
		name        string
		body        string
		seeded      bool
		expectNil   bool
		expectedLen int
	}{
		{
			name:      "absent field means unchanged",
			body:      `{"title":"t"}`,
			expectNil: true,
		}, {
			name:      "null means unchanged",
			body:      `{"title":"t","mentions":null}`,
			seeded:    true,
			expectNil: true,
		}, {
			name:        "empty array means clear",
			body:        `{"title":"t","mentions":[]}`,
			expectedLen: 0,
		}, {
			name:        "non-empty array means replace",
			body:        `{"title":"t","mentions":[{"user_id":"` + userID.String() + `"}]}`,
			expectedLen: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := new(UpdateDraftMaintRequest)
			if tc.seeded {
				req.Mentions = lo.ToPtr([]*Mention{{UserID: userID}})
			}

			require.NoError(t, json.Unmarshal([]byte(tc.body), req))

			if tc.expectNil {
				require.Nil(t, req.Mentions, "must stay nil so the service leaves mentions alone")
				return
			}

			require.NotNil(t, req.Mentions,
				"a supplied array must survive as a non-nil pointer, or clearing turns into a no-op")
			require.Len(t, lo.FromPtr(req.Mentions), tc.expectedLen)
		})
	}
}

// TestToAPIMentionViews pins the read shape: mentions are named from the
// resolved summaries, an unresolved id still appears (labeled), and the response
// carries no messenger-tag information.
func TestToAPIMentionViews(t *testing.T) {
	t.Parallel()

	named := uuid.New()
	unresolved := uuid.New()

	got := ToAPIMentionViews(
		[]uuid.UUID{named, unresolved},
		map[uuid.UUID]*entity.UserSummary{named: {ID: named, Name: "Ivan Petrov"}},
	)

	require.Len(t, got, 2)
	require.Equal(t, &MentionView{UserID: named, DisplayName: "Ivan Petrov"}, got[0])
	require.Equal(t, &MentionView{UserID: unresolved, DisplayName: entity.UnknownUserName}, got[1],
		"an unresolved mention must still be listed, not dropped")
}

// TestMentionViewCarriesNoMessengerTag is a negative assertion on the serialized
// shape: the detail view names who was mentioned, never whether they have a
// messenger configured. This endpoint is readable by guests, so the flag must
// not leak here even though the picker exposes it to editors.
func TestMentionViewCarriesNoMessengerTag(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(&MentionView{UserID: uuid.New(), DisplayName: "Ivan Petrov"})
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))

	require.NotContains(t, decoded, "has_messenger_tag")
	require.NotContains(t, decoded, "telegram_tag")
	require.NotContains(t, decoded, "slack_tag")
	require.Len(t, decoded, 2, "mention view must expose exactly user_id and display_name")
}
