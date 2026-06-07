package notifychannel

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// TestStore_Create_RecordsAuthor verifies the created_by_user_id column round-trips
// through Create.
func TestStore_Create_RecordsAuthor(t *testing.T) {
	ctx := context.Background()

	authorID := uuid.New()
	channel, err := store.Create(ctx, &entity.NotifyChannel{
		Transport:          entity.NotifyTransportSlack,
		TransportChannelID: t.Name() + "-" + xuuid.NewString(),
		Name:               t.Name(),
		Description:        "test channel",
		CreatedByUserID:    &authorID,
	})
	require.NoError(t, err)
	require.NotNil(t, channel.CreatedByUserID)
	require.Equal(t, authorID, *channel.CreatedByUserID)
	require.Nil(t, channel.UpdatedByUserID, "freshly created channel has no editor")
	require.Nil(t, channel.UpdatedAt, "freshly created channel has no updated_at")
}

// TestStore_Update_EditableFields verifies a partial-style merged update writes
// name/description/transport_channel_id and stamps updated_at + updated_by,
// while leaving the author untouched.
func TestStore_Update_EditableFields(t *testing.T) {
	ctx := context.Background()

	authorID := uuid.New()
	created, err := store.Create(ctx, &entity.NotifyChannel{
		Transport:          entity.NotifyTransportTelegram,
		TransportChannelID: t.Name() + "-" + xuuid.NewString(),
		Name:               "before",
		Description:        "before",
		CreatedByUserID:    &authorID,
	})
	require.NoError(t, err)

	editorID := uuid.New()
	created.Name = "after"
	created.Description = "after desc"
	created.TransportChannelID = t.Name() + "-after-" + xuuid.NewString()
	created.UpdatedByUserID = &editorID

	updated, err := store.Update(ctx, created)
	require.NoError(t, err)
	require.Equal(t, "after", updated.Name)
	require.Equal(t, "after desc", updated.Description)
	require.Equal(t, created.TransportChannelID, updated.TransportChannelID)
	require.NotNil(t, updated.UpdatedAt, "update must stamp updated_at")
	require.NotNil(t, updated.UpdatedByUserID)
	require.Equal(t, editorID, *updated.UpdatedByUserID)
	require.Equal(t, entity.NotifyTransportTelegram, updated.Transport, "transport is immutable")
	require.NotNil(t, updated.CreatedByUserID)
	require.Equal(t, authorID, *updated.CreatedByUserID, "author must be preserved on update")

	// Persisted: a subsequent Get reflects the edit.
	got, err := store.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "after", got.Name)
	require.NotNil(t, got.UpdatedByUserID)
	require.Equal(t, editorID, *got.UpdatedByUserID)
}

// TestStore_Update_UnknownIsNotFound verifies updating a non-existent channel
// surfaces the not-found domain error (RETURNING yields no row).
func TestStore_Update_UnknownIsNotFound(t *testing.T) {
	ctx := context.Background()

	_, err := store.Update(ctx, &entity.NotifyChannel{
		ID:                 uuid.New(),
		Transport:          entity.NotifyTransportSlack,
		TransportChannelID: t.Name() + "-" + xuuid.NewString(),
		Name:               "x",
		Description:        "x",
		UpdatedByUserID:    lo.ToPtr(uuid.New()),
	})
	require.ErrorIs(t, err, apperr.ErrNotifyChannelNotFound)
}

// TestStore_Update_DuplicateTransportChannelIsConflict verifies the unique
// (transport, transport_channel_id) constraint surfaces as a conflict when an
// update would collide with another channel.
func TestStore_Update_DuplicateTransportChannelIsConflict(t *testing.T) {
	ctx := context.Background()

	existing := makeChannel(ctx, t, entity.NotifyTransportSlack)
	victim, err := store.Create(ctx, &entity.NotifyChannel{
		Transport:          entity.NotifyTransportSlack,
		TransportChannelID: t.Name() + "-victim-" + xuuid.NewString(),
		Name:               "victim",
		Description:        "victim",
	})
	require.NoError(t, err)

	// Point victim at existing's transport_channel_id → unique violation.
	victim.TransportChannelID = existing.TransportChannelID
	victim.UpdatedByUserID = lo.ToPtr(uuid.New())

	_, err = store.Update(ctx, victim)
	require.ErrorIs(t, err, apperr.ErrNotifyChannelAlreadyExists)
}
