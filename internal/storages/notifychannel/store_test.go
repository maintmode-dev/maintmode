package notifychannel

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestStore_Create_AssignsUUIDAndActive(t *testing.T) {
	ctx := context.Background()

	channel := makeChannel(ctx, t, entity.NotifyTransportSlack)

	require.NotEqual(t, uuid.Nil, channel.ID)
	require.Nil(t, channel.ArchivedAt, "freshly created channel must be active")
	require.Equal(t, entity.NotifyTransportSlack, channel.Transport)
}

func TestStore_Create_DuplicateIsConflict(t *testing.T) {
	ctx := context.Background()

	channel := makeChannel(ctx, t, entity.NotifyTransportSlack)

	_, err := store.Create(ctx, &entity.NotifyChannel{
		Transport:          channel.Transport,
		TransportChannelID: channel.TransportChannelID,
		Name:               "dup",
		Description:        "dup",
	})
	require.ErrorIs(t, err, apperr.ErrNotifyChannelAlreadyExists)
}

func TestStore_Get_ByUUID(t *testing.T) {
	ctx := context.Background()

	channel := makeChannel(ctx, t, entity.NotifyTransportTelegram)

	got, err := store.Get(ctx, channel.ID)
	require.NoError(t, err)
	require.Equal(t, channel.ID, got.ID)
	require.Equal(t, channel.TransportChannelID, got.TransportChannelID)
	require.Equal(t, channel.Transport, got.Transport)
}

func TestStore_Get_UnknownIsNotFound(t *testing.T) {
	ctx := context.Background()

	_, err := store.Get(ctx, uuid.New())
	require.ErrorIs(t, err, apperr.ErrNotifyChannelNotFound)
}

func TestStore_List_HidesArchivedByDefault(t *testing.T) {
	ctx := context.Background()

	prefix := uniquePrefix(t)
	channel := makeNamedChannel(ctx, t, prefix)
	require.NoError(t, store.Archive(ctx, channel.ID))

	active, total, err := store.List(ctx, listCmd(prefix, 50, 0, false))
	require.NoError(t, err)
	require.False(t, containsID(active, channel.ID), "archived channel must be hidden from default List")
	require.Zero(t, total, "total must exclude archived channels by default")

	all, total, err := store.List(ctx, listCmd(prefix, 50, 0, true))
	require.NoError(t, err)
	require.True(t, containsID(all, channel.ID), "archived channel must appear when includeArchived=true")
	require.EqualValues(t, 1, total)
}

func TestStore_List_OrderedByTransportThenChannelID(t *testing.T) {
	ctx := context.Background()

	// Seeded under a unique prefix and walked one page at a time: without the
	// filter this would assert ordering over whichever rows happen to occupy
	// page 0 of a shared catalog, none of them seeded here.
	const seeded = 6

	prefix := uniquePrefix(t)
	for range seeded {
		makeNamedChannel(ctx, t, prefix)
	}

	const pageSize = 4

	first, total, err := store.List(ctx, listCmd(prefix, pageSize, 0, true))
	require.NoError(t, err)
	require.EqualValues(t, seeded, total)
	require.Len(t, first, pageSize)

	second, _, err := store.List(ctx, listCmd(prefix, pageSize, pageSize, true))
	require.NoError(t, err)
	require.Len(t, second, seeded-pageSize)

	walked := slices.Concat(first, second)
	requireOrderedByTransportThenChannelID(t, walked)

	// Paging over a total order neither drops nor repeats a row: the two pages
	// hold `seeded` distinct ids, so they neither overlap nor miss one.
	ids := lo.Map(walked, func(ch *entity.NotifyChannel, _ int) uuid.UUID { return ch.ID })
	require.Len(t, lo.Uniq(ids), seeded,
		"the two pages must not overlap and must cover the seeded set")
}

func TestStore_List_FiltersByNameCaseInsensitively(t *testing.T) {
	ctx := context.Background()

	prefix := uniquePrefix(t)
	wanted := makeNamedChannel(ctx, t, prefix+"-wanted")
	makeNamedChannel(ctx, t, uniquePrefix(t)+"-other")

	channels, total, err := store.List(ctx, listCmd(strings.ToUpper(prefix), 50, 0, false))
	require.NoError(t, err)
	require.Len(t, channels, 1, "an upper-cased query must still match a lower-cased name")
	require.Equal(t, wanted.ID, channels[0].ID)
	require.EqualValues(t, 1, total, "total must count the filtered set, not the catalog")
}

