package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// A session lives under two independent limits, and either one ends it. They
// are checked against timestamps on the row rather than a stored deadline, so
// an operator lowering a limit affects sessions that already exist.
func TestRefresh_SessionLifetimeLimits(t *testing.T) {
	t.Parallel()

	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	// seed writes a predecessor with the given timestamps directly: the two
	// limits are evaluated against them, and going through a normal sign-in
	// would always produce "just now" for both.
	seed := func(t *testing.T, srv *Service, store *refreshtoken.Store,
		sessionStartedAt, lastRotatedAt time.Time,
	) string {
		t.Helper()

		user := testdbutils.MakeUser(ctx, t, users.NewStore(db))
		raw, hashed, err := srv.tokenSrv.GenerateRefreshToken(ctx)
		require.NoError(t, err)

		require.NoError(t, store.Save(ctx, &entity.RefreshToken{
			Token:            hashed,
			UserID:           user.ID,
			Family:           uuid.New(),
			ExpiresAt:        xtime.UTCNow().Add(time.Hour),
			BoundIP:          "10.0.0.1",
			SessionStartedAt: sessionStartedAt,
		}))
		// created_at is set by the database on insert, so the idle clock has to
		// be moved separately to simulate a row written a while ago. Done with
		// raw SQL rather than a production setter: nothing outside a test has
		// any business rewriting when a row was created.
		_, err = db.ExecContext(ctx,
			`UPDATE refresh_tokens SET created_at = $1 WHERE token_hash = $2`,
			lastRotatedAt, hashed)
		require.NoError(t, err)

		return raw
	}

	t.Run("an active session keeps rotating", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)
		store := refreshtoken.NewStore(db)
		now := xtime.UTCNow()

		raw := seed(t, srv, store, now.Add(-24*time.Hour), now.Add(-time.Minute))

		pair, err := srv.Refresh(ctx, raw, "10.0.0.1")
		require.NoError(t, err)
		require.NotEmpty(t, pair.RefreshToken)
	})

	t.Run("idleness past the inactive limit ends the session", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)
		store := refreshtoken.NewStore(db)
		now := xtime.UTCNow()

		// Well inside the maximum, but untouched for longer than the idle limit.
		raw := seed(t, srv, store,
			now.Add(-24*time.Hour),
			now.Add(-srv.cfg.SessionInactiveLifetime).Add(-time.Minute))

		_, err := srv.Refresh(ctx, raw, "10.0.0.1")
		require.ErrorIs(t, err, apperr.ErrTokenExpired)
	})

	t.Run("the maximum lifetime ends even a busy session", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)
		store := refreshtoken.NewStore(db)
		now := xtime.UTCNow()

		// Rotated a minute ago -- as active as a session gets -- but signed in
		// longer ago than the ceiling allows. This is the case the old
		// inherit-the-expiry logic could not express at all.
		raw := seed(t, srv, store,
			now.Add(-srv.cfg.SessionMaxLifetime).Add(-time.Minute),
			now.Add(-time.Minute))

		_, err := srv.Refresh(ctx, raw, "10.0.0.1")
		require.ErrorIs(t, err, apperr.ErrTokenExpired)
	})

	t.Run("the session start is carried across rotation", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)
		store := refreshtoken.NewStore(db)
		now := xtime.UTCNow()
		startedAt := now.Add(-24 * time.Hour)

		raw := seed(t, srv, store, startedAt, now.Add(-time.Minute))

		pair, err := srv.Refresh(ctx, raw, "10.0.0.1")
		require.NoError(t, err)

		successor, err := store.GetByTokenHash(ctx, xhash.HashSha256([]byte(pair.RefreshToken)))
		require.NoError(t, err)

		// Losing it would restart the ceiling on every refresh, quietly turning
		// the maximum lifetime into no maximum at all.
		require.WithinDuration(t, startedAt, successor.SessionStartedAt, time.Second)
	})
}
