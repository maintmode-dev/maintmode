package notifychannel

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db    *sqlx.DB
	store *Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	store = NewStore(db)

	code := m.Run()
	os.Exit(code)
}

// makeChannel inserts a uniquely-named channel so parallel test runs do not
// collide on the (transport, transport_channel_id) unique index.
func makeChannel(ctx context.Context, t *testing.T, transport entity.NotifyTransport) *entity.NotifyChannel {
	t.Helper()

	channel, err := store.Create(ctx, &entity.NotifyChannel{
		Transport:          transport,
		TransportChannelID: t.Name() + "-" + xuuid.NewString(),
		Name:               t.Name(),
		Description:        "test channel",
	})
	require.NoError(t, err)
	require.NotNil(t, channel)

	return channel
}
