package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/labstack/echo/v5"
	valkeylib "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
)

func extractorCtx(t *testing.T, body string) (*echo.Context, *http.Request) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/login/otp/verify", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	return echo.New().NewContext(req, httptest.NewRecorder()), req
}

// TestOTPEmailRateLimitKeyNormalizes pins the folding. Without it "A@x" and
// "a@x" are two budgets and the whole tier is bypassed by holding shift.
func TestOTPEmailRateLimitKeyNormalizes(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		`{"email":"user@example.com"}`:     "otp-email:user@example.com",
		`{"email":"USER@Example.COM"}`:     "otp-email:user@example.com",
		`{"email":"  user@example.com  "}`: "otp-email:user@example.com",
	}

	for body, want := range cases {
		c, _ := extractorCtx(t, body)

		got, err := otpEmailRateLimitKey(c)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}

// TestOTPEmailRateLimitKeyLeavesTheBodyReadable is the reason the extractor
// restores what it read: it runs before the handler binds, and a consumed body
// would make every request fail to parse.
func TestOTPEmailRateLimitKeyLeavesTheBodyReadable(t *testing.T) {
	t.Parallel()

	const body = `{"email":"user@example.com","code":"123456"}`

	c, req := extractorCtx(t, body)

	_, err := otpEmailRateLimitKey(c)
	require.NoError(t, err)

	rest, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.JSONEq(t, body, string(rest), "the handler must still be able to bind")
}

// TestOTPEmailRateLimitKeyNeverErrors is a security test wearing a plumbing
// test's clothes.
//
// Echo answers a non-nil extractor error with HTTP 403 "error while extracting
// identifier", before the store is consulted. That is a response shape none of
// these endpoints otherwise produces, so an attacker could tell a malformed body
// apart from every other failure — exactly the distinction the uniform answers
// exist to remove. Every input, however hostile, must bucket and continue.
func TestOTPEmailRateLimitKeyNeverErrors(t *testing.T) {
	t.Parallel()

	bodies := map[string]string{
		"empty":               ``,
		"not json":            `<<<not json>>>`,
		"json but not object": `["nope"]`,
		"no email field":      `{"code":"123456"}`,
		"empty email":         `{"email":""}`,
		"whitespace email":    `{"email":"   "}`,
		"wrong type":          `{"email":123}`,
		"over the cap":        `{"email":"a@b.c","pad":"` + strings.Repeat("x", otpBodyLimitBytes*2) + `"}`,
	}

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, _ := extractorCtx(t, body)

			got, err := otpEmailRateLimitKey(c)
			require.NoError(t, err, "an extractor error becomes a 403 and leaks the branch")
			require.NotEmpty(t, got, "an empty key would pool every caller into one bucket")
		})
	}
}

// TestOTPEmailRateLimitKeyErrorsOnReadStillBucket covers the one branch whose
// comment argues hardest that returning the error would be the bug -- and which
// no other test could reach, because every other case builds its body from a
// strings.Reader that cannot fail.
//
// A body that errors mid-read is not exotic: a client that hangs up during the
// upload produces exactly this. If that returned a non-nil error, echo would
// answer 403 and hand an attacker a response shape no other failure produces.
func TestOTPEmailRateLimitKeyErrorsOnReadStillBucket(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/login/otp/verify", http.NoBody)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Body = io.NopCloser(iotest.ErrReader(errors.New("connection reset mid-body")))

	c := echo.New().NewContext(req, httptest.NewRecorder())

	got, err := otpEmailRateLimitKey(c)
	require.NoError(t, err, "a failed body read must not become a 403")
	require.Equal(t, otpUnparseableKey, got)
}

