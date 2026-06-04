package usersummary

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestResolveOne_NilID(t *testing.T) {
	t.Parallel()

	srv, _ := initService(t)

	// The zero id is the ONLY case that yields nil ("no author to render"); it
	// never calls the gateway. A real-but-unresolved id degrades to Unknown
	// instead (see TestResolveOne_UnknownUserDegrades).
	require.Nil(t, srv.ResolveOne(context.Background(), uuid.Nil))
}

func TestResolveOne_Resolved(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	mocks.gateway.EXPECT().
		GetUsersByIDs(gomock.Any(), []uuid.UUID{id}).
		Return(map[uuid.UUID]*entity.User{
			id: {ID: id, Name: "Alice", Email: "alice@example.com"},
		}, nil)

	got := srv.ResolveOne(context.Background(), id)
	require.NotNil(t, got)
	require.Equal(t, &entity.UserSummary{ID: id, Name: "Alice", Email: "alice@example.com"}, got)
}

// TestResolveOne_UnknownUserDegrades pins the invariant that distinguishes the
// two nil-vs-non-nil cases of ResolveOne: a real id that auth cannot resolve
// degrades to the labeled "Unknown user" summary (NOT nil). Only the zero id —
// "no author to render" — yields nil (see TestResolveOne_NilID).
func TestResolveOne_UnknownUserDegrades(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	// Auth responds without the requested id (user removed).
	mocks.gateway.EXPECT().
		GetUsersByIDs(gomock.Any(), []uuid.UUID{id}).
		Return(map[uuid.UUID]*entity.User{}, nil)

	got := srv.ResolveOne(context.Background(), id)
	require.NotNil(t, got, "unresolved real id must degrade to Unknown, not nil")
	require.Equal(t, &entity.UserSummary{ID: id, Name: entity.UnknownUserName}, got)
}

func TestResolveMany_Dedup(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	// The same id passed twice must be fetched once (deduped) and the zero id
	// dropped entirely.
	mocks.gateway.EXPECT().
		GetUsersByIDs(gomock.Any(), []uuid.UUID{id}).
		Return(map[uuid.UUID]*entity.User{
			id: {ID: id, Name: "Bob", Email: "bob@example.com"},
		}, nil).
		Times(1)

	out := srv.ResolveMany(context.Background(), []uuid.UUID{id, id, uuid.Nil})
	require.Len(t, out, 1)
	require.Equal(t, "Bob", out[id].Name)
}

func TestResolveMany_AuthDownDegrades(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	mocks.gateway.EXPECT().
		GetUsersByIDs(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("boom: "+apperr.ErrAuthUnavailable.Error()))

	out := srv.ResolveMany(context.Background(), []uuid.UUID{id})

	// Never errors the read: the id is preserved with the "Unknown user" label.
	require.Len(t, out, 1)
	require.Equal(t, &entity.UserSummary{ID: id, Name: entity.UnknownUserName}, out[id])
}

func TestResolveMany_UnknownUserDegrades(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	known := uuid.New()
	missing := uuid.New()
	// Auth responds but does not include `missing` (user removed): that id
	// degrades to the labeled fallback while the known one resolves.
	mocks.gateway.EXPECT().
		GetUsersByIDs(gomock.Any(), gomock.Any()).
		Return(map[uuid.UUID]*entity.User{
			known: {ID: known, Name: "Kate", Email: "kate@example.com"},
		}, nil)

	out := srv.ResolveMany(context.Background(), []uuid.UUID{known, missing})

	require.Equal(t, "Kate", out[known].Name)
	require.Equal(t, &entity.UserSummary{ID: missing, Name: entity.UnknownUserName}, out[missing])
}

func TestResolveMany_CacheHitAvoidsSecondCall(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	// Gateway is hit exactly once; the second resolve (well within the 1-minute
	// TTL) is served from cache.
	mocks.gateway.EXPECT().
		GetUsersByIDs(gomock.Any(), []uuid.UUID{id}).
		Return(map[uuid.UUID]*entity.User{
			id: {ID: id, Name: "Cached", Email: "c@example.com"},
		}, nil).
		Times(1)

	first := srv.ResolveOne(context.Background(), id)
	require.Equal(t, "Cached", first.Name)

	second := srv.ResolveOne(context.Background(), id)
	require.Equal(t, "Cached", second.Name)
}

func TestResolveMany_CacheExpiryRefetches(t *testing.T) {
	t.Parallel()

	// Short real TTL so expiry is observable without waiting a minute.
	const ttl = 20 * time.Millisecond
	srv, mocks := initServiceWithTTL(t, ttl)

	id := uuid.New()
	// After the TTL lapses the gateway is queried again.
	mocks.gateway.EXPECT().
		GetUsersByIDs(gomock.Any(), []uuid.UUID{id}).
		Return(map[uuid.UUID]*entity.User{
			id: {ID: id, Name: "First", Email: "f@example.com"},
		}, nil)
	mocks.gateway.EXPECT().
		GetUsersByIDs(gomock.Any(), []uuid.UUID{id}).
		Return(map[uuid.UUID]*entity.User{
			id: {ID: id, Name: "Second", Email: "s@example.com"},
		}, nil)

	require.Equal(t, "First", srv.ResolveOne(context.Background(), id).Name)

	time.Sleep(ttl * 3) // let the entry expire
	require.Equal(t, "Second", srv.ResolveOne(context.Background(), id).Name)
}

func TestResolveMany_DegradedNotCached(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	id := uuid.New()
	// First call: auth down → degraded, must NOT be cached. Second call: auth
	// recovers and resolves the real profile.
	mocks.gateway.EXPECT().
		GetUsersByIDs(gomock.Any(), []uuid.UUID{id}).
		Return(nil, errors.New("auth down"))
	mocks.gateway.EXPECT().
		GetUsersByIDs(gomock.Any(), []uuid.UUID{id}).
		Return(map[uuid.UUID]*entity.User{
			id: {ID: id, Name: "Recovered", Email: "r@example.com"},
		}, nil)

	require.Equal(t, entity.UnknownUserName, srv.ResolveOne(context.Background(), id).Name)
	require.Equal(t, "Recovered", srv.ResolveOne(context.Background(), id).Name)
}
