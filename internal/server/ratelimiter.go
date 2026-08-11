package server

import (
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	valkeylib "github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/server/ratelimiter"
	"github.com/ruko1202/maintmode/internal/server/ratelimiter/valkeyratelimiter"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// defaultUIRequestsPerMinute is the fallback cap for the /ui/v1 screen group.
//
// 300/min is 5 requests per second sustained: an order of magnitude above any
// live navigation (a page load is single digits of requests) and an order of
// magnitude below what a polling loop can draw. It is sized to separate a
// machine from a person, which is the only job it has — it is not a defense
// against a determined insider, who holds a valid token by definition.
const defaultUIRequestsPerMinute = 300

func NewRateLimiter(appName string, rdb *valkeylib.Client, cfg config.RateLimiterConfig) *ratelimiter.HybridRateLimiter {
	// echo's in-memory fallback is a token bucket whose Rate is in
	// requests/second, so convert the per-minute config.
	fallback := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
		Rate:      float64(cfg.RequestsPerMinute) / 60,
		Burst:     cfg.Burst,
		ExpiresIn: cfg.ExpiresIn,
	})

	return ratelimiter.NewHybridRateLimiter(appName,
		valkeyratelimiter.NewValkeyLimiter(rdb,
			valkeyratelimiter.WithWindowMinute(cfg.RequestsPerMinute),
		),
		ratelimiter.WithTimeout(cfg.Timeout),
		ratelimiter.WithFallbackLimiter(ratelimiter.LimiterNoCtx(fallback.Allow)),
	)
}

// NewUIRateLimiter builds the rate-limiting middleware for the /ui/v1 screen
// group. It reuses the whole hybrid store (Valkey, with the per-replica
// in-memory fallback on a Valkey outage) and differs from NewRateLimiter in the
// two ways the screen group needs: requests are bucketed per user rather than
// per IP, and the threshold comes from its own config block.
//
// The middleware is returned already assembled rather than as a store, because
// the store and the identifier extractor are one decision. Handing back a store
// that the caller keys separately would make "limit /ui/v1 per user" something a
// later edit could undo without touching this file.
//
// It MUST be mounted after RequireAccessToken — see uiV1Middlewares.
func NewUIRateLimiter(appName string, rdb *valkeylib.Client, cfg config.RateLimiterConfig) echo.MiddlewareFunc {
	// Ordered options, not a hand-rolled resolver: the option is a no-op for a
	// non-positive rate, so ours lands and an operator's own value overwrites it.
	// Without the first call an absent config block would inherit the package
	// default of 30/min — the anti-enumeration ceiling for the login surface,
	// which on a screen group answers 429 to ordinary page loads.
	limiter := valkeyratelimiter.NewValkeyLimiter(rdb,
		valkeyratelimiter.WithWindowMinute(defaultUIRequestsPerMinute),
		valkeyratelimiter.WithWindowMinute(cfg.RequestsPerMinute),
	)

	// The fallback mirrors the window the options settled on, so an outage
	// changes where the decision is made, not what the limit is. Burst comes
	// from there too: left to echo, a zero would become a depth of 5 derived
	// from the rate, and depth is what absorbs a page load's parallel calls.
	window := limiter.Limit()
	fallback := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
		Rate:      float64(window.Rate) / 60,
		Burst:     window.Burst,
		ExpiresIn: cfg.ExpiresIn,
	})

	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: ratelimiter.NewHybridRateLimiter(appName,
			limiter,
			ratelimiter.WithTimeout(cfg.Timeout),
			ratelimiter.WithFallbackLimiter(ratelimiter.LimiterNoCtx(fallback.Allow)),
		),
		IdentifierExtractor: uiRateLimitKey,
	})
}

// uiRateLimitKey buckets a /ui/v1 request by the authenticated user, falling
// back to the remote address when the context carries no user.
//
// Keying by user is the point of running this limiter in the application rather
// than at the perimeter: a whole team behind one corporate NAT shares an IP, and
// an IP key would let one person's polling loop throttle their colleagues.
//
// The fallback is unreachable in production — RequireAccessToken either
// populates the context or rejects the request before this runs — but a coarse
// key beats the alternatives if that ever stops holding: returning an error
// fails the request on a limiter's bookkeeping, and an empty key pools every
// caller into a single bucket, which is a sharper version of the very problem
// this limiter was added to fix.
func uiRateLimitKey(c *echo.Context) (string, error) {
	if user, ok := xecho.UserFromEchoCtx(c); ok && user != nil {
		return "user:" + user.ID.String(), nil
	}

	return "ip:" + c.RealIP(), nil
}
