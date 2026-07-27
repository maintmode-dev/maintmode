package apimodels

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// TestUserSummaryOmitsMessengerTags pins the privacy boundary around messenger
// handles: they are readable by admins through the users listing, and by the
// notifier when it renders a mention, but they must never ride along in the
// author/approver summary embedded in ordinary maintenance reads.
//
// The guarantee currently holds by construction — both mappers name their fields
// explicitly rather than embedding the domain type — which is exactly why it
// needs a test. An innocent-looking refactor to struct embedding, or a future
// field added to the API UserSummary, would leak the handles to every user who
// can read a maintenance, and nothing else in the suite would notice.
func TestUserSummaryOmitsMessengerTags(t *testing.T) {
	t.Parallel()

	const (
		telegramTag = "@ruslan"
		slackTag    = "ruslan.k"
	)

	t.Run("from domain user", func(t *testing.T) {
		t.Parallel()

		got := ToAPIUserSummary(&entity.User{
			ID:          xuuid.New(),
			Name:        "Ruslan Kosykh",
			Email:       "ruslan@example.com",
			TelegramTag: lo.ToPtr(telegramTag),
			SlackTag:    lo.ToPtr(slackTag),
		})

		requireNoMessengerTags(t, got, telegramTag, slackTag)
	})

	t.Run("from resolved summary", func(t *testing.T) {
		t.Parallel()

		// entity.UserSummary carries no tags at all — asserting here guards the
		// other direction: that the type is not extended with them later.
		got := ToAPIUserSummaryFromSummary(&entity.UserSummary{
			ID:    xuuid.New(),
			Name:  "Ruslan Kosykh",
			Email: "ruslan@example.com",
		})

		requireNoMessengerTags(t, got, telegramTag, slackTag)
	})
}

// requireNoMessengerTags asserts the serialized summary carries neither handle,
// checking the JSON rather than the struct so a field added with any name is
// still caught.
func requireNoMessengerTags(t *testing.T, summary *UserSummary, tags ...string) {
	t.Helper()

	require.NotNil(t, summary)
	require.Equal(t, "Ruslan Kosykh", summary.DisplayName, "the summary should still carry the name")

	raw, err := json.Marshal(summary)
	require.NoError(t, err)

	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))

	require.NotContains(t, fields, "telegram_tag")
	require.NotContains(t, fields, "slack_tag")

	for _, tag := range tags {
		require.NotContains(t, string(raw), tag,
			"messenger handle %q leaked into the maintenance author summary", tag)
	}
}
