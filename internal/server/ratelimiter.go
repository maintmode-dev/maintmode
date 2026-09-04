package server

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	valkeylib "github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/server/ratelimiter"
	"github.com/ruko1202/maintmode/internal/server/ratelimiter/valkeyratelimiter"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	"github.com/ruko1202/maintmode/internal/utils/xemail"
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
	// Without the default an absent config block would inherit the package
	// default of 30/min — the anti-enumeration ceiling for the login surface,
	// which on a screen group answers 429 to ordinary page loads.
	return keyedRateLimiter(appName, rdb, cfg, defaultUIRequestsPerMinute, uiRateLimitKey)
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

// defaultOTPEmailRequestsPerMinute is the per-address cap when
// auth.otp_email_rate is unset.
//
// One code costs a request plus up to five verify attempts, so a user who works
// through a whole ceiling spends six. Twenty leaves room for a confused user
// retrying a few times without ever approaching a useful guessing rate — the
// per-code attempt ceiling is what bounds guessing, far more tightly than any
// per-minute cap could.
const defaultOTPEmailRequestsPerMinute = 20

// defaultOTPGlobalRequestsPerMinute is the instance-wide cap when
// auth.otp_global_rate is unset: 10/s across the deployment. At the per-address
// cap above, this binds at roughly thirty distinct addresses per minute, which
// is the number to tune against.
const defaultOTPGlobalRequestsPerMinute = 600

// otpBodyLimitBytes caps what the per-address extractor reads before the handler
// binds. The body is a handful of short fields; this middleware runs on an
// unauthenticated route and there is no global body limit in front of it, so an
// unbounded read would be a memory-exhaustion primitive.
const otpBodyLimitBytes = 4 << 10

// NewOTPEmailRateLimiter buckets one-time-code traffic by the address in the
// request body, which is the tier that stops someone grinding one victim's code
// from many addresses. The per-IP limiter cannot: a distributed attacker changes
// IP for free, and the whole point of an invite-only instance is that the victim
// set is known.
func NewOTPEmailRateLimiter(appName string, rdb *valkeylib.Client, cfg config.RateLimiterConfig) echo.MiddlewareFunc {
	return keyedRateLimiter(appName, rdb, cfg, defaultOTPEmailRequestsPerMinute, otpEmailRateLimitKey)
}

// NewOTPGlobalRateLimiter buckets all one-time-code traffic together, which is
// the only tier that sees a sweep spread across addresses AND IP addresses —
// the shape an invite-only instance is otherwise defenseless against, since
// "one request per address per IP" meets no barrier at all from the other two.
//
// It deliberately keeps HybridRateLimiter's default noop fallback rather than
// the in-memory store the other tiers use. Its key is a constant, so a
// per-replica bucket would put every caller in one echo token bucket during a
// Valkey outage: a single attacker could hold the whole instance's sign-in
// surface at 429. Dropping the control during an outage is strictly better than
// converting the outage into a total sign-in outage — and the per-address tier,
// which is what actually protects an individual account, keeps working.
func NewOTPGlobalRateLimiter(appName string, rdb *valkeylib.Client, cfg config.RateLimiterConfig) echo.MiddlewareFunc {
	limiter := valkeyratelimiter.NewValkeyLimiter(rdb,
		valkeyratelimiter.WithWindowMinute(defaultOTPGlobalRequestsPerMinute),
		valkeyratelimiter.WithWindowMinute(cfg.RequestsPerMinute),
	)

	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: ratelimiter.NewHybridRateLimiter(appName, limiter,
			ratelimiter.WithTimeout(cfg.Timeout),
		),
		IdentifierExtractor: otpGlobalRateLimitKey,
	})
}

