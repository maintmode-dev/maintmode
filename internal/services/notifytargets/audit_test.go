package notifytargets

import (
	"context"
	"sync"
	"testing"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	notifychannelstore "github.com/ruko1202/maintmode/internal/storages/notifychannel"
	notifytargetsstore "github.com/ruko1202/maintmode/internal/storages/notifytargets"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// recordingAuditPublisher records published actions instead of enqueuing them.
type recordingAuditPublisher struct {
	mu      sync.Mutex
	actions []audit.Action
}

func (p *recordingAuditPublisher) Publish(_ context.Context, action audit.Action) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.actions = append(p.actions, action)

	return nil
}

func (p *recordingAuditPublisher) published() []audit.Action {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]audit.Action(nil), p.actions...)
}

// Every channel-catalog mutation is recorded with its actor; the idempotent
// repeats of archive / unarchive change nothing and record nothing.
func TestChannelCatalog_PublishesAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	rec := &recordingAuditPublisher{}
	service := NewService(dbtx.NewTxManager(db), notifychannelstore.NewStore(db), notifytargetsstore.NewStore(db), rec)
	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}

	channel, err := service.CreateChannel(ctx, &entity.CreateNotifyChannelCmd{
		Transport:          entity.NotifyTransportSlack,
		TransportChannelID: "C-" + xuuid.NewString(),
		Name:               "before",
		Description:        "audit test",
		Actor:              actor,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := table.MessengerChannels.DELETE().
			WHERE(table.MessengerChannels.ID.EQ(postgres.UUID(channel.ID))).
			ExecContext(context.Background(), db)
		require.NoError(t, err)
	})

	_, err = service.UpdateChannel(ctx, &entity.UpdateNotifyChannelCmd{
		ID:    channel.ID,
		Name:  lo.ToPtr("after"),
		Actor: actor,
	})
	require.NoError(t, err)

	require.NoError(t, service.ArchiveChannel(ctx, actor, channel.ID))
	require.NoError(t, service.ArchiveChannel(ctx, actor, channel.ID))
	require.NoError(t, service.UnarchiveChannel(ctx, actor, channel.ID))
	require.NoError(t, service.UnarchiveChannel(ctx, actor, channel.ID))
	require.NoError(t, service.ArchiveChannel(ctx, actor, uuid.New()), "an unknown id stays a no-op success")

	published := rec.published()
	require.Len(t, published, 4, "create, update, archive, unarchive -- and nothing for the repeats")

	created, ok := published[0].(audit.NotifyChannelCreated)
	require.True(t, ok, "got %T", published[0])
	require.Equal(t, actor, created.Actor)
	require.Equal(t, channel.ID, created.Channel.ID)

	updated, ok := published[1].(audit.NotifyChannelUpdated)
	require.True(t, ok, "got %T", published[1])
	require.Equal(t, actor, updated.Actor)
	require.Equal(t, "after", updated.Channel.Name)
	require.Equal(t, []entity.AuditFieldChange{{Field: "name", Old: "before", New: "after"}}, updated.Changes)

	archived, ok := published[2].(audit.NotifyChannelArchived)
	require.True(t, ok, "got %T", published[2])
	require.Equal(t, channel.ID, archived.Channel.ID)
	require.Equal(t, "after", archived.Channel.Name)

	unarchived, ok := published[3].(audit.NotifyChannelUnarchived)
	require.True(t, ok, "got %T", published[3])
	require.Equal(t, channel.ID, unarchived.Channel.ID)

	// The state transitions themselves still happened.
	got, err := service.GetChannel(ctx, channel.ID)
	require.NoError(t, err)
	require.Nil(t, got.ArchivedAt)
}