func TestStore_List_NameMetacharactersMatchLiterally(t *testing.T) {
	ctx := context.Background()

	// Each of these is a LIKE metacharacter (or the escape character itself):
	// unescaped, the first two would widen the filter instead of narrowing it.
	for _, meta := range []string{"%", "_", `\`} {
		t.Run(meta, func(t *testing.T) {
			prefix := uniquePrefix(t)
			literal := makeNamedChannel(ctx, t, prefix+meta+"literal")
			decoy := makeNamedChannel(ctx, t, prefix+"Xliteral")

			channels, total, err := store.List(ctx, listCmd(prefix+meta, 50, 0, false))
			require.NoError(t, err)
			require.EqualValues(t, 1, total)
			require.Len(t, channels, 1)
			require.Equal(t, literal.ID, channels[0].ID)
			require.False(t, containsID(channels, decoy.ID),
				"%q must match itself, not act as a wildcard", meta)
		})
	}
}

func TestStore_List_BlankNameAppliesNoFilter(t *testing.T) {
	ctx := context.Background()

	// The handler trims, so the store sees "" for a whitespace-only query. The
	// guard has to survive that: a bare `!= ""` check on an untrimmed value
	// would build LIKE '%   %' and quietly return almost nothing.
	makeChannel(ctx, t, entity.NotifyTransportSlack)

	channels, total, err := store.List(ctx, listCmd("", 50, 0, true))
	require.NoError(t, err)
	require.NotEmpty(t, channels, "an empty name must not filter anything out")
	require.GreaterOrEqual(t, total, int64(len(channels)))
}

func TestStore_List_OffsetPastEndIsEmptyPageWithTotal(t *testing.T) {
	ctx := context.Background()

	prefix := uniquePrefix(t)
	makeNamedChannel(ctx, t, prefix)

	channels, total, err := store.List(ctx, listCmd(prefix, 50, 500, false))
	require.NoError(t, err)
	require.Empty(t, channels, "an offset past the end is an empty page, not an error")
	require.EqualValues(t, 1, total, "total describes the filtered set, not the page")
}

func requireOrderedByTransportThenChannelID(t *testing.T, channels []*entity.NotifyChannel) {
	t.Helper()

	for i := 1; i < len(channels); i++ {
		prev, cur := channels[i-1], channels[i]
		prevKey := string(prev.Transport) + ":" + prev.TransportChannelID
		curKey := string(cur.Transport) + ":" + cur.TransportChannelID
		require.LessOrEqual(t, prevKey, curKey,
			"List must be ordered by (transport, transport_channel_id)")
	}
}

func TestStore_Archive_Unarchive_RoundTrip(t *testing.T) {
	ctx := context.Background()

	channel := makeChannel(ctx, t, entity.NotifyTransportTelegram)

	require.NoError(t, store.Archive(ctx, channel.ID))
	archived, err := store.Get(ctx, channel.ID)
	require.NoError(t, err)
	require.NotNil(t, archived.ArchivedAt, "archived channel must carry archived_at")

	require.NoError(t, store.Unarchive(ctx, channel.ID))
	active, err := store.Get(ctx, channel.ID)
	require.NoError(t, err)
	require.Nil(t, active.ArchivedAt, "unarchived channel must clear archived_at")
}

func TestStore_Archive_UnknownIsNoop(t *testing.T) {
	ctx := context.Background()

	// Idempotent: archiving/unarchiving an unknown id updates zero rows and
	// is a success, mirroring resource archive semantics.
	require.NoError(t, store.Archive(ctx, uuid.New()))
	require.NoError(t, store.Unarchive(ctx, uuid.New()))
}

func containsID(channels []*entity.NotifyChannel, id uuid.UUID) bool {
	for _, ch := range channels {
		if ch.ID == id {
			return true
		}
	}
	return false
}