// TestOTPEmailRateLimitKeyBoundsTheBody pins the cap. This middleware runs on an
// unauthenticated route with no global body limit in front of it, so an
// unbounded read is a memory-exhaustion primitive.
func TestOTPEmailRateLimitKeyBoundsTheBody(t *testing.T) {
	t.Parallel()

	huge := `{"email":"user@example.com","pad":"` + strings.Repeat("x", otpBodyLimitBytes*4) + `"}`

	c, req := extractorCtx(t, huge)

	got, err := otpEmailRateLimitKey(c)
	require.NoError(t, err)
	require.Equal(t, otpUnparseableKey, got, "a body over the cap cannot be parsed, so it pools")

	restored, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	// Exactly the cap, not merely "no more than" it. LessOrEqual would also pass
	// if the extractor restored an empty body or skipped the restore entirely --
	// which is the regression the under-cap readability test guards, and which
	// this test would otherwise wave through.
	require.Len(t, restored, otpBodyLimitBytes,
		"the extractor must restore exactly what it was allowed to read")

	// And the documented consequence: a truncated document does not parse, so
	// the handler refuses the request instead of acting on half a body.
	var payload struct {
		Email string `json:"email"`
	}
	require.Error(t, json.Unmarshal(restored, &payload),
		"a truncated body must fail to bind rather than bind partially")
}

// TestOTPEmailRateLimitKeyResistsInvisibleSuffixes is the regression test for a
// real bypass of this tier.
//
// The key used to be built with strings.ToLower(strings.TrimSpace(...)). That
// leaves zero-width and format characters in place, and is.EmailFormat -- the
// only validator downstream -- accepts them. So "victim@example.com" plus a
// U+200B was a DIFFERENT bucket for the same address: unlimited budgets against
// one victim, defeating the one tier built to stop exactly that. Each request
// also wrote an audit row keyed on the attacker's string, into two indexed
// columns.
//
// Every variant below must collapse onto the plain address's bucket.
func TestOTPEmailRateLimitKeyResistsInvisibleSuffixes(t *testing.T) {
	t.Parallel()

	const base = "victim@example.com"

	c, _ := extractorCtx(t, `{"email":"`+base+`"}`)
	want, err := otpEmailRateLimitKey(c)
	require.NoError(t, err)

	for name, variant := range map[string]string{
		"zero-width space":          base + "\u200b",
		"zero-width no-break space": base + "\ufeff",
		"word joiner":               base + "\u2060",
		"leading zero-width space":  "\u200b" + base,
		"upper case":                "VICTIM@Example.COM",
		"surrounding spaces":        "  " + base + "  ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, _ := extractorCtx(t, `{"email":"`+variant+`"}`)

			got, err := otpEmailRateLimitKey(c)
			require.NoError(t, err)
			require.Equal(t, want, got,
				"%s must share the victim's bucket, not get one of its own", name)
		})
	}
}

// TestOTPDistinctAddressesGetDistinctKeys guards the obvious regression: a key
// that ignores its input silently turns the per-address tier into a second
// global one.
func TestOTPDistinctAddressesGetDistinctKeys(t *testing.T) {
	t.Parallel()

	first, _ := extractorCtx(t, `{"email":"a@example.com"}`)
	second, _ := extractorCtx(t, `{"email":"b@example.com"}`)

	keyA, err := otpEmailRateLimitKey(first)
	require.NoError(t, err)
	keyB, err := otpEmailRateLimitKey(second)
	require.NoError(t, err)

	require.NotEqual(t, keyA, keyB)
}

// TestOTPGlobalRateLimitKeyIsConstant pins the tier's whole point: one bucket
// for the deployment, so a sweep spread across addresses and IPs still meets a
// ceiling.
func TestOTPGlobalRateLimitKeyIsConstant(t *testing.T) {
	t.Parallel()

	first, _ := extractorCtx(t, `{"email":"a@example.com"}`)
	second, _ := extractorCtx(t, `{"email":"b@example.com"}`)

	keyA, err := otpGlobalRateLimitKey(first)
	require.NoError(t, err)
	keyB, err := otpGlobalRateLimitKey(second)
	require.NoError(t, err)

	require.Equal(t, keyA, keyB)
	require.NotEmpty(t, keyA,
		"an empty key would pool every caller into echo's zero-value bucket")
}

// callOTP drives the assembled middleware once and reports the status.
func callOTP(t *testing.T, mw echo.MiddlewareFunc, body string) int {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/login/otp/verify", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	c := echo.New().NewContext(req, rec)
	handler := mw(func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	if err := handler(c); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) {
			return he.Code
		}
		require.NoError(t, err)
	}

	return rec.Code
}