// keyedRateLimiter assembles a tier that buckets by its own key and falls back
// to a per-replica bucket sized to match its Valkey window.
//
// The two ordered WithWindowMinute calls are the non-positive guard, not
// decoration: the option ignores a non-positive rate, so the caller's default
// lands and an operator's own value overwrites it. Without the first call an
// absent config block would inherit the package default of 30/min. Reading
// Limit() back rather than sizing the fallback from raw config is what keeps an
// outage a change in where the decision is made and not in what the limit is —
// NewRateLimiter does neither, which is the mismatch worth not copying. Burst
// comes from the same window: left to echo, a zero would become a depth of 5
// derived from the rate, and depth is what absorbs a page load's parallel calls.
//
// The store and the identifier extractor are assembled together on purpose,
// because they are one decision. Handing back a store that each caller keyed
// separately would make "bucket this group per user" or "per address" something
// a later edit could undo without touching this file.
func keyedRateLimiter(
	appName string,
	rdb *valkeylib.Client,
	cfg config.RateLimiterConfig,
	defaultPerMinute int,
	key func(c *echo.Context) (string, error),
) echo.MiddlewareFunc {
	limiter := valkeyratelimiter.NewValkeyLimiter(rdb,
		valkeyratelimiter.WithWindowMinute(defaultPerMinute),
		valkeyratelimiter.WithWindowMinute(cfg.RequestsPerMinute),
	)

	// Burst comes from the window too: left to echo a zero would become a depth
	// of 5 derived from the rate, and depth is what absorbs a page load's
	// parallel calls.
	window := limiter.Limit()
	fallback := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
		Rate:      float64(window.Rate) / 60,
		Burst:     window.Burst,
		ExpiresIn: cfg.ExpiresIn,
	})

	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: ratelimiter.NewHybridRateLimiter(appName, limiter,
			ratelimiter.WithTimeout(cfg.Timeout),
			ratelimiter.WithFallbackLimiter(ratelimiter.LimiterNoCtx(fallback.Allow)),
		),
		IdentifierExtractor: key,
	})
}

// otpGlobalRateLimitKey pools every caller into one bucket.
func otpGlobalRateLimitKey(*echo.Context) (string, error) {
	return "otp-global", nil
}

// otpEmailRateLimitKey buckets a request by the address in its body.
//
// It NEVER returns a non-nil error, and that is a correctness requirement rather
// than a style choice. Echo routes an extractor error to ErrorHandler, whose
// default answers HTTP 403 "error while extracting identifier" before the store
// is consulted — a response shape distinguishable from every other failure of
// these endpoints, which is precisely what their uniform answers exist to
// prevent. A body it cannot read is bucketed under a constant and left for the
// handler to reject like anything else.
//
// The read is bounded and the body is put back for the handler to bind. Over the
// cap the restored copy is truncated, so binding fails and the request is
// refused — the intended outcome. Reading the whole body to avoid that
// truncation would reintroduce the memory-exhaustion primitive the cap exists
// for.
//
// The key is lower-cased and trimmed. Without that, "A@x" and "a@x" are two
// buckets and the tier is bypassed by holding shift. Folding case can in
// principle pool two RFC-5321-distinct mailboxes into one budget; that is
// accepted, since the population owning case-distinct addresses on one domain is
// empty in practice and the failure mode is a shared budget rather than a wrong
// decision. Normalization applies to the KEY only — the stored and audited
// address stays exactly as the caller sent it.
func otpEmailRateLimitKey(c *echo.Context) (string, error) {
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, otpBodyLimitBytes))
	if err != nil {
		//nolint:nilerr // returning the error here is the bug, not the fix: echo
		// turns a non-nil extractor error into HTTP 403 before the store is
		// consulted, which is a response shape these endpoints never otherwise
		// produce and hands an attacker a way to tell a malformed body from
		// every other failure.
		return otpUnparseableKey, nil
	}

	c.Request().Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		//nolint:nilerr // see above: an extractor error would answer 403 and make
		// this branch distinguishable.
		return otpUnparseableKey, nil
	}

	email := xemail.Normalize(payload.Email)
	if email == "" {
		return otpUnparseableKey, nil
	}

	return "otp-email:" + email, nil
}

// otpUnparseableKey pools every request whose address could not be read.
//
// It is a shared bucket, so an attacker sending deliberately malformed bodies
// can exhaust it and push other malformed requests from 401 to 429. Accepted:
// those requests fail either way, and the alternative — rejecting inside the
// limiter — hands back the distinguishable response the uniform answers exist to
// deny.
const otpUnparseableKey = "otp-email:unparseable"
