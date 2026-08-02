package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// TestUIRateLimitKey pins which identity the /ui/v1 limiter buckets by.
//
// The user branch is the whole reason this limiter lives in the application
// instead of in Caddy: keying by IP would put an entire team behind one
// corporate NAT into a single bucket, so one person's polling loop would
// throttle their colleagues.
//
// The IP branch is unreachable in production — the group sits behind
// RequireAccessToken, which populates the context or rejects the request — so
// this test is the only place it is ever exercised. It exists because the
// alternative for a missing user (an error, or an empty key) is worse than a
// coarse key: an empty key silently pools every caller into one bucket, which
// is the failure this ticket is fixing, arrived at from the other direction.
func TestUIRateLimitKey(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	t.Run("a user in the context is keyed by id", func(t *testing.T) {
		t.Parallel()

		c := newEchoContextForTest(t)
		xecho.UserToEchoCtx(c, &entity.User{ID: userID})

		got, err := uiRateLimitKey(c)

		require.NoError(t, err)
		require.Equal(t, "user:"+userID.String(), got)
	})

	// The "ip:" prefix is not decoration: without the prefixes a user whose id
	// happened to read like an address would share that address's bucket.
	t.Run("no user in the context falls back to the remote address", func(t *testing.T) {
		t.Parallel()

		c := newEchoContextForTest(t)

		got, err := uiRateLimitKey(c)

		require.NoError(t, err)
		require.Equal(t, "ip:"+c.RealIP(), got)
	})
}

// TestNewUIRateLimiterFallsBackWithoutRedis exercises the degraded path of the
// assembled /ui/v1 limiter: Redis unreachable, so every decision comes from the
// per-replica in-memory bucket.
//
// The hybrid limiter's fallback mechanism is covered generically in its own
// package. What is only observable here is that NewUIRateLimiter assembles it
// correctly for this group — that the screen routes stay SERVED during a Redis
// outage rather than failing closed, and that the memory store is reached
// through the per-user key. An outage of a dependency added to protect the
// service must not take the service down.
func TestNewUIRateLimiterFallsBackWithoutRedis(t *testing.T) {
	t.Parallel()

	// Port 1 on the loopback refuses instantly, so every Redis call errors and
	// the hybrid limiter defers to the in-memory bucket.
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	// A deliberately tiny window: burst follows the rate, so 2/min is the whole
	// depth of the bucket and the third call has nothing left to draw on. The
	// sustained rate would not refuse it — at 2/min a token returns every 30s,
	// far longer than this test takes — so what answers 429 is the depth.
	const windowPerMinute = 2

	e := echo.New()
	limited := NewUIRateLimiter("test", rdb, config.RateLimiterConfig{
		RequestsPerMinute: windowPerMinute,
		ExpiresIn:         time.Minute,
		Timeout:           200 * time.Millisecond,
	})

	user := &entity.User{ID: uuid.New()}
	handler := limited(func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	call := func() int {
		req := httptest.NewRequest(http.MethodGet, "/ui/v1/calendar", http.NoBody)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		xecho.UserToEchoCtx(c, user)

		if err := handler(c); err != nil {
			var httpErr *echo.HTTPError
			if errors.As(err, &httpErr) {
				return httpErr.Code
			}

			return http.StatusInternalServerError
		}

		return rec.Code
	}

	require.Equal(t, http.StatusOK, call(), "first request is served from the in-memory bucket")
	require.Equal(t, http.StatusOK, call(), "the bucket's burst allows a second")
	require.Equal(t, http.StatusTooManyRequests, call(),
		"the fallback still limits — an outage must not remove the cap entirely")
}

func newEchoContextForTest(t *testing.T) *echo.Context {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/ui/v1/calendar", http.NoBody)

	return e.NewContext(req, httptest.NewRecorder())
}
