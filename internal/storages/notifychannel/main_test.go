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

// makeNamedChannel inserts a channel under a caller-chosen name. Filter tests
// need this: makeChannel names every row after the test, which repeats across
// the `-count 2` runs the suite does against a shared database, so a name-based
// assertion would see rows from the previous run.
//
// The transport is fixed: these tests are about the name filter and paging, and
// the ordering they assert is (transport, transport_channel_id), so holding the
// first key constant is what makes the second one the thing under test.
func makeNamedChannel(ctx context.Context, t *testing.T, name string) *entity.NotifyChannel {
	t.Helper()

	channel, err := store.Create(ctx, &entity.NotifyChannel{
		Transport:          entity.NotifyTransportSlack,
		TransportChannelID: name + "-" + xuuid.NewString(),
		Name:               name,
		Description:        "test channel",
	})
	require.NoError(t, err)
	require.NotNil(t, channel)

	return channel
}

// uniquePrefix returns a name prefix no other test or run shares, so a `name`
// filter selects exactly the rows the caller seeded.
func uniquePrefix(t *testing.T) string {
	t.Helper()

	return t.Name() + "-" + xuuid.NewString()
}

// listCmd builds a command with the paging the store applies verbatim. Tests
// must always set Limit — a zero one renders as LIMIT 0 and returns nothing,
// which reads as "the catalog is empty".
func listCmd(name string, limit, offset int64, includeArchived bool) *entity.ListChannelsCmd {
	return &entity.ListChannelsCmd{
		Name:            name,
		Limit:           limit,
		Offset:          offset,
		IncludeArchived: includeArchived,
	}
}