// unreachableValkey returns a client pointed at a port that refuses instantly,
// so every call errors and the hybrid limiter takes its fallback path.
func unreachableValkey(t *testing.T) *valkeylib.Client {
	t.Helper()

	rdb := valkeylib.NewClient(&valkeylib.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	return rdb
}

// TestNewOTPEmailRateLimiterFallsBackSizedToItsWindow pins the property that
// actually breaks: not that the fallback engages — the hybrid limiter's own
// package covers that — but that it is sized to the SAME window as the Valkey
// limiter it stands in for.
//
// A fallback sized from raw config while the Valkey limiter falls back to the
// package default is a real and silent mismatch; NewRateLimiter has exactly that
// shape. Here the window is read back from the limiter, so a Valkey outage
// changes where the decision is made and not what the limit is.
func TestNewOTPEmailRateLimiterFallsBackSizedToItsWindow(t *testing.T) {
	t.Parallel()

	// Burst follows the rate, so 2/min is the entire depth of the bucket: the
	// third call has nothing to draw on, and at 2/min no token returns within the
	// life of this test.
	const windowPerMinute = 2

	limited := NewOTPEmailRateLimiter("test", unreachableValkey(t), config.RateLimiterConfig{
		RequestsPerMinute: windowPerMinute,
		ExpiresIn:         time.Minute,
		Timeout:           200 * time.Millisecond,
	})

	const body = `{"email":"user@example.com"}`

	require.Equal(t, http.StatusOK, callOTP(t, limited, body))
	require.Equal(t, http.StatusOK, callOTP(t, limited, body))
	require.Equal(t, http.StatusTooManyRequests, callOTP(t, limited, body),
		"the fallback must enforce the configured window, not the package default")
}

// TestNewOTPEmailRateLimiterFallbackKeysPerAddress checks the degraded tier is
// still a per-address tier. A fallback that pooled every caller would turn a
// Valkey outage into one address's traffic throttling everyone else's.
func TestNewOTPEmailRateLimiterFallbackKeysPerAddress(t *testing.T) {
	t.Parallel()

	const windowPerMinute = 2

	limited := NewOTPEmailRateLimiter("test", unreachableValkey(t), config.RateLimiterConfig{
		RequestsPerMinute: windowPerMinute,
		ExpiresIn:         time.Minute,
		Timeout:           200 * time.Millisecond,
	})

	first := `{"email":"first@example.com"}`
	second := `{"email":"second@example.com"}`

	require.Equal(t, http.StatusOK, callOTP(t, limited, first))
	require.Equal(t, http.StatusOK, callOTP(t, limited, first))
	require.Equal(t, http.StatusTooManyRequests, callOTP(t, limited, first))

	require.Equal(t, http.StatusOK, callOTP(t, limited, second),
		"one address exhausting its budget must not spend another's")
}

// TestNewOTPGlobalRateLimiterDoesNotDenyWithoutValkey is the one place this
// change deliberately drops a control instead of degrading it, and the test
// exists so nobody "fixes" it later.
//
// The global tier's key is a constant. Give it the in-memory fallback the other
// tiers use and a Valkey outage puts every caller in one echo token bucket, so a
// single attacker holds the entire instance's sign-in surface at 429 — a
// fail-closed denial of service inside a design that fails open everywhere else.
// Losing the anti-sweep control during an outage is strictly better; the
// per-address tier, which protects an individual account, keeps working.
func TestNewOTPGlobalRateLimiterDoesNotDenyWithoutValkey(t *testing.T) {
	t.Parallel()

	// A window so small that any real bucket would refuse almost immediately.
	limited := NewOTPGlobalRateLimiter("test", unreachableValkey(t), config.RateLimiterConfig{
		RequestsPerMinute: 1,
		ExpiresIn:         time.Minute,
		Timeout:           200 * time.Millisecond,
	})

	const body = `{"email":"user@example.com"}`

	for range 10 {
		require.Equal(t, http.StatusOK, callOTP(t, limited, body),
			"during a Valkey outage the instance-wide tier must not deny")
	}
}
